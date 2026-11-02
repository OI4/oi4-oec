//! The consumer side of an application, sans-IO (ADR-0026 §17): typed
//! subscriptions, the directory of applications (Birth, Close, Will,
//! heartbeat), and requests – `Get` with correlation over ReplyTo
//! (REQ-MSG-015), reassembly of paginated answers (MG 8.3.11), the check
//! of a Query answer (REQ-RES-REFDES-015) and timeouts – plus `Set`/`Del`
//! with their Status event (REQ-RES-EVENT-005) and the Action
//! NewDataSetWriterId with its final response (REQ-ACT-015).
//!
//! The consumer never touches the network: the runtime hands it received
//! publications ([`Consumer::handle_message`]), the SUBACK results
//! ([`Consumer::on_subscribed`]) and the clock, and takes the filter
//! changes ([`Consumer::take_filter_changes`]), the requests to publish
//! ([`Consumer::take_outbound`]) and the completions
//! ([`Consumer::take_completions`]). A request is published only after the
//! broker acknowledged the subscriptions its answer arrives on, so that no
//! answer is lost (there are no retained messages on the bus).
//!
//! The lasting subscriptions are the entries of the SubscriptionList of the
//! application (MG 9.1.9, [`Consumer::take_subscriptions`]); the runtime
//! hands the settings of `Set` and `Del SubscriptionList` back
//! ([`Consumer::set_filter_overrides`]): a deleted filter is no longer
//! subscribed, and a subscription with an Interval delivers the
//! publications of a topic at most once per Interval (REQ-RES-SUBLIST-003).

mod directory;
mod received;
mod request;

use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::fmt;

pub use directory::{AppInfo, AppState, PresenceChange};
pub use received::{Payload, Received};
pub use request::{Address, GetParams, WriteEntry};

use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{
    ActionRequest, ActionRequestMessage, ActionResponseMessage, ActionState, ByteString, MessageId,
    NewDataSetWriterIdRequest, NewDataSetWriterIdResponse, Pagination, PublisherId, TimeoutHint,
    is_bad,
};
use crate::mqtt::SubscribeReturnCode;
use crate::provider::{ConsumerSubscription, SubscriptionOverrides};
use crate::resource::ReferenceDesignationQuery;
use crate::status::StatusCode;
use crate::topic::{
    Method, NAMESPACE, Resource, Service, ServiceType, Subject, Topic, TopicError, TopicFilter,
    mqtt_match,
};

use request::Sender;

/// Why a request could not be built.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum ConsumerError {
    /// The topic of the request is not allowed (REQ-TOP-013).
    Topic(TopicError),
    /// The content of the request is not allowed.
    Request(String),
}

impl fmt::Display for ConsumerError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Topic(e) => write!(f, "topic: {e}"),
            Self::Request(d) => write!(f, "request: {d}"),
        }
    }
}

impl std::error::Error for ConsumerError {}

/// Handle of a subscription of the consumer.
pub type SubscriptionId = u64;

/// Handle of a request of the consumer.
pub type RequestId = u64;

/// How a request ended.
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Outcome {
    /// The answer of a `Get`: all resource DataSetMessages of all pages in
    /// page order, filtered by the Query if one was sent.
    Answers(Vec<Received>),
    /// The StatusCode of the Status event: the result of a `Set` or `Del`
    /// (REQ-RES-EVENT-005), or the Bad answer of a `Get` (e.g. BadNotFound
    /// for an unknown Source, REQ-CMP-047), or the Bad Status of the final
    /// ActionResponse of an Action (REQ-ACT-014).
    Status(StatusCode),
    /// The DataSetWriterId provided by NewDataSetWriterId and how long it
    /// stays reserved (REQ-ACT-022, REQ-ACT-023).
    WriterId(NewDataSetWriterIdResponse),
    /// No (complete) answer before the deadline.
    Timeout,
    /// The consumer was closed.
    Closed,
    /// The broker refused a subscription the answer would arrive on.
    Refused(String),
}

/// A finished request.
#[derive(Debug, Clone, PartialEq)]
pub struct Completion {
    /// The request.
    pub id: RequestId,
    /// How it ended.
    pub outcome: Outcome,
}

/// The subscriptions to add to and remove from the session.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FilterChanges {
    /// Topic filters to subscribe.
    pub subscribe: Vec<String>,
    /// Topic filters to unsubscribe.
    pub unsubscribe: Vec<String>,
}

