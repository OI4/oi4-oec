//! The Config DataSets of the provider (P4 increment 7b, ADR-0026 §15):
//! published masked (REQ-RES-CONFIG-024) on their Filter topic, answered on
//! `Get`, changed by `Set Config` within the definition
//! (REQ-RES-CONFIG-003/023/025) as the application decides
//! (REQ-RES-CONFIG-021/022), persisted across restarts. The guideline
//! vectors of MG B1.1.3 (V-PAY-098 … 102, V-PAY-114) are replayed.

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{
    ConfigSpec, Handled, Ignored, Provider, ProviderConfig, ProviderError, SetConfigRequest,
    StoredConfigValues, WriterIds,
};
use oec_core::resource::{Config, Mam};
use oec_core::status::StatusCode;
use oec_core::topic::ServiceType;
use serde_json::{Value as Json, json};

const MS: u64 = 1_700_000_000_000;
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const CTX: &str = "Overall device settings";
const SECRET: &str = "ssh-it's a secret!";

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
        "DeviceClass": "Oi4.OTConnector",
        "SerialNumber": "SN-1",
        "ProductInstanceUri": piu,
        "RevisionCounter": 0
    }))
    .unwrap()
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

/// The AppId of MG B1.1.3.
fn app() -> String {
    let (_, p) = vector("V-PAY-099");
    p["Messages"][0]["DataSetWriterName"]
        .as_str()
        .unwrap()
        .to_owned()
}

/// The Config of V-PAY-099 (all values empty) as definition.
fn device_config() -> Config {
    let (_, p) = vector("V-PAY-099");
    serde_json::from_value(p["Messages"][0]["Payload"].clone()).unwrap()
}

fn provider_with(get: bool, max_packet_size: usize, values: StoredConfigValues) -> Provider {
    let app = app();
    let lifecycle = Lifecycle::new(
        ServiceType::OTConnector,
        il(&app),
        mam(&app),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get,
        max_packet_size,
        publish_limit: max_packet_size,
    };
    Provider::new(lifecycle, config, WriterIds::new())
        .unwrap()
        .with_config_values(values)
}

/// The application of MG B1.1.3 with the Config of V-PAY-099, connected.
fn provider() -> (Provider, IdentificationLink) {
    let mut p = provider_with(true, 262_144, StoredConfigValues::new());
    let src = il(&app());
    p.add_config(&src, spec(CTX, device_config()), MS).unwrap();
    p.on_connected(Duration::ZERO, MS);
    (p, src)
}

fn spec(context: &str, config: Config) -> ConfigSpec {
    ConfigSpec {
        context: context.to_owned(),
        config,
    }
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

fn topics(ps: &[Publication]) -> Vec<String> {
    ps.iter().map(|p| p.topic.to_string()).collect()
}

fn base() -> String {
    format!("Oi4v2/OTConnector/{}", il(&app()).encode())
}

fn config_topic(method: &str, source: &IdentificationLink, context: &str) -> String {
    format!("{}/{method}/Config/{}/{context}", base(), source.encode())
}

fn request(id: &str, messages: &Json) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": id,
        "MessageType": "ua-data",
        "PublisherId": "ITConnector/https://id.consumer.com/MEService/Inst07322",
        "ReplyTo": "",
        "Messages": messages
    }))
    .unwrap()
}

const SET_ID: &str = "1700000000000-ITConnector/https://id.consumer.com/MEService/Inst07322";

fn publish(h: Handled) -> Vec<Publication> {
    h.into_publications().expect("answered by publications")
}

fn get(p: &mut Provider, rest: &str) -> Vec<Publication> {
    let topic = format!("{}/Get/{rest}", base());
    publish(
        p.handle_message(&topic, &request(SET_ID, &json!([])), MS + 1)
            .unwrap(),
    )
}

