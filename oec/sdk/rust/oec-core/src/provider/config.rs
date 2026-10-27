//! Config DataSets of the provider (MG 8.3.3, MG 9.1.3; P4 increment 7b,
//! ADR-0026 §15): the check of a definition, the checks of a value (Type
//! and Validation, REQ-RES-CONFIG-007/008), the masking of sensitive
//! values (REQ-RES-CONFIG-024, GL-ADR-021), the check of a `Set Config`
//! (REQ-RES-CONFIG-003/005/011/023/025) and the table of the values set,
//! persisted so they survive a restart.
//!
//! Values are held in clear text inside the provider and the table; they
//! are masked in every publication and never written to `Debug` output.
//!
//! # `Set Config`
//!
//! A `Set Config` carries the changed objects of a single Config DataSet
//! (REQ-RES-CONFIG-003); the provider checks it against the current Config
//! and answers the first violation, in the order of the JSON members, with
//! a Status event (REQ-RES-CONFIG-023):
//!
//! | Violation | StatusCode |
//! |---|---|
//! | no Filter, unknown context, group or object | `BadNotFound` |
//! | not exactly one DataSetMessage, `Value` missing, a mandatory object of a group sent missing, no object at all | `BadInvalidArgument` |
//! | a Payload, group or object that is no JSON object | `BadDecodingError` |
//! | `Value` no string, `Type` other than defined, `Value` not of its Type | `BadTypeMismatch` |
//! | another property (Name, Description, Unit, DefaultValue, Mandatory, Sensitive, Validation, Context) other than defined | `BadNotWritable` |
//! | `Value` outside its Validation, Config beyond the MaxPacketSize | `BadOutOfRange` |
//!
//! An empty `Value` unsets the object (MG 9.1.3); the mask `********` as
//! `Value` of a sensitive object keeps its value (REQ-RES-CONFIG-025).
//! Unknown members of an object are ignored. Otherwise the application
//! decides ([`super::Handled::SetConfig`]).

use core::fmt;
use core::str::FromStr;
use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value as Json};

use super::sets::date_time;
use super::write::WriteWire;
use super::{Provider, ProviderError, WriterKey};
use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{
    ByteString, DataSetMessage, DataSetWriterId, DateTime, LocalizedText, MessageId,
};
use crate::resource::{Config, ConfigObject, ConfigType, Context, SENSITIVE_MASK, Validation};
use crate::resource::{PublicationEntry, PublicationMode};
use crate::status::StatusCode;
use crate::topic::{Filter, ListFilter, Resource, Topic};

/// A change of a value: GroupName, ConfigName and the new value.
pub type Change = (String, String, String);

/// A Config DataSet as defined by its owner (REQ-RES-CONFIG-005): the
/// context name, which is its Filter (REQ-RES-CONFIG-015), and the Config
/// with the initial values. A Config without `Context` gets one with the
/// context name as Name.
#[derive(Clone, PartialEq)]
pub struct ConfigSpec {
    /// The context name: Filter and WriterGroupName.
    pub context: String,
    /// The groups and configuration objects with their initial values.
    pub config: Config,
}

impl fmt::Debug for ConfigSpec {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ConfigSpec")
            .field("context", &self.context)
            .field("groups", &self.config.groups.keys().collect::<Vec<_>>())
            .finish_non_exhaustive()
    }
}

/// `GroupName`/`ConfigName` characters (REQ-RES-CONFIG-004):
/// `^[A-Za-z_][A-Za-z0-9_-]*$`.
pub fn valid_name(s: &str) -> bool {
    s.bytes()
        .next()
        .is_some_and(|c| c.is_ascii_alphabetic() || c == b'_')
        && s.bytes()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, b'_' | b'-'))
}

/// JSON number syntax (RFC 8259 6), the form of a Number value
/// (REQ-RES-CONFIG-007): `-? (0 | [1-9][0-9]*) (.[0-9]+)? ([eE][+-]?[0-9]+)?`.
pub fn json_number(s: &str) -> bool {
    let b = s.as_bytes();
    let mut i = usize::from(b.first() == Some(&b'-'));
    let digits = |i: &mut usize| {
        let start = *i;
        while b.get(*i).is_some_and(u8::is_ascii_digit) {
            *i += 1;
        }
        *i - start
    };
    match b.get(i) {
        Some(b'0') => i += 1,
        Some(b'1'..=b'9') => {
            digits(&mut i);
        }
        _ => return false,
    }
    if b.get(i) == Some(&b'.') {
        i += 1;
        if digits(&mut i) == 0 {
            return false;
        }
    }
    if matches!(b.get(i), Some(b'e' | b'E')) {
        i += 1;
        if matches!(b.get(i), Some(b'+' | b'-')) {
            i += 1;
        }
        if digits(&mut i) == 0 {
            return false;
        }
    }
    i == b.len()
}

/// Why a value is refused. The messages never contain the value (it may
/// be sensitive).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum ValueError {
    /// The value does not have the form of its Type (REQ-RES-CONFIG-007).
    Type(ConfigType),
    /// The value is longer than Length characters (REQ-RES-CONFIG-008).
    Length(u32),
    /// The Number is outside Min … Max (REQ-RES-CONFIG-008).
    Range,
    /// The value does not match the Pattern (REQ-RES-CONFIG-008).
    Pattern,
    /// The value is not one of Values (REQ-RES-CONFIG-008).
    Values,
    /// The mask is no value of a sensitive object (REQ-RES-CONFIG-025).
    Mask,
}

