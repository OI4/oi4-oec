//! The consumer side of an application (ADR-0020 §6, ADR-0026 §17):
//! typed subscriptions, the presence of the other applications and
//! requests – `Get` with correlation over ReplyTo (REQ-MSG-015), all pages
//! reassembled (REQ-PAG-002, REQ-PAG-006), a Query answer checked
//! (REQ-RES-REFDES-015), a timeout – `Set`/`Del` with their Status
//! event (REQ-RES-EVENT-005) and the Action NewDataSetWriterId
//! (REQ-ACT-015, [`Consumer::new_data_set_writer_id`]).
//!
//! The protocol logic is the sans-IO `oec_core::consumer::Consumer`; this
//! module adds the callbacks, the waiting (async and blocking) and the
//! connection to the runtime of the [`App`](crate::app::App). Requests are
//! sent only while the application is online: nothing is queued
//! (ADR-0020 §7), a request while offline fails with
//! [`Error::NotConnected`], a lost connection ends the pending ones with it.
//!
//! The callbacks of [`Consumer::subscribe`] and [`Consumer::on_presence`]
//! run on the task of the runtime: they must return quickly and must not
//! block; to ask further, spawn a task or a thread.
//!
//! ```no_run
//! use std::time::Duration;
//!
//! use oec::consumer::{GetOptions, Payload, Presence};
//! use oec::prelude::*;
//!
//! async fn health_of(consumer: &Consumer, st: ServiceType, app: IdentificationLink)
//!     -> oec::Result<usize> {
//!     let options = GetOptions::new().timeout(Duration::from_secs(5)).per_page(50);
//!     let answers = consumer.get(Resource::Health, &Target::app(st, app), &options).await?;
//!     Ok(answers.iter().filter(|r| matches!(r.payload(), Payload::Health(_))).count())
//! }
//!
//! # fn main() -> oec::Result<()> {
//! # let product = Product::new("example.com", "Logger", "LOGGER-1");
//! let app = App::builder(Level::L1, ServiceType::Utility, product).build()?;
//! let consumer = app.consumer();
//! // every Data publication on the bus, until `_data` is dropped
//! let _data = consumer.subscribe(Resource::Data, &Target::any(), |msg| {
//!     println!("{:?} {:?}: {}", msg.source(), msg.filter(), msg.payload_json());
//! })?;
//! let _presence = consumer.on_presence(|p| match p {
//!     Presence::Online { app, .. } => println!("online  {app}"),
//!     Presence::OfflineGraceful { app, .. } | Presence::OfflineUngraceful { app, .. } => {
//!         println!("offline {app}");
//!     }
//!     _ => {}
//! })?;
//! # let _ = health_of;
//! app.run()
//! # }
//! ```

use std::collections::HashMap;
use std::fmt;
use std::future::Future;
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

use oec_core::consumer::{
    self as core, Address, ConsumerError, FilterChanges, GetParams, Outcome, PresenceChange,
    RequestId, SubscriptionId, WriteEntry,
};
use oec_core::lifecycle::Publication;
use oec_core::message::{DataSetWriterId, MessageId};
use oec_core::mqtt::SubscribeReturnCode;
use oec_core::topic::{Filter, Form, Method, Subject, TopicFilter};
use tokio::sync::oneshot;

pub use oec_core::consumer::{AppState, Payload};
pub use oec_core::resource::{Aspect, HealthState, ReferenceDesignationQuery as Query};
/// The OI4 resources (REQ-TOP-012, MG 7.5.1).
pub use oec_core::topic::Resource;

use crate::app::{AppHandle, WeakHandle};
use crate::identity::{IdentificationLink, Mam};
use crate::runtime::unix_ms;
use crate::status::StatusCode;
use crate::{Error, Result, ServiceType};

/// The default timeout of a request.
pub const DEFAULT_TIMEOUT: Duration = Duration::from_secs(10);

// ---------------------------------------------------------------- Target

/// The addressed application and asset. For a subscription, `None` parts
/// become the `+` wildcard (REQ-IL-021); a request needs the ServiceType
/// and the AppId, since a request goes to one application. The encoding of
/// AppId and Source on the topic is internal (REQ-IL-002).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Target {
    /// ServiceType of the application.
    pub service_type: Option<ServiceType>,
    /// AppId of the application.
    pub app: Option<IdentificationLink>,
    /// The asset (`Source`); `None` = every asset (subscription) or the
    /// application without Source (request).
    pub source: Option<IdentificationLink>,
    /// The Filter as decoded value: a Tag (Data, Metadata), a
    /// Context.Name (Config), `<EventCategory>/<EventLevel>` (Event) or
    /// `<ResourceType>[/<Tag>]` (PublicationList, SubscriptionList).
    pub filter: Option<String>,
}

impl Target {
    /// Every application (`Oi4v2/+/+/Pub/<Resource>/#`).
    pub fn any() -> Self {
        Self::default()
    }

    /// One application.
    pub fn app(service_type: ServiceType, app: IdentificationLink) -> Self {
        Self {
            service_type: Some(service_type),
            app: Some(app),
            ..Self::default()
        }
    }