/// A `Set Config` with one DataSetMessage: the request the application
/// decides, or the Status event (level and Number).
fn set(
    p: &mut Provider,
    source: &IdentificationLink,
    context: &str,
    payload: &Json,
) -> Result<SetConfigRequest, Json> {
    let messages = json!([{
        "DataSetWriterId": 43,
        "DataSetWriterName": source.as_str(),
        "Payload": payload
    }]);
    set_messages(p, source, context, &messages)
}

fn set_messages(
    p: &mut Provider,
    source: &IdentificationLink,
    context: &str,
    messages: &Json,
) -> Result<SetConfigRequest, Json> {
    let topic = config_topic("Set", source, context);
    match p
        .handle_message(&topic, &request(SET_ID, messages), MS + 1)
        .unwrap()
    {
        Handled::SetConfig(r) => Ok(r),
        h => Err(status(&publish(h))),
    }
}

/// The Status event that ends `out`: level and Number; checks the
/// ReplyTo.
fn status(out: &[Publication]) -> Json {
    let last = out.last().expect("a Status event");
    let v = json_of(last);
    assert_eq!(v["ReplyTo"], SET_ID);
    let level = last
        .topic
        .to_string()
        .rsplit('/')
        .next()
        .unwrap()
        .to_owned();
    json!({"Level": level, "Number": v["Messages"][0]["Payload"]["Number"]})
}

fn bad(c: StatusCode) -> Json {
    json!({"Level": "Bad", "Number": c.0})
}

fn value_of(p: &Provider, src: &IdentificationLink, g: &str, n: &str) -> String {
    p.config(src, CTX).unwrap().groups[g].objects[n]
        .value
        .clone()
}

/// Nothing published contains the secret in clear.
fn assert_masked(out: &[Publication]) {
    for o in out {
        let s = String::from_utf8_lossy(&o.payload);
        assert!(!s.contains("secret"), "{}: {s}", o.topic);
    }
}

// ------------------------------------------------------ MG B1.1.3 vectors

/// V-PAY-098 is answered with the Config of V-PAY-099; the Set V-PAY-100
/// is decided by the application and answered with the Pub of V-PAY-114,
/// the password masked, then the Status event Good (V-PAY-076 shape).
#[test]
fn guideline_get_and_set_are_answered_like_the_guideline() {
    let (mut p, src) = provider();
    let (get_topic, get_msg) = vector("V-PAY-098");
    let out = publish(
        p.handle_message(&get_topic, &serde_json::to_vec(&get_msg).unwrap(), MS + 1)
            .unwrap(),
    );
    let (pub_topic, expected) = vector("V-PAY-099");
    assert_eq!(topics(&out), [pub_topic.as_str()]);
    let got = json_of(&out[0]);
    assert_eq!(got["ReplyTo"], get_msg["MessageId"]);
    assert_eq!(got["DataSetClassId"], expected["DataSetClassId"]);
    let (g, e) = (&got["Messages"][0], &expected["Messages"][0]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(g[k], e[k], "{k}");
    }

    let (set_topic, set_msg) = vector("V-PAY-100");
    let Handled::SetConfig(req) = p
        .handle_message(&set_topic, &serde_json::to_vec(&set_msg).unwrap(), MS + 2)
        .unwrap()
    else {
        panic!("a Set Config the application decides");
    };
    assert_eq!((req.source(), req.context()), (&src, CTX));
    assert_eq!(req.changes().len(), 9);
    assert_eq!(
        req.proposed().groups["device1"].objects["password"].value,
        SECRET
    );
    let debug = format!("{req:?}");
    assert!(!debug.contains("secret"), "{debug}");

    let (out, changed) = p.complete_set_config(&req, Ok(()), MS + 3);
    assert!(changed);
    let (_, after) = vector("V-PAY-114");
    let (_, good) = vector("V-PAY-076");
    let status_topic = format!("{}/Pub/Event/{}/Status/Good", base(), src.encode());
    assert_eq!(topics(&out), [pub_topic, status_topic]);
    assert_masked(&out);
    let data = json_of(&out[0]);
    assert_eq!(data["ReplyTo"], set_msg["MessageId"]);
    assert_eq!(
        data["Messages"][0]["Payload"],
        after["Messages"][0]["Payload"]
    );
    let event = json_of(&out[1]);
    assert_eq!(event["ReplyTo"], set_msg["MessageId"]);
    assert_eq!(
        event["Messages"][0]["Payload"],
        good["Messages"][0]["Payload"]
    );
    // the state in clear text for the application
    assert_eq!(value_of(&p, &src, "device1", "password"), SECRET);
}

