//! Interoperability tests of P4 increment 5 (ADR-0026 §10, amendment
//! C-051) against the Mosquitto of the dev environment `oec/dev`: login
//! with credentials and with a client certificate (encrypted key with
//! passphrase), Birth → Health → Close → DISCONNECT, the Will of a killed
//! process, reconnect with a new Will and a new Birth after a broker
//! restart, refused logins, a wrong CA, TLS 1.2 and 1.3, the packet size
//! limits of SF-105 and the explicit bootstrap (replaces the smoke test of
//! C-050); since C-053 the Level 2 provider side (assets, `Get` answers),
//! since C-054 Data DataSets with Metadata and an Interval, since C-055
//! the write side (`Set Data`, `Set PublicationList`, Status events),
//! since C-056 the Config (`Get`/`Set Config`, masking).
//!
//! Opt-in: the tests pass at once unless `OEC_INTEROP=1` is set, so the
//! gate needs no container engine. Run them with `oec/dev/interop.sh`
//! (starts the broker, then `cargo test -p oec --test interop`); they read
//! the dev PKI and the sample container storages of `oec/dev/generated`
//! (`OEC_DEV_DIR` overrides the directory `oec/dev`) and restart the broker
//! with `OEC_COMPOSE` (default `docker compose`).

mod common;

use std::io::{self, Read, Write};
use std::net::TcpStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex, MutexGuard, PoisonError};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use oec::app::{AppBuilder, ConnectionState};
use oec::prelude::*;
use oec_core::il::Oi4Identifier;
use oec_core::mqtt::{
    Connect, ConnectReturnCode, Decoder, Delivery, Packet, PacketId, Publish, QoS, Subscribe,
    encode,
};
use rcgen::{
    BasicConstraints, CertificateParams, CertifiedIssuer, DnType, ExtendedKeyUsagePurpose, IsCa,
    KeyPair, KeyUsagePurpose,
};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, StreamOwned, SupportedProtocolVersion,
};
use rustls_pki_types::pem::PemObject;
use rustls_pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use serde_json::Value;

use common::product;

/// Upper bound of a wait.
const WAIT: Duration = Duration::from_secs(20);
/// Upper bound after a broker restart (backoff 1, 2, 4, 8, … s).
const RESTART_WAIT: Duration = Duration::from_secs(60);
/// How long a refused application is watched.
const REFUSED: Duration = Duration::from_secs(3);
/// MaxPacketSize of the sample broker.json and `message_size_limit` of
/// `oec/dev/mosquitto/mosquitto.conf`.
const MAX_PACKET_SIZE: usize = 262_144;
/// The container storage of the child process (see [`child_app`]).
const CHILD_ROOT: &str = "OEC_INTEROP_CHILD_ROOT";

// ---------------------------------------------------------------- dev env

/// The dev environment; holds the lock that serializes the tests.
struct Dev {
    dir: PathBuf,
    generated: PathBuf,
    port: u16,
    cert_port: u16,
    user: String,
    password: String,
    _lock: MutexGuard<'static, ()>,
}

/// `None` (test skipped) unless `OEC_INTEROP=1`.
fn dev() -> Option<Dev> {
    static LOCK: Mutex<()> = Mutex::new(());
    if std::env::var("OEC_INTEROP").as_deref() != Ok("1") {
        eprintln!("OEC_INTEROP not set: interop test skipped (see oec/dev/interop.sh)");
        return None;
    }
    let lock = LOCK.lock().unwrap_or_else(PoisonError::into_inner);
    let dir = std::env::var_os("OEC_DEV_DIR").map_or_else(
        || Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../dev"),
        PathBuf::from,
    );
    let generated = dir.join("generated");
    assert!(
        generated.join(".oec-dev-pki").is_file(),
        "no dev PKI in {}: run `go run ./pki` in oec/dev",
        generated.display()
    );
    let text = |rel: &str| {
        std::fs::read_to_string(generated.join(rel))
            .unwrap()
            .trim()
            .to_owned()
    };
    let port = |root: &str| -> u16 {
        let b: Value =
            serde_json::from_str(&text(&format!("roots/{root}/etc/oi4/mqtt/broker.json"))).unwrap();
        assert_eq!(
            b["Address"], "localhost",
            "the interop tests run on the host: generate the PKI with -address localhost"
        );
        u16::try_from(b["SecurePort"].as_u64().unwrap()).unwrap()
    };
    Some(Dev {
        port: port("oec-dev-app"),
        cert_port: port("oec-dev-cert"),
        user: text("probe/username"),
        password: text("probe/password"),
        dir,
        generated,
        _lock: lock,
    })
}

impl Dev {
    /// A fresh copy of the sample container storage `name` for `test`.
    fn root(&self, name: &str, test: &str) -> PathBuf {
        let to = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!("interop-{test}"));
        let _ = std::fs::remove_dir_all(&to);
        copy_dir(&self.generated.join("roots").join(name), &to);
        to
    }

    fn file(&self, rel: &str) -> PathBuf {
        self.generated.join(rel)
    }

    /// The probe on the credentials listener, subscribed to `Oi4v2/#`.
    fn probe(&self, client_id: &str, clean_session: bool) -> Probe {
        let config = tls(&self.file("broker/ca.pem"), None, ALL);
        let mut probe =
            Probe::open(self.port, config, self.connect(client_id, clean_session)).unwrap();
        probe.subscribe("Oi4v2/#", QoS::AtLeastOnce);
        probe
    }

    fn connect(&self, client_id: &str, clean_session: bool) -> Connect {
        Connect {
            client_id: client_id.to_owned(),
            keep_alive: 60,
            clean_session,
            will: None,
            username: Some(self.user.clone()),
            password: Some(self.password.clone().into_bytes()),
        }
    }

    /// `$OEC_COMPOSE -f oec/dev/compose.yaml <args>`.
    fn compose(&self, args: &[&str]) {
        let cmd = std::env::var("OEC_COMPOSE").unwrap_or_else(|_| "docker compose".to_owned());
        let mut words = cmd.split_whitespace();
        let status = Command::new(words.next().expect("OEC_COMPOSE is empty"))
            .args(words)
            .arg("-f")
            .arg(self.dir.join("compose.yaml"))
            .args(args)
            .stdout(Stdio::null())
            .status()
            .unwrap();
        assert!(status.success(), "{cmd} {args:?}: {status}");
    }
}

fn copy_dir(from: &Path, to: &Path) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let target = to.join(entry.file_name());
        if entry.file_type().unwrap().is_dir() {
            copy_dir(&entry.path(), &target);
        } else {
            std::fs::copy(entry.path(), target).unwrap();
        }
    }
}

fn write(root: &Path, rel: &str, content: &str) {
    let path = root.join(rel);
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(path, content).unwrap();
}

