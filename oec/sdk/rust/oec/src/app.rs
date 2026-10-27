//! Application: level selection, bootstrap, identity and the runtime
//! (ADR-0020 §2 … §5, §10): [`AppBuilder::build`] resolves everything that
//! needs no connection; [`App::run`] / [`App::run_async`] connect over TLS
//! and drive the sans-IO session and provider of `oec-core` (P4 increment
//! 5b, C-050; 6a, C-053; 6b, C-054); [`AppHandle`] registers assets, sets
//! their state and observes and stops the application
//! ([`ConnectionState`]).

use std::collections::{HashMap, HashSet};
use std::fmt;
use std::future::Future;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, MutexGuard, PoisonError};
use std::time::Duration;

use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::{
    MetaDataVersions, Provider, ProviderConfig, PublicationOverrides, SetConfigRequest,
    SetDataRequest, StoredConfigValues, WriterIds,
};
use oec_core::resource::Health;
use tokio::sync::{Notify, watch};

use crate::asset::{Asset, AssetDef};
use crate::config::{ConfigDef, ConfigHandle, ConfigSetHandler, ConfigValues};
use crate::container::{self, MAX_PACKET_SIZE, MIN_PACKET_SIZE};
use crate::data::{DataSet, DataSetDef, SetData, SetHandler, Value};
use crate::event::SyslogLevel;
use crate::identity::{IdentificationLink, Mam, Product};
use crate::store::{FileStore, MemoryStore, Store};
use crate::{Error, Level, Result, ServiceType, runtime};

/// Store key of the RevisionCounter of the application (REQ-RES-MAM-018).
const REVISION_COUNTER: &str = "app/revision-counter";
/// Store key of the DataSetWriterIds (REQ-RES-PUBLIST-006, ADR-0026 §12).
const WRITER_IDS: &str = "app/writer-ids";
/// Store key of the ConfigurationVersions of the Metadata and the first
/// sending of each version (REQ-MSG-032/036, ADR-0026 §12.6).
const META_DATA_VERSIONS: &str = "app/metadata-versions";
/// Store key of the settings changed by `Set PublicationList`
/// (REQ-RES-PUBLIST-016, ADR-0026 §14).
const PUBLICATIONS: &str = "app/publications";
/// Store key of the Config values set by `Set Config` or
/// [`ConfigHandle::set`] (REQ-RES-CONFIG-022, ADR-0026 §15). Sensitive
/// values are stored in clear text: protect the store directory.
const CONFIG_VALUES: &str = "app/config";

/// Where connection settings and secrets come from (ADR-0020 §5).
#[derive(Debug, Clone)]
pub enum Bootstrap {
    /// Container storage contract of the RIG (optional, ADR-0011):
    /// `/etc/oi4/mqtt/broker.json` (Address, SecurePort, MaxPacketSize;
    /// unknown keys ignored, REQ-CNT-003/004), `broker_ca.pem` else
    /// `/etc/oi4/certs/ca.pem` (REQ-CNT-004/005),
    /// `/run/secrets/oi4/{credentials,private_key.pem,passphrase}` and
    /// `/etc/oi4/certs/<containername>.pem` (REQ-CNT-008/009), the container
    /// name from the hostname (`/proc/sys/kernel/hostname`, else
    /// `/etc/hostname`; REQ-CNT-021) as serial number, certificate name and
    /// MQTT ClientId (REQ-MQTT-013), and a file store below
    /// `/etc/oi4/app/oec` if the platform provides the application specific
    /// volume (REQ-CNT-010/011). `root` prefixes all paths (tests, dev
    /// environment); default `/`.
    ContainerStorage {
        /// Prefix of all container paths.
        root: PathBuf,
    },
    /// Explicit configuration, e.g. a smart device or a test without
    /// container (ADR-0011: the SDK works without the container contract).
    Explicit(Broker),
}

impl Bootstrap {
    /// `ContainerStorage` with root `/`.
    pub fn container() -> Self {
        Bootstrap::ContainerStorage {
            root: PathBuf::from("/"),
        }
    }
}

/// Explicit broker settings. TLS is always on (REQ-MQTT-005, port 8883 by
/// default, REQ-MQTT-004); there is no API for a plain connection.
#[derive(Debug, Clone)]
pub struct Broker {
    host: String,
    port: u16,
    ca: Option<PathBuf>,
    credentials: Option<(String, Secret)>,
    client_cert: Option<(PathBuf, PathBuf)>,
    max_packet_size: usize,
}

impl Broker {
    /// Port 8883, MaxPacketSize 262144 bytes (RIG 4.1.1 minimum).
    /// `host` is an IP address or host name (the grammar of the `Address` of
    /// `broker.json`, REQ-CNT-003), checked by [`AppBuilder::build`].
    pub fn new(host: &str) -> Self {
        Broker {
            host: host.to_owned(),
            port: 8883,
            ca: None,
            credentials: None,
            client_cert: None,
            max_packet_size: MIN_PACKET_SIZE,
        }
    }

    /// TLS port of the broker (REQ-MQTT-004), 1 … 65535.
    #[must_use]
    pub fn port(mut self, port: u16) -> Self {
        self.port = port;
        self
    }

    /// CA that verifies the broker certificate (REQ-MQTT-008); required.
    /// Trust comes only from this file, not from the OS trust store
    /// (ADR-0023 §1); for a broker with a certificate of a public CA, give
    /// that CA's root certificate.
    #[must_use]
    pub fn ca_pem(mut self, pem_file: impl Into<PathBuf>) -> Self {
        self.ca = Some(pem_file.into());
        self
    }

    /// Username + password (REQ-MQTT-009). The username is used as provided
    /// (REQ-MQTT-011).
    #[must_use]
    pub fn credentials(mut self, username: &str, password: &str) -> Self {
        self.credentials = Some((username.to_owned(), Secret(password.as_bytes().to_vec())));
        self
    }

    /// Client certificate (mutual TLS). Wins over credentials if both are
    /// given (REQ-MQTT-010).
    #[must_use]
    pub fn client_cert(
        mut self,
        cert_pem: impl Into<PathBuf>,
        key_pem: impl Into<PathBuf>,
    ) -> Self {
        self.client_cert = Some((cert_pem.into(), key_pem.into()));
        self
    }

    /// MaxPacketSize of the Message Bus, 262144 … 268435455 bytes (RIG
    /// 4.1.1, REQ-CNT-003).
    #[must_use]
    pub fn max_packet_size(mut self, bytes: usize) -> Self {
        self.max_packet_size = bytes;
        self
    }