impl ValueError {
    /// The StatusCode of a `Set Config` with such a value:
    /// `BadTypeMismatch` for the Type, `BadOutOfRange` for the Validation
    /// (REQ-RES-CONFIG-023).
    pub fn status(&self) -> StatusCode {
        match self {
            Self::Type(_) => StatusCode::BAD_TYPE_MISMATCH,
            _ => StatusCode::BAD_OUT_OF_RANGE,
        }
    }
}

impl fmt::Display for ValueError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Type(t) => write!(f, "is no {t:?} value (REQ-RES-CONFIG-007)"),
            Self::Length(n) => write!(f, "is longer than {n} characters (REQ-RES-CONFIG-008)"),
            Self::Range => f.write_str("is outside Min … Max (REQ-RES-CONFIG-008)"),
            Self::Pattern => f.write_str("does not match the Pattern (REQ-RES-CONFIG-008)"),
            Self::Values => f.write_str("is not one of the Values (REQ-RES-CONFIG-008)"),
            Self::Mask => f.write_str(
                "is the mask \"********\", which no sensitive value can be (REQ-RES-CONFIG-025)",
            ),
        }
    }
}

impl std::error::Error for ValueError {}

/// Checks a value of `o`: the empty string (unset, REQ-RES-CONFIG-010) is
/// always valid; a non-empty value has the form of the Type
/// (REQ-RES-CONFIG-007) and satisfies the Validation (REQ-RES-CONFIG-008):
/// Length in characters (code points), Min/Max for Type Number, a match of
/// the Pattern anywhere in the value (unanchored, RE2-like syntax of the
/// `regex` crate as the checker's Go RE2; a Pattern that does not compile
/// is not checked, a definition with one is refused) and one of Values.
/// The mask of a sensitive object is refused: it keeps the value in a
/// `Set` (REQ-RES-CONFIG-025) and is resolved before this check.
///
/// # Errors
///
/// The first violation.
pub fn check_value(o: &ConfigObject, value: &str) -> Result<(), ValueError> {
    if value.is_empty() {
        return Ok(());
    }
    if o.sensitive == Some(true) && value == SENSITIVE_MASK {
        return Err(ValueError::Mask);
    }
    let typed = match o.kind {
        ConfigType::Boolean => matches!(value, "true" | "false"),
        ConfigType::Number => json_number(value),
        ConfigType::DateTime => DateTime::from_str(value).is_ok(),
        ConfigType::ByteString => ByteString::from_str(value).is_ok(),
        ConfigType::String => true,
    };
    if !typed {
        return Err(ValueError::Type(o.kind));
    }
    let Some(v) = &o.validation else {
        return Ok(());
    };
    if let Some(n) = v.length
        && value.chars().count() > n as usize
    {
        return Err(ValueError::Length(n));
    }
    if o.kind == ConfigType::Number {
        let x: f64 = value.parse().unwrap_or(f64::NAN);
        let below = v
            .min
            .as_ref()
            .and_then(serde_json::Number::as_f64)
            .is_some_and(|m| x < m);
        let above = v
            .max
            .as_ref()
            .and_then(serde_json::Number::as_f64)
            .is_some_and(|m| x > m);
        if below || above {
            return Err(ValueError::Range);
        }
    }
    if let Some(p) = &v.pattern
        && let Ok(re) = regex::Regex::new(p)
        && !re.is_match(value)
    {
        return Err(ValueError::Pattern);
    }
    if let Some(values) = &v.values
        && !values.iter().any(|a| a == value)
    {
        return Err(ValueError::Values);
    }
    Ok(())
}

/// Checks a definition (REQ-RES-CONFIG-004 … 008): GroupNames and
/// ConfigNames, a Pattern that compiles, Min/Max only for Type Number and
/// Min ≤ Max, and every Value and DefaultValue by [`check_value`].
///
/// # Errors
///
/// The first violation; the message names group and object, never a
/// value.
pub fn check_definition(config: &Config) -> Result<(), String> {
    for (g, group) in &config.groups {
        if !valid_name(g) || g == "Context" {
            return Err(format!(
                "GroupName {g:?}: letters, digits, \"-\" and \"_\", not starting with a digit \
                 or \"-\", not \"Context\" (REQ-RES-CONFIG-004)"
            ));
        }
        for (n, o) in &group.objects {
            if !valid_name(n) || matches!(n.as_ref(), "Name" | "Description") {
                return Err(format!(
                    "ConfigName {n:?} in group {g:?}: letters, digits, \"-\" and \"_\", not \
                     starting with a digit or \"-\", not \"Name\" or \"Description\" \
                     (REQ-RES-CONFIG-004)"
                ));
            }
            if let Some(v) = &o.validation {
                check_validation(o.kind, v).map_err(|e| format!("{g}/{n}: {e}"))?;
            }
            let values = core::iter::once(("Value", Some(&o.value)))
                .chain(core::iter::once(("DefaultValue", o.default_value.as_ref())));
            for (what, value) in values {
                if let Some(value) = value {
                    check_value(o, value).map_err(|e| format!("{g}/{n}: the {what} {e}"))?;
                }
            }
        }
    }
    Ok(())
}

fn check_validation(kind: ConfigType, v: &Validation) -> Result<(), String> {
    if let Some(p) = &v.pattern {
        regex::Regex::new(p).map_err(|e| format!("the Pattern does not compile: {e}"))?;
    }
    if (v.min.is_some() || v.max.is_some()) && kind != ConfigType::Number {
        return Err("Min and Max apply to Type Number only (REQ-RES-CONFIG-008)".to_owned());
    }
    let f = |n: &Option<serde_json::Number>| n.as_ref().and_then(serde_json::Number::as_f64);
    if let (Some(min), Some(max)) = (f(&v.min), f(&v.max))
        && min > max
    {
        return Err(format!("Min {min} is greater than Max {max}"));
    }
    Ok(())
}

