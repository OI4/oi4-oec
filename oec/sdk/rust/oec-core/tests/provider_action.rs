//! The responder of NewDataSetWriterId (P4 increment 8c, ADR-0026 §19): a
//! `Call` is answered at once with one final ActionResponse per
//! ActionRequest on the ResponseAddress (REQ-ACT-012, REQ-ACT-015,
//! REQ-ACT-019); the provided DataSetWriterId is reserved for its `Ttl` in
//! the persisted table and handed out once (REQ-ACT-023); requests the
//! application does not answer are ignored (REQ-ACT-017). The guideline
//! vectors V-PAY-008/009 and the constructed V-PAY-185/186/197/199/203 are
//! replayed against the provider.

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{
    Handled, Ignored, Provider, ProviderConfig, WRITER_ID_TTL_SECONDS, WriterIds, WriterKey,
};
use oec_core::resource::Mam;
use oec_core::status::StatusCode;
use oec_core::topic::{Resource, ServiceType};
use serde_json::{Value as Json, json};

/// The Responder of MG B1.2.1.
const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const MS: u64 = 1_700_000_000_000;

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

fn provider_with(get: bool, max_packet_size: usize, writers: WriterIds) -> Provider {
    let lifecycle = Lifecycle::new(
        ServiceType::OTConnector,
        il(APP),
        mam(APP),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get,
        max_packet_size,
        publish_limit: 262_144,
    };
    Provider::new(lifecycle, config, writers).unwrap()
}

fn provider() -> Provider {
    provider_with(true, 262_144, WriterIds::new())
}

