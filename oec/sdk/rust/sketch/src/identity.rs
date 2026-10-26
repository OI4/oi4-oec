//! Identity: Identification Link, Oi4Identifier and the product description
//! that becomes the MAM (MG 4, MG 5).

use crate::Result;

/// A validated Identification Link (IEC 61406-1, MG 4.1).
///
/// Validated on construction (REQ-IL-008, REQ-IL-009, REQ-IL-016,
/// REQ-IL-018, REQ-IL-019), never altered or normalized afterwards
/// (REQ-IL-013), compared character by character (REQ-IL-015, derived
/// `PartialEq`). Encoding for topics (base64url without padding, REQ-IL-002
/// … 004) and the unencoded payload form (REQ-IL-005) are internal.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct IdentificationLink(String);

impl IdentificationLink {
    /// Parses and validates an existing Identification Link, e.g. from a
    /// nameplate or an OPC UA ProductInstanceUri (REQ-IL-006, REQ-IL-031).
    pub fn parse(il: &str) -> Result<Self> {
        unimplemented!()
    }

    /// The Identification Link exactly as assigned (REQ-IL-005, REQ-IL-013).
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

/// Builder of an Oi4Identifier `<ManufacturerUri>/<Model>/<ProductCode>/<SerialNumber>`
/// (MG 4.3, REQ-IL-007, REQ-IL-022 … REQ-IL-034). Model, ProductCode and
/// SerialNumber are always dnp-encoded (REQ-IL-033, SF-104; the former opt-in
/// `dnp_encoded()` was removed in C-046 addendum 2).
#[derive(Debug, Clone)]
pub struct Oi4Identifier {
    manufacturer_uri: String,
    model: String,
    product_code: String,
    serial_number: Option<String>,
}

impl Oi4Identifier {
    /// `manufacturer_uri` is lower-cased (REQ-IL-023); missing elements
    /// become `"nd"` (REQ-IL-028).
    pub fn new(manufacturer_uri: &str, model: &str, product_code: &str) -> Self {
        unimplemented!()
    }

    pub fn serial_number(self, serial_number: &str) -> Self {
        unimplemented!()
    }

    /// Builds the Identification Link; fails above 255 characters including
    /// the masking characters (REQ-IL-029).
    pub fn build(&self) -> Result<IdentificationLink> {
        unimplemented!()
    }
}

/// OPC UA LocalizedText (REQ-MSG-040). `&str` converts to locale `en-US`
/// (REQ-RES-MAM-024: the Model used for the Oi4Identifier exists in en-US).
#[derive(Debug, Clone)]
pub struct LocalizedText {
    pub locale: String,
    pub text: String,
}

impl From<&str> for LocalizedText {
    fn from(text: &str) -> Self {
        unimplemented!()
    }
}

/// Static description of a product (type level): becomes the MAM of the
/// application and the default for the MAM of its represented assets
/// (REQ-RES-MAM-001 … REQ-RES-MAM-024).
///
/// Fields that depend on the instance – `SerialNumber`, `ProductInstanceUri`
/// and `RevisionCounter` – are filled by the SDK: the serial number is the
/// container name in a container (REQ-CNT-021) or set explicitly
/// ([`crate::app::AppBuilder::serial_number`]); the ProductInstanceUri is the
/// Identification Link (REQ-RES-MAM-016); the RevisionCounter is persisted
/// and incremented on every configuration change (REQ-RES-MAM-018).
#[derive(Debug, Clone)]
pub struct Product {
    manufacturer_uri: String,
    model: LocalizedText,
    product_code: String,
}

impl Product {
    /// The three type-level elements of the Oi4Identifier and the MAM
    /// (REQ-IL-022, REQ-IL-024, REQ-RES-MAM-004, REQ-RES-MAM-007).
    /// Defaults for an application: `Manufacturer` = `manufacturer_uri`,
    /// `HardwareRevision` = `""` (software-only asset, REQ-RES-MAM-008),
    /// `SoftwareRevision` = `"undefined"` (REQ-RES-MAM-010), `DeviceRevision`
    /// = `""` (as in the MG 9.1.1/B1.1.1 examples; MG 5 gives no default),
    /// `DeviceClass` = `Oi4.<ServiceType>` (REQ-RES-MAM-014).
    pub fn new(manufacturer_uri: &str, model: impl Into<LocalizedText>, product_code: &str) -> Self {
        unimplemented!()
    }

    pub fn manufacturer(self, name: impl Into<LocalizedText>) -> Self {
        unimplemented!()
    }

    pub fn description(self, text: impl Into<LocalizedText>) -> Self {
        unimplemented!()
    }

    pub fn hardware_revision(self, revision: &str) -> Self {
        unimplemented!()
    }

    /// Typically `env!("CARGO_PKG_VERSION")` (REQ-RES-MAM-009/010).
    pub fn software_revision(self, revision: &str) -> Self {
        unimplemented!()
    }

    pub fn device_revision(self, revision: &str) -> Self {
        unimplemented!()
    }

    /// URL of a human-readable manual (REQ-RES-MAM-012, REQ-RES-MAM-013).
    pub fn device_manual(self, url: &str) -> Self {
        unimplemented!()
    }

    /// Overrides the derived DeviceClass (devices; applications keep
    /// `Oi4.<ServiceType>`, REQ-RES-MAM-014).
    pub fn device_class(self, class: &str) -> Self {
        unimplemented!()
    }

    pub fn global_asset_id(self, id: &str) -> Self {
        unimplemented!()
    }
}
