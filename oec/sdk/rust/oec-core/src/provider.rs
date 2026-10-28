//! The provider side of an application as a state machine (MG 9.1; P4
//! increments 6a and 6b, ADR-0026 §12 and §13): the sources it serves
//! (itself and its subordinate assets), their MAM, Health, Profile and
//! Data DataSets with Metadata, the PublicationList with stable
//! DataSetWriterIds, the publish modes, Intervals and Precisions of the
//! Data, the answers to `Get` and pagination; and the write side (P4
//! increment 7a, ADR-0026 §14): `Set` and `Del` with Status events,
//! `Set PublicationList` and `Set Data`; and the Config DataSets (P4
//! increment 7b, ADR-0026 §15) with `Get` and `Set Config`.
//!
//! It owns the [`Lifecycle`]. The runtime calls [`Provider::on_connected`]
//! after CONNACK, [`Provider::poll`] at [`Provider::poll_timeout`] and on
//! every change, and [`Provider::handle_message`] for every message of
//! [`Provider::subscriptions`]; it publishes what they return in order.
//!
//! Offline nothing is queued (ADR-0020 §7, state instead of backlog):
//! changes only mark the state, and [`Provider::on_connected`] publishes
//! the current state of every source.

mod action;
pub mod config;
pub mod data;
mod events;
pub mod overrides;
pub mod pagination;
mod refdes;
mod sets;
pub mod sublist;
pub mod versions;
mod write;
pub mod writers;

use core::fmt;
use core::time::Duration;

use serde::Deserialize;
use serde_json::Value;

use crate::il::IdentificationLink;
use crate::lifecycle::{Lifecycle, LifecycleError, Publication, check_health};
use crate::message::{
    DataSet, DataSetMessage, DataSetMetaData, DataSetWriterId, MessageId, MetaDataNetworkMessage,
    PaginationRequest, PublisherId, Query,
};
use crate::resource::{
    Data, Health, HealthState, Mam, Profile, PublicationEntry, PublicationMode,
    ReferenceDesignationQuery,
};
use crate::topic::{
    Filter, ListFilter, Method, Resource, Service, StatusLevel, Subject, Topic, TopicError,
    TopicFilter,
};
pub use action::WRITER_ID_TTL_SECONDS;
use config::ConfigEntry;
pub use config::{ConfigSpec, SetConfigRequest, StoredConfigValues};
use data::{DataSetSpec, Value as FieldValue};
pub use events::{
    GenericEvent, MAX_GENERIC_LEVELS, MAX_QUEUED_EVENTS, syslog_header, syslog_hostname, syslog_tag,
};
pub use overrides::{PublicationOverride, PublicationOverrides};
use pagination::Envelope;
pub use refdes::{ReferenceDesignationRequest, StoredReferenceDesignations};
use sets::{Set, date_time};
pub use sublist::{ConsumerSubscription, SubscriptionOverrides, SubscriptionSetting};
pub use versions::{MetaDataVersions, VersionsError};
pub use write::SetDataRequest;
pub use writers::{WriterIdError, WriterIds, WriterKey};

/// The settings of a [`Provider`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ProviderConfig {
    /// Answer `Get` and serve Profile and PublicationList: Level 2 and
    /// higher (ADR-0020 §3).
    pub get: bool,
    /// The largest payload of an answer to `Get`: the MaxPacketSize of the
    /// Message Bus (REQ-PAG-005, SF-105); larger answers are paginated.
    pub max_packet_size: usize,
    /// The largest payload of a publication not triggered by `Get`, at most
    /// `max_packet_size`; larger publications are paginated (REQ-PAG-007).
    pub publish_limit: usize,
}

/// A change the provider refuses.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum ProviderError {
    /// A value of the application itself is refused.
    Lifecycle(LifecycleError),
    /// No DataSetWriterId can be assigned.
    WriterId(WriterIdError),
    /// An asset with this Identification Link is already registered.
    DuplicateAsset(String),
    /// The application itself is not an asset of its own.
    SelfAsset,
    /// No asset with this Identification Link is registered.
    UnknownAsset(String),
    /// The ProductInstanceUri of a MAM is no Identification Link or not
    /// the one of the asset.
    ProductInstanceUri(String),
    /// A topic can not be built.
    Topic(TopicError),
    /// The definition of a Data DataSet is refused.
    DataSet(String),
    /// The source already has a Data DataSet with this Tag
    /// (REQ-RES-PUBLIST-003).
    DuplicateDataSet {
        /// The source.
        source: String,
        /// The Tag.
        tag: String,
    },
    /// The source has no Data DataSet with this Tag.
    UnknownDataSet {
        /// The source.
        source: String,
        /// The Tag.
        tag: String,
    },
    /// A field or configuration value is refused: unknown field or
    /// object, wrong type or Validation.
    Value(String),
    /// The definition of a Config DataSet is refused.
    Config(String),
    /// The source already has a Config DataSet with this context name
    /// (REQ-RES-CONFIG-015).
    DuplicateConfig {
        /// The source.
        source: String,
        /// The context name.
        context: String,
    },
    /// The source has no Config DataSet with this context name.
    UnknownConfig {
        /// The source.
        source: String,
        /// The context name.
        context: String,
    },
    /// An event is refused.
    Event(String),
}

impl fmt::Display for ProviderError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Lifecycle(e) => e.fmt(f),
            Self::WriterId(e) => e.fmt(f),
            Self::DuplicateAsset(il) => write!(f, "the asset {il:?} is already registered"),
            Self::SelfAsset => f.write_str(
                "the application itself is no subordinate asset; use the application's own handle",
            ),
            Self::UnknownAsset(il) => write!(f, "no asset {il:?} is registered"),
            Self::ProductInstanceUri(e) => write!(f, "invalid ProductInstanceUri: {e}"),
            Self::Topic(e) => e.fmt(f),
            Self::DataSet(e) => write!(f, "invalid Data DataSet: {e}"),
            Self::DuplicateDataSet { source, tag } => {
                write!(f, "the source {source:?} already has a DataSet {tag:?}")
            }
            Self::UnknownDataSet { source, tag } => {
                write!(f, "the source {source:?} has no DataSet {tag:?}")
            }
            Self::Value(e) => write!(f, "invalid value: {e}"),
            Self::Config(e) => write!(f, "invalid Config DataSet: {e}"),
            Self::DuplicateConfig { source, context } => {
                write!(f, "the source {source:?} already has a Config {context:?}")
            }
            Self::UnknownConfig { source, context } => {
                write!(f, "the source {source:?} has no Config {context:?}")
            }
            Self::Event(e) => write!(f, "invalid event: {e}"),
        }
    }
}

impl std::error::Error for ProviderError {}

