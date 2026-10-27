//! The consumer side against the in-process TLS test broker in routing
//! mode: two SDK applications, one asks the other (C-058, ADR-0026 §17).

mod common;
mod testbroker;

use std::sync::mpsc::{Receiver, channel};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::{AppHandle, ConnectionState};
use oec::consumer::{GetOptions, Payload, Presence, Query, WriteOptions};
use oec::prelude::*;
use oec_core::mqtt::Packet;
use serde_json::Value;

use common::{Root, product};
use testbroker::{Pki, Seen, TestBroker, WAIT, pki};

// ---------------------------------------------------------------- setup

fn routing_broker(pki: &Pki) -> TestBroker {
    TestBroker::routing(pki)
}

/// An application on `tb`; `serial` makes the AppId and the ClientId.
fn app(level: Level, serial: &str, root: &Root, pki: &Pki, tb: &TestBroker) -> App {
    root.write("ca.pem", &pki.ca);
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

fn wait_until(what: &str, mut done: impl FnMut() -> bool) {
    let start = Instant::now();
    while !done() {
        assert!(start.elapsed() < WAIT, "not reached: {what}");
        thread::sleep(Duration::from_millis(5));
    }
}

fn wait_online(handle: &AppHandle) {
    wait_until("online", || handle.state() == ConnectionState::Online);
}

fn stop(handle: &AppHandle, run: thread::JoinHandle<oec::Result<()>>) {
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Waits until the broker saw a SUBSCRIBE of `filter` (then it routes).
fn wait_subscribed(tb: &TestBroker, filter: &str) {
    loop {
        if let Seen::Packet(_, Packet::Subscribe(s)) = tb.next()
            && s.filters.iter().any(|(f, _)| f == filter)
        {
            return;
        }
    }
}

/// Waits until the broker saw an UNSUBSCRIBE of `filter`.
fn wait_unsubscribed(tb: &TestBroker, filter: &str) {
    loop {
        if let Seen::Packet(_, Packet::Unsubscribe(u)) = tb.next()
            && u.filters.iter().any(|f| f == filter)
        {
            return;
        }
    }
}

fn recv<T>(rx: &Receiver<T>, what: &str) -> T {
    rx.recv_timeout(WAIT)
        .unwrap_or_else(|e| panic!("{what}: {e}"))
}

fn pump_def(serial: &str) -> AssetDef {
    AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), serial).unwrap()
}

/// The provider: a Level 2 app with a pump (writable `oee`, refused above
/// Count 100) and a second pump, the child of the first.
struct Provider {
    app: App,
    pump: Asset,
    child: Asset,
    oee: DataSet,
}

impl Provider {
    fn new(root: &Root, pki: &Pki, tb: &TestBroker) -> Self {
        let app = app(Level::L2, "P-SN", root, pki, tb);
        let pump = app.add_asset(pump_def("0815")).unwrap();
        let child = app.add_asset(pump_def("0816")).unwrap();
        let oee = pump
            .add_data(
                DataSetDef::new("oee")
                    .field(Field::double("Availability"))
                    .field(Field::new("Count", BuiltinType::UInt32))
                    .publication(
                        Publication::default_data().writable(oec::data::Writable::ModeAndInterval),
                    ),
            )
            .unwrap();
        oee.update([
            ("Availability", oec::data::Value::from(97.5)),
            ("Count", 1u32.into()),
        ])
        .unwrap();
        oee.on_set(|s| match s.get("Count") {
            Some(oec::data::Value::UInt32(c)) if *c <= 100 => Ok(()),
            _ => Err(StatusCode::BadOutOfRange),
        })
        .unwrap();
        Self {
            app,
            pump,
            child,
            oee,
        }
    }
}

fn json(text: &str) -> Value {
    serde_json::from_str(text).unwrap()
}

/// The asset of each answer (DataSetWriterName, REQ-MSG-023), sorted.
fn sources(answers: &[oec::consumer::Received]) -> Vec<String> {
    let mut s: Vec<_> = answers
        .iter()
        .map(|r| r.data_set_writer_name().as_str().to_owned())
        .collect();
    s.sort();
    s
}

// ---------------------------------------------------------------- tests

