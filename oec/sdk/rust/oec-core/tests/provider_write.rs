//! The write side of the provider (P4 increment 7a, ADR-0026 §14): every
//! `Set` and `Del` is answered with a Status event (REQ-RES-EVENT-005,
//! REQ-CMP-043); `Set PublicationList` within the `Config` of the entry
//! (REQ-RES-PUBLIST-016/022/024/025), persisted across restarts; `Set Data`
//! decided by the application (REQ-RES-DATA-009/011/012); a `Get` for an
//! unknown Source gets BadNotFound (REQ-CMP-047). The guideline vectors
//! V-PAY-061/062 are replayed against the provider.

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::data::{BuiltinType as T, DataSetSpec, FieldSpec, Mode, Value, Writable};
use oec_core::provider::{
    FOREIGN_SOURCES, Handled, Ignored, Provider, ProviderConfig, PublicationOverrides,
    SetDataRequest, WriterIds,
};
use oec_core::resource::Mam;
use oec_core::status::StatusCode;
use oec_core::topic::ServiceType;
use serde_json::{Value as Json, json};

const APP: &str = "vendor.com/Gateway/GW-1/SN-1";
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const MS: u64 = 1_700_000_000_000;
const SET_ID: &str = "1700000000000-Aggregation/vendor.com/Client/C-1/1";

fn il(s: &str) -> IdentificationLink {
    IdentificationLink::parse(s).unwrap()
}

fn mam(piu: &str) -> Mam {
    serde_json::from_value(json!({
        "Manufacturer": {"Locale": "en-US", "Text": "Vendor"},
        "ManufacturerUri": "vendor.com",
        "Model": {"Locale": "en-US", "Text": "Gateway"},
        "ProductCode": "GW-1",
        "HardwareRevision": "",
        "SoftwareRevision": "1.0.0",
        "DeviceRevision": "",
        "DeviceManual": "",
        "DeviceClass": "Oi4.Aggregation",
        "SerialNumber": "SN-1",
        "ProductInstanceUri": piu,
        "RevisionCounter": 0
    }))
    .unwrap()
}

fn provider_of(st: ServiceType, app: &str, get: bool, writers: WriterIds) -> Provider {
    let lifecycle = Lifecycle::new(
        st,
        il(app),
        mam(app),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get,
        max_packet_size: 262_144,
        publish_limit: 262_144,
    };
    Provider::new(lifecycle, config, writers).unwrap()
}

/// The application with the asset PUMP, connected.
fn provider() -> (Provider, IdentificationLink) {
    let mut p = provider_of(ServiceType::Aggregation, APP, true, WriterIds::new());
    let pump = p.add_asset(mam(PUMP)).unwrap();
    (p, pump)
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

fn topics(ps: &[Publication]) -> Vec<String> {
    ps.iter().map(|p| p.topic.to_string()).collect()
}

fn topic(method: &str, rest: &str) -> String {
    format!("Oi4v2/Aggregation/{}/{method}/{rest}", il(APP).encode())
}

fn request(messages: &Json) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": SET_ID,
        "MessageType": "ua-data",
        "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
        "ReplyTo": "",
        "Messages": messages
    }))
    .unwrap()
}

fn message(payload: &Json) -> Json {
    json!([{
        "DataSetWriterId": 20,
        "DataSetWriterName": PUMP,
        "Payload": payload
    }])
}

fn temp(tag: &str, mode: Mode, writable: Writable) -> DataSetSpec {
    let mut d = DataSetSpec::new(tag);
    d.fields = vec![
        FieldSpec::new("T", T::Double),
        FieldSpec::new("N", T::Int64),
    ];
    d.publication.mode = mode;
    d.publication.writable = writable;
    d
}

fn publish(h: Handled) -> Vec<Publication> {
    h.into_publications().expect("answered by publications")
}

fn write(p: &mut Provider, method: &str, rest: &str, messages: &Json) -> Vec<Publication> {
    publish(
        p.handle_message(&topic(method, rest), &request(messages), MS + 1)
            .unwrap(),
    )
}

fn set_data(p: &mut Provider, tag: &str, payload: &Json) -> Result<SetDataRequest, Json> {
    let rest = format!("Data/{}/{tag}", il(PUMP).encode());
    match p
        .handle_message(&topic("Set", &rest), &request(&message(payload)), MS + 1)
        .unwrap()
    {
        Handled::SetData(r) => Ok(r),
        h => Err(status(&publish(h))),
    }
}