impl From<LifecycleError> for ProviderError {
    fn from(e: LifecycleError) -> Self {
        Self::Lifecycle(e)
    }
}

impl From<WriterIdError> for ProviderError {
    fn from(e: WriterIdError) -> Self {
        Self::WriterId(e)
    }
}

impl From<TopicError> for ProviderError {
    fn from(e: TopicError) -> Self {
        Self::Topic(e)
    }
}

/// Why [`Provider::handle_message`] does not answer a message.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum Ignored {
    /// The topic is no OEC topic.
    Topic(TopicError),
    /// Not a `Get`, `Set`, `Del` or `Call` to this application.
    NotForUs,
    /// The application does not serve this resource with `Get`, `Set` or
    /// `Del` (or not at Level 1).
    NotServed(Subject),
    /// The payload is no `Get`, `Set` or `Del` NetworkMessage with a valid
    /// MessageId, or no `ua-action-request` this application answers
    /// (REQ-ACT-017).
    Payload(String),
    /// A `Get` of Metadata without Source and Tag or for a DataSet the
    /// application does not provide; silence is conformant (REQ-CMP-044).
    NoMetadata(String),
}

impl fmt::Display for Ignored {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Topic(e) => write!(f, "no OEC topic: {e}"),
            Self::NotForUs => f.write_str("not a Get, Set, Del or Call to this application"),
            Self::NotServed(s) => write!(f, "{s} is not served"),
            Self::Payload(e) => write!(f, "invalid payload: {e}"),
            Self::NoMetadata(e) => write!(f, "Get Metadata not answered (REQ-CMP-044): {e}"),
        }
    }
}

/// The resources served by `Get`.
const SERVED: [Resource; 8] = [
    Resource::Mam,
    Resource::Health,
    Resource::Config,
    Resource::Data,
    Resource::Metadata,
    Resource::Profile,
    Resource::PublicationList,
    Resource::SubscriptionList,
];

/// The resources in the Profile, in its order: those served by `Get` and
/// the Status events (REQ-RES-EVENT-005); the SubscriptionList only in the
/// Profile of the application, which holds every subscription. The
/// common service NewDataSetWriterId follows (MG 8.3.7, REQ-CMP-040).
const PROFILE: [Resource; 9] = [
    Resource::Mam,
    Resource::Health,
    Resource::Config,
    Resource::Data,
    Resource::Metadata,
    Resource::Event,
    Resource::Profile,
    Resource::PublicationList,
    Resource::SubscriptionList,
];

/// The resources a `Set` addresses (MG 9.1); each `Set` gets a Status
/// event, `BadNotFound` or `BadNotWritable` for those without writable
/// content (REQ-CMP-043).
const SET: [Resource; 6] = [
    Resource::Data,
    Resource::Metadata,
    Resource::Config,
    Resource::PublicationList,
    Resource::SubscriptionList,
    Resource::ReferenceDesignation,
];

/// The resources a `Del` addresses (MG 9.1).
const DEL: [Resource; 2] = [Resource::SubscriptionList, Resource::ReferenceDesignation];

/// What [`Provider::handle_message`] asks the runtime to do.
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Handled {
    /// Publish these messages in order.
    Publish(Vec<Publication>),
    /// The application decides about this `Set Data`; then
    /// [`Provider::complete_set_data`] answers.
    SetData(SetDataRequest),
    /// The application decides about this `Set Config`; then
    /// [`Provider::complete_set_config`] answers.
    SetConfig(SetConfigRequest),
    /// The application decides about this `Set` or `Del` of a
    /// ReferenceDesignation; then [`Provider::complete_reference_designation`]
    /// answers.
    SetReferenceDesignation(ReferenceDesignationRequest),
}

impl Handled {
    /// The publications; `None` for a request the application decides.
    pub fn into_publications(self) -> Option<Vec<Publication>> {
        match self {
            Self::Publish(p) => Some(p),
            Self::SetData(_) | Self::SetConfig(_) | Self::SetReferenceDesignation(_) => None,
        }
    }
}

/// The tables changed since they were last taken for persistence.
#[derive(Debug, Clone, Copy, Default)]
#[allow(clippy::struct_field_names, clippy::struct_excessive_bools)]
struct Unsaved {
    writers: bool,
    versions: bool,
    overrides: bool,
    config: bool,
    refdes: bool,
    subscriptions: bool,
}

#[derive(Debug, Clone, Copy, Default)]
struct Dirty {
    mam: bool,
    health: bool,
    profile: bool,
}

/// Pending publications about the application beyond the lifecycle.
#[derive(Debug, Clone, Copy, Default)]
struct Pending {
    profile: bool,
    list: bool,
    /// The SubscriptionList may have changed.
    sublist: bool,
}

#[derive(Debug, Clone)]
struct Asset {
    il: IdentificationLink,
    mam: Mam,
    health: Health,
    dirty: Dirty,
}

/// The provider side of an application: see the [module](self).
#[derive(Debug, Clone)]
pub struct Provider {
    lifecycle: Lifecycle,
    config: ProviderConfig,
    writers: WriterIds,
    assets: Vec<Asset>,
    sets: Vec<Set>,
    versions: MetaDataVersions,
    overrides: PublicationOverrides,
    configs: Vec<ConfigEntry>,
    config_values: StoredConfigValues,
    config_instance: u64,
    refdes: Option<StoredReferenceDesignations>,
    events: events::Events,
    consumer_subs: Vec<ConsumerSubscription>,
    sub_overrides: SubscriptionOverrides,
    /// The settings last handed to the consumer side.
    sub_sent: Option<SubscriptionOverrides>,
    /// The entries last published; `None` after a connect.
    sub_snapshot: Option<Vec<sublist::Listed>>,
    unsaved: Unsaved,
    pending: Pending,
    connected: bool,
}

impl Provider {
    /// The provider of the application of `lifecycle` with the persisted
    /// DataSetWriterIds `writers` (empty at the first start). The MAM and
    /// Health of the application keep the ids of the lifecycle.
    ///
    /// # Errors
    ///
    /// [`ProviderError::WriterId`] if `writers` contradicts the ids of the
    /// lifecycle or is exhausted.
    pub fn new(
        lifecycle: Lifecycle,
        config: ProviderConfig,
        writers: WriterIds,
    ) -> Result<Self, ProviderError> {
        let mut p = Self {
            lifecycle,
            config,
            writers,
            assets: Vec::new(),
            sets: Vec::new(),
            versions: MetaDataVersions::new(),
            overrides: PublicationOverrides::new(),
            configs: Vec::new(),
            config_values: StoredConfigValues::new(),
            config_instance: 0,
            refdes: None,
            events: events::Events::default(),
            consumer_subs: Vec::new(),
            sub_overrides: SubscriptionOverrides::new(),
            sub_sent: None,
            sub_snapshot: None,
            unsaved: Unsaved::default(),
            pending: Pending::default(),
            connected: false,
        };
        let app = p.app_id().as_str().to_owned();
        let pinned = [
            (Resource::Mam, p.lifecycle.mam_writer()),
            (Resource::Health, p.lifecycle.health_writer()),
        ];
        for (resource, id) in pinned {
            p.unsaved.writers |= p.writers.pin(WriterKey::new(resource, &app), id)?;
        }
        if config.get {
            p.assign(WriterKey::new(Resource::Profile, &app))?;
            p.assign(WriterKey::publication_list())?;
            for &level in StatusLevel::ALL {
                p.assign(write::status_key(&app, level))?;
            }
            p.assign(WriterKey::subscription_list())?;
        }
        Ok(p)
    }

