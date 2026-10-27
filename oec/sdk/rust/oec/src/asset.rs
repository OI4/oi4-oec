//! Assets: the application itself ([`crate::app::AppHandle::this`]) and the
//! entities it represents ([`crate::app::AppHandle::add_asset`]), each a
//! `Source` with the resources MAM, Health and – from Level 2 – Profile and
//! the entries of the PublicationList (P4 increment 6a, ADR-0020 amendment
//! C-053, ADR-0026 §12), and their Data DataSets with Metadata
//! ([`Asset::add_data`], increment 6b, C-054).
//!
//! The SDK publishes the state, not every call (ADR-0020 §7): a change is
//! published at once while connected; offline nothing is queued and the
//! current state of every source is published after the next connect.

use std::fmt;

use oec_core::resource::{DetailObject, Health};

use crate::Result;
use crate::app::AppHandle;
use crate::config::{ConfigDef, ConfigHandle};
use crate::data::{DataSet, DataSetDef};
use crate::identity::{IdentificationLink, Mam, Product};

/// Health states (MG 9.1.2), ordered by severity for the aggregation of
/// the `Details` (REQ-RES-HEALTH-002, [`HealthState::severity`]).
pub use oec_core::resource::HealthState;

/// Definition of a represented entity, e.g. a field device without an MQTT
/// client of its own (REQ-CMP-045); registered by
/// [`AppHandle::add_asset`].
#[derive(Debug, Clone)]
pub struct AssetDef {
    pub(crate) il: Option<IdentificationLink>,
    pub(crate) product: Product,
    pub(crate) serial_number: String,
    pub(crate) revision_counter: Option<i32>,
}

impl AssetDef {
    /// Asset with an existing Identification Link, used unchanged
    /// (REQ-IL-006, REQ-RES-MAM-016).
    pub fn new(il: IdentificationLink, product: Product, serial_number: &str) -> Self {
        Self {
            il: Some(il),
            product,
            serial_number: serial_number.to_owned(),
            revision_counter: None,
        }
    }

    /// Asset without Identification Link: the Oi4Identifier is generated
    /// from the product and the serial number (REQ-IL-007,
    /// REQ-RES-MAM-006).
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] if the Oi4Identifier cannot be built (e.g.
    /// longer than 255 characters, REQ-IL-029, or a Model without `en-US`
    /// text, REQ-RES-MAM-024).
    pub fn with_oi4_identifier(product: Product, serial_number: &str) -> Result<Self> {
        let il = product.oi4_identifier(serial_number)?;
        Ok(Self::new(il, product, serial_number))
    }

    /// RevisionCounter taken unchanged from the VendorNameplate of the
    /// asset, `-1` for an asset without one (REQ-RES-MAM-018). Default: the
    /// SDK manages and persists it, starting at 0
    /// ([`Asset::bump_revision_counter`]).
    #[must_use]
    pub fn revision_counter(mut self, value: i32) -> Self {
        self.revision_counter = Some(value);
        self
    }
}

/// One entry of the Health `Details` (REQ-RES-HEALTH-001).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HealthDetail {
    /// The state of this detail.
    pub health: HealthState,
    /// `DiagnosticCode`.
    pub diagnostic_code: Option<String>,
    /// `Location`.
    pub location: Option<String>,
    /// `Description`.
    pub description: Option<String>,
}

impl HealthDetail {
    /// A detail with only its state.
    pub fn new(health: HealthState) -> Self {
        Self {
            health,
            diagnostic_code: None,
            location: None,
            description: None,
        }
    }
}

impl From<HealthDetail> for DetailObject {
    fn from(d: HealthDetail) -> Self {
        DetailObject {
            health: d.health,
            diagnostic_code: d.diagnostic_code,
            location: d.location,
            description: d.description,
        }
    }
}

/// Handle of an asset (`Clone + Send + Sync`): the application itself or a
/// represented entity. Usable before and while the application runs.
#[derive(Clone)]
pub struct Asset {
    pub(crate) handle: AppHandle,
    pub(crate) il: IdentificationLink,
}

impl Asset {
    /// The Identification Link: AppId or `Source` of the topics
    /// (REQ-IL-001).
    pub fn identification_link(&self) -> &IdentificationLink {
        &self.il
    }

    /// The current MAM (REQ-RES-MAM-001 … 024); `None` after
    /// [`Asset::remove`].
    pub fn mam(&self) -> Option<Mam> {
        self.handle.asset_mam(&self.il)
    }

    /// Whether this is the application itself.
    pub fn is_this(&self) -> bool {
        self.handle.identification_link() == &self.il
    }

