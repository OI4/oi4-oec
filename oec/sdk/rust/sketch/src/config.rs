//! Config resource (MG 8.3.3): definition by the owner, validation and
//! atomic application of `Set Config` by the SDK.

use crate::status::StatusCode;
use crate::Result;

/// The five Config types (REQ-RES-CONFIG-006); values are strings on the
/// bus (REQ-RES-CONFIG-007) and typed in the API.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConfigType {
    Boolean,
    ByteString,
    DateTime,
    Number,
    String,
}

/// Definition of one configuration object. Names follow
/// `^[A-Za-z_][A-Za-z0-9_-]*$` (REQ-RES-CONFIG-004), checked on build.
#[derive(Debug, Clone)]
pub struct ConfigObject {
    name: String,
}

impl ConfigObject {
    pub fn new(name: &str, ty: ConfigType, default: &str) -> Self {
        unimplemented!()
    }
    pub fn number(name: &str, default: f64) -> Self {
        unimplemented!()
    }
    pub fn string(name: &str, default: &str) -> Self {
        unimplemented!()
    }
    pub fn boolean(name: &str, default: bool) -> Self {
        unimplemented!()
    }
    /// Display name (`Name`, LocalizedText).
    pub fn label(self, text: &str) -> Self {
        unimplemented!()
    }
    pub fn unit(self, unit: &str) -> Self {
        unimplemented!()
    }
    /// `Mandatory` (REQ-RES-CONFIG-011).
    pub fn mandatory(self) -> Self {
        unimplemented!()
    }
    /// `Sensitive`: masked as `********` in every `Pub`
    /// (REQ-RES-CONFIG-024); the mask in a `Set` keeps the value
    /// (REQ-RES-CONFIG-025).
    pub fn sensitive(self) -> Self {
        unimplemented!()
    }
    /// `Validation` (REQ-RES-CONFIG-008).
    pub fn min(self, min: f64) -> Self {
        unimplemented!()
    }
    pub fn max(self, max: f64) -> Self {
        unimplemented!()
    }
    pub fn max_length(self, length: u32) -> Self {
        unimplemented!()
    }
    pub fn pattern(self, regex: &str) -> Self {
        unimplemented!()
    }
    pub fn one_of(self, values: &[&str]) -> Self {
        unimplemented!()
    }
}

/// Definition of a Config DataSet; `context` is its Filter
/// (REQ-RES-CONFIG-015). An asset may have several (REQ-RES-CONFIG-001).
#[derive(Debug, Clone)]
pub struct ConfigDef {
    context: String,
}

impl ConfigDef {
    pub fn new(context: &str) -> Self {
        unimplemented!()
    }

    /// Display name of the context (`Context.Name`, REQ-RES-CONFIG-002);
    /// default: the context name (amendment C-056).
    pub fn label(self, text: &str) -> Self {
        unimplemented!()
    }

    /// Adds a group (REQ-RES-CONFIG-013); the group of common objects is
    /// named `common` by convention (REQ-RES-CONFIG-014).
    pub fn group(self, name: &str, label: &str, objects: impl IntoIterator<Item = ConfigObject>) -> Self {
        unimplemented!()
    }
}

/// A consistent snapshot of the current (or proposed) values.
#[derive(Debug, Clone)]
pub struct ConfigValues {
    _private: (),
}

impl ConfigValues {
    /// `None` if unset (empty string, REQ-RES-CONFIG-010).
    pub fn number(&self, group: &str, name: &str) -> Option<f64> {
        unimplemented!()
    }
    pub fn string(&self, group: &str, name: &str) -> Option<&str> {
        unimplemented!()
    }
    pub fn boolean(&self, group: &str, name: &str) -> Option<bool> {
        unimplemented!()
    }
    /// Names of the objects that differ from the current values.
    pub fn changed(&self) -> impl Iterator<Item = (&str, &str)> {
        std::iter::empty()
    }
}

/// Handle of a Config DataSet (`Clone + Send + Sync`).
#[derive(Clone)]
pub struct ConfigHandle {
    _private: (),
}

impl ConfigHandle {
    pub fn values(&self) -> ConfigValues {
        unimplemented!()
    }

    /// Called with the proposed values of a `Set Config` that the SDK has
    /// already validated: known groups/objects only (REQ-RES-CONFIG-005),
    /// read-only properties unchanged, mandatory objects present
    /// (REQ-RES-CONFIG-011), Type and Validation (REQ-RES-CONFIG-008),
    /// masks of sensitive objects resolved (REQ-RES-CONFIG-025). On `Ok` the
    /// SDK applies all values at once, persists them, increments the
    /// RevisionCounter (REQ-RES-MAM-018), publishes the changed Config
    /// (REQ-RES-CONFIG-022) and a Status event Good; on `Err` nothing is
    /// applied (REQ-RES-CONFIG-023) and a Status event Bad is sent
    /// (REQ-RES-CONFIG-021). Without handler the values are applied.
    /// Returns an error at Level 1 (no `Set` subscribed) and for a removed
    /// Config (amendment C-056, as `DataSet::on_set`).
    pub fn on_set(
        &self,
        f: impl Fn(&ConfigValues) -> core::result::Result<(), StatusCode> + Send + Sync + 'static,
    ) -> Result<()> {
        unimplemented!()
    }

    /// Changes values from the application side (e.g. a local UI); same
    /// validation and publication, no Status event.
    pub fn set(&self, group: &str, name: &str, value: &str) -> Result<()> {
        unimplemented!()
    }

    /// Removes the Config DataSet (REQ-RES-PUBLIST-017); its persisted
    /// values are kept (amendment C-056).
    pub fn remove(self) -> Result<()> {
        unimplemented!()
    }
}