fn remove(root: &Path, rel: &str) {
    std::fs::remove_file(root.join(rel)).unwrap();
}

/// Standard base64 with padding (RFC 4648), for the secret storage.
fn base64(text: &str) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in text.as_bytes().chunks(3) {
        let n = chunk
            .iter()
            .enumerate()
            .fold(0u32, |n, (i, b)| n | u32::from(*b) << (16 - 8 * i));
        for i in 0..4 {
            out.push(if i <= chunk.len() {
                char::from(ALPHABET[(n >> (18 - 6 * i) & 63) as usize])
            } else {
                '='
            });
        }
    }
    out
}

// ---------------------------------------------------------------- TLS

const ALL: &[&SupportedProtocolVersion] = &[&rustls::version::TLS13, &rustls::version::TLS12];

/// The crypto provider of the SDK build (`crypto-ring` wins).
fn provider() -> Arc<rustls::crypto::CryptoProvider> {
    #[cfg(feature = "crypto-ring")]
    let provider = rustls::crypto::ring::default_provider();
    #[cfg(not(feature = "crypto-ring"))]
    let provider = rustls::crypto::aws_lc_rs::default_provider();
    Arc::new(provider)
}

/// A client configuration trusting `ca`, with an optional client
/// certificate `(cert, key)`.
fn tls(
    ca: &Path,
    client: Option<(&Path, &Path)>,
    versions: &[&'static SupportedProtocolVersion],
) -> Arc<ClientConfig> {
    let mut roots = RootCertStore::empty();
    for cert in CertificateDer::pem_file_iter(ca).unwrap() {
        roots.add(cert.unwrap()).unwrap();
    }
    let builder = ClientConfig::builder_with_provider(provider())
        .with_protocol_versions(versions)
        .unwrap()
        .with_root_certificates(roots);
    Arc::new(match client {
        Some((cert, key)) => builder
            .with_client_auth_cert(
                vec![CertificateDer::from_pem_file(cert).unwrap()],
                PrivateKeyDer::from_pem_file(key).unwrap(),
            )
            .unwrap(),
        None => builder.with_no_client_auth(),
    })
}

/// A CA the broker does not know, with a client certificate for `cn`:
/// `(ca, cert, key)` as PEM.
fn foreign_pki(cn: &str) -> (String, String, String) {
    let mut params = CertificateParams::new(Vec::<String>::new()).unwrap();
    params.is_ca = IsCa::Ca(BasicConstraints::Unconstrained);
    params
        .distinguished_name
        .push(DnType::CommonName, "foreign CA");
    params.key_usages = vec![KeyUsagePurpose::KeyCertSign, KeyUsagePurpose::CrlSign];
    let ca = CertifiedIssuer::self_signed(params, KeyPair::generate().unwrap()).unwrap();
    let key = KeyPair::generate().unwrap();
    let mut params = CertificateParams::new(Vec::<String>::new()).unwrap();
    params.distinguished_name.push(DnType::CommonName, cn);
    params.extended_key_usages = vec![ExtendedKeyUsagePurpose::ClientAuth];
    let cert = params.signed_by(&key, &ca).unwrap();
    (ca.pem(), cert.pem(), key.serialize_pem())
}

// ---------------------------------------------------------------- probe

/// A blocking MQTT 3.1.1 client with the codec of `oec-core`: observes the
/// broker like a second application would.
struct Probe {
    tls: StreamOwned<ClientConnection, TcpStream>,
    decoder: Decoder,
    pending: Vec<Publish>,
    last_sent: Instant,
    next_id: u16,
}

impl Probe {
    /// TCP, TLS, CONNECT and an accepting CONNACK; the error describes the
    /// first failure.
    fn open(port: u16, config: Arc<ClientConfig>, connect: Connect) -> Result<Self, String> {
        let tcp = TcpStream::connect(("localhost", port)).map_err(|e| format!("TCP: {e}"))?;
        tcp.set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let name = ServerName::try_from("localhost").unwrap();
        let conn = ClientConnection::new(config, name).map_err(|e| format!("TLS: {e}"))?;
        let mut probe = Self {
            tls: StreamOwned::new(conn, tcp),
            decoder: Decoder::new(1 << 20),
            pending: Vec::new(),
            last_sent: Instant::now(),
            next_id: 0,
        };
        probe.send(&Packet::Connect(connect))?;
        match probe.packet(Duration::from_secs(5))? {
            Some(Packet::ConnAck(a)) if a.code == ConnectReturnCode::Accepted => Ok(probe),
            Some(Packet::ConnAck(a)) => Err(format!("CONNACK {:?}", a.code)),
            other => Err(format!("expected CONNACK, got {other:?}")),
        }
    }

    fn version(&self) -> Option<rustls::ProtocolVersion> {
        self.tls.conn.protocol_version()
    }

    fn send(&mut self, packet: &Packet) -> Result<(), String> {
        let mut out = Vec::new();
        encode(packet, &mut out).map_err(|e| e.to_string())?;
        self.tls
            .write_all(&out)
            .and_then(|()| self.tls.flush())
            .map_err(|e| format!("write: {e}"))?;
        self.last_sent = Instant::now();
        Ok(())
    }

    /// The next packet; `None` after `timeout`, an error if the connection
    /// ended.
    fn packet(&mut self, timeout: Duration) -> Result<Option<Packet>, String> {
        let deadline = Instant::now() + timeout;
        let mut buf = vec![0; 1 << 16];
        loop {
            if let Some(p) = self.decoder.next_packet().map_err(|e| e.to_string())? {
                return Ok(Some(p));
            }
            if Instant::now() >= deadline {
                return Ok(None);
            }
            if self.last_sent.elapsed() > Duration::from_secs(20) {
                self.send(&Packet::PingReq)?;
            }
            match self.tls.read(&mut buf) {
                Ok(0) => return Err("connection closed by the broker".to_owned()),
                Ok(n) => self.decoder.feed(&buf[..n]),
                Err(e)
                    if matches!(
                        e.kind(),
                        io::ErrorKind::WouldBlock | io::ErrorKind::TimedOut
                    ) => {}
                Err(e) => return Err(format!("read: {e}")),
            }
        }
    }

    fn packet_id(&mut self) -> PacketId {
        self.next_id = self.next_id % u16::MAX + 1;
        PacketId::new(self.next_id).unwrap()
    }

    fn subscribe(&mut self, filter: &str, qos: QoS) {
        let packet_id = self.packet_id();
        self.send(&Packet::Subscribe(Subscribe {
            packet_id,
            filters: vec![(filter.to_owned(), qos)],
        }))
        .unwrap();
        loop {
            match self.packet(WAIT).unwrap() {
                Some(Packet::SubAck(a)) if a.packet_id == packet_id => return,
                Some(Packet::Publish(p)) => self.received(p),
                Some(_) => {}
                None => panic!("no SUBACK for {filter}"),
            }
        }
    }

    fn received(&mut self, p: Publish) {
        if let Delivery::AtLeastOnce { packet_id, .. } = p.delivery {
            self.send(&Packet::PubAck(packet_id)).unwrap();
        }
        self.pending.push(p);
    }

    /// The next PUBLISH matching `filter` within `timeout`.
    fn next(&mut self, timeout: Duration, filter: impl Fn(&Publish) -> bool) -> Option<Publish> {
        let deadline = Instant::now() + timeout;
        loop {
            if let Some(i) = self.pending.iter().position(&filter) {
                return Some(self.pending.remove(i));
            }
            self.pending.clear();
            let left = deadline.saturating_duration_since(Instant::now());
            match self.packet(left).unwrap() {
                Some(Packet::Publish(p)) => self.received(p),
                Some(_) => {}
                None => return None,
            }
        }
    }

    /// The next publication of the container `name`.
    fn of(&mut self, name: &str, timeout: Duration) -> Publish {
        let needle = app_element(name);
        self.next(timeout, |p| p.topic.contains(&needle))
            .unwrap_or_else(|| panic!("nothing of {name} within {timeout:?}"))
    }

    /// Asserts that `name` publishes nothing for `time`.
    fn silent(&mut self, name: &str, time: Duration) {
        let needle = app_element(name);
        if let Some(p) = self.next(time, |p| p.topic.contains(&needle)) {
            panic!("unexpected publication of {name}: {}", p.topic);
        }
    }

    /// PINGREQ → PINGRESP: the connection is alive.
    fn ping(&mut self) -> Result<(), String> {
        self.send(&Packet::PingReq)?;
        loop {
            match self.packet(WAIT)? {
                Some(Packet::PingResp) => return Ok(()),
                Some(Packet::Publish(p)) => self.received(p),
                Some(_) => {}
                None => return Err("no PINGRESP".to_owned()),
            }
        }
    }

    fn disconnect(mut self) {
        let _ = self.send(&Packet::Disconnect);
        self.tls.conn.send_close_notify();
        let _ = self.tls.flush();
    }
}

// ---------------------------------------------------------------- app

/// Records the states reported to `on_connection`.
#[derive(Clone, Default)]
struct States(Arc<Mutex<Vec<ConnectionState>>>);

impl States {
    fn get(&self) -> Vec<ConnectionState> {
        self.0.lock().unwrap().clone()
    }

    fn wait_for(&self, state: ConnectionState) {
        let start = Instant::now();
        while !self.get().contains(&state) {
            assert!(
                start.elapsed() < WAIT,
                "{state:?} not reached: {:?}",
                self.get()
            );
            thread::sleep(Duration::from_millis(20));
        }
    }
}

/// The application of the tests on the container storage `root`.
fn app(root: &Path, states: &States) -> AppBuilder {
    let states = states.clone();
    App::builder(Level::L1, ServiceType::Utility, product())
        .bootstrap(Bootstrap::ContainerStorage {
            root: root.to_owned(),
        })
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| states.0.lock().unwrap().push(s))
}