    /// Checks the settings and picks the means of authentication.
    fn resolve(self) -> Result<(String, u16, PathBuf, usize, Auth)> {
        if !crate::address::valid(&self.host) {
            return Err(Error::Invalid(format!(
                "broker host {:?} is no IP address or host name (REQ-CNT-003)",
                self.host
            )));
        }
        if self.port == 0 {
            return Err(Error::Invalid("broker port 0 (REQ-CNT-003)".to_owned()));
        }
        if !(MIN_PACKET_SIZE..=MAX_PACKET_SIZE).contains(&self.max_packet_size) {
            return Err(Error::Invalid(format!(
                "broker MaxPacketSize {} is outside {MIN_PACKET_SIZE} … {MAX_PACKET_SIZE} \
                 (RIG 4.1.1, REQ-CNT-003)",
                self.max_packet_size
            )));
        }
        for f in self
            .ca
            .iter()
            .chain(self.client_cert.iter().flat_map(|(c, k)| [c, k]))
        {
            existing(f)?;
        }
        let auth = match (self.client_cert, self.credentials) {
            (Some((cert, key)), credentials) => {
                if credentials.is_some() {
                    log::info!("client certificate wins over the credentials (REQ-MQTT-010)");
                }
                Auth::ClientCertificate {
                    cert,
                    key,
                    passphrase: None,
                }
            }
            (None, Some((username, password))) => Auth::Credentials { username, password },
            (None, None) => {
                return Err(Error::LevelNotMet(
                    "the broker needs credentials or a client certificate: every level \
                     authenticates (REQ-CMP-002, REQ-MQTT-009)"
                        .to_owned(),
                ));
            }
        };
        let Some(ca) = self.ca else {
            return Err(Error::LevelNotMet(
                "the broker needs a CA (Broker::ca_pem): every level verifies the broker \
                 certificate (REQ-CMP-002, REQ-MQTT-008)"
                    .to_owned(),
            ));
        };
        Ok((self.host, self.port, ca, self.max_packet_size, auth))
    }
}

fn existing(path: &Path) -> Result<()> {
    std::fs::metadata(path).map(|_| ()).map_err(|e| {
        Error::Io(std::io::Error::new(
            e.kind(),
            format!("{}: {e}", path.display()),
        ))
    })
}

/// Secret octets (password, key passphrase). `Debug` never shows them
/// (G-4.5).
#[derive(Clone, PartialEq, Eq)]
pub struct Secret(pub(crate) Vec<u8>);

impl Secret {
    /// The octets, for the TLS/MQTT layer.
    pub fn expose(&self) -> &[u8] {
        &self.0
    }
}

impl fmt::Debug for Secret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("Secret(***)")
    }
}

/// The means of authentication at the Message Bus (REQ-MQTT-009, REQ-MQTT-010).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum Auth {
    /// Mutual TLS with the certificate of the application.
    ClientCertificate {
        /// PEM certificate (`/etc/oi4/certs/<containername>.pem`, REQ-CNT-009).
        cert: PathBuf,
        /// PEM private key (`/run/secrets/oi4/private_key.pem`).
        key: PathBuf,
        /// Decoded passphrase of an encrypted key (REQ-CNT-008/009).
        passphrase: Option<Secret>,
    },
    /// Username and password.
    Credentials {
        /// Username, used as provided (REQ-MQTT-011).
        username: String,
        /// Password octets.
        password: Secret,
    },
}

/// The resolved connection settings of an [`App`] (read-only).
#[derive(Debug, Clone)]
pub struct ConnectionSettings {
    host: String,
    port: u16,
    ca: PathBuf,
    auth: Auth,
    client_id: String,
    keep_alive: Duration,
    max_packet_size: usize,
    publish_limit: usize,
}

impl ConnectionSettings {
    /// Broker address (REQ-CNT-003/004).
    pub fn host(&self) -> &str {
        &self.host
    }

    /// TLS port (REQ-MQTT-004).
    pub fn port(&self) -> u16 {
        self.port
    }

    /// CA that verifies the broker (REQ-MQTT-008): `broker_ca.pem` or
    /// `ca.pem` of the container storage, or [`Broker::ca_pem`].
    pub fn ca(&self) -> &Path {
        &self.ca
    }

    /// Means of authentication (REQ-MQTT-009/010).
    pub fn auth(&self) -> &Auth {
        &self.auth
    }

    /// MQTT ClientId (REQ-MQTT-013).
    pub fn client_id(&self) -> &str {
        &self.client_id
    }

    /// MQTT keep alive (REQ-MQTT-014).
    pub fn keep_alive(&self) -> Duration {
        self.keep_alive
    }

    /// MaxPacketSize of the Message Bus (RIG 4.1.1).
    pub fn max_packet_size(&self) -> usize {
        self.max_packet_size
    }

    /// Own limit for publications not triggered by a `Get`, at most
    /// [`Self::max_packet_size`] (REQ-PAG-005).
    pub fn publish_limit(&self) -> usize {
        self.publish_limit
    }
}

/// Builder of an [`App`]. Created by [`App::builder`].
pub struct AppBuilder {
    level: Level,
    service_type: ServiceType,
    product: Product,
    bootstrap: Bootstrap,
    identification_link: Option<IdentificationLink>,
    serial_number: Option<String>,
    client_id: Option<String>,
    keep_alive: Duration,
    store: Option<Arc<dyn Store>>,
    max_packet_size: Option<usize>,
    syslog: SyslogLevel,
    reference_designation: Option<bool>,
    on_connection: Option<Callback>,
}

impl AppBuilder {
    /// Default: [`Bootstrap::container`].
    #[must_use]
    pub fn bootstrap(mut self, bootstrap: Bootstrap) -> Self {
        self.bootstrap = bootstrap;
        self
    }

    /// Use an existing Identification Link (REQ-IL-006, REQ-IL-031).
    /// Without it, the SDK generates the Oi4Identifier from the
    /// [`Product`] and the serial number (REQ-IL-007, REQ-RES-MAM-006).
    #[must_use]
    pub fn identification_link(mut self, il: IdentificationLink) -> Self {
        self.identification_link = Some(il);
        self
    }

