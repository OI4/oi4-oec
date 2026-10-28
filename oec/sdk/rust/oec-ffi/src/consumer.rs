//! The consumer side: subscriptions, `Get`, `Set`, `Del` and the Action
//! `NewDataSetWriterId` (ADR-0021 §7, amendments C-060/C-061; ADR-0020
//! §7, §19).

// The ABI boundary: raw pointers and `extern "C"` functions (ADR-0026 §4).
#![allow(unsafe_code)]

use std::ffi::{c_char, c_void};
use std::ptr;
use std::time::Duration;

use oec::consumer::{ActionOptions, GetOptions, Target, WriteOptions};
use oec::identity::IdentificationLink;

use crate::abi::{
    Fail, Res, Status, UserData, c_string, callback, guard, handle, opt_text, outside_callback,
    text,
};
use crate::app::App;
use crate::types::{self, Resource, ServiceType, StatusCode};

/// A received DataSetMessage; every string is borrowed for the callback.
/// `source` and `filter` are NULL where the topic has none.
#[repr(C)]
pub struct Received {
    /// The resource of the topic.
    pub resource: Resource,
    /// The ServiceType of the topic.
    pub service_type: *const c_char,
    /// The AppId of the topic: Identification Link, unencoded (REQ-IL-005).
    pub app: *const c_char,
    /// The Source of the topic or NULL.
    pub source: *const c_char,
    /// The Filter of the topic or NULL.
    pub filter: *const c_char,
    /// DataSetWriterId of the DataSetMessage.
    pub data_set_writer_id: u16,
    /// The Payload of the DataSetMessage as JSON.
    pub payload_json: *const c_char,
}

/// Gets every received DataSetMessage (`msg` borrowed for the call).
pub type ReceivedFn = Option<unsafe extern "C" fn(user_data: *mut c_void, msg: *const Received)>;

type RawReceivedFn = unsafe extern "C" fn(*mut c_void, *const Received);

/// Passes `r` to the C callback `f`.
fn deliver(f: RawReceivedFn, user_data: UserData, r: &oec::consumer::Received) {
    let service_type = c_string(r.service_type().as_str());
    let app = c_string(r.app().as_str());
    let source = r.source().map(|s| c_string(s.as_str()));
    let filter = r.filter().map(|s| c_string(&s));
    let payload = c_string(&r.payload_json());
    let msg = Received {
        resource: types::resource_value(r.resource()),
        service_type: service_type.as_ptr(),
        app: app.as_ptr(),
        source: source.as_ref().map_or(ptr::null(), |s| s.as_ptr()),
        filter: filter.as_ref().map_or(ptr::null(), |s| s.as_ptr()),
        data_set_writer_id: r.data_set_writer_id(),
        payload_json: payload.as_ptr(),
    };
    // SAFETY: the caller's callback with its user_data (header contract);
    // the strings live until it returns.
    callback(|| unsafe { f(user_data.get(), &raw const msg) });
    drop((service_type, app, source, filter, payload));
}

/// The target of a request from the C arguments.
///
/// # Safety
///
/// Strings are NULL or NUL-terminated.
unsafe fn target(
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    filter: *const c_char,
) -> Res<Target> {
    let il = |p, what| -> Res<Option<IdentificationLink>> {
        // SAFETY: header contract.
        Ok(match unsafe { opt_text(p, what) }? {
            Some(s) => Some(IdentificationLink::parse(s)?),
            None => None,
        })
    };
    Ok(Target {
        service_type: types::service_type(service_type)?,
        app: il(app_il, "app_il")?,
        source: il(source_il, "source_il")?,
        // SAFETY: header contract.
        filter: unsafe { opt_text(filter, "filter") }?.map(str::to_owned),
    })
}

fn timeout(ms: u32) -> Option<Duration> {
    (ms != 0).then(|| Duration::from_millis(u64::from(ms)))
}

