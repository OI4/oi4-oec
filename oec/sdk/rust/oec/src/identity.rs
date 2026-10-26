//! Identity: Identification Link, Oi4Identifier and the product description
//! that becomes the MAM (MG 4, MG 5). Wraps `oec_core::il`.

use core::fmt;
use core::str::FromStr;

use crate::{Result, ServiceType};

/// OPC UA LocalizedText (REQ-MSG-040). `&str` and `String` convert to locale
/// `en-US` (REQ-RES-MAM-024: the Model used for the Oi4Identifier exists in
/// en-US).
pub use oec_core::message::LocalizedText;

/// The MAM DataSet (MG 5, REQ-RES-MAM-001) as resolved by
/// [`crate::app::AppBuilder::build`] ([`crate::app::App::mam`]).
pub use oec_core::resource::Mam;

/// A validated Identification Link (IEC 61406-1, MG 4.1).
///
/// Validated on construction (REQ-IL-008, REQ-IL-009, REQ-IL-016,
/// REQ-IL-018, REQ-IL-019), never altered or normalized afterwards
/// (REQ-IL-013), compared character by character (REQ-IL-015, derived
/// `PartialEq`). Encoding for topics (base64url without padding, REQ-IL-002
/// … 004) and the unencoded payload form (REQ-IL-005) are internal.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct IdentificationLink(pub(crate) oec_core::il::IdentificationLink);

impl IdentificationLink {
    /// Parses and validates an existing Identification Link, e.g. from a
    /// nameplate or an OPC UA ProductInstanceUri (REQ-IL-006, REQ-IL-031).
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] with the violations and their stable codes.
    pub fn parse(il: &str) -> Result<Self> {
        Ok(Self(oec_core::il::IdentificationLink::parse(il)?))
    }

    /// The Identification Link exactly as assigned (REQ-IL-005, REQ-IL-013).
    pub fn as_str(&self) -> &str {
        self.0.as_str()
    }
}

impl fmt::Display for IdentificationLink {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

impl FromStr for IdentificationLink {
    type Err = crate::Error;

    fn from_str(s: &str) -> Result<Self> {
        Self::parse(s)
    }
}

impl AsRef<str> for IdentificationLink {
    fn as_ref(&self) -> &str {
        self.as_str()
    }
}

/// Builder of an Oi4Identifier `<ManufacturerUri>/<Model>/<ProductCode>/<SerialNumber>`
/// (MG 4.3, REQ-IL-007, REQ-IL-022 … REQ-IL-034). Model, ProductCode and
/// SerialNumber are always dnp-encoded (REQ-IL-033, SF-104).
#[derive(Debug, Clone)]
pub struct Oi4Identifier(oec_core::il::Oi4Identifier);

impl Oi4Identifier {
    /// `manufacturer_uri` is lower-cased (REQ-IL-023); missing elements
    /// become `"nd"` (REQ-IL-028).
    pub fn new(manufacturer_uri: &str, model: &str, product_code: &str) -> Self {
        Self(oec_core::il::Oi4Identifier::new(
            manufacturer_uri,
            model,
            product_code,
        ))
    }

    /// The SerialNumber element (REQ-IL-022); missing → `"nd"` (REQ-IL-028).
    #[must_use]
    pub fn serial_number(self, serial_number: &str) -> Self {
        Self(self.0.serial_number(serial_number))
    }