/// The Status event that ends `out`: its topic level and Payload; checks
/// the ReplyTo.
fn status(out: &[Publication]) -> Json {
    let last = out.last().expect("a Status event");
    let v = json_of(last);
    assert_eq!(v["ReplyTo"], SET_ID);
    assert_eq!(v["Messages"].as_array().unwrap().len(), 1);
    let m = &v["Messages"][0];
    let level = last
        .topic
        .to_string()
        .rsplit('/')
        .next()
        .unwrap()
        .to_owned();
    assert_eq!(m["WriterGroupName"], format!("Status/{level}"));
    json!({"Level": level, "Number": m["Payload"]["Number"], "Source": m["DataSetWriterName"]})
}

fn code(c: StatusCode) -> Json {
    json!(c.0)
}

// ---------------------------------------------------------------- vectors

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

/// The provider of MG B1.1.4 with the DataSet `oee` of V-PAY-021, which
/// accepts `Set Data`.
fn oee_provider() -> (Provider, IdentificationLink) {
    let (_, meta_pub) = vector("V-PAY-021");
    let meta = &meta_pub["MetaData"];
    let app = meta_pub["DataSetWriterName"].as_str().unwrap().to_owned();
    let mut p = provider_of(ServiceType::OTConnector, &app, true, WriterIds::new());
    let src = il(&app);
    let mut d = DataSetSpec::new(meta["Name"].as_str().unwrap());
    d.class_id = Some(meta["DataSetClassId"].as_str().unwrap().parse().unwrap());
    for f in meta["Fields"].as_array().unwrap() {
        d.fields
            .push(FieldSpec::new(f["Name"].as_str().unwrap(), T::Double));
    }
    d.publication.mode = Mode::ON_REQUEST;
    p.add_data(&src, d, MS).unwrap();
    p.accept_set_data(&src, "oee", true).unwrap();
    (p, src)
}

/// MG B1.1.4: the Set Data V-PAY-061 is answered with a Pub Data with its
/// ReplyTo (REQ-RES-DATA-012) and the Status event V-PAY-062.
#[test]
fn guideline_set_vector_is_answered_like_the_guideline() {
    let (set_topic, set) = vector("V-PAY-061");
    let (status_topic, expected) = vector("V-PAY-062");
    let (mut p, src) = oee_provider();
    p.on_connected(Duration::ZERO, MS);
    let payload = serde_json::to_vec(&set).unwrap();
    let Handled::SetData(req) = p.handle_message(&set_topic, &payload, MS + 1).unwrap() else {
        panic!("a Set Data the application decides");
    };
    assert_eq!((req.source(), req.tag()), (&src, "oee"));
    assert_eq!(req.message_id().as_str(), set["MessageId"]);
    let names: Vec<_> = req.values().iter().map(|(n, _)| n.as_str()).collect();
    assert_eq!(
        names,
        ["Availability", "PerformanceRate", "QualityRate", "Product"]
    );
    assert_eq!(req.values()[3].1, Value::Double(97.0));

    let out = p.complete_set_data(&req, Ok(()), MS + 2);
    let data_topic = set_topic.replace("/Set/", "/Pub/");
    assert_eq!(topics(&out), [data_topic, status_topic]);
    let data = json_of(&out[0]);
    assert_eq!(data["ReplyTo"], set["MessageId"]);
    let (got, requested) = (
        &data["Messages"][0]["Payload"],
        &set["Messages"][0]["Payload"],
    );
    for (k, v) in requested.as_object().unwrap() {
        assert_eq!(got[k].as_f64(), v.as_f64(), "{k}");
    }
    let event = json_of(&out[1]);
    for k in ["MessageType", "PublisherId", "DataSetClassId"] {
        assert_eq!(event[k], expected[k], "{k}");
    }
    assert_eq!(event["ReplyTo"], set["MessageId"]);
    let (got, want) = (&event["Messages"][0], &expected["Messages"][0]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(got[k], want[k], "{k}");
    }
    // the values are the state now
    assert_eq!(
        p.data_values(&src, "oee").unwrap(),
        [99.0, 99.0, 99.0, 97.0].map(Value::Double)
    );
}

