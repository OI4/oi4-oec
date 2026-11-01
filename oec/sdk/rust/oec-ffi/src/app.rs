//! Product, builder, app lifecycle, assets and events (ADR-0021 §3, §6,
//! §8; ADR-0020 §2 … §8).

// The ABI boundary: raw pointers and `extern "C"` functions (ADR-0026 §4).
#![allow(unsafe_code)]

use std::any::Any;
use std::ffi::{c_char, c_int};
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Condvar, Mutex, MutexGuard, PoisonError};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

use oec::app::{AppBuilder, AppHandle, Bootstrap, Broker, ConnectionState as State};
use oec::asset::AssetDef;
use oec::event::GenericEvent;
use oec::identity::IdentificationLink;
use oec::store::FileStore;

use crate::abi::{
    Fail, Handle, OEC_ERR_IO, OEC_ERR_PANIC, OEC_ERR_TIMEOUT, Res, Status, guard, handle, opt_text,
    out, outside_callback, plain, run, text,
};
use crate::types::{
    self, ConnectionState, Health, Level, OEC_STATE_CLOSED, OEC_STATE_CONNECTING, OEC_STATE_ONLINE,
    OEC_STATE_RECONNECTING, OEC_WAIT_FOREVER, ServiceType, SyslogLevel,
};

pub(crate) fn lock<T>(m: &Mutex<T>) -> MutexGuard<'_, T> {
    m.lock().unwrap_or_else(PoisonError::into_inner)
}

/// What an app shares with its child handles.
pub(crate) struct Shared {
    /// Set by a panic in the SDK: the app is unusable.
    pub(crate) poisoned: AtomicBool,
    /// The child handles; they live until the app is freed, so a handle of
    /// a removed child stays valid (its calls fail).
    children: Mutex<Vec<Arc<dyn Any + Send + Sync>>>,
}

impl Shared {
    /// Keeps `child` until the app is freed; its address is the handle.
    pub(crate) fn adopt<T: Any + Send + Sync>(&self, child: T) -> *mut T {
        let child = Arc::new(child);
        let p = Arc::as_ptr(&child).cast_mut();
        lock(&self.children).push(child);
        p
    }
}

// ---------------------------------------------------------------- product

/// Product description of the app or an asset = its MAM
/// (REQ-RES-MAM-001); owned by the caller.
pub struct Product(pub(crate) oec::identity::Product);

/// A product with the three type-level elements of the Oi4Identifier
/// (ManufacturerUri, Model, ProductCode); the other MAM fields have the
/// defaults of the Rust API (ADR-0020) until `oec_product_set`.
///
/// # Safety
///
/// Strings are NULL or NUL-terminated; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_product_new(
    manufacturer_uri: *const c_char,
    model: *const c_char,
    product_code: *const c_char,
    out: *mut *mut Product,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        // SAFETY: header contract.
        let (uri, model, code) = unsafe {
            (
                text(manufacturer_uri, "manufacturer_uri")?,
                text(model, "model")?,
                text(product_code, "product_code")?,
            )
        };
        let product = oec::identity::Product::new(uri, model, code);
        *out = Box::into_raw(Box::new(Product(product)));
        Ok(())
    })
}

/// Sets a MAM field by its name (MG 5): `Manufacturer`, `Description`
/// (en-US), `HardwareRevision`, `SoftwareRevision`, `DeviceRevision`,
/// `DeviceManual`, `DeviceClass`, `GlobalAssetId`. Another name is
/// `OEC_ERR_INVALID_ARGUMENT`; the values are checked when used.
///
/// # Safety
///
/// `p` is NULL or a live product; strings are NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_product_set(
    p: *mut Product,
    mam_field: *const c_char,
    value: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (p, field, value) = unsafe {
            (
                plain(p, "p")?,
                text(mam_field, "mam_field")?,
                text(value, "value")?,
            )
        };
        let product = p.0.clone();
        p.0 = match field {
            "Manufacturer" => product.manufacturer(value),
            "Description" => product.description(value),
            "HardwareRevision" => product.hardware_revision(value),
            "SoftwareRevision" => product.software_revision(value),
            "DeviceRevision" => product.device_revision(value),
            "DeviceManual" => product.device_manual(value),
            "DeviceClass" => product.device_class(value),
            "GlobalAssetId" => product.global_asset_id(value),
            other => return Err(Fail::invalid(format!("{other:?} is no settable MAM field"))),
        };
        Ok(())
    })
}