/// A subscription; owned by the caller.
pub struct Subscription(#[allow(dead_code)] oec::consumer::Subscription);

/// Subscribes to the publications of `resource` at
/// `Oi4v2/<ServiceType>/<AppId>/Pub/<Resource>[/<Source>[/<Filter>]]`:
/// `service_type` `OEC_ST_ANY`, `app_il`, `source_il` or `filter` NULL are
/// wildcards. Becomes a SubscriptionList entry (REQ-RES-SUBLIST-001). `f`
/// runs on the SDK thread for every DataSetMessage, also for the answers
/// to the `Get` of others; it may free the subscription.
///
/// # Safety
///
/// `app` is NULL or a live app; strings are NULL or NUL-terminated; `f`
/// and `user_data` are usable on the SDK thread until the subscription or
/// the app is freed; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_subscribe(
    app: *mut App,
    resource: Resource,
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    filter: *const c_char,
    f: ReceivedFn,
    user_data: *mut c_void,
    out: *mut *mut Subscription,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { crate::abi::out(out, "out") }?;
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        let resource = types::resource(resource)?;
        // SAFETY: header contract.
        let target = unsafe { target(service_type, app_il, source_il, filter) }?;
        let f = f.ok_or_else(|| Fail::invalid("fn is NULL"))?;
        let user_data = UserData::new(user_data);
        let sub = app
            .handle
            .consumer()
            .subscribe(resource, &target, move |r| deliver(f, user_data, r))?;
        *out = Box::into_raw(Box::new(Subscription(sub)));
        Ok(())
    })
}

/// Ends a subscription and releases it (also after the app is freed);
/// NULL is ignored. When it returns from another thread than the SDK
/// thread, the callback may still run once.
///
/// # Safety
///
/// `sub` is NULL or a live subscription, not used afterwards.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_subscription_free(sub: *mut Subscription) {
    if !sub.is_null() {
        let _ = guard(|| {
            // SAFETY: from oec_subscribe, released here (contract).
            drop(unsafe { Box::from_raw(sub) });
            Ok(())
        });
    }
}

/// Blocking `Get` with correlation and page reassembly (REQ-MSG-015,
/// REQ-PAG-002): `f` is called on the calling thread once per received
/// DataSetMessage before the function returns. `service_type` and
/// `app_il` are required; `timeout_ms` 0 = SDK default (10 s).
///
/// # Safety
///
/// `app` is NULL or a live app; strings are NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_get(
    app: *mut App,
    resource: Resource,
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    filter: *const c_char,
    timeout_ms: u32,
    f: ReceivedFn,
    user_data: *mut c_void,
) -> Status {
    guard(|| {
        outside_callback("oec_get")?;
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        let resource = types::resource(resource)?;
        // SAFETY: header contract.
        let target = unsafe { target(service_type, app_il, source_il, filter) }?;
        let f = f.ok_or_else(|| Fail::invalid("fn is NULL"))?;
        let mut options = GetOptions::new();
        if let Some(t) = timeout(timeout_ms) {
            options = options.timeout(t);
        }
        let received = app
            .handle
            .consumer()
            .get_blocking(resource, &target, &options)?;
        let user_data = UserData::new(user_data);
        for r in &received {
            deliver(f, user_data, r);
        }
        Ok(())
    })
}

/// Writes the answer of a `Set`/`Del`: `OEC_OK` for Good, else
/// `OEC_ERR_STATUS` with the code.
fn answer(code: oec::status::StatusCode, out: *mut StatusCode) -> Res<()> {
    let code = code.code();
    // SAFETY: NULL or writable (header contract).
    if let Some(out) = unsafe { out.as_mut() } {
        *out = code;
    }
    if code & 0xC000_0000 == 0 {
        Ok(())
    } else {
        Err(Fail::status(
            format!("the responder answered {code:#010x}"),
            code,
        ))
    }
}

/// Blocking `Set` of `resource` with `payload_json` (a JSON object) as the
/// Payload of its one DataSetMessage; waits for the Status event
/// (REQ-RES-EVENT-005). Its StatusCode goes to `*code` (may be NULL): Good
/// gives `OEC_OK`, Bad `OEC_ERR_STATUS` (REQ-CMP-047). `service_type` and
/// `app_il` are required; `timeout_ms` 0 = SDK default (10 s).
///
/// # Safety
///
/// `app` is NULL or a live app; strings are NULL or NUL-terminated;
/// `code` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_set(
    app: *mut App,
    resource: Resource,
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    filter: *const c_char,
    payload_json: *const c_char,
    timeout_ms: u32,
    code: *mut StatusCode,
) -> Status {
    guard(|| {
        outside_callback("oec_set")?;
        // SAFETY: header contract.
        let (app, payload) = unsafe { (handle(app, "app")?, text(payload_json, "payload_json")?) };
        let resource = types::resource(resource)?;
        // SAFETY: header contract.
        let target = unsafe { target(service_type, app_il, source_il, filter) }?;
        let mut options = WriteOptions::new();
        if let Some(t) = timeout(timeout_ms) {
            options = options.timeout(t);
        }
        let result = app
            .handle
            .consumer()
            .set_blocking(resource, &target, payload, &options)?;
        answer(result, code)
    })
}

