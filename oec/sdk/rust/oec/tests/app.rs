//! `AppBuilder::build` (ADR-0020 §2 … §5): level checks, identity, the
//! container storage contract and the explicit bootstrap.

mod common;

use std::time::Duration;

use oec::app::{Auth, ConnectionSettings};
use oec::identity::LocalizedText;
use oec::prelude::*;
use oec::store::{FileStore, MemoryStore, Store};

use common::{Root, product};

fn container(level: Level, root: &Root) -> oec::Result<App> {
    App::builder(level, ServiceType::Utility, product())
        .bootstrap(root.bootstrap())
        .build()
}

fn explicit() -> Broker {
    Broker::new("mqtt.example.com")
        .credentials("user", "secret")
        .ca_pem(common::ca_file())
}

fn device(level: Level, broker: Broker) -> oec::app::AppBuilder {
    App::builder(level, ServiceType::Utility, product())
        .bootstrap(Bootstrap::Explicit(broker))
        .serial_number("SN-1")
}

fn credentials(c: &ConnectionSettings) -> (&str, &[u8]) {
    match c.auth() {
        Auth::Credentials { username, password } => (username.as_str(), password.expose()),
        other => panic!("expected credentials, got {other:?}"),
    }
}

fn bootstrap_error(r: oec::Result<App>, needle: &str) {
    match r {
        Err(oec::Error::Bootstrap(m)) => assert!(m.contains(needle), "{m}"),
        other => panic!("expected Error::Bootstrap with {needle}, got {other:?}"),
    }
}

#[test]
fn container_defaults() {
    let root = Root::container("defaults");
    let app = container(Level::L2, &root).unwrap();
    // container name = SerialNumber = ClientId (REQ-CNT-021, REQ-MQTT-013)
    assert_eq!(
        app.identification_link().as_str(),
        "example.com/Hello,20OEC/HELLO-1/app-1"
    );
    let c = app.connection();
    assert_eq!(c.client_id(), "app-1");
    assert_eq!(
        (c.host(), c.port(), c.max_packet_size()),
        ("mqtt-broker", 8883, 262_144)
    );
    assert_eq!(c.ca(), root.path("etc/oi4/certs/ca.pem"));
    assert_eq!(credentials(c), ("user", &b"pa:ss"[..]));
    assert_eq!(c.keep_alive(), Duration::from_secs(60));
    // persistent store on the application specific volume (REQ-CNT-010)
    assert!(app.reference_designation());
    assert_eq!(app.level(), Level::L2);
    assert_eq!(app.service_type(), ServiceType::Utility);
    assert_eq!(app.syslog(), SyslogLevel::Notice);
    let mam = app.mam();
    assert_eq!(mam.serial_number, "app-1");
    assert_eq!(mam.product_instance_uri, app.identification_link().as_str());
    assert_eq!(mam.device_class, "Oi4.Utility");
    assert_eq!(mam.software_revision, "1.0.0");
    assert_eq!(mam.revision_counter, 0);
    // secrets never in Debug (G-4.5)
    let debug = format!("{app:?}");
    assert!(
        !debug.contains("pa:ss") && debug.contains("Secret(***)"),
        "{debug}"
    );
    assert!(debug.contains("persistent_store: true"), "{debug}");
}

#[test]
fn broker_ca_wins_over_ca() {
    let root = Root::container("broker-ca");
    root.write("etc/oi4/mqtt/broker_ca.pem", "BROKER CA");
    let app = container(Level::L1, &root).unwrap();
    let ca = root.path("etc/oi4/mqtt/broker_ca.pem");
    assert_eq!(app.connection().ca(), ca);
}

#[test]
fn no_ca() {
    let root = Root::container("no-ca");
    root.remove("etc/oi4/certs/ca.pem");
    bootstrap_error(container(Level::L1, &root), "REQ-CNT-004");
}

#[test]
fn missing_broker_json() {
    let root = Root::container("no-broker-json");
    root.remove("etc/oi4/mqtt/broker.json");
    bootstrap_error(container(Level::L1, &root), "REQ-CNT-004");
}