    /// The lifecycle: Will, Close and the Health of the application.
    pub fn lifecycle(&self) -> &Lifecycle {
        &self.lifecycle
    }

    /// The lifecycle, to change the Health or MAM of the application.
    pub fn lifecycle_mut(&mut self) -> &mut Lifecycle {
        &mut self.lifecycle
    }

    /// The AppId.
    pub fn app_id(&self) -> &IdentificationLink {
        self.lifecycle.app_id()
    }

    /// Whether requests (`Get`, `Set`, `Del`) are served: Level 2 on
    /// ([`ProviderConfig::get`]).
    pub fn serves_requests(&self) -> bool {
        self.config.get
    }

    /// The table of DataSetWriterIds.
    pub fn writer_ids(&self) -> &WriterIds {
        &self.writers
    }

    /// The table as JSON if it changed since the last call; persist it, so
    /// the ids stay stable across restarts (REQ-RES-PUBLIST-006).
    pub fn take_writer_ids(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.unsaved.writers).then(|| self.writers.to_json())
    }

    /// The provider with the persisted ConfigurationVersions of the
    /// Metadata (empty at the first start), so an unchanged Metadata keeps
    /// its version and Timestamp across restarts (REQ-MSG-032).
    #[must_use]
    pub fn with_meta_data_versions(mut self, versions: MetaDataVersions) -> Self {
        self.versions = versions;
        self
    }

    /// The table of ConfigurationVersions.
    pub fn meta_data_versions(&self) -> &MetaDataVersions {
        &self.versions
    }

    /// The table of ConfigurationVersions as JSON if it changed since the
    /// last call; persist it.
    pub fn take_meta_data_versions(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.unsaved.versions).then(|| self.versions.to_json())
    }

    /// The provider with the persisted settings of `Set PublicationList`
    /// (empty at the first start); [`Provider::add_data`] applies them
    /// within what the `Config` of the DataSet allows.
    #[must_use]
    pub fn with_publication_overrides(mut self, overrides: PublicationOverrides) -> Self {
        self.overrides = overrides;
        self
    }

    /// The settings set by `Set PublicationList`.
    pub fn publication_overrides(&self) -> &PublicationOverrides {
        &self.overrides
    }

    /// The settings set by `Set PublicationList` as JSON if they changed
    /// since the last call; persist them (REQ-RES-PUBLIST-016).
    pub fn take_publication_overrides(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.unsaved.overrides).then(|| self.overrides.to_json())
    }

    /// The topic filters to subscribe: `Get` of every served resource,
    /// `Set` and `Del` of every resource they address and `Call` of
    /// NewDataSetWriterId with and without Source (REQ-CMP-040,
    /// REQ-ACT-020), to this application (none at Level 1).
    ///
    /// # Panics
    ///
    /// Never: these filters are allowed forms.
    pub fn subscriptions(&self) -> Vec<TopicFilter> {
        if !self.config.get {
            return Vec::new();
        }
        let st = self.lifecycle.publisher_id().service_type();
        let get = self.served().into_iter().map(|r| (Method::Get, r));
        let set = SET.iter().map(|r| (Method::Set, *r));
        let del = DEL.iter().map(|r| (Method::Del, *r));
        let mut out: Vec<TopicFilter> = get
            .chain(set)
            .chain(del)
            .map(|(m, r)| {
                TopicFilter::builder(m, r)
                    .service_type(st)
                    .app_id(self.app_id().clone())
                    .build()
                    .expect("Get, Set and Del of these resources have valid filters")
            })
            .collect();
        for any_source in [false, true] {
            let b = TopicFilter::builder(Method::Call, Service::NewDataSetWriterId)
                .service_type(st)
                .app_id(self.app_id().clone());
            let b = if any_source {
                b.any_source()
            } else {
                b.without_source()
            };
            out.push(
                b.build()
                    .expect("Call NewDataSetWriterId has valid filters"),
            );
        }
        out
    }

    /// Registers a subordinate asset with its MAM (REQ-CMP-007); its Health
    /// starts as NORMAL_0 with HealthScore 100. Its MAM, Health and Profile
    /// are published at once if connected, and the PublicationList again
    /// (REQ-RES-PUBLIST-017).
    ///
    /// # Errors
    ///
    /// [`ProviderError::ProductInstanceUri`], [`ProviderError::SelfAsset`],
    /// [`ProviderError::DuplicateAsset`], [`ProviderError::WriterId`].
    pub fn add_asset(&mut self, mam: Mam) -> Result<IdentificationLink, ProviderError> {
        let il = IdentificationLink::parse(&mam.product_instance_uri)
            .map_err(|e| ProviderError::ProductInstanceUri(e.to_string()))?;
        if &il == self.app_id() {
            return Err(ProviderError::SelfAsset);
        }
        if self.find(&il).is_some() {
            return Err(ProviderError::DuplicateAsset(il.into_string()));
        }
        let mut keys = vec![
            WriterKey::new(Resource::Mam, il.as_str()),
            WriterKey::new(Resource::Health, il.as_str()),
        ];
        if self.config.get {
            keys.push(WriterKey::new(Resource::Profile, il.as_str()));
            if self.refdes.is_some() {
                keys.push(WriterKey::new(Resource::ReferenceDesignation, il.as_str()));
            }
            keys.extend(
                StatusLevel::ALL
                    .iter()
                    .map(|l| write::status_key(il.as_str(), *l)),
            );
        }
        for key in keys {
            self.assign(key)?;
        }
        self.assets.push(Asset {
            il: il.clone(),
            mam,
            health: normal(),
            dirty: Dirty {
                mam: true,
                health: true,
                profile: self.config.get,
            },
        });
        self.pending.list = self.config.get;
        Ok(il)
    }

    /// Removes a subordinate asset; the PublicationList is published again
    /// (REQ-RES-PUBLIST-017). Its DataSetWriterIds stay reserved for it.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`].
    pub fn remove_asset(&mut self, il: &IdentificationLink) -> Result<(), ProviderError> {
        let i = self
            .find(il)
            .ok_or_else(|| ProviderError::UnknownAsset(il.to_string()))?;
        self.assets.remove(i);
        self.sets.retain(|s| &s.source != il);
        self.configs.retain(|c| &c.source != il);
        self.events.remove(il);
        self.pending.list = self.config.get;
        Ok(())
    }

    /// The subordinate assets in the order of registration.
    pub fn assets(&self) -> impl Iterator<Item = &IdentificationLink> {
        self.assets.iter().map(|a| &a.il)
    }

    /// The MAM of the application or of an asset.
    pub fn mam(&self, il: &IdentificationLink) -> Option<&Mam> {
        if il == self.app_id() {
            return Some(self.lifecycle.mam());
        }
        self.find(il).map(|i| &self.assets[i].mam)
    }

    /// The Health of the application or of an asset.
    pub fn health(&self, il: &IdentificationLink) -> Option<&Health> {
        if il == self.app_id() {
            return Some(self.lifecycle.health());
        }
        self.find(il).map(|i| &self.assets[i].health)
    }

    /// Replaces the MAM of the application or of an asset (e.g. a new
    /// RevisionCounter, REQ-RES-MAM-018); a change is published at once.
    /// Returns whether it changed.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`], [`ProviderError::ProductInstanceUri`]
    /// if the ProductInstanceUri changes, [`ProviderError::Lifecycle`].
    pub fn set_mam(&mut self, il: &IdentificationLink, mam: Mam) -> Result<bool, ProviderError> {
        if il == self.app_id() {
            return Ok(self.lifecycle.set_mam(mam)?);
        }
        let i = self
            .find(il)
            .ok_or_else(|| ProviderError::UnknownAsset(il.to_string()))?;
        if mam.product_instance_uri != il.as_str() {
            return Err(ProviderError::ProductInstanceUri(format!(
                "{:?} is not the asset {il}",
                mam.product_instance_uri
            )));
        }
        let a = &mut self.assets[i];
        if a.mam == mam {
            return Ok(false);
        }
        a.mam = mam;
        a.dirty.mam = true;
        Ok(true)
    }

    /// Sets the Health of the application or of an asset; a change is
    /// published at once (REQ-RES-HEALTH-004). Returns whether it changed.
    /// Only the application has a heartbeat (REQ-TOP-017, SF-106).
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`] and the errors of
    /// [`Lifecycle::set_health`] and [`check_health`].
    pub fn set_health(
        &mut self,
        il: &IdentificationLink,
        health: Health,
    ) -> Result<bool, ProviderError> {
        if il == self.app_id() {
            return Ok(self.lifecycle.set_health(health)?);
        }
        let i = self
            .find(il)
            .ok_or_else(|| ProviderError::UnknownAsset(il.to_string()))?;
        check_health(&health)?;
        let a = &mut self.assets[i];
        if a.health == health {
            return Ok(false);
        }
        a.health = health;
        a.dirty.health = true;
        Ok(true)
    }

    /// Adds a Data DataSet to the application or an asset (MG 8.3.4) and
    /// returns its DataSetWriterId. Its Metadata is derived from the
    /// fields (REQ-RES-META-003) with the DataSetClassId of the definition
    /// or a UUIDv5 of Tag and field structure (REQ-MSG-013) and a
    /// ConfigurationVersion that changes only with the Metadata
    /// (REQ-MSG-036). The DataSet is published once every field has a
    /// value; the Metadata at once while connected (unless the Mode is
    /// OFF_0), and the PublicationList again. A persisted setting of
    /// `Set PublicationList` (see [`Provider::with_publication_overrides`])
    /// replaces Mode, Interval and Precisions of the definition as far as
    /// its `Config` allows.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`], [`ProviderError::DataSet`],
    /// [`ProviderError::Topic`] for a Tag that is no valid Filter,
    /// [`ProviderError::DuplicateDataSet`], [`ProviderError::WriterId`].
    pub fn add_data(
        &mut self,
        source: &IdentificationLink,
        spec: DataSetSpec,
        unix_ms: u64,
    ) -> Result<DataSetWriterId, ProviderError> {
        if source != self.app_id() && self.find(source).is_none() {
            return Err(ProviderError::UnknownAsset(source.to_string()));
        }
        spec.check().map_err(ProviderError::DataSet)?;
        let tag = Filter::Tag(spec.tag.clone());
        self.topic(Resource::Data, Some(source), Some(tag.clone()))?;
        let spec = self.overridden(source, &tag.to_string(), spec);
        if self.find_set(source, &spec.tag).is_some() {
            return Err(ProviderError::DuplicateDataSet {
                source: source.to_string(),
                tag: spec.tag,
            });
        }
        let filter = tag.to_string();
        let class_id = spec
            .class_id
            .unwrap_or_else(|| data::class_id(&spec.tag, &spec.fields));
        let mut meta = data::meta_data(&spec, class_id);
        let mut versions = self.versions.clone();
        let (version, changed) = versions.version(source.as_str(), &filter, &meta, unix_ms);
        meta.configuration_version = Some(version);
        let size = self.meta_message_size(source, &filter, &meta, unix_ms);
        if size > self.config.max_packet_size {
            return Err(ProviderError::DataSet(format!(
                "its Metadata of {size} octets exceeds the MaxPacketSize of {} octets",
                self.config.max_packet_size
            )));
        }
        let writer = self.assign(WriterKey {
            resource: Resource::Data,
            source: source.as_str().to_owned(),
            filter: filter.clone(),
        })?;
        self.versions = versions;
        self.unsaved.versions |= changed;
        let mut set = Set::new(source.clone(), spec, filter, class_id, meta, writer);
        set.meta_pending = set.mode().is_on();
        self.sets.push(set);
        self.pending.list = self.config.get;
        Ok(writer)
    }

    /// Sets field values of a Data DataSet, all or none: the state that is
    /// published as its PublicationList entry says (REQ-MQTT-019,
    /// REQ-RES-PUBLIST-013 … 015). `unix_ms` is the Timestamp of the
    /// DataSetMessage. Returns whether a value changed.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownDataSet`], [`ProviderError::Value`] for an
    /// unknown field or a value that does not fit its type.
    pub fn update_data(
        &mut self,
        source: &IdentificationLink,
        tag: &str,
        values: Vec<(String, FieldValue)>,
        unix_ms: u64,
    ) -> Result<bool, ProviderError> {
        let i = self.set_index(source, tag)?;
        self.sets[i]
            .update(values, unix_ms)
            .map_err(ProviderError::Value)
    }

    /// Removes a Data DataSet; the PublicationList is published again. Its
    /// DataSetWriterId and ConfigurationVersion stay reserved for it.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownDataSet`].
    pub fn remove_data(
        &mut self,
        source: &IdentificationLink,
        tag: &str,
    ) -> Result<(), ProviderError> {
        let i = self.set_index(source, tag)?;
        self.sets.remove(i);
        self.pending.list = self.config.get;
        Ok(())
    }

    /// The Tags of the Data DataSets of a source in the order of addition.
    pub fn data_tags<'a>(
        &'a self,
        source: &'a IdentificationLink,
    ) -> impl Iterator<Item = &'a str> {
        self.sets
            .iter()
            .filter(move |s| &s.source == source)
            .map(Set::tag)
    }

    /// The Metadata of a Data DataSet with its ConfigurationVersion.
    pub fn meta_data(&self, source: &IdentificationLink, tag: &str) -> Option<&DataSetMetaData> {
        self.find_set(source, tag).map(|i| &self.sets[i].meta)
    }

    /// The current values of a Data DataSet in field order, once every
    /// field has one.
    pub fn data_values(&self, source: &IdentificationLink, tag: &str) -> Option<Vec<FieldValue>> {
        self.find_set(source, tag)
            .and_then(|i| self.sets[i].values())
    }

    /// The Will of the next CONNECT (see [`Lifecycle::will`]).
    pub fn will(&mut self, unix_ms: u64) -> crate::mqtt::LastWill {
        self.lifecycle.will(unix_ms)
    }

    /// The Close (see [`Lifecycle::close`]); the provider is offline after.
    pub fn close(&mut self, unix_ms: u64) -> Publication {
        self.connected = false;
        self.events.clear();
        self.lifecycle.close(unix_ms)
    }

    /// The connection is lost: changes wait for [`Self::on_connected`].
    pub fn on_disconnected(&mut self) {
        self.connected = false;
        self.events.clear();
    }

    /// The connection is established (CONNACK): the Birth and the Health of
    /// the application (REQ-LIFE-001, REQ-LIFE-015), then MAM and Health of
    /// every asset, from Level 2 the Profile of every source and the
    /// PublicationList, then every Config, the Metadata of every DataSet
    /// not OFF_0 and the Data of every published DataSet with a value set –
    /// the current state, nothing queued; every Interval starts anew.
    pub fn on_connected(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        self.connected = true;
        let mut out = self.lifecycle.on_connected(now, unix_ms);
        for a in &mut self.assets {
            a.dirty = Dirty {
                mam: true,
                health: true,
                profile: self.config.get,
            };
        }
        self.pending = Pending {
            profile: self.config.get,
            list: self.config.get,
            sublist: self.config.get,
        };
        self.sub_snapshot = None;
        for s in &mut self.sets {
            s.restart();
        }
        for c in &mut self.configs {
            c.pending = true;
        }
        out.extend(self.changes(now, unix_ms));
        out
    }

    /// The instant [`Self::poll`] has work (monotonic time): at once after
    /// a change, else the next heartbeat or the next cyclic publication of
    /// a DataSet with an Interval.
    pub fn poll_timeout(&self) -> Duration {
        let lifecycle = self.lifecycle.poll_timeout();
        if !self.connected {
            return lifecycle;
        }
        if self.dirty() {
            return Duration::ZERO;
        }
        self.sets
            .iter()
            .filter_map(Set::deadline)
            .fold(lifecycle, Duration::min)
    }

    /// The publications due at `now` while connected: those of the
    /// lifecycle, then the changed MAM, Health and Profile of the assets,
    /// the PublicationList after a change of the sources, the changed
    /// Metadata and the due Data.
    pub fn poll(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        if !self.connected {
            return Vec::new();
        }
        let mut out = self.lifecycle.poll(now, unix_ms);
        out.extend(self.changes(now, unix_ms));
        out
    }

    /// Answers a `Get` of a served resource (REQ-RES-PUBLIST-027,
    /// REQ-CMP-041): without Source all DataSetMessages of the application
    /// on `Pub/<Resource>`, with Source those of that source on
    /// `Pub/<Resource>/<Source>[/<Filter>]`, with the ReplyTo of the `Get`
    /// (REQ-MSG-015) and paginated as requested (REQ-PAG-002 … 007).
    ///
    /// # Errors
    ///
    /// Why the message is not answered, for the log.
    pub fn handle_message(
        &mut self,
        topic: &str,
        payload: &[u8],
        unix_ms: u64,
    ) -> Result<Handled, Ignored> {
        let topic = Topic::parse(topic).map_err(Ignored::Topic)?;
        if topic.app_id() != self.app_id() {
            return Err(Ignored::NotForUs);
        }
        match topic.method() {
            Method::Get => self.handle_get(&topic, payload, unix_ms),
            Method::Set | Method::Del => self.handle_write(&topic, payload, unix_ms),
            Method::Call => self.handle_call(&topic, payload, unix_ms),
            _ => Err(Ignored::NotForUs),
        }
    }

    // ------------------------------------------------------------ internals

    fn handle_get(
        &mut self,
        topic: &Topic,
        payload: &[u8],
        unix_ms: u64,
    ) -> Result<Handled, Ignored> {
        let resource = match topic.subject() {
            Subject::Resource(r) if self.config.get && self.served().contains(&r) => r,
            s => return Err(Ignored::NotServed(s)),
        };
        let get = GetWire::parse(payload)?;
        if let Some(src) = topic.source()
            && !self.is_source(src)
        {
            return Ok(self.unknown_source(&get.message_id, unix_ms));
        }
        let source = topic.source();
        if resource == Resource::Metadata {
            return self
                .answer_meta_data(topic, &get, unix_ms)
                .map(Handled::Publish);
        }
        let reply = Topic::new(
            topic.service_type(),
            topic.app_id().clone(),
            Method::Pub,
            resource,
            source.cloned(),
            topic.filter().cloned(),
        )
        .map_err(Ignored::Topic)?;
        let req = get.pagination.as_ref();
        let limit = self.config.max_packet_size;
        let payloads = match resource {
            Resource::Mam => {
                let items = self.mam_items(source);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::Health => {
                let items = self.health_items(source);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::Profile => {
                let items = self.profile_items(source);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::Config => {
                let context = match topic.filter() {
                    Some(Filter::ContextName(c)) => Some(c.as_str()),
                    _ => None,
                };
                let items = self.config_items(source, context);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::Data => {
                let tag = match topic.filter() {
                    Some(Filter::Tag(t)) => Some(t.as_str()),
                    _ => None,
                };
                let (items, classes) = self.data_items(source, tag, |_| true);
                self.series_of(items, &classes, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::ReferenceDesignation => {
                // a Query only without Source, ignored with one
                // (REQ-RES-REFDES-013)
                let query = match (source, &get.query) {
                    (None, Some(q)) => Some(
                        ReferenceDesignationQuery::try_from(&Query(q.clone()))
                            .map_err(|e| Ignored::Payload(format!("Query: {e}")))?,
                    ),
                    _ => None,
                };
                let items = self.refdes_items(source, query.as_ref());
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            Resource::SubscriptionList => {
                let filter = match topic.filter() {
                    Some(Filter::List(f)) => Some(f),
                    _ => None,
                };
                let items = self.sublist_items(source, filter);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
            _ => {
                let filter = match topic.filter() {
                    Some(Filter::List(f)) => Some(f),
                    _ => None,
                };
                let items = self.list_items(source, filter);
                self.series(items, req, Some(&get.message_id), limit, unix_ms)
            }
        };
        Ok(Handled::Publish(
            payloads
                .into_iter()
                .map(|payload| Publication {
                    topic: reply.clone(),
                    payload,
                })
                .collect(),
        ))
    }

    /// The resources served by `Get`: [`SERVED`] and the
    /// ReferenceDesignation if offered.
    fn served(&self) -> Vec<Resource> {
        let mut out = SERVED.to_vec();
        if self.refdes.is_some() {
            out.push(Resource::ReferenceDesignation);
        }
        out
    }

    fn find(&self, il: &IdentificationLink) -> Option<usize> {
        self.assets.iter().position(|a| &a.il == il)
    }

    fn assign(&mut self, key: WriterKey) -> Result<DataSetWriterId, WriterIdError> {
        let (id, new) = self.writers.assign(key)?;
        self.unsaved.writers |= new;
        Ok(id)
    }

    fn writer(&self, resource: Resource, source: &IdentificationLink) -> DataSetWriterId {
        self.writers
            .get(&WriterKey::new(resource, source.as_str()))
            .expect("every served DataSet has a DataSetWriterId")
    }

    fn dirty(&self) -> bool {
        self.pending.profile
            || self.pending.list
            || self.pending.sublist
            || self
                .assets
                .iter()
                .any(|a| a.dirty.mam || a.dirty.health || a.dirty.profile)
            || self.configs.iter().any(|c| c.pending)
            || self.events.pending()
            || self
                .sets
                .iter()
                .any(|s| s.meta_pending || s.due(Duration::ZERO))
    }

    fn find_set(&self, source: &IdentificationLink, tag: &str) -> Option<usize> {
        self.sets
            .iter()
            .position(|s| &s.source == source && s.tag() == tag)
    }

    fn set_index(&self, source: &IdentificationLink, tag: &str) -> Result<usize, ProviderError> {
        self.find_set(source, tag)
            .ok_or_else(|| ProviderError::UnknownDataSet {
                source: source.to_string(),
                tag: tag.to_owned(),
            })
    }

    fn topic(
        &self,
        resource: Resource,
        source: Option<&IdentificationLink>,
        filter: Option<Filter>,
    ) -> Result<Topic, TopicError> {
        Topic::new(
            self.lifecycle.publisher_id().service_type(),
            self.app_id().clone(),
            Method::Pub,
            resource,
            source.cloned(),
            filter,
        )
    }

    /// The pending publications of assets, Profiles and PublicationList,
    /// then Config, Metadata and Data.
    fn changes(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        let mut out = self.resource_changes(unix_ms);
        out.extend(self.event_changes());
        out.extend(self.config_changes(unix_ms));
        out.extend(self.meta_data_changes(unix_ms));
        out.extend(self.data_changes(now, unix_ms));
        out
    }

    /// The changed Metadata on `Pub/Metadata/<Source>/<Tag>`
    /// (REQ-RES-META-003; ON_REQUEST_FILTER_9, REQ-RES-PUBLIST-012).
    fn meta_data_changes(&mut self, unix_ms: u64) -> Vec<Publication> {
        let mut out = Vec::new();
        for i in 0..self.sets.len() {
            if !core::mem::take(&mut self.sets[i].meta_pending) || !self.sets[i].mode().is_on() {
                continue;
            }
            let s = &self.sets[i];
            let topic = self
                .topic(
                    Resource::Metadata,
                    Some(&s.source),
                    Some(Filter::Tag(s.tag().to_owned())),
                )
                .expect("add_data checked the Tag");
            let payload = self.meta_message(i, None, unix_ms);
            out.push(Publication { topic, payload });
        }
        out
    }

    /// The due Data (REQ-RES-PUBLIST-009 … 011, 013 … 015): with
    /// APPLICATION_2 all such DataSets on `Pub/Data`, with SOURCE_3 those
    /// of the source on `Pub/Data/<Source>`, with FILTER_4 the DataSet
    /// alone on `Pub/Data/<Source>/<Tag>`. A group carries every DataSet
    /// of the form that has a value set (the state), due or not.
    fn data_changes(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        let due: Vec<usize> = (0..self.sets.len())
            .filter(|i| self.sets[*i].due(now))
            .collect();
        if due.is_empty() {
            return Vec::new();
        }
        let mut along = vec![false; self.sets.len()];
        let mut out = Vec::new();
        if due.iter().any(|i| self.sets[*i].mode().application()) {
            let (items, classes) = self.data_items(None, None, |s| s.mode().application());
            for (flag, s) in along.iter_mut().zip(&self.sets) {
                *flag |= s.available() && s.mode().application();
            }
            out.extend(self.publish_data(None, None, items, &classes, unix_ms));
        }
        let mut sources: Vec<IdentificationLink> = Vec::new();
        for i in &due {
            let s = &self.sets[*i];
            if s.mode().source() && !sources.contains(&s.source) {
                sources.push(s.source.clone());
            }
        }
        for src in sources {
            let (items, classes) = self.data_items(Some(&src), None, |s| s.mode().source());
            for (flag, s) in along.iter_mut().zip(&self.sets) {
                *flag |= s.source == src && s.available() && s.mode().source();
            }
            out.extend(self.publish_data(Some(&src), None, items, &classes, unix_ms));
        }
        for i in &due {
            let s = &self.sets[*i];
            if s.mode().filter() {
                let src = s.source.clone();
                let tag = Filter::Tag(s.tag().to_owned());
                let items = vec![s.message()];
                let classes = [s.class_id];
                out.extend(self.publish_data(Some(&src), Some(tag), items, &classes, unix_ms));
            }
        }
        for (i, s) in self.sets.iter_mut().enumerate() {
            if due.contains(&i) {
                s.published_due(now);
            } else if along[i] {
                s.published_along();
            }
        }
        out
    }

    fn publish_data(
        &mut self,
        source: Option<&IdentificationLink>,
        filter: Option<Filter>,
        items: Vec<DataSetMessage<Data>>,
        classes: &[uuid::Uuid],
        unix_ms: u64,
    ) -> Vec<Publication> {
        let topic = self
            .topic(Resource::Data, source, filter)
            .expect("add_data checked the Tag");
        let limit = self.config.publish_limit.min(self.config.max_packet_size);
        self.series_of(items, classes, None, None, limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect()
    }

    /// The available Data DataSets of the sources addressed, with Tag
    /// `tag` if given and selected by `select`, in the order of sources
    /// and addition; and their DataSetClassIds.
    fn data_items(
        &self,
        source: Option<&IdentificationLink>,
        tag: Option<&str>,
        select: impl Fn(&Set) -> bool,
    ) -> (Vec<DataSetMessage<Data>>, Vec<uuid::Uuid>) {
        let mut items = Vec::new();
        let mut classes = Vec::new();
        for il in self.sources(source) {
            for s in &self.sets {
                if s.source == il && tag.is_none_or(|t| s.tag() == t) && s.available() && select(s)
                {
                    items.push(s.message());
                    classes.push(s.class_id);
                }
            }
        }
        (items, classes)
    }

    /// Answers a `Get` of Metadata of an available DataSet with its
    /// DataSetMetaData (REQ-RES-META-002); silent otherwise (REQ-CMP-044).
    fn answer_meta_data(
        &mut self,
        topic: &Topic,
        get: &GetWire,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, Ignored> {
        let (Some(source), Some(Filter::Tag(tag))) = (topic.source(), topic.filter()) else {
            return Err(Ignored::NoMetadata(
                "a Get of Metadata addresses Source and Tag (REQ-RES-META-001)".to_owned(),
            ));
        };
        let i = self
            .find_set(source, tag)
            .filter(|i| self.sets[*i].mode().is_on())
            .ok_or_else(|| {
                Ignored::NoMetadata(format!("the source {source} provides no Data {tag:?}"))
            })?;
        let reply = self
            .topic(Resource::Metadata, Some(source), topic.filter().cloned())
            .map_err(Ignored::Topic)?;
        let payload = self.meta_message(i, Some(get.message_id.clone()), unix_ms);
        Ok(vec![Publication {
            topic: reply,
            payload,
        }])
    }

    /// The ua-metadata NetworkMessage of a DataSet; its Timestamp is the
    /// first sending of the version (REQ-MSG-032).
    fn meta_message(&mut self, i: usize, reply_to: Option<MessageId>, unix_ms: u64) -> Vec<u8> {
        let s = &self.sets[i];
        let (first, changed) = self
            .versions
            .first_sent(s.source.as_str(), &s.filter, unix_ms);
        self.unsaved.versions |= changed;
        let s = &self.sets[i];
        let msg = MetaDataNetworkMessage {
            message_id: self.lifecycle.next_message_id(unix_ms),
            publisher_id: self.lifecycle.publisher_id().clone(),
            data_set_writer_id: s.writer,
            writer_group_name: s.filter.clone(),
            data_set_writer_name: s.source.clone(),
            timestamp: date_time(first),
            reply_to,
            meta_data: Some(s.meta.clone()),
        };
        serde_json::to_vec(&msg).expect("a MetaDataNetworkMessage always serializes")
    }

    /// The size of the ua-metadata NetworkMessage of a DataSet with the
    /// longest MessageId and a ReplyTo.
    fn meta_message_size(
        &self,
        source: &IdentificationLink,
        filter: &str,
        meta: &DataSetMetaData,
        unix_ms: u64,
    ) -> usize {
        let publisher = self.lifecycle.publisher_id();
        let id = MessageId::new(unix_ms, Some(u32::MAX), publisher);
        let msg = MetaDataNetworkMessage {
            message_id: id.clone(),
            publisher_id: publisher.clone(),
            data_set_writer_id: DataSetWriterId::new(u16::MAX).expect("a valid id"),
            writer_group_name: filter.to_owned(),
            data_set_writer_name: source.clone(),
            timestamp: date_time(unix_ms),
            reply_to: Some(id),
            meta_data: Some(meta.clone()),
        };
        serde_json::to_vec(&msg).map_or(usize::MAX, |v| v.len())
    }

    /// The pending publications of assets, Profiles and PublicationList.
    fn resource_changes(&mut self, unix_ms: u64) -> Vec<Publication> {
        let mut out = Vec::new();
        for i in 0..self.assets.len() {
            let il = self.assets[i].il.clone();
            let d = core::mem::take(&mut self.assets[i].dirty);
            if d.mam {
                let items = self.mam_items(Some(&il));
                out.extend(self.publish(Resource::Mam, Some(&il), items, unix_ms));
            }
            if d.health {
                let items = self.health_items(Some(&il));
                out.extend(self.publish(Resource::Health, Some(&il), items, unix_ms));
            }
            if d.profile {
                let items = self.profile_items(Some(&il));
                out.extend(self.publish(Resource::Profile, Some(&il), items, unix_ms));
            }
        }
        if core::mem::take(&mut self.pending.profile) {
            let app = self.app_id().clone();
            let items = self.profile_items(Some(&app));
            out.extend(self.publish(Resource::Profile, Some(&app), items, unix_ms));
        }
        let sublist = core::mem::take(&mut self.pending.sublist) || self.pending.list;
        if core::mem::take(&mut self.pending.list) {
            let items = self.list_items(None, None);
            out.extend(self.publish(Resource::PublicationList, None, items, unix_ms));
        }
        if sublist && self.config.get {
            out.extend(self.sublist_changes(unix_ms));
        }
        out
    }

    /// Unsolicited publication on `Pub/<resource>[/<source>]`.
    fn publish<P: DataSet>(
        &mut self,
        resource: Resource,
        source: Option<&IdentificationLink>,
        items: Vec<DataSetMessage<P>>,
        unix_ms: u64,
    ) -> Vec<Publication> {
        let topic = Topic::new(
            self.lifecycle.publisher_id().service_type(),
            self.app_id().clone(),
            Method::Pub,
            resource,
            source.cloned(),
            None,
        )
        .expect("Pub of a served resource with an asset Source is a valid topic");
        let limit = self.config.publish_limit.min(self.config.max_packet_size);
        self.series(items, None, None, limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect()
    }

    fn series<P: DataSet>(
        &mut self,
        items: Vec<DataSetMessage<P>>,
        request: Option<&PaginationRequest>,
        reply_to: Option<&MessageId>,
        limit: usize,
        unix_ms: u64,
    ) -> Vec<Vec<u8>> {
        self.series_of(items, &[], request, reply_to, limit, unix_ms)
    }

    /// [`Self::series`] with the DataSetClassIds of Data items.
    fn series_of<P: DataSet>(
        &mut self,
        items: Vec<DataSetMessage<P>>,
        classes: &[uuid::Uuid],
        request: Option<&PaginationRequest>,
        reply_to: Option<&MessageId>,
        limit: usize,
        unix_ms: u64,
    ) -> Vec<Vec<u8>> {
        let publisher: PublisherId = self.lifecycle.publisher_id().clone();
        let app = self.app_id().clone();
        let env = Envelope {
            publisher: &publisher,
            app: &app,
            reply_to,
            limit,
            classes,
        };
        let lifecycle = &mut self.lifecycle;
        pagination::series(items, request, &env, &mut || {
            lifecycle.next_message_id(unix_ms)
        })
    }

    /// The sources addressed: the given one, or the application and all
    /// assets.
    fn sources(&self, source: Option<&IdentificationLink>) -> Vec<IdentificationLink> {
        match source {
            Some(s) => vec![s.clone()],
            None => core::iter::once(self.app_id().clone())
                .chain(self.assets.iter().map(|a| a.il.clone()))
                .collect(),
        }
    }

    fn mam_items(&self, source: Option<&IdentificationLink>) -> Vec<DataSetMessage<Mam>> {
        self.sources(source)
            .into_iter()
            .filter_map(|il| {
                let mam = self.mam(&il)?.clone();
                let id = self.writer(Resource::Mam, &il);
                Some(DataSetMessage::new(id, il, "", mam).without_message_type())
            })
            .collect()
    }

    fn health_items(&self, source: Option<&IdentificationLink>) -> Vec<DataSetMessage<Health>> {
        self.sources(source)
            .into_iter()
            .filter_map(|il| {
                let health = self.health(&il)?.clone();
                let id = self.writer(Resource::Health, &il);
                Some(DataSetMessage::new(id, il, "", health).without_message_type())
            })
            .collect()
    }

    fn profile_items(&self, source: Option<&IdentificationLink>) -> Vec<DataSetMessage<Profile>> {
        self.sources(source)
            .into_iter()
            .map(|il| {
                let id = self.writer(Resource::Profile, &il);
                let app = &il == self.app_id();
                let resources = PROFILE
                    .iter()
                    .filter(|r| app || **r != Resource::SubscriptionList)
                    .flat_map(|r| {
                        // the ReferenceDesignation if offered, after the
                        // Metadata
                        let refdes = (*r == Resource::Metadata && self.refdes.is_some())
                            .then_some(Resource::ReferenceDesignation);
                        core::iter::once(*r).chain(refdes)
                    })
                    .map(Subject::Resource)
                    .chain(core::iter::once(Subject::Service(
                        Service::NewDataSetWriterId,
                    )))
                    .collect();
                DataSetMessage::new(id, il, "", Profile { resources })
            })
            .collect()
    }

    /// The PublicationList entries: per source MAM, Health and Profile,
    /// each published on its Source topic and answered on `Get`
    /// (ON_REQUEST_SOURCE_10, REQ-RES-PUBLIST-012/026), the
    /// ReferenceDesignation if offered (ON_REQUEST_SOURCE_10), its Status
    /// events Good, Uncertain and Bad (FILTER_4, published only on their
    /// topic), its Syslog (the application) and Generic events, its Config
    /// DataSets (ON_REQUEST_FILTER_9), then its Data DataSets with their
    /// Metadata, each in the order of addition.
    fn list_items(
        &self,
        source: Option<&IdentificationLink>,
        filter: Option<&ListFilter>,
    ) -> Vec<DataSetMessage<PublicationEntry>> {
        let id = self
            .writers
            .get(&WriterKey::publication_list())
            .expect("the PublicationList has a DataSetWriterId");
        let mut out = Vec::new();
        for il in self.sources(source) {
            for r in [Resource::Mam, Resource::Health, Resource::Profile] {
                if filter.is_some_and(|f| f.resource() != r || f.filter().is_some()) {
                    continue;
                }
                let entry = PublicationEntry {
                    resource: r,
                    source: il.as_str().to_owned(),
                    filter: None,
                    data_set_writer_id: self.writer(r, &il),
                    mode: Some(PublicationMode::OnRequestSource),
                    interval: None,
                    precisions: None,
                    config: None,
                };
                out.push(DataSetMessage::new(id, il.clone(), r.as_str(), entry));
            }
            out.extend(self.refdes_list_item(&il, filter, id));
            for &level in StatusLevel::ALL {
                let f = write::status_filter(level);
                if filter.is_some_and(|lf| {
                    lf.resource() != Resource::Event || lf.filter().is_some_and(|t| t != &f)
                }) {
                    continue;
                }
                let key = write::status_key(il.as_str(), level);
                let entry = PublicationEntry {
                    resource: Resource::Event,
                    source: il.as_str().to_owned(),
                    filter: Some(f.to_string()),
                    data_set_writer_id: self
                        .writers
                        .get(&key)
                        .expect("every source has DataSetWriterIds for its Status events"),
                    mode: Some(PublicationMode::Filter),
                    interval: None,
                    precisions: None,
                    config: None,
                };
                out.push(DataSetMessage::new(
                    id,
                    il.clone(),
                    format!("Event/{f}"),
                    entry,
                ));
            }
            out.extend(self.event_list_items(&il, filter, id));
            out.extend(self.config_list_items(&il, filter, id));
            for s in self.sets.iter().filter(|s| s.source == il) {
                for e in s.entries(id) {
                    let p = e.payload();
                    let tag = Filter::Tag(s.tag().to_owned());
                    if filter.is_some_and(|f| {
                        f.resource() != p.resource || f.filter().is_some_and(|t| t != &tag)
                    }) {
                        continue;
                    }
                    out.push(e);
                }
            }
        }
        out
    }
}

fn normal() -> Health {
    Health {
        health: HealthState::Normal,
        health_score: Some(100),
        details: None,
    }
}

/// The parts of a `Get` NetworkMessage the provider uses.
struct GetWire {
    message_id: MessageId,
    pagination: Option<PaginationRequest>,
    query: Option<serde_json::Map<String, Value>>,
}

impl GetWire {
    fn parse(payload: &[u8]) -> Result<Self, Ignored> {
        #[derive(Deserialize)]
        #[serde(rename_all = "PascalCase")]
        struct Wire {
            message_id: MessageId,
            #[serde(default)]
            messages: Vec<Value>,
        }
        let w: Wire =
            serde_json::from_slice(payload).map_err(|e| Ignored::Payload(e.to_string()))?;
        let pagination = w
            .messages
            .iter()
            .find(|m| m.get("DataSetWriterId").and_then(Value::as_u64) == Some(1))
            .and_then(|m| m.get("Payload"))
            .map(PaginationRequest::deserialize)
            .transpose()
            .map_err(|e| Ignored::Payload(format!("PaginationRequest: {e}")))?;
        let query = refdes::query_of(&w.messages).cloned();
        Ok(Self {
            message_id: w.message_id,
            pagination,
            query,
        })
    }
}
