//! Status, the thread-local error state, the guard of every exported
//! function and the helpers for pointers, strings and callbacks
//! (ADR-0021 §3, §4, §7, §8).

// The ABI boundary: raw pointers and `extern "C"` functions (ADR-0026 §4).
#![allow(unsafe_code)]

use std::cell::{Cell, RefCell};
use std::ffi::{CStr, CString, c_char, c_void};
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::sync::Arc;
use std::sync::atomic::Ordering;

use crate::app::Shared;
use crate::types::StatusCode;

/// Result of every fallible function: `OEC_OK` or an `OEC_ERR_*` value.
/// On error `oec_last_error` has the message.
pub type Status = i32;

/// Success.
pub const OEC_OK: Status = 0;
/// An argument is NULL, no UTF-8, out of range or violates the guideline,
/// e.g. an invalid Identification Link (REQ-IL-008) or JSON that does not
/// fit the definition.
pub const OEC_ERR_INVALID_ARGUMENT: Status = 1;
/// The configuration cannot fulfill the chosen level.
pub const OEC_ERR_LEVEL_NOT_MET: Status = 2;
/// Container storage missing or invalid (REQ-CNT-004, REQ-CNT-009), or the
/// TLS material unusable.
pub const OEC_ERR_BOOTSTRAP: Status = 3;
/// Connection, TLS or file I/O.
pub const OEC_ERR_IO: Status = 4;
/// No answer in time (REQ-ACT-009).
pub const OEC_ERR_TIMEOUT: Status = 5;
/// Not now: e.g. `oec_app_run` while running, a blocking request or
/// `oec_app_free` inside a callback.
pub const OEC_ERR_INVALID_STATE: Status = 6;
/// The app was shut down.
pub const OEC_ERR_CLOSED: Status = 7;
/// A request while the app is offline (nothing is queued), or the
/// connection was lost before the answer.
pub const OEC_ERR_NOT_CONNECTED: Status = 8;
/// The responder answered with a Status event Bad (REQ-CMP-047);
/// `oec_last_status_code` has its code.
pub const OEC_ERR_STATUS: Status = 9;
/// A panic inside the SDK; the app is unusable afterwards.
pub const OEC_ERR_PANIC: Status = 98;
/// Any other failure.
pub const OEC_ERR_INTERNAL: Status = 99;

/// Version of this ABI; changes on every incompatible change.
pub const OEC_ABI_VERSION: u32 = 1u32;

const VERSION: &str = concat!(env!("CARGO_PKG_VERSION"), "\0");

thread_local! {
    static LAST_ERROR: RefCell<CString> = RefCell::new(CString::default());
    static LAST_STATUS_CODE: Cell<StatusCode> = const { Cell::new(0) };
    static IN_CALLBACK: Cell<bool> = const { Cell::new(false) };
    /// The app of the handle the current call uses; poisoned on a panic.
    static ACTIVE: RefCell<Option<Arc<Shared>>> = const { RefCell::new(None) };
}

/// An error on its way to the caller.
#[derive(Debug)]
pub(crate) struct Fail {
    status: Status,
    message: String,
    code: StatusCode,
}

impl Fail {
    pub(crate) fn new(status: Status, message: impl Into<String>) -> Self {
        Self {
            status,
            message: message.into(),
            code: 0,
        }
    }

    pub(crate) fn invalid(message: impl Into<String>) -> Self {
        Self::new(OEC_ERR_INVALID_ARGUMENT, message)
    }

    pub(crate) fn state(message: impl Into<String>) -> Self {
        Self::new(OEC_ERR_INVALID_STATE, message)
    }

    /// `OEC_ERR_STATUS` with the Bad StatusCode of the responder.
    pub(crate) fn status(message: impl Into<String>, code: StatusCode) -> Self {
        Self {
            status: OEC_ERR_STATUS,
            message: message.into(),
            code,
        }
    }

    fn report(self) -> Status {
        LAST_ERROR.set(c_string(&self.message));
        LAST_STATUS_CODE.set(self.code);
        self.status
    }
}

impl From<oec::Error> for Fail {
    fn from(e: oec::Error) -> Self {
        let message = e.to_string();
        let (status, code) = match e {
            oec::Error::Invalid(_) => (OEC_ERR_INVALID_ARGUMENT, 0),
            oec::Error::LevelNotMet(_) => (OEC_ERR_LEVEL_NOT_MET, 0),
            oec::Error::Bootstrap(_) => (OEC_ERR_BOOTSTRAP, 0),
            oec::Error::Io(_) => (OEC_ERR_IO, 0),
            oec::Error::Timeout => (OEC_ERR_TIMEOUT, 0),
            oec::Error::Status(c) => (OEC_ERR_STATUS, c.code()),
            oec::Error::NotConnected => (OEC_ERR_NOT_CONNECTED, 0),
            oec::Error::Closed => (OEC_ERR_CLOSED, 0),
            _ => (OEC_ERR_INTERNAL, 0),
        };
        Self {
            status,
            message,
            code,
        }
    }
}