    /// Sets Health and HealthScore (0 … 100) and clears the `Details`; a
    /// change is published at once (REQ-TOP-017). The heartbeat every 60 s
    /// of the application itself is automatic (REQ-TOP-017, SF-106).
    ///
    /// An invalid value – a HealthScore above 100 or NORMAL_0 with score 0
    /// for the application itself, which means Close (REQ-LIFE-014) – is
    /// logged and ignored, as is a removed asset.
    pub fn set_health(&self, health: HealthState, score: Option<u8>) {
        self.handle.set_health(
            &self.il,
            Health {
                health,
                health_score: score,
                details: None,
            },
        );
    }

    /// Replaces the `Details`; `Health` then equals the state of the most
    /// severe detail (REQ-RES-HEALTH-002, A-28), the HealthScore is
    /// omitted, and a change is published at once (REQ-RES-HEALTH-004).
    /// No details: NORMAL_0 without `Details`.
    pub fn set_health_details(&self, details: Vec<HealthDetail>) {
        let health = if details.is_empty() {
            Health {
                health: HealthState::Normal,
                health_score: None,
                details: None,
            }
        } else {
            Health::aggregate(details.into_iter().map(DetailObject::from).collect())
        };
        self.handle.set_health(&self.il, health);
    }

    /// Increments the RevisionCounter after an application-side
    /// configuration change, persists it and publishes the MAM
    /// (REQ-RES-MAM-018). A counter taken from the VendorNameplate
    /// ([`AssetDef::revision_counter`]) is not changed (logged).
    ///
    /// # Errors
    ///
    /// [`crate::Error::Io`] if the store fails, [`crate::Error::Invalid`]
    /// for a removed asset or a counter at `i32::MAX`.
    pub fn bump_revision_counter(&self) -> Result<()> {
        self.handle.bump_revision_counter(&self.il)
    }

    /// Declares a Data DataSet of this asset. The SDK assigns its
    /// DataSetWriterId (REQ-MSG-020, REQ-RES-PUBLIST-006), creates its
    /// PublicationList entries for Data and Metadata (REQ-RES-PUBLIST-001
    /// … 015), derives its Metadata (REQ-RES-META-002/003) with a
    /// ConfigurationVersion that is persisted and changes only with the
    /// Metadata (REQ-MSG-032/036), publishes the Metadata unless the Mode
    /// is `OFF` and answers `Get Data` and `Get Metadata`
    /// (REQ-RES-DATA-007, REQ-CMP-044).
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] for an invalid definition (Tag, field
    /// names and types, Oi4Pv fields, Precisions, DataSetClassId), a Tag
    /// already used by this asset, Metadata larger than the MaxPacketSize
    /// or a removed asset; [`crate::Error::Io`] if the store fails.
    pub fn add_data(&self, def: DataSetDef) -> Result<DataSet> {
        self.handle.add_data(&self.il, def)
    }

    /// Declares a Config DataSet (context) of this asset (MG 8.3.3,
    /// REQ-RES-CONFIG-001). The SDK assigns its DataSetWriterId
    /// (REQ-RES-PUBLIST-006), lists it in the PublicationList with Mode
    /// `ON_REQUEST_FILTER_9` and the context as Filter, publishes it masked
    /// on `Pub/Config/<Source>/<Filter>` at once and after every change
    /// (REQ-RES-CONFIG-022/024) and answers `Get Config` and `Set Config`
    /// from Level 2 on (REQ-RES-CONFIG-016 … 023, [`ConfigHandle::on_set`]).
    /// Values set earlier (persisted in the store) replace the defaults if
    /// the object still exists and the value is still valid.
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] for an invalid definition (see
    /// [`ConfigDef::values`]), a context already used by this asset, a
    /// Config larger than the MaxPacketSize or a removed asset;
    /// [`crate::Error::Io`] if the store fails.
    pub fn add_config(&self, def: ConfigDef) -> Result<ConfigHandle> {
        self.handle.add_config(&self.il, def)
    }

    /// Removes a represented asset: its MAM, Health and Profile are no
    /// longer served and the PublicationList is republished without its
    /// entries (REQ-RES-PUBLIST-017), as are its Data DataSets. Its
    /// DataSetWriterIds stay reserved.
    ///
    /// # Errors
    ///
    /// [`crate::Error::Invalid`] for the application itself or an asset
    /// that is already removed.
    pub fn remove(self) -> Result<()> {
        self.handle.remove_asset(&self.il)
    }
}

impl fmt::Debug for Asset {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Asset")
            .field("identification_link", &self.il)
            .finish_non_exhaustive()
    }
}
