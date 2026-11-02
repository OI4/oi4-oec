//! Listed assets of the OEC Registry (MG A1, REQ-REG-002, ADR-0026
//! amendment C-086 §22): the Registry answers `Get MAM` and `Get Health`
//! with and without Source also for the available assets of others, with
//! its own PublisherId and DataSetWriterIds and the asset's Identification
//! Link as DataSetWriterName. Listed assets are no represented assets: no
//! own publication, no PublicationList entry, no Profile, no other
//! resource (unknown Source, REQ-CMP-047).

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{Ignored, Provider, ProviderConfig, ProviderError, WriterIds};
use oec_core::resource::{Health, HealthState, Mam};
use oec_core::topic::ServiceType;
use serde_json::{Value, json};

const REG: &str = "oi4.org/OEC,20Registry/OEC-REGISTRY/reg-1";
const APP: &str = "vendor.com/Gateway/GW-1/SN-1";
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const MS: u64 = 1_700_000_000_000;
const GET_ID: &str = "1700000000000-Utility/vendor.com/Client/C-1/1";

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

fn health(state: HealthState, score: u8) -> Health {
    Health {
        health: state,
        health_score: Some(score),
        details: None,
    }
}

fn provider(st: ServiceType, get: bool) -> Provider {
    let lifecycle = Lifecycle::new(
        st,
        il(REG),
        mam(REG),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get,
        max_packet_size: 262_144,
        publish_limit: 262_144,
    };
    Provider::new(lifecycle, config, WriterIds::new()).unwrap()
}

fn registry() -> Provider {
    let mut p = provider(ServiceType::Registry, true);
    p.on_connected(Duration::ZERO, MS);
    p
}

fn get_payload() -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": GET_ID,
        "MessageType": "ua-data",
        "PublisherId": "Utility/vendor.com/Client/C-1/1",
        "DataSetClassId": "",
        "ReplyTo": "",
        "Messages": []
    }))
    .unwrap()
}

fn topic(method: &str, rest: &str) -> String {
    format!("Oi4v2/Registry/{}/{method}/{rest}", il(REG).encode())
}

fn get(p: &mut Provider, rest: &str) -> Vec<Publication> {
    p.handle_message(&topic("Get", rest), &get_payload(), MS + 5)
        .unwrap()
        .into_publications()
        .expect("answered by publications")
}

fn json_of(p: &Publication) -> Value {
    serde_json::from_slice(&p.payload).unwrap()
}

/// (DataSetWriterName, DataSetWriterId) of the DataSetMessages.
fn writers(v: &Value) -> Vec<(String, u64)> {
    v["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| {
            (
                m["DataSetWriterName"].as_str().unwrap().to_owned(),
                m["DataSetWriterId"].as_u64().unwrap(),
            )
        })
        .collect()
}

#[test]
fn get_without_source_lists_the_available_assets_of_others() {
    let mut p = registry();
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    p.list_asset(
        il(APP),
        mam(APP),
        health(HealthState::MaintenanceRequired, 70),
    )
    .unwrap();
    for (resource, own) in [("MAM", 10), ("Health", 11)] {
        let out = get(&mut p, resource);
        assert_eq!(out.len(), 1, "{resource}: one NetworkMessage");
        assert_eq!(out[0].topic.to_string(), topic("Pub", resource));
        let v = json_of(&out[0]);
        assert_eq!(
            v["PublisherId"],
            format!("Registry/{REG}"),
            "own PublisherId (A1)"
        );
        assert_eq!(v["ReplyTo"], GET_ID);
        let w = writers(&v);
        // the registry itself first, then the listed assets sorted by IL
        let names: Vec<&str> = w.iter().map(|(n, _)| n.as_str()).collect();
        assert_eq!(names, [REG, APP, PUMP], "{resource}");
        assert_eq!(w[0].1, own);
        // the registry's own, distinct DataSetWriterIds
        let mut ids: Vec<u64> = w.iter().map(|(_, id)| *id).collect();
        ids.sort_unstable();
        ids.dedup();
        assert_eq!(ids.len(), 3, "{resource}: {w:?}");
    }
    let v = json_of(&get(&mut p, "Health")[0]);
    assert_eq!(
        v["Messages"][1]["Payload"]["Health"],
        "MAINTENANCE_REQUIRED_4"
    );
    assert_eq!(v["Messages"][1]["Payload"]["HealthScore"], 70);
    let v = json_of(&get(&mut p, "MAM")[0]);
    assert_eq!(v["Messages"][2]["Payload"]["ProductInstanceUri"], PUMP);
}