impl FilterChanges {
    /// Whether nothing changes.
    pub fn is_empty(&self) -> bool {
        self.subscribe.is_empty() && self.unsubscribe.is_empty()
    }
}

/// What a received publication means for the consumer.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Incoming {
    /// The resource DataSetMessages for the subscriptions whose filter
    /// matches the topic.
    pub deliveries: Vec<(Vec<SubscriptionId>, Received)>,
    /// Changes of presence.
    pub presence: Vec<PresenceChange>,
    /// The presence subscriptions to tell about [`Incoming::presence`].
    pub presence_subscriptions: Vec<SubscriptionId>,
    /// The raw subscriptions ([`Consumer::subscribe_raw`]) whose filter
    /// matches the topic: they get the publication as received, also one
    /// that cannot be decoded ([`Incoming::error`]).
    pub raw: Vec<SubscriptionId>,
    /// The decoded resource DataSetMessages for [`Incoming::raw`]; empty if
    /// the publication cannot be decoded.
    pub items: Vec<Received>,
    /// Why a publication of interest could not be decoded.
    pub error: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
enum FilterState {
    #[default]
    Pending,
    Acked,
    Failed,
}

#[derive(Debug, Default)]
struct FilterUse {
    users: usize,
    state: FilterState,
}

#[derive(Debug)]
enum SubKind {
    Topics,
    Presence,
    /// Publications as received, with the decoded items (C-087).
    Raw,
}

#[derive(Debug)]
struct Sub {
    kind: SubKind,
    filters: Vec<String>,
    interval_ms: u32,
    configurable: bool,
    /// When a publication of a topic was last delivered (with an Interval).
    last: HashMap<String, u64>,
}

#[derive(Debug)]
enum Kind {
    Get {
        resource: Resource,
        query: Option<ReferenceDesignationQuery>,
        pages: BTreeMap<u32, Vec<Received>>,
        expected: Option<usize>,
    },
    Write,
    /// A `Get MAM` of an application first seen by its heartbeat; its
    /// answer feeds the directory, no completion.
    Discovery,
    /// A `Call NewDataSetWriterId`; the final response is related by
    /// PublisherId, RequestorId, CorrelationData and RequestId
    /// (REQ-ACT-015).
    Action(Box<ActionRequestMessage<NewDataSetWriterIdRequest>>),
}

#[derive(Debug)]
struct Pending {
    message_id: String,
    kind: Kind,
    prefix: String,
    filters: Vec<String>,
    deadline: u64,
    outbound: Option<Publication>,
}

const PRESENCE: [&str; 2] = ["Oi4v2/+/+/Pub/MAM/+", "Oi4v2/+/+/Pub/Health/+"];

/// The consumer of an application.
#[derive(Debug)]
pub struct Consumer {
    app: IdentificationLink,
    publisher: PublisherId,
    discovery_timeout_ms: u64,
    subs: BTreeMap<SubscriptionId, Sub>,
    next_sub: SubscriptionId,
    filters: BTreeMap<String, FilterUse>,
    session: BTreeSet<String>,
    pending: BTreeMap<RequestId, Pending>,
    by_message: HashMap<String, RequestId>,
    next_request: RequestId,
    next_action: u16,
    outbound: Vec<Publication>,
    completions: Vec<Completion>,
    directory: directory::Directory,
    overrides: SubscriptionOverrides,
    subs_changed: bool,
}

impl Consumer {
    /// The consumer of the application `app` of `service_type`; it uses
    /// the PublisherId and the IdentificationLink of the application for
    /// its requests (REQ-MSG-006, REQ-MSG-023).
    pub fn new(service_type: ServiceType, app: IdentificationLink) -> Self {
        Self {
            publisher: PublisherId::new(service_type, app.clone()),
            app,
            discovery_timeout_ms: 10_000,
            subs: BTreeMap::new(),
            next_sub: 1,
            filters: BTreeMap::new(),
            session: BTreeSet::new(),
            pending: BTreeMap::new(),
            by_message: HashMap::new(),
            next_request: 1,
            next_action: 1,
            outbound: Vec::new(),
            completions: Vec::new(),
            directory: directory::Directory::default(),
            overrides: SubscriptionOverrides::new(),
            subs_changed: false,
        }
    }

