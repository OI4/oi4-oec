//! The provider side of an application as a state machine (MG 9.1; P4
//! increment 6a, ADR-0026 §12): the sources it serves (itself and its
//! subordinate assets), their MAM, Health and Profile, the PublicationList
//! with stable DataSetWriterIds, the answers to `Get` and pagination.
//!
//! It owns the [`Lifecycle`]. The runtime calls [`Provider::on_connected`]
//! after CONNACK, [`Provider::poll`] at [`Provider::poll_timeout`] and on
//! every change, and [`Provider::handle_message`] for every message of
//! [`Provider::subscriptions`]; it publishes what they return in order.
//!
//! Offline nothing is queued (ADR-0020 §7, state instead of backlog):
//! changes only mark the state, and [`Provider::on_connected`] publishes
//! the current state of every source.

pub mod pagination;
pub mod writers;

use core::fmt;
use core::time::Duration;

use serde::Deserialize;
use serde_json::Value;

use crate::il::IdentificationLink;
use crate::lifecycle::{Lifecycle, LifecycleError, Publication, check_health};
use crate::message::{
    DataSet, DataSetMessage, DataSetWriterId, MessageId, PaginationRequest, PublisherId,
};
use crate::resource::{Health, HealthState, Mam, Profile, PublicationEntry, PublicationMode};
use crate::topic::{Filter, ListFilter, Method, Resource, Subject, Topic, TopicError, TopicFilter};
use pagination::Envelope;
pub use writers::{WriterIdError, WriterIds, WriterKey};

/// The settings of a [`Provider`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ProviderConfig {
    /// Answer `Get` and serve Profile and PublicationList: Level 2 and
    /// higher (ADR-0020 §3).
    pub get: bool,
    /// The largest payload of a publication: the MaxPacketSize of the
    /// Message Bus (REQ-PAG-005, SF-105); larger publications are
    /// paginated.
    pub max_packet_size: usize,
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
    /// Not a `Get` to this application.
    NotForUs,
    /// The application does not serve this resource with `Get` (or not at
    /// Level 1).
    NotServed(Subject),
    /// The Source is no asset of the application (REQ-CMP-047); the Status
    /// event BadNotFound follows with P4 increment 7.
    UnknownSource(IdentificationLink),
    /// The payload is no `Get` NetworkMessage with a valid MessageId.
    Payload(String),
}

impl fmt::Display for Ignored {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Topic(e) => write!(f, "no OEC topic: {e}"),
            Self::NotForUs => f.write_str("not a Get to this application"),
            Self::NotServed(s) => write!(f, "Get {s} is not served"),
            Self::UnknownSource(il) => write!(
                f,
                "Get for the Source {il}, which is no asset of the application (REQ-CMP-047)"
            ),
            Self::Payload(e) => write!(f, "invalid Get payload: {e}"),
        }
    }
}

