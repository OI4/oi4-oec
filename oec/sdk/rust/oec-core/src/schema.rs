//! The embedded D1 schemas (ADR-0019 §7, ADR-0023).
//!
//! [`VERSION`] and [`BASELINE`] identify the embedded schema tree: they are
//! read from `oec/schema/schema.go` at build time, so a working tree such as
//! `2.0.1-dev` reports itself and not the newest release manifest. Every
//! validation result should carry them (ADR-0019 §6).
//!
//! With the feature `strict-validation`, the strict schema sources are
//! compiled into the binary and `Validator` checks a message against the
//! entry schema of its topic form, with the same routing and the same
//! requirement attribution as the schema gate (`oec/schema/tools`,
//! ADR-0019 §8). The tolerant flavor (REQ-MSG-046) is derived in memory by
//! dropping the closures (`unevaluatedProperties`), as the gate does.

mod embedded {
    include!(concat!(env!("OUT_DIR"), "/embedded.rs"));
}

pub use embedded::{BASE_URI, BASELINE, TOLERANT_BASE_URI, VERSION};

#[cfg(feature = "strict-validation")]
mod validator;

#[cfg(feature = "strict-validation")]
pub use validator::{Entry, Flavor, SchemaError, SchemaViolation, SetupError, Validator};
