//! The Syslog and Generic events (P4 increment 7c, ADR-0026 §16):
//! published while connected on `Pub/Event/<Source>/<Category>/<Level>`
//! (REQ-RES-EVENT-007 … 009, 011), with their PublicationList entries
//! (REQ-RES-PUBLIST-012), the Syslog levels switched by
//! `Set PublicationList` and persisted (REQ-TOP-024, REQ-TOP-027). The
//! guideline vectors V-PAY-064/065 are compared with the published events.

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{
    GenericEvent, MAX_GENERIC_LEVELS, MAX_QUEUED_EVENTS, Provider, ProviderConfig, ProviderError,
    PublicationOverrides, WriterIds, syslog_header,
};
use oec_core::resource::Mam;
use oec_core::status::StatusCode;
use oec_core::topic::{ServiceType, SyslogLevel};
use serde_json::{Map, Value as Json, json};

const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const MOTOR: &str = "groe.mca/DeviceModelB/AC33.447/20443";
const MS: u64 = 1_639_693_510_094;

fn il(s: &str) -> IdentificationLink {
    IdentificationLink::parse(s).unwrap()
}

fn mam(piu: &str) -> Mam {
    serde_json::from_value(json!({
        "Manufacturer": {"Locale": "en-US", "Text": "Provider"},
        "ManufacturerUri": "provider.com",
        "Model": {"Locale": "en-US", "Text": "Field Data Service"},
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

fn provider_of(get: bool, overrides: PublicationOverrides) -> Provider {
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
    Provider::new(lifecycle, config, WriterIds::new())
        .unwrap()
        .with_publication_overrides(overrides)
        .with_syslog(SyslogLevel::Notice)
        .unwrap()
}

/// Level 2 with the asset MOTOR, connected.
fn provider() -> Provider {
    let mut p = provider_of(true, PublicationOverrides::new());
    p.add_asset(mam(MOTOR)).unwrap();
    p.on_connected(Duration::ZERO, MS);
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

fn event_topic(source: &str, filter: &str) -> String {
    topic("Pub", &format!("Event/{}/{filter}", il(source).encode()))
}

fn vector(id: &str) -> Json {
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
    serde_json::from_str(v["payload"].as_str().unwrap()).unwrap()
}

fn poll(p: &mut Provider) -> Vec<Publication> {
    p.poll(Duration::from_millis(1), MS + 1)
}

/// A DataSetMessage without the fields an implementation chooses and the
/// optional MessageType (`ua-event`, REQ-MSG-029), which the vectors omit.
fn comparable(m: &Json) -> Json {
    let mut m = m.clone();
    for k in ["DataSetWriterId", "SequenceNumber", "MessageType"] {
        m.as_object_mut().unwrap().remove(k);
    }
    m
}

/// The PublicationList entries of `source` for Event.
fn event_entries(p: &mut Provider, source: &str) -> Vec<Json> {
    let get = json!({
        "MessageId": "1-ITConnector/consumer.com/Client/C-1/1",
        "MessageType": "ua-data",
        "PublisherId": "ITConnector/consumer.com/Client/C-1/1",
        "ReplyTo": "",
        "Messages": []
    });
    let t = topic(
        "Get",
        &format!("PublicationList/{}/Event", il(source).encode()),
    );
    let out = p
        .handle_message(&t, &serde_json::to_vec(&get).unwrap(), MS)
        .unwrap()
        .into_publications()
        .unwrap();
    json_of(&out[0])["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| m["Payload"].clone())
        .filter(|e| !e["Filter"].as_str().unwrap().starts_with("Status/"))
        .collect()
}

#[test]
fn syslog_event_has_the_shape_of_v_pay_064() {
    let mut p = provider();
    let expected = vector("V-PAY-064");
    let want = &expected["Messages"][0];
    let msg = want["Payload"]["Details"]["MSG"].as_str().unwrap();
    let header = syslog_header(MS, "FDS-001-1200-0345");
    assert_eq!(header, want["Payload"]["Details"]["HEADER"]);
    assert_eq!(
        p.syslog_identity(),
        (
            "FDS-001-1200-0345".to_owned(),
            "FieldDataService".to_owned()
        )
    );
    assert!(p.emit_syslog(SyslogLevel::Notice, msg, &header, MS));
    let out = poll(&mut p);
    assert_eq!(topics(&out), [event_topic(APP, "Syslog/Notice")]);
    let v = json_of(&out[0]);
    assert_eq!(v["DataSetClassId"], expected["DataSetClassId"]);
    assert_eq!(comparable(&v["Messages"][0]), comparable(want));
    assert_eq!(poll(&mut p), []);
    // PRI = 8 + severity (Facility 1, user-level)
    for (level, pri) in [(SyslogLevel::Emergency, 8), (SyslogLevel::Error, 11)] {
        assert!(p.emit_syslog(level, "m", &header, MS));
        let out = poll(&mut p);
        assert_eq!(json_of(&out[0])["Messages"][0]["Payload"]["Number"], pri);
    }
}

#[test]
fn generic_event_has_the_shape_of_v_pay_065() {
    let mut p = provider();
    let expected = vector("V-PAY-065");
    let want = &expected["Messages"][0];
    let details: Map<String, Json> =
        serde_json::from_value(want["Payload"]["Details"].clone()).unwrap();
    let event = GenericEvent {
        number: 4711,
        description: Some(want["Payload"]["Description"].as_str().unwrap().to_owned()),
        details: Some(details),
    };
    assert_eq!(
        p.emit_generic(&il(APP), "High", event.clone(), MS),
        Ok(true)
    );
    let out = poll(&mut p);
    // the new level: the PublicationList first, then the event
    assert_eq!(
        topics(&out),
        [
            topic("Pub", "PublicationList"),
            event_topic(APP, "Generic/High")
        ]
    );
    let v = json_of(&out[1]);
    assert_eq!(comparable(&v["Messages"][0]), comparable(want));
    // a known level: only the event, the same DataSetWriterId
    assert_eq!(
        p.emit_generic(&il(APP), "High", event.clone(), MS),
        Ok(true)
    );
    let again = poll(&mut p);
    assert_eq!(topics(&again), [event_topic(APP, "Generic/High")]);
    let id = |o: &Publication| json_of(o)["Messages"][0]["DataSetWriterId"].clone();
    assert_eq!(id(&again[0]), id(&out[1]));
    // an asset, a level with an encoded character
    let bare = GenericEvent {
        number: 1,
        description: None,
        details: None,
    };
    assert_eq!(
        p.emit_generic(&il(MOTOR), "a/b", bare.clone(), MS),
        Ok(true)
    );
    let out = poll(&mut p);
    assert_eq!(topics(&out)[1], event_topic(MOTOR, "Generic/a%2Fb"));
    let m = &json_of(&out[1])["Messages"][0];
    assert_eq!(m["WriterGroupName"], "Generic/a%2Fb");
    assert_eq!(
        m["Payload"],
        json!({"Number": 1, "Category": "CAT_GENERIC_99"})
    );
    // the entries
    let entries = event_entries(&mut p, MOTOR);
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0]["Mode"], "FILTER_4");
    assert_eq!(entries[0].get("Config"), None);
    assert_eq!(entries[0]["DataSetWriterId"], m["DataSetWriterId"]);
}

#[test]
fn generic_events_are_checked() {
    let mut p = provider();
    let e = || GenericEvent {
        number: 1,
        description: None,
        details: None,
    };
    let other = il("groe.mca/Other/O-1/1");
    assert!(matches!(
        p.emit_generic(&other, "x", e(), MS),
        Err(ProviderError::UnknownAsset(_))
    ));
    for bad in ["", "a\0b"] {
        assert!(
            matches!(
                p.emit_generic(&il(APP), bad, e(), MS),
                Err(ProviderError::Topic(_))
            ),
            "{bad:?}"
        );
    }
    for i in 0..MAX_GENERIC_LEVELS {
        p.emit_generic(&il(MOTOR), &format!("L{i}"), e(), MS)
            .unwrap();
    }
    assert!(matches!(
        p.emit_generic(&il(MOTOR), "one-more", e(), MS),
        Err(ProviderError::Event(_))
    ));
    // a known level is fine; the queue is bounded
    poll(&mut p);
    for _ in 0..MAX_QUEUED_EVENTS {
        assert_eq!(p.emit_generic(&il(MOTOR), "L0", e(), MS), Ok(true));
    }
    assert_eq!(p.emit_generic(&il(MOTOR), "L0", e(), MS), Ok(false));
    assert_eq!(poll(&mut p).len(), MAX_QUEUED_EVENTS);
    // removed: the levels and queued events of the asset are gone
    p.emit_generic(&il(MOTOR), "L0", e(), MS).unwrap();
    p.remove_asset(&il(MOTOR)).unwrap();
    assert_eq!(topics(&poll(&mut p)), [topic("Pub", "PublicationList")]);
    p.add_asset(mam(MOTOR)).unwrap();
    poll(&mut p);
    assert_eq!(event_entries(&mut p, MOTOR), Vec::<Json>::new());
}

#[test]
fn nothing_is_queued_offline() {
    let mut p = provider_of(true, PublicationOverrides::new());
    let e = GenericEvent {
        number: 1,
        description: None,
        details: None,
    };
    assert!(!p.emit_syslog(SyslogLevel::Error, "m", "h", MS));
    // the level is registered all the same
    assert_eq!(p.emit_generic(&il(APP), "x", e.clone(), MS), Ok(false));
    let out = p.on_connected(Duration::ZERO, MS);
    assert!(!topics(&out).iter().any(|t| t.contains("/Pub/Event/")));
    assert_eq!(event_entries(&mut p, APP).len(), 8 + 1);
    // queued, then the connection is lost: dropped
    assert!(p.emit_syslog(SyslogLevel::Error, "m", "h", MS));
    p.on_disconnected();
    let out = p.on_connected(Duration::ZERO, MS);
    assert!(!topics(&out).iter().any(|t| t.contains("/Pub/Event/")));
    // Level 1: published, no PublicationList
    let mut l1 = provider_of(false, PublicationOverrides::new());
    l1.on_connected(Duration::ZERO, MS);
    assert!(l1.emit_syslog(SyslogLevel::Error, "m", "h", MS));
    assert_eq!(l1.emit_generic(&il(APP), "x", e, MS), Ok(true));
    assert_eq!(
        topics(&poll(&mut l1)),
        [
            event_topic(APP, "Syslog/Error"),
            event_topic(APP, "Generic/x")
        ]
    );
}

fn set_level(p: &mut Provider, level: &str, mode: &str) -> Vec<Publication> {
    let rest = format!("PublicationList/{}/Event/Syslog/{level}", il(APP).encode());
    let mut entry = event_entries(p, APP)
        .into_iter()
        .find(|e| e["Filter"] == format!("Syslog/{level}"))
        .unwrap();
    entry["Mode"] = json!(mode);
    let set = json!({
        "MessageId": "2-ITConnector/consumer.com/Client/C-1/1",
        "MessageType": "ua-data",
        "PublisherId": "ITConnector/consumer.com/Client/C-1/1",
        "ReplyTo": "",
        "Messages": [{"DataSetWriterId": 20, "DataSetWriterName": APP, "Payload": entry}]
    });
    p.handle_message(&topic("Set", &rest), &serde_json::to_vec(&set).unwrap(), MS)
        .unwrap()
        .into_publications()
        .unwrap()
}

fn status_number(out: &[Publication]) -> Json {
    json_of(out.last().unwrap())["Messages"][0]["Payload"]["Number"].clone()
}

#[test]
fn syslog_levels_are_switched_and_persisted() {
    let mut p = provider();
    // the defaults: down to Notice on, Informational and Debug off
    let modes: Vec<_> = event_entries(&mut p, APP)
        .iter()
        .map(|e| (e["Filter"].clone(), e["Mode"].clone(), e["Config"].clone()))
        .collect();
    let want: Vec<_> = SyslogLevel::ALL
        .iter()
        .map(|l| {
            let on = *l <= SyslogLevel::Notice;
            (
                json!(format!("Syslog/{l}")),
                json!(if on { "FILTER_4" } else { "OFF_0" }),
                json!("MODE_1"),
            )
        })
        .collect();
    assert_eq!(modes, want);
    assert_eq!(p.syslog_max_level(), Some(SyslogLevel::Notice));
    assert!(!p.emit_syslog(SyslogLevel::Debug, "m", "h", MS));
    assert!(event_entries(&mut p, MOTOR).is_empty());
    // Debug on, Error off
    let out = set_level(&mut p, "Debug", "FILTER_4");
    assert_eq!(status_number(&out), json!(StatusCode::GOOD.0));
    assert_eq!(
        json_of(&out[0])["Messages"][0]["Payload"]["Mode"],
        "FILTER_4"
    );
    assert_eq!(p.syslog_max_level(), Some(SyslogLevel::Debug));
    assert!(p.emit_syslog(SyslogLevel::Debug, "m", "h", MS));
    set_level(&mut p, "Error", "OFF_0");
    assert!(!p.emit_syslog(SyslogLevel::Error, "m", "h", MS));
    // only FILTER_4 and OFF_0
    let out = set_level(&mut p, "Error", "ON_REQUEST_FILTER_9");
    assert_eq!(
        status_number(&out),
        json!(StatusCode::BAD_INVALID_ARGUMENT.0)
    );
    // persisted, applied after a restart
    let saved = p.take_publication_overrides().unwrap();
    let mut q = provider_of(true, PublicationOverrides::from_json(&saved).unwrap());
    assert!(q.syslog_enabled(SyslogLevel::Debug));
    assert!(!q.syslog_enabled(SyslogLevel::Error));
    assert!(q.syslog_enabled(SyslogLevel::Warning));
    q.on_connected(Duration::ZERO, MS);
    assert_eq!(event_entries(&mut q, APP)[3]["Mode"], "OFF_0");
}