pub(crate) type Res<T> = Result<T, Fail>;

/// Runs `f` with a panic guard: an error becomes the thread's last error
/// and its status, a panic `OEC_ERR_PANIC` and marks the app of the
/// handle in use as unusable.
pub(crate) fn run<T>(f: impl FnOnce() -> Res<T>) -> Result<T, Status> {
    let outer = ACTIVE.take();
    let result = catch_unwind(AssertUnwindSafe(f));
    let active = ACTIVE.replace(outer);
    match result {
        Ok(Ok(v)) => Ok(v),
        Ok(Err(fail)) => Err(fail.report()),
        Err(panic) => {
            if let Some(shared) = active {
                shared.poisoned.store(true, Ordering::SeqCst);
            }
            let what = panic
                .downcast_ref::<&str>()
                .map(|s| (*s).to_owned())
                .or_else(|| panic.downcast_ref::<String>().cloned())
                .unwrap_or_default();
            Err(Fail::new(
                OEC_ERR_PANIC,
                format!("panic in the SDK: {what}; the app is unusable"),
            )
            .report())
        }
    }
}

/// [`run`] for the functions that return `oec_status`.
pub(crate) fn guard(f: impl FnOnce() -> Res<()>) -> Status {
    run(f).map_or_else(|status| status, |()| OEC_OK)
}

/// A handle that belongs to an app.
pub(crate) trait Handle {
    fn shared(&self) -> &Arc<Shared>;
}

/// The handle behind `p`, NULL refused; fails for an app that panicked.
///
/// # Safety
///
/// `p` is NULL or a live handle of type `T` (header contract).
pub(crate) unsafe fn handle<'a, T: Handle>(p: *const T, what: &str) -> Res<&'a T> {
    // SAFETY: NULL or a live handle (caller contract).
    let h = unsafe { p.as_ref() }.ok_or_else(|| Fail::invalid(format!("{what} is NULL")))?;
    let shared = h.shared();
    ACTIVE.set(Some(Arc::clone(shared)));
    if shared.poisoned.load(Ordering::SeqCst) {
        return Err(Fail::new(
            OEC_ERR_PANIC,
            "the app is unusable after a panic in the SDK",
        ));
    }
    Ok(h)
}

/// A handle without app (product, builder), NULL refused.
///
/// # Safety
///
/// `p` is NULL or a live handle of type `T`, not used concurrently.
pub(crate) unsafe fn plain<'a, T>(p: *mut T, what: &str) -> Res<&'a mut T> {
    // SAFETY: NULL or a live, exclusively used handle (caller contract).
    unsafe { p.as_mut() }.ok_or_else(|| Fail::invalid(format!("{what} is NULL")))
}

/// A borrowed input string, NULL refused.
///
/// # Safety
///
/// `p` is NULL or a NUL-terminated string that lives for the call.
pub(crate) unsafe fn text<'a>(p: *const c_char, what: &str) -> Res<&'a str> {
    // SAFETY: as documented.
    unsafe { opt_text(p, what) }?.ok_or_else(|| Fail::invalid(format!("{what} is NULL")))
}

/// A borrowed input string or NULL.
///
/// # Safety
///
/// `p` is NULL or a NUL-terminated string that lives for the call.
pub(crate) unsafe fn opt_text<'a>(p: *const c_char, what: &str) -> Res<Option<&'a str>> {
    if p.is_null() {
        return Ok(None);
    }
    // SAFETY: a NUL-terminated string (caller contract).
    unsafe { CStr::from_ptr(p) }
        .to_str()
        .map(Some)
        .map_err(|_| Fail::invalid(format!("{what} is no UTF-8")))
}

/// The output pointer `p`, NULL refused; `*p` is set to NULL first.
///
/// # Safety
///
/// `p` is NULL or valid for writes.
pub(crate) unsafe fn out<'a, T>(p: *mut *mut T, what: &str) -> Res<&'a mut *mut T> {
    // SAFETY: NULL or writable (caller contract).
    let out = unsafe { p.as_mut() }.ok_or_else(|| Fail::invalid(format!("{what} is NULL")))?;
    *out = std::ptr::null_mut();
    Ok(out)
}

/// A C string of `s`; a U+0000 inside (never in valid JSON or an IL) is
/// dropped.
pub(crate) fn c_string(s: &str) -> CString {
    CString::new(s).unwrap_or_else(|e| {
        let mut bytes = e.into_vec();
        bytes.retain(|&b| b != 0);
        CString::new(bytes).unwrap_or_default()
    })
}

/// The `user_data` of a callback, passed back unchanged.
#[derive(Clone, Copy)]
pub(crate) struct UserData(*mut c_void);

