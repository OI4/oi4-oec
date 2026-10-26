//! Assets: the application itself and the entities it represents, with their
//! built-in resources MAM, Health and ReferenceDesignation.

use crate::config::{ConfigDef, ConfigHandle};
use crate::data::{DataSet, DataSetDef};
use crate::event::GenericEvent;
use crate::identity::{IdentificationLink, Product};
use crate::status::StatusCode;
use crate::Result;

/// Definition of a represented entity (REQ-CMP-045).
pub struct AssetDef {
    _private: (),
}

impl AssetDef {
    /// Asset with an existing Identification Link (REQ-IL-006).
    pub fn new(il: IdentificationLink, product: Product, serial_number: &str) -> Self {
        unimplemented!()
    }

    /// Asset without Identification Link: the Oi4Identifier is generated
    /// from the product and the serial number (REQ-IL-007, REQ-RES-MAM-006).
    pub fn with_oi4_identifier(product: Product, serial_number: &str) -> Result<Self> {
        unimplemented!()
    }

    /// RevisionCounter taken unchanged from the VendorNameplate of the
    /// asset (REQ-RES-MAM-018); default: managed by the SDK.
    pub fn revision_counter(self, value: i32) -> Self {
        unimplemented!()
    }
}

/// Health states (MG 8.3.2), ordered by severity for the aggregation of
/// `Details` (REQ-RES-HEALTH-002).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HealthState {
    Normal,
    Failure,
    CheckFunction,
    OffSpec,
    MaintenanceRequired,
}

/// One entry of the Health `Details` (REQ-RES-HEALTH-001).
#[derive(Debug, Clone)]
pub struct HealthDetail {
    pub health: HealthState,
    pub diagnostic_code: Option<String>,
    pub location: Option<String>,
    pub description: Option<String>,
}

/// ReferenceDesignation of an asset (MG 8.3.10). Set/Del and persistence
/// are handled by the SDK (REQ-RES-REFDES-002 … 010); the application can
/// observe or veto changes with [`Asset::on_reference_designation`].
#[derive(Debug, Clone, Default)]
pub struct ReferenceDesignation {
    pub function: Option<String>,
    pub product: Option<String>,
    pub location: Option<String>,
}

/// Handle of an asset (`Clone + Send + Sync`).
#[derive(Clone)]
pub struct Asset {
    _private: (),
}

impl Asset {
    pub fn identification_link(&self) -> &IdentificationLink {
        unimplemented!()
    }

    /// Sets Health and HealthScore; publishes on change (REQ-TOP-017).
    /// The heartbeat every 60 s is automatic.
    pub fn set_health(&self, health: HealthState, score: Option<u8>) {
        unimplemented!()
    }

    /// Replaces the `Details`; `Health` then equals the Health with the
    /// highest priority of the Details (REQ-RES-HEALTH-002; ADR-0020
    /// amendment C-049) and a change is published (REQ-RES-HEALTH-004).
    pub fn set_health_details(&self, details: Vec<HealthDetail>) {
        unimplemented!()
    }

    /// Declares a Data DataSet of this asset. The SDK assigns the
    /// DataSetWriterId (REQ-MSG-020, REQ-RES-PUBLIST-006), creates its
    /// PublicationList entry (REQ-RES-PUBLIST-001 … 017), its Metadata
    /// (REQ-RES-META-002/003) and answers `Get Data` and `Get Metadata`
    /// (REQ-RES-DATA-007, REQ-RES-META-002).
    pub fn add_data(&self, def: DataSetDef) -> Result<DataSet> {
        unimplemented!()
    }

    /// Declares a Config DataSet (context) of this asset (MG 8.3.3).
    pub fn add_config(&self, def: ConfigDef) -> Result<ConfigHandle> {
        unimplemented!()
    }

    /// Publishes a Generic event (`CAT_GENERIC_99`, REQ-RES-EVENT-009).
    pub fn emit_event(&self, event: GenericEvent) {
        unimplemented!()
    }

    /// Increments the RevisionCounter after an application-side
    /// configuration change (REQ-RES-MAM-018). Config and MAM changes made
    /// through the SDK increment it automatically.
    pub fn bump_revision_counter(&self) {
        unimplemented!()
    }

    /// Called before a `Set`/`Del ReferenceDesignation` is applied. `None`
    /// means `Del`. Returning an error rejects it with a Status event Bad.
    pub fn on_reference_designation(
        &self,
        f: impl Fn(Option<&ReferenceDesignation>) -> Result<(), StatusCode> + Send + Sync + 'static,
    ) {
        unimplemented!()
    }

    /// Removes a represented asset; its PublicationList entries disappear
    /// and the PublicationList is republished (REQ-RES-PUBLIST-017).
    pub fn remove(self) -> Result<()> {
        unimplemented!()
    }
}
