//! Consumer side: subscriptions, requests with correlation, discovery.

use std::time::Duration;

use crate::data::Value;
use crate::identity::IdentificationLink;
use crate::status::StatusCode;
use crate::{Result, ServiceType};

/// OI4 resources (REQ-TOP-012).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Resource {
    Mam,
    Health,
    Config,
    Data,
    Metadata,
    Event,
    Profile,
    PublicationList,
    SubscriptionList,
    ReferenceDesignation,
}

/// Addressed application/asset; `None` parts become the `+` wildcard in
/// subscriptions (REQ-IL-021). Encoding of AppId/Source is internal.
#[derive(Debug, Clone, Default)]
pub struct Target {
    pub service_type: Option<ServiceType>,
    pub app: Option<IdentificationLink>,
    pub source: Option<IdentificationLink>,
    pub filter: Option<String>,
}

impl Target {
    /// Everything on the bus (`Oi4v2/+/+/Pub/<Resource>/#`).
    pub fn any() -> Self {
        Target::default()
    }
    pub fn app(service_type: ServiceType, app: IdentificationLink) -> Self {
        unimplemented!()
    }
    pub fn source(self, source: IdentificationLink) -> Self {
        unimplemented!()
    }
    pub fn filter(self, filter: &str) -> Self {
        unimplemented!()
    }
}

/// One received DataSetMessage with its topic context. Unknown keys were
/// ignored (REQ-MSG-046).
#[derive(Debug, Clone)]
pub struct Received {
    pub service_type: ServiceType,
    pub app: IdentificationLink,
    pub resource: Resource,
    pub source: Option<IdentificationLink>,
    pub filter: Option<String>,
    pub data_set_writer_id: u16,
    /// Decoded payload fields.
    pub fields: Vec<(String, Value)>,
    /// The payload as received (escape hatch for typed decoding).
    pub payload_json: String,
}

/// Presence of applications and devices from Birth, Close and Will
/// (MG 6.3, REQ-LIFE-*).
#[derive(Debug, Clone)]
pub enum Presence {
    Online { app: IdentificationLink, mam_json: String },
    OfflineGraceful { app: IdentificationLink },
    OfflineUngraceful { app: IdentificationLink },
}

/// Options of a `Get`.
#[derive(Debug, Clone)]
pub struct GetOptions {
    /// Default 10 s.
    pub timeout: Duration,
    /// PaginationRequest `PerPage` (REQ-PAG-001/002); pages are fetched and
    /// reassembled by the SDK.
    pub per_page: Option<u32>,
    /// Locale helper (MG 8.3.12).
    pub locale: Option<String>,
}

/// A subscription; it is an entry of the own SubscriptionList
/// (REQ-RES-SUBLIST-001/002) and ends when dropped.
pub struct Subscription {
    _private: (),
}

impl Subscription {
    /// `Config = CONF_1`: consumers may change or delete the entry via
    /// `Set`/`Del SubscriptionList` (REQ-RES-SUBLIST-004); the SDK
    /// re-subscribes and republishes the list (REQ-RES-SUBLIST-005).
    pub fn configurable(self) -> Self {
        unimplemented!()
    }
    /// Processing interval of the subscriber (REQ-RES-SUBLIST-003).
    pub fn interval(self, interval: Duration) -> Self {
        unimplemented!()
    }
}

/// Consumer API of an application (from [`crate::app::AppHandle::consumer`]).
#[derive(Clone)]
pub struct Consumer {
    _private: (),
}

impl Consumer {
    /// Subscribes to publications of a resource.
    pub fn subscribe(
        &self,
        resource: Resource,
        target: Target,
        f: impl Fn(&Received) + Send + Sync + 'static,
    ) -> Result<Subscription> {
        unimplemented!()
    }

    /// Online/offline of other participants.
    pub fn on_presence(&self, f: impl Fn(&Presence) + Send + Sync + 'static) -> Result<Subscription> {
        unimplemented!()
    }

    /// Sends a `Get` and collects every answer with `ReplyTo` = its
    /// MessageId (REQ-MSG-015, REQ-BUS-001), all pages included
    /// (REQ-PAG-002, REQ-PAG-006). An empty list is a valid answer
    /// (REQ-CMP-041); a Status event Bad ends the request (REQ-CMP-047).
    pub async fn get(&self, resource: Resource, target: Target, options: GetOptions) -> Result<Vec<Received>> {
        unimplemented!()
    }

    pub fn get_blocking(&self, resource: Resource, target: Target, options: GetOptions) -> Result<Vec<Received>> {
        unimplemented!()
    }

    /// Sends a `Set` and waits for the Status event with matching `ReplyTo`
    /// (REQ-RES-EVENT-005). `payload_json` is the DataSetMessage payload.
    pub async fn set(&self, resource: Resource, target: Target, payload_json: &str, timeout: Duration) -> Result<StatusCode> {
        unimplemented!()
    }

    pub async fn del(&self, resource: Resource, target: Target, timeout: Duration) -> Result<StatusCode> {
        unimplemented!()
    }

    /// Common service `NewDataSetWriterId` (`Call`/`Reply`, MG 9.2.1):
    /// returns the DataSetWriterId and its Ttl (REQ-ACT-023); TimeoutHint
    /// and correlation are handled by the SDK (REQ-ACT-009, REQ-ACT-011).
    pub async fn new_data_set_writer_id(
        &self,
        service_type: ServiceType,
        app: IdentificationLink,
        resource: Resource,
        timeout: Duration,
    ) -> Result<(u16, Duration)> {
        unimplemented!()
    }
}
