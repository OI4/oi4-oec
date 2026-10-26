//! The `broker.json` vectors of the baseline
//! (`oec/spec/export/vectors/platform-broker-json.json`, ADR-0026 §3) run
//! end to end through `Bootstrap::ContainerStorage` (REQ-CNT-003,
//! REQ-CNT-004); with `strict-validation` the schema
//! `platform/broker.schema.json` gives the same verdicts.

mod common;

use std::path::PathBuf;

use oec::prelude::*;
use serde_json::Value;

use common::{Root, product};

fn vectors() -> Vec<Value> {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../../spec/export/vectors/platform-broker-json.json");
    let doc: Value = serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
    assert_eq!(doc["kind"], "broker-json");
    assert_eq!(doc["baseline"], oec_core::schema::BASELINE);
    doc["vectors"].as_array().unwrap().clone()
}

fn reqs(v: &Value) -> Vec<&str> {
    v.get("reqs")
        .or_else(|| v.get("violates"))
        .and_then(Value::as_array)
        .unwrap()
        .iter()
        .map(|r| r.as_str().unwrap())
        .collect()
}

#[test]
fn broker_json_vectors() {
    let (mut valid, mut invalid) = (0, 0);
    for v in vectors() {
        let id = v["id"].as_str().unwrap();
        let content = v["content"].as_str().unwrap();
        assert_eq!(reqs(&v), ["REQ-CNT-003"], "{id}");
        let root = Root::container(&format!("vector-{id}"));
        root.write("etc/oi4/mqtt/broker.json", content);
        let result = App::builder(Level::L1, ServiceType::Utility, product())
            .bootstrap(root.bootstrap())
            .build();
        match v["expected"].as_str().unwrap() {
            "valid" => {
                valid += 1;
                let app = result.unwrap_or_else(|e| panic!("{id}: {e}"));
                let doc: Value = serde_json::from_str(content).unwrap();
                let c = app.connection();
                assert_eq!(c.host(), doc["Address"], "{id}");
                assert_eq!(
                    Some(u64::from(c.port())),
                    doc["SecurePort"].as_u64(),
                    "{id}"
                );
                assert_eq!(
                    c.max_packet_size() as u64,
                    doc["MaxPacketSize"].as_u64().unwrap()
                );
                assert_eq!(c.publish_limit(), c.max_packet_size(), "{id}");
            }
            "invalid" => {
                invalid += 1;
                match result {
                    Err(oec::Error::Bootstrap(m)) => {
                        assert!(m.contains("REQ-CNT-003"), "{id}: {m}");
                    }
                    other => panic!("{id}: expected Error::Bootstrap, got {other:?}"),
                }
            }
            e => panic!("{id}: expected {e}"),
        }
    }
    assert_eq!((valid, invalid), (4, 14));
}

#[cfg(feature = "strict-validation")]
#[test]
fn same_verdicts_as_the_schema() {
    use oec_core::schema::{Flavor, Validator};
    let validator = Validator::new().unwrap();
    let entry = validator.schema("platform/broker.schema.json").unwrap();
    for v in vectors() {
        let id = v["id"].as_str().unwrap();
        let doc: Value = serde_json::from_str(v["content"].as_str().unwrap()).unwrap();
        let schema = validator.validate(&entry, Flavor::Strict, &doc);
        match v["expected"].as_str().unwrap() {
            "valid" => assert!(schema.is_ok(), "{id}: {schema:?}"),
            _ => assert_eq!(schema.unwrap_err().reqs, reqs(&v), "{id}"),
        }
    }
}
