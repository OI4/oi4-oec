//! Config (MG 9.1.3, MG 8.3.3): groups of configuration objects.
//!
//! A Config maps group names to [`Group`]s next to the optional `Context`
//! (REQ-RES-CONFIG-002); a Set sends only the changed values in the
//! [`ConfigSet`] form (REQ-RES-CONFIG-003). The names of groups and objects
//! follow REQ-RES-CONFIG-004; values are strings (REQ-RES-CONFIG-007), a
//! sensitive value is masked (REQ-RES-CONFIG-024).

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};
use serde_json::Number;

use crate::message::LocalizedText;

/// The mask of a sensitive value (REQ-RES-CONFIG-024).
pub const SENSITIVE_MASK: &str = "********";

/// Context of a Config (REQ-RES-CONFIG-002).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Context {
    /// Name.
    pub name: LocalizedText,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
}

/// The Config of a Pub or Get response (REQ-RES-CONFIG-002).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct Config {
    /// Context.
    #[serde(rename = "Context", default, skip_serializing_if = "Option::is_none")]
    pub context: Option<Context>,
    /// The groups by name.
    #[serde(flatten)]
    pub groups: BTreeMap<String, Group>,
}

/// A group of configuration objects (REQ-RES-CONFIG-002).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Group {
    /// Name.
    #[serde(rename = "Name")]
    pub name: LocalizedText,
    /// Description.
    #[serde(
        rename = "Description",
        default,
        skip_serializing_if = "Option::is_none"
    )]
    pub description: Option<LocalizedText>,
    /// The configuration objects by name.
    #[serde(flatten)]
    pub objects: BTreeMap<String, ConfigObject>,
}

/// Type of a configuration value (REQ-RES-CONFIG-006).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[allow(missing_docs)]
pub enum ConfigType {
    Boolean,
    ByteString,
    DateTime,
    Number,
    String,
}

/// Validation of a configuration value (REQ-RES-CONFIG-002,
/// REQ-RES-CONFIG-003).
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Validation {
    /// Maximum length.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub length: Option<u32>,
    /// Minimum.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min: Option<Number>,
    /// Maximum.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max: Option<Number>,
    /// Regular expression.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pattern: Option<String>,
    /// Allowed values.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub values: Option<Vec<String>>,
}

/// A configuration object of a Pub (REQ-RES-CONFIG-002): Type, Value and
/// Name are mandatory.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ConfigObject {
    /// Type (REQ-RES-CONFIG-006).
    #[serde(rename = "Type")]
    pub kind: ConfigType,
    /// The value as string (REQ-RES-CONFIG-007).
    pub value: String,
    /// The default value as string.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_value: Option<String>,
    /// Unit.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    /// Mandatory.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mandatory: Option<bool>,
    /// Sensitive: the values are masked (REQ-RES-CONFIG-024).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sensitive: Option<bool>,
    /// Name.
    pub name: LocalizedText,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
    /// Validation.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub validation: Option<Validation>,
}

super::data_set!(Config, "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2");

/// The Config of a Set: only the changed values (REQ-RES-CONFIG-003).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct ConfigSet {
    /// Context.
    #[serde(rename = "Context", default, skip_serializing_if = "Option::is_none")]
    pub context: Option<Context>,
    /// The groups by name.
    #[serde(flatten)]
    pub groups: BTreeMap<String, GroupSet>,
}

/// A group of a Set: Name is optional (REQ-RES-CONFIG-003).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct GroupSet {
    /// Name.
    #[serde(rename = "Name", default, skip_serializing_if = "Option::is_none")]
    pub name: Option<LocalizedText>,
    /// Description.
    #[serde(
        rename = "Description",
        default,
        skip_serializing_if = "Option::is_none"
    )]
    pub description: Option<LocalizedText>,
    /// The changed configuration objects by name.
    #[serde(flatten)]
    pub objects: BTreeMap<String, ConfigObjectSet>,
}

/// A configuration object of a Set: only Value is mandatory
/// (REQ-RES-CONFIG-003).
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ConfigObjectSet {
    /// Type (REQ-RES-CONFIG-006).
    #[serde(rename = "Type", default, skip_serializing_if = "Option::is_none")]
    pub kind: Option<ConfigType>,
    /// The new value as string (REQ-RES-CONFIG-007).
    pub value: String,
    /// The default value as string.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_value: Option<String>,
    /// Unit.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    /// Mandatory.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mandatory: Option<bool>,
    /// Sensitive.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sensitive: Option<bool>,
    /// Name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<LocalizedText>,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
    /// Validation.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub validation: Option<Validation>,
}

super::data_set!(ConfigSet, "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2");