#[test]
fn container_name_from_the_hostname() {
    let root = Root::container("proc-hostname");
    root.write("proc/sys/kernel/hostname", "weather-7\n");
    let app = container(Level::L1, &root).unwrap();
    assert_eq!(app.connection().client_id(), "weather-7");
    assert_eq!(app.mam().serial_number, "weather-7");

    for bad in ["app_1", "-app", &"a".repeat(64), ""] {
        let root = Root::container("bad-hostname");
        root.write("etc/hostname", bad);
        bootstrap_error(container(Level::L1, &root), "REQ-CNT-019");
    }
    let root = Root::container("no-hostname");
    root.remove("etc/hostname");
    bootstrap_error(container(Level::L1, &root), "REQ-CNT-021");
}

#[test]
fn client_certificate_wins() {
    let root = Root::container("client-cert");
    root.write("run/secrets/oi4/private_key.pem", "KEY");
    root.write("run/secrets/oi4/passphrase", "c2VjcmV0");
    root.write("etc/oi4/certs/app-1.pem", "CERT");
    let app = container(Level::L1, &root).unwrap();
    match app.connection().auth() {
        Auth::ClientCertificate {
            cert,
            key,
            passphrase,
        } => {
            assert_eq!(cert, &root.path("etc/oi4/certs/app-1.pem"));
            assert_eq!(key, &root.path("run/secrets/oi4/private_key.pem"));
            assert_eq!(passphrase.as_ref().unwrap().expose(), b"secret");
        }
        other => panic!("{other:?}"),
    }
    // the certificate alone is enough (REQ-CNT-008)
    root.remove("run/secrets/oi4/credentials");
    assert!(container(Level::L1, &root).is_ok());
}

#[test]
fn key_without_certificate() {
    let root = Root::container("key-only");
    root.write("run/secrets/oi4/private_key.pem", "KEY");
    let app = container(Level::L1, &root).unwrap();
    assert_eq!(credentials(app.connection()).0, "user");
    root.remove("run/secrets/oi4/credentials");
    bootstrap_error(container(Level::L1, &root), "REQ-CNT-008");
}

#[test]
fn invalid_secrets() {
    let root = Root::container("no-secrets");
    root.remove("run/secrets/oi4/credentials");
    bootstrap_error(container(Level::L1, &root), "REQ-CNT-008");
    for (bad, needle) in [("dXNlcg==", "no \":\""), ("user:pass", "not base64")] {
        let root = Root::container("bad-credentials");
        root.write("run/secrets/oi4/credentials", bad);
        bootstrap_error(container(Level::L1, &root), needle);
    }
    let root = Root::container("bad-passphrase");
    root.write("run/secrets/oi4/passphrase", "%%%");
    bootstrap_error(container(Level::L1, &root), "passphrase");
}

#[test]
fn no_application_volume() {
    let root = Root::container("no-volume");
    root.remove("etc/oi4/app");
    // the ReferenceDesignation of Level 2 needs persistence (REQ-RES-REFDES-002)
    assert!(matches!(
        container(Level::L2, &root),
        Err(oec::Error::LevelNotMet(_))
    ));
    let app = container(Level::L1, &root).unwrap();
    assert!(!app.reference_designation());
    let app = App::builder(Level::L2, ServiceType::Utility, product())
        .bootstrap(root.bootstrap())
        .reference_designation(false)
        .build()
        .unwrap();
    assert!(!app.reference_designation());
}

#[test]
fn container_identity_rules() {
    let root = Root::container("identity");
    let builder =
        || App::builder(Level::L1, ServiceType::Utility, product()).bootstrap(root.bootstrap());
    assert!(matches!(
        builder().serial_number("x").build(),
        Err(oec::Error::Invalid(m)) if m.contains("REQ-CNT-021")
    ));
    assert!(matches!(
        builder().client_id("x").build(),
        Err(oec::Error::Invalid(m)) if m.contains("REQ-MQTT-013")
    ));
    let il = IdentificationLink::parse("https://id.example.com/4711").unwrap();
    let app = builder()
        .identification_link(il.clone())
        .serial_number("4711")
        .build()
        .unwrap();
    assert_eq!(app.identification_link(), &il);
    assert_eq!(app.mam().serial_number, "4711");
    assert_eq!(
        app.mam().product_instance_uri,
        "https://id.example.com/4711"
    );
    assert_eq!(app.connection().client_id(), "app-1");
}

