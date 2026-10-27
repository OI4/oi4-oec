//! `App::run` / `App::run_async` (ADR-0020 §2/§5, ADR-0026 §9/§10) against
//! an in-process MQTT 3.1.1 test broker over TLS: the broker uses the codec
//! of `oec-core` and rustls on blocking threads, the PKI is generated per
//! test with rcgen. Only loopback connections, no network.

mod common;
mod testbroker;

use std::net::TcpListener;
use std::sync::mpsc::RecvTimeoutError;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::ConnectionState;
use oec::prelude::*;
use oec_core::mqtt::{Connect, ConnectReturnCode, Delivery, Packet, Publish};
use serde_json::Value;

use common::{Root, product};
use testbroker::{Behavior, Pki, Seen, TestBroker, WAIT, encrypted_pem, pki};

// ---------------------------------------------------------------- app

/// Records the states reported to `on_connection`.
#[derive(Clone, Default)]
struct States(Arc<Mutex<Vec<ConnectionState>>>);

impl States {
    fn get(&self) -> Vec<ConnectionState> {
        self.0.lock().unwrap().clone()
    }

    /// Waits until `state` was reported `times` times.
    fn wait_for(&self, state: ConnectionState, times: usize) {
        let start = Instant::now();
        while self.get().iter().filter(|s| **s == state).count() < times {
            assert!(
                start.elapsed() < WAIT,
                "{state:?} ×{times} not reached: {:?}",
                self.get()
            );
            thread::sleep(Duration::from_millis(10));
        }
    }
}

fn builder(broker: Broker, states: &States) -> oec::app::AppBuilder {
    builder_at(Level::L1, broker, states)
}

fn builder_at(level: Level, broker: Broker, states: &States) -> oec::app::AppBuilder {
    let states = states.clone();
    App::builder(level, ServiceType::Utility, product())
        .reference_designation(false)
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("SN-1")
        .client_id("app-1")
        .keep_alive(Duration::from_secs(1))
        .on_connection(move |s| states.0.lock().unwrap().push(s))
}

/// An explicit broker on `port` with the CA of `pki` and credentials.
fn broker(root: &Root, pki: &Pki, port: u16) -> Broker {
    root.write("ca.pem", &pki.ca);
    Broker::new("localhost")
        .port(port)
        .ca_pem(root.path("ca.pem"))
        .credentials("user", "pa:ss")
}

fn spawn(app: App) -> thread::JoinHandle<oec::Result<()>> {
    thread::spawn(move || app.run())
}

fn json(payload: &[u8]) -> Value {
    serde_json::from_slice(payload).unwrap()
}

/// `(Health, HealthScore)` of a Pub Health payload.
fn health(payload: &[u8]) -> (String, u64) {
    let v = json(payload);
    let p = &v["Messages"][0]["Payload"];
    (
        p["Health"].as_str().unwrap().to_owned(),
        p["HealthScore"].as_u64().unwrap(),
    )
}

fn message_id(payload: &[u8]) -> String {
    json(payload)["MessageId"].as_str().unwrap().to_owned()
}

/// The topic form of an Identification Link (REQ-IL-002 … 004).
fn encoded(il: &str) -> String {
    oec_core::il::IdentificationLink::parse(il)
        .unwrap()
        .encode()
}

/// The next PUBLISH, skipping PINGREQ and SUBSCRIBE.
fn publication(tb: &TestBroker) -> Publish {
    loop {
        match tb.packet() {
            (_, Packet::Publish(p)) => return p,
            (_, Packet::PingReq | Packet::Subscribe(_)) => {}
            other => panic!("expected PUBLISH, got {other:?}"),
        }
    }
}

/// The PUBLISHes up to and including the next one on `topic`.
fn publications_until(tb: &TestBroker, topic: &str) -> Vec<Publish> {
    let mut out = Vec::new();
    loop {
        let p = publication(tb);
        let done = p.topic == topic;
        out.push(p);
        if done {
            return out;
        }
    }
}

/// `(Resource, Source)` → DataSetWriterId of a series of publications.
fn writer_ids(ps: &[Publish]) -> Vec<u64> {
    let mut ids: Vec<u64> = ps
        .iter()
        .filter(|p| !p.topic.contains("/Pub/PublicationList"))
        .flat_map(|p| {
            json(&p.payload)["Messages"]
                .as_array()
                .unwrap()
                .iter()
                .map(|m| m["DataSetWriterId"].as_u64().unwrap())
                .collect::<Vec<_>>()
        })
        .collect();
    ids.sort_unstable();
    ids
}