    /// Only this asset.
    #[must_use]
    pub fn source(mut self, source: IdentificationLink) -> Self {
        self.source = Some(source);
        self
    }

    /// Only this Filter.
    #[must_use]
    pub fn filter(mut self, filter: &str) -> Self {
        self.filter = Some(filter.to_owned());
        self
    }

    fn topic_filter(&self, resource: Resource) -> Result<TopicFilter> {
        let mut b = TopicFilter::builder(Method::Pub, resource);
        if let Some(st) = self.service_type {
            b = b.service_type(st);
        }
        if let Some(app) = &self.app {
            b = b.app_id(app.0.clone());
        }
        match (&self.source, &self.filter) {
            (Some(source), _) => b = b.source(source.0.clone()),
            (None, Some(_)) => b = b.any_source(),
            (None, None) => {}
        }
        if let Some(f) = &self.filter {
            b = b.filter(filter_of(Method::Pub, resource, f)?);
        }
        Ok(b.build()?)
    }

    fn address(&self, method: Method, resource: Resource) -> Result<Address> {
        let (Some(service_type), Some(app)) = (self.service_type, &self.app) else {
            return Err(Error::Invalid(
                "a request needs the ServiceType and the AppId of the application".to_owned(),
            ));
        };
        let filter = match &self.filter {
            Some(f) => Some(filter_of(method, resource, f)?),
            None => None,
        };
        Ok(Address {
            service_type,
            app: app.0.clone(),
            source: self.source.as_ref().map(|s| s.0.clone()),
            filter,
        })
    }
}

fn filter_of(method: Method, resource: Resource, value: &str) -> Result<Filter> {
    let kind = Form::of(method, Subject::Resource(resource))
        .and_then(|f| f.filter_kind)
        .ok_or_else(|| Error::Invalid(format!("{method}/{resource} has no Filter")))?;
    Ok(Filter::of_kind(kind, value)?)
}

// -------------------------------------------------------------- Received

/// One received DataSetMessage of a resource with its topic context.
/// Unknown fields were ignored (REQ-MSG-046); [`Self::payload_json`] keeps
/// them.
#[derive(Debug, Clone, PartialEq)]
pub struct Received {
    inner: core::Received,
    resource: Resource,
}

impl Received {
    fn new(inner: core::Received) -> Self {
        let resource = match inner.topic.subject() {
            Subject::Resource(r) => r,
            // the core delivers resource topics only
            Subject::Service(_) => Resource::Event,
        };
        Self { inner, resource }
    }

    /// The topic it was published on.
    pub fn topic(&self) -> String {
        self.inner.topic.to_string()
    }

    /// ServiceType of the publishing application.
    pub fn service_type(&self) -> ServiceType {
        self.inner.topic.service_type()
    }

    /// AppId of the publishing application.
    pub fn app(&self) -> IdentificationLink {
        IdentificationLink(self.inner.topic.app_id().clone())
    }

    /// The resource.
    pub fn resource(&self) -> Resource {
        self.resource
    }

    /// The `Source` of the topic: the asset (REQ-TOP-018).
    pub fn source(&self) -> Option<IdentificationLink> {
        self.inner.topic.source().cloned().map(IdentificationLink)
    }

    /// The Filter of the topic, decoded (levels joined with `/`).
    pub fn filter(&self) -> Option<String> {
        self.inner.topic.filter().map(|f| f.values().join("/"))
    }

    /// DataSetWriterId (REQ-MSG-018).
    pub fn data_set_writer_id(&self) -> u16 {
        self.inner.data_set_writer_id
    }

    /// DataSetWriterName: the asset the DataSet is about (REQ-MSG-023).
    pub fn data_set_writer_name(&self) -> IdentificationLink {
        IdentificationLink(self.inner.data_set_writer_name.clone())
    }

    /// The typed Payload.
    pub fn payload(&self) -> &Payload {
        &self.inner.payload
    }

    /// The Payload as received, unknown fields included, as JSON text.
    pub fn payload_json(&self) -> String {
        self.inner.json.to_string()
    }

    /// The StatusCode of a Status event (REQ-RES-EVENT-003).
    pub fn status(&self) -> Option<StatusCode> {
        self.inner
            .payload
            .status()
            .map(|s| StatusCode::from_code(s.0))
    }

    /// The MessageId of the request this answers (REQ-MSG-015).
    pub fn reply_to(&self) -> Option<String> {
        self.inner.reply_to.as_ref().map(|m| m.as_str().to_owned())
    }
}

// -------------------------------------------------------------- Presence

/// A change of presence of an application (MG 6.3, REQ-LIFE-*): Birth,
/// Close, Will, or first seen by its Health heartbeat.
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Presence {
    /// Online; `mam` is its MAM if already known. An application first
    /// seen by its heartbeat is asked for its MAM and reported again with
    /// it.
    Online {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
        /// Its MAM.
        mam: Option<Box<Mam>>,
    },
    /// Offline with its Close message (REQ-LIFE-005).
    OfflineGraceful {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
    },
    /// Offline with its Will (REQ-LIFE-007).
    OfflineUngraceful {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
    },
}

