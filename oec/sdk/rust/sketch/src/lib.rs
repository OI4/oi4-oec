//! # `oec` – public API sketch of the OEC Rust SDK (D2)
//!
//! **P3 spike (C-042, ADR-0020) – not product code.** This crate fixes the
//! *shape* of the public API so that it can be reviewed and compile-checked
//! (`oec/check.sh`). Every body is `unimplemented!()`; the implementation
//! follows in P4 inside the Cargo workspace `oec/sdk/rust/` (`oec-core`
//! sans-IO, `oec` runtime, `oec-ffi` C ABI, ADR-0011).
//!
//! Design rules (ADR-0020):
//!
//! 1. **The level is chosen, the protocol is automatic.** The developer picks
//!    the Messaging compliance level ([`Level`]) and describes *what* the
//!    application offers (assets, DataSets, Config). The SDK does everything
//!    the guideline prescribes for that level by itself: Birth/Will/Close,
//!    Health heartbeat, Profile, PublicationList, answers to `Get` with
//!    pagination, Status events after `Set`/`Del`, MessageId/`ReplyTo`,
//!    DataSetWriterIds, topic and payload encoding.
//! 2. **State, not messages.** Applications *update state*
//!    ([`DataSet::update`], [`Asset::set_health`]); the SDK decides when a
//!    message is sent (PublicationList `Mode`/`Interval`/`Precisions`).
//! 3. **Blocking by default, async on demand.** [`App::run`] blocks and needs
//!    no async runtime; [`App::run_async`] integrates into an existing tokio
//!    application. Handles are `Clone + Send + Sync`, and state updates are
//!    synchronous and non-blocking, so they work from any thread or task.
//!    Only consumer requests that wait for an answer are `async` (with a
//!    `_blocking` twin).
//! 4. **Handlers decide, the SDK answers.** `Set` handlers return
//!    `Result<(), StatusCode>`; the SDK validates the request before, and
//!    publishes the Status event and the changed state after the handler.
//! 5. **Strict sender, tolerant receiver** (REQ-MSG-001, REQ-MSG-046): every
//!    outgoing message is built from typed values; unknown keys in received
//!    messages are ignored. Outgoing messages can be checked against the
//!    embedded D1 schemas (ADR-0019 §7, feature `strict-validation`).
//!
//! The REQ ids in the doc comments are the traceability links of ADR-0020.
#![allow(dead_code, unused_variables, clippy::new_without_default)]

pub mod app;
pub mod asset;
pub mod config;
pub mod consumer;
pub mod data;
pub mod event;
pub mod identity;
pub mod status;
pub mod store;

use std::fmt;

/// Messaging compliance level of the application (MG 1.3).
///
/// The level is a promise to the Message Bus: the SDK enables every behavior
/// that is mandatory **or recommended** at this level (REQ-CMP-001 …
/// REQ-CMP-047) and rejects at [`app::AppBuilder::build`] a configuration that
/// cannot keep it. Offering more than the level requires (e.g. a writable
/// DataSet at Level 1) is allowed. The Service Container level is a property
/// of the image, not of the SDK (D8, REQ-CNT-*).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Level {
    /// MQTT + TLS + auth, Birth/Will, Pub MAM + Health (REQ-CMP-001 … 007).
    L1,
    /// L1 + `Get`, `Set`/`Del` where offered, Status events (REQ-CMP-008 … 023).
    L2,
    /// All resources, `Set`/`Del` wherever defined, `Call`/`Reply` +
    /// `NewDataSetWriterId`, full MQTT client behavior (REQ-CMP-033 … 040).
    L3,
}

/// `ServiceType` topic element (REQ-TOP-005); one per application. The MAM
/// `DeviceClass` of the application is derived as `Oi4.<ServiceType>`
/// (REQ-RES-MAM-014).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ServiceType {
    Registry,
    OTConnector,
    Utility,
    Persistence,
    Aggregation,
    OOCConnector,
    ITConnector,
}

/// Errors of the SDK API (not protocol status codes, see [`status::StatusCode`]).
#[derive(Debug)]
#[non_exhaustive]
pub enum Error {
    /// An Identification Link, Oi4Identifier element, name or value violates
    /// the guideline (e.g. REQ-IL-008/016/019, REQ-RES-CONFIG-004).
    Invalid(String),
    /// The configuration cannot fulfill the chosen [`Level`].
    LevelNotMet(String),
    /// Container storage (`/etc/oi4`, `/run/secrets/oi4`) missing or invalid.
    Bootstrap(String),
    /// Connection, TLS or I/O failure.
    Io(std::io::Error),
    /// No answer within the timeout (consumer side, REQ-ACT-009).
    Timeout,
    /// The application was shut down.
    Closed,
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        unimplemented!()
    }
}

impl std::error::Error for Error {}

/// Result type of the SDK API.
pub type Result<T, E = Error> = std::result::Result<T, E>;

/// Everything a typical application needs.
pub mod prelude {
    pub use crate::app::{App, AppHandle, Bootstrap, Broker};
    pub use crate::asset::{Asset, AssetDef, HealthState};
    pub use crate::config::{ConfigDef, ConfigObject, ConfigValues};
    pub use crate::consumer::{Consumer, Resource, Target};
    pub use crate::data::{BuiltinType, DataSet, DataSetDef, Field, Mode, Publication, Value};
    pub use crate::event::SyslogLevel;
    pub use crate::identity::{IdentificationLink, Oi4Identifier, Product};
    pub use crate::status::StatusCode;
    pub use crate::{Level, ServiceType};
}