    /// Serial number outside a container (REQ-RES-MAM-015); required with
    /// [`Bootstrap::Explicit`]. In a container the container name is used
    /// (REQ-CNT-021); an explicit serial number is then only allowed with
    /// an explicit Identification Link.
    #[must_use]
    pub fn serial_number(mut self, serial: &str) -> Self {
        self.serial_number = Some(serial.to_owned());
        self
    }

    /// MQTT ClientId outside a container; default derived from the
    /// Identification Link, stable across reconnects (REQ-MQTT-013): `oec`
    /// and 16 hex digits (19 characters, within the limits every MQTT 3.1.1
    /// broker accepts). In a container the container name is the ClientId.
    #[must_use]
    pub fn client_id(mut self, client_id: &str) -> Self {
        self.client_id = Some(client_id.to_owned());
        self
    }

    /// Keep alive, default 60 s (REQ-MQTT-014), whole seconds 1 … 65535.
    /// Clean session, QoS 0 and no retain are fixed (REQ-MQTT-015,
    /// REQ-MQTT-017, REQ-MQTT-018).
    #[must_use]
    pub fn keep_alive(mut self, keep_alive: Duration) -> Self {
        self.keep_alive = keep_alive;
        self
    }

    /// Persistence of SDK state: RevisionCounter (REQ-RES-MAM-018),
    /// ReferenceDesignation (REQ-RES-REFDES-002), changed PublicationList /
    /// SubscriptionList entries and Config values. Default: a file store
    /// under `/etc/oi4/app/oec` in a container with the application specific
    /// volume (REQ-CNT-010), in-memory otherwise. A ReferenceDesignation
    /// needs a persistent store ([`Store::is_persistent`]).
    #[must_use]
    pub fn store(mut self, store: impl Store + 'static) -> Self {
        self.store = Some(Arc::new(store));
        self
    }

    /// Own limit for publications not triggered by a `Get` (REQ-PAG-005);
    /// never above the MaxPacketSize of the Message Bus (clamped).
    #[must_use]
    pub fn max_packet_size(mut self, bytes: usize) -> Self {
        self.max_packet_size = Some(bytes);
        self
    }

    /// Lowest Syslog level that is published (default `Notice`; `Debug`
    /// and `Informational` are off by default, REQ-TOP-024, REQ-TOP-027).
    /// The SDK installs a `log`/`tracing` bridge (ADR-0020 §6).
    #[must_use]
    pub fn syslog(mut self, level: SyslogLevel) -> Self {
        self.syslog = level;
        self
    }

    /// Whether the application offers the resource ReferenceDesignation
    /// with `Get` and `Set` (REQ-CMP-020: Level 2 SHOULD, Level 3 MUST).
    /// Default: off at Level 1, on at Level 2 and 3. An accepted
    /// ReferenceDesignation is persisted (REQ-RES-REFDES-002), so it needs a
    /// persistent [`Store`].
    #[must_use]
    pub fn reference_designation(mut self, offered: bool) -> Self {
        self.reference_designation = Some(offered);
        self
    }

    /// Called on every change of the [`ConnectionState`], on the thread of
    /// the runtime: keep it short and do not block.
    #[must_use]
    pub fn on_connection(mut self, f: impl Fn(ConnectionState) + Send + Sync + 'static) -> Self {
        self.on_connection = Some(Box::new(f));
        self
    }

    /// Validates identity, bootstrap and level (e.g. a ReferenceDesignation
    /// without a persistent store → [`Error::LevelNotMet`]). Reads the
    /// container storage and the store; does not connect.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] for an invalid identity or setting,
    /// [`Error::LevelNotMet`] if the level cannot be kept,
    /// [`Error::Bootstrap`] for a missing or invalid container storage and
    /// [`Error::Io`] for store and file failures.
    pub fn build(self) -> Result<App> {
        let keep_alive = keep_alive(self.keep_alive)?;
        let (resolved, default_store) = self.resolve_bootstrap()?;
        let il = match self.identification_link {
            Some(il) => il,
            None => self.product.oi4_identifier(&resolved.serial_number)?,
        };
        let store = self.store.unwrap_or(default_store);
        let reference_designation =
            reference_designation(self.level, self.reference_designation, store.as_ref())?;
        let revision_counter = revision_counter(store.as_ref(), REVISION_COUNTER)?;
        let mam = self.product.mam(
            Some(self.service_type),
            &resolved.serial_number,
            &il,
            revision_counter,
        )?;
        let publish_limit = publish_limit(self.max_packet_size, resolved.max_packet_size)?;
        let client_id = resolved.client_id.unwrap_or_else(|| derived_client_id(&il));
        let provider = provider(
            self.level,
            self.service_type,
            &il,
            mam,
            resolved.max_packet_size,
            publish_limit,
            store.as_ref(),
        )?;
        Ok(App {
            level: self.level,
            service_type: self.service_type,
            connection: ConnectionSettings {
                host: resolved.host,
                port: resolved.port,
                ca: resolved.ca,
                auth: resolved.auth,
                client_id,
                keep_alive,
                max_packet_size: resolved.max_packet_size,
                publish_limit,
            },
            reference_designation,
            syslog: self.syslog,
            handle: AppHandle::new(self.on_connection, il, store, provider),
        })
    }

    /// Connection settings, serial number and default store of the
    /// bootstrap (ADR-0020 §5).
    fn resolve_bootstrap(&self) -> Result<(Resolved, Arc<dyn Store>)> {
        match &self.bootstrap {
            Bootstrap::ContainerStorage { root } => {
                let c = container::load(root)?;
                if self.serial_number.is_some() && self.identification_link.is_none() {
                    return Err(Error::Invalid(
                        "in a container the container name is the SerialNumber of the \
                         Oi4Identifier (REQ-CNT-021); a serial number is only allowed with \
                         an explicit Identification Link"
                            .to_owned(),
                    ));
                }
                if self.client_id.is_some() {
                    return Err(Error::Invalid(
                        "in a container the container name is the MQTT ClientId (REQ-MQTT-013)"
                            .to_owned(),
                    ));
                }
                let store: Arc<dyn Store> = if let Some(dir) = c.store_dir {
                    Arc::new(FileStore::new(dir))
                } else {
                    log::warn!(
                        "no application specific volume /etc/oi4/app (REQ-CNT-010); \
                         SDK state is not persisted"
                    );
                    Arc::new(MemoryStore::new())
                };
                let resolved = Resolved {
                    serial_number: self.serial_number.clone().unwrap_or_else(|| c.name.clone()),
                    client_id: Some(c.name),
                    host: c.broker.address,
                    port: c.broker.secure_port,
                    ca: c.ca,
                    max_packet_size: c.broker.max_packet_size,
                    auth: c.auth,
                };
                Ok((resolved, store))
            }
            Bootstrap::Explicit(broker) => {
                let (host, port, ca, max_packet_size, auth) = broker.clone().resolve()?;
                let serial_number = self.serial_number.clone().ok_or_else(|| {
                    Error::Invalid(
                        "a serial number is required outside a container \
                         (AppBuilder::serial_number, REQ-RES-MAM-015)"
                            .to_owned(),
                    )
                })?;
                let client_id = self.client_id.as_deref().map(client_id).transpose()?;
                let resolved = Resolved {
                    serial_number,
                    client_id,
                    host,
                    port,
                    ca,
                    max_packet_size,
                    auth,
                };
                Ok((resolved, Arc::new(MemoryStore::new())))
            }
        }
    }
}