impl Presence {
    fn new(change: PresenceChange) -> Option<Self> {
        Some(match change {
            PresenceChange::Online {
                service_type,
                app,
                mam,
            } => Self::Online {
                service_type,
                app: IdentificationLink(app),
                mam,
            },
            PresenceChange::OfflineGraceful { service_type, app } => Self::OfflineGraceful {
                service_type,
                app: IdentificationLink(app),
            },
            PresenceChange::OfflineUngraceful { service_type, app } => Self::OfflineUngraceful {
                service_type,
                app: IdentificationLink(app),
            },
            _ => return None,
        })
    }
}

/// What the consumer knows about an application ([`Consumer::apps`]).
#[derive(Debug, Clone, PartialEq)]
pub struct AppInfo {
    /// ServiceType of the application.
    pub service_type: ServiceType,
    /// AppId of the application.
    pub app: IdentificationLink,
    /// Its presence.
    pub state: AppState,
    /// Its MAM about itself, once seen.
    pub mam: Option<Mam>,
    /// Its last Health state about itself.
    pub health: Option<HealthState>,
    /// Its last HealthScore about itself.
    pub health_score: Option<u8>,
}

// --------------------------------------------------------------- options

/// Options of a `Get` ([`Consumer::get`]).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GetOptions {
    timeout: Duration,
    per_page: Option<u32>,
    locale: Option<String>,
    query: Option<Query>,
    data_set_writer_id: Option<u16>,
}

impl Default for GetOptions {
    fn default() -> Self {
        Self {
            timeout: DEFAULT_TIMEOUT,
            per_page: None,
            locale: None,
            query: None,
            data_set_writer_id: None,
        }
    }
}

impl GetOptions {
    /// Timeout [`DEFAULT_TIMEOUT`], no helper DataSets.
    pub fn new() -> Self {
        Self::default()
    }

    /// The time to wait for all answers.
    #[must_use]
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    /// A PaginationRequest with this PerPage for all pages (REQ-PAG-001,
    /// REQ-PAG-002); the pages are reassembled.
    #[must_use]
    pub fn per_page(mut self, per_page: u32) -> Self {
        self.per_page = Some(per_page);
        self
    }

    /// The Locale helper (MG 8.3.12, REQ-LOC-*).
    #[must_use]
    pub fn locale(mut self, locale: &str) -> Self {
        self.locale = Some(locale.to_owned());
        self
    }

    /// The Query of a `Get ReferenceDesignation` (MG 8.3.9): only the
    /// direct children of a parent; the answer is checked against it, as
    /// a responder without support for the Query answers with all objects
    /// (REQ-RES-REFDES-015).
    #[must_use]
    pub fn query(mut self, query: Query) -> Self {
        self.query = Some(query);
        self
    }

    /// The DataSetWriterId of a `Get Metadata` (MG 8.2.4); without it the
    /// SDK first gets the Data DataSet and uses its DataSetWriterId.
    #[must_use]
    pub fn data_set_writer_id(mut self, id: u16) -> Self {
        self.data_set_writer_id = Some(id);
        self
    }

    fn params(&self) -> Result<GetParams> {
        Ok(GetParams {
            per_page: self.per_page,
            locale: self.locale.clone(),
            query: self.query.clone(),
            data_set_writer_id: self.data_set_writer_id.map(writer_id).transpose()?,
        })
    }
}

/// Options of a `Set` or `Del` ([`Consumer::set`], [`Consumer::del`]).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WriteOptions {
    timeout: Duration,
    data_set_writer_id: Option<u16>,
}

impl Default for WriteOptions {
    fn default() -> Self {
        Self {
            timeout: DEFAULT_TIMEOUT,
            data_set_writer_id: None,
        }
    }
}

impl WriteOptions {
    /// Timeout [`DEFAULT_TIMEOUT`], DataSetWriterId resolved by a `Get`.
    pub fn new() -> Self {
        Self::default()
    }

    /// The time to wait for each answer.
    #[must_use]
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    /// The DataSetWriterId of the DataSetMessage. Without it, a `Set` of
    /// Data, Config or ReferenceDesignation first gets the addressed
    /// DataSet and uses its DataSetWriterId, one of a SubscriptionList
    /// that of the addressed entries; an entry of a PublicationList needs
    /// it.
    #[must_use]
    pub fn data_set_writer_id(mut self, id: u16) -> Self {
        self.data_set_writer_id = Some(id);
        self
    }
}

/// Options of an Action ([`Consumer::new_data_set_writer_id`]).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ActionOptions {
    timeout: Duration,
}

impl Default for ActionOptions {
    fn default() -> Self {
        Self {
            timeout: DEFAULT_TIMEOUT,
        }
    }
}

impl ActionOptions {
    /// Timeout [`DEFAULT_TIMEOUT`].
    pub fn new() -> Self {
        Self::default()
    }

    /// The time to wait for the final response; sent as TimeoutHint
    /// (REQ-ACT-009).
    #[must_use]
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }
}

