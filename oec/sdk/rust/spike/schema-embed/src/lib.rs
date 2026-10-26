//! P3 spike (C-043, ADR-0023): the shape of the embedded schema module of D2.
//!
//! The SDK validates every message it sends against the strict flavor
//! ("strict sender", ADR-0020) before it publishes it. The schemas are
//! compiled into the binary; validators are built lazily per entry schema
//! and cached, because building all of them costs start-up time.

use std::collections::HashMap;
use std::sync::{Mutex, OnceLock};

use jsonschema::{Draft, Registry, Validator};
use serde_json::{json, Value};

mod embedded {
    include!(concat!(env!("OUT_DIR"), "/embedded.rs"));
}

pub use embedded::{SCHEMAS, SCHEMA_BASELINE, SCHEMA_VERSION};

/// `$id` base of the strict flavor (ADR-0019 §6).
pub const BASE: &str = "https://schemas.openindustry4.com/oec/v2/";

static REGISTRY: OnceLock<Registry<'static>> = OnceLock::new();
static VALIDATORS: OnceLock<Mutex<HashMap<String, &'static Validator>>> = OnceLock::new();

/// The registry over all embedded schemas (built once).
pub fn registry() -> &'static Registry<'static> {
    REGISTRY.get_or_init(|| {
        let resources = SCHEMAS.iter().map(|(rel, text)| {
            let doc: Value = serde_json::from_str(text).unwrap_or_else(|e| panic!("{rel}: {e}"));
            assert_eq!(doc["$id"].as_str(), Some(format!("{BASE}{rel}").as_str()), "{rel}: $id");
            (format!("{BASE}{rel}"), doc)
        });
        Registry::new().extend(resources).expect("registry").prepare().expect("prepare registry")
    })
}

/// The cached validator for a schema reference (`$id`, optionally with a
/// fragment), e.g. `…/oec/v2/topics/pub-health.schema.json`.
pub fn validator(reference: &str) -> &'static Validator {
    let cache = VALIDATORS.get_or_init(|| Mutex::new(HashMap::new()));
    let mut cache = cache.lock().unwrap();
    cache.entry(reference.to_string()).or_insert_with(|| {
        let v = jsonschema::options()
            .with_draft(Draft::Draft202012)
            .with_registry(registry())
            .build(&json!({ "$ref": reference }))
            .unwrap_or_else(|e| panic!("{reference}: {e}"));
        Box::leak(Box::new(v))
    })
}