/// Whether ReferenceDesignation is offered: default off at Level 1, on at
/// Level 2 (SHOULD) and Level 3 (MUST, REQ-CMP-020); it needs a persistent
/// store (REQ-RES-REFDES-002).
fn reference_designation(level: Level, offered: Option<bool>, store: &dyn Store) -> Result<bool> {
    let offered = offered.unwrap_or(level >= Level::L2);
    if level == Level::L3 && !offered {
        return Err(Error::LevelNotMet(
            "Level 3 requires the resource ReferenceDesignation (REQ-CMP-020)".to_owned(),
        ));
    }
    if offered && !store.is_persistent() {
        return Err(Error::LevelNotMet(
            "the ReferenceDesignation is persisted (REQ-RES-REFDES-002), the store is \
             not persistent; use a persistent store or, below Level 3, \
             reference_designation(false)"
                .to_owned(),
        ));
    }
    Ok(offered)
}

/// The own limit for publications not triggered by a `Get`, at most the
/// MaxPacketSize of the Message Bus (REQ-PAG-005).
fn publish_limit(own: Option<usize>, bus: usize) -> Result<usize> {
    match own {
        Some(0) => Err(Error::Invalid(
            "the own MaxPacketSize is 0 (REQ-PAG-005)".to_owned(),
        )),
        Some(n) if n > bus => {
            log::warn!(
                "own MaxPacketSize {n} above the {bus} bytes of the Message Bus; clamped \
                 (REQ-PAG-005)"
            );
            Ok(bus)
        }
        Some(n) => Ok(n),
        None => Ok(bus),
    }
}

/// Settings resolved from the bootstrap.
struct Resolved {
    serial_number: String,
    client_id: Option<String>,
    host: String,
    port: u16,
    ca: PathBuf,
    max_packet_size: usize,
    auth: Auth,
}

/// Whole seconds 1 … 65535 (the MQTT Keep Alive field, REQ-MQTT-014); 0
/// would switch off the detection of a lost connection that triggers the
/// Will (REQ-LIFE-008).
fn keep_alive(d: Duration) -> Result<Duration> {
    if d.subsec_nanos() == 0 && (1..=65_535).contains(&d.as_secs()) {
        Ok(d)
    } else {
        Err(Error::Invalid(format!(
            "keep alive {d:?} is not 1 … 65535 whole seconds (REQ-MQTT-014)"
        )))
    }
}

/// An explicit ClientId: a non-empty MQTT UTF-8 string without U+0000
/// (MQTT 3.1.1 3.1.3.1, 1.5.3).
fn client_id(id: &str) -> Result<String> {
    if id.is_empty() || id.len() > 65_535 || id.contains('\0') {
        return Err(Error::Invalid(format!(
            "MQTT ClientId {id:?} is empty, too long or contains U+0000 (REQ-MQTT-013)"
        )));
    }
    Ok(id.to_owned())
}

/// `oec` and the FNV-1a 64 hash of the Identification Link in lower-case
/// hex: unique per Identification Link in practice, stable across
/// reconnects and restarts (REQ-MQTT-013), 19 characters of `0-9a-z`.
fn derived_client_id(il: &IdentificationLink) -> String {
    format!("oec{:016x}", fnv1a64(il.as_str()))
}

/// FNV-1a, 64 bit: short, stable names derived from an Identification Link
/// (ClientId, store keys).
fn fnv1a64(s: &str) -> u64 {
    s.bytes().fold(0xcbf2_9ce4_8422_2325_u64, |h, b| {
        (h ^ u64::from(b)).wrapping_mul(0x0000_0100_0000_01b3)
    })
}

/// Store key of the RevisionCounter of a represented asset: an
/// Identification Link may have 255 characters and characters a store key
/// cannot hold, so the key uses its FNV-1a hash (ADR-0026 §12).
fn asset_revision_counter_key(il: &IdentificationLink) -> String {
    format!("assets/{:016x}/revision-counter", fnv1a64(il.as_str()))
}

/// The persisted RevisionCounter at `key` (REQ-RES-MAM-018), 0 if none.
fn revision_counter(store: &dyn Store, key: &str) -> Result<i32> {
    let Some(raw) = store.load(key)? else {
        return Ok(0);
    };
    std::str::from_utf8(&raw)
        .ok()
        .and_then(|s| s.trim().parse().ok())
        .ok_or_else(|| {
            Error::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("store key {key} holds no integer (REQ-RES-MAM-018)"),
            ))
        })
}