/// The DataSetWriterIds listed in a PublicationList publication.
fn listed_ids(p: &Publish) -> Vec<u64> {
    let mut ids: Vec<u64> = json(&p.payload)["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|m| m["Payload"]["Resource"] != "Event")
        .map(|m| m["Payload"]["DataSetWriterId"].as_u64().unwrap())
        .collect();
    ids.sort_unstable();
    ids
}

/// The sorted filters a Level 2 application subscribes below `prefix`.
fn served_filters(prefix: &str) -> Vec<String> {
    ["ReferenceDesignation", "SubscriptionList"]
        .map(|r| format!("{prefix}Del/{r}/#"))
        .into_iter()
        .chain(
            [
                "Config",
                "Data",
                "Health",
                "MAM",
                "Metadata",
                "Profile",
                "PublicationList",
            ]
            .map(|r| format!("{prefix}Get/{r}/#")),
        )
        .chain(
            [
                "Config",
                "Data",
                "Metadata",
                "PublicationList",
                "ReferenceDesignation",
                "SubscriptionList",
            ]
            .map(|r| format!("{prefix}Set/{r}/#")),
        )
        .collect()
}

/// The number of Event entries of a PublicationList publication.
fn listed_events(p: &Publish) -> usize {
    json(&p.payload)["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|m| m["Payload"]["Resource"] == "Event")
        .count()
}

const GET_ID: &str = "1700000000000-Aggregation/vendor.com/Client/C-1/1";

/// A `Get` NetworkMessage from a consumer, QoS 0.
fn get(topic: String) -> Packet {
    Packet::Publish(Publish {
        topic,
        payload: serde_json::to_vec(&serde_json::json!({
            "MessageId": GET_ID,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "DataSetClassId": "",
            "ReplyTo": "",
            "Messages": []
        }))
        .unwrap(),
        delivery: Delivery::AtMostOnce,
        retain: false,
    })
}

// ---------------------------------------------------------------- tests

/// CONNECT with the Will, Birth before Health, Close before DISCONNECT
/// (REQ-LIFE-001, REQ-LIFE-005, REQ-LIFE-008, REQ-LIFE-015, REQ-MQTT-009,
/// REQ-MQTT-013 … 015).
#[test]
fn lifecycle_on_one_connection() {
    let (pki, root) = (pki(), Root::new("rt-lifecycle"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let il = app.identification_link().to_string();
    assert_eq!(handle.state(), ConnectionState::Closed);
    let run = spawn(app);

    let (n, c) = tb.connect();
    assert_eq!(n, 0);
    assert_eq!(
        (c.client_id.as_str(), c.keep_alive, c.clean_session),
        ("app-1", 1, true)
    );
    assert_eq!(
        (c.username.as_deref(), c.password.as_deref()),
        (Some("user"), Some(&b"pa:ss"[..]))
    );
    let will = c.will.expect("Will (REQ-LIFE-008)");
    assert!(will.topic.starts_with("Oi4v2/Utility/") && will.topic.contains("/Pub/Health/"));
    assert_eq!(health(&will.payload), ("FAILURE_1".to_owned(), 0));

    let (_, birth) = tb.publish();
    assert!(birth.topic.contains("/Pub/MAM/"), "{}", birth.topic);
    let mam = json(&birth.payload);
    assert_eq!(mam["Messages"][0]["DataSetWriterId"], 10);
    assert_eq!(
        mam["Messages"][0]["DataSetWriterName"].as_str().unwrap(),
        il
    );
    assert!(!birth.retain);
    let (_, h) = tb.publish();
    assert_eq!(h.topic, will.topic);
    assert_eq!(health(&h.payload), ("NORMAL_0".to_owned(), 100));
    assert_eq!(json(&h.payload)["Messages"][0]["DataSetWriterId"], 11);
    states.wait_for(ConnectionState::Online, 1);
    assert_eq!(handle.state(), ConnectionState::Online);

    // keep alive: PINGREQ within 1 s, the connection stays up
    let start = Instant::now();
    while start.elapsed() < Duration::from_millis(2500) {
        if let (_, Packet::PingReq) = tb.packet() {
            break;
        }
    }
    assert_eq!(handle.state(), ConnectionState::Online);

    handle.shutdown();
    let rest = tb.until_closed(0);
    let tail: Vec<_> = rest
        .iter()
        .filter(|p| !matches!(p, Packet::PingReq))
        .collect();
    match tail.as_slice() {
        [Packet::Publish(close), Packet::Disconnect] => {
            assert_eq!(close.topic, will.topic);
            assert_eq!(health(&close.payload), ("NORMAL_0".to_owned(), 0));
        }
        other => panic!("expected Close, DISCONNECT; got {other:?}"),
    }
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [
            ConnectionState::Connecting,
            ConnectionState::Online,
            ConnectionState::Closed
        ]
    );
    assert_eq!(handle.state(), ConnectionState::Closed);
}

/// A lost connection: Reconnecting, a new CONNECT with a new Will
/// (REQ-LIFE-016), Birth again (REQ-LIFE-001).
#[test]
fn reconnect_after_broker_drop() {
    let (pki, root) = (pki(), Root::new("rt-reconnect"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let run = spawn(app);

    let (_, first) = tb.connect();
    let (_, birth) = tb.publish();
    assert!(birth.topic.contains("/Pub/MAM/"));
    states.wait_for(ConnectionState::Online, 1);
    tb.drop_connection(0);
    tb.until_closed(0);
    states.wait_for(ConnectionState::Reconnecting, 1);

    // backoff 0.5 … 1 s
    let (n, second) = tb.connect();
    assert_eq!(n, 1);
    let (w1, w2) = (first.will.unwrap(), second.will.unwrap());
    assert_eq!(w1.topic, w2.topic);
    assert_ne!(message_id(&w1.payload), message_id(&w2.payload));
    let (_, birth2) = tb.publish();
    assert_eq!(birth2.topic, birth.topic);
    assert_ne!(message_id(&birth.payload), message_id(&birth2.payload));
    states.wait_for(ConnectionState::Online, 2);

    handle.shutdown();
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [
            ConnectionState::Connecting,
            ConnectionState::Online,
            ConnectionState::Reconnecting,
            ConnectionState::Online,
            ConnectionState::Closed
        ]
    );
}

/// No PINGRESP: the client closes without DISCONNECT, so the broker would
/// publish the Will (REQ-LIFE-008, REQ-MQTT-014), and reconnects.
#[test]
fn keep_alive_timeout_closes_without_disconnect() {
    let (pki, root) = (pki(), Root::new("rt-keepalive"));
    let tb = TestBroker::start(
        &pki,
        Behavior {
            ping: false,
            ..Behavior::default()
        },
    );
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let run = spawn(app);

    tb.connect();
    let packets = tb.until_closed(0);
    assert!(packets.iter().any(|p| matches!(p, Packet::PingReq)));
    assert!(
        !packets.iter().any(|p| matches!(p, Packet::Disconnect)),
        "{packets:?}"
    );
    assert_eq!(tb.connect().0, 1);
    states.wait_for(ConnectionState::Online, 2);
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// A refused CONNECT is retried with backoff; the state stays Connecting
/// until the first CONNACK accepts (ADR-0020 §5).
#[test]
fn refused_connect_is_retried() {
    let (pki, root) = (pki(), Root::new("rt-refused"));
    let tb = TestBroker::start(
        &pki,
        Behavior {
            first_connack: ConnectReturnCode::NotAuthorized,
            ..Behavior::default()
        },
    );
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let run = spawn(app);

    assert_eq!(tb.connect().0, 0);
    tb.until_closed(0);
    assert_eq!(tb.connect().0, 1);
    assert!(tb.publish().1.topic.contains("/Pub/MAM/"));
    states.wait_for(ConnectionState::Online, 1);
    handle.shutdown();
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [
            ConnectionState::Connecting,
            ConnectionState::Online,
            ConnectionState::Closed
        ]
    );
}

/// A broker certificate of another CA is never trusted (REQ-MQTT-008):
/// no CONNECT reaches it, the state stays Connecting, shutdown during the
/// backoff ends `run`.
#[test]
fn untrusted_broker_is_never_online() {
    let (pki, other, root) = (pki(), pki(), Root::new("rt-untrusted"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder(broker(&root, &other, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let run = spawn(app);

    assert!(matches!(tb.next(), Seen::Closed(0)));
    assert!(matches!(tb.next(), Seen::Closed(1)));
    handle.shutdown();
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [ConnectionState::Connecting, ConnectionState::Closed]
    );
}

/// Mutual TLS with the client certificate of an explicit broker: no User
/// Name in CONNECT (REQ-MQTT-010); TLS 1.2 is enough (REQ-MQTT-005).
#[test]
fn client_certificate_over_tls12() {
    let (pki, root) = (pki(), Root::new("rt-mtls"));
    let tb = TestBroker::start(
        &pki,
        Behavior {
            mutual_tls: true,
            tls12_only: true,
            ..Behavior::default()
        },
    );
    root.write("cert.pem", &pki.client_cert)
        .write("key.pem", &pki.client_key.serialize_pem());
    let states = States::default();
    let broker =
        broker(&root, &pki, tb.port).client_cert(root.path("cert.pem"), root.path("key.pem"));
    let app = builder(broker, &states).build().unwrap();
    let handle = app.handle();
    let run = spawn(app);

    assert!(matches!(tb.next(), Seen::Tls(0, true)));
    let (_, c) = tb.connect();
    assert_eq!((c.username, c.password), (None, None));
    states.wait_for(ConnectionState::Online, 1);
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Container storage with an encrypted private key and its passphrase
/// (REQ-CNT-008, REQ-CNT-009).
#[test]
fn encrypted_private_key_with_passphrase() {
    let pki = pki();
    let tb = TestBroker::start(
        &pki,
        Behavior {
            mutual_tls: true,
            ..Behavior::default()
        },
    );
    let root = container(&pki, tb.port, "rt-passphrase");
    root.write(
        "run/secrets/oi4/private_key.pem",
        &encrypted_pem(&pki.client_key, "s3cret"),
    )
    .write("run/secrets/oi4/passphrase", "czNjcmV0\n");
    let states = States::default();
    let app = container_app(&root, &states).build().unwrap();
    let handle = app.handle();
    let run = spawn(app);

    assert!(matches!(tb.next(), Seen::Tls(0, true)));
    assert_eq!(tb.connect().1.client_id, "app-1");
    states.wait_for(ConnectionState::Online, 1);
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Unusable TLS material fails `run` before the first connection.
#[test]
fn unusable_tls_material_fails_run() {
    let pki = pki();
    let encrypted = encrypted_pem(&pki.client_key, "s3cret");
    let bootstrap = |root: &Root, needle: &str| match container_app(root, &States::default())
        .build()
        .unwrap()
        .run()
    {
        Err(oec::Error::Bootstrap(m)) => assert!(m.contains(needle), "{m}"),
        other => panic!("expected Error::Bootstrap with {needle}, got {other:?}"),
    };
    // wrong passphrase
    let root = container(&pki, 1, "rt-wrong-passphrase");
    root.write("run/secrets/oi4/private_key.pem", &encrypted)
        .write("run/secrets/oi4/passphrase", "d3Jvbmc=");
    bootstrap(&root, "passphrase");
    // encrypted, but no passphrase
    root.remove("run/secrets/oi4/passphrase");
    bootstrap(&root, "passphrase");
    // legacy OpenSSL encryption (RFC 1421) is not supported
    root.write(
        "run/secrets/oi4/private_key.pem",
        "-----BEGIN EC PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-256-CBC,00\n\nAA==\n-----END EC PRIVATE KEY-----\n",
    );
    bootstrap(&root, "PKCS#8");
    // a CA file without certificate
    let root = container(&pki, 1, "rt-bad-ca");
    root.write("etc/oi4/certs/ca.pem", "no certificate");
    bootstrap(&root, "ca.pem");
}

/// A container storage for `app-1` with the CA and client certificate of
/// `pki` and the broker on `port`.
fn container(pki: &Pki, port: u16, name: &str) -> Root {
    let root = Root::container(name);
    root.write(
        "etc/oi4/mqtt/broker.json",
        &format!(r#"{{"Address": "localhost", "SecurePort": {port}, "MaxPacketSize": 262144}}"#),
    )
    .write("etc/oi4/certs/ca.pem", &pki.ca)
    .write("etc/oi4/certs/app-1.pem", &pki.client_cert)
    .write(
        "run/secrets/oi4/private_key.pem",
        &pki.client_key.serialize_pem(),
    );
    root.remove("run/secrets/oi4/credentials");
    root
}

fn container_app(root: &Root, states: &States) -> oec::app::AppBuilder {
    let states = states.clone();
    App::builder(Level::L1, ServiceType::Utility, product())
        .bootstrap(root.bootstrap())
        .keep_alive(Duration::from_secs(1))
        .on_connection(move |s| states.0.lock().unwrap().push(s))
}

/// Drops connection `n` and returns the CONNECT of the reconnect.
fn reconnect(tb: &TestBroker, n: usize) -> Connect {
    tb.drop_connection(n);
    tb.until_closed(n);
    let (m, c) = tb.connect();
    assert_eq!(m, n + 1);
    c
}

/// The platform renews the secret storage while the application runs
/// (GL-ADR-022): every reconnect reads `/run/secrets/oi4` again; unusable
/// secrets keep the last good ones; a private key with the certificate of
/// the application wins over the credentials (REQ-MQTT-010).
#[test]
fn renewed_secrets_apply_on_reconnect() {
    let pki = pki();
    let tb = TestBroker::start(&pki, Behavior::default());
    let root = container(&pki, tb.port, "rt-renew");
    root.remove("run/secrets/oi4/private_key.pem")
        .write("run/secrets/oi4/credentials", "dXNlcjpvbmU=\n");
    let states = States::default();
    let app = container_app(&root, &states).build().unwrap();
    let handle = app.handle();
    let run = spawn(app);
    let password = |c: &Connect| c.password.clone().map(|p| String::from_utf8(p).unwrap());

    let (_, c) = tb.connect();
    assert_eq!(
        (c.username.as_deref(), password(&c).as_deref()),
        (Some("user"), Some("one"))
    );
    root.write("run/secrets/oi4/credentials", "dXNlcjp0d28=\n");
    let c = reconnect(&tb, 0);
    assert_eq!(
        (c.username.as_deref(), password(&c).as_deref()),
        (Some("user"), Some("two"))
    );
    // unusable: the last good credentials stay
    root.write("run/secrets/oi4/credentials", "not base64");
    let c = reconnect(&tb, 1);
    assert_eq!(password(&c).as_deref(), Some("two"));
    root.remove("run/secrets/oi4/credentials");
    let c = reconnect(&tb, 2);
    assert_eq!(password(&c).as_deref(), Some("two"));
    // a private key with the certificate wins over the credentials
    root.write("run/secrets/oi4/credentials", "dXNlcjp0d28=\n")
        .write(
            "run/secrets/oi4/private_key.pem",
            &pki.client_key.serialize_pem(),
        );
    let c = reconnect(&tb, 3);
    assert_eq!((c.username, c.password), (None, None));
    states.wait_for(ConnectionState::Online, 5);
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// `shutdown` before `run`: no connection, `run` returns at once.
#[test]
fn shutdown_before_run() {
    let (pki, root) = (pki(), Root::new("rt-early"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    app.shutdown();
    app.run().unwrap();
    assert!(matches!(
        tb.seen.recv_timeout(Duration::from_millis(200)),
        Err(RecvTimeoutError::Timeout)
    ));
    assert!(states.get().iter().all(|s| *s == ConnectionState::Closed));
}

/// `run_async` is a `Send` future for an existing runtime; it installs no
/// signal handlers (ADR-0020 amendment C-050).
#[test]
fn run_async_on_a_spawned_task() {
    let (pki, root) = (pki(), Root::new("rt-async"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder(broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let handle = app.handle();
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .unwrap();
    let task = rt.spawn(app.run_async());
    let waiter = thread::spawn(move || rt.block_on(task));
    tb.connect();
    states.wait_for(ConnectionState::Online, 1);
    handle.shutdown();
    waiter.join().unwrap().unwrap().unwrap();
    assert_eq!(handle.state(), ConnectionState::Closed);
}

/// The broker is unreachable (nothing listens): Connecting, retries, no
/// error.
#[test]
fn unreachable_broker_keeps_connecting() {
    let (pki, root) = (pki(), Root::new("rt-unreachable"));
    let port = TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port();
    let states = States::default();
    let app = builder(broker(&root, &pki, port), &states).build().unwrap();
    let handle = app.handle();
    let run = spawn(app);
    states.wait_for(ConnectionState::Connecting, 1);
    thread::sleep(Duration::from_millis(1200));
    assert_eq!(handle.state(), ConnectionState::Connecting);
    handle.shutdown();
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [ConnectionState::Connecting, ConnectionState::Closed]
    );
}

#[test]
fn handle_is_clone_send_sync() {
    fn check<T: Clone + Send + Sync + 'static>() {}
    check::<oec::app::AppHandle>();
}

/// Level 2 (ADR-0026 §12/§14): the `Get`, `Set` and `Del` subscriptions,
/// after the Birth the
/// state of every source – the app, then each asset with MAM, Health and
/// Profile, the Profile of the app, the PublicationList with the ids of
/// these publications (REQ-RES-PUBLIST-006) – and the answer to a `Get`
/// with ReplyTo (REQ-CMP-008, REQ-RES-MAM-*).
#[test]
fn level_2_publishes_its_sources_and_answers_get() {
    let (pki, root) = (pki(), Root::new("rt-provider"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    let (me, it) = (
        encoded(app.identification_link().as_str()),
        encoded(pump.identification_link().as_str()),
    );
    let handle = app.handle();
    let run = spawn(app);

    let (n, _) = tb.connect();
    let prefix = format!("Oi4v2/Utility/{me}/");
    let mut subscribed = None;
    let mut seen = Vec::new();
    while seen.len() < 7 {
        match tb.packet() {
            (_, Packet::Subscribe(s)) => subscribed = Some(s),
            (_, Packet::Publish(p)) => seen.push(p),
            (_, Packet::PingReq) => {}
            other => panic!("unexpected {other:?}"),
        }
    }
    let mut filters: Vec<String> = subscribed
        .expect("SUBSCRIBE")
        .filters
        .into_iter()
        .map(|(f, _)| f)
        .collect();
    filters.sort();
    assert_eq!(filters, served_filters(&prefix));
    let topics: Vec<&str> = seen.iter().map(|p| p.topic.as_str()).collect();
    assert_eq!(
        topics,
        [
            format!("{prefix}Pub/MAM/{me}"),
            format!("{prefix}Pub/Health/{me}"),
            format!("{prefix}Pub/MAM/{it}"),
            format!("{prefix}Pub/Health/{it}"),
            format!("{prefix}Pub/Profile/{it}"),
            format!("{prefix}Pub/Profile/{me}"),
            format!("{prefix}Pub/PublicationList"),
        ]
    );
    assert_eq!(writer_ids(&seen), listed_ids(&seen[6]));
    // the Status events of both sources and the 8 Syslog levels of the
    // application are listed, not yet published
    assert_eq!(listed_events(&seen[6]), 14);
    states.wait_for(ConnectionState::Online, 1);

    tb.send(n, get(format!("{prefix}Get/MAM/{it}")));
    let answer = publication(&tb);
    assert_eq!(answer.topic, format!("{prefix}Pub/MAM/{it}"));
    let v = json(&answer.payload);
    assert_eq!(v["ReplyTo"], GET_ID);
    assert_eq!(
        v["Messages"][0]["DataSetWriterName"],
        "vendor.com/Pump/P-1/0815"
    );
    assert_eq!(v["Messages"][0]["Payload"]["HardwareRevision"], "undefined");

    // a Get of a resource that is not served is not subscribed: ignored
    tb.send(n, get(format!("{prefix}Get/SubscriptionList/{it}")));
    tb.send(n, get(format!("{prefix}Get/Profile/{me}")));
    let answer = publication(&tb);
    assert_eq!(answer.topic, format!("{prefix}Pub/Profile/{me}"));
    let resources = &json(&answer.payload)["Messages"][0]["Payload"]["Resources"];
    assert_eq!(
        resources,
        &serde_json::json!([
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

    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Changes through the API are published at once while connected
/// (REQ-TOP-017, REQ-RES-HEALTH-004, REQ-RES-MAM-018,
/// REQ-RES-PUBLIST-017); invalid values publish nothing.
#[test]
fn asset_changes_are_published_at_once() {
    let (pki, root) = (pki(), Root::new("rt-asset-changes"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let me = encoded(app.identification_link().as_str());
    let prefix = format!("Oi4v2/Utility/{me}/");
    let list = format!("{prefix}Pub/PublicationList");
    let handle = app.handle();
    let run = spawn(app);
    tb.connect();
    let first = publications_until(&tb, &list);
    assert_eq!(listed_ids(first.last().unwrap()).len(), 3);
    states.wait_for(ConnectionState::Online, 1);

    let pump = handle
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    let it = encoded(pump.identification_link().as_str());
    let added = publications_until(&tb, &list);
    let topics: Vec<&str> = added.iter().map(|p| p.topic.as_str()).collect();
    assert_eq!(
        topics,
        [
            format!("{prefix}Pub/MAM/{it}"),
            format!("{prefix}Pub/Health/{it}"),
            format!("{prefix}Pub/Profile/{it}"),
            list.clone(),
        ]
    );
    assert_eq!(health(&added[1].payload), ("NORMAL_0".to_owned(), 100));
    assert_eq!(listed_ids(&added[3]).len(), 6);

    // invalid: ignored; the next publication is the valid change
    handle.this().set_health(HealthState::Normal, Some(0));
    pump.set_health(HealthState::MaintenanceRequired, Some(80));
    let h = publication(&tb);
    assert_eq!(h.topic, format!("{prefix}Pub/Health/{it}"));
    assert_eq!(
        health(&h.payload),
        ("MAINTENANCE_REQUIRED_4".to_owned(), 80)
    );

    pump.set_health_details(vec![
        HealthDetail::new(HealthState::OffSpec),
        HealthDetail {
            description: Some("bearing".to_owned()),
            ..HealthDetail::new(HealthState::Failure)
        },
    ]);
    let h = json(&publication(&tb).payload);
    let p = &h["Messages"][0]["Payload"];
    assert_eq!(p["Health"], "FAILURE_1");
    assert_eq!(p["Details"].as_array().unwrap().len(), 2);
    assert!(p.get("HealthScore").is_none());

    pump.bump_revision_counter().unwrap();
    let m = publication(&tb);
    assert_eq!(m.topic, format!("{prefix}Pub/MAM/{it}"));
    assert_eq!(
        json(&m.payload)["Messages"][0]["Payload"]["RevisionCounter"],
        1
    );

    pump.remove().unwrap();
    let removed = publication(&tb);
    assert_eq!(removed.topic, list);
    assert_eq!(listed_ids(&removed), listed_ids(first.last().unwrap()));

    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// Offline nothing is queued (ADR-0020 §7): changes while disconnected are
/// published once, as the current state, after the reconnect.
#[test]
fn offline_changes_are_published_as_state_after_reconnect() {
    let (pki, root) = (pki(), Root::new("rt-offline"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L1, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let me = encoded(app.identification_link().as_str());
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    let it = encoded(pump.identification_link().as_str());
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = spawn(app);
    tb.connect();
    // Level 1: no SUBSCRIBE, MAM and Health of the app and the asset
    let topics: Vec<String> = (0..4).map(|_| tb.publish().1.topic).collect();
    assert_eq!(
        topics,
        [
            format!("{prefix}Pub/MAM/{me}"),
            format!("{prefix}Pub/Health/{me}"),
            format!("{prefix}Pub/MAM/{it}"),
            format!("{prefix}Pub/Health/{it}"),
        ]
    );
    states.wait_for(ConnectionState::Online, 1);
    tb.drop_connection(0);
    tb.until_closed(0);
    states.wait_for(ConnectionState::Reconnecting, 1);

    for (h, score) in [
        (HealthState::OffSpec, 50),
        (HealthState::MaintenanceRequired, 70),
    ] {
        pump.set_health(h, Some(score));
    }
    tb.connect();
    let after: Vec<Publish> = (0..4).map(|_| tb.publish().1).collect();
    assert_eq!(after[3].topic, format!("{prefix}Pub/Health/{it}"));
    assert_eq!(
        health(&after[3].payload),
        ("MAINTENANCE_REQUIRED_4".to_owned(), 70)
    );
    handle.shutdown();
    let rest = tb.until_closed(1);
    assert!(
        rest.iter()
            .filter(|p| matches!(p, Packet::Publish(p) if p.topic.contains(&it)))
            .count()
            == 0,
        "{rest:?}"
    );
    run.join().unwrap().unwrap();
}

/// A `Get Metadata` for Source and Tag (REQ-RES-META-002).
fn get_meta(topic: String, source: &str, tag: &str) -> Packet {
    Packet::Publish(Publish {
        topic,
        payload: serde_json::to_vec(&serde_json::json!({
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
        .unwrap(),
        delivery: Delivery::AtMostOnce,
        retain: false,
    })
}

/// The next PUBLISH on `topic`, collecting the others.
fn next_on(tb: &TestBroker, topic: &str, others: &mut Vec<Publish>) -> Publish {
    loop {
        let p = publication(tb);
        if p.topic == topic {
            return p;
        }
        others.push(p);
    }
}

/// The DataSet of a Data publication.
fn data_payload(p: &Publish) -> Value {
    json(&p.payload)["Messages"][0]["Payload"].clone()
}

/// A pump with the DataSet `oee` (Interval 0, a Precision) and the app's
/// `Oi4Pv` (SOURCE, `interval`), both with values.
fn data_sets(app: &App, interval: Duration) -> (Asset, DataSet, DataSet) {
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    let oee = pump
        .add_data(
            DataSetDef::new("oee")
                .field(Field::double("Availability").unit("%"))
                .field(Field::new("Count", BuiltinType::UInt32))
                .publication(Publication::default_data().precision("Availability", 0.5)),
        )
        .unwrap();
    oee.update([
        ("Availability", oec::data::Value::from(97.5)),
        ("Count", 1u32.into()),
    ])
    .unwrap();
    let pv = app
        .this()
        .add_data(
            DataSetDef::oi4pv(Field::double("T")).publication(
                Publication::default_data()
                    .mode(Mode::ON_REQUEST | Mode::SOURCE)
                    .interval(interval),
            ),
        )
        .unwrap();
    pv.update([("Pv", 20.0)]).unwrap();
    (pump, oee, pv)
}

/// `(Resource, Mode)` of the Data and Metadata entries of a
/// PublicationList publication.
fn data_entries(list: &Publish) -> Vec<(String, String)> {
    json(&list.payload)["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| &m["Payload"])
        .filter(|e| e["Resource"] == "Data" || e["Resource"] == "Metadata")
        .map(|e| {
            (
                e["Resource"].as_str().unwrap().to_owned(),
                e["Mode"].as_str().unwrap().to_owned(),
            )
        })
        .collect()
}

/// Data DataSets (P4 increment 6b): the Metadata before the Data
/// (REQ-RES-META-003), the PublicationList entries (REQ-RES-PUBLIST-001 …
/// 012), an Interval publishing cyclically (REQ-RES-PUBLIST-013/014), the
/// Precisions as dead band at Interval 0 (REQ-RES-PUBLIST-015,
/// REQ-MQTT-019), `Get Data`/`Get Metadata` (REQ-RES-DATA-007,
/// REQ-RES-META-002) and the removal (REQ-RES-PUBLIST-017).
#[test]
fn data_sets_publish_their_state() {
    const INTERVAL: Duration = Duration::from_millis(50);
    let (pki, root) = (pki(), Root::new("rt-data"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let (pump, oee, _pv) = data_sets(&app, INTERVAL);
    let (me, it) = (
        encoded(app.identification_link().as_str()),
        encoded(pump.identification_link().as_str()),
    );
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = spawn(app);
    let (n, _) = tb.connect();

    // the state after the connect: Metadata before Data
    let (pv_topic, oee_topic) = (
        format!("{prefix}Pub/Data/{me}"),
        format!("{prefix}Pub/Data/{it}/oee"),
    );
    let mut seen = Vec::new();
    let first_pv = next_on(&tb, &pv_topic, &mut seen);
    let first_oee = if seen.iter().any(|p| p.topic == oee_topic) {
        seen.iter().find(|p| p.topic == oee_topic).unwrap().clone()
    } else {
        next_on(&tb, &oee_topic, &mut seen)
    };
    let topics: Vec<&str> = seen.iter().map(|p| p.topic.as_str()).collect();
    for meta in [
        format!("{prefix}Pub/Metadata/{me}/Oi4Pv"),
        format!("{prefix}Pub/Metadata/{it}/oee"),
    ] {
        assert!(topics.contains(&meta.as_str()), "{meta} not in {topics:?}");
    }
    assert_eq!(data_payload(&first_pv), serde_json::json!({"Pv": 20.0}));
    assert_eq!(
        data_payload(&first_oee),
        serde_json::json!({"Availability": 97.5, "Count": 1})
    );
    let list = seen
        .iter()
        .find(|p| p.topic == format!("{prefix}Pub/PublicationList"))
        .expect("PublicationList");
    let entries = json(&list.payload)["Messages"].as_array().unwrap().len();
    assert_eq!(
        data_entries(list),
        [
            ("Data", "ON_REQUEST_SOURCE_10"),
            ("Metadata", "ON_REQUEST_FILTER_9"),
            ("Data", "ON_REQUEST_FILTER_9"),
            ("Metadata", "ON_REQUEST_FILTER_9"),
        ]
        .map(|(r, m)| (r.to_owned(), m.to_owned()))
    );
    states.wait_for(ConnectionState::Online, 1);

    // the Interval: cyclic, unchanged values included
    let start = Instant::now();
    for _ in 0..3 {
        let p = next_on(&tb, &pv_topic, &mut Vec::new());
        assert_eq!(data_payload(&p), serde_json::json!({"Pv": 20.0}));
    }
    assert!(start.elapsed() >= INTERVAL * 2, "{:?}", start.elapsed());

    // the dead band: 97.7 is within 0.5 of 97.5, 98.2 is not
    oee.update([("Availability", 97.7)]).unwrap();
    oee.update([("Availability", 98.2)]).unwrap();
    let p = next_on(&tb, &oee_topic, &mut Vec::new());
    assert_eq!(data_payload(&p)["Availability"], 98.2);

    // Get Data and Get Metadata of the pump's DataSet
    tb.send(n, get(format!("{prefix}Get/Data/{it}/oee")));
    let answer = next_on(&tb, &oee_topic, &mut Vec::new());
    assert_eq!(json(&answer.payload)["ReplyTo"], GET_ID);
    assert_eq!(data_payload(&answer)["Availability"], 98.2);
    let meta_topic = format!("{prefix}Pub/Metadata/{it}/oee");
    tb.send(
        n,
        get_meta(
            format!("{prefix}Get/Metadata/{it}/oee"),
            pump.identification_link().as_str(),
            "oee",
        ),
    );
    let answer = json(&next_on(&tb, &meta_topic, &mut Vec::new()).payload);
    assert_eq!(answer["ReplyTo"], GET_ID);
    let fields: Vec<&str> = answer["MetaData"]["Fields"]
        .as_array()
        .unwrap()
        .iter()
        .map(|f| f["Name"].as_str().unwrap())
        .collect();
    assert_eq!(fields, ["Availability", "Count"]);

    // removed: the PublicationList without its entries
    oee.remove().unwrap();
    let list = next_on(
        &tb,
        &format!("{prefix}Pub/PublicationList"),
        &mut Vec::new(),
    );
    assert_eq!(
        json(&list.payload)["Messages"].as_array().unwrap().len(),
        entries - 2
    );

    handle.shutdown();
    run.join().unwrap().unwrap();
}

// ------------------------------------------------- write side (C-055)

/// A `Set` NetworkMessage from a consumer with one DataSetMessage for
/// `source`, QoS 0.
fn set(topic: String, source: &str, payload: &Value) -> Packet {
    Packet::Publish(Publish {
        topic,
        payload: serde_json::to_vec(&serde_json::json!({
            "MessageId": GET_ID,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "DataSetClassId": "",
            "ReplyTo": "",
            "Messages": [{
                "DataSetWriterId": 20,
                "DataSetWriterName": source,
                "Payload": payload
            }]
        }))
        .unwrap(),
        delivery: Delivery::AtMostOnce,
        retain: false,
    })
}

/// The next publication on `topic` that answers the request [`GET_ID`].
fn answer_on(tb: &TestBroker, topic: &str) -> Value {
    loop {
        let v = json(&next_on(tb, topic, &mut Vec::new()).payload);
        if v["ReplyTo"] == GET_ID {
            return v;
        }
    }
}

/// The Payload of a Status event on `prefix` + `Pub/Event/<source>/Status/<level>`
/// answering the request [`GET_ID`].
fn status_event(tb: &TestBroker, prefix: &str, source: &str, level: &str) -> Value {
    let topic = format!("{prefix}Pub/Event/{source}/Status/{level}");
    answer_on(tb, &topic)["Messages"][0]["Payload"].clone()
}

/// The pump with the DataSet `oee`, writable Mode and Interval.
fn writable_pump(app: &App) -> (Asset, DataSet) {
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
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
    (pump, oee)
}

/// Runs a Level 2 app with the writable pump on `store` until Online and
/// returns the PublicationList entry of `oee` and the running app.
fn run_with_store(
    pki: &Pki,
    root: &Root,
    store: &oec::store::FileStore,
) -> (
    TestBroker,
    usize,
    String,
    String,
    AppHandle,
    thread::JoinHandle<oec::Result<()>>,
) {
    let tb = TestBroker::start(pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(root, pki, tb.port), &states)
        .store(store.clone())
        .build()
        .unwrap();
    let (pump, _) = writable_pump(&app);
    let (me, it) = (
        encoded(app.identification_link().as_str()),
        encoded(pump.identification_link().as_str()),
    );
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = spawn(app);
    let (n, _) = tb.connect();
    publications_until(&tb, &format!("{prefix}Pub/PublicationList"));
    states.wait_for(ConnectionState::Online, 1);
    (tb, n, prefix, it, handle, run)
}

/// `Set PublicationList` (REQ-RES-PUBLIST-016/024/025): the changed entry
/// with ReplyTo and the Status event Good, the change persisted in the
/// store and applied after a restart; a read-only element is answered with
/// `BadNotWritable`.
#[test]
fn set_publication_list_is_answered_and_persisted() {
    let (pki, root) = (pki(), Root::new("rt-set-publist"));
    let store = oec::store::FileStore::new(root.path("store"));
    let (tb, n, prefix, it, handle, run) = run_with_store(&pki, &root, &store);
    let rest = format!("PublicationList/{it}/Data/oee");
    tb.send(n, get(format!("{prefix}Get/{rest}")));
    let pub_topic = format!("{prefix}Pub/{rest}");
    let mut e = answer_on(&tb, &pub_topic)["Messages"][0]["Payload"].clone();
    assert_ne!(e["Interval"], 3_600_000);
    e["Interval"] = serde_json::json!(3_600_000);
    e["Precisions"] = serde_json::json!({"Availability": 0.5});
    let pump_il = "vendor.com/Pump/P-1/0815";
    tb.send(n, set(format!("{prefix}Set/{rest}"), pump_il, &e));
    let answer = answer_on(&tb, &pub_topic);
    assert_eq!(answer["Messages"][0]["Payload"], e);
    let good = status_event(&tb, &prefix, &it, "Good");
    assert_eq!(good["Number"], 0);

    // read only: the DataSetWriterId
    let mut forbidden = e.clone();
    forbidden["DataSetWriterId"] = serde_json::json!(999);
    tb.send(n, set(format!("{prefix}Set/{rest}"), pump_il, &forbidden));
    let bad = status_event(&tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadNotWritable.code());
    handle.shutdown();
    run.join().unwrap().unwrap();
    drop(tb);

    // restarted on the same store: the changed settings apply
    let (tb, n, prefix, it2, handle, run) = run_with_store(&pki, &root, &store);
    assert_eq!(it2, it);
    tb.send(n, get(format!("{prefix}Get/{rest}")));
    let after = answer_on(&tb, &pub_topic)["Messages"][0]["Payload"].clone();
    assert_eq!(after, e);
    handle.shutdown();
    run.join().unwrap().unwrap();
}

/// `Set Data` (REQ-RES-DATA-009/011/012, REQ-CMP-043): the handler
/// decides; on `Ok` the DataSet with the ReplyTo and Good, on `Err` the
/// Status event with that code; without handler `BadNotWritable`; a
/// missing field `BadInvalidArgument` without calling the handler. A `Get`
/// for an unknown Source: `BadNotFound` under the AppId (REQ-CMP-047,
/// SF-108).
#[test]
fn set_data_is_decided_by_the_handler() {
    let (pki, root) = (pki(), Root::new("rt-set-data"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let (pump, oee) = writable_pump(&app);
    let calls = Arc::new(Mutex::new(Vec::new()));
    let seen = calls.clone();
    oee.on_set(move |s| {
        let fields: Vec<String> = s.fields().map(|(n, _)| n.to_owned()).collect();
        seen.lock().unwrap().push(fields);
        match s.get("Count") {
            Some(oec::data::Value::UInt32(c)) if *c <= 100 => Ok(()),
            _ => Err(StatusCode::BadOutOfRange),
        }
    })
    .unwrap();
    pump.add_data(DataSetDef::new("idle").field(Field::double("X")))
        .unwrap();
    let (me, it) = (
        encoded(app.identification_link().as_str()),
        encoded(pump.identification_link().as_str()),
    );
    let pump_il = pump.identification_link().as_str().to_owned();
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = spawn(app);
    let (n, _) = tb.connect();
    publications_until(&tb, &format!("{prefix}Pub/PublicationList"));
    states.wait_for(ConnectionState::Online, 1);

    let data = format!("{prefix}Pub/Data/{it}/oee");
    let request = serde_json::json!({"Availability": 50.0, "Count": 7, "Unknown": true});
    tb.send(
        n,
        set(format!("{prefix}Set/Data/{it}/oee"), &pump_il, &request),
    );
    let answer = answer_on(&tb, &data);
    assert_eq!(
        answer["Messages"][0]["Payload"],
        serde_json::json!({"Availability": 50.0, "Count": 7})
    );
    assert_eq!(status_event(&tb, &prefix, &it, "Good")["Number"], 0);
    assert_eq!(
        oee.values().unwrap(),
        [oec::data::Value::from(50.0), 7u32.into()]
    );

    let request = serde_json::json!({"Availability": 50.0, "Count": 101});
    tb.send(
        n,
        set(format!("{prefix}Set/Data/{it}/oee"), &pump_il, &request),
    );
    let bad = status_event(&tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadOutOfRange.code());
    assert_eq!(oee.values().unwrap()[1], 7u32.into());

    // checked by the SDK: the handler is not called
    let request = serde_json::json!({"Availability": 50.0});
    tb.send(
        n,
        set(format!("{prefix}Set/Data/{it}/oee"), &pump_il, &request),
    );
    let bad = status_event(&tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadInvalidArgument.code());
    assert_eq!(calls.lock().unwrap().len(), 2);
    assert_eq!(calls.lock().unwrap()[0], ["Availability", "Count"]);

    tb.send(
        n,
        set(
            format!("{prefix}Set/Data/{it}/idle"),
            &pump_il,
            &serde_json::json!({"X": 1.0}),
        ),
    );
    let bad = status_event(&tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadNotWritable.code());

    // a Source that is no asset: answered under the AppId (SF-108)
    let unknown = encoded("vendor.com/Pump/P-1/4711");
    tb.send(n, get(format!("{prefix}Get/MAM/{unknown}")));
    let bad = status_event(&tb, &prefix, &me, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadNotFound.code());

    handle.shutdown();
    run.join().unwrap().unwrap();
}

// ------------------------------------------------- Config (C-056)

/// The Config of the service: an interval with an own rule of the
/// application (multiples of 10), a sensitive API key.
fn service_config(app: &App, calls: &Arc<Mutex<Vec<Vec<String>>>>) -> oec::config::ConfigHandle {
    let config = app
        .this()
        .add_config(
            ConfigDef::new("service").group(
                "common",
                "Common settings",
                [
                    ConfigObject::number("interval", 60.0)
                        .unit("s")
                        .min(10.0)
                        .max(3600.0)
                        .mandatory(),
                    ConfigObject::string("api_key", "").sensitive(),
                ],
            ),
        )
        .unwrap();
    let seen = calls.clone();
    config
        .on_set(move |proposed| {
            let changed = proposed.changed().map(|(g, n)| format!("{g}/{n}"));
            seen.lock().unwrap().push(changed.collect());
            match proposed.number("common", "interval") {
                Some(i) if i % 10.0 != 0.0 => Err(StatusCode::BadOutOfRange),
                _ => Ok(()),
            }
        })
        .unwrap();
    config
}

/// The Payload of a `Set Config` of the group `common`.
fn common(interval: &str, api_key: Option<&str>) -> Value {
    let mut group = serde_json::json!({"interval": {"Value": interval}});
    if let Some(key) = api_key {
        group["api_key"] = serde_json::json!({"Value": key});
    }
    serde_json::json!({ "common": group })
}

/// A running Level 2 app with the service Config.
struct ConfigRun {
    tb: TestBroker,
    n: usize,
    prefix: String,
    me: String,
    config: oec::config::ConfigHandle,
    handle: AppHandle,
    run: thread::JoinHandle<oec::Result<()>>,
}

impl ConfigRun {
    /// Runs the app on `store` until Online.
    fn start(
        pki: &Pki,
        root: &Root,
        store: &oec::store::FileStore,
        calls: &Arc<Mutex<Vec<Vec<String>>>>,
    ) -> Self {
        let tb = TestBroker::start(pki, Behavior::default());
        let states = States::default();
        let app = builder_at(Level::L2, broker(root, pki, tb.port), &states)
            .store(store.clone())
            .build()
            .unwrap();
        let config = service_config(&app, calls);
        let me = encoded(app.identification_link().as_str());
        let prefix = format!("Oi4v2/Utility/{me}/");
        let handle = app.handle();
        let run = spawn(app);
        let (n, _) = tb.connect();
        publications_until(&tb, &format!("{prefix}Pub/PublicationList"));
        states.wait_for(ConnectionState::Online, 1);
        Self {
            tb,
            n,
            prefix,
            me,
            config,
            handle,
            run,
        }
    }

    fn stop(self) {
        self.handle.shutdown();
        self.run.join().unwrap().unwrap();
    }
}

/// Refused and accepted `Set Config` of [`config_is_got_set_and_persisted`].
fn decided_sets(r: &ConfigRun, calls: &Arc<Mutex<Vec<Vec<String>>>>) {
    let (tb, n, prefix, me) = (&r.tb, r.n, r.prefix.as_str(), r.me.as_str());
    let il = r.config.source().as_str();
    let set_topic = format!("{prefix}Set/Config/{me}/service");
    // the application's own rule: refused, nothing applied
    tb.send(n, set(set_topic.clone(), il, &common("25", None)));
    let bad = status_event(tb, prefix, me, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadOutOfRange.code());
    // checked by the SDK: the handler is not called
    tb.send(n, set(set_topic.clone(), il, &common("twenty", None)));
    let bad = status_event(tb, prefix, me, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadTypeMismatch.code());
    tb.send(
        n,
        set(set_topic.clone(), il, &serde_json::json!({"common": {}})),
    );
    let bad = status_event(tb, prefix, me, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadInvalidArgument.code());
    // the mask keeps the API key
    tb.send(n, set(set_topic, il, &common("30", Some("********"))));
    assert_eq!(status_event(tb, prefix, me, "Good")["Number"], 0);
    assert_eq!(
        *calls.lock().unwrap(),
        [
            vec!["common/api_key".to_owned(), "common/interval".to_owned()],
            vec!["common/interval".to_owned()],
            vec!["common/interval".to_owned()],
        ]
    );
    let values = r.config.values();
    assert_eq!(values.number("common", "interval"), Some(30.0));
    assert_eq!(values.string("common", "api_key"), Some("s3cr3t"));
}

/// `Get`/`Set Config` (REQ-RES-CONFIG-016 … 025, REQ-RES-MAM-018): the
/// handler decides about the checked values; on `Ok` the Config with the
/// ReplyTo, sensitive values masked, then Good and the MAM with the
/// incremented RevisionCounter; the SDK refuses a value of another Type
/// without calling the handler; the mask keeps the API key; a change from
/// the application side is published; the values survive a restart.
#[test]
fn config_is_got_set_and_persisted() {
    let (pki, root) = (pki(), Root::new("rt-config"));
    let store = oec::store::FileStore::new(root.path("store"));
    let calls = Arc::new(Mutex::new(Vec::new()));
    let r = ConfigRun::start(&pki, &root, &store, &calls);
    let (prefix, me) = (r.prefix.clone(), r.me.clone());
    let (tb, n, il) = (&r.tb, r.n, r.config.source().as_str());
    let pub_topic = format!("{prefix}Pub/Config/{me}/service");

    tb.send(n, get(format!("{prefix}Get/Config/{me}/service")));
    let got = answer_on(tb, &pub_topic)["Messages"][0].clone();
    assert_eq!(got["WriterGroupName"], "service");
    assert_eq!(got["Payload"]["common"]["interval"]["Value"], "60");

    tb.send(
        n,
        set(
            format!("{prefix}Set/Config/{me}/service"),
            il,
            &common("20", Some("s3cr3t")),
        ),
    );
    let answer = answer_on(tb, &pub_topic);
    let payload = &answer["Messages"][0]["Payload"]["common"];
    assert_eq!(payload["interval"]["Value"], "20");
    assert_eq!(payload["api_key"]["Value"], "********");
    assert!(!answer.to_string().contains("s3cr3t"));
    assert_eq!(status_event(tb, &prefix, &me, "Good")["Number"], 0);
    let mam = next_on(tb, &format!("{prefix}Pub/MAM/{me}"), &mut Vec::new());
    assert_eq!(data_payload(&mam)["RevisionCounter"], 1);
    assert_eq!(
        r.config.values().string("common", "api_key"),
        Some("s3cr3t")
    );

    decided_sets(&r, &calls);

    // from the application side: published at once, no ReplyTo
    r.config.set("common", "interval", "40").unwrap();
    let p = json(&next_on(tb, &pub_topic, &mut Vec::new()).payload);
    assert_eq!(
        p["Messages"][0]["Payload"]["common"]["interval"]["Value"],
        "40"
    );
    assert_ne!(p["ReplyTo"], GET_ID, "{p}");
    r.stop();

    // restarted on the same store
    let r = ConfigRun::start(&pki, &root, &store, &calls);
    assert_eq!(r.me, me);
    r.tb.send(r.n, get(format!("{prefix}Get/Config/{me}/service")));
    let got = answer_on(&r.tb, &pub_topic)["Messages"][0]["Payload"]["common"].clone();
    assert_eq!(got["interval"]["Value"], "40");
    assert_eq!(got["api_key"]["Value"], "********");
    assert_eq!(
        r.config.values().string("common", "api_key"),
        Some("s3cr3t")
    );
    r.stop();
}

/// A `Del` NetworkMessage from a consumer without DataSetMessage, QoS 0.
fn del(topic: String) -> Packet {
    Packet::Publish(Publish {
        topic,
        payload: serde_json::to_vec(&serde_json::json!({
            "MessageId": GET_ID,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "ReplyTo": "",
            "Messages": []
        }))
        .unwrap(),
        delivery: Delivery::AtMostOnce,
        retain: false,
    })
}

/// A running Level 2 app that offers the ReferenceDesignation, with the
/// pump and a handler of the pump that refuses the Location `+forbidden`.
struct RefDesRun {
    tb: TestBroker,
    n: usize,
    prefix: String,
    it: String,
    pump: Asset,
    handle: AppHandle,
    run: thread::JoinHandle<oec::Result<()>>,
}

impl RefDesRun {
    fn start(pki: &Pki, root: &Root, store: &oec::store::FileStore) -> Self {
        let tb = TestBroker::start(pki, Behavior::default());
        let states = States::default();
        let app = builder_at(Level::L2, broker(root, pki, tb.port), &states)
            .reference_designation(true)
            .store(store.clone())
            .build()
            .unwrap();
        let pump = app
            .add_asset(
                AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                    .unwrap(),
            )
            .unwrap();
        pump.on_reference_designation(|rd| match rd.and_then(|r| r.location.as_ref()) {
            Some(l) if l.value == "+forbidden" => Err(StatusCode::BadOutOfRange),
            _ => Ok(()),
        })
        .unwrap();
        let me = encoded(app.identification_link().as_str());
        let it = encoded(pump.identification_link().as_str());
        let prefix = format!("Oi4v2/Utility/{me}/");
        let handle = app.handle();
        let run = spawn(app);
        let (n, _) = tb.connect();
        publications_until(&tb, &format!("{prefix}Pub/PublicationList"));
        states.wait_for(ConnectionState::Online, 1);
        Self {
            tb,
            n,
            prefix,
            it,
            pump,
            handle,
            run,
        }
    }

    fn stop(self) {
        self.handle.shutdown();
        self.run.join().unwrap().unwrap();
    }
}

/// `Set`/`Get`/`Del ReferenceDesignation` (REQ-RES-REFDES-002 … 010,
/// REQ-RES-MAM-018): the handler decides; on `Ok` the object with the
/// ReplyTo, then Good and the MAM with the incremented RevisionCounter;
/// `Del` answers Good without publication, a second `Del` `BadNotFound`;
/// the object survives a restart.
#[test]
fn reference_designation_is_set_got_deleted_and_persisted() {
    let (pki, root) = (pki(), Root::new("rt-refdes"));
    let store = oec::store::FileStore::new(root.path("store"));
    let r = RefDesRun::start(&pki, &root, &store);
    let (tb, n, prefix, it) = (&r.tb, r.n, r.prefix.clone(), r.it.clone());
    let il = r.pump.identification_link().as_str().to_owned();
    let (set_t, pub_t) = (
        format!("{prefix}Set/ReferenceDesignation/{it}"),
        format!("{prefix}Pub/ReferenceDesignation/{it}"),
    );
    let rd = serde_json::json!({"Function": {"Value": "=P1.M1"}, "Location": {"Value": "+R1"}});
    tb.send(n, set(set_t.clone(), &il, &rd));
    let answer = answer_on(tb, &pub_t);
    assert_eq!(answer["Messages"][0]["Payload"], rd);
    assert_eq!(status_event(tb, &prefix, &it, "Good")["Number"], 0);
    let mam = next_on(tb, &format!("{prefix}Pub/MAM/{it}"), &mut Vec::new());
    assert_eq!(data_payload(&mam)["RevisionCounter"], 1);
    assert_eq!(
        r.pump
            .reference_designation()
            .unwrap()
            .function
            .unwrap()
            .value,
        "=P1.M1"
    );

    // refused by the handler: nothing applied
    let refused = serde_json::json!({"Location": {"Value": "+forbidden"}});
    tb.send(n, set(set_t.clone(), &il, &refused));
    let bad = status_event(tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadOutOfRange.code());
    tb.send(n, get(format!("{prefix}Get/ReferenceDesignation/{it}")));
    assert_eq!(answer_on(tb, &pub_t)["Messages"][0]["Payload"], rd);
    r.stop();

    // restarted on the same store: still there; then deleted
    let r = RefDesRun::start(&pki, &root, &store);
    let (tb, n) = (&r.tb, r.n);
    assert_eq!(r.it, it);
    tb.send(n, get(format!("{prefix}Get/ReferenceDesignation/{it}")));
    assert_eq!(answer_on(tb, &pub_t)["Messages"][0]["Payload"], rd);
    tb.send(n, del(format!("{prefix}Del/ReferenceDesignation/{it}")));
    assert_eq!(status_event(tb, &prefix, &it, "Good")["Number"], 0);
    assert!(r.pump.reference_designation().is_none());
    tb.send(n, del(format!("{prefix}Del/ReferenceDesignation/{it}")));
    let bad = status_event(tb, &prefix, &it, "Bad");
    assert_eq!(bad["Number"], StatusCode::BadNotFound.code());
    r.stop();
}

/// The entry of `filter` in the PublicationList publication `list`.
fn list_entry(list: &Publish, filter: &str) -> Value {
    json(&list.payload)["Messages"]
        .as_array()
        .unwrap()
        .iter()
        .find(|e| e["Payload"]["Filter"] == filter)
        .cloned()
        .unwrap_or_else(|| panic!("no PublicationList entry {filter}"))
}

/// Generic events (REQ-RES-EVENT-009, REQ-RES-PUBLIST-*): dropped offline
/// (the Level is listed nevertheless), published on
/// `Pub/Event/<Source>/Generic/<Level>` while connected; a new Level
/// republishes the PublicationList with its entry.
#[test]
fn generic_events_are_published_while_connected() {
    let (pki, root) = (pki(), Root::new("rt-generic"));
    let tb = TestBroker::start(&pki, Behavior::default());
    let states = States::default();
    let app = builder_at(Level::L2, broker(&root, &pki, tb.port), &states)
        .build()
        .unwrap();
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    // offline: dropped, not queued
    pump.emit_event(GenericEvent::new("Maintenance", 1))
        .unwrap();
    assert!(matches!(
        pump.emit_event(GenericEvent::new("Maintenance", 2).details_json("[]")),
        Err(oec::Error::Invalid(_))
    ));
    assert!(pump.emit_event(GenericEvent::new("", 2)).is_err());
    let (me, it) = (
        encoded(app.identification_link().as_str()),
        encoded(pump.identification_link().as_str()),
    );
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = spawn(app);
    let (_, _) = tb.connect();
    let list = publications_until(&tb, &format!("{prefix}Pub/PublicationList"));
    assert!(!list.iter().any(|p| p.topic.contains("/Pub/Event/")));
    let entry = list_entry(list.last().unwrap(), "Generic/Maintenance");
    assert_eq!(entry["Payload"]["Mode"], "FILTER_4");
    states.wait_for(ConnectionState::Online, 1);

    pump.emit_event(
        GenericEvent::new("Maintenance", 42)
            .description("filter clogged")
            .details_json(r#"{"Pressure": 3.5}"#),
    )
    .unwrap();
    let event = next_on(
        &tb,
        &format!("{prefix}Pub/Event/{it}/Generic/Maintenance"),
        &mut Vec::new(),
    );
    let m = &json(&event.payload)["Messages"][0];
    assert_eq!(m["Payload"]["Category"], "CAT_GENERIC_99");
    assert_eq!(m["Payload"]["Number"], 42);
    assert_eq!(m["Payload"]["Description"], "filter clogged");
    assert_eq!(m["Payload"]["Details"]["Pressure"], 3.5);
    assert_eq!(entry["Payload"]["DataSetWriterId"], m["DataSetWriterId"]);

    pump.emit_event(GenericEvent::new("Wear", 7)).unwrap();
    let mut others = Vec::new();
    next_on(
        &tb,
        &format!("{prefix}Pub/Event/{it}/Generic/Wear"),
        &mut others,
    );
    let list = match others
        .into_iter()
        .rfind(|p| p.topic.ends_with("/Pub/PublicationList"))
    {
        Some(list) => list,
        None => next_on(
            &tb,
            &format!("{prefix}Pub/PublicationList"),
            &mut Vec::new(),
        ),
    };
    assert_eq!(
        list_entry(&list, "Generic/Wear")["Payload"]["Mode"],
        "FILTER_4"
    );
    handle.shutdown();
    run.join().unwrap().unwrap();
}