/// `Get` of MAM (all pages, also one per page), Health, Data, Metadata (the
/// DataSetWriterId resolved by a `Get Data`) and the Status `BadNotFound`
/// for an unknown asset (REQ-MSG-015, REQ-PAG-002/006, REQ-CMP-047); the
/// async `get` on another runtime, `get_blocking` inside one refused.
#[test]
fn one_app_gets_from_another() {
    let (pki, root) = (pki(), Root::new("cons-get"));
    let tb = routing_broker(&pki);
    let p = Provider::new(&root, &pki, &tb);
    let app_il = p.app.identification_link().clone();
    let target = Target::app(ServiceType::Utility, app_il.clone());
    let (pump, child) = (
        p.pump.identification_link().clone(),
        p.child.identification_link().clone(),
    );
    let c = app(Level::L1, "C-SN", &root, &pki, &tb);
    let consumer = c.consumer();
    let (ph, prun) = spawn(p.app);
    let (ch, crun) = spawn(c);
    wait_online(&ph);
    wait_online(&ch);

    let all = consumer
        .get_blocking(Resource::Mam, &target, &GetOptions::new())
        .unwrap();
    assert!(all.iter().all(|r| matches!(r.payload(), Payload::Mam(_))));
    let mut expected = vec![
        app_il.as_str().to_owned(),
        pump.as_str().to_owned(),
        child.as_str().to_owned(),
    ];
    expected.sort();
    assert_eq!(sources(&all), expected);
    let paged = consumer
        .get_blocking(Resource::Mam, &target, &GetOptions::new().per_page(1))
        .unwrap();
    assert_eq!(sources(&paged), sources(&all));

    let health = consumer
        .get_blocking(
            Resource::Health,
            &target.clone().source(pump.clone()),
            &GetOptions::new(),
        )
        .unwrap();
    assert_eq!(health.len(), 1);
    assert!(matches!(health[0].payload(), Payload::Health(_)));
    get_data_and_metadata(&consumer, &target.clone().source(pump));

    let unknown = target
        .clone()
        .source(IdentificationLink::parse("vendor.com/Pump/P-1/9999").unwrap());
    assert!(matches!(
        consumer.get_blocking(Resource::Health, &unknown, &GetOptions::new()),
        Err(oec::Error::Status(StatusCode::BadNotFound))
    ));
    on_another_runtime(&consumer, &target);
    stop(&ch, crun);
    stop(&ph, prun);
}

fn get_data_and_metadata(consumer: &Consumer, pump: &Target) {
    let data = consumer
        .get_blocking(
            Resource::Data,
            &pump.clone().filter("oee"),
            &GetOptions::new(),
        )
        .unwrap();
    assert_eq!(data.len(), 1);
    assert_eq!(data[0].filter().as_deref(), Some("oee"));
    assert_eq!(json(&data[0].payload_json())["Availability"], 97.5);
    let meta = consumer
        .get_blocking(
            Resource::Metadata,
            &pump.clone().filter("oee"),
            &GetOptions::new(),
        )
        .unwrap();
    assert_eq!(meta.len(), 1);
    assert!(matches!(meta[0].payload(), Payload::MetaData(_)));
    assert_eq!(meta[0].data_set_writer_id(), data[0].data_set_writer_id());
}

fn on_another_runtime(consumer: &Consumer, target: &Target) {
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_time()
        .build()
        .unwrap();
    let mam = rt
        .block_on(consumer.get(Resource::Mam, target, &GetOptions::new()))
        .unwrap();
    assert!(!mam.is_empty());
    let inside =
        rt.block_on(async { consumer.get_blocking(Resource::Mam, target, &GetOptions::new()) });
    assert!(matches!(inside, Err(oec::Error::Invalid(_))), "{inside:?}");
}

/// `Set Data` decided by the handler of the other app (Good, then
/// `BadOutOfRange`), `Set`/`Get` with Query/`Del ReferenceDesignation`
/// (REQ-RES-EVENT-005, REQ-RES-REFDES-012/015).
#[test]
fn one_app_writes_to_another() {
    let (pki, root) = (pki(), Root::new("cons-write"));
    let tb = routing_broker(&pki);
    let p = Provider::new(&root, &pki, &tb);
    let target = Target::app(ServiceType::Utility, p.app.identification_link().clone());
    let (pump, child, oee) = (p.pump.clone(), p.child.clone(), p.oee.clone());
    let c = app(Level::L1, "C-SN", &root, &pki, &tb);
    let consumer = c.consumer();
    let (ph, prun) = spawn(p.app);
    let (ch, crun) = spawn(c);
    wait_online(&ph);
    wait_online(&ch);

    let at_oee = target
        .clone()
        .source(pump.identification_link().clone())
        .filter("oee");
    let w = WriteOptions::new();
    let good = consumer.set_blocking(
        Resource::Data,
        &at_oee,
        r#"{"Availability": 50.0, "Count": 7}"#,
        &w,
    );
    assert_eq!(good.unwrap(), StatusCode::Good);
    assert_eq!(oee.values().unwrap()[1], oec::data::Value::UInt32(7));
    let refused = consumer.set_blocking(
        Resource::Data,
        &at_oee,
        r#"{"Availability": 50.0, "Count": 500}"#,
        &w,
    );
    assert_eq!(refused.unwrap(), StatusCode::BadOutOfRange);
    assert!(matches!(
        consumer.set_blocking(Resource::Data, &at_oee, "[1]", &w),
        Err(oec::Error::Invalid(_))
    ));
    reference_designation(&consumer, &target, &pump, &child);
    stop(&ch, crun);
    stop(&ph, prun);
}