    // ---- subscriptions --------------------------------------------------

    /// Subscribes to publications on `filters`; [`Consumer::handle_message`]
    /// delivers the matching resource DataSetMessages of `Pub` topics.
    pub fn subscribe(&mut self, filters: &[TopicFilter]) -> SubscriptionId {
        let filters = filters.iter().map(|f| f.as_str().to_owned()).collect();
        self.add_sub(SubKind::Topics, filters)
    }

    /// Subscribes to publications on `filters` as received:
    /// [`Consumer::handle_message`] lists the subscription in
    /// [`Incoming::raw`] for every publication on a matching topic, also
    /// one whose topic or payload cannot be decoded (e.g. to judge it),
    /// with the decoded DataSetMessages in [`Incoming::items`]. Like
    /// [`Consumer::subscribe`] it is an entry of the SubscriptionList.
    pub fn subscribe_raw(&mut self, filters: &[TopicFilter]) -> SubscriptionId {
        let filters = filters.iter().map(|f| f.as_str().to_owned()).collect();
        self.add_sub(SubKind::Raw, filters)
    }

    /// Tracks the presence of all applications (Birth, Close, Will,
    /// heartbeat) and reports its changes to this subscription. While at
    /// least one presence subscription exists, an application first seen
    /// by its heartbeat is asked for its MAM.
    pub fn subscribe_presence(&mut self) -> SubscriptionId {
        let filters = PRESENCE.iter().map(|f| (*f).to_owned()).collect();
        self.add_sub(SubKind::Presence, filters)
    }

    fn add_sub(&mut self, kind: SubKind, filters: Vec<String>) -> SubscriptionId {
        self.acquire(&filters);
        let id = self.next_sub;
        self.next_sub += 1;
        let sub = Sub {
            kind,
            filters,
            interval_ms: 0,
            configurable: false,
            last: HashMap::new(),
        };
        self.subs.insert(id, sub);
        self.subs_changed = true;
        id
    }

    /// Ends a subscription; unknown ids are ignored.
    pub fn unsubscribe(&mut self, id: SubscriptionId) {
        if let Some(sub) = self.subs.remove(&id) {
            self.release(&sub.filters);
            self.subs_changed = true;
        }
    }

    /// Sets the Interval (ms, 0 = every publication) and whether `Set` and
    /// `Del SubscriptionList` may change or delete the subscription `id`
    /// (`CONF_1`, REQ-RES-SUBLIST-004); unknown ids are ignored. A presence
    /// subscription stays `NONE_0` without Interval.
    pub fn configure(&mut self, id: SubscriptionId, interval_ms: u32, configurable: bool) {
        let Some(sub) = self.subs.get_mut(&id) else {
            return;
        };
        if matches!(sub.kind, SubKind::Presence) {
            return;
        }
        if (sub.interval_ms, sub.configurable) != (interval_ms, configurable) {
            sub.interval_ms = interval_ms;
            sub.configurable = configurable;
            self.subs_changed = true;
        }
    }

    /// The Interval (ms) and whether the subscription `id` is configurable;
    /// `(0, false)` for an unknown id.
    pub fn settings(&self, id: SubscriptionId) -> (u32, bool) {
        self.subs
            .get(&id)
            .map_or((0, false), |s| (s.interval_ms, s.configurable))
    }

    /// The lasting subscriptions, one per subscription and topic filter, if
    /// they changed since the last call – the entries of the
    /// SubscriptionList (MG 9.1.9). The answer subscriptions of requests
    /// are transient and not part of it (SF-109).
    pub fn take_subscriptions(&mut self) -> Option<Vec<ConsumerSubscription>> {
        if !std::mem::take(&mut self.subs_changed) {
            return None;
        }
        Some(
            self.subs
                .values()
                .flat_map(|s| {
                    s.filters.iter().map(|f| ConsumerSubscription {
                        topic_path: f.clone(),
                        interval_ms: s.interval_ms,
                        configurable: s.configurable,
                    })
                })
                .collect(),
        )
    }

    /// The settings of `Set` and `Del SubscriptionList` for the
    /// configurable topic filters: a deleted filter is unsubscribed and
    /// delivers nothing; an Interval replaces that of the subscriptions on
    /// the filter.
    pub fn set_filter_overrides(&mut self, overrides: SubscriptionOverrides) {
        self.overrides = overrides;
    }