#[test]
fn revision_counter_from_the_store() {
    let root = Root::container("revision-counter");
    root.write("etc/oi4/app/oec/app/revision-counter", "7\n");
    assert_eq!(
        container(Level::L1, &root).unwrap().mam().revision_counter,
        7
    );
    root.write("etc/oi4/app/oec/app/revision-counter", "seven");
    assert!(matches!(
        container(Level::L1, &root),
        Err(oec::Error::Io(e)) if e.kind() == std::io::ErrorKind::InvalidData
    ));
}

fn pump_def() -> AssetDef {
    AssetDef::with_oi4_identifier(
        Product::new("vendor.com", "Pump", "P-1").manufacturer("Vendor"),
        "0815",
    )
    .unwrap()
}

/// Represented assets (REQ-CMP-045): registration, lookup, MAM defaults of
/// an asset (REQ-RES-MAM-008), removal and the refused registrations.
#[test]
fn assets() {
    let app = device(Level::L2, explicit())
        .reference_designation(false)
        .build()
        .unwrap();
    let this = app.this();
    assert!(this.is_this());
    assert_eq!(this.identification_link(), app.identification_link());
    assert!(app.assets().is_empty());

    let pump = app.add_asset(pump_def()).unwrap();
    assert_eq!(
        pump.identification_link().as_str(),
        "vendor.com/Pump/P-1/0815"
    );
    assert!(!pump.is_this());
    let mam = pump.mam().unwrap();
    assert_eq!(
        (
            mam.hardware_revision.as_str(),
            mam.device_class.as_str(),
            mam.serial_number.as_str(),
            mam.product_instance_uri.as_str(),
            mam.revision_counter
        ),
        ("undefined", "", "0815", "vendor.com/Pump/P-1/0815", 0)
    );
    assert_eq!(app.assets().len(), 1);
    assert!(app.asset(pump.identification_link()).is_some());
    assert!(app.asset(app.identification_link()).unwrap().is_this());

    // twice, or the application itself
    assert!(matches!(
        app.add_asset(pump_def()),
        Err(oec::Error::Invalid(_))
    ));
    let me = AssetDef::new(app.identification_link().clone(), product(), "SN-1");
    assert!(matches!(app.add_asset(me), Err(oec::Error::Invalid(_))));
    // invalid MAM: DeviceManual without host (REQ-RES-MAM-012)
    let bad = AssetDef::with_oi4_identifier(
        Product::new("vendor.com", "Valve", "V-1").device_manual("manual.pdf"),
        "1",
    )
    .unwrap();
    assert!(matches!(app.add_asset(bad), Err(oec::Error::Invalid(_))));

    // invalid Health values are ignored, not panics
    this.set_health(HealthState::Normal, Some(0));
    pump.set_health(HealthState::Normal, Some(101));
    pump.set_health_details(vec![
        HealthDetail::new(HealthState::OffSpec),
        HealthDetail::new(HealthState::Failure),
    ]);

    assert!(matches!(this.clone().remove(), Err(oec::Error::Invalid(_))));
    let il = pump.identification_link().clone();
    pump.clone().remove().unwrap();
    assert!(pump.mam().is_none());
    assert!(app.asset(&il).is_none());
    assert!(matches!(pump.remove(), Err(oec::Error::Invalid(_))));
    assert!(app.assets().is_empty());
}