/// `config` as published: a non-empty Value or DefaultValue of a
/// sensitive object is the mask `********`, an empty one stays empty
/// (REQ-RES-CONFIG-024, GL-ADR-021).
pub fn masked(config: &Config) -> Config {
    let mut out = config.clone();
    for o in out.groups.values_mut().flat_map(|g| g.objects.values_mut()) {
        if o.sensitive == Some(true) {
            if !o.value.is_empty() {
                SENSITIVE_MASK.clone_into(&mut o.value);
            }
            if let Some(d) = o.default_value.as_mut().filter(|d| !d.is_empty()) {
                SENSITIVE_MASK.clone_into(d);
            }
        }
    }
    out
}

/// The values of the Config DataSets set by `Set Config` or the
/// application (only those, not the defaults of the definition): persist
/// it with [`StoredConfigValues::to_json`] after every change, so the
/// configuration survives a restart. It holds sensitive values in clear
/// text; its `Debug` output does not.
#[derive(Clone, Default, PartialEq)]
pub struct StoredConfigValues {
    entries: Vec<StoredConfig>,
}

#[derive(Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct StoredConfig {
    source: String,
    filter: String,
    values: BTreeMap<String, BTreeMap<String, String>>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct StoredWire {
    version: u32,
    entries: Vec<StoredConfig>,
}

impl fmt::Debug for StoredConfigValues {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_list()
            .entries(self.entries.iter().map(|e| (&e.source, &e.filter)))
            .finish()
    }
}

impl StoredConfigValues {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// Other JSON, an unknown version or a Config DataSet that occurs
    /// twice; the message never contains a value.
    pub fn from_json(json: &[u8]) -> Result<Self, String> {
        // The serde message may quote a value; only its position is given.
        let wire: StoredWire = serde_json::from_slice(json).map_err(|e| {
            format!(
                "no table of Config values ({:?} error at line {} column {})",
                e.classify(),
                e.line(),
                e.column()
            )
        })?;
        if wire.version != 1 {
            return Err(format!("unknown version {}", wire.version));
        }
        let mut table = Self::new();
        for e in wire.entries {
            if table.get(&e.source, &e.filter).is_some() {
                return Err(format!(
                    "the Config {} {:?} occurs twice",
                    e.source, e.filter
                ));
            }
            table.entries.push(e);
        }
        Ok(table)
    }

    /// The table as JSON
    /// (`{"Version":1,"Entries":[{"Source","Filter","Values":{group:{name:value}}}]}`).
    ///
    /// # Panics
    ///
    /// Never: the table has only string keys.
    pub fn to_json(&self) -> Vec<u8> {
        serde_json::to_vec(&StoredWire {
            version: 1,
            entries: self.entries.clone(),
        })
        .expect("the table always serializes")
    }

    /// The values set of a Config DataSet by group and name.
    fn get(
        &self,
        source: &str,
        filter: &str,
    ) -> Option<&BTreeMap<String, BTreeMap<String, String>>> {
        self.entries
            .iter()
            .find(|e| e.source == source && e.filter == filter)
            .map(|e| &e.values)
    }

    /// Sets a value; returns whether the table changed.
    fn set(&mut self, source: &str, filter: &str, group: &str, name: &str, value: &str) -> bool {
        let found = self
            .entries
            .iter()
            .position(|e| e.source == source && e.filter == filter);
        let i = found.unwrap_or_else(|| {
            self.entries.push(StoredConfig {
                source: source.to_owned(),
                filter: filter.to_owned(),
                values: BTreeMap::new(),
            });
            self.entries.len() - 1
        });
        let names = self.entries[i].values.entry(group.to_owned()).or_default();
        if names.get(name).is_some_and(|old| old == value) {
            return false;
        }
        names.insert(name.to_owned(), value.to_owned());
        true
    }

    /// Number of Config DataSets with values set.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No values set.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// A Config DataSet of a source.
#[derive(Clone)]
pub(super) struct ConfigEntry {
    pub(super) source: IdentificationLink,
    pub(super) context: String,
    /// The topic-encoded context name.
    pub(super) filter: String,
    /// The current values in clear text.
    pub(super) config: Config,
    pub(super) writer: DataSetWriterId,
    /// Distinguishes a Config DataSet from an earlier one of the same
    /// context.
    instance: u64,
    updated_ms: u64,
    /// To be published on `Pub/Config/<Source>/<Filter>`.
    pub(super) pending: bool,
}

impl fmt::Debug for ConfigEntry {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ConfigEntry")
            .field("source", &self.source)
            .field("context", &self.context)
            .field("writer", &self.writer)
            .finish_non_exhaustive()
    }
}

/// A `Set Config` the provider checked (REQ-RES-CONFIG-023): the
/// application decides about the proposed values, then
/// [`Provider::complete_set_config`] answers. The values are in clear
/// text; `Debug` shows only which objects change.
#[derive(Clone, PartialEq)]
pub struct SetConfigRequest {
    source: IdentificationLink,
    context: String,
    changes: Vec<Change>,
    proposed: Config,
    message_id: MessageId,
    instance: u64,
}

impl fmt::Debug for SetConfigRequest {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("SetConfigRequest")
            .field("source", &self.source)
            .field("context", &self.context)
            .field(
                "changes",
                &self
                    .changes
                    .iter()
                    .map(|(g, n, _)| format!("{g}/{n}"))
                    .collect::<Vec<_>>(),
            )
            .field("message_id", &self.message_id)
            .finish_non_exhaustive()
    }
}

