//! MAM, the Master Asset Model (MG 9.1.1).

use serde::{Deserialize, Serialize};

use crate::message::LocalizedText;

/// The MAM of an asset (REQ-RES-MAM-001); all fields but Description and
/// GlobalAssetId are mandatory and always sent (REQ-MSG-034).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Mam {
    /// Manufacturer.
    pub manufacturer: LocalizedText,
    /// Domain name of the manufacturer (REQ-RES-MAM-004).
    pub manufacturer_uri: String,
    /// Model.
    pub model: LocalizedText,
    /// ProductCode.
    pub product_code: String,
    /// HardwareRevision.
    pub hardware_revision: String,
    /// SoftwareRevision.
    pub software_revision: String,
    /// DeviceRevision.
    pub device_revision: String,
    /// DeviceManual: an absolute URI or empty (REQ-RES-MAM-012).
    pub device_manual: String,
    /// DeviceClass.
    pub device_class: String,
    /// SerialNumber.
    pub serial_number: String,
    /// ProductInstanceUri, at most 255 characters (REQ-RES-MAM-017).
    pub product_instance_uri: String,
    /// RevisionCounter.
    pub revision_counter: i32,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
    /// GlobalAssetId.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub global_asset_id: Option<String>,
}

super::data_set!(Mam, "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58");
