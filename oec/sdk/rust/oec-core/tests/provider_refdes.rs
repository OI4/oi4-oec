//! The resource ReferenceDesignation (P4 increment 7c, ADR-0026 §16):
//! `Get` with and without Source and with the Query of the direct children
//! (REQ-RES-REFDES-004, REQ-RES-REFDES-011 … 014), `Set` and `Del` decided
//! by the application and answered with a Status event (REQ-RES-REFDES-006,
//! REQ-RES-REFDES-007, REQ-RES-REFDES-009), a successful `Set` published
//! with the ReplyTo (REQ-RES-REFDES-010), persisted
//! (REQ-RES-REFDES-002). The guideline vectors V-PAY-041 … 046 and
//! V-PAY-053/054 are replayed against the provider.

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{
    Handled, Ignored, Provider, ProviderConfig, ReferenceDesignationRequest,
    StoredReferenceDesignations, WriterIds,
};
use oec_core::resource::{Mam, ReferenceDesignation};
use oec_core::status::StatusCode;
use oec_core::topic::ServiceType;
use serde_json::{Value as Json, json};

const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const MOTOR: &str = "groe.mca/DeviceModelB/AC33.447/20443";
const VALVE: &str = "groe.mca/Valve/V-1/1";
const MS: u64 = 1_639_693_510_094;

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

fn provider_with(get: bool, table: Option<StoredReferenceDesignations>) -> Provider {
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
        max_packet_size: 262_144,
        publish_limit: 262_144,
    };
    let p = Provider::new(lifecycle, config, WriterIds::new()).unwrap();
    match table {
        Some(t) => p.with_reference_designations(t).unwrap(),
        None => p,
    }
}

/// The application offering ReferenceDesignation, connected, with the
/// assets MOTOR and VALVE.
fn provider() -> Provider {
    let mut p = provider_with(true, Some(StoredReferenceDesignations::new()));
    p.add_asset(mam(MOTOR)).unwrap();
    p.add_asset(mam(VALVE)).unwrap();
    p.on_connected(std::time::Duration::ZERO, MS);
    p
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

fn topics(ps: &[Publication]) -> Vec<String> {
    ps.iter().map(|p| p.topic.to_string()).collect()
}

fn topic(method: &str, rest: &str) -> String {
    format!("Oi4v2/OTConnector/{}/{method}/{rest}", il(APP).encode())
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

fn bytes(v: &Json) -> Vec<u8> {
    serde_json::to_vec(v).unwrap()
}

/// The request of a vector, sent to the provider.
fn handle(p: &mut Provider, id: &str) -> Handled {
    let (t, payload) = vector(id);
    p.handle_message(&t, &bytes(&payload), MS).unwrap()
}

fn publish(h: Handled) -> Vec<Publication> {
    h.into_publications().expect("answered by publications")
}

/// A `Set` (with `payload`) or `Del` of the ReferenceDesignation of
/// `source`.
fn write(p: &mut Provider, method: &str, source: &str, payload: Option<&Json>) -> Handled {
    let messages = payload.map_or_else(
        || json!([]),
        |v| json!([{"DataSetWriterId": 94, "DataSetWriterName": source, "Payload": v}]),
    );
    let request = json!({
        "MessageId": format!("{MS}-ITConnector/consumer.com/Client/C-1/1"),
        "MessageType": "ua-data",
        "PublisherId": "ITConnector/consumer.com/Client/C-1/1",
        "ReplyTo": "",
        "Messages": messages
    });
    let t = topic(
        method,
        &format!("ReferenceDesignation/{}", il(source).encode()),
    );
    p.handle_message(&t, &bytes(&request), MS).unwrap()
}

fn request(h: Handled) -> ReferenceDesignationRequest {
    match h {
        Handled::SetReferenceDesignation(r) => r,
        h => panic!("not decided by the application: {h:?}"),
    }
}

/// The level, Number and Source of the Status event that ends `out`.
fn status(out: &[Publication]) -> (String, u64, String) {
    let last = json_of(out.last().unwrap());
    let m = &last["Messages"][0];
    let level = m["WriterGroupName"]
        .as_str()
        .unwrap()
        .trim_start_matches("Status/")
        .to_owned();
    (
        level,
        m["Payload"]["Number"].as_u64().unwrap(),
        m["DataSetWriterName"].as_str().unwrap().to_owned(),
    )
}

fn good(source: &str) -> (String, u64, String) {
    ("Good".to_owned(), 0, source.to_owned())
}

fn bad(code: StatusCode, source: &str) -> (String, u64, String) {
    ("Bad".to_owned(), u64::from(code.0), source.to_owned())
}

/// The ReferenceDesignation of MOTOR in V-PAY-054.
fn motor_rd() -> Json {
    vector("V-PAY-054").1["Messages"][1]["Payload"].clone()
}

/// Sets the ReferenceDesignation of `source`, accepted.
fn set(p: &mut Provider, source: &str, rd: &Json) -> Vec<Publication> {
    let r = request(write(p, "Set", source, Some(rd)));
    p.complete_reference_designation(&r, Ok(()), MS)
}

#[test]
fn set_of_v_pay_045_publishes_v_pay_043() {
    let mut p = provider();
    let r = request(handle(&mut p, "V-PAY-045"));
    let (_, expected) = vector("V-PAY-043");
    let rd: ReferenceDesignation =
        serde_json::from_value(expected["Messages"][0]["Payload"].clone()).unwrap();
    assert_eq!(r.source(), &il(APP));
    assert_eq!(r.reference_designation(), Some(&rd));
    assert_eq!(
        r.message_id().to_string(),
        vector("V-PAY-045").1["MessageId"]
    );
    let out = p.complete_reference_designation(&r, Ok(()), MS);
    let (t, _) = vector("V-PAY-043");
    assert_eq!(topics(&out)[0], t);
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], expected["ReplyTo"]);
    assert_eq!(v["DataSetClassId"], expected["DataSetClassId"]);
    let (got, want) = (&v["Messages"][0], &expected["Messages"][0]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(got[k], want[k], "{k}");
    }
    assert_eq!(status(&out), good(APP));
    assert_eq!(out.len(), 2);
    assert_eq!(p.reference_designation(&il(APP)), Some(&rd));
    // persisted (REQ-RES-REFDES-002), once
    let saved = p.take_reference_designations().unwrap();
    assert_eq!(p.take_reference_designations(), None);
    let back = StoredReferenceDesignations::from_json(&saved).unwrap();
    assert_eq!(back.get(APP), Some(&rd));
    // V-PAY-041: the Get answered with it, the ReplyTo of the Get
    let out = publish(handle(&mut p, "V-PAY-041"));
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], vector("V-PAY-041").1["MessageId"]);
    assert_eq!(v["Messages"][0]["Payload"], want["Payload"]);
    // V-PAY-046: deleted, nothing published on the resource
    let r = request(handle(&mut p, "V-PAY-046"));
    assert_eq!(r.reference_designation(), None);
    let out = p.complete_reference_designation(&r, Ok(()), MS);
    assert_eq!(out.len(), 1);
    assert_eq!(status(&out), good(APP));
    assert_eq!(p.reference_designation(&il(APP)), None);
    assert!(p.take_reference_designations().is_some());
    // a Get now: an empty list (SF-099)
    let out = publish(handle(&mut p, "V-PAY-041"));
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
}

