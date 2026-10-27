//! The provider side (MG 9.1, P4 increment 6a): sources, Get answers with
//! ReplyTo (REQ-RES-PUBLIST-027, REQ-MSG-015), Profile and PublicationList
//! (REQ-RES-PROFILE-*, REQ-RES-PUBLIST-*), stable DataSetWriterIds and
//! pagination (REQ-PAG-*); with `strict-validation` every generated payload
//! passes the embedded schema.

use std::collections::BTreeMap;
use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::{DataSetWriterId, PaginationRequest};
use oec_core::provider::pagination::plan;
use oec_core::provider::{Ignored, Provider, ProviderConfig, ProviderError, WriterIds, WriterKey};
use oec_core::resource::{Health, HealthState, Mam};
use oec_core::topic::{Resource, ServiceType};
use proptest::prelude::*;
use serde_json::{Value, json};

/// `handle_message` for requests answered by publications.
trait HandleGet {
    fn handle_get(
        &mut self,
        topic: &str,
        payload: &[u8],
        ms: u64,
    ) -> Result<Vec<Publication>, Ignored>;
}

impl HandleGet for Provider {
    fn handle_get(
        &mut self,
        topic: &str,
        payload: &[u8],
        ms: u64,
    ) -> Result<Vec<Publication>, Ignored> {
        self.handle_message(topic, payload, ms)
            .map(|h| h.into_publications().expect("answered by publications"))
    }
}

const APP: &str = "vendor.com/Gateway/GW-1/SN-1";
const MS: u64 = 1_700_000_000_000;

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

fn provider(get: bool, writers: WriterIds) -> Provider {
    let lifecycle = Lifecycle::new(
        ServiceType::Aggregation,
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
    Provider::new(lifecycle, config, writers).unwrap()
}

fn json_of(p: &Publication) -> Value {
    serde_json::from_slice(&p.payload).unwrap()
}

fn topics(ps: &[Publication]) -> Vec<String> {
    ps.iter().map(|p| p.topic.to_string()).collect()
}

fn get(id: &str, messages: &Value) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": id,
        "MessageType": "ua-data",
        "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
        "DataSetClassId": "",
        "ReplyTo": "",
        "Messages": messages
    }))
    .unwrap()
}

const GET_ID: &str = "1700000000000-Aggregation/vendor.com/Client/C-1/1";

fn topic(method: &str, rest: &str) -> String {
    format!("Oi4v2/Aggregation/{}/{method}/{rest}", il(APP).encode())
}

const PUMP: &str = "vendor.com/Pump/P-1/0815";

#[test]
fn level_1_serves_no_get() {
    let mut p = provider(false, WriterIds::new());
    assert!(p.subscriptions().is_empty());
    let out = p.on_connected(Duration::ZERO, MS);
    assert_eq!(
        topics(&out),
        [
            topic("Pub", &format!("MAM/{}", il(APP).encode())),
            topic("Pub", &format!("Health/{}", il(APP).encode()))
        ]
    );
    let r = p.handle_get(&topic("Get", "MAM"), &get(GET_ID, &json!([])), MS);
    assert!(matches!(r, Err(Ignored::NotServed(_))), "{r:?}");
    // assets still publish MAM and Health (REQ-CMP-007)
    p.add_asset(mam(PUMP)).unwrap();
    let out = p.poll(Duration::from_millis(1), MS + 1);
    assert_eq!(
        topics(&out),
        [
            topic("Pub", &format!("MAM/{}", il(PUMP).encode())),
            topic("Pub", &format!("Health/{}", il(PUMP).encode()))
        ]
    );
}