// --------------------------------------------------------------- Set Data

#[test]
fn set_data_is_checked_against_the_definition() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::ON_REQUEST, Writable::Nothing), MS)
        .unwrap();
    p.add_data(&pump, temp("off", Mode::OFF, Writable::Nothing), MS)
        .unwrap();
    let ok = json!({"T": 1.5, "N": "7"});
    let bad = |c: StatusCode| json!({"Level": "Bad", "Number": code(c), "Source": PUMP});
    // the application accepts no Set Data by default (REQ-CMP-043)
    assert_eq!(
        set_data(&mut p, "t", &ok),
        Err(bad(StatusCode::BAD_NOT_WRITABLE))
    );
    p.accept_set_data(&pump, "t", true).unwrap();
    p.accept_set_data(&pump, "off", true).unwrap();
    for tag in ["off", "unknown"] {
        assert_eq!(
            set_data(&mut p, tag, &ok),
            Err(bad(StatusCode::BAD_NOT_FOUND))
        );
    }
    let cases = [
        (json!({"T": 1.5}), StatusCode::BAD_INVALID_ARGUMENT),
        (json!({"T": "x", "N": 1}), StatusCode::BAD_TYPE_MISMATCH),
        (json!({"T": 1.5, "N": 1.5}), StatusCode::BAD_TYPE_MISMATCH),
        (json!([1, 2]), StatusCode::BAD_DECODING_ERROR),
    ];
    for (payload, c) in cases {
        assert_eq!(set_data(&mut p, "t", &payload), Err(bad(c)), "{payload}");
    }
    // an unknown field is ignored (tolerant reader, REQ-MSG-046)
    let req = set_data(&mut p, "t", &json!({"T": 1.5, "N": 7, "X": true})).unwrap();
    assert_eq!(
        req.values(),
        [
            ("T".to_owned(), Value::Double(1.5)),
            ("N".to_owned(), Value::Int64(7))
        ]
    );
}

#[test]
fn set_data_needs_exactly_one_data_set_message() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::ON_REQUEST, Writable::Nothing), MS)
        .unwrap();
    p.accept_set_data(&pump, "t", true).unwrap();
    let rest = format!("Data/{}/t", pump.encode());
    let one = message(&json!({"T": 1, "N": 1}))[0].clone();
    let pagination = json!({"DataSetWriterId": 1, "Payload": {}});
    let cases = [
        (json!([one, one]), StatusCode::BAD_INVALID_ARGUMENT),
        (json!([]), StatusCode::BAD_INVALID_ARGUMENT),
        (json!("x"), StatusCode::BAD_DECODING_ERROR),
        (json!([42]), StatusCode::BAD_DECODING_ERROR),
    ];
    for (messages, c) in cases {
        let out = write(&mut p, "Set", &rest, &messages);
        assert_eq!(status(&out)["Number"], code(c), "{messages}");
    }
    // helper DataSets (DataSetWriterId 1 … 9) aside
    let h = p
        .handle_message(
            &topic("Set", &rest),
            &request(&json!([pagination, one])),
            MS,
        )
        .unwrap();
    assert!(matches!(h, Handled::SetData(_)), "{h:?}");
    // no NetworkMessage with a MessageId: no answer
    let r = p.handle_message(&topic("Set", &rest), b"{}", MS);
    assert!(matches!(r, Err(Ignored::Payload(_))), "{r:?}");
}