/// A DataSetWriterId provided by NewDataSetWriterId (REQ-ACT-022): the
/// responder keeps it for this application's `Set` for `ttl`
/// (REQ-ACT-023).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ProvidedWriterId {
    /// The new DataSetWriterId of the responder.
    pub data_set_writer_id: u16,
    /// How long the responder keeps it.
    pub ttl: Duration,
}

fn writer_id(id: u16) -> Result<DataSetWriterId> {
    DataSetWriterId::new(id).map_err(|e| Error::Invalid(e.to_string()))
}

// ---------------------------------------------------------- shared state

type TopicCallback = Arc<dyn Fn(&Received) + Send + Sync>;
type PresenceCallback = Arc<dyn Fn(&Presence) + Send + Sync>;
type Waiter = oneshot::Sender<Result<Outcome>>;

/// The consumer state of an application, locked briefly, never across an
/// await; lock order: this, then the registry.
pub(crate) struct State {
    core: core::Consumer,
    online: bool,
    shut_down: bool,
    waiters: HashMap<RequestId, Waiter>,
    topics: HashMap<SubscriptionId, TopicCallback>,
    presence: HashMap<SubscriptionId, PresenceCallback>,
}

impl State {
    pub(crate) fn new(service_type: ServiceType, app: &IdentificationLink) -> Self {
        Self {
            core: core::Consumer::new(service_type, app.0.clone()),
            online: false,
            shut_down: false,
            waiters: HashMap::new(),
            topics: HashMap::new(),
            presence: HashMap::new(),
        }
    }

    fn dispatch(&mut self) {
        for c in self.core.take_completions() {
            let result = match c.outcome {
                Outcome::Closed if self.shut_down => Err(Error::Closed),
                Outcome::Closed => Err(Error::NotConnected),
                other => Ok(other),
            };
            if let Some(w) = self.waiters.remove(&c.id) {
                // the requester may have given up
                let _ = w.send(result);
            }
        }
    }
}

impl fmt::Debug for State {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("State")
            .field("online", &self.online)
            .field("pending", &self.core.pending())
            .field("subscriptions", &(self.topics.len() + self.presence.len()))
            .finish_non_exhaustive()
    }
}

/// What the runtime does for the consumer after a poll.
pub(crate) struct Step {
    pub(crate) changes: FilterChanges,
    pub(crate) outbound: Vec<Publication>,
    /// The next deadline in Unix milliseconds.
    pub(crate) deadline: Option<u64>,
}

fn next_id(handle: &AppHandle) -> MessageId {
    handle
        .registry()
        .provider
        .lifecycle_mut()
        .next_message_id(unix_ms())
}

/// Connected: requests may be sent.
pub(crate) fn on_connected(handle: &AppHandle) {
    handle.consumer_state().online = true;
}

/// The connection ended (`shutdown` = by [`AppHandle::shutdown`]): the
/// pending requests end, their answers cannot arrive any more.
pub(crate) fn on_disconnected(handle: &AppHandle, shutdown: bool) {
    let mut st = handle.consumer_state();
    st.online = false;
    st.shut_down |= shutdown;
    st.core.close();
    st.dispatch();
}

/// Hands the changed subscriptions to the SubscriptionList of the provider
/// and the settings of `Set` and `Del SubscriptionList` back to the
/// consumer (MG 9.1.9); before every poll of the provider.
pub(crate) fn sync(handle: &AppHandle) {
    let mut st = handle.consumer_state();
    let mut registry = handle.registry();
    if let Some(subs) = st.core.take_subscriptions() {
        registry.provider.set_consumer_subscriptions(subs);
    }
    if let Some(overrides) = registry.provider.take_filter_overrides() {
        st.core.set_filter_overrides(overrides);
    }
}

/// Expires due requests and takes what the session has to do.
pub(crate) fn poll(handle: &AppHandle) -> Step {
    let mut st = handle.consumer_state();
    st.core.expire(unix_ms());
    let step = Step {
        changes: st.core.take_filter_changes(),
        outbound: st.core.take_outbound(),
        deadline: st.core.next_deadline(),
    };
    st.dispatch();
    step
}

/// The SUBACK results of the session.
pub(crate) fn on_subscribed(handle: &AppHandle, results: &[(String, SubscribeReturnCode)]) {
    for (f, code) in results {
        if *code == SubscribeReturnCode::Failure {
            log::warn!("the broker refused the subscription {f}");
        }
    }
    let mut st = handle.consumer_state();
    st.core.on_subscribed(results);
    st.dispatch();
}

/// A received publication: delivered to the subscriptions, the presence
/// callbacks and the pending requests. The callbacks run without a lock.
pub(crate) fn on_message(handle: &AppHandle, topic: &str, payload: &[u8]) {
    let mut calls: Vec<(TopicCallback, Received)> = Vec::new();
    let mut presence: Vec<(PresenceCallback, Presence)> = Vec::new();
    {
        let mut st = handle.consumer_state();
        let incoming = st
            .core
            .handle_message(topic, payload, unix_ms(), &mut || next_id(handle));
        if let Some(e) = incoming.error {
            log::debug!("received publication ignored: {e}");
        }
        for (ids, item) in incoming.deliveries {
            let item = Received::new(item);
            for id in ids {
                if let Some(f) = st.topics.get(&id) {
                    calls.push((Arc::clone(f), item.clone()));
                }
            }
        }
        for change in incoming.presence {
            if let Some(p) = Presence::new(change) {
                for id in &incoming.presence_subscriptions {
                    if let Some(f) = st.presence.get(id) {
                        presence.push((Arc::clone(f), p.clone()));
                    }
                }
            }
        }
        st.dispatch();
    }
    for (f, item) in calls {
        guarded(|| f(&item));
    }
    for (f, p) in presence {
        guarded(|| f(&p));
    }
}