/// `Get` of the served resources, `Set` and `Del` of the resources they
/// address, to this application.
fn assert_subscriptions(p: &Provider) {
    assert_eq!(
        p.subscriptions()
            .iter()
            .map(ToString::to_string)
            .collect::<Vec<_>>(),
        [
            "Get/MAM",
            "Get/Health",
            "Get/Config",
            "Get/Data",
            "Get/Metadata",
            "Get/Profile",
            "Get/PublicationList",
            "Set/Data",
            "Set/Metadata",
            "Set/Config",
            "Set/PublicationList",
            "Set/SubscriptionList",
            "Set/ReferenceDesignation",
            "Del/SubscriptionList",
            "Del/ReferenceDesignation",
        ]
        .map(|r| {
            let (method, resource) = r.split_once('/').unwrap();
            topic(method, &format!("{resource}/#"))
        })
    );
}

#[test]
fn connect_publishes_the_state_of_every_source() {
    let mut p = provider(true, WriterIds::new());
    assert_subscriptions(&p);
    // offline: nothing is published, nothing queued (ADR-0020 §7)
    let pump = p.add_asset(mam(PUMP)).unwrap();
    let off_spec = Health {
        health: HealthState::OffSpec,
        health_score: Some(50),
        details: None,
    };
    assert_eq!(p.set_health(&pump, off_spec), Ok(true));
    assert_eq!(p.poll(Duration::ZERO, MS), []);
    let out = p.on_connected(Duration::ZERO, MS);
    let (app, asset) = (il(APP).encode(), pump.encode());
    assert_eq!(
        topics(&out),
        [
            topic("Pub", &format!("MAM/{app}")),
            topic("Pub", &format!("Health/{app}")),
            topic("Pub", &format!("MAM/{asset}")),
            topic("Pub", &format!("Health/{asset}")),
            topic("Pub", &format!("Profile/{asset}")),
            topic("Pub", &format!("Profile/{app}")),
            topic("Pub", "PublicationList"),
        ]
    );
    assert_eq!(p.poll(Duration::from_millis(1), MS + 1), []);
    // every DataSetMessage uses the id its PublicationList entry names
    // (REQ-RES-PUBLIST-005), the PublicationList one id (T7-MON-08)
    let list = json_of(&out[6]);
    let entries = list["Messages"].as_array().unwrap();
    assert_eq!(entries.len(), 12);
    let mut named = BTreeMap::new();
    for e in entries {
        let pl = &e["Payload"];
        assert_eq!(e["DataSetWriterId"], entries[0]["DataSetWriterId"]);
        assert_eq!(e["DataSetWriterName"], pl["Source"]);
        if pl["Resource"] == "Event" {
            // the Status events: only on their topic (REQ-RES-EVENT-005)
            let filter = pl["Filter"].as_str().unwrap();
            assert!(filter.starts_with("Status/"), "{filter}");
            assert_eq!(e["WriterGroupName"], format!("Event/{filter}"));
            assert_eq!(pl["Mode"], "FILTER_4");
            continue;
        }
        assert_eq!(e["WriterGroupName"], pl["Resource"]);
        assert_eq!(pl["Mode"], "ON_REQUEST_SOURCE_10");
        named.insert(
            (
                pl["Resource"].as_str().unwrap().to_owned(),
                pl["Source"].as_str().unwrap().to_owned(),
            ),
            pl["DataSetWriterId"].clone(),
        );
    }
    for pubn in &out[..6] {
        let msg = &json_of(pubn)["Messages"][0];
        let key = (
            pubn.topic.subject().to_string(),
            msg["DataSetWriterName"].as_str().unwrap().to_owned(),
        );
        assert_eq!(named[&key], msg["DataSetWriterId"], "{key:?}");
    }
    // the app keeps MAM 10 and Health 11
    assert_eq!(named[&("MAM".to_owned(), APP.to_owned())], 10);
    assert_eq!(named[&("Health".to_owned(), APP.to_owned())], 11);
    // Profile: what is served (REQ-RES-PROFILE-002)
    assert_eq!(
        json_of(&out[5])["Messages"][0]["Payload"]["Resources"],
        json!([
            "MAM",
            "Health",
            "Config",
            "Data",
            "Metadata",
            "Event",
            "Profile",
            "PublicationList"
        ])
    );
    // the Health set while offline
    assert_eq!(
        json_of(&out[3])["Messages"][0]["Payload"]["HealthScore"],
        50
    );
}