/// V-PAY-101 (the complete Config with every read-only property as
/// published) changes nothing after V-PAY-100; V-PAY-102 unsets every
/// value (MG 9.1.3).
#[test]
fn guideline_full_and_empty_sets_are_accepted() {
    let (mut p, src) = provider();
    for (id, changed, port) in [
        ("V-PAY-100", true, "443"),
        ("V-PAY-101", false, "443"),
        ("V-PAY-102", true, ""),
    ] {
        let (topic, msg) = vector(id);
        let Handled::SetConfig(req) = p
            .handle_message(&topic, &serde_json::to_vec(&msg).unwrap(), MS + 2)
            .unwrap()
        else {
            panic!("{id}: a Set Config the application decides");
        };
        let (out, c) = p.complete_set_config(&req, Ok(()), MS + 3);
        assert_eq!(c, changed, "{id}");
        assert_eq!(out.len(), 2, "{id}");
        assert_eq!(value_of(&p, &src, "device1", "port"), port, "{id}");
    }
}

// --------------------------------------------------------- publication

/// The Config is published masked at once while connected, on every
/// connect and after a change from the application side; listed in the
/// PublicationList (ON_REQUEST_FILTER_9) and the Profile.
#[test]
fn config_is_published_listed_and_profiled() {
    let mut p = provider_with(true, 262_144, StoredConfigValues::new());
    let src = il(&app());
    let mut c = device_config();
    c.groups
        .get_mut("device1")
        .unwrap()
        .objects
        .get_mut("password")
        .unwrap()
        .value = SECRET.to_owned();
    let writer = p.add_config(&src, spec(CTX, c), MS).unwrap();
    let out = p.on_connected(Duration::ZERO, MS);
    let config_pub = config_topic("Pub", &src, CTX);
    assert!(topics(&out).contains(&config_pub), "{:?}", topics(&out));
    assert_masked(&out);
    let o = out
        .iter()
        .find(|o| o.topic.to_string() == config_pub)
        .unwrap();
    let m = &json_of(o)["Messages"][0];
    assert_eq!(m["DataSetWriterId"], writer.get());
    assert_eq!(m["WriterGroupName"], CTX);
    assert_eq!(m["Payload"]["device1"]["password"]["Value"], "********");
    assert!(p.poll(Duration::from_millis(1), MS + 1).is_empty());

    // a change from the application side is published at once
    let changed = p
        .set_config(
            &src,
            CTX,
            &[("device1".into(), "port".into(), "8080".into())],
            MS + 2,
        )
        .unwrap();
    assert!(changed);
    let out = p.poll(Duration::from_millis(2), MS + 2);
    assert_eq!(topics(&out), [config_pub.as_str()]);
    assert_eq!(json_of(&out[0])["ReplyTo"], "");

    // PublicationList
    let out = get(&mut p, &format!("PublicationList/{}", src.encode()));
    let entries: Vec<_> = json_of(&out[0])["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| m["Payload"].clone())
        .filter(|e| e["Resource"] == "Config")
        .collect();
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0]["Filter"], CTX);
    assert_eq!(entries[0]["DataSetWriterId"], writer.get());
    assert_eq!(entries[0]["Mode"], "ON_REQUEST_FILTER_9");

    // Profile
    let out = get(&mut p, "Profile");
    let resources = &json_of(&out[0])["Messages"][0]["Payload"]["Resources"];
    assert!(resources.as_array().unwrap().contains(&json!("Config")));
}