/// RevisionCounter of the application and of an asset: incremented,
/// persisted and read again (REQ-RES-MAM-018); a counter from the
/// VendorNameplate stays unchanged. The DataSetWriterIds are persisted
/// (REQ-RES-PUBLIST-006).
#[test]
fn revision_counters_and_writer_ids_are_persisted() {
    let root = Root::new("asset-store");
    let store = || FileStore::new(root.path("store"));
    let app = device(Level::L2, explicit())
        .store(store())
        .build()
        .unwrap();
    let writer_ids = store().load("app/writer-ids").unwrap().unwrap();
    let pump = app.add_asset(pump_def()).unwrap();
    pump.bump_revision_counter().unwrap();
    pump.bump_revision_counter().unwrap();
    app.this().bump_revision_counter().unwrap();
    assert_eq!(pump.mam().unwrap().revision_counter, 2);
    assert_eq!(app.mam().revision_counter, 1);
    let with_pump = store().load("app/writer-ids").unwrap().unwrap();
    assert_ne!(writer_ids, with_pump);

    let nameplate = app
        .add_asset(
            AssetDef::with_oi4_identifier(Product::new("vendor.com", "Valve", "V-1"), "1")
                .unwrap()
                .revision_counter(-1),
        )
        .unwrap();
    nameplate.bump_revision_counter().unwrap();
    assert_eq!(nameplate.mam().unwrap().revision_counter, -1);
    drop(app);

    let app = device(Level::L2, explicit())
        .store(store())
        .build()
        .unwrap();
    assert_eq!(app.mam().revision_counter, 1);
    let pump = app.add_asset(pump_def()).unwrap();
    assert_eq!(pump.mam().unwrap().revision_counter, 2);
    // the same ids again: the table did not change
    let table = store().load("app/writer-ids").unwrap().unwrap();
    let valve = &b"vendor.com/Valve/V-1/1"[..];
    assert!(table.windows(valve.len()).any(|w| w == valve));
    assert!(
        String::from_utf8(table)
            .unwrap()
            .contains("vendor.com/Pump/P-1/0815")
    );
    // a corrupt table is an error, not a silent renumbering
    root.write("store/app/writer-ids", "{}");
    assert!(matches!(
        device(Level::L2, explicit()).store(store()).build(),
        Err(oec::Error::Io(e)) if e.kind() == std::io::ErrorKind::InvalidData
    ));
}

#[test]
fn explicit_broker() {
    let app = device(Level::L1, explicit()).build().unwrap();
    let c = app.connection();
    assert_eq!(
        (c.host(), c.port(), c.ca()),
        ("mqtt.example.com", 8883, common::ca_file().as_path())
    );
    assert_eq!(credentials(c), ("user", &b"secret"[..]));
    // derived ClientId, stable (REQ-MQTT-013)
    assert!(c.client_id().starts_with("oec") && c.client_id().len() == 19);
    let again = device(Level::L1, explicit()).build().unwrap();
    assert_eq!(again.connection().client_id(), c.client_id());
    assert_eq!(app.mam().serial_number, "SN-1");
    assert!(!app.reference_designation());

    let app = device(Level::L1, explicit().port(1883).max_packet_size(1 << 20))
        .client_id("device-1")
        .keep_alive(Duration::from_secs(30))
        .max_packet_size(100_000)
        .syslog(SyslogLevel::Debug)
        .build()
        .unwrap();
    let c = app.connection();
    assert_eq!(
        (c.port(), c.max_packet_size(), c.publish_limit()),
        (1883, 1 << 20, 100_000)
    );
    assert_eq!(c.client_id(), "device-1");
    assert_eq!(c.keep_alive(), Duration::from_secs(30));
    assert_eq!(app.syslog(), SyslogLevel::Debug);
    // the own limit never exceeds the Message Bus (REQ-PAG-005)
    let app = device(Level::L1, explicit())
        .max_packet_size(1 << 30)
        .build()
        .unwrap();
    assert_eq!(app.connection().publish_limit(), 262_144);
}

#[test]
fn explicit_client_certificate_wins() {
    let root = Root::new("explicit-cert");
    root.write("cert.pem", "CERT")
        .write("key.pem", "KEY")
        .write("ca.pem", "CA");
    let broker = explicit()
        .ca_pem(root.path("ca.pem"))
        .client_cert(root.path("cert.pem"), root.path("key.pem"));
    let app = device(Level::L1, broker).build().unwrap();
    assert!(matches!(
        app.connection().auth(),
        Auth::ClientCertificate {
            passphrase: None,
            ..
        }
    ));
    let missing = explicit().ca_pem(root.path("missing.pem"));
    assert!(matches!(
        device(Level::L1, missing).build(),
        Err(oec::Error::Io(e)) if e.kind() == std::io::ErrorKind::NotFound
    ));
}