#[test]
fn changes_are_published_at_once() {
    let mut p = provider(true, WriterIds::new());
    p.on_connected(Duration::ZERO, MS);
    let pump = p.add_asset(mam(PUMP)).unwrap();
    assert_eq!(p.poll_timeout(), Duration::ZERO);
    let out = p.poll(Duration::from_millis(1), MS + 1);
    assert_eq!(out.len(), 4, "{:?}", topics(&out));
    assert_eq!(out[3].topic.to_string(), topic("Pub", "PublicationList"));
    assert!(p.poll_timeout() > Duration::from_millis(1));

    let failure = Health {
        health: HealthState::Failure,
        health_score: Some(0),
        details: None,
    };
    assert_eq!(p.set_health(&pump, failure.clone()), Ok(true));
    assert_eq!(p.set_health(&pump, failure), Ok(false));
    let out = p.poll(Duration::from_millis(2), MS + 2);
    assert_eq!(
        topics(&out),
        [topic("Pub", &format!("Health/{}", pump.encode()))]
    );

    let mut m = mam(APP);
    m.revision_counter = 1;
    assert_eq!(p.set_mam(&il(APP), m), Ok(true));
    assert_eq!(p.poll_timeout(), Duration::ZERO);
    let out = p.poll(Duration::from_millis(3), MS + 3);
    assert_eq!(
        topics(&out),
        [topic("Pub", &format!("MAM/{}", il(APP).encode()))]
    );
    assert_eq!(
        json_of(&out[0])["Messages"][0]["Payload"]["RevisionCounter"],
        1
    );

    p.remove_asset(&pump).unwrap();
    let out = p.poll(Duration::from_millis(4), MS + 4);
    assert_eq!(topics(&out), [topic("Pub", "PublicationList")]);
    // MAM, Health, Profile and the three Status events of the application
    assert_eq!(json_of(&out[0])["Messages"].as_array().unwrap().len(), 6);
}

#[test]
fn asset_registration_errors() {
    let mut p = provider(true, WriterIds::new());
    assert_eq!(p.add_asset(mam(APP)), Err(ProviderError::SelfAsset));
    p.add_asset(mam(PUMP)).unwrap();
    assert!(matches!(
        p.add_asset(mam(PUMP)),
        Err(ProviderError::DuplicateAsset(_))
    ));
    assert!(matches!(
        p.add_asset(mam("no il")),
        Err(ProviderError::ProductInstanceUri(_))
    ));
    let other = il("vendor.com/Pump/P-1/0816");
    assert!(matches!(
        p.remove_asset(&other),
        Err(ProviderError::UnknownAsset(_))
    ));
    let bad = Health {
        health: HealthState::Normal,
        health_score: Some(101),
        details: None,
    };
    assert!(matches!(
        p.set_health(&il(PUMP), bad),
        Err(ProviderError::Lifecycle(_))
    ));
    assert!(matches!(
        p.set_mam(&il(PUMP), mam("vendor.com/Pump/P-1/0816")),
        Err(ProviderError::ProductInstanceUri(_))
    ));
}

