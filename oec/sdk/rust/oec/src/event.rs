//! Events (MG 8.3.6): Status events are automatic, Syslog events come from a
//! `log`/`tracing` bridge, Generic events from the application. In P4
//! increment 4 only the Syslog level exists (used by
//! [`crate::app::AppBuilder::syslog`]); publication follows with
//! increments 5 and 8.

/// Syslog severities (RFC 5424) used as `<Level>` of
/// `Event/<Source>/Syslog/<Level>` (REQ-TOP-023), ordered from the most to
/// the least severe (`Emergency < Debug`).
pub use oec_core::topic::SyslogLevel;