/// The provider of the application (ADR-0026 §12): its lifecycle with the
/// fixed DataSetWriterIds of Birth and Health (ADR-0026 §9.4/§9.5), `Get`,
/// `Set` and `Del` from Level 2, the persisted DataSetWriterIds, Metadata
/// versions, settings of `Set PublicationList` and Config values.
fn provider(
    level: Level,
    service_type: ServiceType,
    il: &IdentificationLink,
    mam: Mam,
    max_packet_size: usize,
    publish_limit: usize,
    store: &dyn Store,
) -> Result<Provider> {
    let writer = |id| {
        DataSetWriterId::new(id).map_err(|e| Error::Invalid(format!("DataSetWriterId {id}: {e}")))
    };
    let lifecycle = Lifecycle::new(
        service_type,
        il.0.clone(),
        mam,
        writer(runtime::MAM_WRITER)?,
        writer(runtime::HEALTH_WRITER)?,
    )
    .map_err(|e| Error::Invalid(format!("lifecycle: {e}")))?;
    let writers = match store.load(WRITER_IDS)? {
        Some(json) => WriterIds::from_json(&json).map_err(|e| {
            Error::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("store key {WRITER_IDS}: {e} (REQ-RES-PUBLIST-006)"),
            ))
        })?,
        None => WriterIds::new(),
    };
    let versions = match store.load(META_DATA_VERSIONS)? {
        Some(json) => MetaDataVersions::from_json(&json).map_err(|e| {
            Error::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("store key {META_DATA_VERSIONS}: {e} (REQ-MSG-036)"),
            ))
        })?,
        None => MetaDataVersions::new(),
    };
    let overrides = match store.load(PUBLICATIONS)? {
        Some(json) => PublicationOverrides::from_json(&json).map_err(|e| {
            Error::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("store key {PUBLICATIONS}: {e} (REQ-RES-PUBLIST-016)"),
            ))
        })?,
        None => PublicationOverrides::new(),
    };
    let config_values = match store.load(CONFIG_VALUES)? {
        Some(json) => StoredConfigValues::from_json(&json).map_err(|e| {
            Error::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("store key {CONFIG_VALUES}: {e} (REQ-RES-CONFIG-022)"),
            ))
        })?,
        None => StoredConfigValues::new(),
    };
    let config = ProviderConfig {
        get: level >= Level::L2,
        max_packet_size,
        publish_limit,
    };
    let mut provider = Provider::new(lifecycle, config, writers)?
        .with_meta_data_versions(versions)
        .with_publication_overrides(overrides)
        .with_config_values(config_values);
    if let Some(json) = provider.take_writer_ids() {
        store.save(WRITER_IDS, &json)?;
    }
    Ok(provider)
}

/// An OEC application: the resolved identity, MAM and connection settings
/// of a level, run by [`App::run`] or [`App::run_async`]. Derefs to
/// [`AppHandle`] for everything that is also possible while it runs.
pub struct App {
    level: Level,
    service_type: ServiceType,
    connection: ConnectionSettings,
    reference_designation: bool,
    syslog: SyslogLevel,
    handle: AppHandle,
}

impl App {
    /// `product` describes the application itself; it becomes its MAM.
    pub fn builder(level: Level, service_type: ServiceType, product: Product) -> AppBuilder {
        AppBuilder {
            level,
            service_type,
            product,
            bootstrap: Bootstrap::container(),
            identification_link: None,
            serial_number: None,
            client_id: None,
            keep_alive: Duration::from_secs(60),
            store: None,
            max_packet_size: None,
            syslog: SyslogLevel::Notice,
            reference_designation: None,
            on_connection: None,
        }
    }

    /// The Messaging compliance level.
    pub fn level(&self) -> Level {
        self.level
    }

    /// The ServiceType (REQ-TOP-005).
    pub fn service_type(&self) -> ServiceType {
        self.service_type
    }

    /// The resolved connection settings.
    pub fn connection(&self) -> &ConnectionSettings {
        &self.connection
    }

    /// Whether the resource ReferenceDesignation is offered (REQ-CMP-020).
    pub fn reference_designation(&self) -> bool {
        self.reference_designation
    }

    /// Lowest published Syslog level (REQ-TOP-024).
    pub fn syslog(&self) -> SyslogLevel {
        self.syslog
    }

    /// Connects and runs until SIGINT/SIGTERM (Windows: Ctrl-C, Close,
    /// Shutdown) or [`AppHandle::shutdown`]: TLS connect with Will
    /// (REQ-LIFE-008, REQ-LIFE-016), Birth (REQ-LIFE-001/015), Health at
    /// start, on change and every 60 s (REQ-TOP-017), the MAM and Health of
    /// the represented assets, from Level 2 Profile, PublicationList and the
    /// answers to `Get` (REQ-CMP-008), reconnect, Close on shutdown
    /// (REQ-LIFE-005). Blocking;
    /// starts its own runtime (a tokio `current_thread` runtime on the
    /// calling thread).
    ///
    /// A lost connection is not an error: the SDK reconnects with
    /// exponential backoff (1 s doubling to 60 s, with jitter) and reports
    /// [`ConnectionState::Reconnecting`].
    ///
    /// # Errors
    ///
    /// Before the first connection only: [`Error::Bootstrap`] if the CA,
    /// the client certificate or its key cannot be loaded (REQ-MQTT-008,
    /// REQ-CNT-009), [`Error::Invalid`] for a host that is no TLS server
    /// name, [`Error::Io`] if the runtime cannot start.
    pub fn run(self) -> Result<()> {
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_io()
            .enable_time()
            .build()?;
        let handle = self.handle();
        rt.block_on(async move {
            let signals = tokio::spawn(runtime::signals(handle));
            let result = runtime::run(self).await;
            signals.abort();
            result
        })
    }

    /// A handle that outlives `run` (threads, tasks, signal handlers).
    pub fn handle(&self) -> AppHandle {
        self.handle.clone()
    }

    /// Same as [`App::run`] inside an existing tokio runtime (with I/O and
    /// time enabled). The future is `Send`, so it can be spawned. It does
    /// not listen for signals: the application owns them and calls
    /// [`AppHandle::shutdown`].
    ///
    /// # Errors
    ///
    /// As [`App::run`].
    #[allow(clippy::manual_async_fn)]
    pub fn run_async(self) -> impl Future<Output = Result<()>> + Send {
        async move { runtime::run(self).await }
    }
}

impl std::ops::Deref for App {
    type Target = AppHandle;
    fn deref(&self) -> &AppHandle {
        &self.handle
    }
}

/// Connection state reported to [`AppBuilder::on_connection`] and by
/// [`AppHandle::state`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConnectionState {
    /// Connecting for the first time (TCP, TLS, CONNECT); retried with
    /// backoff until the broker accepts.
    Connecting,
    /// Connected; Birth sent as first message (REQ-LIFE-001, REQ-LIFE-015).
    Online,
    /// Connection lost; the SDK reconnects with backoff and sends Birth
    /// again (REQ-LIFE-001). The broker publishes the Will (REQ-LIFE-008).
    Reconnecting,
    /// Close sent, connection closed (REQ-LIFE-005, REQ-CMP-046). Also the
    /// state before `run` and after it returned: no connection, no attempt.
    Closed,
}

type Callback = Box<dyn Fn(ConnectionState) + Send + Sync>;

