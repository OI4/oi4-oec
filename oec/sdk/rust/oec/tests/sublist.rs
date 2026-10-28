//! The SubscriptionList against the in-process TLS test broker in routing
//! mode (C-059, P4 increment 8b, ADR-0026 §18): the subscriptions of one
//! application listed, changed and deleted by another, persisted across a
//! restart.

mod common;
mod testbroker;

use std::sync::mpsc::{Receiver, channel};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::{AppHandle, ConnectionState};
use oec::consumer::{Consumer, GetOptions, WriteOptions};
use oec::prelude::*;
use oec_core::mqtt::Packet;
use serde_json::Value;

use common::{Root, product};
use testbroker::{Pki, Seen, TestBroker, WAIT, pki};

const OEE: &str = "Oi4v2/+/+/Pub/Data/+/oee";

/// An application on `tb`; `serial` makes the AppId and the ClientId.
fn app(level: Level, serial: &str, root: &Root, pki: &Pki, tb: &TestBroker) -> App {
    if !root.path("ca.pem").exists() {
        root.write("ca.pem", &pki.ca);
    }
    let broker = Broker::new("localhost")
        .port(tb.port)
        .ca_pem(root.path("ca.pem"))
        .credentials("user", "pa:ss");
    App::builder(level, ServiceType::Utility, product())
        .reference_designation(level == Level::L2)
        .store(oec::store::FileStore::new(
            root.path(&format!("store-{serial}")),
        ))
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number(serial)
        .client_id(serial)
        .keep_alive(Duration::from_secs(1))
        .build()
        .unwrap()
}

fn spawn(app: App) -> (AppHandle, thread::JoinHandle<oec::Result<()>>) {
    let handle = app.handle();
    (handle, thread::spawn(move || app.run()))
}

fn wait_online(handle: &AppHandle) {
    let start = Instant::now();
    while handle.state() != ConnectionState::Online {
        assert!(start.elapsed() < WAIT, "not online");
        thread::sleep(Duration::from_millis(5));
    }
}

fn stop(handle: &AppHandle, run: thread::JoinHandle<oec::Result<()>>) {
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Waits until the broker saw a packet `wanted` accepts.
fn wait_for(tb: &TestBroker, wanted: impl Fn(&Packet) -> bool) {
    loop {
        if let Seen::Packet(_, p) = tb.next()
            && wanted(&p)
        {
            return;
        }
    }
}

fn subscribed(filter: &str) -> impl Fn(&Packet) -> bool + '_ {
    move |p| matches!(p, Packet::Subscribe(s) if s.filters.iter().any(|(f, _)| f == filter))
}

fn unsubscribed(filter: &str) -> impl Fn(&Packet) -> bool + '_ {
    move |p| matches!(p, Packet::Unsubscribe(u) if u.filters.iter().any(|f| f == filter))
}

/// A publication of the `oee` Data with `Count`.
fn oee_count(count: u32) -> impl Fn(&Packet) -> bool {
    move |p| {
        let Packet::Publish(p) = p else { return false };
        p.topic.contains("/Pub/Data/")
            && p.topic.ends_with("/oee")
            && serde_json::from_slice::<Value>(&p.payload)
                .is_ok_and(|v| v["Messages"][0]["Payload"]["Count"] == count)
    }
}

fn recv<T>(rx: &Receiver<T>, what: &str) -> T {
    rx.recv_timeout(WAIT)
        .unwrap_or_else(|e| panic!("{what}: {e}"))
}

/// The provider: a Level 2 app with a pump and its writable `oee`.
struct Provider {
    app: App,
    oee: DataSet,
}

impl Provider {
    fn new(root: &Root, pki: &Pki, tb: &TestBroker) -> Self {
        let app = app(Level::L2, "P-SN", root, pki, tb);
        let pump = app
            .add_asset(
                AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                    .unwrap(),
            )
            .unwrap();
        let oee = pump
            .add_data(
                DataSetDef::new("oee")
                    .field(Field::new("Count", BuiltinType::UInt32))
                    .publication(
                        Publication::default_data().writable(oec::data::Writable::ModeAndInterval),
                    ),
            )
            .unwrap();
        oee.update([("Count", 1u32)]).unwrap();
        oee.on_set(|_| Ok(())).unwrap();
        Self { app, oee }
    }
}