/// The application in a child process (this test binary running
/// [`child_app`]); killed on drop.
struct ChildApp(Child);

impl ChildApp {
    fn spawn(root: &Path) -> Self {
        let child = Command::new(std::env::current_exe().unwrap())
            .args(["child_app", "--exact", "--ignored", "--test-threads=1"])
            .env(CHILD_ROOT, root)
            .env_remove("OEC_INTEROP")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        Self(child)
    }

    /// SIGKILL / TerminateProcess: no Close, no DISCONNECT.
    fn kill(&mut self) {
        self.0.kill().unwrap();
        self.0.wait().unwrap();
    }
}

impl Drop for ChildApp {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

/// The body of the child process; does nothing in a normal test run.
#[test]
#[ignore = "child process of the interop tests"]
fn child_app() {
    let Some(root) = std::env::var_os(CHILD_ROOT) else {
        return;
    };
    app(Path::new(&root), &States::default())
        .build()
        .unwrap()
        .run()
        .unwrap();
}

/// `/<AppId>/` of the application in the container `name`: the serial
/// number is the container name (REQ-CNT-019, REQ-CNT-021), the topic
/// element is base64url (REQ-IL-002).
fn app_element(name: &str) -> String {
    let id = Oi4Identifier::new("example.com", "Hello OEC", "HELLO-1")
        .serial_number(name)
        .build()
        .unwrap();
    format!("/{}/", id.encode())
}

fn json(payload: &[u8]) -> Value {
    serde_json::from_slice(payload).unwrap()
}

/// `(Health, HealthScore)` of a Pub Health payload.
fn health(p: &Publish) -> (String, u64) {
    assert!(p.topic.contains("/Pub/Health/"), "{}", p.topic);
    let v = json(&p.payload);
    let h = &v["Messages"][0]["Payload"];
    (
        h["Health"].as_str().unwrap().to_owned(),
        h["HealthScore"].as_u64().unwrap(),
    )
}

fn birth(p: &Publish) {
    assert!(
        p.topic.contains("/Pub/MAM/"),
        "expected the Birth, got {}",
        p.topic
    );
    assert_eq!(json(&p.payload)["Messages"][0]["DataSetWriterId"], 10);
}

/// The creation time of a MessageId (`<Unix ms><counter>-<PublisherId>`,
/// REQ-MSG-007).
fn created_ms(p: &Publish) -> u64 {
    let v = json(&p.payload);
    v["MessageId"].as_str().unwrap()[..13].parse().unwrap()
}

fn now_ms() -> u64 {
    u64::try_from(
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_millis(),
    )
    .unwrap()
}

/// The application on `root` never gets online: `Connecting` until the
/// shutdown, no publication of `name` (ADR-0020 §5).
fn never_online(dev: &Dev, root: &Path, name: &str) {
    let mut probe = dev.probe(&format!("oec-probe-{name}"), true);
    let states = States::default();
    let app = app(root, &states).build().unwrap();
    let handle = app.handle();
    let run = thread::spawn(move || app.run());
    probe.silent(name, REFUSED);
    assert_eq!(handle.state(), ConnectionState::Connecting);
    handle.shutdown();
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [ConnectionState::Connecting, ConnectionState::Closed]
    );
    probe.disconnect();
}

// ---------------------------------------------------------------- tests