/// Releases a product; NULL is ignored. Builders and assets keep a copy.
///
/// # Safety
///
/// `p` is NULL or a live product, not used afterwards.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_product_free(p: *mut Product) {
    if !p.is_null() {
        // SAFETY: from oec_product_new (contract).
        drop(unsafe { Box::from_raw(p) });
    }
}

// ---------------------------------------------------------------- builder

/// The configuration of an app; owned by the caller until
/// `oec_builder_build` (or `oec_builder_free`).
pub struct Builder {
    inner: Option<AppBuilder>,
    broker: Option<Broker>,
    root: Option<PathBuf>,
}

impl Builder {
    fn with(&mut self, f: impl FnOnce(AppBuilder) -> AppBuilder) -> Res<()> {
        let b = self
            .inner
            .take()
            .ok_or_else(|| Fail::state("the builder is unusable after a panic"))?;
        self.inner = Some(f(b));
        Ok(())
    }

    fn broker(&mut self, f: impl FnOnce(Broker) -> Broker) -> Res<()> {
        let b = self
            .broker
            .take()
            .ok_or_else(|| Fail::state("no explicit broker: call oec_builder_broker first"))?;
        self.broker = Some(f(b));
        Ok(())
    }
}

/// A builder for an app of `level` and `service_type` (not `OEC_ST_ANY`)
/// described by `product` (copied).
///
/// # Safety
///
/// `product` is NULL or a live product; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_new(
    level: Level,
    service_type: ServiceType,
    product: *const Product,
    out: *mut *mut Builder,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        let level = types::level(level)?;
        let st = types::service_type(service_type)?
            .ok_or_else(|| Fail::invalid("an app needs a ServiceType, not OEC_ST_ANY"))?;
        // SAFETY: header contract.
        let product =
            unsafe { product.as_ref() }.ok_or_else(|| Fail::invalid("product is NULL"))?;
        let inner = oec::app::App::builder(level, st, product.0.clone());
        *out = Box::into_raw(Box::new(Builder {
            inner: Some(inner),
            broker: None,
            root: None,
        }));
        Ok(())
    })
}

/// Container storage contract (REQ-CNT-004, REQ-CNT-009, REQ-CNT-021)
/// below `root` ("/" in production). This is the default; it replaces an
/// explicit broker.
///
/// # Safety
///
/// `b` is NULL or a live builder; `root` is NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_container_storage(
    b: *mut Builder,
    root: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, root) = unsafe { (plain(b, "b")?, text(root, "root")?) };
        b.root = Some(PathBuf::from(root));
        b.broker = None;
        Ok(())
    })
}

/// Explicit broker instead of the container storage: TLS always, `port` 0
/// = 8883 (REQ-MQTT-005); `ca_pem_file` NULL = no CA file (the run fails
/// without one). Replaces the container storage and an earlier broker.
///
/// # Safety
///
/// `b` is NULL or a live builder; strings are NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_broker(
    b: *mut Builder,
    host: *const c_char,
    port: u16,
    ca_pem_file: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, host, ca) = unsafe {
            (
                plain(b, "b")?,
                text(host, "host")?,
                opt_text(ca_pem_file, "ca_pem_file")?,
            )
        };
        let mut broker = Broker::new(host);
        if port != 0 {
            broker = broker.port(port);
        }
        if let Some(ca) = ca {
            broker = broker.ca_pem(ca);
        }
        b.broker = Some(broker);
        b.root = None;
        Ok(())
    })
}