/// The SubscriptionList entries `consumer` gets from `target` with
/// `filter`: (TopicPath, Interval, Config) and the DataSetWriterId.
fn entries(consumer: &Consumer, target: &Target, filter: &str) -> Vec<(String, Value, Value, u16)> {
    let answers = consumer
        .get_blocking(
            Resource::SubscriptionList,
            &target.clone().filter(filter),
            &GetOptions::new(),
        )
        .unwrap();
    answers
        .iter()
        .map(|r| {
            let v: Value = serde_json::from_str(&r.payload_json()).unwrap();
            (
                v["TopicPath"].as_str().unwrap().to_owned(),
                v["Interval"].clone(),
                v["Config"].clone(),
                r.data_set_writer_id(),
            )
        })
        .collect()
}

fn sublist_target(app: &App) -> Target {
    let il = app.identification_link().clone();
    Target::app(ServiceType::Utility, il.clone()).source(il)
}

fn set(consumer: &Consumer, target: &Target, filter: &str, payload: &str) -> StatusCode {
    consumer
        .set_blocking(
            Resource::SubscriptionList,
            &target.clone().filter(filter),
            payload,
            &WriteOptions::new(),
        )
        .unwrap()
}

/// `Del` of the `oee` entry in the SubscriptionList of `target`.
fn del(consumer: &Consumer, target: &Target) -> StatusCode {
    let entry = format!(r#"{{"TopicPath": "{OEE}", "Config": "CONF_1"}}"#);
    consumer
        .del_blocking(
            Resource::SubscriptionList,
            &target.clone().filter("Data/oee"),
            Some(&entry),
            &WriteOptions::new(),
        )
        .unwrap()
}

// ---------------------------------------------------------------- tests

/// The lasting subscriptions of the consumer side are listed (configurable
/// or not) next to the `Set` subscription of a writable Data DataSet
/// (SF-107); `Set` throttles the configurable one to its Interval, `Del`
/// ends it (UNSUBSCRIBE), a non-configurable one is refused
/// (REQ-RES-SUBLIST-004/008/010/011, REQ-RES-SUBLIST-016).
#[test]
fn another_app_lists_sets_and_deletes_subscriptions() {
    let (pki, root) = (pki(), Root::new("sublist-run"));
    let tb = TestBroker::routing(&pki);
    let c = app(Level::L2, "C-SN", &root, &pki, &tb);
    let consumer = c.consumer();
    let (data_tx, data) = channel();
    let _oee_sub = consumer
        .subscribe(Resource::Data, &Target::any().filter("oee"), move |r| {
            let v: Value = serde_json::from_str(&r.payload_json()).unwrap();
            let _ = data_tx.send(v["Count"].as_u64().unwrap());
        })
        .unwrap()
        .configurable();
    let _health_sub = consumer
        .subscribe(Resource::Health, &Target::any(), |_| {})
        .unwrap()
        .interval(Duration::from_millis(250));
    let c_target = sublist_target(&c);
    let (ch, crun) = spawn(c);
    wait_online(&ch);
    wait_for(&tb, subscribed(OEE));

    let p = Provider::new(&root, &pki, &tb);
    let (pc, oee, p_target) = (p.app.consumer(), p.oee.clone(), sublist_target(&p.app));
    let (ph, prun) = spawn(p.app);
    wait_online(&ph);
    assert_eq!(recv(&data, "Count 1"), 1);

    // listed: the consumer's subscriptions and the provider's Set topic
    let oee_entry = entries(&pc, &c_target, "Data/oee");
    assert_eq!(oee_entry.len(), 1, "{oee_entry:?}");
    assert_eq!(oee_entry[0].0, OEE);
    assert_eq!(oee_entry[0].2, "CONF_1");
    let health = entries(&pc, &c_target, "Health");
    assert_eq!(health.len(), 1, "{health:?}");
    assert_eq!(
        (&health[0].1, &health[0].2),
        (&Value::from(250), &Value::from("NONE_0"))
    );
    let writable = entries(&consumer, &p_target, "Data");
    assert_eq!(writable.len(), 1, "{writable:?}");
    assert!(writable[0].0.contains("/Set/Data/") && writable[0].0.ends_with("/oee"));
    assert_eq!(writable[0].2, "NONE_0");

    // Set: at most one publication per minute
    let interval = format!(r#"{{"TopicPath": "{OEE}", "Interval": 60000, "Config": "CONF_1"}}"#);
    assert_eq!(set(&pc, &c_target, "Data/oee", &interval), StatusCode::Good);
    assert_eq!(entries(&pc, &c_target, "Data/oee")[0].1, 60000);
    oee.update([("Count", 2u32)]).unwrap();
    assert_eq!(recv(&data, "Count 2"), 2);
    oee.update([("Count", 3u32)]).unwrap();
    wait_for(&tb, oee_count(3));
    let refused = format!(r#"{{"TopicPath": "{}", "Config": "CONF_1"}}"#, health[0].0);
    assert_eq!(
        set(&pc, &c_target, "Health", &refused),
        StatusCode::BadNotWritable
    );

    // Del: unsubscribed, no longer listed; a Set restores it
    assert_eq!(del(&pc, &c_target), StatusCode::Good);
    wait_for(&tb, unsubscribed(OEE));
    assert!(entries(&pc, &c_target, "Data/oee").is_empty());
    oee.update([("Count", 4u32)]).unwrap();
    wait_for(&tb, oee_count(4));
    let restore = pc.set_blocking(
        Resource::SubscriptionList,
        &c_target.clone().filter("Data/oee"),
        &format!(r#"{{"TopicPath": "{OEE}", "Config": "CONF_1"}}"#),
        &WriteOptions::new().data_set_writer_id(oee_entry[0].3),
    );
    assert_eq!(restore.unwrap(), StatusCode::Good);
    wait_for(&tb, subscribed(OEE));
    oee.update([("Count", 5u32)]).unwrap();
    assert_eq!(recv(&data, "Count 5"), 5);
    stop(&ph, prun);
    stop(&ch, crun);
    assert!(
        data.try_iter().all(|c| c != 3 && c != 4),
        "throttled or deleted"
    );
}

/// A `Del` is persisted: after a restart the subscription stays deleted
/// and unlisted (REQ-RES-SUBLIST-011).
#[test]
fn a_deleted_subscription_stays_deleted_after_a_restart() {
    let (pki, root) = (pki(), Root::new("sublist-persist"));
    let tb = TestBroker::routing(&pki);
    let p = app(Level::L1, "P-SN", &root, &pki, &tb);
    let pc = p.consumer();
    let (ph, prun) = spawn(p);
    let start = |tb: &TestBroker| {
        let c = app(Level::L2, "C-SN", &root, &pki, tb);
        let sub = c
            .consumer()
            .subscribe(Resource::Data, &Target::any().filter("oee"), |_| {})
            .unwrap()
            .configurable();
        let target = sublist_target(&c);
        let (ch, crun) = spawn(c);
        wait_online(&ch);
        (sub, target, ch, crun)
    };
    wait_online(&ph);
    let (sub, c_target, ch, crun) = start(&tb);
    assert_eq!(entries(&pc, &c_target, "Data/oee").len(), 1);
    assert_eq!(del(&pc, &c_target), StatusCode::Good);
    stop(&ch, crun);
    drop(sub);

    let (_sub, c_target, ch, crun) = start(&tb);
    assert!(entries(&pc, &c_target, "Data/oee").is_empty());
    stop(&ch, crun);
    stop(&ph, prun);
}