/// Credentials (REQ-MQTT-009, REQ-CNT-008/009): Birth, then Health
/// NORMAL_0/100, on shutdown the Close NORMAL_0/0 and DISCONNECT, so the
/// broker discards the Will (REQ-LIFE-001, REQ-LIFE-005, REQ-LIFE-008,
/// REQ-LIFE-015, MG 6.3.4).
#[test]
fn credentials_birth_health_close_disconnect() {
    let Some(dev) = dev() else { return };
    let root = dev.root("oec-dev-app", "lifecycle");
    let mut probe = dev.probe("oec-probe-lifecycle", true);
    let states = States::default();
    let app = app(&root, &states).build().unwrap();
    let handle = app.handle();
    let run = thread::spawn(move || app.run());

    birth(&probe.of("oec-dev-app", WAIT));
    assert_eq!(
        health(&probe.of("oec-dev-app", WAIT)),
        ("NORMAL_0".to_owned(), 100)
    );
    states.wait_for(ConnectionState::Online);
    handle.shutdown();
    assert_eq!(
        health(&probe.of("oec-dev-app", WAIT)),
        ("NORMAL_0".to_owned(), 0)
    );
    run.join().unwrap().unwrap();
    probe.silent("oec-dev-app", Duration::from_secs(2));
    assert_eq!(
        states.get(),
        [
            ConnectionState::Connecting,
            ConnectionState::Online,
            ConnectionState::Closed
        ]
    );
    probe.disconnect();
}

/// Client certificate with a PKCS#8 PBES2 key and its passphrase
/// (REQ-MQTT-010, REQ-CNT-008/009) in a child process; killing it makes the
/// broker publish the Will FAILURE_1/0 (REQ-LIFE-008, REQ-MQTT-014).
#[test]
fn client_certificate_will_of_a_killed_process() {
    let Some(dev) = dev() else { return };
    let root = dev.root("oec-dev-cert", "will");
    let mut probe = dev.probe("oec-probe-will", true);
    let mut child = ChildApp::spawn(&root);

    birth(&probe.of("oec-dev-cert", WAIT));
    assert_eq!(
        health(&probe.of("oec-dev-cert", WAIT)),
        ("NORMAL_0".to_owned(), 100)
    );
    child.kill();
    assert_eq!(
        health(&probe.of("oec-dev-cert", WAIT)),
        ("FAILURE_1".to_owned(), 0)
    );
    probe.disconnect();
}

/// A broker restart: the application reconnects with a new Will and sends
/// a new Birth (REQ-LIFE-001, REQ-LIFE-016); a persistent probe session
/// (Clean Session 0, QoS 1) keeps what was published while it reconnected.
#[test]
fn broker_restart_new_will_and_birth() {
    let Some(dev) = dev() else { return };
    let root = dev.root("oec-dev-app", "restart");
    let id = "oec-probe-restart";
    let mut probe = dev.probe(id, false);
    let mut child = ChildApp::spawn(&root);
    birth(&probe.of("oec-dev-app", WAIT));
    assert_eq!(health(&probe.of("oec-dev-app", WAIT)).1, 100);

    let restart = now_ms();
    dev.compose(&["restart", "mosquitto"]);
    drop(probe);
    let start = Instant::now();
    let mut probe = loop {
        let config = tls(&dev.file("broker/ca.pem"), None, ALL);
        match Probe::open(dev.port, config, dev.connect(id, false)) {
            Ok(p) => break p,
            Err(e) => assert!(start.elapsed() < WAIT, "probe reconnect: {e}"),
        }
        thread::sleep(Duration::from_millis(200));
    };
    // QoS 1 redeliveries from before the restart (DUP) are skipped; a Will
    // of the first CONNECT would never match (REQ-LIFE-016)
    let needle = app_element("oec-dev-app");
    let after = |probe: &mut Probe, timeout| {
        probe
            .next(timeout, |p| {
                p.topic.contains(&needle) && created_ms(p) >= restart
            })
            .expect("nothing of oec-dev-app created after the restart")
    };
    birth(&after(&mut probe, RESTART_WAIT));
    assert_eq!(health(&after(&mut probe, WAIT)).1, 100);

    child.kill();
    assert_eq!(
        health(&after(&mut probe, WAIT)),
        ("FAILURE_1".to_owned(), 0)
    );
    probe.disconnect();
    // purge the persistent session
    let config = tls(&dev.file("broker/ca.pem"), None, ALL);
    Probe::open(dev.port, config, dev.connect(id, true))
        .unwrap()
        .disconnect();
}

/// The explicit bootstrap (`Bootstrap::Explicit`, ADR-0020 §5) with the
/// CA file and credentials, as an application outside a container would
/// use it; replaces the smoke test of C-050.
#[test]
fn explicit_broker() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-explicit", true);
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let states = States::default();
    let on = states.clone();
    let app = App::builder(Level::L1, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-explicit")
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let handle = app.handle();
    let run = thread::spawn(move || app.run());
    birth(&probe.of("oec-explicit", WAIT));
    assert_eq!(health(&probe.of("oec-explicit", WAIT)).1, 100);
    states.wait_for(ConnectionState::Online);
    handle.shutdown();
    assert_eq!(health(&probe.of("oec-explicit", WAIT)).1, 0);
    run.join().unwrap().unwrap();
    assert_eq!(
        states.get(),
        [
            ConnectionState::Connecting,
            ConnectionState::Online,
            ConnectionState::Closed
        ]
    );
    probe.disconnect();
}

/// Refused logins never get online (REQ-MQTT-009, REQ-MQTT-010): a wrong
/// password, an unknown user, valid credentials on the certificate
/// listener, a client certificate of a foreign CA.
#[test]
fn refused_logins() {
    let Some(dev) = dev() else { return };
    let creds = "run/secrets/oi4/credentials";

    let root = dev.root("oec-dev-app", "wrong-password");
    write(&root, creds, &base64("oec-dev-app:wrong"));
    never_online(&dev, &root, "oec-dev-app");
    let config = tls(&dev.file("broker/ca.pem"), None, ALL);
    let mut connect = dev.connect("oec-probe-refused", true);
    connect.password = Some(b"wrong".to_vec());
    let refused = Probe::open(dev.port, config, connect).err().unwrap();
    assert!(refused.contains("CONNACK"), "{refused}");

    let root = dev.root("oec-dev-app", "unknown-user");
    write(&root, creds, &base64("nobody:secret"));
    never_online(&dev, &root, "oec-dev-app");

    let root = dev.root("oec-dev-cert", "no-certificate");
    for f in [
        "etc/oi4/certs/oec-dev-cert.pem",
        "run/secrets/oi4/private_key.pem",
        "run/secrets/oi4/passphrase",
    ] {
        remove(&root, f);
    }
    write(
        &root,
        creds,
        &base64(&format!("{}:{}", dev.user, dev.password)),
    );
    never_online(&dev, &root, "oec-dev-cert");

    let root = dev.root("oec-dev-cert", "foreign-certificate");
    let (_, cert, key) = foreign_pki("oec-dev-cert");
    write(&root, "etc/oi4/certs/oec-dev-cert.pem", &cert);
    write(&root, "run/secrets/oi4/private_key.pem", &key);
    remove(&root, "run/secrets/oi4/passphrase");
    never_online(&dev, &root, "oec-dev-cert");
}