/// The resources served by `Get`, in the order of the Profile.
const SERVED: [Resource; 4] = [
    Resource::Mam,
    Resource::Health,
    Resource::Profile,
    Resource::PublicationList,
];

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
    writers_changed: bool,
    assets: Vec<Asset>,
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
            writers_changed: false,
            assets: Vec::new(),
            pending: Pending::default(),
            connected: false,
        };
        let app = p.app_id().as_str().to_owned();
        let pinned = [
            (Resource::Mam, p.lifecycle.mam_writer()),
            (Resource::Health, p.lifecycle.health_writer()),
        ];
        for (resource, id) in pinned {
            p.writers_changed |= p.writers.pin(WriterKey::new(resource, &app), id)?;
        }
        if config.get {
            p.assign(WriterKey::new(Resource::Profile, &app))?;
            p.assign(WriterKey::publication_list())?;
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

    /// The table of DataSetWriterIds.
    pub fn writer_ids(&self) -> &WriterIds {
        &self.writers
    }

    /// The table as JSON if it changed since the last call; persist it, so
    /// the ids stay stable across restarts (REQ-RES-PUBLIST-006).
    pub fn take_writer_ids(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.writers_changed).then(|| self.writers.to_json())
    }

    /// The topic filters to subscribe: `Get` of every served resource to
    /// this application (none at Level 1).
    ///
    /// # Panics
    ///
    /// Never: these filters are allowed forms.
    pub fn subscriptions(&self) -> Vec<TopicFilter> {
        if !self.config.get {
            return Vec::new();
        }
        let st = self.lifecycle.publisher_id().service_type();
        SERVED
            .iter()
            .map(|r| {
                TopicFilter::builder(Method::Get, *r)
                    .service_type(st)
                    .app_id(self.app_id().clone())
                    .build()
                    .expect("Get of the served resources has a valid filter")
            })
            .collect()
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
        let mut resources = vec![Resource::Mam, Resource::Health];
        if self.config.get {
            resources.push(Resource::Profile);
        }
        for r in resources {
            self.assign(WriterKey::new(r, il.as_str()))?;
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

    /// The Will of the next CONNECT (see [`Lifecycle::will`]).
    pub fn will(&mut self, unix_ms: u64) -> crate::mqtt::LastWill {
        self.lifecycle.will(unix_ms)
    }

    /// The Close (see [`Lifecycle::close`]); the provider is offline after.
    pub fn close(&mut self, unix_ms: u64) -> Publication {
        self.connected = false;
        self.lifecycle.close(unix_ms)
    }

    /// The connection is lost: changes wait for [`Self::on_connected`].
    pub fn on_disconnected(&mut self) {
        self.connected = false;
    }

    /// The connection is established (CONNACK): the Birth and the Health of
    /// the application (REQ-LIFE-001, REQ-LIFE-015), then MAM and Health of
    /// every asset, from Level 2 the Profile of every source and the
    /// PublicationList – the current state, nothing queued.
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
        };
        out.extend(self.changes(unix_ms));
        out
    }

    /// The instant [`Self::poll`] has work (monotonic time).
    pub fn poll_timeout(&self) -> Duration {
        if self.connected && self.dirty() {
            Duration::ZERO
        } else {
            self.lifecycle.poll_timeout()
        }
    }

    /// The publications due at `now` while connected: those of the
    /// lifecycle, then the changed MAM, Health and Profile of the assets
    /// and the PublicationList after a change of the sources.
    pub fn poll(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        if !self.connected {
            return Vec::new();
        }
        let mut out = self.lifecycle.poll(now, unix_ms);
        out.extend(self.changes(unix_ms));
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
    ) -> Result<Vec<Publication>, Ignored> {
        let topic = Topic::parse(topic).map_err(Ignored::Topic)?;
        if topic.method() != Method::Get || topic.app_id() != self.app_id() {
            return Err(Ignored::NotForUs);
        }
        let resource = match topic.subject() {
            Subject::Resource(r) if self.config.get && SERVED.contains(&r) => r,
            s => return Err(Ignored::NotServed(s)),
        };
        if let Some(src) = topic.source()
            && src != self.app_id()
            && self.find(src).is_none()
        {
            return Err(Ignored::UnknownSource(src.clone()));
        }
        let get = GetWire::parse(payload)?;
        let source = topic.source();
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
        let payloads = match resource {
            Resource::Mam => {
                let items = self.mam_items(source);
                self.series(items, req, Some(&get.message_id), unix_ms)
            }
            Resource::Health => {
                let items = self.health_items(source);
                self.series(items, req, Some(&get.message_id), unix_ms)
            }
            Resource::Profile => {
                let items = self.profile_items(source);
                self.series(items, req, Some(&get.message_id), unix_ms)
            }
            _ => {
                let filter = match topic.filter() {
                    Some(Filter::List(f)) => Some(f),
                    _ => None,
                };
                let items = self.list_items(source, filter);
                self.series(items, req, Some(&get.message_id), unix_ms)
            }
        };
        Ok(payloads
            .into_iter()
            .map(|payload| Publication {
                topic: reply.clone(),
                payload,
            })
            .collect())
    }

    // ------------------------------------------------------------ internals

    fn find(&self, il: &IdentificationLink) -> Option<usize> {
        self.assets.iter().position(|a| &a.il == il)
    }

    fn assign(&mut self, key: WriterKey) -> Result<DataSetWriterId, WriterIdError> {
        let (id, new) = self.writers.assign(key)?;
        self.writers_changed |= new;
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
            || self
                .assets
                .iter()
                .any(|a| a.dirty.mam || a.dirty.health || a.dirty.profile)
    }

    /// The pending publications of assets, Profiles and PublicationList.
    fn changes(&mut self, unix_ms: u64) -> Vec<Publication> {
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
        if core::mem::take(&mut self.pending.list) {
            let items = self.list_items(None, None);
            out.extend(self.publish(Resource::PublicationList, None, items, unix_ms));
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
        self.series(items, None, None, unix_ms)
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
        unix_ms: u64,
    ) -> Vec<Vec<u8>> {
        let publisher: PublisherId = self.lifecycle.publisher_id().clone();
        let app = self.app_id().clone();
        let env = Envelope {
            publisher: &publisher,
            app: &app,
            reply_to,
            limit: self.config.max_packet_size,
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
                let resources = SERVED.iter().map(|r| Subject::Resource(*r)).collect();
                DataSetMessage::new(id, il, "", Profile { resources })
            })
            .collect()
    }

    /// The PublicationList entries: per source MAM, Health and Profile,
    /// each published on its Source topic and answered on `Get`
    /// (ON_REQUEST_SOURCE_10, REQ-RES-PUBLIST-012/026).
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
        Ok(Self {
            message_id: w.message_id,
            pagination,
        })
    }
}
