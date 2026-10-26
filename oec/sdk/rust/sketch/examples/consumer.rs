// Consumer side inside an existing tokio application (here without tokio:
// the sketch has no dependencies, so `get_blocking` is used in `main` and the
// async form is shown in `fetch_all_mam`).
use std::time::Duration;

use oec::consumer::{GetOptions, Presence};
use oec::prelude::*;

async fn fetch_all_mam(consumer: &Consumer) -> oec::Result<usize> {
    let options = GetOptions { timeout: Duration::from_secs(5), per_page: Some(50), locale: None };
    let answers = consumer.get(Resource::Mam, Target::any(), options).await?;
    Ok(answers.len())
}

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "Logger", "LOGGER-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/logger/manual.pdf");
    let app = App::builder(Level::L1, ServiceType::Utility, product).build()?;
    let consumer = app.consumer();

    // Every Data publication on the bus; the subscription becomes an entry of
    // the own SubscriptionList and ends when dropped.
    let _data = consumer.subscribe(Resource::Data, Target::any(), |msg| {
        println!("{:?} {:?}: {}", msg.source, msg.filter, msg.payload_json);
    })?;
    let _presence = consumer.on_presence(|p| match p {
        Presence::Online { app, .. } => println!("online  {}", app.as_str()),
        Presence::OfflineGraceful { app } | Presence::OfflineUngraceful { app } => {
            println!("offline {}", app.as_str())
        }
    })?;

    let handle = app.handle();
    std::thread::spawn(move || {
        let options = GetOptions { timeout: Duration::from_secs(5), per_page: None, locale: None };
        if let Ok(all) = handle.consumer().get_blocking(Resource::Health, Target::any(), options) {
            println!("{} Health objects", all.len());
        }
    });
    let _ = fetch_all_mam;
    app.run()
}
