//! Config resource (MG 8.3.3): definition by the owner and the validated
//! values with string-keyed typed getters (ADR-0020 §10, Q-022).
//!
//! P4 increment 4 covers the definition, its validation
//! ([`ConfigDef::values`]) and the getters. Publication, `Set Config`
//! handling and the `ConfigHandle` of the sketch follow with increment 7.

use std::collections::BTreeMap;
use std::str::FromStr;

use oec_core::message::{ByteString, DateTime, LocalizedText};
use oec_core::resource::{self, Config, Group, Validation};

use crate::{Error, Result};

/// The five Config types (REQ-RES-CONFIG-006); values are strings on the
/// bus (REQ-RES-CONFIG-007) and typed in the API.
pub use oec_core::resource::ConfigType;

/// Definition of one configuration object. Names follow
/// `^[A-Za-z_][A-Za-z0-9_-]*$` (REQ-RES-CONFIG-004), checked by
/// [`ConfigDef::values`].
#[derive(Debug, Clone)]
pub struct ConfigObject {
    name: String,
    kind: ConfigType,
    default: String,
    label: Option<String>,
    unit: Option<String>,
    mandatory: bool,
    sensitive: bool,
    min: Option<f64>,
    max: Option<f64>,
    length: Option<u32>,
    pattern: Option<String>,
    values: Option<Vec<String>>,
}

impl ConfigObject {
    /// An object of `ty` with `default` in the string form of the bus
    /// (REQ-RES-CONFIG-007); `""` means unset. The default is also the
    /// initial value.
    pub fn new(name: &str, ty: ConfigType, default: &str) -> Self {
        Self {
            name: name.to_owned(),
            kind: ty,
            default: default.to_owned(),
            label: None,
            unit: None,
            mandatory: false,
            sensitive: false,
            min: None,
            max: None,
            length: None,
            pattern: None,
            values: None,
        }
    }

    /// A `Number` object.
    pub fn number(name: &str, default: f64) -> Self {
        Self::new(name, ConfigType::Number, &default.to_string())
    }

    /// A `String` object.
    pub fn string(name: &str, default: &str) -> Self {
        Self::new(name, ConfigType::String, default)
    }

    /// A `Boolean` object.
    pub fn boolean(name: &str, default: bool) -> Self {
        Self::new(
            name,
            ConfigType::Boolean,
            if default { "true" } else { "false" },
        )
    }

    /// Display name (`Name`, LocalizedText in `en-US`); default: the name.
    #[must_use]
    pub fn label(mut self, text: &str) -> Self {
        self.label = Some(text.to_owned());
        self
    }

    /// `Unit` (REQ-RES-CONFIG-009).
    #[must_use]
    pub fn unit(mut self, unit: &str) -> Self {
        self.unit = Some(unit.to_owned());
        self
    }

    /// `Mandatory` (REQ-RES-CONFIG-011).
    #[must_use]
    pub fn mandatory(mut self) -> Self {
        self.mandatory = true;
        self
    }

    /// `Sensitive`: masked as `********` in every `Pub`
    /// (REQ-RES-CONFIG-024); the mask in a `Set` keeps the value
    /// (REQ-RES-CONFIG-025).
    #[must_use]
    pub fn sensitive(mut self) -> Self {
        self.sensitive = true;
        self
    }

    /// `Validation.Min` (REQ-RES-CONFIG-008, Type Number).
    #[must_use]
    pub fn min(mut self, min: f64) -> Self {
        self.min = Some(min);
        self
    }

    /// `Validation.Max` (REQ-RES-CONFIG-008, Type Number).
    #[must_use]
    pub fn max(mut self, max: f64) -> Self {
        self.max = Some(max);
        self
    }

    /// `Validation.Length`: at most `length` characters (REQ-RES-CONFIG-008).
    #[must_use]
    pub fn max_length(mut self, length: u32) -> Self {
        self.length = Some(length);
        self
    }

    /// `Validation.Pattern` (REQ-RES-CONFIG-008). Published as given; the
    /// SDK checks values against it from P4 increment 7 on (A-21).
    #[must_use]
    pub fn pattern(mut self, regex: &str) -> Self {
        self.pattern = Some(regex.to_owned());
        self
    }

