//! The in-process MQTT 3.1.1 test broker over TLS of the runtime tests:
//! the codec of `oec-core` and rustls on blocking threads, a PKI generated
//! per test with rcgen. Only loopback connections, no network.

#![allow(dead_code)]

use std::io::{ErrorKind, Read, Write};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::sync::mpsc::{Receiver, RecvTimeoutError, Sender, channel};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

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

/// Upper bound for every wait of a test.
pub(crate) const WAIT: Duration = Duration::from_secs(15);

// ---------------------------------------------------------------- PKI

/// A CA with a server certificate for `localhost`/127.0.0.1 and a client
/// certificate (PEM).
pub(crate) struct Pki {
    pub(crate) ca: String,
    pub(crate) server_cert: String,
    pub(crate) server_key: String,
    pub(crate) client_cert: String,
    pub(crate) client_key: KeyPair,
}

pub(crate) fn pki() -> Pki {
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
pub(crate) fn encrypted_pem(key: &KeyPair, passphrase: &str) -> String {
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
pub(crate) fn provider() -> Arc<rustls::crypto::CryptoProvider> {
    #[cfg(feature = "crypto-ring")]
    let provider = rustls::crypto::ring::default_provider();
    #[cfg(not(feature = "crypto-ring"))]
    let provider = rustls::crypto::aws_lc_rs::default_provider();
    Arc::new(provider)
}

// ---------------------------------------------------------------- broker

/// What the test broker saw, per connection (numbered from 0).
#[derive(Debug)]
pub(crate) enum Seen {
    /// TLS handshake done; whether the client presented a certificate.
    Tls(usize, bool),
    /// A packet from the client.
    Packet(usize, Packet),
    /// The connection ended (EOF, error or dropped by the test).
    Closed(usize),
}

/// How the broker behaves.
#[derive(Clone, Copy)]
pub(crate) struct Behavior {
    /// Answer of the first CONNECT; later ones are accepted.
    pub(crate) first_connack: ConnectReturnCode,
    /// Answer PINGREQ.
    pub(crate) ping: bool,
    /// Require a client certificate.
    pub(crate) mutual_tls: bool,
    /// Offer TLS 1.2 only.
    pub(crate) tls12_only: bool,
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

/// Packets the test sends to a client: `(connection, packet)`.
pub(crate) type Outbox = Arc<Mutex<Vec<(usize, Packet)>>>;

pub(crate) struct TestBroker {
    pub(crate) port: u16,
    pub(crate) seen: Receiver<Seen>,
    sockets: Arc<Mutex<Vec<TcpStream>>>,
    outbox: Outbox,
}

impl TestBroker {
    pub(crate) fn start(pki: &Pki, behavior: Behavior) -> Self {
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
        let outbox = Outbox::default();
        let accepted = Arc::clone(&sockets);
        let queued = Arc::clone(&outbox);
        thread::spawn(move || {
            for (n, tcp) in listener.incoming().enumerate() {
                let Ok(tcp) = tcp else { return };
                accepted.lock().unwrap().push(tcp.try_clone().unwrap());
                let (config, tx, queued) = (Arc::clone(&config), tx.clone(), Arc::clone(&queued));
                thread::spawn(move || serve(n, tcp, &config, behavior, &tx, &queued));
            }
        });
        Self {
            port,
            seen,
            sockets,
            outbox,
        }
    }

    /// Sends `packet` to the client of connection `n` (e.g. a `Get`).
    pub(crate) fn send(&self, n: usize, packet: Packet) {
        self.outbox.lock().unwrap().push((n, packet));
    }

    /// Next event, failing the test after [`WAIT`].
    pub(crate) fn next(&self) -> Seen {
        match self.seen.recv_timeout(WAIT) {
            Ok(s) => s,
            Err(RecvTimeoutError::Timeout) => panic!("test broker: nothing within {WAIT:?}"),
            Err(e) => panic!("test broker: {e}"),
        }
    }

    /// The next packet (skipping TLS events), with its connection.
    pub(crate) fn packet(&self) -> (usize, Packet) {
        loop {
            match self.next() {
                Seen::Packet(n, p) => return (n, p),
                Seen::Tls(..) => {}
                Seen::Closed(n) => panic!("connection {n} closed, expected a packet"),
            }
        }
    }

    pub(crate) fn connect(&self) -> (usize, Connect) {
        match self.packet() {
            (n, Packet::Connect(c)) => (n, c),
            other => panic!("expected CONNECT, got {other:?}"),
        }
    }

    pub(crate) fn publish(&self) -> (usize, Publish) {
        match self.packet() {
            (n, Packet::Publish(p)) => (n, p),
            other => panic!("expected PUBLISH, got {other:?}"),
        }
    }

    /// Waits until connection `n` is closed, returning the packets before.
    pub(crate) fn until_closed(&self, n: usize) -> Vec<Packet> {
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
    pub(crate) fn drop_connection(&self, n: usize) {
        let _ = self.sockets.lock().unwrap()[n].shutdown(Shutdown::Both);
    }
}

pub(crate) fn serve(
    n: usize,
    tcp: TcpStream,
    config: &Arc<ServerConfig>,
    b: Behavior,
    tx: &Sender<Seen>,
    outbox: &Outbox,
) {
    // A short read timeout lets the loop send the packets of the outbox.
    tcp.set_read_timeout(Some(Duration::from_millis(10)))
        .unwrap();
    let mut tls = StreamOwned::new(ServerConnection::new(Arc::clone(config)).unwrap(), tcp);
    let mut decoder = Decoder::new(1 << 20);
    let mut buf = [0; 4096];
    let mut handshaken = false;
    loop {
        let read = match tls.read(&mut buf) {
            Err(e) if matches!(e.kind(), ErrorKind::WouldBlock | ErrorKind::TimedOut) => {
                let mine: Vec<Packet> = {
                    let mut queued = outbox.lock().unwrap();
                    let (mine, rest) = queued.drain(..).partition(|(m, _)| *m == n);
                    *queued = rest;
                    mine.into_iter().map(|(_, p)| p).collect()
                };
                for p in mine {
                    let mut out = Vec::new();
                    encode(&p, &mut out).unwrap();
                    if tls.write_all(&out).and_then(|()| tls.flush()).is_err() {
                        break;
                    }
                }
                continue;
            }
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