#[test]
fn explicit_rejections() {
    let invalid = |b: oec::app::AppBuilder| matches!(b.build(), Err(oec::Error::Invalid(_)));
    // no authentication: no level is met (REQ-CMP-002, REQ-MQTT-009)
    assert!(matches!(
        device(Level::L1, Broker::new("broker")).build(),
        Err(oec::Error::LevelNotMet(_))
    ));
    // no CA: the broker certificate cannot be verified (REQ-MQTT-008); no
    // fallback to the trust store of the platform (ADR-0020 amendment C-050)
    assert!(matches!(
        device(Level::L1, Broker::new("broker").credentials("u", "p")).build(),
        Err(oec::Error::LevelNotMet(m)) if m.contains("REQ-MQTT-008")
    ));
    assert!(invalid(device(
        Level::L1,
        Broker::new("broker:8883")
            .credentials("u", "p")
            .ca_pem(common::ca_file())
    )));
    assert!(invalid(device(Level::L1, explicit().port(0))));
    assert!(invalid(device(Level::L1, explicit().max_packet_size(1000))));
    assert!(invalid(
        device(Level::L1, explicit()).keep_alive(Duration::ZERO)
    ));
    assert!(invalid(device(Level::L1, explicit()).max_packet_size(0)));
    assert!(invalid(device(Level::L1, explicit()).client_id("")));
    // serial number required outside a container (REQ-RES-MAM-015)
    assert!(invalid(
        App::builder(Level::L1, ServiceType::Utility, product())
            .bootstrap(Bootstrap::Explicit(explicit()))
    ));
}

#[test]
fn levels() {
    let not_met = |b: oec::app::AppBuilder| matches!(b.build(), Err(oec::Error::LevelNotMet(_)));
    // MemoryStore is for Level 1 (ADR-0020 §3)
    assert!(not_met(device(Level::L2, explicit())));
    assert!(not_met(device(Level::L3, explicit())));
    assert!(not_met(
        device(Level::L3, explicit()).store(MemoryStore::new())
    ));
    let root = Root::new("levels");
    let file_store = || FileStore::new(root.path("store"));
    assert!(file_store().is_persistent());
    // Level 3 always offers ReferenceDesignation (REQ-CMP-020)
    assert!(not_met(
        device(Level::L3, explicit())
            .store(file_store())
            .reference_designation(false)
    ));
    let app = device(Level::L3, explicit())
        .store(file_store())
        .build()
        .unwrap();
    assert!(app.reference_designation());
    assert!(Level::L1 < Level::L2 && Level::L2 < Level::L3);
    // offering more than the level requires is allowed
    let app = device(Level::L1, explicit())
        .store(file_store())
        .reference_designation(true)
        .build()
        .unwrap();
    assert!(app.reference_designation());
}

#[test]
fn invalid_identity() {
    let build = |p: Product| {
        App::builder(Level::L1, ServiceType::Utility, p)
            .bootstrap(Bootstrap::Explicit(explicit()))
            .serial_number("SN-1")
            .build()
    };
    let message = |p: Product| match build(p) {
        Err(oec::Error::Invalid(m)) => m,
        other => panic!("{other:?}"),
    };
    assert!(message(Product::new("https://example.com", "M", "P")).contains("manufacturer-uri"));
    assert!(
        message(Product::new(
            "example.com",
            LocalizedText::new("de-DE", "M"),
            "P"
        ))
        .contains("REQ-RES-MAM-024")
    );
    assert!(
        message(product().device_manual("/usr/share/doc/manual.pdf")).contains("REQ-RES-MAM-012")
    );
    let long = "x".repeat(300);
    assert!(message(Product::new("example.com", long.as_str(), "P")).contains("too-long"));
    assert!(IdentificationLink::parse("not a link").is_err());
    let oi4 = Oi4Identifier::new("Example.com", "M", "P")
        .serial_number("S")
        .build()
        .unwrap();
    assert_eq!(oi4.as_str(), "example.com/M/P/S");
}