/// The broker certificate is verified against the CA of the container
/// storage only (REQ-MQTT-008): with a foreign `ca.pem` the application
/// never gets online, the probe fails the handshake.
#[test]
fn wrong_ca() {
    let Some(dev) = dev() else { return };
    let (ca, _, _) = foreign_pki("unused");
    let root = dev.root("oec-dev-app", "wrong-ca");
    write(&root, "etc/oi4/certs/ca.pem", &ca);
    never_online(&dev, &root, "oec-dev-app");

    let config = tls(&root.join("etc/oi4/certs/ca.pem"), None, ALL);
    let err = Probe::open(dev.port, config, dev.connect("oec-probe-ca", true))
        .err()
        .unwrap();
    assert!(err.contains("UnknownIssuer"), "{err}");
}

/// Both listeners speak TLS 1.2 and TLS 1.3 (REQ-MQTT-005); the certificate
/// listener with the probe certificate (REQ-MQTT-010).
#[test]
fn tls_versions() {
    let Some(dev) = dev() else { return };
    let ca = dev.file("broker/ca.pem");
    let (cert, key) = (dev.file("probe/probe.pem"), dev.file("probe/probe.key"));
    for (version, expected) in [
        (&rustls::version::TLS12, rustls::ProtocolVersion::TLSv1_2),
        (&rustls::version::TLS13, rustls::ProtocolVersion::TLSv1_3),
    ] {
        let p = Probe::open(
            dev.port,
            tls(&ca, None, &[version]),
            dev.connect("oec-probe-tls", true),
        )
        .unwrap();
        assert_eq!(p.version(), Some(expected));
        p.disconnect();
        let mut connect = dev.connect("oec-probe-tls", true);
        (connect.username, connect.password) = (None, None);
        let p = Probe::open(
            dev.cert_port,
            tls(&ca, Some((&cert, &key)), &[version]),
            connect,
        )
        .unwrap();
        assert_eq!(p.version(), Some(expected));
        p.disconnect();
    }
}

/// SF-105: a payload of exactly MaxPacketSize behind a long topic passes
/// the dev broker (its whole-packet limit has room for the header), a
/// larger payload is dropped without disconnect (`message_size_limit`).
#[test]
fn max_packet_size_limits_the_payload() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-size", true);
    let topic = format!("oec-interop/size/{}", "t".repeat(1000));
    probe.subscribe("oec-interop/size/#", QoS::AtMostOnce);
    for (size, delivered) in [(MAX_PACKET_SIZE, true), (MAX_PACKET_SIZE + 1, false)] {
        probe
            .send(&Packet::Publish(Publish {
                topic: topic.clone(),
                payload: vec![b'x'; size],
                delivery: Delivery::AtMostOnce,
                retain: false,
            }))
            .unwrap();
        let got = probe.next(Duration::from_secs(3), |p| p.topic == topic);
        assert_eq!(got.map(|p| p.payload.len()), delivered.then_some(size));
        probe.ping().unwrap();
    }
    probe.disconnect();
}

/// Level 2 provider side (P4 increment 6a, ADR-0026 §12): the application
/// and an asset publish MAM, Health and Profile, then the PublicationList;
/// a `Get` through Mosquitto is answered with `ReplyTo` = the MessageId of
/// the request (REQ-RES-PUBLIST-027, REQ-MSG-*); an asset Health change is
/// published at once.
#[test]
fn level_2_assets_and_get_answers() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-provider", true);
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let states = States::default();
    let on = states.clone();
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-provider")
        .reference_designation(false)
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-1"), "0815")
                .unwrap(),
        )
        .unwrap();
    let encoded = |il: &str| {
        oec_core::il::IdentificationLink::parse(il)
            .unwrap()
            .encode()
    };
    let me = encoded(app.identification_link().as_str());
    let it = encoded(pump.identification_link().as_str());
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = thread::spawn(move || app.run());

    let mut topics = Vec::new();
    while topics.len() < 7 {
        topics.push(probe.of("oec-provider", WAIT).topic);
    }
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
    states.wait_for(ConnectionState::Online);

    let request = "1700000000000-Aggregation/vendor.com/Client/C-1/1";
    probe
        .send(&Packet::Publish(Publish {
            topic: format!("{prefix}Get/PublicationList"),
            payload: serde_json::to_vec(&serde_json::json!({
                "MessageId": request,
                "MessageType": "ua-data",
                "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
                // the PublicationList DataSetClassId: a valid request for D7
                "DataSetClassId": "217434d6-6e1e-4230-b907-f52bc9ffe152",
                "ReplyTo": "",
                "Messages": []
            }))
            .unwrap(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }))
        .unwrap();
    let answer = probe
        .next(WAIT, |p| p.topic == format!("{prefix}Pub/PublicationList"))
        .expect("no answer to the Get");
    let v = json(&answer.payload);
    assert_eq!(v["ReplyTo"], request);
    // MAM, Health, Profile and the 3 Status events of each source (C-055),
    // the 8 Syslog levels of the app (C-057)
    assert_eq!(v["Messages"].as_array().unwrap().len(), 20);

    pump.set_health(HealthState::MaintenanceRequired, Some(70));
    let p = probe
        .next(WAIT, |p| p.topic == format!("{prefix}Pub/Health/{it}"))
        .expect("no asset Health");
    assert_eq!(health(&p), ("MAINTENANCE_REQUIRED_4".to_owned(), 70));

    handle.shutdown();
    run.join().unwrap().unwrap();
    probe.disconnect();
}

/// A request (`Get`, `Set`) through the broker from the probe.
fn get(probe: &mut Probe, topic: String, payload: &Value) {
    probe
        .send(&Packet::Publish(Publish {
            topic,
            payload: serde_json::to_vec(payload).unwrap(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }))
        .unwrap();
}

/// A pump with the DataSet `oee` (FILTER, a Precision) and the app's
/// `Oi4Pv` (SOURCE, Interval 200 ms), both with values.
fn data_sets(app: &App) -> (Asset, DataSet, DataSet) {
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-2"), "0815")
                .unwrap(),
        )
        .unwrap();
    let oee = pump
        .add_data(
            DataSetDef::new("oee")
                .field(Field::double("Availability").unit("%").range(0.0, 100.0))
                .publication(Publication::default_data().precision("Availability", 0.5)),
        )
        .unwrap();
    oee.update([("Availability", 97.5)]).unwrap();
    let pv = app
        .this()
        .add_data(
            DataSetDef::oi4pv(Field::double("T")).publication(
                Publication::default_data()
                    .mode(Mode::ON_REQUEST | Mode::SOURCE)
                    .interval(Duration::from_millis(200)),
            ),
        )
        .unwrap();
    pv.update([("Pv", 20.5)]).unwrap();
    (pump, oee, pv)
}

