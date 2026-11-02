//! The consumer side (P4 increment 8a, ADR-0026 §17), without network:
//! the requests are the guideline vectors (V-PAY-002/007/020/046/053/055/
//! 061/100), pass the strict schema, and are answered by the real
//! [`Provider`]: `Get` correlated over the ReplyTo (REQ-MSG-015), paginated
//! answers reassembled (REQ-PAG-002 … 005), the Query answer checked
//! (REQ-RES-REFDES-015, REQ-QRY-004), BadNotFound for an unknown Source
//! (REQ-CMP-047), `Set`/`Del` ended by their Status event
//! (REQ-RES-EVENT-005); the directory follows Birth, Close, Will and the
//! heartbeat (REQ-LIFE-*); requests wait for their SUBACK and time out.

use std::time::Duration;

use oec_core::consumer::{
    Address, AppState, Completion, Consumer, GetParams, Outcome, Payload, PresenceChange, Received,
    WriteEntry,
};
use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::{DataSetWriterId, MessageId, MessageIds, PublisherId};
use oec_core::mqtt::{QoS, SubscribeReturnCode};
use oec_core::provider::data::{BuiltinType as T, DataSetSpec, FieldSpec, Mode, Value};
use oec_core::provider::{
    ConfigSpec, Handled, Provider, ProviderConfig, StoredReferenceDesignations, WriterIds,
};
use oec_core::resource::{Aspect, Mam, ReferenceDesignationQuery};
use oec_core::status::StatusCode;
use oec_core::topic::{Filter, Method, Resource, ServiceType, TopicFilter};
use proptest::prelude::*;
use serde_json::{Value as Json, json};

const CONSUMER: &str = "https://id.consumer.com/MEService/Inst07322";
const PROVIDER: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const MOTOR: &str = "groe.mca/DeviceModelB/AC33.447/20443";
const VALVE: &str = "groe.mca/Valve/V-1/1";
const CTX: &str = "Overall device settings";
const MS: u64 = 1_639_693_510_000;
const TIMEOUT: u64 = 10_000;

fn il(s: &str) -> IdentificationLink {
    IdentificationLink::parse(s).unwrap()
}

fn mam(piu: &str) -> Mam {
    serde_json::from_value(json!({
        "Manufacturer": {"Locale": "en-US", "Text": "Provider"},
        "ManufacturerUri": "provider.com",
        "Model": {"Locale": "en-US", "Text": "FieldDataService"},
        "ProductCode": "FDS-001",
        "HardwareRevision": "",
        "SoftwareRevision": "1.0.0",
        "DeviceRevision": "",
        "DeviceManual": "",
        "DeviceClass": "Oi4.OTConnector",
        "SerialNumber": "1200-0345",
        "ProductInstanceUri": piu,
        "RevisionCounter": 0
    }))
    .unwrap()
}