impl SetConfigRequest {
    /// The Source of the Config DataSet.
    pub fn source(&self) -> &IdentificationLink {
        &self.source
    }

    /// The context name (Filter) of the Config DataSet.
    pub fn context(&self) -> &str {
        &self.context
    }

    /// The values the `Set` carries (masks of sensitive objects resolved:
    /// they keep the value and are not listed), in the order GroupName,
    /// ConfigName.
    pub fn changes(&self) -> &[Change] {
        &self.changes
    }

    /// The complete Config with the values of the `Set` applied, in clear
    /// text.
    pub fn proposed(&self) -> &Config {
        &self.proposed
    }

    /// The MessageId of the `Set`: the ReplyTo of the answers.
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }
}

/// Upper bound of a Config NetworkMessage without the Config, the Source,
/// the Filter and the two MessageIds.
const ENVELOPE: usize = 512;

/// Whether `requested` (a JSON value of a read-only property) equals the
/// current one.
fn same<T: for<'de> Deserialize<'de> + PartialEq>(requested: &Json, current: Option<&T>) -> bool {
    match (T::deserialize(requested), current) {
        (Ok(r), Some(c)) => &r == c,
        (_, None) => requested.is_null(),
        (Err(_), Some(_)) => false,
    }
}

fn same_validation(requested: &Json, current: Option<&Validation>) -> bool {
    let Ok(r) = Validation::deserialize(requested) else {
        return false;
    };
    let c = current.cloned().unwrap_or_default();
    let f = |n: &Option<serde_json::Number>| n.as_ref().and_then(serde_json::Number::as_f64);
    r.length == c.length
        && f(&r.min) == f(&c.min)
        && f(&r.max) == f(&c.max)
        && r.pattern == c.pattern
        && r.values == c.values
}

/// Checks one configuration object of a `Set` against its definition and
/// returns the new value, `None` for the mask of a sensitive object
/// (REQ-RES-CONFIG-025).
fn object_change(o: &ConfigObject, set: &Map<String, Json>) -> Result<Option<String>, StatusCode> {
    let value = match set.get("Value") {
        None => return Err(StatusCode::BAD_INVALID_ARGUMENT),
        Some(Json::String(v)) => v,
        Some(_) => return Err(StatusCode::BAD_TYPE_MISMATCH),
    };
    if let Some(t) = set.get("Type")
        && ConfigType::deserialize(t).ok() != Some(o.kind)
    {
        return Err(StatusCode::BAD_TYPE_MISMATCH);
    }
    let sensitive = o.sensitive == Some(true);
    let flag = |j: &Json, c: Option<bool>| j.as_bool().is_some_and(|b| b == c.unwrap_or(false));
    for (key, j) in set {
        let unchanged = match key.as_str() {
            "Name" => same(j, Some(&o.name)),
            "Description" => same(j, o.description.as_ref()),
            "Unit" => same(j, o.unit.as_ref()),
            "DefaultValue" => {
                same(j, o.default_value.as_ref())
                    || (sensitive
                        && j.as_str() == Some(SENSITIVE_MASK)
                        && o.default_value.as_ref().is_some_and(|d| !d.is_empty()))
            }
            "Mandatory" => flag(j, o.mandatory),
            "Sensitive" => flag(j, o.sensitive),
            "Validation" => same_validation(j, o.validation.as_ref()),
            // Value and Type are checked above; unknown keys are ignored
            // (tolerant reader, MG 8).
            _ => true,
        };
        if !unchanged {
            return Err(StatusCode::BAD_NOT_WRITABLE);
        }
    }
    if sensitive && value == SENSITIVE_MASK {
        return Ok(None);
    }
    check_value(o, value).map_err(|e| e.status())?;
    Ok(Some(value.clone()))
}

/// Checks the payload of a `Set Config` against the current Config
/// (REQ-RES-CONFIG-003/005/011/023/025) and returns the changes, the
/// first violation otherwise (see the module documentation).
fn parse_set(current: &Config, payload: &Map<String, Json>) -> Result<Vec<Change>, StatusCode> {
    let mut changes = Vec::new();
    let mut objects = 0usize;
    for (g, gj) in payload {
        if g == "Context" {
            if !same(gj, current.context.as_ref()) {
                return Err(StatusCode::BAD_NOT_WRITABLE);
            }
            continue;
        }
        let group = current.groups.get(g).ok_or(StatusCode::BAD_NOT_FOUND)?;
        let Json::Object(gm) = gj else {
            return Err(StatusCode::BAD_DECODING_ERROR);
        };
        for (n, oj) in gm {
            match n.as_str() {
                "Name" if same(oj, Some(&group.name)) => continue,
                "Description" if same(oj, group.description.as_ref()) => continue,
                "Name" | "Description" => return Err(StatusCode::BAD_NOT_WRITABLE),
                _ => {}
            }
            let o = group.objects.get(n).ok_or(StatusCode::BAD_NOT_FOUND)?;
            let Json::Object(om) = oj else {
                return Err(StatusCode::BAD_DECODING_ERROR);
            };
            objects += 1;
            if let Some(v) = object_change(o, om)? {
                changes.push((g.clone(), n.clone(), v));
            }
        }
        let missing = group
            .objects
            .iter()
            .any(|(n, o)| o.mandatory == Some(true) && !gm.contains_key(n));
        if missing {
            return Err(StatusCode::BAD_INVALID_ARGUMENT);
        }
    }
    if objects == 0 {
        return Err(StatusCode::BAD_INVALID_ARGUMENT);
    }
    Ok(changes)
}

