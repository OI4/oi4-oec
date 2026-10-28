//! Consumer side inside an existing tokio application: subscriptions and
//! presence (callbacks), `Get` (async) and the Action NewDataSetWriterId of
//! another application, e.g. of `examples/demo.rs`:
//!
//! ```sh
//! OEC_ROOT=oec/dev/generated/roots/oec-dev-cert cargo run -p oec \
//!     --example consumer -- example.com/OEC,20Demo/DEMO-1/oec-dev-app
//! ```
//!
//! `OEC_ROOT` is the container storage of RIG 4 (default `/`);
//! `OEC_DEMO_SECONDS=n` shuts it down gracefully after n seconds.
use std::path::PathBuf;
use std::time::Duration;

use oec::app::ConnectionState;
use oec::consumer::{ActionOptions, GetOptions, Presence};
use oec::prelude::*;

/// The MAM of `app` and its assets, 50 per page.
async fn count_mam(consumer: &Consumer, app: &Target) -> oec::Result<usize> {
    let options = GetOptions::new()
        .timeout(Duration::from_secs(5))
        .per_page(50);
    let answers = consumer.get(Resource::Mam, app, &options).await?;
    Ok(answers.len())
}

/// A DataSetWriterId of `app` for an entry of its PublicationList.
async fn new_writer_id(consumer: &Consumer, app: &Target) -> oec::Result<u16> {
    let options = ActionOptions::new().timeout(Duration::from_secs(5));
    let id = consumer
        .new_data_set_writer_id(Resource::Data, app, &options)
        .await?;
    Ok(id.data_set_writer_id)
}

/// What the application does once it is online: ask the OTConnector
/// named on the command line.
async fn requests(consumer: &Consumer) -> oec::Result<()> {
    let Some(other) = std::env::args().nth(1) else {
        return Ok(());
    };
    let app = Target::app(ServiceType::OTConnector, IdentificationLink::parse(&other)?);
    println!("MAM objects: {:?}", count_mam(consumer, &app).await);
    println!(
        "new DataSetWriterId: {:?}",
        new_writer_id(consumer, &app).await
    );
    Ok(())
}

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "Logger", "LOGGER-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/logger/manual.pdf");
    let root = std::env::var_os("OEC_ROOT").map_or_else(|| PathBuf::from("/"), PathBuf::from);
    let (online, mut state) = tokio::sync::watch::channel(ConnectionState::Connecting);
    let app = App::builder(Level::L1, ServiceType::Utility, product)
        .bootstrap(Bootstrap::ContainerStorage { root })
        .on_connection(move |s| {
            let _ = online.send(s);
        })
        .build()?;
    let consumer = app.consumer();
    let handle = app.handle();

    // Every Data publication on the bus; the subscription becomes an entry
    // of the own SubscriptionList and ends when dropped.
    let _data = consumer.subscribe(Resource::Data, &Target::any(), |r| {
        println!("{:?}: {}", r.source(), r.payload_json());
    })?;
    let _presence = consumer.on_presence(|p| match p {
        Presence::Online { app, .. } => println!("online  {}", app.as_str()),
        Presence::OfflineGraceful { app, .. } | Presence::OfflineUngraceful { app, .. } => {
            println!("offline {}", app.as_str());
        }
        _ => {}
    })?;

    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()?;
    runtime.block_on(async move {
        let run = tokio::spawn(app.run_async());
        // requests are not queued while offline (ADR-0020 §7)
        if state
            .wait_for(|s| *s == ConnectionState::Online)
            .await
            .is_ok()
        {
            requests(&consumer).await?;
            if let Some(secs) = std::env::var("OEC_DEMO_SECONDS")
                .ok()
                .and_then(|s| s.parse::<u64>().ok())
            {
                tokio::time::sleep(Duration::from_secs(secs)).await;
                handle.shutdown();
            }
        }
        run.await.map_err(|e| oec::Error::Invalid(e.to_string()))?
    })
}
