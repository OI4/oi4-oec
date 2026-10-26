//! TLS transport of the MQTT connection (ADR-0023 §1, amendment C-050):
//! tokio `TcpStream` + tokio-rustls. rustls speaks only TLS 1.2 and 1.3
//! (REQ-MQTT-005); the broker certificate is verified against the CA of the
//! connection settings only, never the OS trust store (REQ-MQTT-008); a
//! client certificate is used for mutual TLS, otherwise the credentials go
//! into CONNECT (REQ-MQTT-009/010).

use std::future::{Future, poll_fn};
use std::io;
use std::path::Path;
use std::pin::{Pin, pin};
use std::sync::Arc;
use std::task::Poll;
use std::time::Duration;

use pkcs8::der::Decode;
use rustls::client::ClientConfig;
use rustls::crypto::CryptoProvider;
use rustls::{RootCertStore, version};
use rustls_pki_types::pem::PemObject;
use rustls_pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer, ServerName};
use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
use tokio::net::TcpStream;
use tokio_rustls::TlsConnector;
use tokio_rustls::client::TlsStream;

use crate::app::{Auth, ConnectionSettings, Secret};
use crate::{Error, Result};

/// The TLS stream of one connection.
pub(crate) type Stream = TlsStream<TcpStream>;

/// PEM label of a PKCS#8 `EncryptedPrivateKeyInfo` (RFC 5958, RFC 7468).
const ENCRYPTED_PKCS8: &str = "-----BEGIN ENCRYPTED PRIVATE KEY-----";
/// Header of the legacy OpenSSL PEM encryption (RFC 1421), not supported.
const LEGACY_ENCRYPTED: &str = "Proc-Type: 4,ENCRYPTED";

/// The crypto provider of rustls: ring with the feature `crypto-ring`,
/// otherwise aws-lc-rs (ADR-0023 §1).
fn provider() -> Arc<CryptoProvider> {
    #[cfg(feature = "crypto-ring")]
    {
        Arc::new(rustls::crypto::ring::default_provider())
    }
    #[cfg(all(feature = "crypto-aws-lc-rs", not(feature = "crypto-ring")))]
    {
        Arc::new(rustls::crypto::aws_lc_rs::default_provider())
    }
}

#[cfg(not(any(feature = "crypto-aws-lc-rs", feature = "crypto-ring")))]
compile_error!("oec needs a crypto provider: enable `crypto-aws-lc-rs` (default) or `crypto-ring`");

/// The TLS client of the connection settings: TLS 1.3 and 1.2 only
/// (REQ-MQTT-005), the roots of the CA file only (REQ-MQTT-008), the
/// client certificate for mutual TLS (REQ-MQTT-010).
///
/// # Errors
///
/// [`Error::Bootstrap`] if a file is missing, holds no usable PEM item or
/// the key cannot be decrypted (REQ-CNT-008/009).
pub(crate) fn client_config(settings: &ConnectionSettings) -> Result<Arc<ClientConfig>> {
    let roots = roots(settings.ca())?;
    let builder = ClientConfig::builder_with_provider(provider())
        .with_protocol_versions(&[&version::TLS13, &version::TLS12])
        .map_err(|e| Error::Bootstrap(format!("TLS: {e}")))?
        .with_root_certificates(roots);
    let config = match settings.auth() {
        Auth::ClientCertificate {
            cert,
            key,
            passphrase,
        } => {
            let chain = certificates(cert)?;
            let key = private_key(key, passphrase.as_ref())?;
            builder.with_client_auth_cert(chain, key).map_err(|e| {
                Error::Bootstrap(format!(
                    "client certificate {} and its key do not fit: {e} (REQ-MQTT-009)",
                    cert.display()
                ))
            })?
        }
        Auth::Credentials { .. } => builder.with_no_client_auth(),
    };
    Ok(Arc::new(config))
}

/// The trust anchors of the CA file (REQ-MQTT-008).
fn roots(ca: &Path) -> Result<RootCertStore> {
    let mut roots = RootCertStore::empty();
    let (added, _ignored) = roots.add_parsable_certificates(certificates(ca)?);
    if added == 0 {
        return Err(Error::Bootstrap(format!(
            "{}: no usable CA certificate (REQ-MQTT-008)",
            ca.display()
        )));
    }
    Ok(roots)
}

/// All certificates of a PEM file, at least one.
fn certificates(path: &Path) -> Result<Vec<CertificateDer<'static>>> {
    let pem = read_file(path)?;
    let certs = CertificateDer::pem_slice_iter(&pem)
        .collect::<std::result::Result<Vec<_>, _>>()
        .map_err(|e| Error::Bootstrap(format!("{}: {e}", path.display())))?;
    if certs.is_empty() {
        return Err(Error::Bootstrap(format!(
            "{}: no PEM certificate",
            path.display()
        )));
    }
    Ok(certs)
}

