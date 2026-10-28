//! The subscriptions of the consumer side as entries of the SubscriptionList
//! (P4 increment 8b, ADR-0026 §18), without network: the consumer hands
//! its lasting subscriptions to the provider of the same application, and
//! the settings of `Set` and `Del SubscriptionList` back – an Interval
//! delivers a topic at most once per Interval (REQ-RES-SUBLIST-003), a
//! deleted filter is unsubscribed (REQ-RES-SUBLIST-011). The application is
//! that of MG B1.1.9, which subscribes to its own Data `oee` (V-PAY-058,
//! V-PAY-072 … 074).

use std::time::Duration;

use oec_core::consumer::{Address, Consumer, GetParams, Outcome};
use oec_core::il::IdentificationLink;
use oec_core::lifecycle::Lifecycle;
use oec_core::message::{DataSetWriterId, MessageIds, PublisherId};
use oec_core::provider::{
    ConsumerSubscription, Handled, Provider, ProviderConfig, SubscriptionOverrides, WriterIds,
};
use oec_core::resource::Mam;
use oec_core::status::StatusCode;
use oec_core::topic::{Filter, Method, Resource, ServiceType, TopicFilter};
use serde_json::{Value as Json, json};

const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
const MS: u64 = 1_639_693_510_000;

fn il(s: &str) -> IdentificationLink {
    IdentificationLink::parse(s).unwrap()
}

fn vector(id: &str) -> (String, Vec<u8>) {
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
        v["payload"].as_str().unwrap().as_bytes().to_vec(),
    )
}

fn provider() -> Provider {
    let mam: Mam = serde_json::from_value(json!({
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
        "ProductInstanceUri": APP,
        "RevisionCounter": 0
    }))
    .unwrap();
    let lifecycle = Lifecycle::new(
        ServiceType::OTConnector,
        il(APP),
        mam,
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get: true,
        max_packet_size: 262_144,
        publish_limit: 262_144,
    };
    Provider::new(lifecycle, config, WriterIds::new()).unwrap()
}

/// `Pub/Data/<APP>/oee` of the application (MG B1.1.9).
fn oee() -> TopicFilter {
    TopicFilter::builder(Method::Pub, Resource::Data)
        .service_type(ServiceType::OTConnector)
        .app_id(il(APP))
        .source(il(APP))
        .filter(Filter::Tag("oee".to_owned()))
        .build()
        .unwrap()
}

/// The subscriptions delivered a publication of `oee` at `now`.
fn delivered(c: &mut Consumer, now: u64) -> usize {
    let (topic, payload) = vector("V-PAY-058");
    let ids = &mut MessageIds::new(PublisherId::new(ServiceType::OTConnector, il(APP)));
    let mut next = || ids.next_id(now);
    let incoming = c.handle_message(&topic, &payload, now, &mut next);
    incoming
        .deliveries
        .first()
        .map_or(0, |(subs, _)| subs.len())
}

/// A `Set` or `Del SubscriptionList` vector to the provider; `Good`.
fn write(p: &mut Provider, vector_id: &str) {
    let (topic, payload) = vector(vector_id);
    let Handled::Publish(out) = p.handle_message(&topic, &payload, MS).unwrap() else {
        panic!("answered by the provider");
    };
    let status: Json = serde_json::from_slice(&out.last().unwrap().payload).unwrap();
    assert_eq!(
        status["Messages"][0]["Payload"]["Number"],
        StatusCode::GOOD.0
    );
}

#[test]
fn subscriptions_are_handed_over_when_they_change() {
    let mut c = Consumer::new(ServiceType::OTConnector, il(APP));
    assert!(c.take_subscriptions().is_none());
    let id = c.subscribe(&[oee()]);
    let presence = c.subscribe_presence();
    c.configure(id, 250, true);
    // a presence subscription is not configurable
    c.configure(presence, 5, true);
    let subs = c.take_subscriptions().unwrap();
    let entry = |path: &str, interval_ms, configurable| ConsumerSubscription {
        topic_path: path.to_owned(),
        interval_ms,
        configurable,
    };
    assert_eq!(
        subs,
        [
            entry(oee().as_str(), 250, true),
            entry("Oi4v2/+/+/Pub/MAM/+", 0, false),
            entry("Oi4v2/+/+/Pub/Health/+", 0, false),
        ]
    );
    assert!(c.take_subscriptions().is_none());
    c.configure(id, 250, true);
    assert!(c.take_subscriptions().is_none(), "unchanged");
    c.unsubscribe(presence);
    assert_eq!(c.take_subscriptions().unwrap().len(), 1);
    // the answer subscriptions of a request are not listed (SF-109)
    let ids = &mut MessageIds::new(PublisherId::new(ServiceType::OTConnector, il(APP)));
    let mut next = || ids.next_id(MS);
    let address = Address::app(ServiceType::OTConnector, il(APP));
    c.get(
        Resource::Mam,
        &address,
        &GetParams::default(),
        MS + 10,
        MS,
        &mut next,
    )
    .unwrap();
    assert!(c.take_subscriptions().is_none());
}