fn vector(id: &str) -> (String, Json) {
    for file in ["payload-guideline.json", "payload-constructed.json"] {
        let path = format!(
            "{}/../../../spec/export/vectors/{file}",
            env!("CARGO_MANIFEST_DIR")
        );
        let all: Json = serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
        if let Some(v) = all["vectors"]
            .as_array()
            .unwrap()
            .iter()
            .find(|v| v["id"] == id)
        {
            return (
                v["topic"].as_str().unwrap().to_owned(),
                serde_json::from_str(v["payload"].as_str().unwrap()).unwrap(),
            );
        }
    }
    panic!("no vector {id}")
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

fn call(
    p: &mut Provider,
    topic: &str,
    request: &Json,
    now: u64,
) -> Result<Vec<Publication>, Ignored> {
    match p.handle_message(topic, &serde_json::to_vec(request).unwrap(), now)? {
        Handled::Publish(out) => Ok(out),
        other => panic!("{other:?}"),
    }
}

/// The single ActionResponse of the single answer.
fn answer(p: &mut Provider, topic: &str, request: &Json, now: u64) -> Json {
    let out = call(p, topic, request, now).unwrap();
    assert_eq!(out.len(), 1);
    assert_eq!(out[0].topic.to_string(), request["ResponseAddress"]);
    let m = json_of(&out[0]);
    assert_eq!(m["Messages"].as_array().unwrap().len(), 1, "{m}");
    m["Messages"][0].clone()
}

fn provided_id(response: &Json) -> u16 {
    assert_eq!(response["Status"], json!({}), "{response}");
    assert_eq!(response["Payload"]["Ttl"], WRITER_ID_TTL_SECONDS);
    u16::try_from(response["Payload"]["DataSetWriterId"].as_u64().unwrap()).unwrap()
}

fn status_code(c: StatusCode) -> Json {
    json!({"Code": c.0})
}

/// With a Source: the Call topic of MG B1.2.1 for the asset PUMP.
fn for_source(source: &str) -> (String, Json) {
    let (topic, mut request) = vector("V-PAY-008");
    let s = il(source).encode();
    request["ResponseAddress"] = json!(format!(
        "{}/{s}",
        request["ResponseAddress"].as_str().unwrap()
    ));
    (format!("{topic}/{s}"), request)
}

#[test]
fn guideline_call_is_answered_like_the_guideline() {
    let mut p = provider();
    let (topic, request) = vector("V-PAY-008");
    let (reply, expected) = vector("V-PAY-009");
    let out = call(&mut p, &topic, &request, MS).unwrap();
    assert_eq!(out.len(), 1);
    assert_eq!(out[0].topic.to_string(), reply);
    let mut got = json_of(&out[0]);
    // the own MessageId and Timestamp, a new id with the Ttl of the SDK
    assert!(
        got["MessageId"]
            .as_str()
            .unwrap()
            .ends_with("-OTConnector/provider.com/FieldDataService/FDS-001/1200-0345")
    );
    let id = provided_id(&got["Messages"][0]);
    assert!(id > 11, "neither 8 nor an id in use: {id}");
    for k in ["MessageId", "Timestamp"] {
        got[k] = expected[k].clone();
    }
    got["Messages"][0]["Payload"] = expected["Messages"][0]["Payload"].clone();
    assert_eq!(got, expected);
}

/// The CorrelationData of the request is copied (REQ-ACT-007), and the
/// RequestId is that of the ActionRequest (REQ-ACT-012).
#[test]
fn correlation_data_and_request_id_are_copied() {
    let mut p = provider();
    let (topic, mut request) = vector("V-PAY-008");
    request["CorrelationData"] = json!("AQIDBA==");
    request["Messages"][0]["RequestId"] = json!(4711);
    let out = call(&mut p, &topic, &request, MS).unwrap();
    let got = json_of(&out[0]);
    assert_eq!(got["CorrelationData"], "AQIDBA==");
    assert_eq!(got["Messages"][0]["RequestId"], 4711);
}

/// Every provided id is unique, never one in use, and reserved: the next
/// id assigned by the SDK is another one (REQ-ACT-023, SF-112).
#[test]
fn provided_ids_are_unique_and_reserved() {
    let mut p = provider();
    let (topic, request) = vector("V-PAY-008");
    let ids: Vec<u16> = (0..3)
        .map(|i| provided_id(&answer(&mut p, &topic, &request, MS + i)))
        .collect();
    assert_eq!(ids.len(), 3);
    assert!(
        ids[0] != ids[1] && ids[1] != ids[2] && ids[0] != ids[2],
        "{ids:?}"
    );
    for id in &ids {
        assert!(
            p.writer_ids()
                .is_reserved_at(DataSetWriterId::new(*id).unwrap(), MS + 59_999)
        );
        assert!(
            !p.writer_ids()
                .is_reserved_at(DataSetWriterId::new(*id).unwrap(), MS + 60_003)
        );
    }
    let pump = p.add_asset(mam(PUMP)).unwrap();
    let mam_id = p
        .writer_ids()
        .get(&WriterKey::new(Resource::Mam, pump.as_str()))
        .unwrap();
    assert!(!ids.contains(&mam_id.get()), "{mam_id:?} {ids:?}");
}

/// The reservations are persisted: after a restart within the Ttl no id
/// is provided twice.
#[test]
fn reservations_survive_restarts() {
    let mut p = provider();
    let _ = p.take_writer_ids();
    let (topic, request) = vector("V-PAY-008");
    let first = provided_id(&answer(&mut p, &topic, &request, MS));
    let saved = p.take_writer_ids().expect("the reservation is saved");
    let mut q = provider_with(true, 262_144, WriterIds::from_json(&saved).unwrap());
    let second = provided_id(&answer(&mut q, &topic, &request, MS + 1));
    assert_ne!(first, second);
}

/// A Source the application represents is answered; an unknown Source
/// gets BadNotFound with an empty Payload (SF-113).
#[test]
fn the_source_must_be_known() {
    let mut p = provider();
    p.add_asset(mam(PUMP)).unwrap();
    let (topic, request) = for_source(PUMP);
    provided_id(&answer(&mut p, &topic, &request, MS));
    let (topic, request) = for_source("vendor.com/Pump/P-1/0816");
    let r = answer(&mut p, &topic, &request, MS);
    assert_eq!(r["Status"], status_code(StatusCode::BAD_NOT_FOUND));
    assert_eq!(r["Payload"], json!({}));
    assert_eq!(r["ActionState"], 2);
}

/// An invalid request Payload is rejected like V-PAY-203 (REQ-ACT-014).
#[test]
fn invalid_resources_are_rejected_like_the_vector() {
    let mut p = provider();
    let (_, expected) = vector("V-PAY-203");
    for id in ["V-PAY-199", "V-PAY-200"] {
        let (topic, request) = vector(id);
        assert_eq!(
            answer(&mut p, &topic, &request, MS),
            expected["Messages"][0],
            "{id}"
        );
    }
}

/// No id is left to reserve: BadResourceUnavailable until the
/// reservations expire.
#[test]
fn reservations_are_capped() {
    let mut p = provider();
    let (topic, request) = vector("V-PAY-008");
    let mut n = 0;
    let last = loop {
        let r = answer(&mut p, &topic, &request, MS);
        if r["Status"] != json!({}) {
            break r;
        }
        n += 1;
        assert!(n <= 1000, "never capped");
    };
    assert!(n >= 16, "{n}");
    assert_eq!(
        last["Status"],
        status_code(StatusCode::BAD_RESOURCE_UNAVAILABLE)
    );
    assert_eq!(last["Payload"], json!({}));
    let later = MS + u64::from(WRITER_ID_TTL_SECONDS) * 1000 + 1;
    provided_id(&answer(&mut p, &topic, &request, later));
}

/// Several ActionRequests in one Call: one response each, split into
/// messages by the MaxPacketSize; ActionRequests of other Actions get no
/// response.
#[test]
fn several_requests_are_answered_and_split() {
    let (topic, mut request) = vector("V-PAY-008");
    let one = request["Messages"][0].clone();
    let mut other = one.clone();
    other["DataSetWriterId"] = json!(7);
    let mut second = one.clone();
    second["RequestId"] = json!(2);
    request["Messages"] = json!([one, other, second]);
    let mut p = provider();
    let out = call(&mut p, &topic, &request, MS).unwrap();
    assert_eq!(out.len(), 1);
    let ids: Vec<Json> = json_of(&out[0])["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| m["RequestId"].clone())
        .collect();
    assert_eq!(ids, [json!(1), json!(2)]);
    // room for one response only
    let limit = out[0].payload.len() - 1;

    let mut p = provider_with(true, limit, WriterIds::new());
    let out = call(&mut p, &topic, &request, MS).unwrap();
    assert_eq!(out.len(), 2);
    let (a, b) = (json_of(&out[0]), json_of(&out[1]));
    assert_ne!(a["MessageId"], b["MessageId"]);
    assert_eq!(a["Messages"][0]["RequestId"], 1);
    assert_eq!(b["Messages"][0]["RequestId"], 2);
    assert!(out.iter().all(|o| o.payload.len() <= limit));
}

/// Requests the application does not answer get no response
/// (REQ-ACT-017): no ActionRequest of NewDataSetWriterId (V-PAY-197), a
/// ResponseAddress that is no Reply topic (V-PAY-185) or not the Reply
/// topic of the Call topic (V-PAY-186), a PublisherId of another
/// application; and nothing at Level 1.
#[test]
fn unanswerable_requests_are_ignored() {
    let mut p = provider();
    let before = p.writer_ids().to_json();
    for id in ["V-PAY-185", "V-PAY-186", "V-PAY-197"] {
        let (topic, request) = vector(id);
        let r = call(&mut p, &topic, &request, MS);
        assert!(matches!(r, Err(Ignored::Payload(_))), "{id}: {r:?}");
    }
    let (topic, mut request) = vector("V-PAY-008");
    request["PublisherId"] = json!("OTConnector/provider.com/FieldDataService/FDS-001/1200-0346");
    assert!(matches!(
        call(&mut p, &topic, &request, MS),
        Err(Ignored::Payload(_))
    ));
    assert_eq!(p.writer_ids().to_json(), before, "nothing reserved");

    let mut p = provider_with(false, 262_144, WriterIds::new());
    let (topic, request) = vector("V-PAY-008");
    assert!(matches!(
        call(&mut p, &topic, &request, MS),
        Err(Ignored::NotServed(_))
    ));
    assert!(p.subscriptions().is_empty());
}

#[cfg(feature = "strict-validation")]
#[test]
fn every_answer_passes_the_schema() {
    use oec_core::schema::{Flavor, Validator};
    let validator = Validator::new().unwrap();
    let mut p = provider();
    p.add_asset(mam(PUMP)).unwrap();
    let mut all = Vec::new();
    for (topic, mut request) in [
        vector("V-PAY-008"),
        vector("V-PAY-199"),
        for_source(PUMP),
        for_source("vendor.com/Pump/P-1/0816"),
    ] {
        request["CorrelationData"] = json!("AQIDBA==");
        all.extend(call(&mut p, &topic, &request, MS).unwrap());
    }
    assert_eq!(all.len(), 4);
    for o in all {
        validator
            .validate_topic(&o.topic, Flavor::Strict, &json_of(&o))
            .unwrap_or_else(|e| panic!("{}: {e}\n{}", o.topic, json_of(&o)));
    }
}