/// Data DataSets through Mosquitto (P4 increment 6b): the Metadata of
/// each DataSet before its Data (REQ-RES-META-003), the Data of the app's
/// `Oi4Pv` every Interval (REQ-RES-PUBLIST-013), the pump's DataSet on its
/// Filter topic with its DataSetClassId (REQ-MSG-011/013), and `Get Data` /
/// `Get Metadata` answered with `ReplyTo` (REQ-RES-DATA-007,
/// REQ-RES-META-002).
#[test]
fn level_2_data_sets_and_metadata() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-data", true);
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let states = States::default();
    let on = states.clone();
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-data")
        .reference_designation(false)
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let (pump, _oee, _pv) = data_sets(&app);
    let encoded = |il: &str| {
        oec_core::il::IdentificationLink::parse(il)
            .unwrap()
            .encode()
    };
    let me = encoded(app.identification_link().as_str());
    let it = encoded(pump.identification_link().as_str());
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = thread::spawn(move || app.run());

    let (oee_meta, oee_data, pv_data) = (
        format!("{prefix}Pub/Metadata/{it}/oee"),
        format!("{prefix}Pub/Data/{it}/oee"),
        format!("{prefix}Pub/Data/{me}"),
    );
    let mut topics = Vec::new();
    let mut oee_first = None;
    while oee_first.is_none() || topics.iter().filter(|t| **t == pv_data).count() < 3 {
        let p = probe.of("oec-data", WAIT);
        if p.topic == oee_data && oee_first.is_none() {
            oee_first = Some(p.clone());
        }
        topics.push(p.topic);
    }
    let position = |t: &str| topics.iter().position(|x| x == t).unwrap();
    assert!(position(&oee_meta) < position(&oee_data), "{topics:?}");
    assert!(
        position(&format!("{prefix}Pub/Metadata/{me}/Oi4Pv")) < position(&pv_data),
        "{topics:?}"
    );
    let v = json(&oee_first.unwrap().payload);
    assert_eq!(v["Messages"][0]["Payload"]["Availability"], 97.5);
    let class = v["DataSetClassId"].as_str().unwrap().to_owned();
    assert_eq!(class.len(), 36);
    states.wait_for(ConnectionState::Online);

    let (request, meta_request) = (
        "1700000000000-Aggregation/vendor.com/Client/C-1/1",
        "1700000000001-Aggregation/vendor.com/Client/C-1/1",
    );
    get(
        &mut probe,
        format!("{prefix}Get/Data/{it}/oee"),
        &serde_json::json!({
            "MessageId": request,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "DataSetClassId": class,
            "ReplyTo": "",
            "Messages": []
        }),
    );
    let answer = probe
        .next(WAIT, |p| p.topic == oee_data)
        .expect("no answer to Get Data");
    assert_eq!(json(&answer.payload)["ReplyTo"], request);
    get(
        &mut probe,
        format!("{prefix}Get/Metadata/{it}/oee"),
        &serde_json::json!({
            "MessageId": meta_request,
            "MessageType": "ua-metadata",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "DataSetWriterId": 10,
            "WriterGroupName": "oee",
            "DataSetWriterName": pump.identification_link().as_str(),
            "Timestamp": "2023-11-14T22:13:20.000Z",
            "ReplyTo": "",
            "MetaData": {}
        }),
    );
    let answer = probe
        .next(WAIT, |p| p.topic == oee_meta)
        .expect("no answer to Get Metadata");
    let v = json(&answer.payload);
    assert_eq!(v["ReplyTo"], meta_request);
    assert_eq!(v["MetaData"]["Fields"][0]["Name"], "Availability");

    handle.shutdown();
    run.join().unwrap().unwrap();
    probe.disconnect();
}

/// The DataSetClassId of the PublicationList (REQ-MSG-010).
const PUBLICATION_LIST: &str = "217434d6-6e1e-4230-b907-f52bc9ffe152";

/// A request NetworkMessage of the probe with the DataSetClassId of the
/// resource and `messages`.
fn request(id: &str, class: &str, messages: &Value) -> Value {
    serde_json::json!({
        "MessageId": id,
        "MessageType": "ua-data",
        "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
        "DataSetClassId": class,
        "ReplyTo": "",
        "Messages": messages.clone()
    })
}

/// The first DataSetMessage of `answer` with `payload`: a valid `Set`
/// DataSetMessage (WriterGroupName, DataSetWriterId of the provider).
fn with_payload(answer: &Value, payload: &Value) -> Value {
    let mut dsm = answer["Messages"][0].clone();
    dsm["Payload"] = payload.clone();
    serde_json::json!([dsm])
}

/// The next publication on `topic` answering `id`.
fn answer(probe: &mut Probe, topic: &str, id: &str) -> Value {
    let p = probe
        .next(WAIT, |p| {
            p.topic == topic && json(&p.payload)["ReplyTo"] == id
        })
        .unwrap_or_else(|| panic!("no answer to {id} on {topic}"));
    json(&p.payload)
}

/// A Level 2 app with a pump whose DataSet `speed` is writable (Set Data
/// within 0 … 3000, Set PublicationList of Mode and Interval).
fn write_app(dev: &Dev, states: &States) -> (App, Asset) {
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let on = states.clone();
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-write")
        .reference_designation(false)
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-3"), "0815")
                .unwrap(),
        )
        .unwrap();
    let speed = pump
        .add_data(
            DataSetDef::new("speed")
                .field(Field::double("Setpoint").range(0.0, 3000.0))
                .publication(
                    Publication::default_data().writable(oec::data::Writable::ModeAndInterval),
                ),
        )
        .unwrap();
    speed.update([("Setpoint", 1000.0)]).unwrap();
    speed
        .on_set(|s| match s.get("Setpoint") {
            Some(oec::data::Value::Double(v)) if (0.0..=3000.0).contains(v) => Ok(()),
            _ => Err(StatusCode::BadOutOfRange),
        })
        .unwrap();
    (app, pump)
}