#[test]
fn writer_ids_are_stable_across_restarts() {
    let mut p = provider(true, WriterIds::new());
    p.add_asset(mam(PUMP)).unwrap();
    let table = p.take_writer_ids().expect("new ids");
    assert_eq!(p.take_writer_ids(), None);
    let loaded = WriterIds::from_json(&table).unwrap();
    let pump_mam = loaded.get(&WriterKey::new(Resource::Mam, PUMP)).unwrap();

    // restart: another asset first, then the pump again
    let mut q = provider(true, loaded);
    assert_eq!(q.take_writer_ids(), None);
    q.add_asset(mam("vendor.com/Pump/P-1/0816")).unwrap();
    q.add_asset(mam(PUMP)).unwrap();
    assert_eq!(
        q.writer_ids().get(&WriterKey::new(Resource::Mam, PUMP)),
        Some(pump_mam)
    );
    // removed assets keep their ids
    q.remove_asset(&il(PUMP)).unwrap();
    q.add_asset(mam("vendor.com/Pump/P-1/0817")).unwrap();
    let ids: Vec<u16> = [PUMP, "vendor.com/Pump/P-1/0816", "vendor.com/Pump/P-1/0817"]
        .iter()
        .map(|s| {
            q.writer_ids()
                .get(&WriterKey::new(Resource::Mam, s))
                .unwrap()
                .get()
        })
        .collect();
    assert!(
        ids[0] != ids[1] && ids[1] != ids[2] && ids[0] != ids[2],
        "{ids:?}"
    );
}

#[test]
fn get_is_answered_with_reply_to() {
    let mut p = provider(true, WriterIds::new());
    let pump = p.add_asset(mam(PUMP)).unwrap();
    p.on_connected(Duration::ZERO, MS);

    // without Source: all sources, one NetworkMessage, Pub/<Resource>
    for res in ["MAM", "Health", "Profile"] {
        let out = p
            .handle_get(&topic("Get", res), &get(GET_ID, &json!([])), MS + 1)
            .unwrap();
        assert_eq!(topics(&out), [topic("Pub", res)]);
        let v = json_of(&out[0]);
        assert_eq!(v["ReplyTo"], GET_ID);
        let names: Vec<_> = v["Messages"]
            .as_array()
            .unwrap()
            .iter()
            .map(|m| m["DataSetWriterName"].as_str().unwrap().to_owned())
            .collect();
        assert_eq!(names, [APP, PUMP], "{res}");
    }
    // with Source: that source on Pub/<Resource>/<Source>
    let rest = format!("MAM/{}", pump.encode());
    let out = p
        .handle_get(&topic("Get", &rest), &get(GET_ID, &json!([])), MS + 2)
        .unwrap();
    assert_eq!(topics(&out), [topic("Pub", &rest)]);
    assert_eq!(
        json_of(&out[0])["Messages"][0]["Payload"]["ProductInstanceUri"],
        PUMP
    );

    // PublicationList with Source and Filter: the addressed entry
    let rest = format!("PublicationList/{}/Health", pump.encode());
    let out = p
        .handle_get(&topic("Get", &rest), &get(GET_ID, &json!([])), MS + 3)
        .unwrap();
    assert_eq!(topics(&out), [topic("Pub", &rest)]);
    let v = json_of(&out[0]);
    assert_eq!(v["Messages"].as_array().unwrap().len(), 1);
    assert_eq!(v["Messages"][0]["Payload"]["Resource"], "Health");
    assert_eq!(v["Messages"][0]["Payload"]["Source"], PUMP);

    // a Filter without content: Messages [] (REQ-CMP-041)
    let rest = format!("PublicationList/{}/Data/temp", pump.encode());
    let out = p
        .handle_get(&topic("Get", &rest), &get(GET_ID, &json!([])), MS + 4)
        .unwrap();
    assert_eq!(json_of(&out[0])["Messages"], json!([]));

    // an unknown Source: no Pub, the Status event BadNotFound under the
    // AppId (REQ-CMP-047, SF-108)
    let other = il("vendor.com/Pump/P-1/0816");
    let rest = format!("MAM/{}", other.encode());
    let out = p
        .handle_get(&topic("Get", &rest), &get(GET_ID, &json!([])), MS + 5)
        .unwrap();
    assert_eq!(
        topics(&out),
        [topic(
            "Pub",
            &format!("Event/{}/Status/Bad", il(APP).encode())
        )]
    );
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], GET_ID);
    assert_eq!(v["Messages"][0]["DataSetWriterName"], APP);
    assert!(v["Messages"][0]["DataSetWriterId"].as_u64().unwrap() < 100);
    assert_eq!(v["Messages"][0]["Payload"]["Number"], 0x803E_0000_u32);
    assert_eq!(
        v["Messages"][0]["Payload"]["Details"]["SymbolicId"],
        "BadNotFound"
    );

    // not for us, not a Get, bad payload
    let other = format!("Oi4v2/Aggregation/{}/Get/MAM", il(PUMP).encode());
    assert_eq!(
        p.handle_get(&other, &get(GET_ID, &json!([])), MS),
        Err(Ignored::NotForUs)
    );
    assert!(matches!(
        p.handle_get(&topic("Get", "MAM"), b"{}", MS),
        Err(Ignored::Payload(_))
    ));
    assert!(matches!(
        p.handle_get("x/y", b"{}", MS),
        Err(Ignored::Topic(_))
    ));
}