/// Username and password for the explicit broker (after
/// `oec_builder_broker`, else `OEC_ERR_INVALID_STATE`).
///
/// # Safety
///
/// `b` is NULL or a live builder; strings are NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_credentials(
    b: *mut Builder,
    user: *const c_char,
    password: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, user, password) = unsafe {
            (
                plain(b, "b")?,
                text(user, "user")?,
                text(password, "password")?,
            )
        };
        b.broker(|broker| broker.credentials(user, password))
    })
}

/// Client certificate and its key (PEM files) for the explicit broker
/// (after `oec_builder_broker`, else `OEC_ERR_INVALID_STATE`).
///
/// # Safety
///
/// `b` is NULL or a live builder; strings are NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_client_cert(
    b: *mut Builder,
    cert_pem_file: *const c_char,
    key_pem_file: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, cert, key) = unsafe {
            (
                plain(b, "b")?,
                text(cert_pem_file, "cert_pem_file")?,
                text(key_pem_file, "key_pem_file")?,
            )
        };
        b.broker(|broker| broker.client_cert(cert, key))
    })
}

/// The AppId instead of the derived Oi4Identifier (checked, REQ-IL-008).
///
/// # Safety
///
/// `b` is NULL or a live builder; `il` is NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_identification_link(
    b: *mut Builder,
    il: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, il) = unsafe { (plain(b, "b")?, text(il, "il")?) };
        let il = IdentificationLink::parse(il)?;
        b.with(|inner| inner.identification_link(il))
    })
}

/// The SerialNumber of the app (default: from the container storage).
///
/// # Safety
///
/// `b` is NULL or a live builder; `serial` is NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_serial_number(
    b: *mut Builder,
    serial: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, serial) = unsafe { (plain(b, "b")?, text(serial, "serial")?) };
        b.with(|inner| inner.serial_number(serial))
    })
}

/// A file store in `dir` for what the SDK persists (RevisionCounter,
/// DataSetWriterIds, Config values, …).
///
/// # Safety
///
/// `b` is NULL or a live builder; `dir` is NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_store_dir(b: *mut Builder, dir: *const c_char) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (b, dir) = unsafe { (plain(b, "b")?, text(dir, "dir")?) };
        b.with(|inner| inner.store(FileStore::new(dir)))
    })
}

/// The lowest Syslog level that is published (default
/// `OEC_SYSLOG_NOTICE`, REQ-TOP-024).
///
/// # Safety
///
/// `b` is NULL or a live builder.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_syslog_level(b: *mut Builder, lowest: SyslogLevel) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let b = unsafe { plain(b, "b") }?;
        let level = types::syslog_level(lowest)?;
        b.with(|inner| inner.syslog(level))
    })
}

/// Builds the app (checks level, identity and bootstrap). Consumes the
/// builder in every case, also on error.
///
/// # Safety
///
/// `b` is NULL or a live builder, not used afterwards; `out` is NULL or
/// writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_build(b: *mut Builder, out: *mut *mut App) -> Status {
    guard(|| {
        // SAFETY: from oec_builder_new, consumed here (contract).
        let b = (!b.is_null())
            .then(|| unsafe { Box::from_raw(b) })
            .ok_or_else(|| Fail::invalid("b is NULL"))?;
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        let Builder {
            inner,
            broker,
            root,
        } = *b;
        let mut inner =
            inner.ok_or_else(|| Fail::state("the builder is unusable after a panic"))?;
        if let Some(broker) = broker {
            inner = inner.bootstrap(Bootstrap::Explicit(broker));
        } else if let Some(root) = root {
            inner = inner.bootstrap(Bootstrap::ContainerStorage { root });
        }
        let app = inner.build()?;
        let shared = Arc::new(Shared {
            poisoned: AtomicBool::new(false),
            children: Mutex::new(Vec::new()),
        });
        let handle = app.handle();
        let this = Arc::new(Asset {
            asset: handle.this(),
            shared: Arc::clone(&shared),
        });
        *out = Box::into_raw(Box::new(App {
            shared,
            inner: Mutex::new(Some(app)),
            handle,
            this,
            thread: Mutex::new(None),
            ended: Arc::new(Ended::default()),
        }));
        Ok(())
    })
}