    /// Builds the Identification Link; fails above 255 characters including
    /// the masking characters (REQ-IL-029) or with an invalid
    /// ManufacturerUri (REQ-IL-024).
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] with the violations and their stable codes.
    pub fn build(&self) -> Result<IdentificationLink> {
        Ok(IdentificationLink(self.0.build()?))
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
/// (REQ-RES-MAM-018).
#[derive(Debug, Clone)]
pub struct Product {
    manufacturer_uri: String,
    model: LocalizedText,
    code: String,
    manufacturer: Option<LocalizedText>,
    description: Option<LocalizedText>,
    hardware_revision: String,
    software_revision: String,
    device_revision: String,
    device_manual: String,
    device_class: Option<String>,
    global_asset_id: Option<String>,
}

impl Product {
    /// The three type-level elements of the Oi4Identifier and the MAM
    /// (REQ-IL-022, REQ-IL-024, REQ-RES-MAM-004, REQ-RES-MAM-007).
    /// Defaults for an application: `Manufacturer` = `manufacturer_uri`,
    /// `HardwareRevision` = `""` (software-only asset, REQ-RES-MAM-008),
    /// `SoftwareRevision` = `"undefined"` (REQ-RES-MAM-010), `DeviceRevision`
    /// = `""` (as in the MG 9.1.1/B1.1.1 examples; MG 5 gives no default),
    /// `DeviceManual` = `""` (REQ-RES-MAM-012), `DeviceClass` =
    /// `Oi4.<ServiceType>` (REQ-RES-MAM-014).
    pub fn new(
        manufacturer_uri: &str,
        model: impl Into<LocalizedText>,
        product_code: &str,
    ) -> Self {
        Self {
            manufacturer_uri: manufacturer_uri.to_owned(),
            model: model.into(),
            code: product_code.to_owned(),
            manufacturer: None,
            description: None,
            hardware_revision: String::new(),
            software_revision: "undefined".to_owned(),
            device_revision: String::new(),
            device_manual: String::new(),
            device_class: None,
            global_asset_id: None,
        }
    }

    /// `Manufacturer` (REQ-RES-MAM-002).
    #[must_use]
    pub fn manufacturer(mut self, name: impl Into<LocalizedText>) -> Self {
        self.manufacturer = Some(name.into());
        self
    }

    /// `Description` (optional).
    #[must_use]
    pub fn description(mut self, text: impl Into<LocalizedText>) -> Self {
        self.description = Some(text.into());
        self
    }

    /// `HardwareRevision` (REQ-RES-MAM-008).
    #[must_use]
    pub fn hardware_revision(mut self, revision: &str) -> Self {
        revision.clone_into(&mut self.hardware_revision);
        self
    }

    /// Typically `env!("CARGO_PKG_VERSION")` (REQ-RES-MAM-009/010).
    #[must_use]
    pub fn software_revision(mut self, revision: &str) -> Self {
        revision.clone_into(&mut self.software_revision);
        self
    }

    /// `DeviceRevision` (REQ-RES-MAM-011).
    #[must_use]
    pub fn device_revision(mut self, revision: &str) -> Self {
        revision.clone_into(&mut self.device_revision);
        self
    }

    /// URL of a human-readable manual (REQ-RES-MAM-012, REQ-RES-MAM-013):
    /// scheme and host, no local file path; checked by
    /// [`crate::app::AppBuilder::build`].
    #[must_use]
    pub fn device_manual(mut self, url: &str) -> Self {
        url.clone_into(&mut self.device_manual);
        self
    }

    /// Overrides the derived DeviceClass (devices; applications keep
    /// `Oi4.<ServiceType>`, REQ-RES-MAM-014).
    #[must_use]
    pub fn device_class(mut self, class: &str) -> Self {
        self.device_class = Some(class.to_owned());
        self
    }

    /// `GlobalAssetId` (optional).
    #[must_use]
    pub fn global_asset_id(mut self, id: &str) -> Self {
        self.global_asset_id = Some(id.to_owned());
        self
    }

    /// The Oi4Identifier of this product with `serial_number` (REQ-IL-007,
    /// REQ-RES-MAM-006): the Model in `en-US` (REQ-RES-MAM-024, REQ-IL-027).
    pub(crate) fn oi4_identifier(&self, serial_number: &str) -> Result<IdentificationLink> {
        if self.model.locale != "en-US" {
            return Err(crate::Error::Invalid(format!(
                "the Model used for the Oi4Identifier is in locale {:?}, not \"en-US\" \
                 (REQ-RES-MAM-024, REQ-IL-027)",
                self.model.locale
            )));
        }
        Oi4Identifier::new(&self.manufacturer_uri, &self.model.text, &self.code)
            .serial_number(serial_number)
            .build()
    }

    /// The MAM of the application (REQ-RES-MAM-001 … 024).
    pub(crate) fn mam(
        &self,
        service_type: ServiceType,
        serial_number: &str,
        il: &IdentificationLink,
        revision_counter: i32,
    ) -> Result<Mam> {
        let manufacturer_uri = self.manufacturer_uri.to_lowercase();
        if !oec_core::il::valid_manufacturer_uri(&manufacturer_uri) {
            return Err(crate::Error::Invalid(format!(
                "ManufacturerUri {:?} is no lower-case domain name without scheme, port, \
                 path and \"www.\" (REQ-RES-MAM-004, REQ-IL-024)",
                self.manufacturer_uri
            )));
        }
        if !valid_device_manual(&self.device_manual) {
            return Err(crate::Error::Invalid(format!(
                "DeviceManual {:?} is neither empty nor a URL with scheme and host \
                 (REQ-RES-MAM-012)",
                self.device_manual
            )));
        }
        Ok(Mam {
            manufacturer: self
                .manufacturer
                .clone()
                .unwrap_or_else(|| LocalizedText::from(manufacturer_uri.as_str())),
            manufacturer_uri,
            model: self.model.clone(),
            product_code: self.code.clone(),
            hardware_revision: self.hardware_revision.clone(),
            software_revision: self.software_revision.clone(),
            device_revision: self.device_revision.clone(),
            device_manual: self.device_manual.clone(),
            device_class: self
                .device_class
                .clone()
                .unwrap_or_else(|| format!("Oi4.{service_type}")),
            serial_number: serial_number.to_owned(),
            product_instance_uri: il.as_str().to_owned(),
            revision_counter,
            description: self.description.clone(),
            global_asset_id: self.global_asset_id.clone(),
        })
    }
}

/// Empty or `<scheme>://[<userinfo>@]<host>[/?#…]`, the pattern of the MAM
/// schema for REQ-RES-MAM-012.
fn valid_device_manual(s: &str) -> bool {
    if s.is_empty() {
        return true;
    }
    let Some((scheme, rest)) = s.split_once("://") else {
        return false;
    };
    let scheme_ok = scheme
        .bytes()
        .next()
        .is_some_and(|c| c.is_ascii_alphabetic())
        && scheme
            .bytes()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, b'+' | b'.' | b'-'));
    let authority = rest.split(['/', '?', '#']).next().unwrap_or_default();
    let host = match authority.split_once('@') {
        Some((_, host)) => host,
        None => authority,
    };
    scheme_ok && !host.is_empty() && !host.contains('@')
}