    /// The Interval a publication on `topic` is delivered with to `sub`:
    /// the smallest of its matching filters that are not deleted, `None`
    /// if none matches.
    fn interval(&self, sub: &Sub, topic: &str) -> Option<u32> {
        sub.filters
            .iter()
            .filter(|f| !self.overrides.is_deleted(f) && mqtt_match(f, topic))
            .map(|f| self.overrides.interval(f).unwrap_or(sub.interval_ms))
            .min()
    }

    fn tracking(&self) -> bool {
        self.subs
            .values()
            .any(|s| matches!(s.kind, SubKind::Presence))
    }

    /// A released filter keeps its state until the session unsubscribes
    /// it ([`Consumer::take_filter_changes`]): used again before, it is
    /// still subscribed and needs no new SUBACK.
    fn release(&mut self, filters: &[String]) {
        for f in filters {
            if let Some(u) = self.filters.get_mut(f) {
                u.users = u.users.saturating_sub(1);
            }
        }
    }

    fn acquire(&mut self, filters: &[String]) {
        for f in filters {
            let u = self.filters.entry(f.clone()).or_default();
            if u.users == 0 && u.state == FilterState::Failed {
                // a refused filter released and used again is subscribed anew
                u.state = FilterState::Pending;
            }
            u.users += 1;
        }
    }

    /// The topic filters to subscribe and unsubscribe at the session since
    /// the last call.
    pub fn take_filter_changes(&mut self) -> FilterChanges {
        // A refused filter is not subscribed again while it is in use; once
        // released, its next use subscribes it anew.
        // A deleted filter stays subscribed while a request waits for its
        // answer on it.
        let requests: BTreeSet<&String> = self.pending.values().flat_map(|p| &p.filters).collect();
        let wanted: BTreeSet<String> = self
            .filters
            .iter()
            .filter(|(f, u)| {
                u.users > 0
                    && u.state != FilterState::Failed
                    && (!self.overrides.is_deleted(f) || requests.contains(f))
            })
            .map(|(f, _)| f.clone())
            .collect();
        let changes = FilterChanges {
            subscribe: wanted.difference(&self.session).cloned().collect(),
            unsubscribe: self.session.difference(&wanted).cloned().collect(),
        };
        self.filters.retain(|_, u| u.users > 0);
        for f in &changes.unsubscribe {
            // a filter subscribed again later waits for a new SUBACK
            if let Some(u) = self.filters.get_mut(f) {
                u.state = FilterState::Pending;
            }
        }
        self.session = wanted;
        changes
    }

    /// Takes the SUBACK results of the session: requests waiting for an
    /// acknowledged filter are released ([`Consumer::take_outbound`]),
    /// requests whose filter was refused end with [`Outcome::Refused`].
    pub fn on_subscribed(&mut self, results: &[(String, SubscribeReturnCode)]) {
        for (filter, code) in results {
            if *code == SubscribeReturnCode::Failure {
                // the session does not hold a refused filter
                self.session.remove(filter);
            }
            if let Some(u) = self.filters.get_mut(filter) {
                u.state = match code {
                    SubscribeReturnCode::Success(_) => FilterState::Acked,
                    SubscribeReturnCode::Failure => FilterState::Failed,
                };
            }
        }
        self.release_ready();
    }

    fn release_ready(&mut self) {
        let mut refused = Vec::new();
        for (id, p) in &mut self.pending {
            if p.outbound.is_none() {
                continue;
            }
            let states: Vec<FilterState> = p
                .filters
                .iter()
                .map(|f| {
                    self.filters
                        .get(f)
                        .map_or(FilterState::Pending, |u| u.state)
                })
                .collect();
            if let Some(i) = states.iter().position(|s| *s == FilterState::Failed) {
                refused.push((*id, p.filters[i].clone()));
            } else if states.iter().all(|s| *s == FilterState::Acked) {
                self.outbound.extend(p.outbound.take());
            }
        }
        for (id, filter) in refused {
            self.finish(
                id,
                Outcome::Refused(format!("the broker refused the subscription {filter}")),
            );
        }
    }

    // ---- requests -------------------------------------------------------

