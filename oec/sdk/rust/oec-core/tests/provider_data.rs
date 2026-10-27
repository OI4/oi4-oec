//! The Data DataSets of the provider (P4 increment 6b): Metadata from the
//! field definitions (REQ-RES-META-*), Get Data and Get Metadata
//! (REQ-RES-DATA-007, REQ-CMP-044), the PublicationList entries with their
//! Modes (REQ-RES-PUBLIST-007 … 012/026), Intervals and Precisions
//! (REQ-RES-PUBLIST-013 … 015, REQ-MQTT-019), the DataSetClassId
//! (REQ-MSG-011, REQ-MSG-013) and the ConfigurationVersion across restarts
//! (REQ-MSG-032, REQ-MSG-036). The guideline vectors V-PAY-020/021 and
//! V-PAY-055/058 are replayed against the provider.

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::data::{
    BuiltinType as T, DataSetSpec, FieldSpec, Mode, OI4PV, Value, Writable,
};
use oec_core::provider::{
    Ignored, MetaDataVersions, Provider, ProviderConfig, ProviderError, WriterIds,
};
use oec_core::resource::Mam;
use oec_core::topic::ServiceType;
use serde_json::{Value as Json, json};
use uuid::Uuid;

const APP: &str = "vendor.com/Gateway/GW-1/SN-1";
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const MS: u64 = 1_700_000_000_000;
const GET_ID: &str = "1700000000000-Aggregation/vendor.com/Client/C-1/1";

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

fn provider(get: bool) -> Provider {
    provider_of(ServiceType::Aggregation, APP, get, WriterIds::new())
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

fn get(messages: &Json) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": GET_ID,
        "MessageType": "ua-data",
        "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
        "ReplyTo": "",
        "Messages": messages
    }))
    .unwrap()
}

fn get_meta(source: &str, tag: &str) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": GET_ID,
        "MessageType": "ua-metadata",
        "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
        "DataSetWriterId": 10,
        "WriterGroupName": tag,
        "DataSetWriterName": source,
        "Timestamp": "2023-11-14T22:13:20.000Z",
        "ReplyTo": "",
        "MetaData": {}
    }))
    .unwrap()
}

fn temp(tag: &str, mode: Mode) -> DataSetSpec {
    let mut d = DataSetSpec::new(tag);
    d.fields = vec![FieldSpec::new("T", T::Double)];
    d.publication.mode = mode;
    d
}