/// The private key of a PEM file: PKCS#8, PKCS#1 (RSA) or SEC1 (EC), or a
/// PKCS#8 `ENCRYPTED PRIVATE KEY` (PBES2: PBKDF2 or scrypt with AES)
/// decrypted with the passphrase (REQ-CNT-008/009). The legacy OpenSSL
/// encryption (`Proc-Type: 4,ENCRYPTED`) and PBES1 (DES, RC2) are refused.
pub(crate) fn private_key(
    path: &Path,
    passphrase: Option<&Secret>,
) -> Result<PrivateKeyDer<'static>> {
    let pem = read_file(path)?;
    let text = String::from_utf8_lossy(&pem);
    let bootstrap = |what: &str| Error::Bootstrap(format!("{}: {what}", path.display()));
    if text.contains(ENCRYPTED_PKCS8) {
        let Some(passphrase) = passphrase else {
            return Err(bootstrap(
                "the key is encrypted, but there is no passphrase (REQ-CNT-008/009)",
            ));
        };
        let start = text.find(ENCRYPTED_PKCS8).unwrap_or(0);
        // PEM → DER first: the PEM reader of `der` cannot decode the
        // algorithm parameters of PBES2.
        let (_, der) = pkcs8::der::pem::decode_vec(text[start..].as_bytes())
            .map_err(|e| bootstrap(&format!("no PKCS#8 encrypted key: {e}")))?;
        let info = pkcs8::EncryptedPrivateKeyInfoRef::from_der(&der)
            .map_err(|e| bootstrap(&format!("no PKCS#8 encrypted key: {e}")))?;
        let doc = info.decrypt(passphrase.expose()).map_err(|e| {
            bootstrap(&format!(
                "cannot decrypt the key: {e} (wrong passphrase or not PBES2 with \
                 PBKDF2/scrypt and AES; REQ-CNT-009)"
            ))
        })?;
        return Ok(PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(
            doc.as_bytes().to_vec(),
        )));
    }
    if text.contains(LEGACY_ENCRYPTED) {
        return Err(bootstrap(
            "legacy OpenSSL key encryption is not supported; convert the key to an \
             encrypted PKCS#8 key (`openssl pkcs8 -topk8 -v2 aes-256-cbc`) (REQ-CNT-009)",
        ));
    }
    if passphrase.is_some() {
        log::info!(
            "{}: the key is not encrypted, the passphrase is not used",
            path.display()
        );
    }
    PrivateKeyDer::from_pem_slice(&pem).map_err(|e| bootstrap(&format!("no PEM private key: {e}")))
}

fn read_file(path: &Path) -> Result<Vec<u8>> {
    std::fs::read(path).map_err(|e| Error::Bootstrap(format!("{}: {e}", path.display())))
}

/// The name the broker certificate is verified against: the host name or
/// IP address of the settings (REQ-MQTT-008).
pub(crate) fn server_name(host: &str) -> Result<ServerName<'static>> {
    ServerName::try_from(host.to_owned()).map_err(|e| {
        Error::Invalid(format!(
            "broker host {host:?} is no DNS name or IP address for TLS: {e}"
        ))
    })
}

/// TCP connect and TLS handshake within `timeout`.
pub(crate) async fn connect(
    settings: &ConnectionSettings,
    name: ServerName<'static>,
    config: Arc<ClientConfig>,
    timeout: Duration,
) -> io::Result<Stream> {
    let attempt = async {
        let tcp = TcpStream::connect((settings.host(), settings.port())).await?;
        tcp.set_nodelay(true)?;
        TlsConnector::from(config).connect(name, tcp).await
    };
    tokio::time::timeout(timeout, attempt)
        .await
        .map_err(|_| io::Error::new(io::ErrorKind::TimedOut, "TCP/TLS connect timed out"))?
}

/// Reads what is available (0 = end of stream). Cancel safe: no data is
/// consumed unless the future completes.
pub(crate) async fn read<S: AsyncRead + Unpin>(
    stream: &mut S,
    buf: &mut [u8],
) -> io::Result<usize> {
    poll_fn(|cx| {
        let mut rb = ReadBuf::new(buf);
        match Pin::new(&mut *stream).poll_read(cx, &mut rb) {
            Poll::Ready(Ok(())) => Poll::Ready(Ok(rb.filled().len())),
            Poll::Ready(Err(e)) => Poll::Ready(Err(e)),
            Poll::Pending => Poll::Pending,
        }
    })
    .await
}

/// Writes all of `buf` and flushes.
pub(crate) async fn write_all<S: AsyncWrite + Unpin>(
    stream: &mut S,
    mut buf: &[u8],
) -> io::Result<()> {
    while !buf.is_empty() {
        let n = poll_fn(|cx| Pin::new(&mut *stream).poll_write(cx, buf)).await?;
        if n == 0 {
            return Err(io::ErrorKind::WriteZero.into());
        }
        buf = &buf[n..];
    }
    poll_fn(|cx| Pin::new(&mut *stream).poll_flush(cx)).await
}

/// Sends the TLS close_notify and closes the write side.
pub(crate) async fn shutdown<S: AsyncWrite + Unpin>(stream: &mut S) -> io::Result<()> {
    poll_fn(|cx| Pin::new(&mut *stream).poll_shutdown(cx)).await
}

/// The first of two futures to complete.
pub(crate) enum Either<A, B> {
    Left(A),
    Right(B),
}

/// Polls `a`, then `b`; returns the first result (the other is dropped).
pub(crate) async fn race<A: Future, B: Future>(a: A, b: B) -> Either<A::Output, B::Output> {
    let mut a = pin!(a);
    let mut b = pin!(b);
    poll_fn(|cx| {
        if let Poll::Ready(x) = a.as_mut().poll(cx) {
            return Poll::Ready(Either::Left(x));
        }
        if let Poll::Ready(y) = b.as_mut().poll(cx) {
            return Poll::Ready(Either::Right(y));
        }
        Poll::Pending
    })
    .await
}
