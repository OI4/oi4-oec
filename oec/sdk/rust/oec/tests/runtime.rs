//! `App::run` / `App::run_async` (ADR-0020 §2/§5, ADR-0026 §9/§10) against
//! an in-process MQTT 3.1.1 test broker over TLS: the broker uses the codec
//! of `oec-core` and rustls on blocking threads, the PKI is generated per
//! test with rcgen. Only loopback connections, no network.

mod common;

use std::io::{Read, Write};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::sync::mpsc::{Receiver, RecvTimeoutError, Sender, channel};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::ConnectionState;
use oec::prelude::*;
use oec_core::mqtt::{
    ConnAck, Connect, ConnectReturnCode, Decoder, Delivery, Packet, Publish, SubAck,
    SubscribeReturnCode, encode,
};
use pkcs8::der::Decode;
use rcgen::{
    BasicConstraints, CertificateParams, CertifiedIssuer, DnType, ExtendedKeyUsagePurpose, IsCa,
    KeyPair, KeyUsagePurpose,
};
use rustls::server::WebPkiClientVerifier;
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use rustls_pki_types::pem::PemObject;
use rustls_pki_types::{CertificateDer, PrivateKeyDer};
use serde_json::Value;

use common::{Root, product};

/// Upper bound for every wait of a test.
const WAIT: Duration = Duration::from_secs(15);

// ---------------------------------------------------------------- PKI

/// A CA with a server certificate for `localhost`/127.0.0.1 and a client
/// certificate (PEM).
struct Pki {
    ca: String,
    server_cert: String,
    server_key: String,
    client_cert: String,
    client_key: KeyPair,
}

fn pki() -> Pki {
    let mut params = CertificateParams::new(Vec::<String>::new()).unwrap();
    params.is_ca = IsCa::Ca(BasicConstraints::Unconstrained);
    params
        .distinguished_name
        .push(DnType::CommonName, "OEC test CA");
    params.key_usages = vec![KeyUsagePurpose::KeyCertSign, KeyUsagePurpose::CrlSign];
    let ca = CertifiedIssuer::self_signed(params, KeyPair::generate().unwrap()).unwrap();

    let server_key = KeyPair::generate().unwrap();
    let mut params = CertificateParams::new(vec!["localhost".into(), "127.0.0.1".into()]).unwrap();
    params.extended_key_usages = vec![ExtendedKeyUsagePurpose::ServerAuth];
    let server_cert = params.signed_by(&server_key, &ca).unwrap();

    let client_key = KeyPair::generate().unwrap();
    let mut params = CertificateParams::new(Vec::<String>::new()).unwrap();
    params.distinguished_name.push(DnType::CommonName, "app-1");
    params.extended_key_usages = vec![ExtendedKeyUsagePurpose::ClientAuth];
    let client_cert = params.signed_by(&client_key, &ca).unwrap();

    Pki {
        ca: ca.pem(),
        server_cert: server_cert.pem(),
        server_key: server_key.serialize_pem(),
        client_cert: client_cert.pem(),
        client_key,
    }
}

/// `key` as PKCS#8 `ENCRYPTED PRIVATE KEY` (PBES2: PBKDF2-SHA256 +
/// AES-256-CBC), as written by `openssl pkcs8 -topk8 -v2 aes-256-cbc`.
fn encrypted_pem(key: &KeyPair, passphrase: &str) -> String {
    let der = key.serialize_der();
    let info = pkcs8::PrivateKeyInfoRef::from_der(&der).unwrap();
    let params = pkcs8::pkcs5::pbes2::Parameters::generate_pbkdf2_sha256_aes256cbc(
        2048,
        b"0123456789abcdef",
        [7; 16],
    )
    .unwrap();
    info.encrypt_with_params(params, passphrase)
        .unwrap()
        .to_pem("ENCRYPTED PRIVATE KEY", pkcs8::LineEnding::LF)
        .unwrap()
        .to_string()
}

/// The crypto provider of the SDK build (`crypto-ring` wins).
fn provider() -> Arc<rustls::crypto::CryptoProvider> {
    #[cfg(feature = "crypto-ring")]
    let provider = rustls::crypto::ring::default_provider();
    #[cfg(not(feature = "crypto-ring"))]
    let provider = rustls::crypto::aws_lc_rs::default_provider();
    Arc::new(provider)
}

// ---------------------------------------------------------------- broker

/// What the test broker saw, per connection (numbered from 0).
#[derive(Debug)]
enum Seen {
    /// TLS handshake done; whether the client presented a certificate.
    Tls(usize, bool),
    /// A packet from the client.
    Packet(usize, Packet),
    /// The connection ended (EOF, error or dropped by the test).
    Closed(usize),
}

/// How the broker behaves.
#[derive(Clone, Copy)]
struct Behavior {
    /// Answer of the first CONNECT; later ones are accepted.
    first_connack: ConnectReturnCode,
    /// Answer PINGREQ.
    ping: bool,
    /// Require a client certificate.
    mutual_tls: bool,
    /// Offer TLS 1.2 only.
    tls12_only: bool,
}

impl Default for Behavior {
    fn default() -> Self {
        Self {
            first_connack: ConnectReturnCode::Accepted,
            ping: true,
            mutual_tls: false,
            tls12_only: false,
        }
    }
}

struct TestBroker {
    port: u16,
    seen: Receiver<Seen>,
    sockets: Arc<Mutex<Vec<TcpStream>>>,
}