#[test]
fn set_data_completion_answers_as_decided() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::FILTER, Writable::Nothing), MS)
        .unwrap();
    p.accept_set_data(&pump, "t", true).unwrap();
    p.on_connected(Duration::ZERO, MS);
    let ok = json!({"T": 1.5, "N": 7});
    let status_of = |out: &[Publication]| status(out)["Number"].clone();
    // rejected: only the Status event, the state is unchanged
    let req = set_data(&mut p, "t", &ok).unwrap();
    let out = p.complete_set_data(&req, Err(StatusCode::BAD_OUT_OF_RANGE), MS + 2);
    assert_eq!(out.len(), 1);
    assert_eq!(status_of(&out), code(StatusCode::BAD_OUT_OF_RANGE));
    assert_eq!(p.data_values(&pump, "t"), None);
    // a Good code as error is Bad
    let req = set_data(&mut p, "t", &ok).unwrap();
    let out = p.complete_set_data(&req, Err(StatusCode::GOOD), MS + 2);
    assert_eq!(status_of(&out), code(StatusCode::BAD));
    // accepted: the DataSet on its FILTER topic with the ReplyTo, which is
    // its publication on change, too
    let req = set_data(&mut p, "t", &ok).unwrap();
    let out = p.complete_set_data(&req, Ok(()), MS + 2);
    let data = format!("Data/{}/t", pump.encode());
    assert_eq!(topics(&out)[0], topic("Pub", &data));
    assert_eq!(json_of(&out[0])["ReplyTo"], SET_ID);
    assert_eq!(status_of(&out), code(StatusCode::GOOD));
    assert_eq!(p.poll(Duration::from_millis(3), MS + 3), []);
    // removed meanwhile: BadNotFound
    let req = set_data(&mut p, "t", &ok).unwrap();
    p.remove_data(&pump, "t").unwrap();
    let out = p.complete_set_data(&req, Ok(()), MS + 4);
    assert_eq!(status_of(&out), code(StatusCode::BAD_NOT_FOUND));
}

// ----------------------------------------------------- Set PublicationList

fn entry(p: &mut Provider, rest: &str) -> Json {
    let out = publish(
        p.handle_message(&topic("Get", rest), &request(&json!([])), MS)
            .unwrap(),
    );
    json_of(&out[0])["Messages"][0]["Payload"].clone()
}

fn set_entry(p: &mut Provider, rest: &str, payload: &Json) -> Vec<Publication> {
    write(p, "Set", rest, &message(payload))
}

#[test]
fn set_publication_list_changes_what_config_allows() {
    let (mut p, pump) = provider();
    let id = p
        .add_data(
            &pump,
            temp("t", Mode::ON_REQUEST, Writable::ModeAndInterval),
            MS,
        )
        .unwrap();
    p.add_data(&pump, temp("m", Mode::ON_REQUEST, Writable::Mode), MS)
        .unwrap();
    p.update_data(
        &pump,
        "t",
        vec![("T".into(), 1.0.into()), ("N".into(), 1_i64.into())],
        MS,
    )
    .unwrap();
    p.on_connected(Duration::ZERO, MS);
    assert_eq!(p.take_publication_overrides(), None);
    let rest = format!("PublicationList/{}/Data/t", pump.encode());
    let mut e = entry(&mut p, &rest);
    e["Mode"] = json!("ON_REQUEST_FILTER_9");
    e["Interval"] = json!(500);
    e["Precisions"] = json!({"T": 0.5});
    let out = set_entry(&mut p, &rest, &e);
    // the changed entry with the ReplyTo (REQ-RES-PUBLIST-025), then Good
    assert_eq!(topics(&out)[0], topic("Pub", &rest));
    let answer = json_of(&out[0]);
    assert_eq!(answer["ReplyTo"], SET_ID);
    assert_eq!(answer["Messages"][0]["Payload"], e);
    assert_eq!(status(&out)["Level"], "Good");
    assert_eq!(entry(&mut p, &rest), e);
    assert_eq!(e["DataSetWriterId"], id.get());
    assert!(p.take_publication_overrides().is_some());
    // published at once under the new settings
    let out = p.poll(Duration::from_millis(1), MS + 2);
    assert_eq!(
        topics(&out),
        [topic("Pub", &format!("Data/{}/t", pump.encode()))]
    );
    assert_eq!(p.poll_timeout(), Duration::from_millis(501));
    // an unchanged entry: Good, nothing to persist
    let out = set_entry(&mut p, &rest, &e);
    assert_eq!(status(&out)["Level"], "Good");
    assert_eq!(p.take_publication_overrides(), None);
    // MODE_1: the Interval is read only
    let rest = format!("PublicationList/{}/Data/m", pump.encode());
    let mut e = entry(&mut p, &rest);
    e["Interval"] = json!(100);
    let out = set_entry(&mut p, &rest, &e);
    assert_eq!(status(&out)["Number"], code(StatusCode::BAD_NOT_WRITABLE));
}