/// Blocking `Del` of `resource`, with `payload_json` (NULL = none) as its
/// one DataSetMessage where the resource defines one; otherwise as
/// `oec_set`.
///
/// # Safety
///
/// As for `oec_set`; `payload_json` may be NULL.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_del(
    app: *mut App,
    resource: Resource,
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    filter: *const c_char,
    payload_json: *const c_char,
    timeout_ms: u32,
    code: *mut StatusCode,
) -> Status {
    guard(|| {
        outside_callback("oec_del")?;
        // SAFETY: header contract.
        let (app, payload) =
            unsafe { (handle(app, "app")?, opt_text(payload_json, "payload_json")?) };
        let resource = types::resource(resource)?;
        // SAFETY: header contract.
        let target = unsafe { target(service_type, app_il, source_il, filter) }?;
        let mut options = WriteOptions::new();
        if let Some(t) = timeout(timeout_ms) {
            options = options.timeout(t);
        }
        let result = app
            .handle
            .consumer()
            .del_blocking(resource, &target, payload, &options)?;
        answer(result, code)
    })
}

/// Blocking Action `NewDataSetWriterId` (MG 7.3, REQ-ACT-017): asks the
/// application `app_il` for a DataSetWriterId for a DataSet of `resource`
/// (`source_il` NULL = the application itself). On success `*id` is the
/// provided DataSetWriterId and `*ttl_seconds` the time its reservation
/// lasts; a Status event Bad gives `OEC_ERR_STATUS`. `service_type` and
/// `app_il` are required; `timeout_ms` 0 = SDK default (10 s).
///
/// # Safety
///
/// `app` is NULL or a live app; strings are NULL or NUL-terminated; `id`
/// and `ttl_seconds` are NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_new_data_set_writer_id(
    app: *mut App,
    resource: Resource,
    service_type: ServiceType,
    app_il: *const c_char,
    source_il: *const c_char,
    timeout_ms: u32,
    id: *mut u16,
    ttl_seconds: *mut u32,
) -> Status {
    guard(|| {
        outside_callback("oec_new_data_set_writer_id")?;
        // SAFETY: header contract.
        let (id, ttl) = unsafe {
            (
                id.as_mut().ok_or_else(|| Fail::invalid("id is NULL"))?,
                ttl_seconds
                    .as_mut()
                    .ok_or_else(|| Fail::invalid("ttl_seconds is NULL"))?,
            )
        };
        *id = 0;
        *ttl = 0;
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        let resource = types::resource(resource)?;
        // SAFETY: header contract.
        let target = unsafe { target(service_type, app_il, source_il, ptr::null()) }?;
        let mut options = ActionOptions::new();
        if let Some(t) = timeout(timeout_ms) {
            options = options.timeout(t);
        }
        let provided = app
            .handle
            .consumer()
            .new_data_set_writer_id_blocking(resource, &target, &options)?;
        *id = provided.data_set_writer_id;
        *ttl = u32::try_from(provided.ttl.as_secs()).unwrap_or(u32::MAX);
        Ok(())
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::abi::OEC_ERR_STATUS;
    use crate::types::{OEC_ST_ANY, OEC_ST_UTILITY};

    #[test]
    fn targets() {
        // SAFETY: NULL and C string literals.
        unsafe {
            let t = target(OEC_ST_ANY, ptr::null(), ptr::null(), ptr::null()).unwrap();
            assert!(t.service_type.is_none() && t.app.is_none() && t.filter.is_none());
            let t = target(
                OEC_ST_UTILITY,
                c"urn:example.com/M/C/S".as_ptr(),
                ptr::null(),
                c"Weather".as_ptr(),
            )
            .unwrap();
            assert_eq!(t.app.unwrap().as_str(), "urn:example.com/M/C/S");
            assert_eq!(t.filter.as_deref(), Some("Weather"));
            assert!(target(OEC_ST_ANY, c"no IL".as_ptr(), ptr::null(), ptr::null()).is_err());
            assert!(target(9, ptr::null(), ptr::null(), ptr::null()).is_err());
        }
        assert_eq!(timeout(0), None);
        assert_eq!(timeout(5), Some(Duration::from_millis(5)));
    }

    #[test]
    fn answers() {
        let mut code = 7;
        assert!(answer(oec::status::StatusCode::Good, &raw mut code).is_ok());
        assert_eq!(code, 0);
        let status = guard(|| answer(oec::status::StatusCode::BadNotFound, &raw mut code));
        assert_eq!(code, 0x803E_0000);
        assert_eq!(status, OEC_ERR_STATUS);
        assert_eq!(crate::abi::oec_last_status_code(), 0x803E_0000);
        assert!(answer(oec::status::StatusCode::Good, ptr::null_mut()).is_ok());
    }
}