/// Releases a builder that is not built; NULL is ignored.
///
/// # Safety
///
/// `b` is NULL or a live builder, not used afterwards.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_builder_free(b: *mut Builder) {
    if !b.is_null() {
        // SAFETY: from oec_builder_new (contract).
        drop(unsafe { Box::from_raw(b) });
    }
}

// -------------------------------------------------------------------- app

/// An application; owned by the caller.
pub struct App {
    shared: Arc<Shared>,
    inner: Mutex<Option<oec::app::App>>,
    pub(crate) handle: AppHandle,
    this: Arc<Asset>,
    thread: Mutex<Option<JoinHandle<()>>>,
    ended: Arc<Ended>,
}

/// The result of the run of `oec_app_start`, `None` while it runs.
#[derive(Default)]
struct Ended {
    result: Mutex<Option<Res<()>>>,
    done: Condvar,
}

impl Ended {
    fn set(&self, result: Res<()>) {
        *lock(&self.result) = Some(result);
        self.done.notify_all();
    }
}

impl Handle for App {
    fn shared(&self) -> &Arc<Shared> {
        &self.shared
    }
}

impl App {
    fn take(&self) -> Res<oec::app::App> {
        lock(&self.inner)
            .take()
            .ok_or_else(|| Fail::state("the app runs or ran already"))
    }
}

/// Connects and blocks until `oec_app_shutdown` or SIGINT/SIGTERM: Birth,
/// Will, Health heartbeat, Close as in the Rust API (REQ-LIFE-001,
/// REQ-LIFE-005, REQ-LIFE-008, REQ-TOP-017). Once per app. Errors before
/// the first connection only (CA, client certificate, host); a lost
/// connection is re-established.
///
/// # Safety
///
/// `app` is NULL or a live app.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_run(app: *mut App) -> Status {
    guard(|| {
        outside_callback("oec_app_run")?;
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        Ok(app.take()?.run()?)
    })
}

/// Non-blocking alternative to `oec_app_run`: runs on an SDK thread until
/// `oec_app_shutdown` or `oec_app_free`; no signal handling. Errors
/// before the first connection end that thread; `oec_app_wait` reports
/// them.
///
/// # Safety
///
/// `app` is NULL or a live app.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_start(app: *mut App) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        let inner = app.take()?;
        let io = |e: std::io::Error| Fail::new(OEC_ERR_IO, format!("SDK thread: {e}"));
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_io()
            .enable_time()
            .build()
            .map_err(io)?;
        let ended = Arc::clone(&app.ended);
        let shared = Arc::clone(&app.shared);
        let thread = std::thread::Builder::new()
            .name("oec-app".to_owned())
            .spawn(move || {
                let result = catch_unwind(AssertUnwindSafe(|| rt.block_on(inner.run_async())));
                let result = if let Ok(r) = result {
                    r.map_err(Fail::from)
                } else {
                    shared.poisoned.store(true, Ordering::SeqCst);
                    Err(Fail::new(
                        OEC_ERR_PANIC,
                        "panic on the SDK thread; the app is unusable",
                    ))
                };
                ended.set(result);
            })
            .map_err(io)?;
        *lock(&app.thread) = Some(thread);
        Ok(())
    })
}

/// Sends Close and stops the app (REQ-LIFE-005); idempotent. Before a run
/// it makes the run return at once. NULL is ignored.
///
/// # Safety
///
/// `app` is NULL or a live app.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_shutdown(app: *mut App) {
    // SAFETY: NULL or a live app (contract).
    if let Some(app) = unsafe { app.as_ref() } {
        app.handle.shutdown();
    }
}

