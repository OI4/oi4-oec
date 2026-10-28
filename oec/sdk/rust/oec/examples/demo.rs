//! Demo application of the SDK (P4 increment 8b, C-059): one Level 2
//! OTConnector that is provider and consumer at once – the application
//! under test of D7 `oec-check test` (ROADMAP P4).
//!
//! Provider side: a represented pump with a writable `oee` Data DataSet
//! (`Set Data` checked by a handler, Mode and Interval changeable with
//! `Set PublicationList`), a Config `service` whose `Period` paces the
//! updates, the ReferenceDesignation; MAM, Health, Profile,
//! PublicationList, SubscriptionList and the answers to `Get`, `Set` and
//! `Del` come from the SDK.
//!
//! Consumer side: the presence of the applications on the bus and every
//! `oee` Data publication, at most once per second; the subscription is
//! configurable, so other applications change its Interval with `Set
//! SubscriptionList` or delete it with `Del SubscriptionList`.
//!
//! Run it against the broker of the dev environment (`oec/dev`; the
//! container storage of RIG 4 under `OEC_ROOT`, default `/`):
//!
//! ```sh
//! OEC_ROOT=oec/dev/generated/roots/oec-dev-app \
//!     cargo run -p oec --example demo
//! ```
//!
//! `OEC_DEMO_SECONDS=n` shuts it down gracefully (Close) after n seconds.

use std::path::PathBuf;
use std::thread;
use std::time::Duration;

use oec::consumer::Presence;
use oec::data::{Value, Writable};
use oec::prelude::*;

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "OEC Demo", "DEMO-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/demo/manual.pdf");
    let root = std::env::var_os("OEC_ROOT").map_or_else(|| PathBuf::from("/"), PathBuf::from);
    let app = App::builder(Level::L2, ServiceType::OTConnector, product)
        .bootstrap(Bootstrap::ContainerStorage { root })
        .build()?;
    app.this().set_health(HealthState::Normal, Some(100));

    // Config of the application: the update period of the pump in seconds.
    let config = app.this().add_config(
        ConfigDef::new("service").group(
            "common",
            "Common settings",
            [ConfigObject::number("Period", 5.0)
                .label("Update period")
                .unit("s")
                .min(1.0)
                .max(3600.0)
                .mandatory()],
        ),
    )?;

    // The represented pump (no MQTT client of its own → Oi4Identifier).
    let pump = app.add_asset(AssetDef::with_oi4_identifier(
        Product::new("vendor.example.com", "Pump", "P-100"),
        "0001",
    )?)?;
    pump.set_health(HealthState::Normal, Some(100));
    let oee = pump.add_data(
        DataSetDef::new("oee")
            .field(Field::double("Availability").unit("%"))
            .field(Field::new("Count", BuiltinType::UInt32))
            .publication(Publication::default_data().writable(Writable::ModeAndInterval)),
    )?;
    oee.update([("Availability", Value::from(97.5)), ("Count", 0u32.into())])?;
    // The SDK checks the types, answers with the Status event and publishes
    // the accepted values; the handler only decides.
    oee.on_set(|set| match set.get("Count") {
        Some(Value::UInt32(c)) if *c <= 1_000_000 => Ok(()),
        _ => Err(StatusCode::BadOutOfRange),
    })?;

    let consumer = app.consumer();
    let _presence = consumer.on_presence(|p| match p {
        Presence::Online { app, mam, .. } => {
            let model = mam.as_ref().map(|m| m.model.text.clone());
            println!("online  {} {model:?}", app.as_str());
        }
        Presence::OfflineGraceful { app, .. } | Presence::OfflineUngraceful { app, .. } => {
            println!("offline {}", app.as_str());
        }
        _ => {}
    })?;
    let _oee = consumer
        .subscribe(Resource::Data, &Target::any().filter("oee"), |r| {
            let source = r.source().map(|s| s.as_str().to_owned());
            println!("oee     {source:?} {}", r.payload_json());
        })?
        .interval(Duration::from_secs(1))
        .configurable();

    // The process: one count per period, until the DataSet is gone.
    thread::spawn(move || {
        let mut count = 0u32;
        loop {
            let period = config
                .values()
                .number("common", "Period")
                .unwrap_or(5.0)
                .clamp(1.0, 3600.0);
            thread::sleep(Duration::from_secs_f64(period));
            count = count.wrapping_add(1) % 1_000_000;
            if oee.update([("Count", count)]).is_err() {
                return;
            }
        }
    });

    if let Some(secs) = std::env::var("OEC_DEMO_SECONDS")
        .ok()
        .and_then(|s| s.parse::<u64>().ok())
    {
        let handle = app.handle();
        thread::spawn(move || {
            thread::sleep(Duration::from_secs(secs));
            handle.shutdown();
        });
    }
    app.run()
}
