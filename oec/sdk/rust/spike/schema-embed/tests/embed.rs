//! The embedded schemas reproduce the verdicts of the schema test suite
//! (`oec/schema/dist/cases.json`, written by `oec/schema/check.sh`) for every
//! strict-flavor case. The tolerant flavor is not embedded: the SDK validates
//! only what it sends (strict sender) and reads tolerantly with serde.

use std::time::Instant;
use std::{fs, path::Path};

use oec_schema_embed_spike::{registry, validator, SCHEMAS, SCHEMA_BASELINE, SCHEMA_VERSION};
use serde_json::Value;

#[test]
fn embeds_the_release() {
    assert_eq!(SCHEMA_VERSION, "2.0.0");
    assert_eq!(SCHEMA_BASELINE, "168e1d1");
    let manifest: Value = serde_json::from_str(
        &fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../schema/release/2.0.0.json")).unwrap(),
    )
    .unwrap();
    assert_eq!(manifest["files"].as_array().unwrap().len(), SCHEMAS.len());
}

#[test]
fn reproduces_the_strict_cases() {
    let cases = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../schema/dist/cases.json");
    let text = fs::read_to_string(&cases)
        .unwrap_or_else(|e| panic!("{}: {e} – run oec/schema/check.sh first", cases.display()));
    let cases: Vec<Value> = serde_json::from_str(&text).unwrap();

    let t0 = Instant::now();
    registry();
    let t_registry = t0.elapsed();

    let (mut run, mut failures) = (0, Vec::new());
    let t1 = Instant::now();
    for c in &cases {
        let schema = c["schema"].as_str().unwrap();
        if schema.contains("/oec/v2/tolerant/") {
            continue;
        }
        run += 1;
        let got = validator(schema).is_valid(&c["instance"]);
        if got != c["valid"].as_bool().unwrap() {
            failures.push(c["id"].as_str().unwrap().to_string());
        }
    }
    println!(
        "{} embedded schemas ({} bytes), registry {:?}, {run} strict cases incl. lazy validator builds {:?}",
        SCHEMAS.len(),
        SCHEMAS.iter().map(|(_, t)| t.len()).sum::<usize>(),
        t_registry,
        t1.elapsed()
    );
    assert!(run > 600, "too few strict cases: {run}");
    assert!(failures.is_empty(), "verdicts differ: {failures:?}");
}