/// `Get Config` without Source, with Source and with Filter; an unknown
/// Filter is answered with an empty list (MG 9.1 "Resources without
/// content").
#[test]
fn get_config_selects_by_source_and_filter() {
    let (mut p, src) = provider();
    let pump = p.add_asset(mam(PUMP)).unwrap();
    p.add_config(&pump, spec("pump", device_config()), MS)
        .unwrap();
    p.add_config(&pump, spec("Second", device_config()), MS)
        .unwrap();
    let groups = |out: &[Publication]| -> Vec<(String, String)> {
        json_of(&out[0])["Messages"]
            .as_array()
            .unwrap()
            .iter()
            .map(|m| {
                (
                    m["DataSetWriterName"].as_str().unwrap().to_owned(),
                    m["WriterGroupName"].as_str().unwrap().to_owned(),
                )
            })
            .collect()
    };
    let (a, b) = (src.as_str().to_owned(), pump.as_str().to_owned());
    assert_eq!(
        groups(&get(&mut p, "Config")),
        [
            (a.clone(), CTX.to_owned()),
            (b.clone(), "pump".to_owned()),
            (b.clone(), "Second".to_owned())
        ]
    );
    let by_source = get(&mut p, &format!("Config/{}", pump.encode()));
    assert_eq!(
        by_source[0].topic.to_string(),
        format!("{}/Pub/Config/{}", base(), pump.encode())
    );
    assert_eq!(groups(&by_source).len(), 2);
    let one = get(&mut p, &format!("Config/{}/Second", pump.encode()));
    assert_eq!(groups(&one), [(b, "Second".to_owned())]);
    assert_eq!(
        one[0].topic.to_string(),
        config_topic("Pub", &pump, "Second")
    );
    let none = get(&mut p, &format!("Config/{}/nope", pump.encode()));
    assert_eq!(json_of(&none[0])["Messages"], json!([]));
    assert_masked(&none);
}

// ---------------------------------------------------------- definitions

#[test]
fn definitions_are_checked() {
    let mut p = provider_with(true, 262_144, StoredConfigValues::new());
    let src = il(&app());
    let unknown = il(PUMP);
    assert!(matches!(
        p.add_config(&unknown, spec("a", device_config()), MS),
        Err(ProviderError::UnknownAsset(_))
    ));
    let mut c = device_config();
    c.groups
        .get_mut("device1")
        .unwrap()
        .objects
        .get_mut("port")
        .unwrap()
        .value = "x".into();
    let e = p.add_config(&src, spec("a", c), MS).unwrap_err();
    assert!(matches!(e, ProviderError::Config(_)), "{e}");
    assert!(e.to_string().contains("device1/port"), "{e}");
    assert!(matches!(
        p.add_config(&src, spec("", device_config()), MS),
        Err(ProviderError::Topic(_))
    ));
    // the context name is a Topic encoded String (REQ-TOP-019)
    p.add_config(&src, spec("a/b", device_config()), MS)
        .unwrap();
    let out = p.on_connected(Duration::ZERO, MS);
    assert!(topics(&out).contains(&config_topic("Pub", &src, "a%2Fb")));
    p.add_config(&src, spec("a", device_config()), MS).unwrap();
    assert!(matches!(
        p.add_config(&src, spec("a", device_config()), MS),
        Err(ProviderError::DuplicateConfig { .. })
    ));
    // a Config without Context gets the context name
    let mut c = device_config();
    c.context = None;
    p.add_config(&src, spec("b", c), MS).unwrap();
    let ctx = p.config(&src, "b").unwrap().context.clone().unwrap();
    assert_eq!(ctx.name.text, "b");
    assert_eq!(
        p.config_contexts(&src).collect::<Vec<_>>(),
        ["a/b", "a", "b"]
    );
    // the Config must fit into one packet
    let mut small = provider_with(true, 2048, StoredConfigValues::new());
    assert!(matches!(
        small.add_config(&src, spec("a", device_config()), MS),
        Err(ProviderError::Config(_))
    ));
}