#[test]
fn query_of_v_pay_053_answers_the_children_of_v_pay_054() {
    let mut p = provider();
    let (_, app_rd) = vector("V-PAY-043");
    set(&mut p, APP, &app_rd["Messages"][0]["Payload"]);
    set(&mut p, MOTOR, &motor_rd());
    // VALVE: a child of another parent
    let other = json!({"Function": {"Value": "=P1.L3.V1", "Parent": {
        "Value": "=P1.L3", "IdentificationLink": "consumer.com/LineController/LC-200/4712"}}});
    set(&mut p, VALVE, &other);
    let out = publish(handle(&mut p, "V-PAY-053"));
    let (t, expected) = vector("V-PAY-054");
    assert_eq!(topics(&out), [t]);
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], expected["ReplyTo"]);
    let names = |m: &Json| {
        m["Messages"]
            .as_array()
            .unwrap()
            .iter()
            .map(|m| (m["DataSetWriterName"].clone(), m["Payload"].clone()))
            .collect::<Vec<_>>()
    };
    assert_eq!(names(&v), names(&expected));
    // without Query: every source with an object
    let (_, mut get) = vector("V-PAY-053");
    get["Messages"] = json!([]);
    let t = topic("Get", "ReferenceDesignation");
    let out = publish(p.handle_message(&t, &bytes(&get), MS).unwrap());
    assert_eq!(json_of(&out[0])["Messages"].as_array().unwrap().len(), 3);
    // the aspect Location: no child of LC-200/4711
    get["Messages"] = vector("V-PAY-053").1["Messages"].clone();
    get["Messages"][0]["Payload"]["Aspect"] = json!("Location");
    let out = publish(p.handle_message(&t, &bytes(&get), MS).unwrap());
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
    // a Query with a Source is ignored (REQ-RES-REFDES-013)
    let t = topic(
        "Get",
        &format!("ReferenceDesignation/{}", il(VALVE).encode()),
    );
    let out = publish(p.handle_message(&t, &bytes(&get), MS).unwrap());
    assert_eq!(names(&json_of(&out[0]))[0].1, other);
    // an invalid Query: ignored like an invalid PaginationRequest
    get["Messages"][0]["Payload"] = json!({"Aspect": "Function"});
    let t = topic("Get", "ReferenceDesignation");
    let r = p.handle_message(&t, &bytes(&get), MS);
    assert!(matches!(r, Err(Ignored::Payload(_))), "{r:?}");
}

