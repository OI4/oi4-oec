//! `oec-config`: the config service D10 of the OEC demos (ADR-0028 §7,
//! A-304). It finds the OEC applications on the Message Bus, reads their
//! Config and SubscriptionList, and changes writable Configs with
//! `Set Config`; an HTTP API (A-357) serves the tests and the Web UI.
//!
//! Environment: `OEC_ROOT` (container storage, default `/`),
//! `OEC_CONFIG_FILE`, `OEC_UI_LISTEN` (see [`settings`]) and
//! `OEC_CONFIG_LOG` (`error`, `warn`, `info` (default), `debug`) for the
//! local log on stderr.

mod api;
mod directory;
mod http;
mod service;
mod settings;
mod topic;

use std::net::TcpListener;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::mpsc;
use std::sync::{Arc, OnceLock, Weak};

use log::{LevelFilter, Log, Metadata, Record};
use oec::app::ConnectionState;
use oec::prelude::*;

use crate::service::Service;
use crate::settings::Settings;

/// The local log on stderr (ADR-0028 §2); the SDK bridges the records to
/// Syslog events on the bus too.
struct Stderr(LevelFilter);

impl Log for Stderr {
    fn enabled(&self, m: &Metadata<'_>) -> bool {
        m.level() <= self.0
    }
    fn log(&self, r: &Record<'_>) {
        if self.enabled(r.metadata()) {
            eprintln!("{} {}: {}", r.level(), r.target(), r.args());
        }
    }
    fn flush(&self) {}
}

fn product() -> Product {
    Product::new("oi4.org", "OEC Config", "OEC-CONFIG")
        .manufacturer("Open Industry 4.0 Alliance")
        .description("Demo config service of OI4 Open Edge Computing")
        .software_revision(env!("CARGO_PKG_VERSION"))
}

fn run() -> Result<(), String> {
    let level = match std::env::var("OEC_CONFIG_LOG").as_deref() {
        Ok("error") => LevelFilter::Error,
        Ok("warn") => LevelFilter::Warn,
        Ok("debug") => LevelFilter::Debug,
        _ => LevelFilter::Info,
    };
    oec::event::install_bridge(Some(Box::new(Stderr(level)))).map_err(|e| e.to_string())?;
    let root = std::env::var_os("OEC_ROOT").map_or_else(|| PathBuf::from("/"), PathBuf::from);
    let settings = Settings::load(&root, |k| std::env::var(k).ok())?;

    let slot: Arc<OnceLock<Weak<Service>>> = Arc::new(OnceLock::new());
    let on_state = Arc::clone(&slot);
    let app = App::builder(Level::L1, ServiceType::Utility, product())
        .bootstrap(Bootstrap::ContainerStorage { root })
        .on_connection(move |state| {
            if let Some(s) = on_state.get().and_then(Weak::upgrade) {
                s.connection(state == ConnectionState::Online);
            }
        })
        .build()
        .map_err(|e| e.to_string())?;
    let me = (
        ServiceType::Utility.as_str().to_owned(),
        app.identification_link().as_str().to_owned(),
    );
    let listen = settings.listen.clone();
    let (jobs, queue) = mpsc::channel();
    let service = Service::new(app.consumer(), settings, me, jobs);
    let _ = slot.set(Arc::downgrade(&service));
    service.watch().map_err(|e| e.to_string())?;

    let worker = Arc::clone(&service);
    std::thread::Builder::new()
        .name("oec-config-discovery".to_owned())
        .spawn(move || worker.work(&queue))
        .map_err(|e| e.to_string())?;
    if let Some(addr) = listen {
        let listener = TcpListener::bind(&addr).map_err(|e| format!("listen {addr}: {e}"))?;
        log::info!(
            "HTTP API on http://{}",
            listener.local_addr().map_err(|e| e.to_string())?
        );
        let routed = Arc::clone(&service);
        let handler: http::Handler = Arc::new(move |r| api::route(&*routed, r));
        std::thread::Builder::new()
            .name("oec-config-http".to_owned())
            .spawn(move || http::serve(&listener, &handler))
            .map_err(|e| e.to_string())?;
    }
    app.run().map_err(|e| e.to_string())
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            eprintln!("oec-config: {e}");
            ExitCode::FAILURE
        }
    }
}