struct Shared {
    state: Mutex<ConnectionState>,
    on_connection: Option<Callback>,
    shutdown: watch::Sender<bool>,
    app_id: IdentificationLink,
    store: Arc<dyn Store>,
    registry: Mutex<Registry>,
    /// Wakes the runtime after a change of the registry.
    changed: Notify,
}

/// The sources of the application; locked briefly, never across an await.
pub(crate) struct Registry {
    pub(crate) provider: Provider,
    /// Assets whose RevisionCounter comes from the VendorNameplate
    /// (REQ-RES-MAM-018).
    nameplate: HashSet<IdentificationLink>,
    /// The `Set Data` handlers by Source and Tag.
    set_handlers: HashMap<(IdentificationLink, String), Arc<SetHandler>>,
    /// The `Set Config` handlers by Source and context.
    config_handlers: HashMap<(IdentificationLink, String), Arc<ConfigSetHandler>>,
}

/// Cheap, cloneable handle (`Send + Sync`), usable from any thread or task.
#[derive(Clone)]
pub struct AppHandle {
    shared: Arc<Shared>,
}

impl AppHandle {
    fn new(
        on_connection: Option<Callback>,
        app_id: IdentificationLink,
        store: Arc<dyn Store>,
        provider: Provider,
    ) -> Self {
        Self {
            shared: Arc::new(Shared {
                state: Mutex::new(ConnectionState::Closed),
                on_connection,
                shutdown: watch::Sender::new(false),
                app_id,
                store,
                registry: Mutex::new(Registry {
                    provider,
                    nameplate: HashSet::new(),
                    set_handlers: HashMap::new(),
                    config_handlers: HashMap::new(),
                }),
                changed: Notify::new(),
            }),
        }
    }

    /// The Identification Link, the AppId of the topics (REQ-IL-001).
    pub fn identification_link(&self) -> &IdentificationLink {
        &self.shared.app_id
    }

    /// The current MAM of the application (REQ-RES-MAM-001 … 024).
    pub fn mam(&self) -> Mam {
        self.registry().provider.lifecycle().mam().clone()
    }

    /// The application itself as asset (`Source` = AppId).
    pub fn this(&self) -> Asset {
        Asset {
            handle: self.clone(),
            il: self.shared.app_id.clone(),
        }
    }