fn pagination(v: &Value) -> &Value {
    let m = &v["Messages"][0];
    assert_eq!(m["DataSetWriterId"], 2, "{v}");
    &m["Payload"]
}

#[test]
fn get_honours_the_pagination_request() {
    let mut p = provider(true, WriterIds::new());
    for n in 0..4 {
        p.add_asset(mam(&format!("vendor.com/Pump/P-1/{n}")))
            .unwrap();
    }
    p.on_connected(Duration::ZERO, MS);
    let req = |per_page: u32, page: u32| {
        get(
            GET_ID,
            &json!([{
                "DataSetWriterId": 1,
                "DataSetWriterName": "vendor.com/Client/C-1/1",
                "WriterGroupName": "PaginationRequest",
                "Payload": {"PerPage": per_page, "Page": page}
            }]),
        )
    };
    // all pages: 5 MAM, 2 per page
    let out = p
        .handle_get(&topic("Get", "MAM"), &req(2, 0), MS + 1)
        .unwrap();
    assert_eq!(out.len(), 3);
    let first = json_of(&out[0]);
    let series = first["MessageId"].clone();
    for (i, o) in out.iter().enumerate() {
        let v = json_of(o);
        let pg = pagination(&v);
        let page = u32::try_from(i).unwrap() + 1;
        assert_eq!(
            (
                pg["TotalCount"].clone(),
                pg["PerPage"].clone(),
                pg["Page"].clone(),
                pg["HasNext"].clone()
            ),
            (json!(5), json!(2), json!(page), json!(page < 3))
        );
        assert_eq!(pg["PaginationId"], series);
        assert_eq!(v["ReplyTo"], GET_ID);
        assert_eq!(
            v["Messages"].as_array().unwrap().len(),
            if page < 3 { 3 } else { 2 }
        );
    }
    // one page: its own MessageId is the PaginationId (REQ-PAG-005)
    let out = p
        .handle_get(&topic("Get", "MAM"), &req(2, 2), MS + 2)
        .unwrap();
    assert_eq!(out.len(), 1);
    let v = json_of(&out[0]);
    assert_eq!(pagination(&v)["PaginationId"], v["MessageId"]);
    assert_eq!(pagination(&v)["Page"], 2);
    // beyond the last page: empty, HasNext false
    let out = p
        .handle_get(&topic("Get", "MAM"), &req(2, 9), MS + 3)
        .unwrap();
    let v = json_of(&out[0]);
    assert_eq!(v["Messages"].as_array().unwrap().len(), 1);
    assert_eq!(pagination(&v)["HasNext"], false);
}