// SAFETY: the SDK only passes the pointer back to the caller's callback;
// the header obliges the caller to make it usable on the SDK thread.
unsafe impl Send for UserData {}
// SAFETY: as for Send; the SDK never dereferences it.
unsafe impl Sync for UserData {}

impl UserData {
    pub(crate) fn new(p: *mut c_void) -> Self {
        Self(p)
    }

    pub(crate) fn get(self) -> *mut c_void {
        self.0
    }
}

/// Runs a callback of the caller: inside it, blocking requests and
/// `oec_app_free` are refused ([`outside_callback`]).
pub(crate) fn callback<R>(f: impl FnOnce() -> R) -> R {
    struct Reset(bool);
    impl Drop for Reset {
        fn drop(&mut self) {
            IN_CALLBACK.set(self.0);
        }
    }
    let _reset = Reset(IN_CALLBACK.replace(true));
    f()
}

/// `OEC_ERR_INVALID_STATE` inside a callback.
pub(crate) fn outside_callback(what: &str) -> Res<()> {
    if IN_CALLBACK.get() {
        return Err(Fail::state(format!(
            "{what} inside a callback (it would wait for the thread that runs the callback)"
        )));
    }
    Ok(())
}

/// The message of the last error on the calling thread; never NULL,
/// borrowed until the next failing `oec_` call on this thread.
#[unsafe(no_mangle)]
pub extern "C" fn oec_last_error() -> *const c_char {
    LAST_ERROR.with_borrow(|s| s.as_ptr())
}

/// The Bad StatusCode of the last `OEC_ERR_STATUS` on the calling thread;
/// `OEC_GOOD` if the last error was another one.
#[unsafe(no_mangle)]
pub extern "C" fn oec_last_status_code() -> StatusCode {
    LAST_STATUS_CODE.get()
}

/// The ABI version of the loaded library (`OEC_ABI_VERSION` it was built
/// with).
#[unsafe(no_mangle)]
pub extern "C" fn oec_abi_version() -> u32 {
    OEC_ABI_VERSION
}

/// The SDK version; static, never freed.
#[unsafe(no_mangle)]
pub extern "C" fn oec_version() -> *const c_char {
    VERSION.as_ptr().cast()
}

/// Releases a string returned by the SDK (e.g. by `oec_config_get`); NULL
/// is ignored.
///
/// # Safety
///
/// `s` is NULL or a string returned by the SDK and not yet released.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_string_free(s: *mut c_char) {
    if !s.is_null() {
        // SAFETY: created by CString::into_raw of this library (contract).
        drop(unsafe { CString::from_raw(s) });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn last_error() -> String {
        // SAFETY: never NULL, NUL-terminated.
        unsafe { CStr::from_ptr(oec_last_error()) }
            .to_str()
            .unwrap()
            .to_owned()
    }

    #[test]
    fn errors_and_panics_are_reported() {
        assert_eq!(last_error(), "");
        assert_eq!(guard(|| Ok(())), OEC_OK);
        assert_eq!(
            guard(|| Err(Fail::invalid("bad\0 input"))),
            OEC_ERR_INVALID_ARGUMENT
        );
        assert_eq!(last_error(), "bad input");
        assert_eq!(
            guard(|| Err(oec::Error::Status(oec::status::StatusCode::BadNotFound).into())),
            OEC_ERR_STATUS
        );
        assert_eq!(oec_last_status_code(), 0x803E_0000);
        assert_eq!(guard(|| Err(oec::Error::Timeout.into())), OEC_ERR_TIMEOUT);
        assert_eq!(oec_last_status_code(), 0);
        assert_eq!(guard(|| panic!("boom")), OEC_ERR_PANIC);
        assert!(last_error().contains("boom"), "{}", last_error());
    }

    #[test]
    fn callbacks_refuse_blocking_calls() {
        assert!(outside_callback("x").is_ok());
        let inner = callback(|| callback(|| outside_callback("x").unwrap_err().status));
        assert_eq!(inner, OEC_ERR_INVALID_STATE);
        assert!(outside_callback("x").is_ok());
        let _ = std::panic::catch_unwind(|| callback(|| panic!("reset anyway")));
        assert!(outside_callback("x").is_ok());
    }

    #[test]
    fn strings() {
        // SAFETY: NULL and C string literals.
        unsafe {
            assert!(text(std::ptr::null(), "x").is_err());
            assert_eq!(opt_text(std::ptr::null(), "x").unwrap(), None);
            assert_eq!(text(c"ok".as_ptr(), "x").unwrap(), "ok");
            assert!(text(c"\xff".as_ptr(), "x").is_err());
            oec_string_free(std::ptr::null_mut());
            oec_string_free(c_string("owned").into_raw());
            let v = CStr::from_ptr(oec_version()).to_str().unwrap();
            assert_eq!(v, env!("CARGO_PKG_VERSION"));
        }
        assert_eq!(oec_abi_version(), OEC_ABI_VERSION);
    }
}