/// Runs a callback; a panic is logged, it does not stop the runtime.
fn guarded(f: impl FnOnce()) {
    if catch_unwind(AssertUnwindSafe(f)).is_err() {
        log::error!("a consumer callback panicked");
    }
}

// ---------------------------------------------------------- subscription

/// A subscription ([`Consumer::subscribe`], [`Consumer::on_presence`]);
/// ends when dropped. Each subscription is an entry of the SubscriptionList
/// of an application of Level 2 or higher (MG 9.1.9, REQ-CMP-039), by
/// default without Interval and not configurable (`NONE_0`);
/// [`Self::interval`] and [`Self::configurable`] change that.
#[must_use = "the subscription ends when dropped"]
pub struct Subscription {
    handle: WeakHandle,
    id: SubscriptionId,
}

impl Subscription {
    /// Processes the publications of a topic at most once per `interval`
    /// (REQ-RES-SUBLIST-003): a publication within the Interval after the
    /// last delivered one of the same topic is dropped. Zero (the default)
    /// delivers every publication; milliseconds beyond `u32::MAX` are
    /// capped. No effect on a presence subscription.
    pub fn interval(self, interval: Duration) -> Self {
        let ms = u32::try_from(interval.as_millis()).unwrap_or(u32::MAX);
        self.configure(|_, configurable| (ms, configurable));
        self
    }

    /// Lets other applications change the Interval of the subscription with
    /// `Set SubscriptionList` and delete it with `Del SubscriptionList`
    /// (`CONF_1`, REQ-RES-SUBLIST-004); a deleted subscription delivers
    /// nothing until a `Set` lists it again. The settings are persisted with
    /// the store of the application and apply to every subscription on the
    /// same topic filter if all of them are configurable. No effect on a
    /// presence subscription.
    pub fn configurable(self) -> Self {
        self.configure(|interval, _| (interval, true));
        self
    }

    fn configure(&self, f: impl FnOnce(u32, bool) -> (u32, bool)) {
        if let Some(handle) = self.handle.upgrade() {
            {
                let mut st = handle.consumer_state();
                let (interval, configurable) = st.core.settings(self.id);
                let (interval, configurable) = f(interval, configurable);
                st.core.configure(self.id, interval, configurable);
            }
            handle.wake();
        }
    }
}

impl Drop for Subscription {
    fn drop(&mut self) {
        if let Some(handle) = self.handle.upgrade() {
            {
                let mut st = handle.consumer_state();
                st.core.unsubscribe(self.id);
                st.topics.remove(&self.id);
                st.presence.remove(&self.id);
            }
            handle.wake();
        }
    }
}

impl fmt::Debug for Subscription {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Subscription")
            .field("id", &self.id)
            .finish_non_exhaustive()
    }
}

// --------------------------------------------------------------- Consumer

/// The consumer API of an application ([`AppHandle::consumer`]); cheap to
/// clone, usable from any thread or task.
#[derive(Clone)]
pub struct Consumer {
    handle: AppHandle,
}

impl fmt::Debug for Consumer {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Consumer")
            .field("app", self.handle.identification_link())
            .finish_non_exhaustive()
    }
}

/// Forgets a request whose future was dropped before its answer.
struct CancelOnDrop<'a> {
    handle: &'a AppHandle,
    id: Option<RequestId>,
}

impl Drop for CancelOnDrop<'_> {
    fn drop(&mut self) {
        if let Some(id) = self.id {
            let mut st = self.handle.consumer_state();
            st.core.cancel(id);
            st.waiters.remove(&id);
            drop(st);
            self.handle.wake();
        }
    }
}

impl Consumer {
    pub(crate) fn new(handle: AppHandle) -> Self {
        Self { handle }
    }

    /// Subscribes to the publications of `resource` at `target`
    /// (`Oi4v2/<ServiceType>/<AppId>/Pub/<Resource>[/<Source>[/<Filter>]]`,
    /// `+` and `#` for what `target` leaves open); `f` gets every
    /// DataSetMessage, also the answers to the `Get` of others.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] if the Filter does not fit the resource
    /// (REQ-TOP-013).
    pub fn subscribe(
        &self,
        resource: Resource,
        target: &Target,
        f: impl Fn(&Received) + Send + Sync + 'static,
    ) -> Result<Subscription> {
        let filter = target.topic_filter(resource)?;
        let id = {
            let mut st = self.handle.consumer_state();
            let id = st.core.subscribe(&[filter]);
            st.topics.insert(id, Arc::new(f));
            id
        };
        self.handle.wake();
        Ok(self.subscription(id))
    }

