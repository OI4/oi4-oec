//! Profile, PublicationList and SubscriptionList (MG 9.1.7 … 9.1.9).

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};
use serde_json::Number;

use crate::message::DataSetWriterId;
use crate::topic::{Resource, Subject};

/// The Profile: the resources and services an application supports
/// (REQ-RES-PROFILE-001).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Profile {
    /// Supported resources and services.
    pub resources: Vec<Subject>,
}

super::data_set!(Profile, "48017c6a-05c8-48d7-9d85-4b08bbb707f3");

/// Publication mode in the VerboseEncoding (REQ-RES-PUBLIST-001); the modes
/// a resource allows are REQ-RES-PUBLIST-012.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[allow(missing_docs)]
pub enum PublicationMode {
    #[serde(rename = "OFF_0")]
    Off,
    #[serde(rename = "ON_REQUEST_1")]
    OnRequest,
    #[serde(rename = "APPLICATION_2")]
    Application,
    #[serde(rename = "SOURCE_3")]
    Source,
    #[serde(rename = "FILTER_4")]
    Filter,
    #[serde(rename = "APPLICATION_SOURCE_5")]
    ApplicationSource,
    #[serde(rename = "APPLICATION_FILTER_6")]
    ApplicationFilter,
    #[serde(rename = "SOURCE_FILTER_7")]
    SourceFilter,
    #[serde(rename = "APPLICATION_SOURCE_FILTER_8")]
    ApplicationSourceFilter,
    #[serde(rename = "ON_REQUEST_FILTER_9")]
    OnRequestFilter,
    #[serde(rename = "ON_REQUEST_SOURCE_10")]
    OnRequestSource,
    #[serde(rename = "ON_REQUEST_APPLICATION_11")]
    OnRequestApplication,
    #[serde(rename = "ON_REQUEST_SOURCE_FILTER_12")]
    OnRequestSourceFilter,
    #[serde(rename = "ON_REQUEST_APPLICATION_FILTER_13")]
    OnRequestApplicationFilter,
    #[serde(rename = "ON_REQUEST_APPLICATION_SOURCE_14")]
    OnRequestApplicationSource,
    #[serde(rename = "ON_REQUEST_APPLICATION_SOURCE_FILTER_15")]
    OnRequestApplicationSourceFilter,
}

/// Which settings of a publication can be configured
/// (REQ-RES-PUBLIST-001).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum PublicationConfig {
    /// `NONE_0`.
    #[serde(rename = "NONE_0")]
    None,
    /// `MODE_1`.
    #[serde(rename = "MODE_1")]
    Mode,
    /// `INTERVAL_2`.
    #[serde(rename = "INTERVAL_2")]
    Interval,
    /// `MODE_AND_INTERVAL_3`.
    #[serde(rename = "MODE_AND_INTERVAL_3")]
    ModeAndInterval,
}

/// One entry of the PublicationList (REQ-RES-PUBLIST-001); whether a
/// Filter is present depends on the resource (REQ-RES-PUBLIST-002).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct PublicationEntry {
    /// The published resource.
    pub resource: Resource,
    /// The asset, an IdentificationLink or empty.
    pub source: String,
    /// The Filter of the resource.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub filter: Option<String>,
    /// The DataSetWriterId of the publication.
    pub data_set_writer_id: DataSetWriterId,
    /// Publication mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<PublicationMode>,
    /// Interval in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interval: Option<u32>,
    /// Deadband per field (number ≥ 0).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub precisions: Option<BTreeMap<String, Number>>,
    /// Configurable settings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub config: Option<PublicationConfig>,
}

super::data_set!(PublicationEntry, "217434d6-6e1e-4230-b907-f52bc9ffe152");

/// Whether a subscription can be configured (REQ-RES-SUBLIST-001).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum SubscriptionConfig {
    /// `NONE_0`.
    #[serde(rename = "NONE_0")]
    None,
    /// `CONF_1`.
    #[serde(rename = "CONF_1")]
    Conf,
}

/// One entry of the SubscriptionList (REQ-RES-SUBLIST-001). The TopicPath
/// is kept as text: it may be a topic filter with wildcards
/// (REQ-RES-SUBLIST-002, [`crate::topic::TopicFilter`]).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct SubscriptionEntry {
    /// The subscribed topic or topic filter.
    pub topic_path: String,
    /// Interval in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interval: Option<u32>,
    /// Configurable.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub config: Option<SubscriptionConfig>,
}

super::data_set!(SubscriptionEntry, "e5d68c47-c276-4929-8ab9-4c1090cac785");
