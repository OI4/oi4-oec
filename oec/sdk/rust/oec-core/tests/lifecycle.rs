//! Lifecycle and Health heartbeat (MG 6.3, MG 9.1.2): the generated Will,
//! Birth and Close equal the role vectors V-PAY-025, V-PAY-023 and
//! V-PAY-024 of the spec baseline (ADR-0026 §3); with `strict-validation`
//! every generated payload passes the embedded schema.

use std::collections::BTreeSet;
use std::path::PathBuf;
use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{HEALTH_HEARTBEAT, HealthBeat, Lifecycle, LifecycleError, Publication};
use oec_core::message::{DataSetWriterId, Message, MessageIds, PublisherId};
use oec_core::mqtt::QoS;
use oec_core::resource::{DetailObject, Health, HealthState, Mam};
use oec_core::topic::{Resource, Topic};
use serde_json::Value;

const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const WILL_MS: u64 = 1_639_693_499_990;
const BIRTH_MS: u64 = 1_639_693_500_000;
const CLOSE_MS: u64 = 1_639_693_570_000;

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..")
}

/// The vector `id` of `payload-guideline.json`: (role, topic, payload).
fn vector(id: &str) -> (String, Topic, Value) {
    let path = root().join("spec/export/vectors/payload-guideline.json");
    let doc: Value = serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
    let v = doc["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap()
        .clone();
    (
        v["role"].as_str().unwrap().to_owned(),
        Topic::parse(v["topic"].as_str().unwrap()).unwrap(),
        serde_json::from_str(v["payload"].as_str().unwrap()).unwrap(),
    )
}

fn json(payload: &[u8]) -> Value {
    serde_json::from_slice(payload).unwrap()
}

fn app() -> IdentificationLink {
    IdentificationLink::parse(APP).unwrap()
}

/// The MAM of the Birth vector V-PAY-023.
fn mam() -> Mam {
    let (_, topic, payload) = vector("V-PAY-023");
    let Message::Mam(m) = Message::from_value(&topic, payload).unwrap() else {
        panic!("V-PAY-023 is no MAM")
    };
    m.payloads().next().unwrap().payload().clone()
}

fn lifecycle() -> Lifecycle {
    let (_, topic, _) = vector("V-PAY-023");
    Lifecycle::new(
        topic.service_type(),
        app(),
        mam(),
        DataSetWriterId::new(55).unwrap(),
        DataSetWriterId::new(3456).unwrap(),
    )
    .unwrap()
}

fn health(state: HealthState, score: u8) -> Health {
    Health {
        health: state,
        health_score: Some(score),
        details: None,
    }
}

fn detail(state: HealthState) -> DetailObject {
    DetailObject {
        health: state,
        diagnostic_code: Some("E-17".into()),
        location: None,
        description: Some("sensor".into()),
    }
}

fn resource(p: &Publication) -> Resource {
    match p.topic.subject() {
        oec_core::topic::Subject::Resource(r) => r,
        other @ oec_core::topic::Subject::Service(_) => panic!("{other:?}"),
    }
}

fn health_of(p: &Publication) -> Health {
    let Message::Health(m) = Message::decode(&p.topic, &p.payload).unwrap() else {
        panic!("no Health on {}", p.topic)
    };
    m.payloads().next().unwrap().payload().clone()
}

#[test]
fn will_birth_and_close_equal_the_role_vectors() {
    let mut l = lifecycle();
    // REQ-LIFE-008 … 010, REQ-LIFE-016
    let will = l.will(WILL_MS);
    let (role, topic, expected) = vector("V-PAY-025");
    assert_eq!(role, "will");
    assert_eq!(will.topic, topic.to_string());
    assert_eq!(will.qos, QoS::AtMostOnce);
    assert_eq!(json(&will.payload), expected);
    // REQ-LIFE-001 … 003
    let birth = l.birth(BIRTH_MS);
    let (role, topic, expected) = vector("V-PAY-023");
    assert_eq!(role, "birth");
    assert_eq!(birth.topic, topic);
    assert_eq!(&birth.topic, l.mam_topic());
    assert_eq!(json(&birth.payload), expected);
    // REQ-LIFE-005 … 007
    let close = l.close(CLOSE_MS);
    let (role, topic, expected) = vector("V-PAY-024");
    assert_eq!(role, "close");
    assert_eq!(close.topic, topic);
    assert_eq!(&close.topic, l.health_topic());
    assert_eq!(json(&close.payload), expected);
}

#[test]
fn the_lifecycle_is_about_the_application_itself() {
    let (_, topic, _) = vector("V-PAY-023");
    let other = IdentificationLink::parse("provider.com/FieldDataService/FDS-001/other").unwrap();
    assert!(matches!(
        Lifecycle::new(
            topic.service_type(),
            other,
            mam(),
            DataSetWriterId::new(55).unwrap(),
            DataSetWriterId::new(3456).unwrap(),
        ),
        Err(LifecycleError::NotSelf { .. })
    ));
    assert!(matches!(
        Lifecycle::new(
            topic.service_type(),
            app(),
            mam(),
            DataSetWriterId::new(55).unwrap(),
            DataSetWriterId::new(55).unwrap(),
        ),
        Err(LifecycleError::SharedWriterId(55))
    ));
    let l = lifecycle();
    assert_eq!(l.app_id(), &app());
    assert_eq!(l.publisher_id().app_id(), &app());
    assert_eq!(l.mam().product_instance_uri, APP);
}

#[test]
fn every_will_has_its_own_message_id() {
    // REQ-LIFE-016 with REQ-MSG-008
    let mut l = lifecycle();
    let a = json(&l.will(WILL_MS).payload);
    let b = json(&l.will(WILL_MS).payload);
    assert_ne!(a["MessageId"], b["MessageId"]);
    assert_eq!(a["Messages"], b["Messages"]);
}

#[test]
fn birth_and_health_follow_the_connect() {
    // REQ-LIFE-001, REQ-LIFE-015, REQ-TOP-017
    let mut l = lifecycle();
    let t0 = Duration::from_secs(5);
    let out = l.on_connected(t0, BIRTH_MS);
    assert_eq!(
        out.iter().map(resource).collect::<Vec<_>>(),
        [Resource::Mam, Resource::Health]
    );
    assert_eq!(health_of(&out[1]), health(HealthState::Normal, 100));
    // heartbeat every 60 s (REQ-RES-HEALTH-004)
    assert_eq!(l.poll_timeout(), t0 + HEALTH_HEARTBEAT);
    assert!(
        l.poll(t0 + Duration::from_secs(59), BIRTH_MS + 59_000)
            .is_empty()
    );
    let beat = l.poll(t0 + HEALTH_HEARTBEAT, BIRTH_MS + 60_000);
    assert_eq!(beat.len(), 1);
    assert_eq!(health_of(&beat[0]), health(HealthState::Normal, 100));
    // a reconnect starts again with the Birth
    let again = l.on_connected(Duration::from_secs(100), BIRTH_MS + 100_000);
    assert_eq!(resource(&again[0]), Resource::Mam);
    assert_eq!(again.len(), 2);
}

#[test]
fn a_change_is_published_at_once() {
    let mut l = lifecycle();
    let t0 = Duration::from_secs(0);
    l.on_connected(t0, BIRTH_MS);
    assert_eq!(l.set_health(health(HealthState::Normal, 100)), Ok(false));
    let mut h = Health::aggregate(vec![detail(HealthState::OffSpec)]);
    h.health_score = Some(70);
    assert_eq!(l.set_health(h.clone()), Ok(true));
    assert_eq!(l.poll_timeout(), Duration::ZERO);
    let out = l.poll(Duration::from_secs(1), BIRTH_MS + 1_000);
    assert_eq!(out.len(), 1);
    assert_eq!(health_of(&out[0]), h);
    // a change of the Details only is a change, too
    let mut h2 = h.clone();
    h2.details.as_mut().unwrap()[0].description = Some("other".into());
    assert_eq!(l.set_health(h2), Ok(true));
    assert_eq!(l.poll(Duration::from_secs(2), BIRTH_MS + 2_000).len(), 1);
    assert_eq!(l.poll_timeout(), Duration::from_secs(62));
}

#[test]
fn health_values_are_checked() {
    let mut l = lifecycle();
    // REQ-LIFE-014: NORMAL_0/0 is the Close only
    assert_eq!(
        l.set_health(health(HealthState::Normal, 0)),
        Err(LifecycleError::CloseHealth)
    );
    // REQ-RES-HEALTH-001
    assert_eq!(
        l.set_health(health(HealthState::Normal, 101)),
        Err(LifecycleError::HealthScore(101))
    );
    // REQ-RES-HEALTH-002: Health equals the most severe detail
    let mut h = Health::aggregate(vec![
        detail(HealthState::MaintenanceRequired),
        detail(HealthState::CheckFunction),
    ]);
    assert_eq!(h.health, HealthState::CheckFunction);
    h.health = HealthState::Failure;
    assert_eq!(
        l.set_health(h),
        Err(LifecycleError::Aggregation {
            health: HealthState::Failure,
            details: HealthState::CheckFunction,
        })
    );
    // empty Details aggregate nothing
    let mut h = health(HealthState::OffSpec, 50);
    h.details = Some(Vec::new());
    assert_eq!(l.set_health(h), Ok(true));
    assert_eq!(l.health().health, HealthState::OffSpec);
    // errors carry their rule
    assert!(
        LifecycleError::CloseHealth
            .to_string()
            .contains("REQ-LIFE-014")
    );
}

#[test]
fn recovery_from_failure_starts_with_a_birth() {
    // REQ-LIFE-012, REQ-LIFE-017
    let mut l = lifecycle();
    l.on_connected(Duration::ZERO, BIRTH_MS);
    l.set_health(health(HealthState::Failure, 0)).unwrap();
    let out = l.poll(Duration::from_secs(1), BIRTH_MS + 1_000);
    assert_eq!(
        out.iter().map(resource).collect::<Vec<_>>(),
        [Resource::Health]
    );
    // the heartbeat of the failure needs no Birth
    let out = l.poll(Duration::from_secs(61), BIRTH_MS + 61_000);
    assert_eq!(
        out.iter().map(resource).collect::<Vec<_>>(),
        [Resource::Health]
    );
    l.set_health(health(HealthState::Normal, 90)).unwrap();
    let out = l.poll(Duration::from_secs(62), BIRTH_MS + 62_000);
    assert_eq!(
        out.iter().map(resource).collect::<Vec<_>>(),
        [Resource::Mam, Resource::Health]
    );
    assert_eq!(health_of(&out[1]), health(HealthState::Normal, 90));
    // a failure with another score is no unavailability
    l.set_health(health(HealthState::Failure, 10)).unwrap();
    l.poll(Duration::from_secs(63), BIRTH_MS + 63_000);
    l.set_health(health(HealthState::Normal, 90)).unwrap();
    let out = l.poll(Duration::from_secs(64), BIRTH_MS + 64_000);
    assert_eq!(
        out.iter().map(resource).collect::<Vec<_>>(),
        [Resource::Health]
    );
}

#[test]
fn asset_health_is_published_on_the_asset() {
    // REQ-CMP-007
    let mut l = lifecycle();
    let asset = IdentificationLink::parse("vendor.com/Pump/P-1/0815").unwrap();
    let writer = DataSetWriterId::new(17).unwrap();
    let h = health(HealthState::MaintenanceRequired, 80);
    let p = l.asset_health(&asset, writer, &h, BIRTH_MS).unwrap();
    assert_eq!(p.topic.source(), Some(&asset));
    assert_eq!(p.topic.app_id(), &app());
    assert_eq!(resource(&p), Resource::Health);
    let v = json(&p.payload);
    assert_eq!(v["Messages"][0]["DataSetWriterId"], 17);
    assert_eq!(v["Messages"][0]["DataSetWriterName"], asset.as_str());
    assert_eq!(health_of(&p), h);
    assert_eq!(
        l.asset_health(&app(), writer, &h, BIRTH_MS),
        Err(LifecycleError::SelfAsset)
    );
    assert!(matches!(
        l.asset_health(&asset, writer, &health(HealthState::Normal, 200), BIRTH_MS),
        Err(LifecycleError::HealthScore(200))
    ));
}

#[test]
fn health_beat_schedules_an_asset() {
    let mut b = HealthBeat::new(health(HealthState::Normal, 100));
    assert_eq!(b.poll_timeout(), Duration::ZERO);
    assert!(b.poll(Duration::from_secs(10)).is_some());
    assert_eq!(b.poll_timeout(), Duration::from_secs(70));
    assert!(b.poll(Duration::from_secs(69)).is_none());
    assert!(!b.set(health(HealthState::Normal, 100)));
    assert!(b.set(health(HealthState::Normal, 99)));
    assert_eq!(
        b.poll(Duration::from_secs(20)).unwrap().health_score,
        Some(99)
    );
    assert!(b.poll(Duration::from_secs(21)).is_none());
    b.restart();
    assert!(b.poll(Duration::from_secs(21)).is_some());
    assert_eq!(b.health().health_score, Some(99));
}

#[test]
fn message_ids_are_unique_and_monotonic() {
    // REQ-MSG-007, REQ-MSG-008
    let mut ids = MessageIds::new(PublisherId::new(
        lifecycle().publisher_id().service_type(),
        app(),
    ));
    let mut seen = BTreeSet::new();
    for ms in [
        BIRTH_MS,
        BIRTH_MS,
        BIRTH_MS,
        BIRTH_MS + 1,
        BIRTH_MS - 5,
        BIRTH_MS + 1,
    ] {
        assert!(seen.insert(ids.next_id(ms).as_str().to_owned()));
    }
    let first = MessageIds::new(ids.publisher().clone()).next_id(BIRTH_MS);
    assert_eq!(
        first.as_str(),
        format!("{BIRTH_MS}-OTConnector/{APP}"),
        "the first id of a millisecond has no counter"
    );
}

#[cfg(feature = "strict-validation")]
mod strict {
    use std::time::Duration;

    use oec_core::schema::{Flavor, Validator};

    use super::{
        BIRTH_MS, CLOSE_MS, HealthState, IdentificationLink, Publication, WILL_MS, detail, health,
        json, lifecycle,
    };
    use oec_core::message::DataSetWriterId;
    use oec_core::resource::Health;
    use oec_core::topic::Topic;

    #[test]
    fn generated_payloads_pass_the_strict_schema() {
        let validator = Validator::new().unwrap();
        let mut l = lifecycle();
        let will = l.will(WILL_MS);
        let mut all = vec![Publication {
            topic: Topic::parse(&will.topic).unwrap(),
            payload: will.payload,
        }];
        all.extend(l.on_connected(Duration::ZERO, BIRTH_MS));
        let mut h = Health::aggregate(vec![
            detail(HealthState::Failure),
            detail(HealthState::OffSpec),
        ]);
        h.health_score = Some(0);
        l.set_health(h.clone()).unwrap();
        all.extend(l.poll(Duration::from_secs(1), BIRTH_MS + 1));
        l.set_health(health(HealthState::Normal, 100)).unwrap();
        all.extend(l.poll(Duration::from_secs(2), BIRTH_MS + 2));
        let asset = IdentificationLink::parse("vendor.com/Pump/P-1/0815").unwrap();
        all.push(
            l.asset_health(&asset, DataSetWriterId::new(19).unwrap(), &h, BIRTH_MS + 3)
                .unwrap(),
        );
        all.push(l.close(CLOSE_MS));
        assert_eq!(all.len(), 8);
        for p in all {
            validator
                .validate_topic(&p.topic, Flavor::Strict, &json(&p.payload))
                .unwrap_or_else(|e| panic!("{}: {e}", p.topic));
        }
    }
}