    /// Reports the presence of the other applications (Birth, Close,
    /// Will, heartbeat; MG 6.3) to `f`; while it exists,
    /// [`Self::apps`] knows them.
    ///
    /// # Errors
    ///
    /// None so far; the `Result` keeps the API open.
    pub fn on_presence(
        &self,
        f: impl Fn(&Presence) + Send + Sync + 'static,
    ) -> Result<Subscription> {
        let id = {
            let mut st = self.handle.consumer_state();
            let id = st.core.subscribe_presence();
            st.presence.insert(id, Arc::new(f));
            id
        };
        self.handle.wake();
        Ok(self.subscription(id))
    }

    fn subscription(&self, id: SubscriptionId) -> Subscription {
        Subscription {
            handle: self.handle.downgrade(),
            id,
        }
    }

    /// The applications seen since the first [`Self::on_presence`].
    pub fn apps(&self) -> Vec<AppInfo> {
        self.handle
            .consumer_state()
            .core
            .apps()
            .into_iter()
            .map(|a| AppInfo {
                service_type: a.service_type,
                app: IdentificationLink(a.app),
                state: a.state,
                mam: a.mam,
                health: a.health.as_ref().map(|h| h.health),
                health_score: a.health.and_then(|h| h.health_score),
            })
            .collect()
    }

    /// Sends a `Get` of `resource` to the application of `target` and
    /// collects every answer with ReplyTo = its MessageId (REQ-MSG-015),
    /// all pages in order (REQ-PAG-002, REQ-PAG-006). An empty list is a
    /// valid answer (REQ-CMP-041).
    ///
    /// # Errors
    ///
    /// [`Error::Status`] for a Status event Bad of the responder, e.g.
    /// `BadNotFound` for an unknown asset (REQ-CMP-047),
    /// [`Error::Timeout`] without (all) answers in time,
    /// [`Error::NotConnected`] while offline, [`Error::Closed`] after a
    /// shutdown, [`Error::Invalid`] for a target without ServiceType and
    /// AppId or a request the guideline does not allow (REQ-TOP-013),
    /// [`Error::Io`] if the broker refused the subscription for the answer.
    pub async fn get(
        &self,
        resource: Resource,
        target: &Target,
        options: &GetOptions,
    ) -> Result<Vec<Received>> {
        let address = target.address(Method::Get, resource)?;
        let mut params = options.params()?;
        if resource == Resource::Metadata && params.data_set_writer_id.is_none() {
            let data = self
                .get_once(
                    Resource::Data,
                    &address,
                    &GetParams::default(),
                    options.timeout,
                )
                .await?;
            let id = data
                .first()
                .map(Received::data_set_writer_id)
                .ok_or(Error::Status(StatusCode::BadNotFound))?;
            params.data_set_writer_id = Some(writer_id(id)?);
        }
        self.get_once(resource, &address, &params, options.timeout)
            .await
    }

    /// [`Self::get`] for threads outside the runtime.
    ///
    /// # Errors
    ///
    /// As [`Self::get`]; [`Error::Invalid`] if called on a thread of a tokio
    /// runtime (use [`Self::get`] there).
    pub fn get_blocking(
        &self,
        resource: Resource,
        target: &Target,
        options: &GetOptions,
    ) -> Result<Vec<Received>> {
        block_on(self.get(resource, target, options))?
    }

    /// Sends a `Set` of `resource` with `payload_json` as the Payload of
    /// its one DataSetMessage and waits for the Status event with ReplyTo =
    /// its MessageId (REQ-RES-EVENT-005); returns its StatusCode, Good or
    /// Bad.
    ///
    /// # Errors
    ///
    /// As [`Self::get`]; [`Error::Invalid`] if `payload_json` is no JSON
    /// object.
    pub async fn set(
        &self,
        resource: Resource,
        target: &Target,
        payload_json: &str,
        options: &WriteOptions,
    ) -> Result<StatusCode> {
        self.write(Method::Set, resource, target, Some(payload_json), options)
            .await
    }

    /// Sends a `Del` of `resource` (with `payload_json` as its one
    /// DataSetMessage where the resource defines one, SF-098; none for a
    /// `Del ReferenceDesignation`) and waits for the Status event.
    ///
    /// # Errors
    ///
    /// As [`Self::set`].
    pub async fn del(
        &self,
        resource: Resource,
        target: &Target,
        payload_json: Option<&str>,
        options: &WriteOptions,
    ) -> Result<StatusCode> {
        self.write(Method::Del, resource, target, payload_json, options)
            .await
    }

    /// [`Self::set`] for threads outside the runtime.
    ///
    /// # Errors
    ///
    /// As [`Self::set`] and [`Self::get_blocking`].
    pub fn set_blocking(
        &self,
        resource: Resource,
        target: &Target,
        payload_json: &str,
        options: &WriteOptions,
    ) -> Result<StatusCode> {
        block_on(self.set(resource, target, payload_json, options))?
    }

