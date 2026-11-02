//! Tests of the API documents (ADR-0029 §8, §11, A-385): `api/asyncapi.json`
//! is valid against the vendored AsyncAPI 3.1.0 JSON Schema and describes
//! exactly the MQTT interface of the registry – the Registry services of
//! MG A1 on its own AppId, its own publications, the subscriptions and the
//! `Get`s it uses to collect – with the D1 topic schemas as payloads.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use oec::ServiceType;
use serde_json::Value;

use crate::bus::COLLECTED;

const D1: &str = "https://schemas.openindustry4.com/oec/v2/topics/";

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn json(path: &Path) -> Value {
    let text = std::fs::read_to_string(path).unwrap_or_else(|e| panic!("{}: {e}", path.display()));
    serde_json::from_str(&text).unwrap_or_else(|e| panic!("{}: {e}", path.display()))
}

fn asyncapi() -> Value {
    json(&root().join("api/asyncapi.json"))
}

/// Resolves a local `$ref` (`#/a/b`) of `doc`.
fn resolve<'a>(doc: &'a Value, reference: &Value) -> &'a Value {
    let r = reference["$ref"].as_str().expect("a $ref");
    let pointer = r
        .strip_prefix('#')
        .unwrap_or_else(|| panic!("not local: {r}"));
    doc.pointer(pointer)
        .unwrap_or_else(|| panic!("unresolved: {r}"))
}

#[test]
fn asyncapi_is_valid() {
    let meta = json(&root().join("api/meta/asyncapi-3.1.0.schema.json"));
    let validator = jsonschema::validator_for(&meta).expect("the meta-schema compiles");
    let doc = asyncapi();
    let errors: Vec<String> = validator
        .iter_errors(&doc)
        .map(|e| format!("{} at {}", e, e.instance_path()))
        .collect();
    assert!(errors.is_empty(), "{errors:#?}");
    // a broken document is rejected (the validator is not vacuous)
    let mut broken = doc.clone();
    broken["channels"]["getMam"]["address"] = Value::from(42);
    assert!(!validator.is_valid(&broken));
    assert_eq!(doc["info"]["version"], env!("CARGO_PKG_VERSION"));
}

/// The channels: (own AppId, method, resource, with Source).
#[test]
fn asyncapi_channels_are_the_interface() {
    let doc = asyncapi();
    let collected: BTreeSet<&str> = COLLECTED.iter().map(|r| r.as_str()).collect();
    let mut own = BTreeSet::new();
    let mut others = BTreeSet::new();
    for (name, channel) in doc["channels"].as_object().expect("channels") {
        let address = channel["address"].as_str().expect("address");
        let levels: Vec<&str> = address.split('/').collect();
        assert_eq!(levels[0], "Oi4v2", "{name}");
        // the parameters are exactly the {…} levels of the address
        let used: BTreeSet<&str> = levels
            .iter()
            .filter_map(|l| l.strip_prefix('{').and_then(|l| l.strip_suffix('}')))
            .collect();
        let declared: BTreeSet<&str> = channel["parameters"]
            .as_object()
            .map(|p| p.keys().map(String::as_str).collect())
            .unwrap_or_default();
        assert_eq!(used, declared, "{name}");
        let (method, resource) = (levels[3], levels[4]);
        assert!(collected.contains(resource), "{name}: {resource}");
        let source = match &levels[5..] {
            [] => false,
            ["{source}"] => true,
            rest => panic!("{name}: {rest:?}"),
        };
        match (levels[1], levels[2]) {
            ("Registry", "{appId}") => assert!(own.insert((method, resource, source)), "{name}"),
            ("{serviceType}", "{otherAppId}") => {
                let names: BTreeSet<&str> = channel["parameters"]["serviceType"]["enum"]
                    .as_array()
                    .expect("enum")
                    .iter()
                    .filter_map(Value::as_str)
                    .collect();
                let all: BTreeSet<&str> = ServiceType::ALL.iter().map(|s| s.as_str()).collect();
                assert_eq!(names, all, "{name}");
                assert!(others.insert((method, resource, source)), "{name}");
            }
            other => panic!("{name}: {other:?}"),
        }
        // one message, the D1 schema of the topic
        let messages = channel["messages"].as_object().expect("messages");
        assert_eq!(messages.len(), 1, "{name}");
        let message = resolve(&doc, messages.values().next().expect("one"));
        let file = format!(
            "{}-{}.schema.json",
            method.to_lowercase(),
            resource.to_lowercase()
        );
        assert_eq!(message["payload"]["$ref"], format!("{D1}{file}"), "{name}");
        let schema = json(&root().join("../schema/v2/topics").join(&file));
        assert_eq!(schema["$id"], format!("{D1}{file}"), "{name}");
    }
    // MG A1 (REQ-REG-002): Get and Pub, with and without Source, per resource
    let mut want = BTreeSet::new();
    for r in &collected {
        for m in ["Get", "Pub"] {
            for s in [false, true] {
                want.insert((m, *r, s));
            }
        }
    }
    assert_eq!(own, want);
    // collecting: every Pub with Source (subscribed with #), Get without
    let want: BTreeSet<_> = collected
        .iter()
        .flat_map(|r| [("Pub", *r, true), ("Get", *r, false)])
        .collect();
    assert_eq!(others, want);
}

/// Every operation points to a channel of the document and its messages;
/// the registry receives requests to itself and publications of others,
/// and sends its publications and `Get`s to others only (ADR-0008).
#[test]
fn asyncapi_operations_are_read_only() {
    let doc = asyncapi();
    let mut received = BTreeSet::new();
    for (name, op) in doc["operations"].as_object().expect("operations") {
        let channel = resolve(&doc, &op["channel"]);
        let address = channel["address"].as_str().expect("address");
        for m in op["messages"].as_array().expect("messages") {
            resolve(&doc, m);
        }
        let levels: Vec<&str> = address.split('/').collect();
        let own = levels[1] == "Registry";
        let action = op["action"].as_str().expect("action");
        match (own, levels[3], action) {
            (true, "Get", "receive") | (false, "Pub", "receive") => {
                received.insert(address);
            }
            (true, "Pub", "send") | (false, "Get", "send") => {}
            other => panic!("{name}: {other:?}"),
        }
        if let Some(reply) = op.get("reply") {
            assert!(own && levels[3] == "Get", "{name}");
            let to = resolve(&doc, &reply["channel"])["address"]
                .as_str()
                .expect("address");
            assert_eq!(to, address.replace("/Get/", "/Pub/"), "{name}");
            for m in reply["messages"].as_array().expect("messages") {
                resolve(&doc, m);
            }
        }
    }
    // every receiving channel has its operation
    for channel in doc["channels"].as_object().expect("channels").values() {
        let address = channel["address"].as_str().expect("address");
        let levels: Vec<&str> = address.split('/').collect();
        if (levels[1] == "Registry") == (levels[3] == "Get") {
            assert!(received.contains(address), "{address}");
        }
    }
}