impl Provider {
    /// The provider with the persisted values of the Config DataSets
    /// (empty at the first start); [`Provider::add_config`] applies them.
    #[must_use]
    pub fn with_config_values(mut self, values: StoredConfigValues) -> Self {
        self.config_values = values;
        self
    }

    /// The values set by `Set Config` or [`Provider::set_config`].
    pub fn config_values(&self) -> &StoredConfigValues {
        &self.config_values
    }

    /// The values set as JSON if they changed since the last call; persist
    /// them.
    pub fn take_config_values(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.unsaved.config).then(|| self.config_values.to_json())
    }

    /// Adds a Config DataSet to the application or an asset (MG 8.3.3,
    /// REQ-RES-CONFIG-001) and returns its DataSetWriterId. The definition
    /// is checked by [`check_definition`]; a persisted value (see
    /// [`Provider::with_config_values`]) replaces the initial one if the
    /// object still exists and the value is still valid. The Config is
    /// published, masked, on `Pub/Config/<Source>/<Filter>` at once while
    /// connected and after every change (ON_REQUEST_FILTER_9), answered on
    /// `Get` and changed by `Set Config` from Level 2 on.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`], [`ProviderError::Topic`] for a
    /// context name that is no valid Filter, [`ProviderError::Config`] for
    /// an invalid definition or one whose message exceeds the
    /// MaxPacketSize, [`ProviderError::DuplicateConfig`],
    /// [`ProviderError::WriterId`].
    pub fn add_config(
        &mut self,
        source: &IdentificationLink,
        spec: ConfigSpec,
        unix_ms: u64,
    ) -> Result<DataSetWriterId, ProviderError> {
        if !self.is_source(source) {
            return Err(ProviderError::UnknownAsset(source.to_string()));
        }
        let filter = Filter::context_name(&spec.context)?;
        self.topic(Resource::Config, Some(source), Some(filter.clone()))?;
        if self.find_config(source, &spec.context).is_some() {
            return Err(ProviderError::DuplicateConfig {
                source: source.to_string(),
                context: spec.context,
            });
        }
        let mut config = spec.config;
        check_definition(&config).map_err(ProviderError::Config)?;
        if config.context.is_none() {
            config.context = Some(Context {
                name: LocalizedText::from(spec.context.as_str()),
                description: None,
            });
        }
        let filter = filter.to_string();
        if let Some(stored) = self.config_values.get(source.as_str(), &filter) {
            for (g, names) in stored {
                for (n, v) in names {
                    if let Some(o) = config.groups.get_mut(g).and_then(|g| g.objects.get_mut(n))
                        && check_value(o, v).is_ok()
                    {
                        v.clone_into(&mut o.value);
                    }
                }
            }
        }
        if !self.config_fits(source, &filter, &config) {
            return Err(ProviderError::Config(format!(
                "its message exceeds the MaxPacketSize of {} octets",
                self.config.max_packet_size
            )));
        }
        let writer = self.assign(WriterKey {
            resource: Resource::Config,
            source: source.as_str().to_owned(),
            filter: filter.clone(),
        })?;
        self.config_instance += 1;
        self.configs.push(ConfigEntry {
            source: source.clone(),
            context: spec.context,
            filter,
            config,
            writer,
            instance: self.config_instance,
            updated_ms: unix_ms,
            pending: true,
        });
        self.pending.list = self.config.get;
        Ok(writer)
    }

    /// Removes a Config DataSet: it is no longer published or answered and
    /// the PublicationList is published again; its DataSetWriterId stays
    /// reserved and its persisted values are kept.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownConfig`].
    pub fn remove_config(
        &mut self,
        source: &IdentificationLink,
        context: &str,
    ) -> Result<(), ProviderError> {
        let i = self
            .find_config(source, context)
            .ok_or_else(|| ProviderError::UnknownConfig {
                source: source.to_string(),
                context: context.to_owned(),
            })?;
        self.configs.remove(i);
        self.pending.list = self.config.get;
        Ok(())
    }

    /// The current Config of a Config DataSet with its values in clear
    /// text.
    pub fn config(&self, source: &IdentificationLink, context: &str) -> Option<&Config> {
        self.find_config(source, context)
            .map(|i| &self.configs[i].config)
    }

    /// The context names of the Config DataSets of a source in the order
    /// of addition.
    pub fn config_contexts<'a>(
        &'a self,
        source: &'a IdentificationLink,
    ) -> impl Iterator<Item = &'a str> {
        self.configs
            .iter()
            .filter(move |c| &c.source == source)
            .map(|c| c.context.as_str())
    }

    /// Sets values of a Config DataSet from the application side, all or
    /// none, checked by [`check_value`]; a change is persisted (see
    /// [`Provider::take_config_values`]) and published at once. No Status
    /// event. Returns whether a value changed.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownConfig`], [`ProviderError::Value`] for an
    /// unknown object or a refused value (the message names the object,
    /// never the value).
    pub fn set_config(
        &mut self,
        source: &IdentificationLink,
        context: &str,
        changes: &[Change],
        unix_ms: u64,
    ) -> Result<bool, ProviderError> {
        let i = self
            .find_config(source, context)
            .ok_or_else(|| ProviderError::UnknownConfig {
                source: source.to_string(),
                context: context.to_owned(),
            })?;
        let mut next = self.configs[i].config.clone();
        for (g, n, v) in changes {
            let o = next
                .groups
                .get_mut(g)
                .and_then(|group| group.objects.get_mut(n))
                .ok_or_else(|| ProviderError::Value(format!("no configuration object {g}/{n}")))?;
            check_value(o, v)
                .map_err(|e| ProviderError::Value(format!("{g}/{n}: the value {e}")))?;
            v.clone_into(&mut o.value);
        }
        let filter = self.configs[i].filter.clone();
        if !self.config_fits(source, &filter, &next) {
            return Err(ProviderError::Value(format!(
                "the Config {context:?} would exceed the MaxPacketSize of {} octets",
                self.config.max_packet_size
            )));
        }
        let modified = self.write_config(i, changes, unix_ms);
        self.configs[i].pending |= modified;
        Ok(modified)
    }

    /// Answers a `Set Config` the application decided: on `Ok` the values
    /// are applied and persisted, the complete Config is published,
    /// masked, on `Pub/Config/<Source>/<Filter>` with the ReplyTo of the
    /// `Set` (REQ-RES-CONFIG-022), then the Status event Good; on `Err`
    /// only the Status event with that code (REQ-RES-CONFIG-021/023), an
    /// `Err` with a Good code as `Bad`. If the Config DataSet was removed
    /// or added anew meanwhile, the answer is `BadNotFound`. Returns the
    /// publications and whether a value changed.
    pub fn complete_set_config(
        &mut self,
        request: &SetConfigRequest,
        result: Result<(), StatusCode>,
        unix_ms: u64,
    ) -> (Vec<Publication>, bool) {
        let mut code = match result {
            Ok(()) => StatusCode::GOOD,
            Err(c) if c.is_good() => StatusCode::BAD,
            Err(c) => c,
        };
        let mut out = Vec::new();
        let mut changed = false;
        if code.is_good() {
            match self.apply_set_config(request, unix_ms) {
                Ok((p, c)) => (out, changed) = (p, c),
                Err(c) => code = c,
            }
        }
        out.extend(self.status_event(&request.source, code, Some(&request.message_id), unix_ms));
        (out, changed)
    }

    // ------------------------------------------------------------ internals

    pub(super) fn find_config(&self, source: &IdentificationLink, context: &str) -> Option<usize> {
        self.configs
            .iter()
            .position(|c| &c.source == source && c.context == context)
    }

    /// Whether the Config NetworkMessage of `config` with a ReplyTo stays
    /// within the MaxPacketSize.
    fn config_fits(&self, source: &IdentificationLink, filter: &str, config: &Config) -> bool {
        let body = serde_json::to_vec(&masked(config)).map_or(usize::MAX, |v| v.len());
        let publisher = self.app_id().as_str().len() + 16;
        let size = body + ENVELOPE + 2 * (publisher + 24) + source.as_str().len() + filter.len();
        size <= self.config.max_packet_size
    }

    /// Applies checked changes and records them in the table; returns
    /// whether a value changed.
    fn write_config(&mut self, i: usize, changes: &[Change], unix_ms: u64) -> bool {
        let Self {
            configs,
            config_values,
            unsaved,
            ..
        } = self;
        let e = &mut configs[i];
        let mut modified = false;
        for (g, n, v) in changes {
            let Some(o) = e
                .config
                .groups
                .get_mut(g)
                .and_then(|group| group.objects.get_mut(n))
            else {
                continue;
            };
            if &o.value != v {
                v.clone_into(&mut o.value);
                modified = true;
            }
            unsaved.config |= config_values.set(e.source.as_str(), &e.filter, g, n, v);
        }
        if modified {
            e.updated_ms = unix_ms;
        }
        modified
    }

    /// Checks a `Set Config` (REQ-RES-CONFIG-019, REQ-RES-CONFIG-023).
    pub(super) fn set_config_request(
        &self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
    ) -> Result<SetConfigRequest, StatusCode> {
        let Some(Filter::ContextName(context)) = topic.filter() else {
            return Err(StatusCode::BAD_NOT_FOUND);
        };
        let i = self
            .find_config(source, context)
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        let Some(Json::Object(payload)) = wire.single()?.get("Payload") else {
            return Err(StatusCode::BAD_DECODING_ERROR);
        };
        let e = &self.configs[i];
        let changes = parse_set(&e.config, payload)?;
        let mut proposed = e.config.clone();
        for (g, n, v) in &changes {
            if let Some(o) = proposed
                .groups
                .get_mut(g)
                .and_then(|group| group.objects.get_mut(n))
            {
                v.clone_into(&mut o.value);
            }
        }
        if !self.config_fits(source, &e.filter, &proposed) {
            return Err(StatusCode::BAD_OUT_OF_RANGE);
        }
        Ok(SetConfigRequest {
            source: source.clone(),
            context: context.clone(),
            changes,
            proposed,
            message_id: wire.message_id.clone(),
            instance: e.instance,
        })
    }

    fn apply_set_config(
        &mut self,
        r: &SetConfigRequest,
        unix_ms: u64,
    ) -> Result<(Vec<Publication>, bool), StatusCode> {
        let i = self
            .find_config(&r.source, &r.context)
            .filter(|i| self.configs[*i].instance == r.instance)
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        let changed = self.write_config(i, &r.changes, unix_ms);
        self.configs[i].pending = false;
        let out = self.publish_config(i, Some(&r.message_id), unix_ms);
        Ok((out, changed))
    }

    /// The Config DataSetMessages of the sources addressed, with context
    /// `context` if given, masked, in the order of sources and addition.
    pub(super) fn config_items(
        &self,
        source: Option<&IdentificationLink>,
        context: Option<&str>,
    ) -> Vec<DataSetMessage<Config>> {
        let mut out = Vec::new();
        for il in self.sources(source) {
            for c in &self.configs {
                if c.source == il && context.is_none_or(|x| c.context == x) {
                    out.push(Self::config_message(c));
                }
            }
        }
        out
    }

    fn config_message(c: &ConfigEntry) -> DataSetMessage<Config> {
        DataSetMessage::new(
            c.writer,
            c.source.clone(),
            c.filter.clone(),
            masked(&c.config),
        )
        .with_timestamp(date_time(c.updated_ms))
    }

    /// One Config on `Pub/Config/<Source>/<Filter>`.
    fn publish_config(
        &mut self,
        i: usize,
        reply_to: Option<&MessageId>,
        unix_ms: u64,
    ) -> Vec<Publication> {
        let c = &self.configs[i];
        let Ok(topic) = self.topic(
            Resource::Config,
            Some(&c.source),
            Some(Filter::ContextName(c.context.clone())),
        ) else {
            return Vec::new();
        };
        let items = vec![Self::config_message(c)];
        let limit = self.config.max_packet_size;
        self.series(items, None, reply_to, limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect()
    }

    /// The pending Configs (ON_REQUEST_FILTER_9).
    pub(super) fn config_changes(&mut self, unix_ms: u64) -> Vec<Publication> {
        let mut out = Vec::new();
        for i in 0..self.configs.len() {
            if core::mem::take(&mut self.configs[i].pending) {
                out.extend(self.publish_config(i, None, unix_ms));
            }
        }
        out
    }

    /// The PublicationList entries of the Config DataSets of `il`
    /// (ON_REQUEST_FILTER_9: published on their Filter topic after every
    /// change and answered on `Get`; not configurable).
    pub(super) fn config_list_items(
        &self,
        il: &IdentificationLink,
        filter: Option<&ListFilter>,
        list_writer: DataSetWriterId,
    ) -> Vec<DataSetMessage<PublicationEntry>> {
        let mut out = Vec::new();
        for c in self.configs.iter().filter(|c| &c.source == il) {
            let f = Filter::ContextName(c.context.clone());
            if filter.is_some_and(|lf| {
                lf.resource() != Resource::Config || lf.filter().is_some_and(|t| t != &f)
            }) {
                continue;
            }
            let entry = PublicationEntry {
                resource: Resource::Config,
                source: il.as_str().to_owned(),
                filter: Some(c.filter.clone()),
                data_set_writer_id: c.writer,
                mode: Some(PublicationMode::OnRequestFilter),
                interval: None,
                precisions: None,
                config: None,
            };
            out.push(DataSetMessage::new(
                list_writer,
                il.clone(),
                format!("Config/{}", c.filter),
                entry,
            ));
        }
        out
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::resource::Group;

    fn object(kind: ConfigType, value: &str) -> ConfigObject {
        ConfigObject {
            kind,
            value: value.to_owned(),
            default_value: None,
            unit: None,
            mandatory: None,
            sensitive: None,
            name: LocalizedText::from("x"),
            description: None,
            validation: None,
        }
    }

    fn with(v: Validation, kind: ConfigType) -> ConfigObject {
        ConfigObject {
            validation: Some(v),
            ..object(kind, "")
        }
    }

    #[test]
    fn names() {
        for ok in ["a", "_", "eth0", "ip_address", "a-b", "Z9", "aut0-r3fr3sh"] {
            assert!(valid_name(ok), "{ok}");
        }
        for bad in ["", "0a", "-a", "a b", "a.b", "ä"] {
            assert!(!valid_name(bad), "{bad}");
        }
    }

    #[test]
    fn json_numbers() {
        for ok in ["0", "-0", "1", "60", "1.5", "-1.25e-3", "1E+9", "1e400"] {
            assert!(json_number(ok), "{ok}");
        }
        for bad in [
            "", "-", "01", "1.", ".5", "+1", "1e", "0x10", "NaN", "inf", " 1",
        ] {
            assert!(!json_number(bad), "{bad}");
        }
    }

    #[test]
    fn values_by_type() {
        let cases = [
            (ConfigType::Boolean, "true", true),
            (ConfigType::Boolean, "True", false),
            (ConfigType::Number, "443", true),
            (ConfigType::Number, "4,5", false),
            (ConfigType::DateTime, "2026-10-27T08:00:00Z", true),
            (ConfigType::DateTime, "2026-02-30T00:00:00Z", false),
            (ConfigType::ByteString, "YWJj", true),
            (ConfigType::ByteString, "abc", false),
            (ConfigType::String, "anything", true),
        ];
        for (kind, value, ok) in cases {
            let r = check_value(&object(kind, ""), value);
            assert_eq!(r.is_ok(), ok, "{kind:?} {value}");
            if !ok {
                assert_eq!(r.unwrap_err().status(), StatusCode::BAD_TYPE_MISMATCH);
            }
        }
        for kind in [
            ConfigType::Boolean,
            ConfigType::Number,
            ConfigType::DateTime,
        ] {
            assert!(check_value(&object(kind, ""), "").is_ok());
        }
    }

    #[test]
    fn validation() {
        let number = |min: i64, max: i64| {
            with(
                Validation {
                    min: Some(min.into()),
                    max: Some(max.into()),
                    ..Validation::default()
                },
                ConfigType::Number,
            )
        };
        assert!(check_value(&number(0, 65535), "65535").is_ok());
        assert_eq!(
            check_value(&number(0, 65535), "65536"),
            Err(ValueError::Range)
        );
        assert_eq!(check_value(&number(0, 10), "-1e-9"), Err(ValueError::Range));
        let length = with(
            Validation {
                length: Some(3),
                ..Validation::default()
            },
            ConfigType::String,
        );
        assert!(check_value(&length, "äöü").is_ok());
        assert_eq!(check_value(&length, "abcd"), Err(ValueError::Length(3)));
        let ip = with(
            Validation {
                pattern: Some(
                    r"^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$"
                        .to_owned(),
                ),
                ..Validation::default()
            },
            ConfigType::String,
        );
        assert!(check_value(&ip, "192.168.1.123").is_ok());
        assert_eq!(check_value(&ip, "192.168.1.256"), Err(ValueError::Pattern));
        let unanchored = with(
            Validation {
                pattern: Some("b+".to_owned()),
                ..Validation::default()
            },
            ConfigType::String,
        );
        assert!(check_value(&unanchored, "abba").is_ok());
        let one_of = with(
            Validation {
                values: Some(vec!["K".to_owned(), "°C".to_owned()]),
                ..Validation::default()
            },
            ConfigType::String,
        );
        assert!(check_value(&one_of, "°C").is_ok());
        assert_eq!(check_value(&one_of, "°F"), Err(ValueError::Values));
        assert_eq!(ValueError::Values.status(), StatusCode::BAD_OUT_OF_RANGE);
    }

    #[test]
    fn mask_is_no_sensitive_value() {
        let mut o = object(ConfigType::String, "");
        assert!(check_value(&o, SENSITIVE_MASK).is_ok());
        o.sensitive = Some(true);
        assert_eq!(check_value(&o, SENSITIVE_MASK), Err(ValueError::Mask));
        assert!(!ValueError::Mask.to_string().contains("secret"));
    }

    fn config(objects: Vec<(&str, ConfigObject)>) -> Config {
        Config {
            context: None,
            groups: BTreeMap::from([(
                "g".to_owned(),
                Group {
                    name: LocalizedText::from("G"),
                    description: None,
                    objects: objects
                        .into_iter()
                        .map(|(n, o)| (n.to_owned(), o))
                        .collect(),
                },
            )]),
        }
    }

    #[test]
    fn definitions() {
        assert!(check_definition(&config(vec![("a", object(ConfigType::Number, "1"))])).is_ok());
        let bad = [
            config(vec![("Name", object(ConfigType::String, ""))]),
            config(vec![("1a", object(ConfigType::String, ""))]),
            config(vec![("a", object(ConfigType::Number, "x"))]),
            config(vec![(
                "a",
                ConfigObject {
                    default_value: Some("x".to_owned()),
                    ..object(ConfigType::Boolean, "")
                },
            )]),
            config(vec![(
                "a",
                with(
                    Validation {
                        pattern: Some("(".to_owned()),
                        ..Validation::default()
                    },
                    ConfigType::String,
                ),
            )]),
            config(vec![(
                "a",
                with(
                    Validation {
                        min: Some(1.into()),
                        ..Validation::default()
                    },
                    ConfigType::String,
                ),
            )]),
            config(vec![(
                "a",
                with(
                    Validation {
                        min: Some(2.into()),
                        max: Some(1.into()),
                        ..Validation::default()
                    },
                    ConfigType::Number,
                ),
            )]),
            config(vec![(
                "a",
                ConfigObject {
                    sensitive: Some(true),
                    ..object(ConfigType::String, SENSITIVE_MASK)
                },
            )]),
        ];
        for c in bad {
            assert!(check_definition(&c).is_err());
        }
        let mut context = config(vec![]);
        let g = context.groups.remove("g").unwrap();
        context.groups.insert("Context".to_owned(), g);
        assert!(check_definition(&context).is_err());
    }

    #[test]
    fn masking() {
        let secret = ConfigObject {
            sensitive: Some(true),
            default_value: Some("dflt".to_owned()),
            ..object(ConfigType::String, "s3cr3t")
        };
        let unset = ConfigObject {
            sensitive: Some(true),
            default_value: Some(String::new()),
            ..object(ConfigType::String, "")
        };
        let plain = object(ConfigType::String, "visible");
        let m = masked(&config(vec![("a", secret), ("b", unset), ("c", plain)]));
        let o = &m.groups["g"].objects;
        assert_eq!(o["a"].value, SENSITIVE_MASK);
        assert_eq!(o["a"].default_value.as_deref(), Some(SENSITIVE_MASK));
        assert_eq!(o["b"].value, "");
        assert_eq!(o["b"].default_value.as_deref(), Some(""));
        assert_eq!(o["c"].value, "visible");
    }

    #[test]
    fn table_round_trip_without_leaks() {
        let mut t = StoredConfigValues::new();
        assert!(t.set("a.com/M/P/S", "net", "g", "password", "s3cr3t"));
        assert!(!t.set("a.com/M/P/S", "net", "g", "password", "s3cr3t"));
        assert!(t.set("a.com/M/P/S", "net", "g", "password", ""));
        assert!(t.set("a.com/M/P/S", "dev", "g", "port", "443"));
        assert_eq!(t.len(), 2);
        let back = StoredConfigValues::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
        assert_eq!(back.get("a.com/M/P/S", "dev").unwrap()["g"]["port"], "443");
        assert!(t.set("a.com/M/P/S", "net", "g", "password", "s3cr3t"));
        assert!(!format!("{t:?}").contains("s3cr3t"));
        let broken =
            br#"{"Version":1,"Entries":[{"Source":"s","Filter":"f","Values":{"g":"s3cr3t"}}]}"#;
        let e = StoredConfigValues::from_json(broken).unwrap_err();
        assert!(!e.contains("s3cr3t"), "{e}");
        assert!(StoredConfigValues::from_json(br#"{"Version":2,"Entries":[]}"#).is_err());
        let twice = br#"{"Version":1,"Entries":[{"Source":"s","Filter":"f","Values":{}},{"Source":"s","Filter":"f","Values":{}}]}"#;
        assert!(StoredConfigValues::from_json(twice).is_err());
    }
}
