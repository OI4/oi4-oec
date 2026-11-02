//! `oec-registry`: the OEC Registry D5 of P7 (ADR-0029, MG A1). Step 1
//! (ADR-0008): read-only. It collects the MAM and Health of the assets on
//! the Message Bus (REQ-REG-001) and offers them to others with the
//! Registry services `Get/MAM[/<Source>]` and `Get/Health[/<Source>]` on
//! its own AppId (REQ-REG-002).
//!
//! Environment: `OEC_ROOT` (container storage, default `/`) and
//! `OEC_REGISTRY_LOG` (`error`, `warn`, `info` (default), `debug`) for the
//! local log on stderr.

#[cfg(test)]
mod api_tests;
mod bus;
mod directory;

use std::path::PathBuf;
use std::process::ExitCode;

use log::{LevelFilter, Log, Metadata, Record};
use oec::prelude::*;

/// The local log on stderr; the SDK bridges the records to Syslog events
/// on the bus too.
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

/// The product of the registry (ADR-0029 §2).
fn product() -> Product {
    Product::new("oi4.org", "OEC Registry", "OEC-REGISTRY")
        .manufacturer("Open Industry 4.0 Alliance")
        .description("OEC Registry of OI4 Open Edge Computing")
        .software_revision(env!("CARGO_PKG_VERSION"))
}

/// The Messaging level of the current increment (ADR-0029 §2): Level 2,
/// the first level that serves `Get` (ADR-0020 §3).
const LEVEL: Level = Level::L2;

fn run() -> Result<(), String> {
    let level = match std::env::var("OEC_REGISTRY_LOG").as_deref() {
        Ok("error") => LevelFilter::Error,
        Ok("warn") => LevelFilter::Warn,
        Ok("debug") => LevelFilter::Debug,
        _ => LevelFilter::Info,
    };
    oec::event::install_bridge(Some(Box::new(Stderr(level)))).map_err(|e| e.to_string())?;
    let root = std::env::var_os("OEC_ROOT").map_or_else(|| PathBuf::from("/"), PathBuf::from);
    let app = App::builder(LEVEL, ServiceType::Registry, product())
        .bootstrap(Bootstrap::ContainerStorage { root })
        .build()
        .map_err(|e| e.to_string())?;
    log::info!(
        "OEC Registry {} at Level {LEVEL:?}",
        app.identification_link().as_str()
    );
    let _bus = bus::Bus::start(app.handle())?;
    app.run().map_err(|e| e.to_string())
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            eprintln!("oec-registry: {e}");
            ExitCode::FAILURE
        }
    }
}
