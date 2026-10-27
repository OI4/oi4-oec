//! The consumer side of an application, sans-IO (ADR-0026 §17): typed
//! subscriptions, the directory of applications (Birth, Close, Will,
//! heartbeat), and requests – `Get` with correlation over ReplyTo
//! (REQ-MSG-015), reassembly of paginated answers (MG 8.3.11), the check
//! of a Query answer (REQ-RES-REFDES-015) and timeouts – plus `Set`/`Del`
//! with their Status event (REQ-RES-EVENT-005).
//!
//! The consumer never touches the network: the runtime hands it received
//! publications ([`Consumer::handle_message`]), the SUBACK results
//! ([`Consumer::on_subscribed`]) and the clock, and takes the filter
//! changes ([`Consumer::take_filter_changes`]), the requests to publish
//! ([`Consumer::take_outbound`]) and the completions
//! ([`Consumer::take_completions`]). A request is published only after the
//! broker acknowledged the subscriptions its answer arrives on, so that no
//! answer is lost (there are no retained messages on the bus).

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
use crate::message::{MessageId, Pagination, PublisherId};
use crate::mqtt::SubscribeReturnCode;
use crate::resource::ReferenceDesignationQuery;
use crate::status::StatusCode;
use crate::topic::{
    Method, NAMESPACE, Resource, ServiceType, Subject, Topic, TopicError, TopicFilter, mqtt_match,
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
    /// for an unknown Source, REQ-CMP-047).
    Status(StatusCode),
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
}

#[derive(Debug)]
struct Sub {
    kind: SubKind,
    filters: Vec<String>,
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
    outbound: Vec<Publication>,
    completions: Vec<Completion>,
    directory: directory::Directory,
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
            outbound: Vec::new(),
            completions: Vec::new(),
            directory: directory::Directory::default(),
        }
    }

    // ---- subscriptions --------------------------------------------------

    /// Subscribes to publications on `filters`; [`Consumer::handle_message`]
    /// delivers the matching resource DataSetMessages of `Pub` topics.
    pub fn subscribe(&mut self, filters: &[TopicFilter]) -> SubscriptionId {
        let filters = filters.iter().map(|f| f.as_str().to_owned()).collect();
        self.add_sub(SubKind::Topics, filters)
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
        for f in &filters {
            self.filters.entry(f.clone()).or_default().users += 1;
        }
        let id = self.next_sub;
        self.next_sub += 1;
        self.subs.insert(id, Sub { kind, filters });
        id
    }

    /// Ends a subscription; unknown ids are ignored.
    pub fn unsubscribe(&mut self, id: SubscriptionId) {
        if let Some(sub) = self.subs.remove(&id) {
            self.release(&sub.filters);
        }
    }

    fn tracking(&self) -> bool {
        self.subs
            .values()
            .any(|s| matches!(s.kind, SubKind::Presence))
    }

    fn release(&mut self, filters: &[String]) {
        for f in filters {
            if let Some(u) = self.filters.get_mut(f) {
                u.users -= 1;
                if u.users == 0 {
                    self.filters.remove(f);
                }
            }
        }
    }

    fn acquire(&mut self, filters: &[String]) {
        for f in filters {
            self.filters.entry(f.clone()).or_default().users += 1;
        }
    }

    /// The topic filters to subscribe and unsubscribe at the session since
    /// the last call.
    pub fn take_filter_changes(&mut self) -> FilterChanges {
        // A refused filter is not subscribed again while it is in use; once
        // released, its next use subscribes it anew.
        let wanted: BTreeSet<String> = self
            .filters
            .iter()
            .filter(|(_, u)| u.state != FilterState::Failed)
            .map(|(f, _)| f.clone())
            .collect();
        let changes = FilterChanges {
            subscribe: wanted.difference(&self.session).cloned().collect(),
            unsubscribe: self.session.difference(&wanted).cloned().collect(),
        };
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
        let mut incoming = Incoming::default();
        let Ok(parsed) = Topic::parse(topic) else {
            return incoming;
        };
        let Subject::Resource(resource) = parsed.subject() else {
            return incoming;
        };
        if parsed.method() != Method::Pub {
            return incoming;
        }
        let subs: Vec<SubscriptionId> = self
            .subs
            .iter()
            .filter(|(_, s)| {
                matches!(s.kind, SubKind::Topics) && s.filters.iter().any(|f| mqtt_match(f, topic))
            })
            .map(|(id, _)| *id)
            .collect();
        let presence = self.tracking() && matches!(resource, Resource::Mam | Resource::Health);
        let answers = self.pending.values().any(|p| topic.starts_with(&p.prefix));
        if subs.is_empty() && !presence && !answers {
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
        if !subs.is_empty() {
            incoming.deliveries = decoded
                .items
                .into_iter()
                .map(|r| (subs.clone(), r))
                .collect();
        }
        incoming
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