#[test]
fn set_publication_list_rejects_what_config_forbids() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::ON_REQUEST, Writable::Interval), MS)
        .unwrap();
    let s = pump.encode();
    let data = format!("PublicationList/{s}/Data/t");
    let e = entry(&mut p, &data);
    let changed = |k: &str, v: Json| {
        let mut c = e.clone();
        c[k] = v;
        c
    };
    let cases = [
        (
            changed("Mode", json!("ON_REQUEST_FILTER_9")),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            changed("DataSetWriterId", json!(999)),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            changed("Config", json!("MODE_AND_INTERVAL_3")),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (changed("Filter", json!("u")), StatusCode::BAD_NOT_WRITABLE),
        (
            changed("Precisions", json!({"X": 1})),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        (
            changed("Precisions", json!({"T": -1})),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        (
            changed("Mode", json!("NO_MODE")),
            StatusCode::BAD_DECODING_ERROR,
        ),
    ];
    for (payload, c) in cases {
        let out = set_entry(&mut p, &data, &payload);
        assert_eq!(status(&out)["Number"], code(c), "{payload}");
        assert_eq!(out.len(), 1);
    }
    // an entry the source does not have
    let out = set_entry(&mut p, &format!("PublicationList/{s}/Data/u"), &e);
    assert_eq!(status(&out)["Number"], code(StatusCode::BAD_NOT_FOUND));
    // MAM has Config NONE_0: unchanged is a success, a change is not
    let mam = format!("PublicationList/{s}/MAM");
    let m = entry(&mut p, &mam);
    assert_eq!(status(&set_entry(&mut p, &mam, &m))["Level"], "Good");
    let mut changed = m.clone();
    changed["Mode"] = json!("OFF_0");
    let out = set_entry(&mut p, &mam, &changed);
    assert_eq!(status(&out)["Number"], code(StatusCode::BAD_NOT_WRITABLE));
}

#[test]
fn set_publication_list_off_removes_the_metadata_entry() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::ON_REQUEST, Writable::Mode), MS)
        .unwrap();
    p.on_connected(Duration::ZERO, MS);
    let rest = format!("PublicationList/{}/Data/t", pump.encode());
    let mut e = entry(&mut p, &rest);
    e["Mode"] = json!("OFF_0");
    assert_eq!(status(&set_entry(&mut p, &rest, &e))["Level"], "Good");
    // the PublicationList again: without the Metadata entry
    let out = p.poll(Duration::from_millis(1), MS + 2);
    assert_eq!(topics(&out), [topic("Pub", "PublicationList")]);
    let meta = format!("PublicationList/{}/Metadata/t", pump.encode());
    let out = publish(
        p.handle_message(&topic("Get", &meta), &request(&json!([])), MS)
            .unwrap(),
    );
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
}

#[test]
fn set_publication_list_survives_restarts() {
    let start = |overrides: PublicationOverrides, writable: Writable| {
        let (p, pump) = provider();
        let mut p = p.with_publication_overrides(overrides);
        p.add_data(&pump, temp("t", Mode::ON_REQUEST, writable), MS)
            .unwrap();
        (p, pump)
    };
    let (mut p, pump) = start(PublicationOverrides::new(), Writable::ModeAndInterval);
    let rest = format!("PublicationList/{}/Data/t", pump.encode());
    let mut e = entry(&mut p, &rest);
    e["Mode"] = json!("ON_REQUEST_SOURCE_FILTER_12");
    e["Interval"] = json!(250);
    set_entry(&mut p, &rest, &e);
    let saved = PublicationOverrides::from_json(&p.take_publication_overrides().unwrap()).unwrap();
    // restarted: the settings apply again
    let (mut p, _) = start(saved.clone(), Writable::ModeAndInterval);
    assert_eq!(entry(&mut p, &rest), e);
    // the definition now allows only Mode changes: only the Mode applies
    let (mut p, _) = start(saved, Writable::Mode);
    let got = entry(&mut p, &rest);
    assert_eq!(got["Mode"], e["Mode"]);
    assert_eq!(got.get("Interval"), None);
}

// ---------------------------------------------------- other Set and Del