fn set(p: &mut Provider, src: &IdentificationLink, tag: &str, v: f64, ms: u64) -> bool {
    p.update_data(src, tag, vec![("T".to_owned(), Value::from(v))], ms)
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

/// The DataSet `oee` of MG B1.1.4/B1.1.5 rebuilt from the Metadata of
/// V-PAY-021.
fn oee_from(meta: &Json) -> DataSetSpec {
    let mut d = DataSetSpec::new(meta["Name"].as_str().unwrap());
    d.description = Some(meta["Description"]["Text"].as_str().unwrap().to_owned());
    d.class_id = Some(meta["DataSetClassId"].as_str().unwrap().parse().unwrap());
    for f in meta["Fields"].as_array().unwrap() {
        assert_eq!(f["BuiltInType"], 11);
        let mut fs = FieldSpec::new(f["Name"].as_str().unwrap(), T::Double);
        fs.description = Some(f["Description"]["Text"].as_str().unwrap().to_owned());
        for p in f["Properties"].as_array().unwrap() {
            let v = &p["Value"]["Value"];
            match p["Key"].as_str().unwrap() {
                "EURange" => {
                    fs.range = Some((v["Low"].as_f64().unwrap(), v["High"].as_f64().unwrap()));
                }
                "EngineeringUnits" => {
                    fs.unit = Some(v["DisplayName"]["Text"].as_str().unwrap().to_owned());
                }
                k => panic!("unexpected property {k}"),
            }
        }
        d.fields.push(fs);
    }
    d
}

/// MG B1.1.4/B1.1.5: the Get vectors V-PAY-055 and V-PAY-020 are answered
/// like V-PAY-058 and V-PAY-021, up to the generated ids and times.
#[test]
fn guideline_get_vectors_are_answered_like_the_guideline() {
    let (_, meta_pub) = vector("V-PAY-021");
    let (data_topic, data_pub) = vector("V-PAY-058");
    let app = data_pub["Messages"][0]["DataSetWriterName"]
        .as_str()
        .unwrap()
        .to_owned();
    let mut p = provider_of(ServiceType::OTConnector, &app, true, WriterIds::new());
    let src = il(&app);
    p.add_data(&src, oee_from(&meta_pub["MetaData"]), MS)
        .unwrap();
    let values = &data_pub["Messages"][0]["Payload"];
    let changes = values
        .as_object()
        .unwrap()
        .iter()
        .map(|(k, v)| (k.clone(), Value::from(v.as_f64().unwrap())))
        .collect();
    p.update_data(&src, "oee", changes, MS).unwrap();
    p.on_connected(Duration::ZERO, MS);

    // Get Data/<Source>/oee → Pub Data/<Source>/oee
    let (get_topic, get_payload) = vector("V-PAY-055");
    let out = p
        .handle_message(
            &get_topic,
            &serde_json::to_vec(&get_payload).unwrap(),
            MS + 1,
        )
        .unwrap();
    assert_eq!(topics(&out), [data_topic]);
    let ans = json_of(&out[0]);
    for k in ["MessageType", "PublisherId", "DataSetClassId", "ReplyTo"] {
        assert_eq!(ans[k], data_pub[k], "{k}");
    }
    let (m, want) = (&ans["Messages"][0], &data_pub["Messages"][0]);
    for k in ["WriterGroupName", "DataSetWriterName"] {
        assert_eq!(m[k], want[k], "{k}");
    }
    for (k, v) in want["Payload"].as_object().unwrap() {
        assert_eq!(m["Payload"][k].as_f64(), v.as_f64(), "{k}");
    }
    assert_eq!(m["Payload"].as_object().unwrap().len(), 4);

    // Get Metadata/<Source>/oee → Pub Metadata/<Source>/oee
    let (get_topic, get_payload) = vector("V-PAY-020");
    let (meta_topic, _) = vector("V-PAY-021");
    let out = p
        .handle_message(
            &get_topic,
            &serde_json::to_vec(&get_payload).unwrap(),
            MS + 2,
        )
        .unwrap();
    assert_eq!(topics(&out), [meta_topic]);
    let ans = json_of(&out[0]);
    for k in [
        "MessageType",
        "PublisherId",
        "WriterGroupName",
        "DataSetWriterName",
        "ReplyTo",
    ] {
        assert_eq!(ans[k], meta_pub[k], "{k}");
    }
    // the same DataSetWriterId as the Data (REQ-MSG-033)
    assert_eq!(ans["DataSetWriterId"], m["DataSetWriterId"]);
    let (got, want) = (&ans["MetaData"], &meta_pub["MetaData"]);
    for k in [
        "Name",
        "Description",
        "DataSetClassId",
        "Namespaces",
        "StructureDataTypes",
        "EnumDataTypes",
        "SimpleDataTypes",
    ] {
        assert_eq!(got[k], want[k], "{k}");
    }
    let fields = got["Fields"].as_array().unwrap();
    assert_eq!(fields.len(), 4);
    for (g, w) in fields.iter().zip(want["Fields"].as_array().unwrap()) {
        for k in [
            "Name",
            "Description",
            "FieldFlags",
            "BuiltInType",
            "DataType",
            "ValueRank",
            "ArrayDimensions",
            "MaxStringLength",
        ] {
            assert_eq!(g[k], w[k], "{k}");
        }
        let range = |v: &Json| v["Properties"][0]["Value"]["Value"].clone();
        assert_eq!(range(g)["Low"].as_f64(), range(w)["Low"].as_f64());
        assert_eq!(range(g)["High"].as_f64(), range(w)["High"].as_f64());
        let unit = |v: &Json| v["Properties"][1]["Value"]["Value"]["DisplayName"].clone();
        assert_eq!(unit(g), unit(w));
    }
    // the version is a VersionTime of the first add (REQ-MSG-036)
    let v = &got["ConfigurationVersion"];
    assert_eq!(v["MajorVersion"], v["MinorVersion"]);
}

// ------------------------------------------------------------------ modes

const MODES: [(&str, Mode); 5] = [
    ("f", Mode::ON_REQUEST.union(Mode::FILTER)),
    ("s", Mode::ON_REQUEST.union(Mode::SOURCE)),
    ("a", Mode::ON_REQUEST.union(Mode::APPLICATION)),
    ("r", Mode::ON_REQUEST),
    ("o", Mode::OFF),
];

/// A provider with a pump DataSet per Mode and a SOURCE DataSet of the app.
fn modes_provider() -> (Provider, IdentificationLink, IdentificationLink) {
    let mut p = provider(true);
    let app = il(APP);
    let pump = p.add_asset(mam(PUMP)).unwrap();
    for (tag, mode) in MODES {
        p.add_data(&pump, temp(tag, mode), MS).unwrap();
    }
    p.add_data(&app, temp("s2", Mode::SOURCE), MS).unwrap();
    (p, app, pump)
}

fn writer_group_names(v: &Json) -> Vec<String> {
    v["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| m["WriterGroupName"].as_str().unwrap().to_owned())
        .collect()
}

#[test]
fn every_mode_publishes_its_forms() {
    let (mut p, app, pump) = modes_provider();
    let out = p.on_connected(Duration::ZERO, MS);
    let (app_seg, pump_seg) = (app.encode(), pump.encode());
    // no values yet: only the Metadata of what is not OFF (REQ-RES-META-003)
    let meta: Vec<_> = topics(&out)
        .into_iter()
        .filter(|t| t.contains("/Pub/Metadata/"))
        .collect();
    assert_eq!(
        meta,
        ["f", "s", "a", "r"]
            .map(|t| topic("Pub", &format!("Metadata/{pump_seg}/{t}")))
            .into_iter()
            .chain([topic("Pub", &format!("Metadata/{app_seg}/s2"))])
            .collect::<Vec<_>>()
    );
    assert!(topics(&out).iter().all(|t| !t.contains("/Pub/Data")));

    // a value set: each DataSet in its forms, a group with every member
    // that has values
    for (tag, _) in MODES {
        set(&mut p, &pump, tag, 1.0, MS + 1);
    }
    let out = p.poll(Duration::from_millis(1), MS + 1);
    assert_eq!(
        topics(&out),
        [
            topic("Pub", "Data"),
            topic("Pub", &format!("Data/{pump_seg}")),
            topic("Pub", &format!("Data/{pump_seg}/f")),
        ]
    );
    assert_eq!(writer_group_names(&json_of(&out[0])), ["a"]);
    assert_eq!(writer_group_names(&json_of(&out[1])), ["s"]);
    assert_eq!(writer_group_names(&json_of(&out[2])), ["f"]);
    // one class per NetworkMessage: its DataSetClassId (REQ-MSG-011)
    let class = p.meta_data(&pump, "f").unwrap().data_set_class_id.unwrap();
    assert_eq!(json_of(&out[2])["DataSetClassId"], class.to_string());
    // MessageType ua-keyframe, Timestamp, MetaDataVersion (REQ-MSG-018)
    let m = &json_of(&out[2])["Messages"][0];
    assert_eq!(m["Payload"], json!({"T": 1.0}));
    assert_eq!(m["Timestamp"], "2023-11-14T22:13:20.001Z");
    let v = p
        .meta_data(&pump, "f")
        .unwrap()
        .configuration_version
        .unwrap();
    assert_eq!(m["MetaDataVersion"]["MajorVersion"], v.major_version);
    assert!(m.get("MessageType").is_none_or(|t| t == "ua-keyframe"));

    // the app's SOURCE DataSet joins its own group, not the pump's
    set(&mut p, &app, "s2", 2.0, MS + 2);
    set(&mut p, &pump, "s", 2.0, MS + 2);
    let out = p.poll(Duration::from_millis(2), MS + 2);
    assert_eq!(
        topics(&out),
        [
            topic("Pub", &format!("Data/{pump_seg}")),
            topic("Pub", &format!("Data/{app_seg}"))
        ]
    );
    // ON_REQUEST and OFF never publish
    set(&mut p, &pump, "r", 3.0, MS + 3);
    set(&mut p, &pump, "o", 3.0, MS + 3);
    assert_eq!(p.poll(Duration::from_millis(3), MS + 3), []);
}

#[test]
fn get_data_and_get_metadata_follow_the_modes() {
    let (mut p, app, pump) = modes_provider();
    p.on_connected(Duration::ZERO, MS);
    let pump_seg = pump.encode();
    for (tag, _) in MODES {
        set(&mut p, &pump, tag, 1.0, MS + 1);
    }
    set(&mut p, &app, "s2", 2.0, MS + 1);

    // Get Data: every DataSet not OFF (REQ-RES-PUBLIST-007/008)
    let out = p
        .handle_message(
            &topic("Get", &format!("Data/{pump_seg}")),
            &get(&json!([])),
            MS + 4,
        )
        .unwrap();
    assert_eq!(topics(&out), [topic("Pub", &format!("Data/{pump_seg}"))]);
    let v = json_of(&out[0]);
    assert_eq!(writer_group_names(&v), ["f", "s", "a", "r"]);
    assert_eq!(v["ReplyTo"], GET_ID);
    // different classes: no DataSetClassId (REQ-MSG-011)
    assert!(v.get("DataSetClassId").is_none_or(|c| c == ""), "{v}");
    let out = p
        .handle_message(&topic("Get", "Data"), &get(&json!([])), MS + 4)
        .unwrap();
    assert_eq!(
        writer_group_names(&json_of(&out[0])),
        ["s2", "f", "s", "a", "r"]
    );
    // OFF: an empty answer (REQ-CMP-041), no Metadata (REQ-CMP-044)
    let out = p
        .handle_message(
            &topic("Get", &format!("Data/{pump_seg}/o")),
            &get(&json!([])),
            MS + 5,
        )
        .unwrap();
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
    let r = p.handle_message(
        &topic("Get", &format!("Metadata/{pump_seg}/o")),
        &get_meta(PUMP, "o"),
        MS + 5,
    );
    assert!(matches!(r, Err(Ignored::NoMetadata(_))), "{r:?}");
    let r = p.handle_message(
        &topic("Get", &format!("Metadata/{pump_seg}/unknown")),
        &get_meta(PUMP, "unknown"),
        MS + 5,
    );
    assert!(matches!(r, Err(Ignored::NoMetadata(_))), "{r:?}");
    // an unknown Tag of Data: empty (REQ-CMP-041)
    let out = p
        .handle_message(
            &topic("Get", &format!("Data/{pump_seg}/unknown")),
            &get(&json!([])),
            MS + 6,
        )
        .unwrap();
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
}

#[test]
fn the_publication_list_describes_data_and_metadata() {
    let mut p = provider(true);
    let pump = p.add_asset(mam(PUMP)).unwrap();
    let mut d = temp("oee", Mode::ON_REQUEST | Mode::SOURCE | Mode::FILTER);
    d.publication.interval_ms = 1000;
    d.publication.writable = Writable::ModeAndInterval;
    let id = p.add_data(&pump, d, MS).unwrap();
    let mut d = temp("t%/1", Mode::FILTER);
    d.publication.precisions.insert("T".to_owned(), 0.5);
    p.add_data(&pump, d, MS).unwrap();
    p.add_data(&pump, temp("off", Mode::OFF), MS).unwrap();
    let out = p.on_connected(Duration::ZERO, MS);
    let list = out
        .iter()
        .find(|o| o.topic.to_string() == topic("Pub", "PublicationList"))
        .unwrap();
    let v = json_of(list);
    let entries: Vec<&Json> = v["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|m| m["DataSetWriterName"] == PUMP)
        .collect();
    let groups: Vec<_> = entries
        .iter()
        .map(|e| e["WriterGroupName"].clone())
        .collect();
    assert_eq!(
        groups,
        [
            "MAM",
            "Health",
            "Profile",
            "Data/oee",
            "Metadata/oee",
            "Data/t%25%2F1",
            "Metadata/t%25%2F1",
            "Data/off"
        ]
    );
    let pl = |i: usize| entries[i]["Payload"].clone();
    assert_eq!(
        pl(3),
        json!({
            "Resource": "Data",
            "Source": PUMP,
            "Filter": "oee",
            "DataSetWriterId": id.get(),
            "Mode": "ON_REQUEST_SOURCE_FILTER_12",
            "Interval": 1000,
            "Config": "MODE_AND_INTERVAL_3"
        })
    );
    assert_eq!(
        pl(4),
        json!({
            "Resource": "Metadata",
            "Source": PUMP,
            "Filter": "oee",
            "DataSetWriterId": id.get(),
            "Mode": "ON_REQUEST_FILTER_9"
        })
    );
    // FILTER_4 is sent as ON_REQUEST_FILTER_9 (REQ-RES-PUBLIST-026)
    assert_eq!(pl(5)["Mode"], "ON_REQUEST_FILTER_9");
    assert_eq!(pl(5)["Filter"], "t%25%2F1");
    assert_eq!(pl(5)["Precisions"], json!({"T": 0.5}));
    assert_eq!(pl(7)["Mode"], "OFF_0");

    // Get PublicationList/<Source>/Data[/<Tag>] and Metadata
    let s = pump.encode();
    let ask = |p: &mut Provider, rest: &str| -> Vec<Json> {
        let out = p
            .handle_message(&topic("Get", rest), &get(&json!([])), MS + 1)
            .unwrap();
        json_of(&out[0])["Messages"].as_array().unwrap().clone()
    };
    let got = ask(&mut p, &format!("PublicationList/{s}/Data"));
    assert_eq!(got.len(), 3);
    let got = ask(&mut p, &format!("PublicationList/{s}/Data/oee"));
    assert_eq!(got.len(), 1);
    assert_eq!(got[0]["Payload"], pl(3));
    let got = ask(&mut p, &format!("PublicationList/{s}/Metadata/t%25%2F1"));
    assert_eq!(got.len(), 1);
    assert_eq!(got[0]["Payload"], pl(6));
    assert!(ask(&mut p, &format!("PublicationList/{s}/Metadata/off")).is_empty());

    // remove: the list again, the id stays reserved
    p.remove_data(&pump, "oee").unwrap();
    let out = p.poll(Duration::from_millis(1), MS + 2);
    assert_eq!(topics(&out), [topic("Pub", "PublicationList")]);
    let again = p
        .add_data(&pump, temp("oee", Mode::ON_REQUEST), MS + 3)
        .unwrap();
    assert_eq!(again, id);
    // removing the asset removes its DataSets
    p.remove_asset(&pump).unwrap();
    assert_eq!(p.data_tags(&pump).count(), 0);
}

// ------------------------------------------------ dead band and Interval

#[test]
fn precisions_are_a_dead_band() {
    let mut p = provider(false);
    let app = il(APP);
    let mut d = temp("t", Mode::FILTER);
    d.publication.precisions.insert("T".to_owned(), 0.5);
    p.add_data(&app, d, MS).unwrap();
    p.on_connected(Duration::ZERO, MS);
    let sent = |p: &mut Provider, v: f64, n: u64| -> Option<f64> {
        set(p, &app, "t", v, MS + n);
        let out = p.poll(Duration::from_millis(n), MS + n);
        assert!(out.len() <= 1);
        out.first()
            .map(|o| json_of(o)["Messages"][0]["Payload"]["T"].as_f64().unwrap())
    };
    assert_eq!(sent(&mut p, 10.0, 1), Some(10.0));
    assert_eq!(sent(&mut p, 10.3, 2), None);
    assert_eq!(sent(&mut p, 9.6, 3), None);
    // measured against the last published value, not the last value
    assert_eq!(sent(&mut p, 10.5, 4), Some(10.5));
    assert_eq!(sent(&mut p, 10.0, 5), Some(10.0));
    // a non-finite value is always a change, sent as a string
    set(&mut p, &app, "t", f64::NAN, MS + 6);
    let out = p.poll(Duration::from_millis(6), MS + 6);
    assert_eq!(json_of(&out[0])["Messages"][0]["Payload"]["T"], "NaN");
    // but the state is current: Get answers the last value
    set(&mut p, &app, "t", 1.0, MS + 7);
    p.poll(Duration::from_millis(7), MS + 7);
    set(&mut p, &app, "t", 1.2, MS + 8);
    assert_eq!(p.poll(Duration::from_millis(8), MS + 8), []);
    assert_eq!(p.data_values(&app, "t"), Some(vec![Value::Double(1.2)]));
}

#[test]
fn an_interval_publishes_cyclically() {
    let mut p = provider(false);
    let app = il(APP);
    let mut d = temp("t", Mode::FILTER);
    d.publication.interval_ms = 100;
    // ignored with an Interval (REQ-RES-PUBLIST-014)
    d.publication.precisions.insert("T".to_owned(), 1000.0);
    p.add_data(&app, d, MS).unwrap();
    let t0 = Duration::from_secs(5);
    p.on_connected(t0, MS);
    let heartbeat = p.poll_timeout();
    assert!(heartbeat > t0 + Duration::from_millis(100));
    // the first value set at once
    set(&mut p, &app, "t", 1.0, MS);
    assert_eq!(p.poll_timeout(), Duration::ZERO);
    assert_eq!(p.poll(t0, MS).len(), 1);
    let due = t0 + Duration::from_millis(100);
    assert_eq!(p.poll_timeout(), due);
    // a change waits for the Interval
    set(&mut p, &app, "t", 2.0, MS + 10);
    assert_eq!(p.poll_timeout(), due);
    assert_eq!(p.poll(due - Duration::from_millis(1), MS + 99), []);
    let out = p.poll(due, MS + 100);
    assert_eq!(json_of(&out[0])["Messages"][0]["Payload"]["T"], 2.0);
    // without a change as well (REQ-RES-PUBLIST-013), no catch-up burst
    let late = due + Duration::from_millis(250);
    assert_eq!(p.poll(late, MS + 350).len(), 1);
    assert_eq!(p.poll_timeout(), late + Duration::from_millis(100));
    // offline: nothing; after the reconnect at once, then the cycle anew
    p.on_disconnected();
    assert_eq!(p.poll(late + Duration::from_secs(1), MS + 1350), []);
    let t1 = late + Duration::from_secs(2);
    let out = p.on_connected(t1, MS + 2350);
    assert!(topics(&out).contains(&topic("Pub", &format!("Data/{}/t", app.encode()))));
    assert_eq!(p.poll_timeout(), t1 + Duration::from_millis(100));
}

// --------------------------------------------- level 1, versions, errors

#[test]
fn level_1_publishes_metadata_and_data_without_get() {
    let mut p = provider(false);
    let app = il(APP);
    p.add_data(&app, temp("t", Mode::ON_REQUEST | Mode::FILTER), MS)
        .unwrap();
    set(&mut p, &app, "t", 1.0, MS);
    let out = p.on_connected(Duration::ZERO, MS);
    let a = app.encode();
    assert_eq!(
        topics(&out),
        [
            topic("Pub", &format!("MAM/{a}")),
            topic("Pub", &format!("Health/{a}")),
            topic("Pub", &format!("Metadata/{a}/t")),
            topic("Pub", &format!("Data/{a}/t")),
        ]
    );
    assert!(p.subscriptions().is_empty());
    let r = p.handle_message(&topic("Get", &format!("Data/{a}/t")), &get(&json!([])), MS);
    assert!(matches!(r, Err(Ignored::NotServed(_))), "{r:?}");
}

#[test]
fn metadata_versions_survive_restarts() {
    let app = il(APP);
    let start = |versions: MetaDataVersions, writers: WriterIds, d: DataSetSpec, ms: u64| {
        let mut p = provider_of(ServiceType::Aggregation, APP, true, writers)
            .with_meta_data_versions(versions);
        p.add_data(&app, d, ms).unwrap();
        let out = p.on_connected(Duration::ZERO, ms);
        let meta = out
            .iter()
            .find(|o| o.topic.to_string().contains("/Pub/Metadata/"))
            .map(json_of)
            .unwrap();
        let versions = p.take_meta_data_versions().map_or_else(
            || p.meta_data_versions().clone(),
            |j| MetaDataVersions::from_json(&j).unwrap(),
        );
        let writers = WriterIds::from_json(&p.writer_ids().to_json()).unwrap();
        (meta, versions, writers)
    };
    let d = temp("t", Mode::FILTER);
    let (m1, v, w) = start(MetaDataVersions::new(), WriterIds::new(), d.clone(), MS);
    // unchanged: same version, same Timestamp (first sending), same class
    let (m2, v, w) = start(v, w, d.clone(), MS + 60_000);
    assert_eq!(m2["MetaData"], m1["MetaData"]);
    assert_eq!(m2["Timestamp"], m1["Timestamp"]);
    assert_eq!(m2["DataSetWriterId"], m1["DataSetWriterId"]);
    // a new description: only the MinorVersion (REQ-MSG-036)
    let mut d2 = d.clone();
    d2.description = Some("Temperature".to_owned());
    let (m3, v, w) = start(v, w, d2.clone(), MS + 120_000);
    let ver = |m: &Json, k: &str| m["MetaData"]["ConfigurationVersion"][k].as_u64().unwrap();
    assert_eq!(ver(&m3, "MajorVersion"), ver(&m1, "MajorVersion"));
    assert_eq!(ver(&m3, "MinorVersion"), ver(&m1, "MinorVersion") + 120);
    assert_eq!(
        m3["MetaData"]["DataSetClassId"],
        m1["MetaData"]["DataSetClassId"]
    );
    assert_ne!(m3["Timestamp"], m1["Timestamp"]);
    // a new structure: a new class and MajorVersion (REQ-MSG-013)
    let mut d3 = d2;
    d3.fields[0].ty = T::Float;
    let (m4, _, _) = start(v, w, d3, MS + 180_000);
    assert_ne!(
        m4["MetaData"]["DataSetClassId"],
        m1["MetaData"]["DataSetClassId"]
    );
    assert_eq!(ver(&m4, "MajorVersion"), ver(&m1, "MajorVersion") + 180);
    assert_eq!(ver(&m4, "MinorVersion"), ver(&m4, "MajorVersion"));
}

#[test]
fn definitions_and_values_are_checked() {
    let mut p = provider(true);
    let app = il(APP);
    let other = il(PUMP);
    assert!(matches!(
        p.add_data(&other, temp("t", Mode::FILTER), MS),
        Err(ProviderError::UnknownAsset(_))
    ));
    assert!(matches!(
        p.add_data(&app, DataSetSpec::new("empty"), MS),
        Err(ProviderError::DataSet(_))
    ));
    assert!(matches!(
        p.add_data(&app, temp("", Mode::FILTER), MS),
        Err(ProviderError::Topic(_))
    ));
    let mut pv = DataSetSpec::new(OI4PV);
    pv.fields = vec![FieldSpec::new("Sv1", T::Double)];
    assert!(matches!(
        p.add_data(&app, pv.clone(), MS),
        Err(ProviderError::DataSet(_))
    ));
    pv.fields.push(FieldSpec::new("Pv", T::Double));
    p.add_data(&app, pv, MS).unwrap();
    let mut d = DataSetSpec::new("mixed");
    d.fields = vec![
        FieldSpec::new("i", T::SByte),
        FieldSpec::new("g", T::Guid),
        FieldSpec::new("b", T::Boolean),
    ];
    p.add_data(&app, d, MS).unwrap();
    assert!(matches!(
        p.add_data(&app, temp("mixed", Mode::FILTER), MS),
        Err(ProviderError::DuplicateDataSet { .. })
    ));
    let upd = |p: &mut Provider, values: Vec<(&str, Value)>| {
        p.update_data(
            &app,
            "mixed",
            values.into_iter().map(|(k, v)| (k.to_owned(), v)).collect(),
            MS,
        )
    };
    assert!(matches!(
        upd(&mut p, vec![("x", Value::from(1))]),
        Err(ProviderError::Value(_))
    ));
    assert!(matches!(
        upd(&mut p, vec![("i", Value::from(200))]),
        Err(ProviderError::Value(_))
    ));
    assert!(matches!(
        upd(&mut p, vec![("b", Value::from(1))]),
        Err(ProviderError::Value(_))
    ));
    assert!(matches!(
        upd(&mut p, vec![("g", Value::from("not a guid"))]),
        Err(ProviderError::Value(_))
    ));
    // atomic: a refused update changes nothing
    assert!(matches!(
        upd(&mut p, vec![("i", Value::from(1)), ("b", Value::from(2))]),
        Err(ProviderError::Value(_))
    ));
    assert_eq!(p.data_values(&app, "mixed"), None);
    let g = "6F9619FF-8B86-D011-B42D-00C04FC964FF";
    assert_eq!(
        upd(
            &mut p,
            vec![
                ("i", Value::from(-5)),
                ("g", Value::from(g)),
                ("b", Value::from(true))
            ]
        ),
        Ok(true)
    );
    assert_eq!(
        upd(&mut p, vec![("i", Value::from(-5i64))]),
        Ok(false),
        "the same value after the conversion"
    );
    assert_eq!(
        p.data_values(&app, "mixed").unwrap()[1],
        Value::String(g.to_lowercase())
    );
    assert!(matches!(
        p.update_data(&app, "nope", Vec::new(), MS),
        Err(ProviderError::UnknownDataSet { .. })
    ));
    assert!(matches!(
        p.remove_data(&app, "nope"),
        Err(ProviderError::UnknownDataSet { .. })
    ));
    // a reserved or nil class id (REQ-MSG-014)
    let mut d = temp("c", Mode::FILTER);
    d.class_id = Some(Uuid::nil());
    assert!(matches!(
        p.add_data(&app, d, MS),
        Err(ProviderError::DataSet(_))
    ));
}

#[test]
fn metadata_larger_than_the_bus_is_refused() {
    let lifecycle = Lifecycle::new(
        ServiceType::Aggregation,
        il(APP),
        mam(APP),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get: true,
        max_packet_size: 4096,
        publish_limit: 4096,
    };
    let mut p = Provider::new(lifecycle, config, WriterIds::new()).unwrap();
    let mut d = DataSetSpec::new("big");
    d.fields = (0..40)
        .map(|i| FieldSpec::new(format!("field-{i}"), T::Double))
        .collect();
    assert!(matches!(
        p.add_data(&il(APP), d, MS),
        Err(ProviderError::DataSet(_))
    ));
}

#[test]
fn many_data_sets_are_paginated_with_their_class() {
    let lifecycle = Lifecycle::new(
        ServiceType::Aggregation,
        il(APP),
        mam(APP),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get: true,
        max_packet_size: 4096,
        publish_limit: 2048,
    };
    let mut p = Provider::new(lifecycle, config, WriterIds::new()).unwrap();
    let app = il(APP);
    let class = Uuid::parse_str("1badfd0b-5a3c-4e8c-9fed-ea47fe25ce67").unwrap();
    for n in 0..40 {
        let mut d = temp(&format!("t{n}"), Mode::APPLICATION);
        d.class_id = Some(class);
        p.add_data(&app, d, MS).unwrap();
        set(&mut p, &app, &format!("t{n}"), f64::from(n), MS);
    }
    let out = p.on_connected(Duration::ZERO, MS);
    let data: Vec<_> = out
        .iter()
        .filter(|o| o.topic.to_string() == topic("Pub", "Data"))
        .collect();
    assert!(data.len() > 1);
    let mut total = 0;
    for o in &data {
        assert!(o.payload.len() <= 2048);
        let v = json_of(o);
        // one class on every page, the Pagination helper aside
        assert_eq!(v["DataSetClassId"], class.to_string());
        total += v["Messages"].as_array().unwrap().len() - 1;
    }
    assert_eq!(total, 40);
}

#[cfg(feature = "strict-validation")]
mod strict {
    use super::*;
    use oec_core::message::LocalizedText;
    use oec_core::schema::{Flavor, Validator};

    /// A DataSet with a field of every type, all set.
    fn add_all_types(p: &mut Provider, pump: &IdentificationLink) {
        let mut d = DataSetSpec::new("all-types");
        d.description = Some("Every type".to_owned());
        let types = [
            T::Boolean,
            T::SByte,
            T::Byte,
            T::Int16,
            T::UInt16,
            T::Int32,
            T::UInt32,
            T::Int64,
            T::UInt64,
            T::Float,
            T::Double,
            T::String,
            T::DateTime,
            T::Guid,
            T::ByteString,
            T::LocalizedText,
        ];
        d.fields = types
            .iter()
            .enumerate()
            .map(|(i, t)| {
                let mut f = FieldSpec::new(format!("f{i}"), *t);
                if t.is_numeric() {
                    f.unit = Some("m".to_owned());
                    f.range = Some((-1.5, 100.0));
                }
                f
            })
            .collect();
        d.publication.mode = Mode::ON_REQUEST | Mode::APPLICATION | Mode::SOURCE | Mode::FILTER;
        d.publication.precisions.insert("f10".to_owned(), 0.25);
        d.publication.writable = Writable::Interval;
        p.add_data(pump, d, MS).unwrap();
        let values: Vec<Value> = vec![
            true.into(),
            (-8i8).into(),
            8u8.into(),
            (-16i16).into(),
            16u16.into(),
            (-32i32).into(),
            32u32.into(),
            i64::MIN.into(),
            u64::MAX.into(),
            1.1f32.into(),
            f64::NAN.into(),
            "s".into(),
            Value::DateTime(MS.try_into().unwrap()),
            "6f9619ff-8b86-d011-b42d-00c04fc964ff".into(),
            vec![0u8, 1, 2].into(),
            LocalizedText::new("de-DE", "Hallo").into(),
        ];
        let changes = values
            .into_iter()
            .enumerate()
            .map(|(i, v)| (format!("f{i}"), v))
            .collect();
        p.update_data(pump, "all-types", changes, MS).unwrap();
    }

    #[test]
    fn generated_data_payloads_pass_the_strict_schema() {
        let validator = Validator::new().unwrap();
        let mut p = provider(true);
        let app = il(APP);
        let pump = p.add_asset(mam(PUMP)).unwrap();
        add_all_types(&mut p, &pump);
        let mut pv = DataSetSpec::new(OI4PV);
        pv.fields = vec![
            FieldSpec::new("Pv", T::Double),
            FieldSpec::new("Sv1", T::Float),
        ];
        pv.publication.mode = Mode::ON_REQUEST | Mode::SOURCE;
        p.add_data(&app, pv, MS).unwrap();
        p.update_data(
            &app,
            OI4PV,
            vec![
                ("Pv".to_owned(), 1.5.into()),
                ("Sv1".to_owned(), 2.5f32.into()),
            ],
            MS,
        )
        .unwrap();
        let mut all = p.on_connected(Duration::ZERO, MS);
        let s = pump.encode();
        for (n, rest) in [
            "Data".to_owned(),
            format!("Data/{s}"),
            format!("Data/{s}/all-types"),
            "PublicationList".to_owned(),
            format!("PublicationList/{s}/Data/all-types"),
            format!("Profile/{s}"),
        ]
        .iter()
        .enumerate()
        {
            all.extend(
                p.handle_message(&topic("Get", rest), &get(&json!([])), MS + n as u64 + 1)
                    .unwrap(),
            );
        }
        for (src, tag) in [(PUMP, "all-types"), (APP, OI4PV)] {
            let enc = il(src).encode();
            all.extend(
                p.handle_message(
                    &topic("Get", &format!("Metadata/{enc}/{tag}")),
                    &get_meta(src, tag),
                    MS + 9,
                )
                .unwrap(),
            );
        }
        assert!(all.len() >= 16, "{:?}", topics(&all));
        for o in all {
            validator
                .validate_topic(&o.topic, Flavor::Strict, &json_of(&o))
                .unwrap_or_else(|e| panic!("{}: {e}\n{}", o.topic, json_of(&o)));
        }
    }
}
