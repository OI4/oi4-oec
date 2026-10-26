//! Acceptance test of the message layer: the payload vectors of the spec
//! baseline (`oec/spec/export/vectors/payload-*.json`, ADR-0026).
//!
//! - Every valid vector decodes into the typed model and encodes back to the
//!   same JSON value (tolerant vectors: the encoding drops the unknown keys
//!   and passes the strict schema).
//! - No invalid vector makes the decoder panic.
//! - With the feature `strict-validation`, the embedded schemas judge every
//!   vector as the schema gate does (ADR-0019 §8): same entry routing, same
//!   verdict, same REQ ids, the rule-layer partition of
//!   `oec/schema/tests/rule-layer.yaml` included.

use std::path::PathBuf;

use oec_core::message::{DataSetMetaData, DecodeError, Message};
use oec_core::topic::Topic;
use serde_json::Value;

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..")
}

fn payload_vectors() -> Vec<Value> {
    let mut all = Vec::new();
    for f in ["payload-guideline.json", "payload-constructed.json"] {
        let path = root().join("spec/export/vectors").join(f);
        let text = std::fs::read_to_string(&path).unwrap();
        let doc: Value = serde_json::from_str(&text).unwrap();
        all.extend(doc["vectors"].as_array().unwrap().iter().cloned());
    }
    assert_eq!(all.len(), 245, "payload vectors of the baseline");
    all
}

fn str_field<'a>(v: &'a Value, key: &str) -> Option<&'a str> {
    v.get(key).and_then(Value::as_str)
}

fn payload(v: &Value) -> Value {
    serde_json::from_str(str_field(v, "payload").unwrap()).unwrap()
}

fn is_bare_metadata(v: &Value) -> bool {
    str_field(v, "object") == Some("DataSetMetaDataType")
}

fn is_tolerant(v: &Value) -> bool {
    str_field(v, "mode") == Some("tolerant")
}

/// Decodes a vector into the model and encodes it again.
fn round_trip(v: &Value) -> Result<Value, String> {
    let instance = payload(v);
    if is_bare_metadata(v) {
        let m: DataSetMetaData = serde_json::from_value(instance).map_err(|e| e.to_string())?;
        return serde_json::to_value(m).map_err(|e| e.to_string());
    }
    let topic = Topic::parse(str_field(v, "topic").unwrap()).map_err(|e| e.to_string())?;
    Message::from_value(&topic, instance)
        .and_then(|m| m.to_value())
        .map_err(|e: DecodeError| e.to_string())
}