    /// `Validation.Values`: the allowed values (REQ-RES-CONFIG-008).
    #[must_use]
    pub fn one_of(mut self, values: &[&str]) -> Self {
        self.values = Some(values.iter().map(|&v| v.to_owned()).collect());
        self
    }

    fn resolve(&self, group: &str) -> Result<resource::ConfigObject> {
        let at = || format!("{group}/{}", self.name);
        if !valid_name(&self.name) || matches!(self.name.as_str(), "Name" | "Description") {
            return Err(Error::Invalid(format!(
                "ConfigName {:?} in group {group:?}: letters, digits, \"-\" and \"_\", not \
                 starting with a digit or \"-\", not \"Name\" or \"Description\" \
                 (REQ-RES-CONFIG-004)",
                self.name
            )));
        }
        let number = |v: Option<f64>, key: &str| -> Result<Option<serde_json::Number>> {
            v.map(|f| {
                serde_json::Number::from_f64(f).ok_or_else(|| {
                    Error::Invalid(format!("{}: Validation {key} {f} is no number", at()))
                })
            })
            .transpose()
        };
        let validation = Validation {
            length: self.length,
            min: number(self.min, "Min")?,
            max: number(self.max, "Max")?,
            pattern: self.pattern.clone(),
            values: self.values.clone(),
        };
        let has_validation = validation.length.is_some()
            || validation.min.is_some()
            || validation.max.is_some()
            || validation.pattern.is_some()
            || validation.values.is_some();
        let object = resource::ConfigObject {
            kind: self.kind,
            value: self.default.clone(),
            default_value: Some(self.default.clone()),
            unit: self.unit.clone(),
            mandatory: self.mandatory.then_some(true),
            sensitive: self.sensitive.then_some(true),
            name: LocalizedText::from(self.label.as_deref().unwrap_or(&self.name)),
            description: None,
            validation: has_validation.then_some(validation),
        };
        check_value(&object, &self.default)
            .map_err(|e| Error::Invalid(format!("{}: default {:?} {e}", at(), self.default)))?;
        Ok(object)
    }
}

/// Definition of a Config DataSet; `context` is its Filter
/// (REQ-RES-CONFIG-015). An asset may have several (REQ-RES-CONFIG-001).
#[derive(Debug, Clone)]
pub struct ConfigDef {
    context: String,
    groups: Vec<(String, String, Vec<ConfigObject>)>,
}

impl ConfigDef {
    /// `context` is the Filter of the topic and the WriterGroupName
    /// (REQ-RES-CONFIG-015, REQ-TOP-022).
    pub fn new(context: &str) -> Self {
        Self {
            context: context.to_owned(),
            groups: Vec::new(),
        }
    }

    /// Adds a group (REQ-RES-CONFIG-013); the group of common objects is
    /// named `common` by convention (REQ-RES-CONFIG-014).
    #[must_use]
    pub fn group(
        mut self,
        name: &str,
        label: &str,
        objects: impl IntoIterator<Item = ConfigObject>,
    ) -> Self {
        self.groups.push((
            name.to_owned(),
            label.to_owned(),
            objects.into_iter().collect(),
        ));
        self
    }

    /// The Filter (context name) of the DataSet.
    pub fn context(&self) -> &str {
        &self.context
    }

    /// Validates the definition and returns its initial values (the
    /// defaults): the context is a valid Filter (REQ-RES-CONFIG-015,
    /// REQ-TOP-019), names follow REQ-RES-CONFIG-004 and are unique, every
    /// default has the form of its Type (REQ-RES-CONFIG-007) and satisfies
    /// its Validation except Pattern (REQ-RES-CONFIG-008, A-21).
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] naming the first violation.
    pub fn values(&self) -> Result<ConfigValues> {
        oec_core::topic::Filter::context_name(&self.context)?;
        let mut groups = BTreeMap::new();
        for (name, label, objects) in &self.groups {
            if !valid_name(name) || name == "Context" {
                return Err(Error::Invalid(format!(
                    "GroupName {name:?}: letters, digits, \"-\" and \"_\", not starting with \
                     a digit or \"-\", not \"Context\" (REQ-RES-CONFIG-004)"
                )));
            }
            let mut resolved = BTreeMap::new();
            for o in objects {
                if resolved.insert(o.name.clone(), o.resolve(name)?).is_some() {
                    return Err(Error::Invalid(format!(
                        "ConfigName {:?} twice in group {name:?}",
                        o.name
                    )));
                }
            }
            let group = Group {
                name: LocalizedText::from(label.as_str()),
                description: None,
                objects: resolved,
            };
            if groups.insert(name.clone(), group).is_some() {
                return Err(Error::Invalid(format!(
                    "GroupName {name:?} twice in Config {:?}",
                    self.context
                )));
            }
        }
        Ok(ConfigValues {
            config: Config {
                context: None,
                groups,
            },
            changed: Vec::new(),
        })
    }
}