#[test]
fn large_publications_are_paginated() {
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
        max_packet_size: 8192,
        publish_limit: 4096,
    };
    let mut p = Provider::new(lifecycle, config, WriterIds::new()).unwrap();
    for n in 0..30 {
        p.add_asset(mam(&format!("vendor.com/Pump/P-1/{n}")))
            .unwrap();
    }
    let out = p.on_connected(Duration::ZERO, MS);
    let list: Vec<_> = out
        .iter()
        .filter(|o| o.topic.to_string() == topic("Pub", "PublicationList"))
        .collect();
    assert!(list.len() > 1);
    let mut total = 0;
    for o in &list {
        assert!(o.payload.len() <= 4096);
        let v = json_of(o);
        total += v["Messages"].as_array().unwrap().len() - 1;
        assert_eq!(pagination(&v)["TotalCount"], 186);
    }
    // 31 sources with MAM, Health, Profile and three Status events
    assert_eq!(total, 186);
    // a Get answer may use the MaxPacketSize of the bus
    let out = p
        .handle_get(&topic("Get", "MAM"), &get(GET_ID, &json!([])), MS + 1)
        .unwrap();
    assert!(out.len() > 1 && out.iter().all(|o| o.payload.len() <= 8192));
    assert!(out.iter().any(|o| o.payload.len() > 4096));
}

proptest! {
    /// REQ-PAG-004/005/007: every page but the last is full, pages are
    /// consecutive, cover all items once and fit if the items do.
    #[test]
    fn plan_invariants(
        sizes in prop::collection::vec(1usize..400, 0..60),
        request in prop::option::of((1u32..20, 0u32..8)),
        limit in 500usize..3000,
    ) {
        let req = request.map(|(per_page, page)| PaginationRequest { per_page, page });
        let plan = plan(&sizes, req.as_ref(), limit, 100, 180);
        let per_page = plan.per_page as usize;
        if let Some(r) = &req {
            prop_assert!(plan.per_page <= r.per_page);
            prop_assert!(plan.paginated);
        }
        let all_pages = req.as_ref().is_none_or(|r| r.page == 0);
        let single_fit = sizes.iter().all(|s| 180 + s < limit);
        if all_pages {
            let mut next = 0;
            for (i, (page, range)) in plan.pages.iter().enumerate() {
                prop_assert_eq!(*page as usize, i + 1);
                prop_assert_eq!(range.start, next);
                next = range.end;
                if i + 1 < plan.pages.len() {
                    prop_assert_eq!(range.len(), per_page);
                }
                // every page fits if every item fits alone
                let cost: usize = sizes[range.clone()].iter().map(|s| s + 1).sum();
                prop_assert!(!plan.paginated || !single_fit || 180 + cost <= limit);
            }
            prop_assert_eq!(next, sizes.len());
        } else {
            prop_assert_eq!(plan.pages.len(), 1);
            prop_assert!(plan.pages[0].1.len() <= per_page);
        }
        if !plan.paginated {
            let cost: usize = sizes.iter().map(|s| s + 1).sum();
            prop_assert!(100 + cost <= limit);
        }
    }
}

#[cfg(feature = "strict-validation")]
mod strict {
    use super::*;
    use oec_core::schema::{Flavor, Validator};

    #[test]
    fn generated_payloads_pass_the_strict_schema() {
        let validator = Validator::new().unwrap();
        let mut p = provider(true, WriterIds::new());
        let pump = p.add_asset(mam(PUMP)).unwrap();
        p.set_health(
            &pump,
            Health::aggregate(vec![oec_core::resource::DetailObject {
                health: HealthState::MaintenanceRequired,
                diagnostic_code: Some("E-17".to_owned()),
                location: None,
                description: None,
            }]),
        )
        .unwrap();
        let mut all = p.on_connected(Duration::ZERO, MS);
        for (n, rest) in [
            "MAM",
            "Health",
            "Profile",
            "PublicationList",
            &format!("Profile/{}", pump.encode()),
            &format!("PublicationList/{}/MAM", pump.encode()),
        ]
        .iter()
        .enumerate()
        {
            all.extend(
                p.handle_get(
                    &topic("Get", rest),
                    &get(GET_ID, &json!([])),
                    MS + n as u64 + 1,
                )
                .unwrap(),
            );
        }
        assert_eq!(all.len(), 13);
        for o in all {
            validator
                .validate_topic(&o.topic, Flavor::Strict, &json_of(&o))
                .unwrap_or_else(|e| panic!("{}: {e}", o.topic));
        }
    }
}
