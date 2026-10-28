//! Events (MG 8.3.6): Status events are automatic, Syslog events come from
//! the `log` bridge or [`syslog`], Generic events from the application
//! ([`crate::asset::Asset::emit_event`], P4 increment 7c, ADR-0020
//! amendment C-057).
//!
//! # The Syslog bridge
//!
//! [`crate::app::AppBuilder::build`] installs a [`log::Log`] once per
//! process if no logger is installed yet: every record of the application
//! then becomes a Syslog event of the application itself on
//! `Pub/Event/<AppId>/Syslog/<Level>` (REQ-RES-EVENT-007/008), with
//! `Error` → `Error`, `Warn` → `Warning`, `Info` → `Informational` and
//! `Debug`/`Trace` → `Debug`. Which levels are published is set by
//! [`crate::app::AppBuilder::syslog`] (default `Notice`, so `Debug` and
//! `Informational` are off, REQ-TOP-024) and, from Level 2, by
//! `Set PublicationList` per level (REQ-TOP-027); the bridge follows it
//! with [`log::set_max_level`]. Without a local logger the records are not
//! printed; to keep one, pass it to [`install_bridge`] before `build`.
//!
//! **Application logs become messages on the Message Bus:** never log
//! secrets (GL-ADR-021). The records of the SDK itself and of its
//! transport (targets `oec`, `oec_core`, `rustls`, `tokio`, `mio`) are not
//! published, and a record logged while the bridge publishes is dropped,
//! so the bridge cannot feed itself. At most 50 records at once and 10 per
//! second on average are published; the number of dropped records follows
//! as a `Warning`. The `MSG` is cut to [`MAX_MESSAGE_BYTES`].
//!
//! If another logger is installed first, the bridge is inactive and only
//! [`syslog`] publishes. With several [`crate::app::App`]s in one process
//! the first one alive receives the records.

use std::cell::Cell;
use std::sync::{Mutex, OnceLock, PoisonError};

use log::{LevelFilter, Log, Metadata, Record};

use crate::app::{AppHandle, WeakHandle};
use crate::{Error, Result};

/// Syslog severities (RFC 5424) used as `<Level>` of
/// `Event/<Source>/Syslog/<Level>` (REQ-TOP-023), ordered from the most to
/// the least severe (`Emergency < Debug`).
pub use oec_core::topic::SyslogLevel;

/// The longest content of a Syslog `MSG` in bytes; longer records are cut
/// at a character boundary.
pub const MAX_MESSAGE_BYTES: usize = 1024;

/// A Generic event (`CAT_GENERIC_99`, MG 8.3.6.3, REQ-RES-EVENT-009),
/// published on `Pub/Event/<Source>/Generic/<Level>` by
/// [`crate::asset::Asset::emit_event`]. Number, Level, Description and
/// Details are defined by the application, which documents them
/// (REQ-RES-EVENT-010).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GenericEvent {
    pub(crate) level: String,
    pub(crate) number: u32,
    pub(crate) description: Option<String>,
    pub(crate) details_json: Option<String>,
}

impl GenericEvent {
    /// An event with its `<Level>` (a free topic level, encoded like a
    /// Filter, e.g. `Maintenance`) and its `Number`. Each Level of a
    /// source is one PublicationList entry (Mode `FILTER_4`), at most 16
    /// per source.
    pub fn new(level: &str, number: u32) -> Self {
        Self {
            level: level.to_owned(),
            number,
            description: None,
            details_json: None,
        }
    }

    /// The `Description`.
    #[must_use]
    pub fn description(mut self, text: &str) -> Self {
        self.description = Some(text.to_owned());
        self
    }

    /// The `Details`: a JSON object (checked by `emit_event`).
    #[must_use]
    pub fn details_json(mut self, json: &str) -> Self {
        self.details_json = Some(json.to_owned());
        self
    }

    /// The `<Level>`.
    pub fn level(&self) -> &str {
        &self.level
    }

    /// The `Number`.
    pub fn number(&self) -> u32 {
        self.number
    }

    /// The `<Level>` and the core event; [`Error::Invalid`] if the Details
    /// are no JSON object.
    pub(crate) fn into_core(self) -> Result<(String, oec_core::provider::GenericEvent)> {
        let details = match &self.details_json {
            None => None,
            Some(json) => match serde_json::from_str(json) {
                Ok(serde_json::Value::Object(map)) => Some(map),
                _ => {
                    return Err(Error::Invalid(format!(
                        "the Details of the Generic event {} are no JSON object \
                         (REQ-RES-EVENT-009)",
                        self.number
                    )));
                }
            },
        };
        Ok((
            self.level,
            oec_core::provider::GenericEvent {
                number: self.number,
                description: self.description,
                details,
            },
        ))
    }
}