/// Level 1: the Config is published, but no `Get` or `Set` is served.
#[test]
fn level_1_publishes_config_without_requests() {
    let mut p = provider_with(false, 262_144, StoredConfigValues::new());
    let src = il(&app());
    p.add_config(&src, spec(CTX, device_config()), MS).unwrap();
    assert!(p.subscriptions().is_empty());
    let out = p.on_connected(Duration::ZERO, MS);
    assert!(topics(&out).contains(&config_topic("Pub", &src, CTX)));
}

// ------------------------------------------------------------- Set checks

/// The refused `Set Config` payloads: what, Payload, StatusCode.
fn refused_sets() -> Vec<(&'static str, Json, StatusCode)> {
    let ok =
        |v: &str| json!({"device1": {"aut0-r3fr3sh": {"Value": "true"}, "protocol": {"Value": v}}});
    vec![
        ("unknown context", ok("x"), StatusCode::BAD_NOT_FOUND),
        (
            "unknown group",
            json!({"nope": {}}),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "unknown object",
            json!({"common": {"nope": {"Value": ""}}}),
            StatusCode::BAD_NOT_FOUND,
        ),
        (
            "group no object",
            json!({"common": 1}),
            StatusCode::BAD_DECODING_ERROR,
        ),
        (
            "object no object",
            json!({"common": {"date-time": "x"}}),
            StatusCode::BAD_DECODING_ERROR,
        ),
        (
            "Value missing",
            json!({"common": {"date-time": {}}}),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        (
            "Value no string",
            json!({"device1": {"port": {"Value": 80}, "aut0-r3fr3sh": {"Value": "true"}, "protocol": {"Value": "x"}}}),
            StatusCode::BAD_TYPE_MISMATCH,
        ),
        (
            "Type differs",
            json!({"common": {"date-time": {"Type": "String", "Value": ""}}}),
            StatusCode::BAD_TYPE_MISMATCH,
        ),
        (
            "not of its Type",
            json!({"common": {"date-time": {"Value": "yesterday"}}}),
            StatusCode::BAD_TYPE_MISMATCH,
        ),
        (
            "Name differs",
            json!({"common": {"date-time": {"Value": "", "Name": {"Locale": "en-US", "Text": "x"}}}}),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            "group Name differs",
            json!({"common": {"Name": {"Locale": "en-US", "Text": "x"}, "date-time": {"Value": ""}}}),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            "Validation differs",
            json!({"device1": {"port": {"Value": "1", "Validation": {"Min": 1, "Max": 65535}}, "aut0-r3fr3sh": {"Value": "true"}, "protocol": {"Value": "x"}}}),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            "Context differs",
            json!({"Context": {"Name": {"Locale": "en-US", "Text": "x"}}, "common": {"date-time": {"Value": ""}}}),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        (
            "Max",
            json!({"device1": {"port": {"Value": "65536"}, "aut0-r3fr3sh": {"Value": "true"}, "protocol": {"Value": "x"}}}),
            StatusCode::BAD_OUT_OF_RANGE,
        ),
        ("Pattern", ip("256.1.1.1"), StatusCode::BAD_OUT_OF_RANGE),
        (
            "Values",
            json!({"device1": {"unit": {"Value": "mK"}, "aut0-r3fr3sh": {"Value": "true"}, "protocol": {"Value": "x"}}}),
            StatusCode::BAD_OUT_OF_RANGE,
        ),
        (
            "mandatory missing",
            json!({"device1": {"port": {"Value": "1"}}}),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        ("no object", json!({}), StatusCode::BAD_INVALID_ARGUMENT),
    ]
}

#[test]
fn set_config_status_codes() {
    let (mut p, src) = provider();
    let cases = refused_sets();
    for (what, payload, code) in cases {
        let ctx = if what == "unknown context" {
            "nope"
        } else {
            CTX
        };
        assert_eq!(set(&mut p, &src, ctx, &payload), Err(bad(code)), "{what}");
    }
    // the Payload is no object; not exactly one DataSetMessage
    assert_eq!(
        set(&mut p, &src, CTX, &json!([])),
        Err(bad(StatusCode::BAD_DECODING_ERROR))
    );
    assert_eq!(
        set_messages(&mut p, &src, CTX, &json!([])),
        Err(bad(StatusCode::BAD_INVALID_ARGUMENT))
    );
    // nothing changed
    assert_eq!(value_of(&p, &src, "device1", "port"), "");
    // a valid partial update with unknown members (tolerant reader)
    let req = set(&mut p, &src, CTX, &ip("10.0.0.1")).unwrap();
    assert_eq!(req.changes().len(), 3);
}

fn ip(v: &str) -> Json {
    json!({"device1": {
        "ip_address": {"Value": v, "Comment": "ignored"},
        "aut0-r3fr3sh": {"Value": "true"},
        "protocol": {"Value": "HTTPS"}
    }})
}

/// The mask keeps a sensitive value (REQ-RES-CONFIG-025); a value is set
/// in clear text and never published.
#[test]
fn the_mask_keeps_a_sensitive_value() {
    let (mut p, src) = provider();
    let with_password = |v: &str| {
        json!({"device1": {
            "password": {"Value": v, "Sensitive": true},
            "aut0-r3fr3sh": {"Value": "true"},
            "protocol": {"Value": "HTTPS"}
        }})
    };
    let req = set(&mut p, &src, CTX, &with_password(SECRET)).unwrap();
    let (out, _) = p.complete_set_config(&req, Ok(()), MS + 2);
    assert_masked(&out);
    let req = set(&mut p, &src, CTX, &with_password("********")).unwrap();
    assert!(req.changes().iter().all(|(_, n, _)| n != "password"));
    p.complete_set_config(&req, Ok(()), MS + 3);
    assert_eq!(value_of(&p, &src, "device1", "password"), SECRET);
    // Sensitive is read-only
    let mut flip = with_password("x");
    flip["device1"]["password"]["Sensitive"] = json!(false);
    assert_eq!(
        set(&mut p, &src, CTX, &flip),
        Err(bad(StatusCode::BAD_NOT_WRITABLE))
    );
    // the mask is no value of an object that is not sensitive
    let mut user = with_password("x");
    user["device1"]["user-name"] = json!({"Value": "********"});
    let req = set(&mut p, &src, CTX, &user).unwrap();
    p.complete_set_config(&req, Ok(()), MS + 4);
    assert_eq!(value_of(&p, &src, "device1", "user-name"), "********");
    // unset: an empty value
    let req = set(&mut p, &src, CTX, &with_password("")).unwrap();
    let (out, changed) = p.complete_set_config(&req, Ok(()), MS + 5);
    assert!(changed);
    assert_eq!(
        json_of(&out[0])["Messages"][0]["Payload"]["device1"]["password"]["Value"],
        ""
    );
}

// --------------------------------------------------------- completion

#[test]
fn the_application_decides() {
    let (mut p, src) = provider();
    let req = set(&mut p, &src, CTX, &ip("10.0.0.1")).unwrap();
    // refused: only the Status event, nothing applied
    let (out, changed) = p.complete_set_config(&req, Err(StatusCode::BAD_OUT_OF_RANGE), MS + 2);
    assert!(!changed);
    assert_eq!(out.len(), 1);
    assert_eq!(status(&out), bad(StatusCode::BAD_OUT_OF_RANGE));
    assert_eq!(value_of(&p, &src, "device1", "ip_address"), "");
    assert!(p.take_config_values().is_none());
    // an Err with a Good code is Bad
    let (out, _) = p.complete_set_config(&req, Err(StatusCode::GOOD), MS + 3);
    assert_eq!(status(&out), bad(StatusCode::BAD));
    // accepted: applied and to be persisted
    let (out, changed) = p.complete_set_config(&req, Ok(()), MS + 4);
    assert!(changed);
    assert_eq!(status(&out), json!({"Level": "Good", "Number": 0}));
    assert_eq!(value_of(&p, &src, "device1", "ip_address"), "10.0.0.1");
    assert!(p.take_config_values().is_some());
    assert!(p.take_config_values().is_none());
    // nothing pending: the answer was the publication
    assert!(p.poll(Duration::from_millis(5), MS + 5).is_empty());
}

/// A request for a Config DataSet removed or added anew meanwhile is
/// answered with BadNotFound.
#[test]
fn a_stale_request_is_not_applied() {
    let (mut p, _) = provider();
    let pump = p.add_asset(mam(PUMP)).unwrap();
    p.add_config(&pump, spec(CTX, device_config()), MS).unwrap();
    let req = set(&mut p, &pump, CTX, &ip("10.0.0.1")).unwrap();
    p.remove_asset(&pump).unwrap();
    p.add_asset(mam(PUMP)).unwrap();
    p.add_config(&pump, spec(CTX, device_config()), MS).unwrap();
    let (out, changed) = p.complete_set_config(&req, Ok(()), MS + 2);
    assert!(!changed);
    assert_eq!(status(&out), bad(StatusCode::BAD_NOT_FOUND));
    assert_eq!(
        p.config(&pump, CTX).unwrap().groups["device1"].objects["ip_address"].value,
        ""
    );
}

/// `Del Config` is not supported (MG 9.1.3): no allowed topic, not
/// subscribed.
#[test]
fn del_config_is_no_request() {
    let (mut p, src) = provider();
    let topic = config_topic("Del", &src, CTX);
    let messages =
        json!([{"DataSetWriterId": 43, "DataSetWriterName": src.as_str(), "Payload": {}}]);
    let r = p.handle_message(&topic, &request(SET_ID, &messages), MS + 1);
    assert!(matches!(r, Err(Ignored::Topic(_))), "{r:?}");
    assert!(
        !p.subscriptions()
            .iter()
            .any(|s| s.to_string().contains("/Del/Config")),
        "Del Config is not subscribed"
    );
}

/// A Set whose Config would exceed the MaxPacketSize is refused.
#[test]
fn a_config_beyond_the_packet_size_is_refused() {
    let mut p = provider_with(true, 4096, StoredConfigValues::new());
    let src = il(&app());
    p.add_config(&src, spec(CTX, device_config()), MS).unwrap();
    let mut long = ip("10.0.0.1");
    long["device1"]["user-name"] = json!({"Value": "x".repeat(4096)});
    assert_eq!(
        set(&mut p, &src, CTX, &long),
        Err(bad(StatusCode::BAD_OUT_OF_RANGE))
    );
    let e = p
        .set_config(
            &src,
            CTX,
            &[("device1".into(), "user-name".into(), "x".repeat(4096))],
            MS,
        )
        .unwrap_err();
    assert!(matches!(e, ProviderError::Value(_)), "{e}");
}

#[test]
fn set_config_from_the_application_is_checked() {
    let (mut p, src) = provider();
    let change = |g: &str, n: &str, v: &str| vec![(g.to_owned(), n.to_owned(), v.to_owned())];
    assert!(matches!(
        p.set_config(&src, "nope", &change("device1", "port", "1"), MS),
        Err(ProviderError::UnknownConfig { .. })
    ));
    let e = p
        .set_config(&src, CTX, &change("device1", "port", "99999"), MS)
        .unwrap_err();
    assert!(e.to_string().contains("device1/port"), "{e}");
    assert!(!e.to_string().contains("99999"), "{e}");
    assert!(matches!(
        p.set_config(&src, CTX, &change("device1", "nope", "1"), MS),
        Err(ProviderError::Value(_))
    ));
    // all or nothing
    let mut both = change("device1", "port", "1");
    both.extend(change("device1", "unit", "mK"));
    assert!(p.set_config(&src, CTX, &both, MS).is_err());
    assert_eq!(value_of(&p, &src, "device1", "port"), "");
    assert!(
        !p.set_config(&src, CTX, &change("device1", "port", ""), MS)
            .unwrap()
    );
}

// ---------------------------------------------------------- persistence

/// The values set survive a restart; a value no longer valid for the
/// definition falls back to the initial one.
#[test]
fn values_are_persisted() {
    let (mut p, src) = provider();
    let req = set(&mut p, &src, CTX, &ip("10.0.0.1")).unwrap();
    p.complete_set_config(&req, Ok(()), MS + 2);
    let password = vec![(
        "device1".to_owned(),
        "password".to_owned(),
        SECRET.to_owned(),
    )];
    p.set_config(&src, CTX, &password, MS + 3).unwrap();
    let json = p.take_config_values().unwrap();
    let debug = format!("{:?}", p.config_values());
    assert!(!debug.contains("secret"), "{debug}");

    let stored = StoredConfigValues::from_json(&json).unwrap();
    let mut restarted = provider_with(true, 262_144, stored.clone());
    restarted
        .add_config(&src, spec(CTX, device_config()), MS)
        .unwrap();
    assert_eq!(
        value_of(&restarted, &src, "device1", "ip_address"),
        "10.0.0.1"
    );
    assert_eq!(value_of(&restarted, &src, "device1", "password"), SECRET);
    assert_eq!(value_of(&restarted, &src, "device1", "protocol"), "HTTPS");
    assert!(restarted.take_config_values().is_none());

    // the definition became stricter: the initial value applies
    let mut stricter = provider_with(true, 262_144, stored);
    let mut c = device_config();
    let o = c
        .groups
        .get_mut("device1")
        .unwrap()
        .objects
        .get_mut("ip_address")
        .unwrap();
    o.validation.as_mut().unwrap().pattern = Some("^192\\.".into());
    stricter.add_config(&src, spec(CTX, c), MS).unwrap();
    assert_eq!(value_of(&stricter, &src, "device1", "ip_address"), "");
    assert_eq!(value_of(&stricter, &src, "device1", "protocol"), "HTTPS");

    assert!(StoredConfigValues::from_json(b"{}").is_err());
    assert!(StoredConfigValues::from_json(b"not json").is_err());
}

#[cfg(feature = "strict-validation")]
#[test]
fn every_answer_passes_the_schema() {
    use oec_core::schema::{Flavor, Validator};
    let validator = Validator::new().unwrap();
    let (mut p, src) = provider();
    let mut all = get(&mut p, "Config");
    all.extend(get(&mut p, &format!("Config/{}/{CTX}", src.encode())));
    all.extend(get(&mut p, &format!("PublicationList/{}", src.encode())));
    let req = set(&mut p, &src, CTX, &ip("10.0.0.1")).unwrap();
    all.extend(p.complete_set_config(&req, Ok(()), MS + 2).0);
    let topic = config_topic("Set", &src, CTX);
    let refused =
        json!([{"DataSetWriterId": 43, "DataSetWriterName": src.as_str(), "Payload": ip("1")}]);
    all.extend(publish(
        p.handle_message(&topic, &request(SET_ID, &refused), MS + 3)
            .unwrap(),
    ));
    assert_eq!(all.len(), 6);
    for o in all {
        validator
            .validate_topic(&o.topic, Flavor::Strict, &json_of(&o))
            .unwrap_or_else(|e| panic!("{}: {e}\n{}", o.topic, json_of(&o)));
    }
}