    /// [`Self::del`] for threads outside the runtime.
    ///
    /// # Errors
    ///
    /// As [`Self::del`] and [`Self::get_blocking`].
    pub fn del_blocking(
        &self,
        resource: Resource,
        target: &Target,
        payload_json: Option<&str>,
        options: &WriteOptions,
    ) -> Result<StatusCode> {
        block_on(self.del(resource, target, payload_json, options))?
    }

    /// Calls the Action NewDataSetWriterId of the application of `target`
    /// for its asset `target.source` (none: the application itself) and
    /// waits for its final response (MG 9.2.1, REQ-ACT-015): a new
    /// DataSetWriterId of the responder for an entry of `resource` and how
    /// long it is kept (REQ-ACT-022, REQ-ACT-023). TimeoutHint,
    /// CorrelationData and RequestId are set by the SDK (REQ-ACT-009,
    /// REQ-ACT-011).
    ///
    /// # Errors
    ///
    /// [`Error::Status`] for a Bad Status of the response, e.g.
    /// `BadNotFound` for an unknown asset; [`Error::Invalid`] for a target
    /// with a Filter (REQ-ACT-020); otherwise as [`Self::get`].
    pub async fn new_data_set_writer_id(
        &self,
        resource: Resource,
        target: &Target,
        options: &ActionOptions,
    ) -> Result<ProvidedWriterId> {
        let (Some(service_type), Some(app), None) =
            (target.service_type, &target.app, &target.filter)
        else {
            return Err(Error::Invalid(
                "NewDataSetWriterId needs the ServiceType and the AppId and uses no Filter"
                    .to_owned(),
            ));
        };
        let address = Address {
            service_type,
            app: app.0.clone(),
            source: target.source.as_ref().map(|s| s.0.clone()),
            filter: None,
        };
        let outcome = self
            .request(options.timeout, move |c, next, now, deadline| {
                c.new_data_set_writer_id(resource, &address, deadline, now, next)
            })
            .await?;
        match outcome {
            Outcome::WriterId(r) => Ok(ProvidedWriterId {
                data_set_writer_id: r.data_set_writer_id.get(),
                ttl: Duration::from_secs(u64::from(r.ttl)),
            }),
            Outcome::Status(code) => Err(Error::Status(StatusCode::from_code(code.0))),
            other => Err(unexpected(other)),
        }
    }

    /// [`Self::new_data_set_writer_id`] for threads outside the runtime.
    ///
    /// # Errors
    ///
    /// As [`Self::new_data_set_writer_id`] and [`Self::get_blocking`].
    pub fn new_data_set_writer_id_blocking(
        &self,
        resource: Resource,
        target: &Target,
        options: &ActionOptions,
    ) -> Result<ProvidedWriterId> {
        block_on(self.new_data_set_writer_id(resource, target, options))?
    }

    async fn write(
        &self,
        method: Method,
        resource: Resource,
        target: &Target,
        payload_json: Option<&str>,
        options: &WriteOptions,
    ) -> Result<StatusCode> {
        let address = target.address(method, resource)?;
        let entry = match payload_json {
            None => None,
            Some(json) => {
                let payload: serde_json::Value = serde_json::from_str(json)
                    .map_err(|e| Error::Invalid(format!("payload: {e}")))?;
                if !payload.is_object() {
                    return Err(Error::Invalid("the payload is no JSON object".to_owned()));
                }
                let id = match options.data_set_writer_id {
                    Some(id) => id,
                    None => self.resolve_writer(resource, target, options).await?,
                };
                Some(WriteEntry {
                    data_set_writer_id: writer_id(id)?,
                    payload,
                })
            }
        };
        let timeout = options.timeout;
        let outcome = self
            .request(timeout, move |c, next, now, deadline| {
                c.write(
                    method,
                    resource,
                    &address,
                    entry.as_ref(),
                    deadline,
                    now,
                    next,
                )
            })
            .await?;
        match outcome {
            Outcome::Status(code) => Ok(StatusCode::from_code(code.0)),
            other => Err(unexpected(other)),
        }
    }

    /// The DataSetWriterId of the one DataSet a `Set` addresses.
    async fn resolve_writer(
        &self,
        resource: Resource,
        target: &Target,
        options: &WriteOptions,
    ) -> Result<u16> {
        if resource == Resource::SubscriptionList {
            return self.resolve_sublist_writer(target, options).await;
        }
        if !matches!(
            resource,
            Resource::Data | Resource::Config | Resource::ReferenceDesignation
        ) {
            return Err(Error::Invalid(format!(
                "a {resource} entry needs its DataSetWriterId (WriteOptions::data_set_writer_id)"
            )));
        }
        if resource == Resource::ReferenceDesignation {
            return self.resolve_refdes_writer(target, options).await;
        }
        let address = target.address(Method::Get, resource)?;
        let answers = self
            .get_once(resource, &address, &GetParams::default(), options.timeout)
            .await?;
        match answers.as_slice() {
            [one] => Ok(one.data_set_writer_id()),
            [] => Err(Error::Status(StatusCode::BadNotFound)),
            _ => Err(Error::Invalid(format!(
                "{} {resource} DataSets answered; address one with Source and Filter",
                answers.len()
            ))),
        }
    }