    /// Adds a represented entity, e.g. a device without an MQTT client of
    /// its own: the application publishes and answers on its behalf with
    /// its own level (REQ-CMP-006, REQ-CMP-007, REQ-CMP-045). Its MAM
    /// (REQ-RES-MAM-019) and its Health (NORMAL_0, score 100, until set) are
    /// published, from Level 2 also its Profile and the PublicationList
    /// (REQ-RES-PUBLIST-017). Its DataSetWriterIds are persisted and stay
    /// the same after a restart (REQ-RES-PUBLIST-006).
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] for an invalid MAM (REQ-RES-MAM-004/012), the
    /// application itself or an asset that is already registered,
    /// [`Error::Io`] if the store fails.
    pub fn add_asset(&self, def: AssetDef) -> Result<Asset> {
        let il = match def.il {
            Some(il) => il,
            None => def.product.oi4_identifier(&def.serial_number)?,
        };
        let store = self.shared.store.as_ref();
        let counter = match def.revision_counter {
            Some(value) => value,
            None => revision_counter(store, &asset_revision_counter_key(&il))?,
        };
        let mam = def.product.mam(None, &def.serial_number, &il, counter)?;
        let mut registry = self.registry();
        registry.provider.add_asset(mam)?;
        if let Some(json) = registry.provider.take_writer_ids()
            && let Err(e) = store.save(WRITER_IDS, &json)
        {
            let _ = registry.provider.remove_asset(&il.0);
            return Err(e.into());
        }
        if def.revision_counter.is_some() {
            registry.nameplate.insert(il.clone());
        }
        drop(registry);
        self.shared.changed.notify_one();
        Ok(Asset {
            handle: self.clone(),
            il,
        })
    }

    /// The application itself or a represented asset.
    pub fn asset(&self, il: &IdentificationLink) -> Option<Asset> {
        let known = il == &self.shared.app_id || self.registry().provider.mam(&il.0).is_some();
        known.then(|| Asset {
            handle: self.clone(),
            il: il.clone(),
        })
    }

    /// The represented assets, in the order of registration.
    pub fn assets(&self) -> Vec<Asset> {
        self.registry()
            .provider
            .assets()
            .map(|il| Asset {
                handle: self.clone(),
                il: IdentificationLink(il.clone()),
            })
            .collect()
    }

    /// The current connection state.
    pub fn state(&self) -> ConnectionState {
        *self
            .shared
            .state
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
    }

    /// Sends Close and stops [`App::run`] (REQ-LIFE-005). Idempotent; a
    /// shutdown before `run` makes it return at once without connecting.
    pub fn shutdown(&self) {
        self.shared.shutdown.send_replace(true);
    }

    /// Sets the state and calls the callback on a change.
    pub(crate) fn set_state(&self, state: ConnectionState) {
        let previous = std::mem::replace(
            &mut *self
                .shared
                .state
                .lock()
                .unwrap_or_else(PoisonError::into_inner),
            state,
        );
        if previous != state {
            log::info!("connection state: {state:?}");
            if let Some(f) = &self.shared.on_connection {
                f(state);
            }
        }
    }

    /// The shutdown flag.
    pub(crate) fn subscribe(&self) -> watch::Receiver<bool> {
        self.shared.shutdown.subscribe()
    }

    /// The sources, locked.
    pub(crate) fn registry(&self) -> MutexGuard<'_, Registry> {
        self.shared
            .registry
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
    }

    /// Resolves after a change of the registry (a stored permit if none
    /// waited).
    pub(crate) async fn changed(&self) {
        self.shared.changed.notified().await;
    }

    /// See [`Asset::set_health`]: an invalid value is logged and ignored.
    pub(crate) fn set_health(&self, il: &IdentificationLink, health: Health) {
        let changed = self.registry().provider.set_health(&il.0, health);
        match changed {
            Ok(true) => self.shared.changed.notify_one(),
            Ok(false) => {}
            Err(e) => log::warn!("Health of {il} ignored: {e}"),
        }
    }

    /// See [`Asset::bump_revision_counter`].
    pub(crate) fn bump_revision_counter(&self, il: &IdentificationLink) -> Result<()> {
        let mut registry = self.registry();
        self.bump_locked(&mut registry, il)?;
        drop(registry);
        self.shared.changed.notify_one();
        Ok(())
    }

    /// Increments and persists the RevisionCounter of `il` and sets its MAM
    /// (published by the runtime); a counter from the VendorNameplate is
    /// only logged.
    fn bump_locked(&self, registry: &mut Registry, il: &IdentificationLink) -> Result<()> {
        if registry.nameplate.contains(il) {
            log::warn!(
                "the RevisionCounter of {il} comes from its VendorNameplate and is not \
                 incremented (REQ-RES-MAM-018)"
            );
            return Ok(());
        }
        let this = il == &self.shared.app_id;
        let mut mam = if this {
            registry.provider.lifecycle().mam().clone()
        } else {
            registry
                .provider
                .mam(&il.0)
                .cloned()
                .ok_or_else(|| Error::Invalid(format!("no asset {il}")))?
        };
        mam.revision_counter = mam.revision_counter.checked_add(1).ok_or_else(|| {
            Error::Invalid(format!(
                "the RevisionCounter of {il} is at its maximum (REQ-RES-MAM-018)"
            ))
        })?;
        let key = if this {
            REVISION_COUNTER.to_owned()
        } else {
            asset_revision_counter_key(il)
        };
        self.shared
            .store
            .save(&key, mam.revision_counter.to_string().as_bytes())?;
        registry.provider.set_mam(&il.0, mam)?;
        Ok(())
    }

    /// See [`Asset::mam`].
    pub(crate) fn asset_mam(&self, il: &IdentificationLink) -> Option<Mam> {
        let registry = self.registry();
        if il == &self.shared.app_id {
            Some(registry.provider.lifecycle().mam().clone())
        } else {
            registry.provider.mam(&il.0).cloned()
        }
    }

    /// See [`Asset::add_data`]: the DataSetWriterId and the Metadata
    /// version are persisted before the DataSet is published.
    pub(crate) fn add_data(&self, il: &IdentificationLink, def: DataSetDef) -> Result<DataSet> {
        let spec = def.resolve()?;
        let tag = spec.tag.clone();
        let store = self.shared.store.as_ref();
        let mut registry = self.registry();
        registry
            .provider
            .add_data(&il.0, spec, runtime::unix_ms())?;
        let saved = Self::save(&mut registry, store);
        if let Err(e) = saved {
            let _ = registry.provider.remove_data(&il.0, &tag);
            return Err(e);
        }
        drop(registry);
        self.shared.changed.notify_one();
        Ok(DataSet {
            handle: self.clone(),
            source: il.clone(),
            tag,
        })
    }

    /// Persists changed DataSetWriterIds, Metadata versions, settings of
    /// `Set PublicationList` and Config values.
    fn save(registry: &mut Registry, store: &dyn Store) -> Result<()> {
        if let Some(json) = registry.provider.take_writer_ids() {
            store.save(WRITER_IDS, &json)?;
        }
        if let Some(json) = registry.provider.take_meta_data_versions() {
            store.save(META_DATA_VERSIONS, &json)?;
        }
        if let Some(json) = registry.provider.take_publication_overrides() {
            store.save(PUBLICATIONS, &json)?;
        }
        if let Some(json) = registry.provider.take_config_values() {
            store.save(CONFIG_VALUES, &json)?;
        }
        Ok(())
    }

    /// Persists what the runtime changed (the first sending of a Metadata
    /// version, a `Set PublicationList`, a `Set Config`); a failing store
    /// is logged.
    pub(crate) fn save_changes(&self, registry: &mut Registry) {
        if let Err(e) = Self::save(registry, self.shared.store.as_ref()) {
            log::warn!("store: {e}");
        }
    }

    /// See [`DataSet::update`].
    pub(crate) fn update_data(
        &self,
        il: &IdentificationLink,
        tag: &str,
        values: Vec<(String, Value)>,
    ) -> Result<()> {
        let changed =
            self.registry()
                .provider
                .update_data(&il.0, tag, values, runtime::unix_ms())?;
        if changed {
            self.shared.changed.notify_one();
        }
        Ok(())
    }

    /// See [`DataSet::values`].
    pub(crate) fn data_values(&self, il: &IdentificationLink, tag: &str) -> Option<Vec<Value>> {
        self.registry().provider.data_values(&il.0, tag)
    }

    /// See [`DataSet::on_set`].
    pub(crate) fn on_set_data(
        &self,
        il: &IdentificationLink,
        tag: &str,
        handler: Arc<SetHandler>,
    ) -> Result<()> {
        let mut registry = self.registry();
        registry.provider.accept_set_data(&il.0, tag, true)?;
        registry
            .set_handlers
            .insert((il.clone(), tag.to_owned()), handler);
        Ok(())
    }

    /// A `Set Data` the provider checked: the handler decides outside the
    /// lock, then the provider answers (ADR-0026 §14).
    pub(crate) fn complete_set_data(&self, request: &SetDataRequest) -> Vec<Publication> {
        let key = (
            IdentificationLink(request.source().clone()),
            request.tag().to_owned(),
        );
        let handler = self.registry().set_handlers.get(&key).cloned();
        let decision = handler.map_or(Err(crate::status::StatusCode::BadNotWritable), |h| {
            let set = SetData {
                values: request.values().to_vec(),
            };
            std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| h(&set))).unwrap_or_else(
                |_| {
                    log::error!("the Set Data handler of {:?} panicked", request.tag());
                    Err(crate::status::StatusCode::BadInternalError)
                },
            )
        });
        let mut registry = self.registry();
        let out = registry.provider.complete_set_data(
            request,
            decision.map_err(crate::status::StatusCode::to_core),
            runtime::unix_ms(),
        );
        self.save_changes(&mut registry);
        out
    }

    /// See [`Asset::add_config`]: the DataSetWriterId is persisted before
    /// the Config is published.
    pub(crate) fn add_config(
        &self,
        il: &IdentificationLink,
        def: ConfigDef,
    ) -> Result<ConfigHandle> {
        let spec = def.resolve()?;
        let context = spec.context.clone();
        let store = self.shared.store.as_ref();
        let mut registry = self.registry();
        registry
            .provider
            .add_config(&il.0, spec, runtime::unix_ms())?;
        if let Err(e) = Self::save(&mut registry, store) {
            let _ = registry.provider.remove_config(&il.0, &context);
            return Err(e);
        }
        drop(registry);
        self.shared.changed.notify_one();
        Ok(ConfigHandle {
            handle: self.clone(),
            source: il.clone(),
            context,
        })
    }

    /// See [`ConfigHandle::values`].
    pub(crate) fn config_values(
        &self,
        il: &IdentificationLink,
        context: &str,
    ) -> Option<ConfigValues> {
        let registry = self.registry();
        let config = registry.provider.config(&il.0, context)?;
        Some(ConfigValues::new(config.clone(), Vec::new()))
    }

    /// See [`ConfigHandle::on_set`].
    pub(crate) fn on_set_config(
        &self,
        il: &IdentificationLink,
        context: &str,
        handler: Arc<ConfigSetHandler>,
    ) -> Result<()> {
        let mut registry = self.registry();
        if !registry.provider.serves_requests() {
            return Err(Error::Invalid(
                "Set Config is served from Level 2 on (no Set subscribed)".to_owned(),
            ));
        }
        if registry.provider.config(&il.0, context).is_none() {
            return Err(Error::Invalid(format!("no Config {context:?} of {il}")));
        }
        registry
            .config_handlers
            .insert((il.clone(), context.to_owned()), handler);
        Ok(())
    }

    /// See [`ConfigHandle::set`].
    pub(crate) fn set_config(
        &self,
        il: &IdentificationLink,
        context: &str,
        changes: &[(String, String, String)],
    ) -> Result<()> {
        let store = self.shared.store.as_ref();
        let mut registry = self.registry();
        let modified = registry
            .provider
            .set_config(&il.0, context, changes, runtime::unix_ms())?;
        if modified {
            Self::save(&mut registry, store)?;
            self.bump_locked(&mut registry, il)?;
            drop(registry);
            self.shared.changed.notify_one();
        }
        Ok(())
    }

    /// A `Set Config` the provider checked: the handler decides outside the
    /// lock (without one the values are applied), then the provider
    /// answers; a change increments the RevisionCounter (REQ-RES-MAM-018,
    /// ADR-0026 §15).
    pub(crate) fn complete_set_config(&self, request: &SetConfigRequest) -> Vec<Publication> {
        let il = IdentificationLink(request.source().clone());
        let (handler, changed) = {
            let registry = self.registry();
            let current = registry.provider.config(&il.0, request.context());
            let changed = request
                .changes()
                .iter()
                .filter(|(g, n, v)| {
                    current
                        .and_then(|c| c.groups.get(g))
                        .and_then(|group| group.objects.get(n))
                        .is_none_or(|o| &o.value != v)
                })
                .map(|(g, n, _)| (g.clone(), n.clone()))
                .collect();
            let key = (il.clone(), request.context().to_owned());
            (registry.config_handlers.get(&key).cloned(), changed)
        };
        let decision = handler.map_or(Ok(()), |h| {
            let proposed = ConfigValues::new(request.proposed().clone(), changed);
            std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| h(&proposed))).unwrap_or_else(
                |_| {
                    log::error!("the Set Config handler of {:?} panicked", request.context());
                    Err(crate::status::StatusCode::BadInternalError)
                },
            )
        });
        let mut registry = self.registry();
        let (out, modified) = registry.provider.complete_set_config(
            request,
            decision.map_err(crate::status::StatusCode::to_core),
            runtime::unix_ms(),
        );
        self.save_changes(&mut registry);
        if modified && let Err(e) = self.bump_locked(&mut registry, &il) {
            log::warn!("RevisionCounter of {il}: {e}");
        }
        out
    }

    /// See [`ConfigHandle::remove`].
    pub(crate) fn remove_config(&self, il: &IdentificationLink, context: &str) -> Result<()> {
        let mut registry = self.registry();
        registry.provider.remove_config(&il.0, context)?;
        registry
            .config_handlers
            .remove(&(il.clone(), context.to_owned()));
        drop(registry);
        self.shared.changed.notify_one();
        Ok(())
    }

    /// See [`DataSet::remove`].
    pub(crate) fn remove_data(&self, il: &IdentificationLink, tag: &str) -> Result<()> {
        let mut registry = self.registry();
        registry.provider.remove_data(&il.0, tag)?;
        registry.set_handlers.remove(&(il.clone(), tag.to_owned()));
        drop(registry);
        self.shared.changed.notify_one();
        Ok(())
    }

    /// See [`Asset::remove`].
    pub(crate) fn remove_asset(&self, il: &IdentificationLink) -> Result<()> {
        let mut registry = self.registry();
        registry.provider.remove_asset(&il.0)?;
        registry.nameplate.remove(il);
        registry.set_handlers.retain(|(source, _), _| source != il);
        registry
            .config_handlers
            .retain(|(source, _), _| source != il);
        drop(registry);
        self.shared.changed.notify_one();
        Ok(())
    }
}

