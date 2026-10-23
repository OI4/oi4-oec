//! Second, independent JSON Schema 2020-12 validator of the ADR-0019 PoC.
//! Loads every schema of the bundle written by `schematest` (dist/), runs the
//! cases of dist/cases.json with the Rust `jsonschema` crate and reports every
//! verdict that differs from the first validator (santhosh-tekuri/jsonschema).

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::{env, fs, process};

use jsonschema::{Draft, Registry};
use serde_json::{json, Value};

fn collect(dir: &Path, out: &mut Vec<PathBuf>) {
    for e in fs::read_dir(dir).expect("read dist") {
        let p = e.expect("dir entry").path();
        if p.is_dir() {
            collect(&p, out);
        } else if p.to_string_lossy().ends_with(".schema.json") {
            out.push(p);
        }
    }
}

fn main() {
    let dist = PathBuf::from(env::args().nth(1).unwrap_or_else(|| "../../dist".into()));
    let mut files = Vec::new();
    collect(&dist, &mut files);
    let mut resources = Vec::new();
    for f in &files {
        let doc: Value = serde_json::from_str(&fs::read_to_string(f).expect("read schema")).expect("parse schema");
        let id = doc["$id"].as_str().expect("$id").to_string();
        // every file is a valid 2020-12 schema on its own
        if let Err(e) = jsonschema::meta::validate(&doc) {
            eprintln!("FAIL {id}: not a valid schema: {e}");
            process::exit(1);
        }
        resources.push((id, doc));
    }
    let registry = Registry::new()
        .extend(resources.clone())
        .expect("registry")
        .prepare()
        .expect("prepare registry");
    let cases: Vec<Value> =
        serde_json::from_str(&fs::read_to_string(dist.join("cases.json")).expect("read cases")).expect("parse cases");
    let mut validators = BTreeMap::new();
    let mut failures = 0;
    for c in &cases {
        let id = c["id"].as_str().unwrap();
        let schema = c["schema"].as_str().unwrap().to_string();
        let v = validators.entry(schema.clone()).or_insert_with(|| {
            jsonschema::options()
                .with_draft(Draft::Draft202012)
                .with_registry(&registry)
                .build(&json!({"$ref": schema}))
                .expect("compile")
        });
        let got = v.is_valid(&c["instance"]);
        let want = c["valid"].as_bool().unwrap();
        if got != want {
            failures += 1;
            let errs: Vec<String> = v.iter_errors(&c["instance"]).map(|e| format!("{} {}", e.instance_path(), e)).collect();
            eprintln!("FAIL {id}: jsonschema-rs says valid={got}, santhosh valid={want}: {errs:?}");
        }
    }
    println!(
        "jsonschema-rs: {} schemas, {} cases, {} disagreements",
        resources.len(),
        cases.len(),
        failures
    );
    if failures > 0 {
        process::exit(1);
    }
}