/// A consistent snapshot of the current (or proposed) values.
#[derive(Debug, Clone)]
pub struct ConfigValues {
    config: Config,
    changed: Vec<(String, String)>,
}

impl ConfigValues {
    fn object(&self, group: &str, name: &str) -> Option<&resource::ConfigObject> {
        self.config.groups.get(group)?.objects.get(name)
    }

    /// The value of a `Number` object; `None` if unset (empty string,
    /// REQ-RES-CONFIG-010), unknown or of another Type.
    pub fn number(&self, group: &str, name: &str) -> Option<f64> {
        let o = self.object(group, name)?;
        (o.kind == ConfigType::Number && !o.value.is_empty())
            .then(|| o.value.parse().ok())
            .flatten()
    }

    /// The value of any object in its string form (REQ-RES-CONFIG-007);
    /// `None` if unset (empty string, REQ-RES-CONFIG-010) or unknown.
    pub fn string(&self, group: &str, name: &str) -> Option<&str> {
        let o = self.object(group, name)?;
        (!o.value.is_empty()).then_some(o.value.as_str())
    }

    /// The value of a `Boolean` object; `None` if unset, unknown or of
    /// another Type.
    pub fn boolean(&self, group: &str, name: &str) -> Option<bool> {
        let o = self.object(group, name)?;
        match (o.kind, o.value.as_str()) {
            (ConfigType::Boolean, "true") => Some(true),
            (ConfigType::Boolean, "false") => Some(false),
            _ => None,
        }
    }

    /// `(group, name)` of the objects that differ from the current values;
    /// empty for the initial values.
    pub fn changed(&self) -> impl Iterator<Item = (&str, &str)> {
        self.changed.iter().map(|(g, n)| (g.as_str(), n.as_str()))
    }
}

/// `GroupName`/`ConfigName` characters (REQ-RES-CONFIG-004).
fn valid_name(s: &str) -> bool {
    s.bytes()
        .next()
        .is_some_and(|c| c.is_ascii_alphabetic() || c == b'_')
        && s.bytes()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, b'_' | b'-'))
}

/// A non-empty value has the form of its Type (REQ-RES-CONFIG-007) and
/// satisfies the Validation (REQ-RES-CONFIG-008); Pattern is not checked
/// yet (A-21). Returns the reason otherwise.
fn check_value(o: &resource::ConfigObject, value: &str) -> std::result::Result<(), String> {
    if value.is_empty() {
        return Ok(());
    }
    let typed = match o.kind {
        ConfigType::Boolean => matches!(value, "true" | "false"),
        ConfigType::Number => json_number(value),
        ConfigType::DateTime => DateTime::from_str(value).is_ok(),
        ConfigType::ByteString => ByteString::from_str(value).is_ok(),
        ConfigType::String => true,
    };
    if !typed {
        return Err(format!(
            "is no {o:?} value (REQ-RES-CONFIG-007)",
            o = o.kind
        ));
    }
    let Some(v) = &o.validation else {
        return Ok(());
    };
    if let Some(n) = v.length
        && value.chars().count() > n as usize
    {
        return Err(format!(
            "is longer than {n} characters (REQ-RES-CONFIG-008)"
        ));
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
            return Err("is outside Min … Max (REQ-RES-CONFIG-008)".to_owned());
        }
    }
    if let Some(values) = &v.values
        && !values.iter().any(|a| a == value)
    {
        return Err(format!("is not one of {values:?} (REQ-RES-CONFIG-008)"));
    }
    Ok(())
}

