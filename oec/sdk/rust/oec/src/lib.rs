//! # `oec` – the OI4 OEC v2 SDK for Rust (D2)
//!
//! The public API of ADR-0020. The developer picks the Messaging compliance
//! level ([`Level`]) and describes *what* the application offers; the SDK
//! does what the guideline prescribes for that level. The sans-IO protocol
//! logic lives in the internal crate `oec-core` (ADR-0026 §2); this crate
//! re-exports or wraps its types and adds the I/O: bootstrap from the
//! container storage, persistence and – from P4 increment 5 – the MQTT
//! connection.
//!
//! **State of P4 (C-048, increment 4):** [`app::App::builder`] with
//! [`app::Bootstrap`] (container storage contract or explicit broker),
//! identity ([`identity`]), persistence ([`store`]), the Config model with
//! string-keyed typed getters ([`config`], Q-022) and the errors ([`Error`]).
//! [`app::AppBuilder::build`] resolves and validates everything that does not
//! need a connection. `App::run`, the handles, assets, DataSets, events and
//! the consumer follow with increments 5 … 8; until then their contract is
//! the P3 sketch `oec/sdk/rust/sketch` (ADR-0020 §1).
//!
//! ```no_run
//! use oec::prelude::*;
//!
//! fn main() -> oec::Result<()> {
//!     let product = Product::new("example.com", "Hello OEC", "HELLO-1")
//!         .manufacturer("Example Inc.")
//!         .software_revision(env!("CARGO_PKG_VERSION"))
//!         .device_manual("https://example.com/hello/manual.pdf");
//!     let app = App::builder(Level::L1, ServiceType::Utility, product)
//!         .bootstrap(Bootstrap::container())
//!         .build()?;
//!     println!("{} connects to {}", app.identification_link(), app.connection().host());
//!     Ok(())
//! }
//! ```
//!
//! The REQ ids in the doc comments are the traceability links of ADR-0020.

pub mod app;
pub mod config;
pub mod event;
pub mod identity;
pub mod status;
pub mod store;

mod address;
mod container;

/// `ServiceType` topic element (REQ-TOP-005); one per application. The MAM
/// `DeviceClass` of the application is derived as `Oi4.<ServiceType>`
/// (REQ-RES-MAM-014).
pub use oec_core::topic::ServiceType;

/// Messaging compliance level of the application (MG 1.3).
///
/// The level is a promise to the Message Bus: the SDK enables every behavior
/// that is mandatory **or recommended** at this level (REQ-CMP-001 …
/// REQ-CMP-047) and rejects at [`app::AppBuilder::build`] a configuration that
/// cannot keep it. Offering more than the level requires (e.g. a writable
/// DataSet at Level 1) is allowed. The Service Container level is a property
/// of the image, not of the SDK (D8, REQ-CNT-*).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Level {
    /// MQTT + TLS + auth, Birth/Will, Pub MAM + Health (REQ-CMP-001 … 007).
    L1,
    /// L1 + `Get`, `Set`/`Del` where offered, Status events (REQ-CMP-008 … 023).
    L2,
    /// All resources, `Set`/`Del` wherever defined, `Call`/`Reply` +
    /// `NewDataSetWriterId`, full MQTT client behavior (REQ-CMP-033 … 040).
    L3,
}

/// Errors of the SDK API (not protocol status codes, see
/// [`status::StatusCode`]; ADR-0020 §14).
///
/// The structured errors of `oec-core` are wrapped into the message of
/// [`Error::Invalid`] together with their stable codes (ADR-0026 §2), e.g.
/// `invalid Identification Link: … [forbidden-character]`.
#[derive(Debug, thiserror::Error)]
#[non_exhaustive]
pub enum Error {
    /// An Identification Link, Oi4Identifier element, name or value violates
    /// the guideline (e.g. REQ-IL-008/016/019, REQ-RES-CONFIG-004).
    #[error("{0}")]
    Invalid(String),
    /// The configuration cannot fulfill the chosen [`Level`].
    #[error("level not met: {0}")]
    LevelNotMet(String),
    /// Container storage (`/etc/oi4`, `/run/secrets/oi4`) missing or invalid.
    #[error("bootstrap: {0}")]
    Bootstrap(String),
    /// Connection, TLS or I/O failure (including the [`store::Store`]).
    #[error("I/O: {0}")]
    Io(#[from] std::io::Error),
    /// No answer within the timeout (consumer side, REQ-ACT-009).
    #[error("timeout")]
    Timeout,
    /// The application was shut down.
    #[error("closed")]
    Closed,
}

impl From<oec_core::il::IlError> for Error {
    fn from(e: oec_core::il::IlError) -> Self {
        let codes: Vec<&str> = e
            .violations()
            .iter()
            .map(oec_core::il::Violation::code)
            .collect();
        Self::Invalid(format!("{e} [{}]", codes.join(", ")))
    }
}

impl From<oec_core::topic::TopicError> for Error {
    fn from(e: oec_core::topic::TopicError) -> Self {
        Self::Invalid(format!("{e} [{}]", e.code()))
    }
}

/// Result type of the SDK API.
pub type Result<T, E = Error> = std::result::Result<T, E>;

/// Everything a typical application needs (the items of increment 4; the
/// sketch prelude grows with increments 5 … 8).
pub mod prelude {
    pub use crate::app::{App, Bootstrap, Broker};
    pub use crate::config::{ConfigDef, ConfigObject, ConfigValues};
    pub use crate::event::SyslogLevel;
    pub use crate::identity::{IdentificationLink, Oi4Identifier, Product};
    pub use crate::status::StatusCode;
    pub use crate::{Level, ServiceType};
}