/// Shuts the app down, waits for its SDK thread and releases it with all
/// its child handles; NULL is ignored. Refused inside a callback (the
/// error is set, nothing is released).
///
/// # Safety
///
/// `app` is NULL or a live app, not used concurrently or afterwards.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_free(app: *mut App) {
    if app.is_null() {
        return;
    }
    let _ = guard(|| {
        outside_callback("oec_app_free")?;
        // SAFETY: from oec_builder_build, released here (contract).
        let app = unsafe { Box::from_raw(app) };
        app.handle.shutdown();
        if let Some(thread) = lock(&app.thread).take() {
            let _ = thread.join();
        }
        lock(&app.shared.children).clear();
        Ok(())
    });
}

/// The connection state of `app` (`OEC_STATE_*`) into `*state`; also
/// inside a callback.
///
/// # Safety
///
/// `app` is NULL or a live app; `state` is NULL or valid for writes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_state(app: *const App, state: *mut ConnectionState) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        // SAFETY: NULL or writable (contract).
        let state = unsafe { state.as_mut() }.ok_or_else(|| Fail::invalid("state is NULL"))?;
        *state = match app.handle.state() {
            State::Connecting => OEC_STATE_CONNECTING,
            State::Online => OEC_STATE_ONLINE,
            State::Reconnecting => OEC_STATE_RECONNECTING,
            State::Closed => OEC_STATE_CLOSED,
        };
        Ok(())
    })
}

/// Waits up to `timeout_ms` (0 = do not wait, `OEC_WAIT_FOREVER` = no
/// limit) until the run of `oec_app_start` has ended and returns its
/// result: `OEC_OK` after `oec_app_shutdown`, the start error otherwise
/// (`OEC_ERR_BOOTSTRAP`, `OEC_ERR_INVALID_ARGUMENT`, `OEC_ERR_IO`, as for
/// `oec_app_run`), every time it is asked. `OEC_ERR_TIMEOUT` while it
/// still runs; `OEC_ERR_INVALID_STATE` without `oec_app_start` and inside
/// a callback.
///
/// # Safety
///
/// `app` is NULL or a live app.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_wait(app: *mut App, timeout_ms: u32) -> Status {
    guard(|| {
        outside_callback("oec_app_wait")?;
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        if lock(&app.thread).is_none() {
            return Err(Fail::state("oec_app_wait without oec_app_start"));
        }
        let ended = &app.ended;
        let deadline = (timeout_ms != OEC_WAIT_FOREVER)
            .then(|| Instant::now() + Duration::from_millis(timeout_ms.into()));
        let mut result = lock(&ended.result);
        loop {
            if let Some(r) = result.as_ref() {
                return r.clone();
            }
            result = match deadline {
                None => ended
                    .done
                    .wait(result)
                    .unwrap_or_else(PoisonError::into_inner),
                Some(d) => {
                    let left = d.saturating_duration_since(Instant::now());
                    if left.is_zero() {
                        return Err(Fail::new(
                            OEC_ERR_TIMEOUT,
                            format!("the app still runs after {timeout_ms} ms"),
                        ));
                    }
                    ended
                        .done
                        .wait_timeout(result, left)
                        .unwrap_or_else(PoisonError::into_inner)
                        .0
                }
            };
        }
    })
}

// ------------------------------------------------------------------ assets

/// The app itself or a represented asset; owned by the app.
pub struct Asset {
    pub(crate) asset: oec::asset::Asset,
    pub(crate) shared: Arc<Shared>,
}

impl Handle for Asset {
    fn shared(&self) -> &Arc<Shared> {
        &self.shared
    }
}

/// The app itself as asset; never NULL for a live app.
///
/// # Safety
///
/// `app` is NULL or a live app.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_this(app: *mut App) -> *mut Asset {
    run(|| {
        // SAFETY: header contract.
        let app = unsafe { handle(app, "app") }?;
        Ok(Arc::as_ptr(&app.this).cast_mut())
    })
    .unwrap_or(std::ptr::null_mut())
}