#[test]
fn app_is_send_and_sync() {
    fn assert_send_sync<T: Send + Sync + 'static>() {}
    assert_send_sync::<App>();
    assert_send_sync::<oec::app::AppBuilder>();
    assert_send_sync::<oec::Error>();
}

#[test]
fn error_display() {
    assert_eq!(oec::Error::Timeout.to_string(), "timeout");
    assert_eq!(oec::Error::Closed.to_string(), "closed");
    assert_eq!(
        oec::Error::LevelNotMet("x".into()).to_string(),
        "level not met: x"
    );
    assert_eq!(
        oec::Error::Bootstrap("x".into()).to_string(),
        "bootstrap: x"
    );
    let io: oec::Error = std::io::Error::other("disk").into();
    assert_eq!(io.to_string(), "I/O: disk");
}

/// The resolved MAM validates against the MAM schema (REQ-RES-MAM-001 …).
#[cfg(feature = "strict-validation")]
#[test]
fn mam_matches_schema() {
    use oec_core::schema::{Flavor, Validator};
    let validator = Validator::new().unwrap();
    let entry = validator
        .schema("resources/mam.schema.json#/$defs/MAM")
        .unwrap();
    let root = Root::container("mam-schema");
    for app in [
        container(Level::L1, &root).unwrap(),
        device(Level::L1, explicit()).build().unwrap(),
    ] {
        let doc = serde_json::to_value(app.mam()).unwrap();
        validator.validate(&entry, Flavor::Strict, &doc).unwrap();
        let asset = app.add_asset(pump_def()).unwrap();
        let doc = serde_json::to_value(asset.mam().unwrap()).unwrap();
        validator.validate(&entry, Flavor::Strict, &doc).unwrap();
    }
}

fn oee_def() -> DataSetDef {
    DataSetDef::new("oee")
        .description("Overall equipment effectiveness")
        .field(Field::double("Availability").unit("%").range(0.0, 100.0))
        .field(Field::new("Count", BuiltinType::UInt32))
        .publication(
            Publication::default_data()
                .mode(Mode::ON_REQUEST | Mode::SOURCE)
                .precision("Availability", 0.5),
        )
}

/// Data DataSets (P4 increment 6b): declaration, state, removal and the
/// refused definitions and values; the Metadata versions are persisted and
/// unchanged by a restart with the same structure (REQ-MSG-036).
#[test]
fn data_sets_are_declared_and_persisted() {
    let root = Root::new("data-store");
    let store = || FileStore::new(root.path("store"));
    let app = device(Level::L2, explicit())
        .store(store())
        .build()
        .unwrap();
    let pump = app.add_asset(pump_def()).unwrap();
    let oee = pump.add_data(oee_def()).unwrap();
    assert_eq!(
        (oee.tag(), oee.source()),
        ("oee", pump.identification_link())
    );
    let pv = app
        .this()
        .add_data(DataSetDef::oi4pv(Field::double("Temperature").unit("°C")))
        .unwrap();
    assert_eq!(pv.tag(), "Oi4Pv");
    let versions = store().load("app/metadata-versions").unwrap().unwrap();

    // a DataSet has values once every field has one; all or none is set
    assert_eq!(oee.values(), None);
    oee.update([("Availability", 97.5)]).unwrap();
    assert_eq!(oee.values(), None);
    assert!(matches!(
        oee.update([("Count", Value::from(1)), ("Unknown", Value::from(2))]),
        Err(oec::Error::Invalid(_))
    ));
    assert!(matches!(
        oee.update([("Count", -1)]),
        Err(oec::Error::Invalid(_))
    ));
    oee.update([("Count", 7)]).unwrap();
    assert_eq!(
        oee.values(),
        Some(vec![Value::Double(97.5), Value::UInt32(7)])
    );

    // removed: no values, no updates; the asset's removal drops its DataSets
    pv.clone().remove().unwrap();
    assert_eq!(pv.values(), None);
    assert!(matches!(
        pv.update([("Pv", 1.0)]),
        Err(oec::Error::Invalid(_))
    ));
    assert!(matches!(pv.remove(), Err(oec::Error::Invalid(_))));
    pump.clone().remove().unwrap();
    assert!(matches!(
        oee.update([("Count", 8)]),
        Err(oec::Error::Invalid(_))
    ));
    drop(app);

    // a restart with the same structure: the same versions
    let app = device(Level::L2, explicit())
        .store(store())
        .build()
        .unwrap();
    let pump = app.add_asset(pump_def()).unwrap();
    pump.add_data(oee_def()).unwrap();
    app.this()
        .add_data(DataSetDef::oi4pv(Field::double("Temperature").unit("°C")))
        .unwrap();
    assert_eq!(
        store().load("app/metadata-versions").unwrap().unwrap(),
        versions
    );
    // another structure: a new version
    app.this()
        .add_data(DataSetDef::new("oee").field(Field::double("Availability")))
        .unwrap();
    assert_ne!(
        store().load("app/metadata-versions").unwrap().unwrap(),
        versions
    );
    drop(app);
    // a corrupt table is an error
    root.write("store/app/metadata-versions", "[]");
    assert!(matches!(
        device(Level::L2, explicit()).store(store()).build(),
        Err(oec::Error::Io(e)) if e.kind() == std::io::ErrorKind::InvalidData
    ));
}

