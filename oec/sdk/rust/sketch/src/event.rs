//! Events (MG 8.3.6): Status events are automatic, Syslog events come from a
//! `log` bridge, Generic events from the application (crate: ADR-0020
//! amendment C-057; `tracing` users bridge via `tracing-log`). The crate
//! adds `install_bridge(local)` to keep a local logger next to the bridge
//! (not sketched: the sketch has no dependencies).

/// Syslog severities (RFC 5424) used as `<Level>` of `Event/<Source>/Syslog/<Level>`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum SyslogLevel {
    Emergency,
    Alert,
    Critical,
    Error,
    Warning,
    Notice,
    Informational,
    Debug,
}

/// A Generic event (`CAT_GENERIC_99`, REQ-RES-EVENT-009); numbers and
/// details should be documented by the application (REQ-RES-EVENT-010).
#[derive(Debug, Clone)]
pub struct GenericEvent {
    _private: (),
}

impl GenericEvent {
    /// An event with its `<Level>` (a free topic level) and `Number`.
    pub fn new(level: &str, number: u32) -> Self {
        unimplemented!()
    }

    pub fn description(self, text: &str) -> Self {
        unimplemented!()
    }

    /// Free JSON object (`Details`), checked by `emit_event`.
    pub fn details_json(self, json: &str) -> Self {
        unimplemented!()
    }
}

/// Publishes a Syslog event of the application directly (normally the
/// `log`/`tracing` bridge does this, e.g. `log::warn!(...)`). Each level is
/// one PublicationList entry; Debug and Informational are off by default
/// (REQ-TOP-024, REQ-TOP-027, REQ-RES-EVENT-007/008).
pub fn syslog(level: SyslogLevel, message: &str) {
    unimplemented!()
}