/// Adds a represented asset (REQ-CMP-045): its MAM and Health are
/// published and served. `il` NULL = the Oi4Identifier of `product` and
/// `serial_number`.
///
/// # Safety
///
/// `app` and `product` are NULL or live handles; strings are NULL or
/// NUL-terminated; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_add_asset(
    app: *mut App,
    il: *const c_char,
    product: *const Product,
    serial_number: *const c_char,
    out: *mut *mut Asset,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        // SAFETY: header contract.
        let (app, il, serial) = unsafe {
            (
                handle(app, "app")?,
                opt_text(il, "il")?,
                text(serial_number, "serial_number")?,
            )
        };
        // SAFETY: header contract.
        let product =
            unsafe { product.as_ref() }.ok_or_else(|| Fail::invalid("product is NULL"))?;
        let def = match il {
            Some(il) => AssetDef::new(IdentificationLink::parse(il)?, product.0.clone(), serial),
            None => AssetDef::with_oi4_identifier(product.0.clone(), serial)?,
        };
        let asset = app.handle.add_asset(def)?;
        *out = app.shared.adopt(Asset {
            asset,
            shared: Arc::clone(&app.shared),
        });
        Ok(())
    })
}

/// Removes a represented asset with its DataSets (REQ-RES-PUBLIST-017);
/// the handle stays valid, its calls fail. Not for the app itself.
///
/// # Safety
///
/// `asset` is NULL or a live asset handle.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_asset_remove(asset: *mut Asset) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let asset = unsafe { handle(asset, "asset") }?;
        Ok(asset.asset.clone().remove()?)
    })
}

/// Sets Health and HealthScore (0 … 100, `score` < 0: none); a change is
/// published at once (REQ-TOP-017).
///
/// # Safety
///
/// `asset` is NULL or a live asset handle.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_asset_set_health(
    asset: *mut Asset,
    health: Health,
    score: c_int,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let asset = unsafe { handle(asset, "asset") }?;
        let health = types::health(health)?;
        let score = match score {
            s if s < 0 => None,
            s => Some(
                u8::try_from(s)
                    .ok()
                    .filter(|s| *s <= 100)
                    .ok_or_else(|| Fail::invalid(format!("HealthScore {s} > 100")))?,
            ),
        };
        asset.asset.set_health(health, score);
        Ok(())
    })
}

// ------------------------------------------------------------------ events

/// Publishes a Syslog event of the app (REQ-TOP-023) if `level` is
/// switched on (default: `OEC_SYSLOG_NOTICE` and more severe,
/// REQ-TOP-024); offline it is dropped.
///
/// # Safety
///
/// `app` is NULL or a live app; `message` is NULL or NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_app_syslog(
    app: *mut App,
    level: SyslogLevel,
    message: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (app, message) = unsafe { (handle(app, "app")?, text(message, "message")?) };
        app.handle.emit_syslog(types::syslog_level(level)?, message);
        Ok(())
    })
}

/// Publishes a Generic event of the asset on
/// `Pub/Event/<Source>/Generic/<level>` (REQ-RES-EVENT-009); `level` is a
/// free topic level (e.g. "Maintenance", at most 16 per asset),
/// `description` and `details_json` (a JSON object) may be NULL. Offline
/// it is dropped.
///
/// # Safety
///
/// `asset` is NULL or a live asset handle; strings are NULL or
/// NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_asset_emit_event(
    asset: *mut Asset,
    level: *const c_char,
    number: u32,
    description: *const c_char,
    details_json: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (asset, level, description, details) = unsafe {
            (
                handle(asset, "asset")?,
                text(level, "level")?,
                opt_text(description, "description")?,
                opt_text(details_json, "details_json")?,
            )
        };
        let mut event = GenericEvent::new(level, number);
        if let Some(d) = description {
            event = event.description(d);
        }
        if let Some(d) = details {
            event = event.details_json(d);
        }
        Ok(asset.asset.emit_event(event)?)
    })
}