#[test]
fn get_with_the_source_of_a_listed_asset() {
    let mut p = registry();
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    for resource in ["MAM", "Health"] {
        let rest = format!("{resource}/{}", il(PUMP).encode());
        let out = get(&mut p, &rest);
        assert_eq!(out.len(), 1);
        assert_eq!(out[0].topic.to_string(), topic("Pub", &rest));
        let w = writers(&json_of(&out[0]));
        assert_eq!(w.len(), 1);
        assert_eq!(w[0].0, PUMP);
    }
    // the DataSetWriterIds stay the same for every answer and after a
    // replacement of the payloads
    let before = writers(&json_of(&get(&mut p, "MAM")[0]));
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Failure, 0))
        .unwrap();
    let after = writers(&json_of(&get(&mut p, "MAM")[0]));
    assert_eq!(before, after);
    let v = json_of(&get(&mut p, &format!("Health/{}", il(PUMP).encode()))[0]);
    assert_eq!(v["Messages"][0]["Payload"]["Health"], "FAILURE_1");
}

#[test]
fn listed_assets_are_no_represented_assets() {
    let mut p = registry();
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    // never published on its own initiative: no MAM, Health, Profile or
    // PublicationList change
    let out = p.poll(Duration::from_secs(120), MS + 120_000);
    assert!(
        out.iter()
            .all(|o| !o.topic.to_string().contains(&il(PUMP).encode())),
        "{:?}",
        out.iter().map(|o| o.topic.to_string()).collect::<Vec<_>>()
    );
    assert!(p.assets().next().is_none(), "not a represented asset");
    // no Profile, PublicationList entry or other resource
    let v = json_of(&get(&mut p, "PublicationList")[0]);
    assert!(!v.to_string().contains(PUMP), "{v}");
    let v = json_of(&get(&mut p, "Profile")[0]);
    assert!(!v.to_string().contains(PUMP), "{v}");
    for rest in [
        format!("Profile/{}", il(PUMP).encode()),
        format!("Config/{}", il(PUMP).encode()),
    ] {
        let out = get(&mut p, &rest);
        assert_eq!(out.len(), 1, "{rest}");
        let t = out[0].topic.to_string();
        assert_eq!(
            t,
            topic("Pub", &format!("Event/{}/Status/Bad", il(REG).encode())),
            "{rest}: unknown Source (REQ-CMP-047)"
        );
        let v = json_of(&out[0]);
        assert_eq!(v["ReplyTo"], GET_ID);
    }
}

#[test]
fn unlisted_assets_are_unknown_again() {
    let mut p = registry();
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    assert_eq!(p.listed().cloned().collect::<Vec<_>>(), [il(PUMP)]);
    assert!(p.unlist_asset(&il(PUMP)));
    assert!(!p.unlist_asset(&il(PUMP)));
    assert!(p.listed().next().is_none());
    let v = json_of(&get(&mut p, "MAM")[0]);
    assert_eq!(writers(&v).len(), 1, "only the registry itself");
    let out = get(&mut p, &format!("MAM/{}", il(PUMP).encode()));
    assert!(
        out[0].topic.to_string().contains("/Pub/Event/"),
        "unknown Source"
    );
}

#[test]
fn listing_is_refused_outside_a_registry_and_for_own_assets() {
    let mut p = provider(ServiceType::Utility, true);
    assert_eq!(
        p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100)),
        Err(ProviderError::NotRegistry)
    );
    let mut p = registry();
    assert_eq!(
        p.list_asset(il(REG), mam(REG), health(HealthState::Normal, 100)),
        Err(ProviderError::SelfAsset)
    );
    p.add_asset(mam(APP)).unwrap();
    assert_eq!(
        p.list_asset(il(APP), mam(APP), health(HealthState::Normal, 100)),
        Err(ProviderError::DuplicateAsset(APP.to_owned()))
    );
    // a listed asset can not become a represented asset either
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    assert_eq!(
        p.add_asset(mam(PUMP)),
        Err(ProviderError::DuplicateAsset(PUMP.to_owned()))
    );
    assert!(
        ProviderError::NotRegistry
            .to_string()
            .contains("ServiceType Registry")
    );
}

#[test]
fn level_1_lists_but_serves_no_get() {
    // Get is served from Level 2 on (ADR-0020 §3); the registry runs at L3.
    let mut p = provider(ServiceType::Registry, false);
    p.on_connected(Duration::ZERO, MS);
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    let r = p.handle_message(&topic("Get", "MAM"), &get_payload(), MS);
    assert!(matches!(r, Err(Ignored::NotServed(_))), "{r:?}");
}

#[test]
fn the_writer_ids_of_listed_assets_are_saved() {
    let mut p = registry();
    let _ = p.take_writer_ids();
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 100))
        .unwrap();
    let saved = p.take_writer_ids().expect("new DataSetWriterIds to save");
    let text = String::from_utf8(saved).unwrap();
    assert!(text.contains(PUMP), "{text}");
    // a replacement assigns nothing new
    p.list_asset(il(PUMP), mam(PUMP), health(HealthState::Normal, 90))
        .unwrap();
    assert!(p.take_writer_ids().is_none());
}