/// Publishes a Syslog event of the application directly, as the bridge
/// does for a `log` record; with the same rules (level switched on, rate
/// limit, [`MAX_MESSAGE_BYTES`]). Dropped without an application, while
/// it is offline or if the level is off.
pub fn syslog(level: SyslogLevel, message: &str) {
    let Some(_guard) = Guard::enter() else {
        return;
    };
    if let Some(handle) = target() {
        handle.queue_syslog(level, message);
    }
}

/// [`syslog`] for one application, e.g. of several in one process (the C
/// ABI, ADR-0021); the same rules.
pub(crate) fn syslog_of(handle: &AppHandle, level: SyslogLevel, message: &str) {
    let Some(_guard) = Guard::enter() else {
        return;
    };
    handle.queue_syslog(level, message);
}

/// Installs the bridge as the logger of the process, before
/// [`crate::app::AppBuilder::build`] (which installs it without `local`
/// otherwise). Every record is passed to `local` too, e.g. a logger that
/// prints; the maximum level is then [`LevelFilter::Trace`] and `local`
/// filters itself.
///
/// # Errors
///
/// [`Error::Invalid`] if a logger (or the bridge) is installed already.
pub fn install_bridge(local: Option<Box<dyn Log>>) -> Result<()> {
    let mut installed = false;
    let bridge = BRIDGE.get_or_init(|| {
        installed = true;
        Bridge { local }
    });
    if !installed {
        return Err(Error::Invalid(
            "the Syslog bridge is installed already".to_owned(),
        ));
    }
    if log::set_logger(bridge).is_err() {
        ACTIVE.get_or_init(|| false);
        return Err(Error::Invalid(
            "another logger is installed; the Syslog bridge is inactive".to_owned(),
        ));
    }
    ACTIVE.get_or_init(|| true);
    sync_max_level();
    Ok(())
}

/// Called by `build`: installs the bridge unless a logger is installed,
/// and makes `handle` the target unless a live application is.
pub(crate) fn register(handle: &AppHandle) {
    if ACTIVE.get().is_none() {
        let _ = install_bridge(None);
    }
    {
        let mut target = TARGET.lock().unwrap_or_else(PoisonError::into_inner);
        if target.as_ref().and_then(WeakHandle::upgrade).is_none() {
            *target = Some(handle.downgrade());
        }
    }
    sync_max_level();
}

/// Sets [`log::set_max_level`] to the most verbose Syslog level published
/// now, if the bridge is the logger without a local logger.
pub(crate) fn sync_max_level() {
    if ACTIVE.get() != Some(&true) {
        return;
    }
    if BRIDGE.get().is_some_and(|b| b.local.is_some()) {
        log::set_max_level(LevelFilter::Trace);
        return;
    }
    let level = target().and_then(|h| h.syslog_max_level());
    log::set_max_level(max_level(level));
}

/// The `log` filter that lets the records of `level` and above through.
fn max_level(level: Option<SyslogLevel>) -> LevelFilter {
    match level {
        None => LevelFilter::Off,
        Some(SyslogLevel::Debug) => LevelFilter::Trace,
        Some(SyslogLevel::Informational) => LevelFilter::Info,
        Some(SyslogLevel::Notice | SyslogLevel::Warning) => LevelFilter::Warn,
        Some(_) => LevelFilter::Error,
    }
}

/// The Syslog level of a `log` level.
pub(crate) fn syslog_level(level: log::Level) -> SyslogLevel {
    match level {
        log::Level::Error => SyslogLevel::Error,
        log::Level::Warn => SyslogLevel::Warning,
        log::Level::Info => SyslogLevel::Informational,
        log::Level::Debug | log::Level::Trace => SyslogLevel::Debug,
    }
}

/// Whether the records of `target` are never published: the SDK and its
/// transport.
pub(crate) fn excluded(target: &str) -> bool {
    ["oec", "oec_core", "rustls", "tokio", "mio"]
        .iter()
        .any(|p| target == *p || target.strip_prefix(p).is_some_and(|r| r.starts_with("::")))
}

/// `content` cut to at most `max` bytes at a character boundary.
pub(crate) fn truncate(content: &str, max: usize) -> &str {
    if content.len() <= max {
        return content;
    }
    let mut end = max;
    while !content.is_char_boundary(end) {
        end -= 1;
    }
    &content[..end]
}

static BRIDGE: OnceLock<Bridge> = OnceLock::new();
/// Whether the bridge is the logger of the process (unset: not tried).
static ACTIVE: OnceLock<bool> = OnceLock::new();
static TARGET: Mutex<Option<WeakHandle>> = Mutex::new(None);

