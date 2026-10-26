//! Events (MG 8.3.6): Status events are automatic, Syslog events come from a
//! `log`/`tracing` bridge, Generic events from the application.

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
    pub number: u32,
    pub description: Option<String>,
    /// Free JSON object (`Details`).
    pub details_json: Option<String>,
}

/// Publishes a Syslog event of the application directly (normally the
/// `log`/`tracing` bridge does this, e.g. `log::warn!(...)`). Each level is
/// one PublicationList entry; Debug and Informational are off by default
/// (REQ-TOP-024, REQ-TOP-027, REQ-RES-EVENT-007/008).
pub fn syslog(level: SyslogLevel, message: &str) {
    unimplemented!()
}
