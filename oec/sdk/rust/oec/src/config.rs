//! Config resource (MG 8.3.3): definition by the owner, the validated
//! values with string-keyed typed getters (ADR-0020 §10, Q-022) and the
//! [`ConfigHandle`] of a Config DataSet (ADR-0026 §15).
//!
//! The SDK publishes the Config masked (REQ-RES-CONFIG-024, GL-ADR-021),
//! answers `Get Config` and checks every `Set Config` before the
//! application decides about it ([`ConfigHandle::on_set`]); accepted
//! values are applied at once, persisted and published again
//! (REQ-RES-CONFIG-019 … 025). Values are never logged.

use std::collections::BTreeMap;
use std::fmt;

use oec_core::message::LocalizedText;
use oec_core::provider::ConfigSpec;
use oec_core::provider::config::check_definition;
use oec_core::resource::{self, Config, Group, SENSITIVE_MASK, Validation};

use crate::app::AppHandle;
use crate::identity::IdentificationLink;
use crate::status::StatusCode;
use crate::{Error, Result};

/// The five Config types (REQ-RES-CONFIG-006); values are strings on the
/// bus (REQ-RES-CONFIG-007) and typed in the API.
pub use oec_core::resource::ConfigType;

/// Definition of one configuration object. Names follow
/// `^[A-Za-z_][A-Za-z0-9_-]*$` (REQ-RES-CONFIG-004), checked by
/// [`ConfigDef::values`] and [`crate::asset::Asset::add_config`].
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

    /// `Validation.Pattern` (REQ-RES-CONFIG-008): a value matches it
    /// anywhere (unanchored; use `^…$` for the whole value). Syntax of the
    /// Rust `regex` crate (RE2-like, Unicode classes); a Pattern that does
    /// not compile makes the definition invalid.
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
        let number = |v: Option<f64>, key: &str| -> Result<Option<serde_json::Number>> {
            v.map(|f| {
                serde_json::Number::from_f64(f).ok_or_else(|| {
                    Error::Invalid(format!(
                        "{group}/{}: Validation {key} is no number",
                        self.name
                    ))
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
        Ok(resource::ConfigObject {
            kind: self.kind,
            value: self.default.clone(),
            default_value: Some(self.default.clone()),
            unit: self.unit.clone(),
            mandatory: self.mandatory.then_some(true),
            sensitive: self.sensitive.then_some(true),
            name: LocalizedText::from(self.label.as_deref().unwrap_or(&self.name)),
            description: None,
            validation: has_validation.then_some(validation),
        })
    }
}

/// Definition of a Config DataSet; `context` is its Filter
/// (REQ-RES-CONFIG-015). An asset may have several (REQ-RES-CONFIG-001).
#[derive(Debug, Clone)]
pub struct ConfigDef {
    context: String,
    label: Option<String>,
    groups: Vec<(String, String, Vec<ConfigObject>)>,
}

impl ConfigDef {
    /// `context` is the Filter of the topic and the WriterGroupName
    /// (REQ-RES-CONFIG-015, REQ-TOP-022); a `/` in it is topic-encoded
    /// (REQ-TOP-019).
    pub fn new(context: &str) -> Self {
        Self {
            context: context.to_owned(),
            label: None,
            groups: Vec::new(),
        }
    }

    /// Display name of the context (`Context.Name`, LocalizedText in
    /// `en-US`, REQ-RES-CONFIG-002); default: the context name.
    #[must_use]
    pub fn label(mut self, text: &str) -> Self {
        self.label = Some(text.to_owned());
        self
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
    /// Pattern compiles, Min/Max are given for Type Number only with
    /// Min ≤ Max, and every default has the form of its Type
    /// (REQ-RES-CONFIG-007) and satisfies its Validation
    /// (REQ-RES-CONFIG-008).
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] naming the first violation, never a value.
    pub fn values(&self) -> Result<ConfigValues> {
        Ok(ConfigValues::new(
            self.clone().resolve()?.config,
            Vec::new(),
        ))
    }

    /// The checked definition for the provider.
    pub(crate) fn resolve(self) -> Result<ConfigSpec> {
        oec_core::topic::Filter::context_name(&self.context)?;
        let mut groups = BTreeMap::new();
        for (name, label, objects) in &self.groups {
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
        let config = Config {
            context: Some(resource::Context {
                name: LocalizedText::from(self.label.as_deref().unwrap_or(&self.context)),
                description: None,
            }),
            groups,
        };
        check_definition(&config).map_err(Error::Invalid)?;
        Ok(ConfigSpec {
            context: self.context,
            config,
        })
    }
}

/// A consistent snapshot of the current (or proposed) values. Values are in
/// clear text; `Debug` shows sensitive values masked.
#[derive(Clone)]
pub struct ConfigValues {
    config: Config,
    changed: Vec<(String, String)>,
}

impl ConfigValues {
    pub(crate) fn new(config: Config, changed: Vec<(String, String)>) -> Self {
        Self { config, changed }
    }

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
    /// empty for the current values.
    pub fn changed(&self) -> impl Iterator<Item = (&str, &str)> {
        self.changed.iter().map(|(g, n)| (g.as_str(), n.as_str()))
    }
}

impl fmt::Debug for ConfigValues {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let values = self.config.groups.iter().flat_map(|(g, group)| {
            group.objects.iter().map(move |(n, o)| {
                let shown = if o.sensitive == Some(true) && !o.value.is_empty() {
                    SENSITIVE_MASK
                } else {
                    o.value.as_str()
                };
                (format!("{g}/{n}"), shown)
            })
        });
        f.debug_struct("ConfigValues")
            .field("values", &values.collect::<BTreeMap<_, _>>())
            .field("changed", &self.changed)
            .finish()
    }
}

/// The decision about a `Set Config` (see [`ConfigHandle::on_set`]).
pub(crate) type ConfigSetHandler =
    dyn Fn(&ConfigValues) -> core::result::Result<(), StatusCode> + Send + Sync;

/// Handle of a Config DataSet (`Clone + Send + Sync`), usable before and
/// while the application runs.
#[derive(Clone)]
pub struct ConfigHandle {
    pub(crate) handle: AppHandle,
    pub(crate) source: IdentificationLink,
    pub(crate) context: String,
}

impl ConfigHandle {
    /// The current values; empty (every getter `None`) after
    /// [`ConfigHandle::remove`].
    pub fn values(&self) -> ConfigValues {
        self.handle
            .config_values(&self.source, &self.context)
            .unwrap_or_else(|| ConfigValues::new(Config::default(), Vec::new()))
    }

    /// The context name (Filter).
    pub fn context(&self) -> &str {
        &self.context
    }

    /// The `Source`: the application itself or a represented asset.
    pub fn source(&self) -> &IdentificationLink {
        &self.source
    }

    /// Called with the proposed values of a `Set Config` (Level 2 on) that
    /// the SDK has already validated: known groups/objects only
    /// (REQ-RES-CONFIG-005), read-only properties unchanged, mandatory
    /// objects present (REQ-RES-CONFIG-011), Type (REQ-RES-CONFIG-007) and
    /// Validation (REQ-RES-CONFIG-008), masks of sensitive objects
    /// resolved (REQ-RES-CONFIG-025). On `Ok` the SDK applies all values at
    /// once, persists them, increments the RevisionCounter if a value
    /// changed (REQ-RES-MAM-018), publishes the Config with the ReplyTo of
    /// the `Set` (REQ-RES-CONFIG-022) and a Status event Good; on `Err`
    /// nothing is applied (REQ-RES-CONFIG-023) and a Status event with that
    /// code is sent (REQ-RES-CONFIG-021). Without handler the values are
    /// applied. The handler runs outside the SDK's locks; a panic is
    /// answered with `BadInternalError`. A later call replaces the
    /// handler.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] at Level 1 (no `Set` is subscribed) or for a
    /// removed Config DataSet.
    pub fn on_set(
        &self,
        f: impl Fn(&ConfigValues) -> core::result::Result<(), StatusCode> + Send + Sync + 'static,
    ) -> Result<()> {
        self.handle
            .on_set_config(&self.source, &self.context, std::sync::Arc::new(f))
    }

    /// Changes a value from the application side (e.g. a local UI): the
    /// same checks as for a `Set Config`, `""` unsets it; a change is
    /// persisted, increments the RevisionCounter (REQ-RES-MAM-018) and is
    /// published at once. No Status event, no call of the
    /// [`ConfigHandle::on_set`] handler.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] for an unknown object, a value that does not pass
    /// the checks (the message names the object, never the value) or a
    /// removed Config DataSet; then nothing is set. [`Error::Io`] if the
    /// store fails.
    pub fn set(&self, group: &str, name: &str, value: &str) -> Result<()> {
        self.handle.set_config(
            &self.source,
            &self.context,
            &[(group.to_owned(), name.to_owned(), value.to_owned())],
        )
    }

    /// Removes the Config DataSet: it is no longer published or answered
    /// and the PublicationList is published again (REQ-RES-PUBLIST-017).
    /// Its DataSetWriterId stays reserved and its persisted values are
    /// kept for a Config DataSet of the same context added later.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] if it is already removed.
    pub fn remove(self) -> Result<()> {
        self.handle.remove_config(&self.source, &self.context)
    }
}

impl fmt::Debug for ConfigHandle {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ConfigHandle")
            .field("source", &self.source)
            .field("context", &self.context)
            .finish_non_exhaustive()
    }
}

#[cfg(test)]
mod tests {
    use super::{ConfigDef, ConfigObject, ConfigType};
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
        assert!(invalid(one(ConfigObject::string("s", "").pattern("("))));
        assert!(invalid(one(
            ConfigObject::string("s", "x").pattern("^[0-9]+$")
        )));
        assert!(invalid(one(ConfigObject::string("s", "").min(1.0))));
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
        assert!(!invalid(one(
            ConfigObject::string("s", "42").pattern("^[0-9]+$")
        )));
        assert!(!invalid(ConfigDef::new("a/b").label("A B")));
    }

    /// Sensitive values are masked in `Debug`.
    #[test]
    fn debug_masks_sensitive_values() {
        let def = ConfigDef::new("c").group(
            "g",
            "G",
            [
                ConfigObject::string("secret", "s3cr3t").sensitive(),
                ConfigObject::string("plain", "visible"),
            ],
        );
        let shown = format!("{:?}", def.values().unwrap());
        assert!(!shown.contains("s3cr3t"), "{shown}");
        assert!(
            shown.contains("********") && shown.contains("visible"),
            "{shown}"
        );
    }

    /// Error messages never contain a value.
    #[test]
    fn errors_never_show_values() {
        let e = ConfigDef::new("c")
            .group(
                "g",
                "G",
                [ConfigObject::string("s", "s3cr3t").one_of(&["a"])],
            )
            .values()
            .unwrap_err();
        assert!(!e.to_string().contains("s3cr3t"), "{e}");
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
        let doc = serde_json::to_value(&def().resolve().unwrap().config).unwrap();
        validator.validate(&entry, Flavor::Strict, &doc).unwrap();
    }
}