/// The write side through Mosquitto (P4 increment 7a): `Set Data` decided
/// by the handler, answered with the DataSet and the Status event Good or
/// the handler's code (REQ-RES-DATA-011/012, REQ-RES-EVENT-005);
/// `Set PublicationList` answered with the changed entry
/// (REQ-RES-PUBLIST-025); a `Get` for an unknown Source with `BadNotFound`
/// under the AppId (REQ-CMP-047, SF-108).
#[test]
fn level_2_set_data_and_publication_list() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-write", true);
    let states = States::default();
    let (app, pump) = write_app(&dev, &states);
    let encoded = |il: &str| {
        oec_core::il::IdentificationLink::parse(il)
            .unwrap()
            .encode()
    };
    let me = encoded(app.identification_link().as_str());
    let it = encoded(pump.identification_link().as_str());
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = thread::spawn(move || app.run());
    states.wait_for(ConnectionState::Online);
    let (good, bad) = (
        format!("{prefix}Pub/Event/{it}/Status/Good"),
        format!("{prefix}Pub/Event/{it}/Status/Bad"),
    );

    // the DataSetClassId and DataSetMessage of the DataSet (REQ-MSG-010/016)
    let data = format!("{prefix}Pub/Data/{it}/speed");
    let first = probe.next(WAIT, |p| p.topic == data).expect("no Pub Data");
    let first = json(&first.payload);
    let class = first["DataSetClassId"].as_str().unwrap().to_owned();

    let id = "1700000000000-Aggregation/vendor.com/Client/C-1/1";
    let set = serde_json::json!({"Setpoint": 1500.0});
    get(
        &mut probe,
        format!("{prefix}Set/Data/{it}/speed"),
        &request(id, &class, &with_payload(&first, &set)),
    );
    assert_eq!(answer(&mut probe, &data, id)["Messages"][0]["Payload"], set);
    assert_eq!(
        answer(&mut probe, &good, id)["Messages"][0]["Payload"]["Number"],
        0
    );

    let id = "1700000000001-Aggregation/vendor.com/Client/C-1/1";
    let set = serde_json::json!({"Setpoint": 5000.0});
    get(
        &mut probe,
        format!("{prefix}Set/Data/{it}/speed"),
        &request(id, &class, &with_payload(&first, &set)),
    );
    let event = answer(&mut probe, &bad, id);
    assert_eq!(
        event["Messages"][0]["Payload"]["Number"],
        StatusCode::BadOutOfRange.code()
    );

    let rest = format!("PublicationList/{it}/Data/speed");
    let id = "1700000000002-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Get/{rest}"),
        &request(id, PUBLICATION_LIST, &serde_json::json!([])),
    );
    let listed = answer(&mut probe, &format!("{prefix}Pub/{rest}"), id);
    let mut entry = listed["Messages"][0]["Payload"].clone();
    entry["Interval"] = serde_json::json!(60_000);
    let id = "1700000000003-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Set/{rest}"),
        &request(id, PUBLICATION_LIST, &with_payload(&listed, &entry)),
    );
    let changed = answer(&mut probe, &format!("{prefix}Pub/{rest}"), id);
    assert_eq!(changed["Messages"][0]["Payload"], entry);
    assert_eq!(
        answer(&mut probe, &good, id)["Messages"][0]["Payload"]["Number"],
        0
    );

    let unknown = encoded("vendor.com/Pump/P-3/4711");
    let id = "1700000000004-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Get/MAM/{unknown}"),
        &request(
            id,
            "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58",
            &serde_json::json!([]),
        ),
    );
    // answered under the AppId (SF-108)
    let event = answer(
        &mut probe,
        &format!("{prefix}Pub/Event/{me}/Status/Bad"),
        id,
    );
    assert_eq!(
        event["Messages"][0]["Payload"]["Number"],
        StatusCode::BadNotFound.code()
    );

    handle.shutdown();
    run.join().unwrap().unwrap();
    probe.disconnect();
}

/// The DataSetClassId of the Config (REQ-MSG-010).
const CONFIG: &str = "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2";

/// Config through Mosquitto (P4 increment 7b): `Get Config` with the
/// sensitive API key masked (GL-ADR-021, REQ-RES-CONFIG-016/018); a
/// `Set Config` with the mask keeps the key and is answered with the Config
/// and Good; the application's own rule refuses with its code
/// (REQ-RES-CONFIG-020 … 025, REQ-RES-EVENT-005).
#[test]
fn level_2_config() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-config", true);
    let states = States::default();
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let on = states.clone();
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-config")
        .reference_designation(false)
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let config = app
        .this()
        .add_config(
            ConfigDef::new("service").group(
                "common",
                "Common settings",
                [
                    ConfigObject::number("interval", 60.0)
                        .min(10.0)
                        .max(3600.0)
                        .mandatory(),
                    ConfigObject::string("api_key", "").sensitive(),
                ],
            ),
        )
        .unwrap();
    config
        .on_set(|proposed| match proposed.number("common", "interval") {
            Some(i) if i % 10.0 != 0.0 => Err(StatusCode::BadOutOfRange),
            _ => Ok(()),
        })
        .unwrap();
    config.set("common", "api_key", "s3cr3t").unwrap();
    let me = oec_core::il::IdentificationLink::parse(app.identification_link().as_str())
        .unwrap()
        .encode();
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = thread::spawn(move || app.run());
    states.wait_for(ConnectionState::Online);
    let topic = format!("{prefix}Pub/Config/{me}/service");
    // Online follows CONNACK; the SUBSCRIBE precedes the first publication
    // on the same connection, so the broker knows it once this arrives.
    probe
        .next(WAIT, |p| p.topic == topic)
        .expect("no Pub Config at connect");

    let id = "1700000000010-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Get/Config/{me}/service"),
        &request(id, CONFIG, &serde_json::json!([])),
    );
    let got = answer(&mut probe, &topic, id);
    assert_eq!(got["DataSetClassId"], CONFIG);
    assert_eq!(got["Messages"][0]["WriterGroupName"], "service");
    let common = &got["Messages"][0]["Payload"]["common"];
    assert_eq!(common["interval"]["Value"], "60");
    assert_eq!(common["api_key"]["Value"], "********");
    assert!(!String::from_utf8_lossy(&serde_json::to_vec(&got).unwrap()).contains("s3cr3t"));

    let id = "1700000000011-Aggregation/vendor.com/Client/C-1/1";
    let set = serde_json::json!({"common": {
        "interval": {"Value": "30"},
        "api_key": {"Value": "********"}
    }});
    get(
        &mut probe,
        format!("{prefix}Set/Config/{me}/service"),
        &request(id, CONFIG, &with_payload(&got, &set)),
    );
    let changed = answer(&mut probe, &topic, id);
    let common = &changed["Messages"][0]["Payload"]["common"];
    assert_eq!(common["interval"]["Value"], "30");
    assert_eq!(common["api_key"]["Value"], "********");
    let good = format!("{prefix}Pub/Event/{me}/Status/Good");
    assert_eq!(
        answer(&mut probe, &good, id)["Messages"][0]["Payload"]["Number"],
        0
    );

    let id = "1700000000012-Aggregation/vendor.com/Client/C-1/1";
    let set = serde_json::json!({"common": {"interval": {"Value": "25"}}});
    get(
        &mut probe,
        format!("{prefix}Set/Config/{me}/service"),
        &request(id, CONFIG, &with_payload(&got, &set)),
    );
    let bad = format!("{prefix}Pub/Event/{me}/Status/Bad");
    assert_eq!(
        answer(&mut probe, &bad, id)["Messages"][0]["Payload"]["Number"],
        StatusCode::BadOutOfRange.code()
    );
    let values = config.values();
    assert_eq!(values.number("common", "interval"), Some(30.0));
    assert_eq!(values.string("common", "api_key"), Some("s3cr3t"));

    handle.shutdown();
    run.join().unwrap().unwrap();
    probe.disconnect();
}