thread_local! {
    static PUBLISHING: Cell<bool> = const { Cell::new(false) };
}

fn target() -> Option<AppHandle> {
    TARGET
        .lock()
        .unwrap_or_else(PoisonError::into_inner)
        .as_ref()
        .and_then(WeakHandle::upgrade)
}

/// Marks the thread as publishing; `None` if it is already.
struct Guard;

impl Guard {
    fn enter() -> Option<Self> {
        PUBLISHING.with(|p| (!p.replace(true)).then_some(Self))
    }
}

impl Drop for Guard {
    fn drop(&mut self) {
        PUBLISHING.with(|p| p.set(false));
    }
}

struct Bridge {
    local: Option<Box<dyn Log>>,
}

impl Log for Bridge {
    fn enabled(&self, metadata: &Metadata<'_>) -> bool {
        self.local.as_ref().is_some_and(|l| l.enabled(metadata))
            || (!excluded(metadata.target())
                && target().is_some_and(|h| h.syslog_enabled(syslog_level(metadata.level()))))
    }

    fn log(&self, record: &Record<'_>) {
        if let Some(local) = &self.local
            && local.enabled(record.metadata())
        {
            local.log(record);
        }
        if excluded(record.target()) {
            return;
        }
        let Some(_guard) = Guard::enter() else {
            return;
        };
        if let Some(handle) = target() {
            handle.queue_syslog(syslog_level(record.level()), &record.args().to_string());
        }
    }

    fn flush(&self) {
        if let Some(local) = &self.local {
            local.flush();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn levels() {
        assert_eq!(syslog_level(log::Level::Error), SyslogLevel::Error);
        assert_eq!(syslog_level(log::Level::Warn), SyslogLevel::Warning);
        assert_eq!(syslog_level(log::Level::Info), SyslogLevel::Informational);
        assert_eq!(syslog_level(log::Level::Debug), SyslogLevel::Debug);
        assert_eq!(syslog_level(log::Level::Trace), SyslogLevel::Debug);
        assert_eq!(max_level(None), LevelFilter::Off);
        assert_eq!(max_level(Some(SyslogLevel::Debug)), LevelFilter::Trace);
        assert_eq!(
            max_level(Some(SyslogLevel::Informational)),
            LevelFilter::Info
        );
        assert_eq!(max_level(Some(SyslogLevel::Notice)), LevelFilter::Warn);
        assert_eq!(max_level(Some(SyslogLevel::Warning)), LevelFilter::Warn);
        assert_eq!(max_level(Some(SyslogLevel::Error)), LevelFilter::Error);
        assert_eq!(max_level(Some(SyslogLevel::Emergency)), LevelFilter::Error);
    }

    #[test]
    fn exclusions() {
        for t in [
            "oec",
            "oec::runtime",
            "oec_core::provider",
            "rustls::conn",
            "tokio",
            "mio::poll",
        ] {
            assert!(excluded(t), "{t}");
        }
        for t in [
            "oecx",
            "my_app",
            "oec_weather::main",
            "tokio_util2x",
            "app::oec",
        ] {
            assert!(!excluded(t), "{t}");
        }
    }

    #[test]
    fn truncation() {
        assert_eq!(truncate("abc", 3), "abc");
        assert_eq!(truncate("abcd", 3), "abc");
        assert_eq!(truncate("aé", 2), "a");
        assert_eq!(truncate("€€", 4), "€");
    }

    #[test]
    fn generic_event_details() {
        let e = GenericEvent::new("Maintenance", 7)
            .description("filter clogged")
            .details_json(r#"{"Pressure": 3}"#);
        let (level, core) = e.into_core().unwrap();
        assert_eq!(level, "Maintenance");
        assert_eq!(core.number, 7);
        assert_eq!(core.description.as_deref(), Some("filter clogged"));
        assert_eq!(core.details.unwrap()["Pressure"], 3);
        assert!(
            GenericEvent::new("l", 1)
                .details_json("[1]")
                .into_core()
                .is_err()
        );
        assert!(
            GenericEvent::new("l", 1)
                .details_json("{")
                .into_core()
                .is_err()
        );
        assert!(
            GenericEvent::new("l", 1)
                .into_core()
                .unwrap()
                .1
                .details
                .is_none()
        );
    }

    #[test]
    fn guard_is_not_reentrant() {
        let g = Guard::enter();
        assert!(g.is_some());
        assert!(Guard::enter().is_none());
        drop(g);
        assert!(Guard::enter().is_some());
    }
}