/// Refused Data DataSet definitions: a Tag twice, no field, an empty Tag,
/// no UUID as DataSetClassId, an Interval of no whole ms, a Precision of an
/// unknown field, an Oi4Pv field other than Sv1 … Sv255.
#[test]
fn invalid_data_sets_are_refused() {
    let app = device(Level::L2, explicit())
        .reference_designation(false)
        .build()
        .unwrap();
    let pump = app.add_asset(pump_def()).unwrap();
    pump.add_data(oee_def()).unwrap();
    let invalid =
        |r: oec::Result<DataSet>| assert!(matches!(r, Err(oec::Error::Invalid(_))), "{r:?}");
    invalid(pump.add_data(oee_def()));
    invalid(pump.add_data(DataSetDef::new("empty")));
    invalid(pump.add_data(DataSetDef::new("").field(Field::double("x"))));
    invalid(
        pump.add_data(
            DataSetDef::new("c")
                .field(Field::double("x"))
                .class_id("no-uuid"),
        ),
    );
    invalid(
        pump.add_data(
            DataSetDef::new("d")
                .field(Field::double("x"))
                .publication(Publication::default_data().interval(Duration::from_micros(1500))),
        ),
    );
    invalid(
        pump.add_data(
            DataSetDef::new("e")
                .field(Field::double("x"))
                .publication(Publication::default_data().precision("y", 1.0)),
        ),
    );
    invalid(
        app.this()
            .add_data(DataSetDef::oi4pv(Field::double("p")).field(Field::double("x"))),
    );
}

/// `DataSet::on_set` (C-055): refused at Level 1 (no `Set` subscribed) and
/// for a removed DataSet; a corrupt table of `Set PublicationList`
/// settings is an error.
#[test]
fn set_handlers_and_publication_settings() {
    let ok = |_: &oec::data::SetData| Ok(());
    let app = device(Level::L1, explicit())
        .reference_designation(false)
        .build()
        .unwrap();
    let oee = app
        .add_asset(pump_def())
        .unwrap()
        .add_data(oee_def())
        .unwrap();
    assert!(matches!(oee.on_set(ok), Err(oec::Error::Invalid(_))));

    let root = Root::new("publications-store");
    let store = || FileStore::new(root.path("store"));
    let app = device(Level::L2, explicit())
        .store(store())
        .build()
        .unwrap();
    let oee = app
        .add_asset(pump_def())
        .unwrap()
        .add_data(oee_def())
        .unwrap();
    oee.on_set(ok).unwrap();
    oee.clone().remove().unwrap();
    assert!(matches!(oee.on_set(ok), Err(oec::Error::Invalid(_))));
    drop(app);
    root.write("store/app/publications", "{\"Version\":2,\"Entries\":[]}");
    assert!(matches!(
        device(Level::L2, explicit()).store(store()).build(),
        Err(oec::Error::Io(e)) if e.kind() == std::io::ErrorKind::InvalidData
    ));
}