#[test]
fn other_writes_are_answered_with_status_bad() {
    let (mut p, pump) = provider();
    p.add_data(&pump, temp("t", Mode::ON_REQUEST, Writable::Nothing), MS)
        .unwrap();
    let s = pump.encode();
    let any = message(&json!({}));
    let cases = [
        ("Set", format!("Config/{s}/a"), StatusCode::BAD_NOT_FOUND),
        (
            "Set",
            format!("SubscriptionList/{s}/Data/t"),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "Set",
            format!("ReferenceDesignation/{s}"),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "Del",
            format!("SubscriptionList/{s}/Data/t"),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "Del",
            format!("ReferenceDesignation/{s}"),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "Set",
            format!("Metadata/{s}/t"),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        ("Set", format!("Metadata/{s}/u"), StatusCode::BAD_NOT_FOUND),
    ];
    for (method, rest, c) in cases {
        let out = write(&mut p, method, &rest, &any);
        assert_eq!(out.len(), 1);
        assert_eq!(
            status(&out),
            json!({"Level": "Bad", "Number": code(c), "Source": PUMP}),
            "{method} {rest}"
        );
        // the Status events of an asset use its persisted ids
        let id = json_of(&out[0])["Messages"][0]["DataSetWriterId"]
            .as_u64()
            .unwrap();
        assert!(id < 100, "{id}");
    }
    // a Source that is no asset (REQ-CMP-043)
    let other = il("vendor.com/Pump/P-1/0816");
    let out = write(&mut p, "Set", &format!("Data/{}/t", other.encode()), &any);
    assert_eq!(status(&out)["Source"], other.as_str());
    assert_eq!(
        topics(&out),
        [topic(
            "Pub",
            &format!("Event/{}/Status/Bad", other.encode())
        )]
    );
    // Level 1: no Set
    let mut l1 = provider_of(ServiceType::Aggregation, APP, false, WriterIds::new());
    let r = l1.handle_message(&topic("Set", &format!("Data/{s}/t")), &request(&any), MS);
    assert!(matches!(r, Err(Ignored::NotServed(_))), "{r:?}");
}

#[test]
fn status_events_for_foreign_sources_are_bounded() {
    let (mut p, _) = provider();
    let ids: Vec<_> = (0..=FOREIGN_SOURCES)
        .map(|n| {
            let src = il(&format!("vendor.com/Other/O-1/{n}"));
            let rest = format!("MAM/{}", src.encode());
            p.handle_message(&topic("Get", &rest), &request(&json!([])), MS)
                .map(|h| json_of(&publish(h)[0])["Messages"][0]["DataSetWriterId"].clone())
        })
        .collect();
    assert_eq!(ids[0], Ok(json!(65535)));
    assert_eq!(
        ids[FOREIGN_SOURCES - 1],
        Ok(json!(65535 - FOREIGN_SOURCES + 1))
    );
    assert!(matches!(
        ids[FOREIGN_SOURCES],
        Err(Ignored::UnknownSource(_))
    ));
    // transient: not persisted
    let json = String::from_utf8(p.writer_ids().to_json()).unwrap();
    assert!(!json.contains("Other"), "{json}");
}

#[cfg(feature = "strict-validation")]
#[test]
fn every_answer_passes_the_schema() {
    use oec_core::schema::{Flavor, Validator};
    let validator = Validator::new().unwrap();
    let (mut p, pump) = provider();
    p.add_data(
        &pump,
        temp("t", Mode::ON_REQUEST, Writable::ModeAndInterval),
        MS,
    )
    .unwrap();
    p.accept_set_data(&pump, "t", true).unwrap();
    let s = pump.encode();
    let rest = format!("PublicationList/{s}/Data/t");
    let mut e = entry(&mut p, &rest);
    e["Interval"] = json!(100);
    let mut all = set_entry(&mut p, &rest, &e);
    all.extend(write(
        &mut p,
        "Set",
        &format!("Config/{s}/a"),
        &message(&json!({})),
    ));
    let req = set_data(&mut p, "t", &json!({"T": 1, "N": 2})).unwrap();
    all.extend(p.complete_set_data(&req, Ok(()), MS + 2));
    assert_eq!(all.len(), 5);
    for o in all {
        validator
            .validate_topic(&o.topic, Flavor::Strict, &json_of(&o))
            .unwrap_or_else(|e| panic!("{}: {e}\n{}", o.topic, json_of(&o)));
    }
}