    /// A `Get` of `resource` from the application at `address`
    /// (MG 8.3, MG 9.1); the answer arrives as one [`Completion`].
    ///
    /// # Errors
    ///
    /// [`ConsumerError`] if the request is not allowed, e.g. a Query with a
    /// Source or a `Get Event`.
    pub fn get(
        &mut self,
        resource: Resource,
        address: &Address,
        params: &GetParams,
        deadline_ms: u64,
        now_ms: u64,
        next_id: &mut dyn FnMut() -> MessageId,
    ) -> Result<RequestId, ConsumerError> {
        let topic = address.topic(Method::Get, resource)?;
        let message_id = next_id();
        let payload = request::get(
            resource,
            address,
            params,
            self.sender(message_id.clone(), now_ms),
        )?;
        let kind = Kind::Get {
            resource,
            query: params.query.clone(),
            pages: BTreeMap::new(),
            expected: None,
        };
        let filters = vec![
            answer_filter(address, Some(resource)),
            answer_filter(address, None),
        ];
        Ok(self.start(
            kind,
            address,
            filters,
            &message_id,
            Publication { topic, payload },
            deadline_ms,
        ))
    }

    /// A `Set` (`method` = [`Method::Set`]) or `Del` of `resource` at
    /// `address`; ends with the Status event of the responder
    /// ([`Outcome::Status`]). `entry` is the one DataSetMessage, `None`
    /// only for a `Del ReferenceDesignation` (REQ-RES-REFDES-008).
    ///
    /// # Errors
    ///
    /// [`ConsumerError`] if the request is not allowed.
    #[allow(clippy::too_many_arguments)]
    pub fn write(
        &mut self,
        method: Method,
        resource: Resource,
        address: &Address,
        entry: Option<&WriteEntry>,
        deadline_ms: u64,
        now_ms: u64,
        next_id: &mut dyn FnMut() -> MessageId,
    ) -> Result<RequestId, ConsumerError> {
        let topic = address.topic(method, resource)?;
        let message_id = next_id();
        let payload = request::write(
            method,
            resource,
            address,
            entry,
            self.sender(message_id.clone(), now_ms),
        )?;
        let filters = vec![answer_filter(address, None)];
        Ok(self.start(
            Kind::Write,
            address,
            filters,
            &message_id,
            Publication { topic, payload },
            deadline_ms,
        ))
    }

    /// The Action NewDataSetWriterId of the application at `address` for
    /// the asset of its Source (MG 8.4.1, REQ-ACT-020): a DataSetWriterId
    /// for `resource`; ends with [`Outcome::WriterId`], or with the Bad
    /// Status of the response ([`Outcome::Status`]). The TimeoutHint is the
    /// time until `deadline_ms` (REQ-ACT-009).
    ///
    /// # Errors
    ///
    /// [`ConsumerError`] if `address` has a Filter (REQ-ACT-020).
    pub fn new_data_set_writer_id(
        &mut self,
        resource: Resource,
        address: &Address,
        deadline_ms: u64,
        now_ms: u64,
        next_id: &mut dyn FnMut() -> MessageId,
    ) -> Result<RequestId, ConsumerError> {
        if address.filter.is_some() {
            return Err(ConsumerError::Request(
                "NewDataSetWriterId uses no Filter (REQ-ACT-020)".to_owned(),
            ));
        }
        let topic = Topic::new(
            address.service_type,
            address.app.clone(),
            Method::Call,
            Service::NewDataSetWriterId,
            address.source.clone(),
            None,
        )
        .map_err(ConsumerError::Topic)?;
        let message_id = next_id();
        let timeout = u32::try_from(deadline_ms.saturating_sub(now_ms)).unwrap_or(u32::MAX);
        let timeout = core::num::NonZeroU32::new(timeout).unwrap_or(core::num::NonZeroU32::MIN);
        // the consumer RequestId is the CorrelationData (REQ-ACT-007)
        let correlation = ByteString::from_bytes(&self.next_request.to_be_bytes());
        let first = ActionRequest::common(
            self.action_request_id(),
            NewDataSetWriterIdRequest { resource },
        );
        let request = ActionRequestMessage::new(
            &topic,
            self.publisher.clone(),
            message_id.clone(),
            request::date_time(now_ms),
            TimeoutHint::from_millis(timeout),
            first,
        )
        .map_err(|e| ConsumerError::Request(e.to_string()))?
        .with_correlation_data(correlation);
        let payload =
            serde_json::to_vec(&request).map_err(|e| ConsumerError::Request(e.to_string()))?;
        let reply = request.response_address().to_string();
        let filters = vec![reply.clone()];
        let id = self.start(
            Kind::Action(Box::new(request)),
            address,
            filters,
            &message_id,
            Publication { topic, payload },
            deadline_ms,
        );
        if let Some(p) = self.pending.get_mut(&id) {
            p.prefix = reply;
        }
        Ok(id)
    }