/// JSON number syntax (RFC 8259 6): `-? (0 | [1-9][0-9]*) (.[0-9]+)? ([eE][+-]?[0-9]+)?`.
fn json_number(s: &str) -> bool {
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

#[cfg(test)]
mod tests {
    use super::{ConfigDef, ConfigObject, ConfigType, json_number, valid_name};
    use crate::Error;

    fn def() -> ConfigDef {
        ConfigDef::new("weather").group(
            "common",
            "Common",
            [
                ConfigObject::number("interval", 60.0)
                    .unit("s")
                    .min(1.0)
                    .max(3600.0),
                ConfigObject::string("city", "Berlin").max_length(32),
                ConfigObject::boolean("metric", true),
                ConfigObject::string("api_key", "").sensitive().mandatory(),
                ConfigObject::new("since", ConfigType::DateTime, "2026-10-26T08:00:00Z"),
                ConfigObject::string("mode", "fast").one_of(&["fast", "slow"]),
            ],
        )
    }

    #[test]
    fn typed_getters() {
        let v = def().values().unwrap();
        assert_eq!(v.number("common", "interval"), Some(60.0));
        assert_eq!(v.string("common", "interval"), Some("60"));
        assert_eq!(v.string("common", "city"), Some("Berlin"));
        assert_eq!(v.boolean("common", "metric"), Some(true));
        assert_eq!(v.string("common", "api_key"), None);
        assert_eq!(v.number("common", "city"), None);
        assert_eq!(v.boolean("common", "city"), None);
        assert_eq!(v.number("other", "interval"), None);
        assert_eq!(v.changed().count(), 0);
    }

    #[test]
    fn invalid_definitions() {
        let invalid = |d: ConfigDef| matches!(d.values(), Err(Error::Invalid(_)));
        let one = |o: ConfigObject| ConfigDef::new("c").group("g", "G", [o]);
        assert!(invalid(ConfigDef::new("")));
        assert!(invalid(ConfigDef::new("c").group("Context", "C", [])));
        assert!(invalid(ConfigDef::new("c").group("1g", "G", [])));
        assert!(invalid(ConfigDef::new("c").group("g", "G", []).group(
            "g",
            "G",
            []
        )));
        assert!(invalid(one(ConfigObject::string("Name", ""))));
        assert!(invalid(one(ConfigObject::string("-x", ""))));
        assert!(invalid(ConfigDef::new("c").group(
            "g",
            "G",
            [ConfigObject::string("a", ""), ConfigObject::string("a", "")]
        )));
        assert!(invalid(one(ConfigObject::number("n", f64::NAN))));
        assert!(invalid(one(
            ConfigObject::number("n", 0.0).min(f64::INFINITY)
        )));
        assert!(invalid(one(ConfigObject::number("n", 0.0).min(1.0))));
        assert!(invalid(one(ConfigObject::number("n", 5.0).max(1.0))));
        assert!(invalid(one(ConfigObject::string("s", "abc").max_length(2))));
        assert!(invalid(one(ConfigObject::string("s", "x").one_of(&["a"]))));
        assert!(invalid(one(ConfigObject::new(
            "b",
            ConfigType::Boolean,
            "yes"
        ))));
        assert!(invalid(one(ConfigObject::new(
            "d",
            ConfigType::DateTime,
            "2026-02-30T00:00:00Z"
        ))));
        assert!(invalid(one(ConfigObject::new(
            "y",
            ConfigType::ByteString,
            "abc"
        ))));
        assert!(invalid(one(ConfigObject::new(
            "n",
            ConfigType::Number,
            "01"
        ))));
        assert!(!invalid(one(ConfigObject::new(
            "y",
            ConfigType::ByteString,
            "YWJj"
        ))));
        assert!(!invalid(
            one(ConfigObject::string("s", "äöü").max_length(3))
        ));
        assert!(!invalid(one(ConfigObject::new(
            "n",
            ConfigType::Number,
            ""
        ))));
    }

    #[test]
    fn names() {
        for ok in ["a", "_", "eth0", "ip_address", "a-b", "Z9"] {
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

    /// The resolved Config validates against the Config DataSet schema.
    #[cfg(feature = "strict-validation")]
    #[test]
    fn config_matches_schema() {
        use oec_core::schema::{Flavor, Validator};
        let validator = Validator::new().expect("embedded schemas compile");
        let entry = validator
            .schema("resources/config.schema.json#/$defs/Config")
            .expect("Config schema embedded");
        let doc = serde_json::to_value(&def().values().unwrap().config).unwrap();
        validator.validate(&entry, Flavor::Strict, &doc).unwrap();
    }
}