#[test]
fn the_application_decides() {
    let mut p = provider();
    let rd = motor_rd();
    // refused: only the Status event with the code
    let r = request(write(&mut p, "Set", MOTOR, Some(&rd)));
    let out = p.complete_reference_designation(&r, Err(StatusCode::BAD_USER_ACCESS_DENIED), MS);
    assert_eq!(status(&out), bad(StatusCode::BAD_USER_ACCESS_DENIED, MOTOR));
    assert_eq!(out.len(), 1);
    // an Err with a Good code is Bad
    let out = p.complete_reference_designation(&r, Err(StatusCode::GOOD), MS);
    assert_eq!(status(&out), bad(StatusCode::BAD, MOTOR));
    assert_eq!(p.reference_designation(&il(MOTOR)), None);
    assert_eq!(p.take_reference_designations(), None);
    // a Set of the same object again: Good, published, not saved again
    let out = set(&mut p, MOTOR, &rd);
    assert_eq!(status(&out), good(MOTOR));
    assert!(p.take_reference_designations().is_some());
    let out = set(&mut p, MOTOR, &rd);
    assert_eq!(out.len(), 2);
    assert_eq!(p.take_reference_designations(), None);
    // the asset removed before the answer
    let r = request(write(&mut p, "Del", MOTOR, None));
    p.remove_asset(&il(MOTOR)).unwrap();
    let out = p.complete_reference_designation(&r, Ok(()), MS);
    assert_eq!(status(&out), bad(StatusCode::BAD_NOT_FOUND, APP));
    // the stored object stays for the asset when it comes back
    p.add_asset(mam(MOTOR)).unwrap();
    assert!(p.reference_designation(&il(MOTOR)).is_some());
}

#[test]
fn invalid_writes_are_answered_with_status_bad() {
    let mut p = provider();
    let not_found = StatusCode::BAD_NOT_FOUND;
    // Del without an object: nothing to delete
    let out = publish(write(&mut p, "Del", MOTOR, None));
    assert_eq!(status(&out), bad(not_found, MOTOR));
    // a Set without a message, with a non-object, with two
    let decoding = StatusCode::BAD_DECODING_ERROR;
    let out = publish(write(&mut p, "Set", MOTOR, Some(&json!(1))));
    assert_eq!(status(&out), bad(decoding, MOTOR));
    let out = publish(write(&mut p, "Set", MOTOR, Some(&json!({"Function": 1}))));
    assert_eq!(status(&out), bad(decoding, MOTOR));
    // unknown elements are ignored (REQ-MSG-046)
    let r = request(write(&mut p, "Set", MOTOR, Some(&json!({"X": 1}))));
    assert_eq!(
        r.reference_designation(),
        Some(&ReferenceDesignation::default())
    );
    // a Source that is no asset: under the AppId (SF-108)
    let out = publish(write(
        &mut p,
        "Set",
        "groe.mca/Other/O-1/1",
        Some(&motor_rd()),
    ));
    assert_eq!(status(&out), bad(not_found, APP));
    // not offered: BadNotFound as before (A-92)
    let mut q = provider_with(true, None);
    q.on_connected(std::time::Duration::ZERO, MS);
    let out = publish(write(&mut q, "Set", APP, Some(&motor_rd())));
    assert_eq!(status(&out), bad(not_found, APP));
    assert_eq!(q.take_reference_designations(), None);
    let r = q.handle_message(
        &topic("Get", "ReferenceDesignation"),
        &bytes(&vector("V-PAY-041").1),
        MS,
    );
    assert!(matches!(r, Err(Ignored::NotServed(_))), "{r:?}");
}

#[test]
fn profile_and_publication_list_name_the_resource() {
    let mut p = provider_with(true, Some(StoredReferenceDesignations::new()));
    p.add_asset(mam(MOTOR)).unwrap();
    assert!(p.offers_reference_designation());
    assert!(
        p.subscriptions()
            .iter()
            .any(|f| f.to_string() == topic("Get", "ReferenceDesignation/#"))
    );
    let out = p.on_connected(std::time::Duration::ZERO, MS);
    // RD is not published on connect, only on Get and after Set
    assert!(
        !topics(&out)
            .iter()
            .any(|t| t.contains("/Pub/ReferenceDesignation"))
    );
    let profile = out
        .iter()
        .find(|o| o.topic.to_string() == topic("Pub", &format!("Profile/{}", il(MOTOR).encode())))
        .unwrap();
    assert_eq!(
        json_of(profile)["Messages"][0]["Payload"]["Resources"],
        json!([
            "MAM",
            "Health",
            "Config",
            "Data",
            "Metadata",
            "ReferenceDesignation",
            "Event",
            "Profile",
            "PublicationList"
        ])
    );
    let list = json_of(out.last().unwrap());
    let entries: Vec<_> = list["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|m| m["WriterGroupName"] == "ReferenceDesignation")
        .collect();
    assert_eq!(entries.len(), 2);
    for e in entries {
        assert_eq!(e["Payload"]["Mode"], "ON_REQUEST_SOURCE_10");
        assert_eq!(e["Payload"]["Resource"], "ReferenceDesignation");
        assert_eq!(e["Payload"].get("Config"), None);
    }
    // Level 1: offered, but no request served and no entry
    let q = provider_with(false, Some(StoredReferenceDesignations::new()));
    assert!(
        !q.subscriptions()
            .iter()
            .any(|f| f.to_string().contains("ReferenceDesignation"))
    );
}