#[cfg(test)]
mod tests {
    use super::{IdentificationLink, Product, valid_device_manual};
    use crate::ServiceType;

    #[test]
    fn device_manual() {
        for ok in [
            "",
            "https://example.com/manual.pdf",
            "https://user@example.com",
            "ftp://h?x",
            "a+b.c-d://h#f",
        ] {
            assert!(valid_device_manual(ok), "{ok:?}");
        }
        for bad in [
            "/usr/share/doc/manual.pdf",
            "file:///manual.pdf",
            "https://",
            "https://a@b@c",
            "1http://h",
            "manual.pdf",
        ] {
            assert!(!valid_device_manual(bad), "{bad:?}");
        }
    }

    #[test]
    fn mam_defaults() {
        let p = Product::new("Example.com", "Hello OEC", "HELLO-1");
        let il = p.oi4_identifier("hello-1").unwrap();
        assert_eq!(il.as_str(), "example.com/Hello,20OEC/HELLO-1/hello-1");
        let mam = p.mam(ServiceType::Utility, "hello-1", &il, 3).unwrap();
        assert_eq!(mam.manufacturer.text, "example.com");
        assert_eq!(mam.manufacturer.locale, "en-US");
        assert_eq!(mam.manufacturer_uri, "example.com");
        assert_eq!(mam.software_revision, "undefined");
        assert_eq!(mam.device_class, "Oi4.Utility");
        assert_eq!(mam.product_instance_uri, il.as_str());
        assert_eq!(mam.revision_counter, 3);
    }

    #[test]
    fn model_locale_for_the_oi4_identifier() {
        let p = Product::new(
            "example.com",
            super::LocalizedText::new("de-DE", "Wetter"),
            "W",
        );
        assert!(matches!(
            p.oi4_identifier("x"),
            Err(crate::Error::Invalid(_))
        ));
        // an explicit Identification Link does not need the en-US Model
        let il = IdentificationLink::parse("https://example.com/w/1").unwrap();
        assert!(p.mam(ServiceType::Utility, "1", &il, 0).is_ok());
    }

    #[test]
    fn invalid_il_carries_codes() {
        let e = IdentificationLink::parse("").unwrap_err().to_string();
        assert!(e.contains("[empty]"), "{e}");
    }
}
