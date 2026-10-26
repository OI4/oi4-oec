//! Event (MG 9.1.6): Syslog, Status and Generic events.

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

/// Event category in the VerboseEncoding (REQ-RES-EVENT-001); it matches
/// the first level of the WriterGroupName (REQ-RES-EVENT-002).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum EventCategory {
    /// `CAT_SYSLOG_0` (REQ-RES-EVENT-007, REQ-RES-EVENT-008).
    #[serde(rename = "CAT_SYSLOG_0")]
    Syslog,
    /// `CAT_STATUS_1` (REQ-RES-EVENT-003, REQ-RES-EVENT-004).
    #[serde(rename = "CAT_STATUS_1")]
    Status,
    /// `CAT_GENERIC_99`.
    #[serde(rename = "CAT_GENERIC_99")]
    Generic,
}

/// An Event (REQ-RES-EVENT-001); its DataSetMessages have the MessageType
/// `ua-event` (REQ-MSG-029).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Event {
    /// Number: the syslog PRI, the StatusCode or a generic number.
    pub number: u32,
    /// Category.
    pub category: EventCategory,
    /// Description; a Syslog event has none (REQ-RES-EVENT-007).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Details: `MSG`/`HEADER` of a Syslog event, `SymbolicId` of a Status
    /// event (REQ-RES-EVENT-003, REQ-RES-EVENT-007).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub details: Option<Map<String, Value>>,
}

super::data_set!(Event, "543ae05e-b6d9-4161-a0a3-350a0fac5976", Event);