impl fmt::Debug for AppHandle {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("AppHandle")
            .field("identification_link", &self.shared.app_id)
            .field("state", &self.state())
            .field("shutdown", &*self.shared.shutdown.borrow())
            .finish_non_exhaustive()
    }
}

impl fmt::Debug for App {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("App")
            .field("level", &self.level)
            .field("service_type", &self.service_type)
            .field("identification_link", self.identification_link())
            .field("connection", &self.connection)
            .field(
                "persistent_store",
                &self.handle.shared.store.is_persistent(),
            )
            .field("reference_designation", &self.reference_designation)
            .field("syslog", &self.syslog)
            .field("handle", &self.handle)
            .finish()
    }
}

#[cfg(test)]
mod tests {
    use super::{derived_client_id, keep_alive};
    use crate::identity::IdentificationLink;
    use std::time::Duration;

    #[test]
    fn derived_client_id_is_stable_and_short() {
        let il = IdentificationLink::parse("example.com/m/p/s").unwrap();
        let id = derived_client_id(&il);
        assert_eq!(id, derived_client_id(&il));
        assert_eq!(id.len(), 19);
        assert!(
            id.bytes()
                .all(|c| c.is_ascii_digit() || c.is_ascii_lowercase())
        );
        let other = IdentificationLink::parse("example.com/m/p/t").unwrap();
        assert_ne!(id, derived_client_id(&other));
    }

    #[test]
    fn keep_alive_bounds() {
        assert!(keep_alive(Duration::from_secs(1)).is_ok());
        assert!(keep_alive(Duration::from_secs(65_535)).is_ok());
        assert!(keep_alive(Duration::ZERO).is_err());
        assert!(keep_alive(Duration::from_secs(65_536)).is_err());
        assert!(keep_alive(Duration::from_millis(1500)).is_err());
    }
}