impl TestBroker {
    fn start(pki: &Pki, behavior: Behavior) -> Self {
        let chain = vec![CertificateDer::from_pem_slice(pki.server_cert.as_bytes()).unwrap()];
        let key = PrivateKeyDer::from_pem_slice(pki.server_key.as_bytes()).unwrap();
        let versions: &[&rustls::SupportedProtocolVersion] = if behavior.tls12_only {
            &[&rustls::version::TLS12]
        } else {
            &[&rustls::version::TLS13, &rustls::version::TLS12]
        };
        let builder = ServerConfig::builder_with_provider(provider())
            .with_protocol_versions(versions)
            .unwrap();
        let builder = if behavior.mutual_tls {
            let mut roots = RootCertStore::empty();
            roots
                .add(CertificateDer::from_pem_slice(pki.ca.as_bytes()).unwrap())
                .unwrap();
            let verifier = WebPkiClientVerifier::builder_with_provider(roots.into(), provider())
                .build()
                .unwrap();
            builder.with_client_cert_verifier(verifier)
        } else {
            builder.with_no_client_auth()
        };
        let config = Arc::new(builder.with_single_cert(chain, key).unwrap());

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let (tx, seen) = channel();
        let sockets = Arc::new(Mutex::new(Vec::new()));
        let accepted = Arc::clone(&sockets);
        thread::spawn(move || {
            for (n, tcp) in listener.incoming().enumerate() {
                let Ok(tcp) = tcp else { return };
                accepted.lock().unwrap().push(tcp.try_clone().unwrap());
                let (config, tx) = (Arc::clone(&config), tx.clone());
                thread::spawn(move || serve(n, tcp, &config, behavior, &tx));
            }
        });
        Self {
            port,
            seen,
            sockets,
        }
    }

    /// Next event, failing the test after [`WAIT`].
    fn next(&self) -> Seen {
        match self.seen.recv_timeout(WAIT) {
            Ok(s) => s,
            Err(RecvTimeoutError::Timeout) => panic!("test broker: nothing within {WAIT:?}"),
            Err(e) => panic!("test broker: {e}"),
        }
    }

    /// The next packet (skipping TLS events), with its connection.
    fn packet(&self) -> (usize, Packet) {
        loop {
            match self.next() {
                Seen::Packet(n, p) => return (n, p),
                Seen::Tls(..) => {}
                Seen::Closed(n) => panic!("connection {n} closed, expected a packet"),
            }
        }
    }

    fn connect(&self) -> (usize, Connect) {
        match self.packet() {
            (n, Packet::Connect(c)) => (n, c),
            other => panic!("expected CONNECT, got {other:?}"),
        }
    }

    fn publish(&self) -> (usize, Publish) {
        match self.packet() {
            (n, Packet::Publish(p)) => (n, p),
            other => panic!("expected PUBLISH, got {other:?}"),
        }
    }

    /// Waits until connection `n` is closed, returning the packets before.
    fn until_closed(&self, n: usize) -> Vec<Packet> {
        let mut packets = Vec::new();
        loop {
            match self.next() {
                Seen::Closed(m) if m == n => return packets,
                Seen::Packet(m, p) if m == n => packets.push(p),
                _ => {}
            }
        }
    }

    /// Drops connection `n` without DISCONNECT (broker restart, network).
    fn drop_connection(&self, n: usize) {
        let _ = self.sockets.lock().unwrap()[n].shutdown(Shutdown::Both);
    }
}

fn serve(n: usize, tcp: TcpStream, config: &Arc<ServerConfig>, b: Behavior, tx: &Sender<Seen>) {
    let mut tls = StreamOwned::new(ServerConnection::new(Arc::clone(config)).unwrap(), tcp);
    let mut decoder = Decoder::new(1 << 20);
    let mut buf = [0; 4096];
    let mut handshaken = false;
    loop {
        let read = match tls.read(&mut buf) {
            Ok(0) | Err(_) => break,
            Ok(read) => read,
        };
        if !handshaken {
            handshaken = true;
            let client_cert = tls.conn.peer_certificates().is_some();
            let _ = tx.send(Seen::Tls(n, client_cert));
        }
        decoder.feed(&buf[..read]);
        while let Ok(Some(packet)) = decoder.next_packet() {
            let reply = match &packet {
                Packet::Connect(_) => Some(Packet::ConnAck(ConnAck {
                    session_present: false,
                    code: if n == 0 {
                        b.first_connack
                    } else {
                        ConnectReturnCode::Accepted
                    },
                })),
                Packet::PingReq if b.ping => Some(Packet::PingResp),
                Packet::Publish(Publish {
                    delivery: Delivery::AtLeastOnce { packet_id, .. },
                    ..
                }) => Some(Packet::PubAck(*packet_id)),
                Packet::Subscribe(s) => Some(Packet::SubAck(SubAck {
                    packet_id: s.packet_id,
                    codes: vec![SubscribeReturnCode::Success(s.filters[0].1); s.filters.len()],
                })),
                _ => None,
            };
            let disconnect = matches!(packet, Packet::Disconnect);
            let _ = tx.send(Seen::Packet(n, packet));
            if let Some(reply) = reply {
                let mut out = Vec::new();
                encode(&reply, &mut out).unwrap();
                if tls.write_all(&out).and_then(|()| tls.flush()).is_err() {
                    break;
                }
            }
            if disconnect {
                let _ = tx.send(Seen::Closed(n));
                return;
            }
        }
    }
    let _ = tx.send(Seen::Closed(n));
}

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
    let states = states.clone();
    App::builder(Level::L1, ServiceType::Utility, product())
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