    /// The DataSetWriterId of the SubscriptionList of the application of
    /// `target`: one for all its entries (REQ-RES-SUBLIST-001), taken from
    /// the answer to a `Get` of the addressed entries.
    async fn resolve_sublist_writer(&self, target: &Target, options: &WriteOptions) -> Result<u16> {
        let address = target.address(Method::Get, Resource::SubscriptionList)?;
        let answers = self
            .get_once(
                Resource::SubscriptionList,
                &address,
                &GetParams::default(),
                options.timeout,
            )
            .await?;
        answers
            .first()
            .map(Received::data_set_writer_id)
            .ok_or(Error::Status(StatusCode::BadNotFound))
    }

    /// The DataSetWriterId of the ReferenceDesignation of the asset of
    /// `target`, also before it has one: from its PublicationList entry
    /// (ON_REQUEST_SOURCE_10, REQ-RES-PUBLIST-002).
    async fn resolve_refdes_writer(&self, target: &Target, options: &WriteOptions) -> Result<u16> {
        let Some(source) = &target.source else {
            return Err(Error::Invalid(
                "a ReferenceDesignation is set for one asset (Target::source)".to_owned(),
            ));
        };
        let list = Target {
            source: Some(source.clone()),
            filter: Some(Resource::ReferenceDesignation.to_string()),
            ..target.clone()
        };
        let address = list.address(Method::Get, Resource::PublicationList)?;
        let answers = self
            .get_once(
                Resource::PublicationList,
                &address,
                &GetParams::default(),
                options.timeout,
            )
            .await?;
        answers
            .iter()
            .find_map(|r| match r.payload() {
                Payload::PublicationList(e)
                    if e.resource == Resource::ReferenceDesignation
                        && e.source == source.0.as_str() =>
                {
                    Some(e.data_set_writer_id.get())
                }
                _ => None,
            })
            .ok_or(Error::Status(StatusCode::BadNotFound))
    }

    async fn get_once(
        &self,
        resource: Resource,
        address: &Address,
        params: &GetParams,
        timeout: Duration,
    ) -> Result<Vec<Received>> {
        let outcome = self
            .request(timeout, |c, next, now, deadline| {
                c.get(resource, address, params, deadline, now, next)
            })
            .await?;
        match outcome {
            Outcome::Answers(all) => Ok(all.into_iter().map(Received::new).collect()),
            Outcome::Status(code) => Err(Error::Status(StatusCode::from_code(code.0))),
            other => Err(unexpected(other)),
        }
    }

    /// Starts a request in the core and waits for its completion.
    async fn request(
        &self,
        timeout: Duration,
        start: impl FnOnce(
            &mut core::Consumer,
            &mut dyn FnMut() -> MessageId,
            u64,
            u64,
        ) -> std::result::Result<RequestId, ConsumerError>,
    ) -> Result<Outcome> {
        let (tx, rx) = oneshot::channel();
        let handle = &self.handle;
        let id = {
            let mut st = handle.consumer_state();
            if st.shut_down {
                return Err(Error::Closed);
            }
            if !st.online {
                return Err(Error::NotConnected);
            }
            let now = unix_ms();
            let ms = u64::try_from(timeout.as_millis()).unwrap_or(u64::MAX);
            let id = start(
                &mut st.core,
                &mut || next_id(handle),
                now,
                now.saturating_add(ms),
            )
            .map_err(|e| Error::Invalid(e.to_string()))?;
            st.waiters.insert(id, tx);
            id
        };
        handle.wake();
        let mut guard = CancelOnDrop {
            handle,
            id: Some(id),
        };
        let result = rx.await;
        guard.id = None;
        result.unwrap_or(Err(Error::Closed))
    }
}

fn unexpected(outcome: Outcome) -> Error {
    match outcome {
        Outcome::Timeout => Error::Timeout,
        Outcome::Refused(reason) => Error::Io(std::io::Error::new(
            std::io::ErrorKind::PermissionDenied,
            reason,
        )),
        Outcome::Closed => Error::Closed,
        other => Error::Invalid(format!("unexpected answer: {other:?}")),
    }
}

/// Runs `f` to completion on the calling thread; the futures of the
/// consumer wait for a `tokio::sync::oneshot` only, which needs no runtime.
fn block_on<F: Future>(f: F) -> Result<F::Output> {
    struct Unpark(std::thread::Thread);
    impl Wake for Unpark {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
        fn wake_by_ref(self: &Arc<Self>) {
            self.0.unpark();
        }
    }
    if tokio::runtime::Handle::try_current().is_ok() {
        return Err(Error::Invalid(
            "a blocking request on a thread of a tokio runtime; use the async form".to_owned(),
        ));
    }
    let waker = Waker::from(Arc::new(Unpark(std::thread::current())));
    let mut cx = Context::from_waker(&waker);
    let mut f = pin!(f);
    loop {
        if let Poll::Ready(v) = f.as_mut().poll(&mut cx) {
            return Ok(v);
        }
        std::thread::park();
    }
}