/// MG B1.1.9 end to end: the `Set` V-PAY-072 gives the subscription an
/// Interval of 1000 ms, the `Del` V-PAY-074 deletes it, a `Set` lists it
/// again.
#[test]
fn set_and_del_subscription_list_reach_the_subscription() {
    let mut c = Consumer::new(ServiceType::OTConnector, il(APP));
    let id = c.subscribe(&[oee()]);
    c.configure(id, 0, true);
    let mut p = provider();
    p.set_consumer_subscriptions(c.take_subscriptions().unwrap());
    p.on_connected(Duration::ZERO, MS);
    c.set_filter_overrides(p.take_filter_overrides().unwrap());
    assert_eq!(c.take_filter_changes().subscribe, [oee().as_str()]);
    assert_eq!((delivered(&mut c, MS), delivered(&mut c, MS + 1)), (1, 1));

    write(&mut p, "V-PAY-072");
    c.set_filter_overrides(p.take_filter_overrides().unwrap());
    let at = |c: &mut Consumer, ms: &[u64]| -> Vec<usize> {
        ms.iter().map(|ms| delivered(c, MS + ms)).collect()
    };
    // at most once per Interval, the rest is dropped
    assert_eq!(
        at(&mut c, &[10, 500, 1009, 1010, 1500, 2010]),
        [1, 0, 0, 1, 0, 1]
    );
    assert!(c.take_filter_changes().is_empty());

    write(&mut p, "V-PAY-074");
    c.set_filter_overrides(p.take_filter_overrides().unwrap());
    assert_eq!(c.take_filter_changes().unsubscribe, [oee().as_str()]);
    assert_eq!(delivered(&mut c, MS + 5000), 0);

    write(&mut p, "V-PAY-072");
    c.set_filter_overrides(p.take_filter_overrides().unwrap());
    assert_eq!(c.take_filter_changes().subscribe, [oee().as_str()]);
    assert_eq!(delivered(&mut c, MS + 6000), 1);
}

/// A deleted filter that is also the answer filter of a request is
/// subscribed while the request waits, and delivers nothing to the
/// subscription.
#[test]
fn a_deleted_filter_serves_requests() {
    let mut c = Consumer::new(ServiceType::OTConnector, il(APP));
    let all = TopicFilter::builder(Method::Pub, Resource::Mam)
        .service_type(ServiceType::OTConnector)
        .app_id(il(APP))
        .build()
        .unwrap();
    let id = c.subscribe(std::slice::from_ref(&all));
    c.configure(id, 0, true);
    let mut deleted = SubscriptionOverrides::new();
    deleted.set(
        all.as_str(),
        oec_core::provider::SubscriptionSetting::Deleted,
    );
    c.set_filter_overrides(deleted);
    assert!(c.take_filter_changes().is_empty());

    let ids = &mut MessageIds::new(PublisherId::new(ServiceType::OTConnector, il(APP)));
    let mut next = || ids.next_id(MS);
    let address = Address::app(ServiceType::OTConnector, il(APP));
    c.get(
        Resource::Mam,
        &address,
        &GetParams::default(),
        MS + 10,
        MS,
        &mut next,
    )
    .unwrap();
    let changes = c.take_filter_changes();
    assert!(
        changes.subscribe.contains(&all.as_str().to_owned()),
        "{changes:?}"
    );
    c.expire(MS + 10);
    assert_eq!(c.take_completions()[0].outcome, Outcome::Timeout);
    let changes = c.take_filter_changes();
    assert!(
        changes.unsubscribe.contains(&all.as_str().to_owned()),
        "{changes:?}"
    );
}