/// The DataSetClassId of the ReferenceDesignation (REQ-MSG-010).
const REFERENCE_DESIGNATION: &str = "27a75019-164a-496d-a38b-90e8a55c2cfa";

/// A Level 2 app with the ReferenceDesignation on a persistent store and a
/// pump whose handler refuses the Location `+forbidden`.
fn refdes_app(states: &States, broker: Broker) -> (App, Asset) {
    let store = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join("interop-refdes-store");
    let _ = std::fs::remove_dir_all(&store);
    let on = states.clone();
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("oec-refdes")
        .reference_designation(true)
        .store(oec::store::FileStore::new(store))
        .keep_alive(Duration::from_secs(5))
        .on_connection(move |s| on.0.lock().unwrap().push(s))
        .build()
        .unwrap();
    let pump = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Pump", "P-4"), "0815")
                .unwrap(),
        )
        .unwrap();
    pump.on_reference_designation(|rd| match rd.and_then(|r| r.location.as_ref()) {
        Some(l) if l.value == "+forbidden" => Err(StatusCode::BadOutOfRange),
        _ => Ok(()),
    })
    .unwrap();
    (app, pump)
}

/// A Generic event of `pump` on `events` + `/Generic/Maintenance`
/// (REQ-RES-EVENT-009).
fn generic_event(probe: &mut Probe, pump: &Asset, events: &str) {
    pump.emit_event(
        GenericEvent::new("Maintenance", 42)
            .description("filter clogged")
            .details_json(r#"{"Pressure": 3.5}"#),
    )
    .unwrap();
    let topic = format!("{events}/Generic/Maintenance");
    let event = probe
        .next(WAIT, |p| p.topic == topic)
        .expect("no Generic event");
    let m = &json(&event.payload)["Messages"][0];
    assert_eq!(m["MessageType"], "ua-event");
    assert_eq!(m["Payload"]["Category"], "CAT_GENERIC_99");
    assert_eq!(m["Payload"]["Number"], 42);
}

/// The ReferenceDesignation and Generic events through Mosquitto (P4
/// increment 7c): `Set` answered with the object and Good, the handler's
/// refusal with its code, `Get` with the object, `Del` with Good and a
/// second `Del` with `BadNotFound` (REQ-RES-REFDES-*, REQ-RES-EVENT-005);
/// a Generic event on `Pub/Event/<Source>/Generic/<Level>`
/// (REQ-RES-EVENT-009).
#[test]
fn level_2_reference_designation_and_generic_events() {
    let Some(dev) = dev() else { return };
    let mut probe = dev.probe("oec-probe-refdes", true);
    let states = States::default();
    let broker = Broker::new("localhost")
        .port(dev.port)
        .ca_pem(dev.file("broker/ca.pem"))
        .credentials(&dev.user, &dev.password);
    let (app, pump) = refdes_app(&states, broker);
    let encoded = |il: &str| {
        oec_core::il::IdentificationLink::parse(il)
            .unwrap()
            .encode()
    };
    let me = encoded(app.identification_link().as_str());
    let it = encoded(pump.identification_link().as_str());
    let il = pump.identification_link().as_str().to_owned();
    let prefix = format!("Oi4v2/Utility/{me}/");
    let handle = app.handle();
    let run = thread::spawn(move || app.run());
    states.wait_for(ConnectionState::Online);
    // the SUBSCRIBE precedes the first publication on the same connection
    let mam = format!("{prefix}Pub/MAM/{it}");
    probe.next(WAIT, |p| p.topic == mam).expect("no Pub MAM");
    let (rd_topic, good, bad) = (
        format!("{prefix}Pub/ReferenceDesignation/{it}"),
        format!("{prefix}Pub/Event/{it}/Status/Good"),
        format!("{prefix}Pub/Event/{it}/Status/Bad"),
    );
    let dsm = |payload: &Value| {
        serde_json::json!([{
            "DataSetWriterId": 20,
            "WriterGroupName": "",
            "DataSetWriterName": il,
            "Payload": payload
        }])
    };

    let rd = serde_json::json!({"Function": {"Value": "=P1.M1"}, "Location": {"Value": "+R1"}});
    let id = "1700000000020-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Set/ReferenceDesignation/{it}"),
        &request(id, REFERENCE_DESIGNATION, &dsm(&rd)),
    );
    let set = answer(&mut probe, &rd_topic, id);
    assert_eq!(set["DataSetClassId"], REFERENCE_DESIGNATION);
    assert_eq!(set["Messages"][0]["Payload"], rd);
    assert_eq!(
        answer(&mut probe, &good, id)["Messages"][0]["Payload"]["Number"],
        0
    );

    let id = "1700000000021-Aggregation/vendor.com/Client/C-1/1";
    let refused = serde_json::json!({"Location": {"Value": "+forbidden"}});
    get(
        &mut probe,
        format!("{prefix}Set/ReferenceDesignation/{it}"),
        &request(id, REFERENCE_DESIGNATION, &dsm(&refused)),
    );
    assert_eq!(
        answer(&mut probe, &bad, id)["Messages"][0]["Payload"]["Number"],
        StatusCode::BadOutOfRange.code()
    );

    let id = "1700000000022-Aggregation/vendor.com/Client/C-1/1";
    get(
        &mut probe,
        format!("{prefix}Get/ReferenceDesignation/{it}"),
        &request(id, REFERENCE_DESIGNATION, &serde_json::json!([])),
    );
    assert_eq!(
        answer(&mut probe, &rd_topic, id)["Messages"][0]["Payload"],
        rd
    );

    for (id, code) in [
        ("1700000000023-Aggregation/vendor.com/Client/C-1/1", None),
        (
            "1700000000024-Aggregation/vendor.com/Client/C-1/1",
            Some(StatusCode::BadNotFound),
        ),
    ] {
        get(
            &mut probe,
            format!("{prefix}Del/ReferenceDesignation/{it}"),
            &request(id, REFERENCE_DESIGNATION, &serde_json::json!([])),
        );
        let (topic, number) = code.map_or((&good, 0), |c| (&bad, c.code()));
        assert_eq!(
            answer(&mut probe, topic, id)["Messages"][0]["Payload"]["Number"],
            number
        );
    }
    assert!(pump.reference_designation().is_none());

    generic_event(&mut probe, &pump, &format!("{prefix}Pub/Event/{it}"));

    handle.shutdown();
    run.join().unwrap().unwrap();
    probe.disconnect();
}
