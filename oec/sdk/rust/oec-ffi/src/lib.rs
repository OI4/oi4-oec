//! C ABI of the OEC Rust SDK (D2, ADR-0021): opaque handles with builder
//! setters, JSON text for structured values, `oec_status` plus a
//! thread-local message for errors. Every function wraps the Rust API of
//! [`oec`] 1:1 (ADR-0021 §10); the ABI has no protocol logic of its own.
//!
//! The header `include/oec.h` is generated from this crate by cbindgen
//! (`cbindgen.toml`, ADR-0023) and compared with a fresh output by
//! `oec/sdk/check.sh`. The memory and thread rules are in its header
//! comment (`cbindgen.toml`).
//!
//! Panics never cross the ABI: every exported function runs inside a
//! guard that turns a panic into `OEC_ERR_PANIC` and marks the app
//! unusable.

mod abi;
mod app;
mod asset;
mod consumer;
mod types;

pub use abi::*;
pub use app::*;
pub use asset::*;
pub use consumer::*;
pub use types::*;