#[test]
fn valid_vectors_round_trip() {
    let mut failures = Vec::new();
    let mut n = 0;
    for v in payload_vectors() {
        if str_field(&v, "expected") != Some("valid") {
            continue;
        }
        n += 1;
        let id = str_field(&v, "id").unwrap();
        match round_trip(&v) {
            Err(e) => failures.push(format!("{id}: does not decode: {e}")),
            Ok(out) if is_tolerant(&v) => {
                // the unknown keys are dropped; the encoding is stable
                let mut again = v.clone();
                again["payload"] = Value::String(out.to_string());
                match round_trip(&again) {
                    Ok(out2) if out2 == out => {}
                    other => failures.push(format!("{id}: unstable re-encoding: {other:?}")),
                }
            }
            Ok(out) if out != payload(&v) => {
                failures.push(format!("{id}: re-encodes to {out}"));
            }
            Ok(_) => {}
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
    assert_eq!(n, 245 - 143, "valid payload vectors");
}

#[test]
fn invalid_vectors_do_not_panic() {
    let (mut decoded, mut refused) = (0, 0);
    for v in payload_vectors() {
        if str_field(&v, "expected") == Some("valid") {
            continue;
        }
        match round_trip(&v) {
            Ok(_) => decoded += 1,
            Err(_) => refused += 1,
        }
    }
    // the decoder refuses what breaks the model; the rest is left to the
    // schema (`strict-validation`) and the rule layer
    assert_eq!(decoded + refused, 143);
    eprintln!(
        "invalid payload vectors: {refused} refused by the decoder, {decoded} left to the schema"
    );
}

#[cfg(feature = "strict-validation")]
mod strict {
    use std::collections::HashMap;

    use oec_core::schema::{Entry, Flavor, Validator};
    use oec_core::topic::Topic;

    use serde_json::Value;

    use super::{payload, payload_vectors, root, str_field};

    fn strings(v: &Value, key: &str) -> Vec<String> {
        v.get(key)
            .and_then(Value::as_array)
            .map(|a| a.iter().map(|s| s.as_str().unwrap().to_owned()).collect())
            .unwrap_or_default()
    }

    /// The `schema` partition per vector of the expectation list (a minimal
    /// reader of the one fixed layout of rule-layer.yaml; the test needs no
    /// YAML dependency).
    fn rule_layer() -> HashMap<String, Vec<String>> {
        let text = std::fs::read_to_string(root().join("schema/tests/rule-layer.yaml")).unwrap();
        let mut out = HashMap::new();
        let mut current = None;
        for line in text.lines() {
            let t = line.trim();
            if let Some(id) = t.strip_prefix("- vector:") {
                current = Some(id.trim().to_owned());
            } else if let Some(list) = t.strip_prefix("schema:") {
                let id = current.take().expect("schema: without vector:");
                let list = list.trim().trim_start_matches('[').trim_end_matches(']');
                let reqs = list
                    .split(',')
                    .map(str::trim)
                    .filter(|s| !s.is_empty())
                    .map(str::to_owned)
                    .collect();
                out.insert(id, reqs);
            }
        }
        assert_eq!(out.len(), 25, "rule-layer.yaml expectations");
        out
    }

    #[test]
    fn verdicts_equal_the_schema_gate() {
        let validator = Validator::new().unwrap();
        let expects = rule_layer();
        let mut failures = Vec::new();
        let (mut routed, mut rejected, mut agnostic) = (0, 0, 0);
        for v in payload_vectors() {
            let id = str_field(&v, "id").unwrap();
            let flavor = if str_field(&v, "mode") == Some("tolerant") {
                Flavor::Tolerant
            } else {
                Flavor::Strict
            };
            let valid = str_field(&v, "expected") == Some("valid");
            let instance = payload(&v);
            let topic = str_field(&v, "topic").and_then(|t| Topic::parse(t).ok());
            let (entry, generic) = if str_field(&v, "object") == Some("DataSetMetaDataType") {
                (Some(Entry::resource_metadata()), Entry::data_set_metadata())
            } else {
                (
                    topic.as_ref().and_then(|t| validator.entry(&t.rule())),
                    Entry::message(),
                )
            };
            let Some(entry) = entry else {
                agnostic += 1;
                if valid && let Err(e) = validator.validate(&generic, flavor, &instance) {
                    failures.push(format!("{id}: valid vector rejected: {e}"));
                }
                continue;
            };
            routed += 1;
            let b = validator.validate(&entry, flavor, &instance);
            let a = validator.validate(&generic, flavor, &instance);
            if valid {
                if let Err(e) = b {
                    failures.push(format!("{id}: valid vector rejected: {e}"));
                }
                if let Err(e) = a {
                    failures.push(format!("{id}: valid vector rejected: {e}"));
                }
                continue;
            }
            let mut want = expects
                .get(id)
                .cloned()
                .unwrap_or_else(|| strings(&v, "violates"));
            want.sort();
            match (want.is_empty(), b) {
                (true, Err(e)) => failures.push(format!("{id}: rule-layer vector rejected: {e}")),
                (false, Ok(())) => failures.push(format!("{id}: invalid vector accepted")),
                (false, Err(e)) if e.reqs != want => {
                    failures.push(format!("{id}: reports {:?}, want {want:?}: {e}", e.reqs));
                }
                (false, Err(_)) => rejected += 1,
                _ => {}
            }
        }
        assert!(failures.is_empty(), "{}", failures.join("\n"));
        // as the gate: every vector has an entry schema (bare MetaData included)
        assert_eq!((routed, agnostic), (245, 0), "routing");
        assert_eq!(rejected, 143 - 25 + 1, "rejected with the gate's REQs");
    }

    /// What the model encodes passes the strict entry schema, also for the
    /// tolerant vectors whose unknown keys the decoder drops.
    #[test]
    fn encodings_pass_the_strict_schema() {
        let validator = Validator::new().unwrap();
        let mut failures = Vec::new();
        for v in payload_vectors() {
            if str_field(&v, "expected") != Some("valid") {
                continue;
            }
            let id = str_field(&v, "id").unwrap();
            let out = super::round_trip(&v).unwrap();
            let entry = if super::is_bare_metadata(&v) {
                Entry::resource_metadata()
            } else {
                let topic = Topic::parse(str_field(&v, "topic").unwrap()).unwrap();
                validator.entry_for(&topic)
            };
            if let Err(e) = validator.validate(&entry, Flavor::Strict, &out) {
                failures.push(format!("{id}: {e}"));
            }
        }
        assert!(failures.is_empty(), "{}", failures.join("\n"));
    }
}