fn reference_designation(consumer: &Consumer, target: &Target, pump: &Asset, child: &Asset) {
    let w = WriteOptions::new();
    let at_child = target.clone().source(child.identification_link().clone());
    let rd = format!(
        r#"{{"Function": {{"Value": "=P1.M2", "Parent": {{"Value": "=P1", "IdentificationLink": "{}"}}}}}}"#,
        pump.identification_link().as_str()
    );
    let set = consumer.set_blocking(Resource::ReferenceDesignation, &at_child, &rd, &w);
    assert_eq!(set.unwrap(), StatusCode::Good);
    let children = consumer
        .get_blocking(
            Resource::ReferenceDesignation,
            target,
            &GetOptions::new().query(Query {
                parent: pump.identification_link().as_str().to_owned(),
                aspect: None,
            }),
        )
        .unwrap();
    assert_eq!(children.len(), 1, "{children:?}");
    assert_eq!(
        &children[0].data_set_writer_name(),
        child.identification_link()
    );
    assert!(matches!(
        children[0].payload(),
        Payload::ReferenceDesignation(_)
    ));
    let del = consumer.del_blocking(Resource::ReferenceDesignation, &at_child, None, &w);
    assert_eq!(del.unwrap(), StatusCode::Good);
    let again = consumer.del_blocking(Resource::ReferenceDesignation, &at_child, None, &w);
    assert_eq!(again.unwrap(), StatusCode::BadNotFound);
}

/// Presence (Birth, Will, Close; MG 6.3) and a typed subscription that
/// ends with its drop (UNSUBSCRIBE).
#[test]
fn presence_and_subscriptions() {
    let (pki, root) = (pki(), Root::new("cons-sub"));
    let tb = routing_broker(&pki);
    let c = app(Level::L1, "C-SN", &root, &pki, &tb);
    let consumer = c.consumer();
    let (presence_tx, presence) = channel();
    let _presence = consumer
        .on_presence(move |p| {
            let _ = presence_tx.send(p.clone());
        })
        .unwrap();
    let (data_tx, data) = channel();
    let subscription = consumer
        .subscribe(Resource::Data, &Target::any().filter("oee"), move |r| {
            let _ = data_tx.send((r.source(), r.payload_json()));
        })
        .unwrap();
    let (ch, crun) = spawn(c);
    wait_online(&ch);
    wait_subscribed(&tb, "Oi4v2/+/+/Pub/Data/+/oee");

    let p = Provider::new(&root, &pki, &tb);
    let p_il = p.app.identification_link().clone();
    let pump = p.pump.identification_link().clone();
    let oee = p.oee.clone();
    let (ph, prun) = spawn(p.app);
    let online = |what| loop {
        if let Presence::Online { app, .. } = recv(&presence, what)
            && app == p_il
        {
            return;
        }
    };
    online("Birth");
    let (source, payload) = recv(&data, "Data of oee");
    assert_eq!(source.as_ref(), Some(&pump));
    assert_eq!(json(&payload)["Count"], 1);
    let info = consumer.apps();
    let known = info.iter().find(|a| a.app == p_il).unwrap();
    assert!(known.mam.is_some());

    // the Will of a lost connection, then a new Birth
    tb.drop_connection(1);
    loop {
        if let Presence::OfflineUngraceful { app, .. } = recv(&presence, "Will")
            && app == p_il
        {
            break;
        }
    }
    online("second Birth");

    drop(subscription);
    wait_unsubscribed(&tb, "Oi4v2/+/+/Pub/Data/+/oee");
    oee.update([("Count", 2u32)]).unwrap();
    stop(&ph, prun);
    loop {
        if let Presence::OfflineGraceful { app, .. } = recv(&presence, "Close")
            && app == p_il
        {
            break;
        }
    }
    assert!(
        data.try_iter().all(|(_, p)| json(&p)["Count"] != 2),
        "a publication after the drop"
    );
    stop(&ch, crun);
}

/// Requests while offline (`NotConnected`), without answer (`Timeout`),
/// without AppId (`Invalid`) and after the shutdown (`Closed`).
#[test]
fn requests_fail_cleanly() {
    let (pki, root) = (pki(), Root::new("cons-fail"));
    let tb = routing_broker(&pki);
    let c = app(Level::L1, "C-SN", &root, &pki, &tb);
    let consumer = c.consumer();
    let nobody = Target::app(
        ServiceType::Utility,
        IdentificationLink::parse("vendor.com/Nobody/N-1/1").unwrap(),
    );
    let quick = GetOptions::new().timeout(Duration::from_millis(200));
    assert!(matches!(
        consumer.get_blocking(Resource::Mam, &nobody, &quick),
        Err(oec::Error::NotConnected)
    ));
    let (ch, crun) = spawn(c);
    wait_online(&ch);
    assert!(matches!(
        consumer.get_blocking(Resource::Mam, &nobody, &quick),
        Err(oec::Error::Timeout)
    ));
    assert!(matches!(
        consumer.get_blocking(Resource::Mam, &Target::any(), &quick),
        Err(oec::Error::Invalid(_))
    ));
    stop(&ch, crun);
    assert!(matches!(
        consumer.get_blocking(Resource::Mam, &nobody, &quick),
        Err(oec::Error::Closed)
    ));
}