fn vector(id: &str) -> (String, Json) {
    let file = concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../spec/export/vectors/payload-guideline.json"
    );
    let all: Json = serde_json::from_str(&std::fs::read_to_string(file).unwrap()).unwrap();
    let v = all["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap();
    (
        v["topic"].as_str().unwrap().to_owned(),
        serde_json::from_str(v["payload"].as_str().unwrap()).unwrap(),
    )
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

/// Checks a generated message against the strict schema (feature
/// `strict-validation`).
fn validate(topic: &str, payload: &[u8]) {
    #[cfg(feature = "strict-validation")]
    {
        use oec_core::schema::{Flavor, Validator};
        use oec_core::topic::Topic;
        let v: Json = serde_json::from_slice(payload).unwrap();
        Validator::new()
            .unwrap()
            .validate_topic(&Topic::parse(topic).unwrap(), Flavor::Strict, &v)
            .unwrap_or_else(|e| panic!("{topic}: {e}"));
    }
    #[cfg(not(feature = "strict-validation"))]
    let _ = (topic, payload);
}

fn consumer() -> Consumer {
    Consumer::new(ServiceType::ITConnector, il(CONSUMER))
}

fn ok(filters: &[String]) -> Vec<(String, SubscribeReturnCode)> {
    filters
        .iter()
        .map(|f| (f.clone(), SubscribeReturnCode::Success(QoS::AtLeastOnce)))
        .collect()
}

/// Acknowledges every new filter of the consumer.
fn ack(c: &mut Consumer) {
    let changes = c.take_filter_changes();
    c.on_subscribed(&ok(&changes.subscribe));
}

fn provider_at() -> Address {
    Address::app(ServiceType::OTConnector, il(PROVIDER))
}

fn at(source: &str, filter: Option<Filter>) -> Address {
    Address {
        source: Some(il(source)),
        filter,
        ..provider_at()
    }
}

fn tag(t: &str) -> Filter {
    Filter::Tag(t.to_owned())
}

// ------------------------------------------------------------- requests

/// The one request the consumer builds with the MessageId of the vector,
/// compared with the vector.
fn assert_request(
    id: &str,
    ms: u64,
    build: impl FnOnce(&mut Consumer, &mut dyn FnMut() -> MessageId),
) {
    let (topic, expected) = vector(id);
    let mut c = consumer();
    let publisher = PublisherId::new(ServiceType::ITConnector, il(CONSUMER));
    let mut next = || MessageId::new(ms, None, &publisher);
    build(&mut c, &mut next);
    ack(&mut c);
    let out = c.take_outbound();
    assert_eq!(out.len(), 1, "{id}");
    assert_eq!(out[0].topic.to_string(), topic, "{id}");
    validate(&topic, &out[0].payload);
    assert_eq!(json_of(&out[0]), expected, "{id}");
}

fn get(
    resource: Resource,
    address: Address,
    params: GetParams,
) -> impl FnOnce(&mut Consumer, &mut dyn FnMut() -> MessageId) {
    move |c, next| {
        c.get(resource, &address, &params, MS + TIMEOUT, MS, next)
            .unwrap();
    }
}

fn write(
    method: Method,
    resource: Resource,
    address: Address,
    entry: Option<WriteEntry>,
) -> impl FnOnce(&mut Consumer, &mut dyn FnMut() -> MessageId) {
    move |c, next| {
        c.write(
            method,
            resource,
            &address,
            entry.as_ref(),
            MS + TIMEOUT,
            MS,
            next,
        )
        .unwrap();
    }
}

fn entry(id: u16, vector_id: &str) -> WriteEntry {
    WriteEntry {
        data_set_writer_id: DataSetWriterId::new(id).unwrap(),
        payload: vector(vector_id).1["Messages"][0]["Payload"].clone(),
    }
}

#[test]
fn get_requests_are_the_guideline_vectors() {
    let none = GetParams::default;
    assert_request(
        "V-PAY-002",
        MS,
        get(Resource::Health, provider_at(), none()),
    );
    let locale = GetParams {
        locale: Some("de-DE".to_owned()),
        ..none()
    };
    assert_request("V-PAY-007", MS, get(Resource::Mam, provider_at(), locale));
    assert_request(
        "V-PAY-055",
        MS,
        get(Resource::Data, at(PROVIDER, Some(tag("oee"))), none()),
    );
    let meta = GetParams {
        data_set_writer_id: Some(DataSetWriterId::new(93).unwrap()),
        ..none()
    };
    assert_request(
        "V-PAY-020",
        MS,
        get(Resource::Metadata, at(PROVIDER, Some(tag("oee"))), meta),
    );
    let query = GetParams {
        query: Some(ReferenceDesignationQuery {
            parent: "consumer.com/LineController/LC-200/4711".to_owned(),
            aspect: Some(Aspect::Function),
        }),
        ..none()
    };
    let refdes = get(Resource::ReferenceDesignation, provider_at(), query);
    assert_request("V-PAY-053", MS + 20_000, refdes);
}

#[test]
fn write_requests_are_the_guideline_vectors() {
    let set_data = write(
        Method::Set,
        Resource::Data,
        at(PROVIDER, Some(tag("oee"))),
        Some(entry(93, "V-PAY-061")),
    );
    assert_request("V-PAY-061", MS, set_data);
    let ctx = Some(Filter::ContextName(CTX.to_owned()));
    let set_config = write(
        Method::Set,
        Resource::Config,
        at(PROVIDER, ctx),
        Some(entry(43, "V-PAY-100")),
    );
    assert_request("V-PAY-100", MS, set_config);
    let del = write(
        Method::Del,
        Resource::ReferenceDesignation,
        at(PROVIDER, None),
        None,
    );
    assert_request("V-PAY-046", MS + 10_000, del);
}

#[test]
fn requests_outside_the_matrix_are_refused() {
    let mut c = consumer();
    let ids = MessageIds::new(PublisherId::new(ServiceType::ITConnector, il(CONSUMER)));
    let mut ids = ids;
    let mut next = || ids.next_id(MS);
    let p = GetParams::default();
    // no Get Event, Query only without Source, Set Data needs Source and Tag
    assert!(
        c.get(Resource::Event, &provider_at(), &p, MS, MS, &mut next)
            .is_err()
    );
    let q = GetParams {
        query: Some(ReferenceDesignationQuery {
            parent: PROVIDER.to_owned(),
            aspect: None,
        }),
        ..GetParams::default()
    };
    assert!(
        c.get(
            Resource::ReferenceDesignation,
            &at(PROVIDER, None),
            &q,
            MS,
            MS,
            &mut next
        )
        .is_err()
    );
    assert!(
        c.get(Resource::Mam, &provider_at(), &q, MS, MS, &mut next)
            .is_err()
    );
    let e = entry(93, "V-PAY-061");
    assert!(
        c.write(
            Method::Set,
            Resource::Data,
            &provider_at(),
            Some(&e),
            MS,
            MS,
            &mut next
        )
        .is_err()
    );
    assert!(
        c.write(
            Method::Set,
            Resource::Mam,
            &at(PROVIDER, None),
            Some(&e),
            MS,
            MS,
            &mut next
        )
        .is_err()
    );
    // a Del ReferenceDesignation carries nothing, a Set exactly one
    let del = c.write(
        Method::Del,
        Resource::ReferenceDesignation,
        &at(PROVIDER, None),
        Some(&e),
        MS,
        MS,
        &mut next,
    );
    assert!(del.is_err());
    let set = c.write(
        Method::Set,
        Resource::Data,
        &at(PROVIDER, Some(tag("oee"))),
        None,
        MS,
        MS,
        &mut next,
    );
    assert!(set.is_err());
    // a Get Metadata needs the DataSetWriterId
    assert!(
        c.get(
            Resource::Metadata,
            &at(PROVIDER, Some(tag("oee"))),
            &p,
            MS,
            MS,
            &mut next
        )
        .is_err()
    );
    assert_eq!(c.pending(), 0);
    assert!(c.take_filter_changes().is_empty());
}

// ------------------------------------------------------------ the loop

/// A consumer and the real provider on a lossless bus that acknowledges
/// every subscription.
struct Bus {
    c: Consumer,
    p: Provider,
    ids: MessageIds,
    now: u64,
    /// The filters the consumer unsubscribed.
    unsubscribed: Vec<String>,
}

impl Bus {
    fn new(p: Provider) -> Self {
        Self {
            c: consumer(),
            p,
            ids: MessageIds::new(PublisherId::new(ServiceType::ITConnector, il(CONSUMER))),
            now: MS,
            unsubscribed: Vec::new(),
        }
    }

    fn get(&mut self, resource: Resource, address: &Address, params: &GetParams) -> u64 {
        let (ids, now) = (&mut self.ids, self.now);
        let mut next = || ids.next_id(now);
        self.c
            .get(resource, address, params, now + TIMEOUT, now, &mut next)
            .unwrap()
    }

    fn write(
        &mut self,
        method: Method,
        resource: Resource,
        address: &Address,
        e: Option<&WriteEntry>,
    ) -> u64 {
        let (ids, now) = (&mut self.ids, self.now);
        let mut next = || ids.next_id(now);
        self.c
            .write(method, resource, address, e, now + TIMEOUT, now, &mut next)
            .unwrap()
    }

    fn action(&mut self, resource: Resource, address: &Address) -> u64 {
        let (ids, now) = (&mut self.ids, self.now);
        let mut next = || ids.next_id(now);
        self.c
            .new_data_set_writer_id(resource, address, now + TIMEOUT, now, &mut next)
            .unwrap()
    }

    /// Delivers provider publications to the consumer.
    fn deliver(&mut self, ps: Vec<Publication>) -> Vec<oec_core::consumer::Incoming> {
        let (ids, now) = (&mut self.ids, self.now);
        let mut next = || ids.next_id(now);
        ps.into_iter()
            .map(|p| {
                let t = p.topic.to_string();
                validate(&t, &p.payload);
                self.c.handle_message(&t, &p.payload, now, &mut next)
            })
            .collect()
    }

    /// The answers of the provider to one request; the application of the
    /// provider accepts every `Set` and `Del`.
    fn answer(&mut self, request: &Publication) -> Vec<Publication> {
        let t = request.topic.to_string();
        validate(&t, &request.payload);
        match self.p.handle_message(&t, &request.payload, self.now) {
            Ok(Handled::Publish(ps)) => ps,
            Ok(Handled::SetData(r)) => self.p.complete_set_data(&r, Ok(()), self.now),
            Ok(Handled::SetConfig(r)) => self.p.complete_set_config(&r, Ok(()), self.now).0,
            Ok(Handled::SetReferenceDesignation(r)) => {
                self.p.complete_reference_designation(&r, Ok(()), self.now)
            }
            Ok(h) => panic!("{h:?}"),
            Err(_) => Vec::new(),
        }
    }

    /// Runs until no request is left to publish; returns the completions.
    fn run(&mut self) -> Vec<Completion> {
        loop {
            let changes = self.c.take_filter_changes();
            self.c.on_subscribed(&ok(&changes.subscribe));
            self.unsubscribed.extend(changes.unsubscribe);
            let out = self.c.take_outbound();
            if out.is_empty() {
                return self.c.take_completions();
            }
            for o in out {
                let answers = self.answer(&o);
                self.deliver(answers);
            }
        }
    }

    fn one(&mut self) -> Outcome {
        let mut done = self.run();
        assert_eq!(done.len(), 1, "{done:?}");
        done.remove(0).outcome
    }
}

fn answers(o: Outcome) -> Vec<Received> {
    match o {
        Outcome::Answers(a) => a,
        o => panic!("no answer: {o:?}"),
    }
}

fn names(a: &[Received]) -> Vec<String> {
    a.iter()
        .map(|r| r.data_set_writer_name.to_string())
        .collect()
}

fn lifecycle(app: &str) -> Lifecycle {
    Lifecycle::new(
        ServiceType::OTConnector,
        il(app),
        mam(app),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap()
}

fn provider_with(max_packet_size: usize, assets: &[String]) -> Provider {
    let config = ProviderConfig {
        get: true,
        max_packet_size,
        publish_limit: max_packet_size,
    };
    let mut p = Provider::new(lifecycle(PROVIDER), config, WriterIds::new())
        .unwrap()
        .with_reference_designations(StoredReferenceDesignations::new())
        .unwrap();
    for a in assets {
        p.add_asset(mam(a)).unwrap();
    }
    p
}

/// The provider of MG B1.1 with MOTOR and VALVE, the DataSet `oee` of
/// V-PAY-021 (writable) and the Config of V-PAY-099, connected.
fn provider() -> Provider {
    let mut p = provider_with(262_144, &[MOTOR.to_owned(), VALVE.to_owned()]);
    let src = il(PROVIDER);
    let mut d = DataSetSpec::new("oee");
    for f in ["Availability", "PerformanceRate", "QualityRate", "Product"] {
        d.fields.push(FieldSpec::new(f, T::Double));
    }
    d.publication.mode = Mode::ON_REQUEST;
    p.add_data(&src, d, MS).unwrap();
    p.accept_set_data(&src, "oee", true).unwrap();
    let values = ["Availability", "PerformanceRate", "QualityRate", "Product"]
        .map(|f| (f.to_owned(), Value::Double(1.0)));
    p.update_data(&src, "oee", values.to_vec(), MS).unwrap();
    let config =
        serde_json::from_value(vector("V-PAY-099").1["Messages"][0]["Payload"].clone()).unwrap();
    p.add_config(
        &src,
        ConfigSpec {
            context: CTX.to_owned(),
            config,
        },
        MS,
    )
    .unwrap();
    p.on_connected(Duration::ZERO, MS);
    p
}

#[test]
fn get_is_answered_and_correlated_over_the_reply_to() {
    let mut bus = Bus::new(provider());
    bus.get(Resource::Mam, &provider_at(), &GetParams::default());
    let a = answers(bus.one());
    assert_eq!(names(&a), [PROVIDER, MOTOR, VALVE]);
    assert!(a.iter().all(|r| matches!(r.payload, Payload::Mam(_))));
    assert!(a.iter().all(|r| r.reply_to.is_some()));
    // one Source
    bus.get(Resource::Health, &at(MOTOR, None), &GetParams::default());
    let a = answers(bus.one());
    assert_eq!(names(&a), [MOTOR]);
    assert!(matches!(a[0].payload, Payload::Health(_)));
    // Data and its Metadata with the DataSetWriterId of the Data
    bus.get(
        Resource::Data,
        &at(PROVIDER, Some(tag("oee"))),
        &GetParams::default(),
    );
    let a = answers(bus.one());
    let Payload::Data(d) = &a[0].payload else {
        panic!()
    };
    assert_eq!(a[0].json["Availability"], 1.0);
    assert_eq!(d.0.len(), 4);
    let meta = GetParams {
        data_set_writer_id: Some(DataSetWriterId::new(a[0].data_set_writer_id).unwrap()),
        ..GetParams::default()
    };
    bus.get(Resource::Metadata, &at(PROVIDER, Some(tag("oee"))), &meta);
    let a = answers(bus.one());
    let Payload::MetaData(m) = &a[0].payload else {
        panic!()
    };
    assert_eq!(m.fields.as_ref().map(Vec::len), Some(4));
    // the Config, the PublicationList and the Profile
    for r in [
        Resource::Config,
        Resource::PublicationList,
        Resource::Profile,
    ] {
        bus.get(r, &provider_at(), &GetParams::default());
        assert!(!answers(bus.one()).is_empty(), "{r}");
    }
    assert_eq!(bus.c.pending(), 0);
}

#[test]
fn unknown_source_ends_the_get_with_bad_not_found() {
    let mut bus = Bus::new(provider());
    bus.get(
        Resource::Health,
        &at("vendor.com/None/N-1/1", None),
        &GetParams::default(),
    );
    assert_eq!(bus.one(), Outcome::Status(StatusCode::BAD_NOT_FOUND));
    // the answer filters are released
    assert_eq!(bus.unsubscribed.len(), 2);
}

#[test]
fn set_and_del_end_with_their_status_event() {
    let mut bus = Bus::new(provider());
    let e = entry(93, "V-PAY-061");
    bus.write(
        Method::Set,
        Resource::Data,
        &at(PROVIDER, Some(tag("oee"))),
        Some(&e),
    );
    assert_eq!(bus.one(), Outcome::Status(StatusCode::GOOD));
    assert_eq!(
        bus.p.data_values(&il(PROVIDER), "oee").unwrap()[3],
        Value::Double(97.0)
    );
    let ctx = Some(Filter::ContextName(CTX.to_owned()));
    let e = entry(43, "V-PAY-100");
    bus.write(Method::Set, Resource::Config, &at(PROVIDER, ctx), Some(&e));
    assert!(matches!(bus.one(), Outcome::Status(_)));
    let rd = WriteEntry {
        data_set_writer_id: DataSetWriterId::new(94).unwrap(),
        payload: vector("V-PAY-045").1["Messages"][0]["Payload"].clone(),
    };
    bus.write(
        Method::Set,
        Resource::ReferenceDesignation,
        &at(MOTOR, None),
        Some(&rd),
    );
    assert_eq!(bus.one(), Outcome::Status(StatusCode::GOOD));
    bus.write(
        Method::Del,
        Resource::ReferenceDesignation,
        &at(MOTOR, None),
        None,
    );
    assert_eq!(bus.one(), Outcome::Status(StatusCode::GOOD));
    assert_eq!(bus.p.reference_designation(&il(MOTOR)), None);
    // a Set of an unknown DataSet: BadNotFound
    let e = entry(93, "V-PAY-061");
    bus.write(
        Method::Set,
        Resource::Data,
        &at(PROVIDER, Some(tag("none"))),
        Some(&e),
    );
    assert_eq!(bus.one(), Outcome::Status(StatusCode::BAD_NOT_FOUND));
}

/// The `Call NewDataSetWriterId` is that of MG B1.2.1 (V-PAY-008) with
/// CorrelationData (REQ-ACT-007) and passes the strict schema.
#[test]
fn new_data_set_writer_id_request_is_the_guideline_vector() {
    let (topic, expected) = vector("V-PAY-008");
    let mut c = consumer();
    let publisher = PublisherId::new(ServiceType::ITConnector, il(CONSUMER));
    let mut next = || MessageId::new(MS, None, &publisher);
    c.new_data_set_writer_id(
        Resource::ReferenceDesignation,
        &provider_at(),
        MS + TIMEOUT,
        MS,
        &mut next,
    )
    .unwrap();
    // nothing is published before the Reply topic is subscribed
    assert!(c.take_outbound().is_empty());
    let changes = c.take_filter_changes();
    assert_eq!(
        changes.subscribe,
        [expected["ResponseAddress"].as_str().unwrap()]
    );
    c.on_subscribed(&ok(&changes.subscribe));
    let out = c.take_outbound();
    assert_eq!(out.len(), 1);
    assert_eq!(out[0].topic.to_string(), topic);
    validate(&topic, &out[0].payload);
    let mut got = json_of(&out[0]);
    assert!(got["CorrelationData"].is_string(), "{got}");
    got.as_object_mut().unwrap().remove("CorrelationData");
    assert_eq!(got, expected);
    // a Filter is not used (REQ-ACT-020)
    assert!(
        c.new_data_set_writer_id(
            Resource::Data,
            &at(PROVIDER, Some(tag("oee"))),
            MS + TIMEOUT,
            MS,
            &mut next
        )
        .is_err()
    );
}

/// The real provider answers: the reserved id with its Ttl, BadNotFound
/// for an unknown Source (SF-113); the Reply filter is released.
#[test]
fn new_data_set_writer_id_ends_with_the_provided_id() {
    let mut bus = Bus::new(provider());
    bus.action(Resource::Data, &provider_at());
    let Outcome::WriterId(r) = bus.one() else {
        panic!()
    };
    assert_eq!(r.ttl, oec_core::provider::WRITER_ID_TTL_SECONDS);
    assert!(bus.p.writer_ids().is_reserved_at(r.data_set_writer_id, MS));
    bus.action(Resource::Health, &at(MOTOR, None));
    let Outcome::WriterId(m) = bus.one() else {
        panic!()
    };
    assert_ne!(m.data_set_writer_id, r.data_set_writer_id);
    bus.action(Resource::Data, &at("vendor.com/None/N-1/1", None));
    assert_eq!(bus.one(), Outcome::Status(StatusCode::BAD_NOT_FOUND));
    assert_eq!(bus.unsubscribed.len(), 3, "{:?}", bus.unsubscribed);
    assert_eq!(bus.c.pending(), 0);
}

/// Only the final response to the own ActionRequest ends the request:
/// an intermediate response (REQ-ACT-016), another CorrelationData or
/// RequestId, or another Responder are ignored; without a final response
/// the request times out (REQ-ACT-009). Concurrent requests get their
/// own RequestId (REQ-ACT-011).
#[test]
fn only_the_final_response_ends_an_action() {
    let mut c = consumer();
    let mut ids = MessageIds::new(PublisherId::new(ServiceType::ITConnector, il(CONSUMER)));
    let mut next = || ids.next_id(MS);
    let first = c
        .new_data_set_writer_id(Resource::Data, &provider_at(), MS + TIMEOUT, MS, &mut next)
        .unwrap();
    c.new_data_set_writer_id(Resource::Data, &provider_at(), MS + TIMEOUT, MS, &mut next)
        .unwrap();
    ack(&mut c);
    let out = c.take_outbound();
    assert_eq!(out.len(), 2);
    let requests: Vec<Json> = out.iter().map(json_of).collect();
    assert_ne!(
        requests[0]["Messages"][0]["RequestId"],
        requests[1]["Messages"][0]["RequestId"]
    );
    assert_ne!(
        requests[0]["CorrelationData"],
        requests[1]["CorrelationData"]
    );
    let reply = requests[0]["ResponseAddress"].as_str().unwrap().to_owned();
    let response = |request: &Json, state: u8| {
        json!({
            "MessageId": format!("{MS}-OTConnector/{PROVIDER}"),
            "MessageType": "ua-action-response",
            "PublisherId": format!("OTConnector/{PROVIDER}"),
            "Timestamp": "2021-12-16T22:25:10.094Z",
            "RequestorId": request["RequestorId"],
            "CorrelationData": request["CorrelationData"],
            "Messages": [{
                "DataSetWriterId": 8,
                "ActionTargetId": 1,
                "Status": {},
                "RequestId": request["Messages"][0]["RequestId"],
                "ActionState": state,
                "Payload": {"DataSetWriterId": 4711, "Ttl": 10}
            }]
        })
    };
    let mut send = |c: &mut Consumer, m: &Json| {
        let payload = serde_json::to_vec(m).unwrap();
        validate(&reply, &payload);
        let mut next = || ids.next_id(MS);
        let incoming = c.handle_message(&reply, &payload, MS, &mut next);
        assert_eq!(incoming.error, None);
        c.take_completions()
    };
    assert!(
        send(&mut c, &response(&requests[0], 1)).is_empty(),
        "intermediate"
    );
    let mut other = response(&requests[0], 2);
    other["CorrelationData"] = json!("AAAAAAAAAAA=");
    assert!(send(&mut c, &other).is_empty(), "other CorrelationData");
    let mut other = response(&requests[0], 2);
    other["Messages"][0]["RequestId"] = requests[1]["Messages"][0]["RequestId"].clone();
    assert!(send(&mut c, &other).is_empty(), "other RequestId");
    let mut other = response(&requests[0], 2);
    other["PublisherId"] = json!(format!("OTConnector/{MOTOR}"));
    assert!(send(&mut c, &other).is_empty(), "other Responder");
    let done = send(&mut c, &response(&requests[0], 2));
    assert_eq!(done.len(), 1);
    assert_eq!(done[0].id, first);
    let Outcome::WriterId(r) = &done[0].outcome else {
        panic!("{done:?}")
    };
    assert_eq!((r.data_set_writer_id.get(), r.ttl), (4711, 10));
    // the second gets no answer
    c.expire(MS + TIMEOUT);
    assert_eq!(c.take_completions()[0].outcome, Outcome::Timeout);
    assert_eq!(c.take_filter_changes().unsubscribe, [reply]);
}

fn assets(n: usize) -> Vec<String> {
    (0..n)
        .map(|i| format!("vendor.com/Pump/P-1/{i:04}"))
        .collect()
}

#[test]
fn paginated_answers_are_reassembled_in_page_order() {
    let all = assets(25);
    let mut bus = Bus::new(provider_with(262_144, &all));
    bus.p.on_connected(Duration::ZERO, MS);
    let per_page = GetParams {
        per_page: Some(10),
        ..GetParams::default()
    };
    bus.get(Resource::Mam, &provider_at(), &per_page);
    let a = answers(bus.one());
    let mut expected = vec![PROVIDER.to_owned()];
    expected.extend(all.iter().cloned());
    assert_eq!(names(&a), expected);
    // without PaginationRequest: paginated within the MaxPacketSize
    let mut bus = Bus::new(provider_with(4_096, &all));
    bus.p.on_connected(Duration::ZERO, MS);
    bus.get(Resource::Mam, &provider_at(), &GetParams::default());
    ack(&mut bus.c);
    let out = bus.c.take_outbound();
    let pages = bus.answer(&out[0]);
    assert!(pages.len() > 1, "paginated");
    bus.deliver(pages);
    assert_eq!(
        names(&answers(bus.c.take_completions().remove(0).outcome)),
        expected
    );
}

fn refdes(parent: &str) -> Json {
    json!({"Function": {"Value": "=P1.L3.M1", "Parent": {"Value": "=P1.L3", "IdentificationLink": parent}}})
}

#[test]
fn a_query_answer_is_checked_against_the_parent() {
    let parent = "consumer.com/LineController/LC-200/4711";
    let mut bus = Bus::new(provider());
    for (source, p) in [
        (PROVIDER, parent),
        (MOTOR, parent),
        (VALVE, "consumer.com/Other/O-1/1"),
    ] {
        let e = WriteEntry {
            data_set_writer_id: DataSetWriterId::new(94).unwrap(),
            payload: refdes(p),
        };
        bus.write(
            Method::Set,
            Resource::ReferenceDesignation,
            &at(source, None),
            Some(&e),
        );
        assert_eq!(bus.one(), Outcome::Status(StatusCode::GOOD));
    }
    let query = GetParams {
        query: Some(ReferenceDesignationQuery {
            parent: parent.to_owned(),
            aspect: Some(Aspect::Function),
        }),
        ..GetParams::default()
    };
    bus.get(Resource::ReferenceDesignation, &provider_at(), &query);
    assert_eq!(names(&answers(bus.one())), [PROVIDER, MOTOR]);
    // a provider without Query support answers all objects
    // (REQ-QRY-004): the consumer keeps the children only
    bus.get(Resource::ReferenceDesignation, &provider_at(), &query);
    ack(&mut bus.c);
    let mut get = bus.c.take_outbound().remove(0);
    let mut v = json_of(&get);
    v["Messages"] = json!([]);
    get.payload = serde_json::to_vec(&v).unwrap();
    let all = bus.answer(&get);
    assert_eq!(json_of(&all[0])["Messages"].as_array().unwrap().len(), 3);
    bus.deliver(all);
    let done = bus.c.take_completions();
    assert_eq!(names(&answers(done[0].outcome.clone())), [PROVIDER, MOTOR]);
}

// ---------------------------------------------- acknowledgement and time

#[test]
fn a_request_waits_for_its_subscriptions_and_times_out() {
    let mut c = consumer();
    let mut ids = MessageIds::new(PublisherId::new(ServiceType::ITConnector, il(CONSUMER)));
    let mut next = || ids.next_id(MS);
    let id = c
        .get(
            Resource::Mam,
            &provider_at(),
            &GetParams::default(),
            MS + TIMEOUT,
            MS,
            &mut next,
        )
        .unwrap();
    let changes = c.take_filter_changes();
    assert_eq!(changes.subscribe.len(), 2);
    assert!(
        changes
            .subscribe
            .iter()
            .all(|f| f.starts_with("Oi4v2/OTConnector/"))
    );
    assert!(c.take_outbound().is_empty(), "not before the SUBACK");
    c.on_subscribed(&ok(&changes.subscribe[..1]));
    assert!(c.take_outbound().is_empty(), "not before every SUBACK");
    c.on_subscribed(&ok(&changes.subscribe[1..]));
    assert_eq!(c.take_outbound().len(), 1);
    assert_eq!(c.next_deadline(), Some(MS + TIMEOUT));
    c.expire(MS + TIMEOUT - 1);
    assert!(c.take_completions().is_empty());
    c.expire(MS + TIMEOUT);
    let done = c.take_completions();
    assert_eq!(
        done,
        [Completion {
            id,
            outcome: Outcome::Timeout
        }]
    );
    assert_eq!(c.take_filter_changes().unsubscribe.len(), 2);
    assert_eq!(c.next_deadline(), None);
}

#[test]
fn a_refused_subscription_ends_the_request_and_close_ends_all() {
    let mut c = consumer();
    let mut ids = MessageIds::new(PublisherId::new(ServiceType::ITConnector, il(CONSUMER)));
    let mut next = || ids.next_id(MS);
    let params = GetParams::default();
    let first = c
        .get(
            Resource::Mam,
            &provider_at(),
            &params,
            MS + 1,
            MS,
            &mut next,
        )
        .unwrap();
    let changes = c.take_filter_changes();
    let refused: Vec<_> = changes
        .subscribe
        .iter()
        .map(|f| (f.clone(), SubscribeReturnCode::Failure))
        .collect();
    c.on_subscribed(&refused);
    let done = c.take_completions();
    assert_eq!(done[0].id, first);
    assert!(matches!(done[0].outcome, Outcome::Refused(_)));
    assert!(c.take_outbound().is_empty());
    // shared answer filters: the second Get of the same resource needs
    // no new subscription
    let second = c
        .get(
            Resource::Health,
            &provider_at(),
            &params,
            MS + 1,
            MS,
            &mut next,
        )
        .unwrap();
    let third = c
        .get(
            Resource::Health,
            &at(MOTOR, None),
            &params,
            MS + 1,
            MS,
            &mut next,
        )
        .unwrap();
    ack(&mut c);
    assert_eq!(c.take_outbound().len(), 2);
    c.cancel(third);
    c.close();
    assert_eq!(
        c.take_completions(),
        [Completion {
            id: second,
            outcome: Outcome::Closed
        }]
    );
}

// ----------------------------------------------------------- directory

fn presence(incoming: &[oec_core::consumer::Incoming]) -> Vec<PresenceChange> {
    incoming.iter().flat_map(|i| i.presence.clone()).collect()
}

fn is_online(c: &PresenceChange, with_mam: bool) -> bool {
    matches!(c, PresenceChange::Online { mam, .. } if mam.is_some() == with_mam)
}

#[test]
fn the_directory_follows_birth_close_and_will() {
    let mut p = provider_with(262_144, &[]);
    let birth = p.on_connected(Duration::ZERO, MS);
    let mut bus = Bus::new(provider_with(262_144, &[]));
    let sub = bus.c.subscribe_presence();
    let changes = bus.c.take_filter_changes();
    assert_eq!(
        changes.subscribe,
        ["Oi4v2/+/+/Pub/Health/+", "Oi4v2/+/+/Pub/MAM/+"]
    );
    let incoming = bus.deliver(birth);
    let online = presence(&incoming);
    assert_eq!(online.len(), 1);
    assert!(is_online(&online[0], true));
    assert!(incoming.iter().any(|i| i.presence_subscriptions == [sub]));
    assert_eq!(bus.c.apps()[0].state, AppState::Online);
    let close = p.close(MS + 1);
    let gone = presence(&bus.deliver(vec![close]));
    assert!(matches!(gone[..], [PresenceChange::OfflineGraceful { .. }]));
    // a heartbeat does not bring it back, only the next Birth (REQ-LIFE-013)
    p.on_disconnected();
    let birth = p.on_connected(Duration::ZERO, MS + 2);
    let health: Vec<_> = birth
        .iter()
        .filter(|b| b.topic.to_string().contains("/Health"))
        .cloned()
        .collect();
    assert!(presence(&bus.deliver(health)).is_empty());
    assert!(is_online(&presence(&bus.deliver(birth))[0], true));
    let will = p.will(MS + 3);
    let will = Publication {
        topic: oec_core::topic::Topic::parse(&will.topic).unwrap(),
        payload: will.payload,
    };
    let gone = presence(&bus.deliver(vec![will]));
    assert!(matches!(
        gone[..],
        [PresenceChange::OfflineUngraceful { .. }]
    ));
    assert_eq!(bus.c.apps()[0].state, AppState::OfflineUngraceful);
    bus.c.unsubscribe(sub);
    assert_eq!(bus.c.take_filter_changes().unsubscribe.len(), 2);
}

#[test]
fn an_application_first_seen_by_its_heartbeat_is_asked_for_its_mam() {
    let mut bus = Bus::new(provider());
    bus.c.subscribe_presence();
    ack(&mut bus.c);
    let heartbeat = bus.p.poll(Duration::from_secs(61), MS + 61_000);
    let heartbeat: Vec<_> = heartbeat
        .into_iter()
        .filter(|b| b.topic.to_string().contains("/Health"))
        .collect();
    assert!(!heartbeat.is_empty());
    let first = presence(&bus.deliver(heartbeat));
    assert!(is_online(&first[0], false));
    // the discovery Get MAM: answered, reported once more with the MAM
    ack(&mut bus.c);
    let out = bus.c.take_outbound();
    assert_eq!(out.len(), 1);
    assert!(
        out[0]
            .topic
            .to_string()
            .ends_with(&format!("/Get/MAM/{}", il(PROVIDER).encode()))
    );
    let answer = bus.answer(&out[0]);
    let again = presence(&bus.deliver(answer));
    assert!(is_online(&again[0], true));
    assert!(
        bus.c.take_completions().is_empty(),
        "no completion of its own"
    );
    assert!(bus.c.apps()[0].mam.is_some());
}

#[test]
fn subscriptions_deliver_the_matching_items() {
    let mut bus = Bus::new(provider());
    let data = TopicFilter::builder(Method::Pub, Resource::Data)
        .any_source()
        .build()
        .unwrap();
    let mams = TopicFilter::builder(Method::Pub, Resource::Mam)
        .build()
        .unwrap();
    let a = bus.c.subscribe(&[data]);
    let b = bus.c.subscribe(&[mams]);
    let changes = bus.c.take_filter_changes();
    assert_eq!(changes.subscribe.len(), 2);
    let birth = bus.p.on_connected(Duration::ZERO, MS);
    let got: Vec<_> = bus
        .deliver(birth)
        .into_iter()
        .flat_map(|i| i.deliveries)
        .collect();
    assert!(
        got.iter()
            .all(|(subs, r)| subs == &[b] && matches!(r.payload, Payload::Mam(_)))
    );
    assert_eq!(got.len(), 3);
    // the answer to another consumer's Get is seen by the subscription too
    bus.get(
        Resource::Data,
        &at(PROVIDER, Some(tag("oee"))),
        &GetParams::default(),
    );
    ack(&mut bus.c);
    let out = bus.c.take_outbound();
    let answer = bus.answer(&out[0]);
    let got: Vec<_> = bus
        .deliver(answer)
        .into_iter()
        .flat_map(|i| i.deliveries)
        .collect();
    assert_eq!(got.len(), 1);
    assert_eq!(got[0].0, [a]);
    bus.c.unsubscribe(a);
    bus.c.unsubscribe(b);
    let changes = bus.c.take_filter_changes();
    // with the answer filters of the Get
    assert_eq!(changes.unsubscribe.len(), 4, "{changes:?}");
}

/// A raw subscription (C-087, ADR-0026 §23) gets every publication on its
/// filters as received, with the decoded items, also one that cannot be
/// decoded; it is an entry of the SubscriptionList like any other.
#[test]
fn raw_subscriptions_get_the_publication_as_received() {
    let mut bus = Bus::new(provider());
    let mams = TopicFilter::builder(Method::Pub, Resource::Mam)
        .build()
        .unwrap();
    let typed = bus.c.subscribe(std::slice::from_ref(&mams));
    let raw = bus.c.subscribe_raw(std::slice::from_ref(&mams));
    let changes = bus.c.take_filter_changes();
    assert_eq!(changes.subscribe, [mams.as_str().to_owned()]);
    let list = bus.c.take_subscriptions().unwrap();
    assert_eq!(list.len(), 2);
    assert!(list.iter().all(|e| e.topic_path == mams.as_str()));
    let birth = bus.p.on_connected(Duration::ZERO, MS);
    let incoming: Vec<_> = bus
        .deliver(birth)
        .into_iter()
        .filter(|i| !i.raw.is_empty() || !i.deliveries.is_empty())
        .collect();
    assert_eq!(incoming.len(), 3);
    for i in &incoming {
        assert_eq!(i.raw, [raw]);
        assert_eq!(i.items.len(), 1);
        assert!(matches!(i.items[0].payload, Payload::Mam(_)));
        assert!(i.deliveries.iter().all(|(subs, _)| subs == &[typed]));
        assert_eq!(i.error, None);
    }
    // undecodable: only the raw subscription gets it, with the error
    let topic = Address {
        source: None,
        ..provider_at()
    }
    .topic(Method::Pub, Resource::Mam)
    .unwrap()
    .to_string();
    let mut next = || bus.ids.next_id(bus.now);
    let i = bus
        .c
        .handle_message(&topic, b"{not json", bus.now, &mut next);
    assert_eq!(i.raw, [raw]);
    assert!(i.items.is_empty() && i.deliveries.is_empty());
    assert!(i.error.is_some());
    // a topic that does not parse (an AppId that is no base64url)
    let i = bus
        .c
        .handle_message("Oi4v2/OTConnector/!!/Pub/MAM", b"{}", bus.now, &mut next);
    assert_eq!(i.raw, [raw]);
    assert!(i.error.is_some());
    bus.c.unsubscribe(raw);
    let i = bus
        .c
        .handle_message(&topic, b"{not json", bus.now, &mut next);
    assert!(i.raw.is_empty());
    assert_eq!(bus.c.take_subscriptions().unwrap().len(), 1);
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(24))]

    /// Pages arriving in any order are reassembled in page order, each
    /// asset once (REQ-PAG-002, REQ-PAG-004).
    #[test]
    fn pages_in_any_order_are_reassembled(n in 0usize..40, per_page in 1u32..12, seed in any::<u64>()) {
        let all = assets(n);
        let mut bus = Bus::new(provider_with(262_144, &all));
        bus.p.on_connected(Duration::ZERO, MS);
        let params = GetParams { per_page: Some(per_page), ..GetParams::default() };
        bus.get(Resource::Mam, &provider_at(), &params);
        ack(&mut bus.c);
        let out = bus.c.take_outbound();
        let mut pages = bus.answer(&out[0]);
        prop_assert_eq!(pages.len(), (n + 1).div_ceil(per_page as usize));
        // a deterministic shuffle
        let mut s = seed;
        for i in (1..pages.len()).rev() {
            s = s.wrapping_mul(6_364_136_223_846_793_005).wrapping_add(1);
            pages.swap(i, usize::try_from(s >> 33).unwrap() % (i + 1));
        }
        bus.deliver(pages);
        let mut expected = vec![PROVIDER.to_owned()];
        expected.extend(all);
        let done = bus.c.take_completions();
        prop_assert_eq!(done.len(), 1);
        prop_assert_eq!(names(&answers(done[0].outcome.clone())), expected);
    }
}

#[test]
fn an_answer_filter_used_again_before_its_unsubscribe_releases_the_request() {
    let mut bus = Bus::new(provider());
    bus.get(Resource::Health, &at(MOTOR, None), &GetParams::default());
    ack(&mut bus.c);
    for o in bus.c.take_outbound() {
        let answers = bus.answer(&o);
        bus.deliver(answers);
    }
    assert_eq!(bus.c.take_completions().len(), 1);
    // the next request reuses the released filters before the session
    // has unsubscribed them: they are still subscribed and acknowledged
    bus.get(Resource::Health, &at(MOTOR, None), &GetParams::default());
    let a = answers(bus.one());
    assert_eq!(names(&a), [MOTOR]);
    // unsubscribed once, after the second answer
    assert_eq!(bus.unsubscribed.len(), 2, "{:?}", bus.unsubscribed);
}