    /// A RequestId unique among the pending ActionRequests (REQ-ACT-011).
    fn action_request_id(&mut self) -> u16 {
        let used: BTreeSet<u16> = self
            .pending
            .values()
            .filter_map(|p| match &p.kind {
                Kind::Action(r) => r.messages().first().map(ActionRequest::request_id),
                _ => None,
            })
            .collect();
        loop {
            let id = self.next_action;
            self.next_action = self.next_action.wrapping_add(1);
            if !used.contains(&id) {
                return id;
            }
        }
    }

    fn sender(&self, message_id: MessageId, unix_ms: u64) -> Sender<'_> {
        Sender {
            app: &self.app,
            publisher: &self.publisher,
            message_id,
            unix_ms,
        }
    }

    fn start(
        &mut self,
        kind: Kind,
        address: &Address,
        filters: Vec<String>,
        message_id: &MessageId,
        publication: Publication,
        deadline: u64,
    ) -> RequestId {
        self.acquire(&filters);
        let id = self.next_request;
        self.next_request += 1;
        self.by_message.insert(message_id.as_str().to_owned(), id);
        self.pending.insert(
            id,
            Pending {
                message_id: message_id.as_str().to_owned(),
                kind,
                prefix: address.pub_prefix(),
                filters,
                deadline,
                outbound: Some(publication),
            },
        );
        self.release_ready();
        id
    }

    /// The requests to publish now: their answer subscriptions are
    /// acknowledged.
    pub fn take_outbound(&mut self) -> Vec<Publication> {
        std::mem::take(&mut self.outbound)
    }

    /// The finished requests since the last call.
    pub fn take_completions(&mut self) -> Vec<Completion> {
        std::mem::take(&mut self.completions)
    }

    /// The earliest deadline of a pending request.
    pub fn next_deadline(&self) -> Option<u64> {
        self.pending.values().map(|p| p.deadline).min()
    }

    /// Ends every request whose deadline has passed with
    /// [`Outcome::Timeout`].
    pub fn expire(&mut self, now_ms: u64) {
        let due: Vec<RequestId> = self
            .pending
            .iter()
            .filter(|(_, p)| p.deadline <= now_ms)
            .map(|(id, _)| *id)
            .collect();
        for id in due {
            self.finish(id, Outcome::Timeout);
        }
    }

    /// Forgets a request without a completion.
    pub fn cancel(&mut self, id: RequestId) {
        if let Some(p) = self.pending.remove(&id) {
            self.by_message.remove(&p.message_id);
            self.release(&p.filters);
        }
    }

    /// Ends every pending request with [`Outcome::Closed`].
    pub fn close(&mut self) {
        let all: Vec<RequestId> = self.pending.keys().copied().collect();
        for id in all {
            self.finish(id, Outcome::Closed);
        }
    }

    /// The number of pending requests.
    pub fn pending(&self) -> usize {
        self.pending.len()
    }

    fn finish(&mut self, id: RequestId, outcome: Outcome) {
        let Some(p) = self.pending.remove(&id) else {
            return;
        };
        self.by_message.remove(&p.message_id);
        self.release(&p.filters);
        let outcome = match (p.kind, outcome) {
            (Kind::Discovery, _) => return,
            (Kind::Get { query, pages, .. }, Outcome::Answers(_)) => {
                let mut all: Vec<Received> = pages.into_values().flatten().collect();
                if let Some(q) = query {
                    all.retain(|r| match &r.payload {
                        Payload::ReferenceDesignation(rd) => q.matches(rd),
                        _ => false,
                    });
                }
                Outcome::Answers(all)
            }
            (_, o) => o,
        };
        self.completions.push(Completion { id, outcome });
    }

    // ---- received publications ------------------------------------------

    /// The applications seen since the first presence subscription.
    pub fn apps(&self) -> Vec<AppInfo> {
        self.directory.apps()
    }

    /// Takes a publication received on `topic`. `now_ms` and `next_id`
    /// serve the `Get MAM` of an application first seen by its heartbeat.
    pub fn handle_message(
        &mut self,
        topic: &str,
        payload: &[u8],
        now_ms: u64,
        next_id: &mut dyn FnMut() -> MessageId,
    ) -> Incoming {
        let mut incoming = Incoming {
            raw: self.due(topic, now_ms, true),
            ..Incoming::default()
        };
        let Ok(parsed) = Topic::parse(topic) else {
            if !incoming.raw.is_empty() {
                incoming.error = Some(format!("{topic}: not a topic of the guideline"));
            }
            return incoming;
        };
        if parsed.method() == Method::Reply {
            incoming.error = self.reply(topic, payload).err();
            return incoming;
        }
        let Subject::Resource(resource) = parsed.subject() else {
            return incoming;
        };
        if parsed.method() != Method::Pub {
            return incoming;
        }
        let subs = self.due(topic, now_ms, false);
        let presence = self.tracking() && matches!(resource, Resource::Mam | Resource::Health);
        let answers = self
            .pending
            .values()
            .any(|p| !matches!(p.kind, Kind::Action(_)) && topic.starts_with(&p.prefix));
        if subs.is_empty() && !presence && !answers && incoming.raw.is_empty() {
            return incoming;
        }
        let decoded = match received::decode(&parsed, payload) {
            Ok(d) => d,
            Err(e) => {
                incoming.error = Some(format!("{topic}: {e}"));
                return incoming;
            }
        };
        if presence {
            let (changes, discover) =
                self.directory
                    .observe(&parsed, decoded.reply_to.as_ref(), &decoded.items);
            if let Some((st, app)) = discover {
                self.discover(st, app, now_ms, next_id);
            }
            if !changes.is_empty() {
                incoming.presence_subscriptions = self
                    .subs
                    .iter()
                    .filter(|(_, s)| matches!(s.kind, SubKind::Presence))
                    .map(|(id, _)| *id)
                    .collect();
                incoming.presence = changes;
            }
        }
        if answers {
            self.answer(
                topic,
                resource,
                decoded.pagination.as_ref(),
                decoded.reply_to.as_ref(),
                &decoded.items,
            );
        }
        if !incoming.raw.is_empty() {
            incoming.items.clone_from(&decoded.items);
        }
        if !subs.is_empty() {
            incoming.deliveries = decoded
                .items
                .into_iter()
                .map(|r| (subs.clone(), r))
                .collect();
        }
        incoming
    }

    /// The topic subscriptions (`raw`: the raw ones) a publication on
    /// `topic` is delivered to now: a matching filter that is not deleted,
    /// and the Interval since the last delivery of the topic passed
    /// (REQ-RES-SUBLIST-003).
    fn due(&mut self, topic: &str, now_ms: u64, raw: bool) -> Vec<SubscriptionId> {
        let intervals: Vec<(SubscriptionId, u32)> = self
            .subs
            .iter()
            .filter(|(_, s)| {
                if raw {
                    matches!(s.kind, SubKind::Raw)
                } else {
                    matches!(s.kind, SubKind::Topics)
                }
            })
            .filter_map(|(id, s)| Some((*id, self.interval(s, topic)?)))
            .collect();
        let mut due = Vec::new();
        for (id, interval) in intervals {
            let sub = self.subs.get_mut(&id).expect("listed above");
            if interval == 0 {
                sub.last.remove(topic);
            } else {
                if let Some(last) = sub.last.get(topic)
                    && now_ms.saturating_sub(*last) < u64::from(interval)
                {
                    continue;
                }
                sub.last.insert(topic.to_owned(), now_ms);
            }
            due.push(id);
        }
        due
    }

    fn discover(
        &mut self,
        service_type: ServiceType,
        app: IdentificationLink,
        now_ms: u64,
        next_id: &mut dyn FnMut() -> MessageId,
    ) {
        let address = Address {
            service_type,
            app: app.clone(),
            source: Some(app),
            filter: None,
        };
        let Ok(topic) = address.topic(Method::Get, Resource::Mam) else {
            return;
        };
        let message_id = next_id();
        let Ok(payload) = request::get(
            Resource::Mam,
            &address,
            &GetParams::default(),
            self.sender(message_id.clone(), now_ms),
        ) else {
            return;
        };
        let filters = vec![answer_filter(&address, Some(Resource::Mam))];
        let deadline = now_ms.saturating_add(self.discovery_timeout_ms);
        self.start(
            Kind::Discovery,
            &address,
            filters,
            &message_id,
            Publication { topic, payload },
            deadline,
        );
    }

    /// A `ua-action-response` on the ResponseAddress of a pending Action:
    /// its final response for the ActionRequest ends the request; an
    /// intermediate one (ActionState 1, REQ-ACT-016) and responses to other
    /// requests are ignored.
    fn reply(&mut self, topic: &str, payload: &[u8]) -> Result<(), String> {
        if !self
            .pending
            .values()
            .any(|p| matches!(p.kind, Kind::Action(_)) && p.outbound.is_none() && p.prefix == topic)
        {
            return Ok(());
        }
        let message: ActionResponseMessage<NewDataSetWriterIdResponse> =
            serde_json::from_slice(payload).map_err(|e| format!("{topic}: {e}"))?;
        let done = self.pending.iter().find_map(|(id, p)| {
            let Kind::Action(request) = &p.kind else {
                return None;
            };
            if p.outbound.is_some() || p.prefix != topic || !message.correlates(request) {
                return None;
            }
            let first = request.messages().first()?;
            let response = message
                .messages()
                .iter()
                .find(|r| r.answers(first) && r.action_state() == ActionState::Done)?;
            let outcome = match (response.status(), response.payload()) {
                (Some(code), _) if is_bad(code.get()) => Outcome::Status(StatusCode(code.get())),
                (_, Some(r)) => Outcome::WriterId(*r),
                // a Good response without its fields
                (_, None) => Outcome::Status(StatusCode::BAD_DECODING_ERROR),
            };
            Some((*id, outcome))
        });
        if let Some((id, outcome)) = done {
            self.finish(id, outcome);
        }
        Ok(())
    }

    fn answer(
        &mut self,
        topic: &str,
        resource: Resource,
        pagination: Option<&Pagination>,
        reply_to: Option<&MessageId>,
        items: &[Received],
    ) {
        let Some(id) = reply_to
            .and_then(|m| self.by_message.get(m.as_str()))
            .copied()
        else {
            return;
        };
        let Some(p) = self.pending.get_mut(&id) else {
            return;
        };
        if !topic.starts_with(&p.prefix) || p.outbound.is_some() {
            return;
        }
        if resource == Resource::Event {
            let Some(code) = items.iter().find_map(|r| r.payload.status()) else {
                return;
            };
            match p.kind {
                Kind::Write => self.finish(id, Outcome::Status(code)),
                Kind::Get { .. } | Kind::Discovery if !code.is_good() => {
                    self.finish(id, Outcome::Status(code));
                }
                _ => {}
            }
            return;
        }
        let done = match &mut p.kind {
            Kind::Get {
                resource: wanted,
                pages,
                expected,
                ..
            } if *wanted == resource => add_page(pages, expected, pagination, items),
            Kind::Discovery if resource == Resource::Mam => true,
            _ => false,
        };
        if done {
            self.finish(id, Outcome::Answers(Vec::new()));
        }
    }
}

/// Adds a page of a `Get` answer; `true` once the answer is complete.
fn add_page(
    pages: &mut BTreeMap<u32, Vec<Received>>,
    expected: &mut Option<usize>,
    pagination: Option<&Pagination>,
    items: &[Received],
) -> bool {
    let Some(pg) = pagination else {
        pages.insert(0, items.to_vec());
        return true;
    };
    pages.insert(pg.page, items.to_vec());
    if expected.is_none() {
        let per_page = pg.per_page.max(1);
        let count = pg.total_count.div_ceil(per_page).max(1);
        *expected = Some(usize::try_from(count).unwrap_or(usize::MAX));
    }
    expected.is_some_and(|n| pages.len() >= n)
}

/// The topic filter the answers of a request arrive on: the resource
/// publications of the responder, or (`None`) its Status events.
fn answer_filter(address: &Address, resource: Option<Resource>) -> String {
    let base = format!(
        "{NAMESPACE}/{}/{}/Pub",
        address.service_type,
        address.app.encode()
    );
    match resource {
        Some(r) => format!("{base}/{r}/#"),
        None => format!("{base}/Event/+/Status/+"),
    }
}
