//! Checks of the API sketch (ADR-0020): the DX promise and the REQ tracing.

use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};

fn manifest_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

/// P3 DX goal: "hello world" OEC app in at most 20 lines.
#[test]
fn hello_world_has_at_most_20_lines() {
    let hello = fs::read_to_string(manifest_dir().join("examples/hello.rs")).unwrap();
    let lines = hello.lines().count();
    assert!(lines <= 20, "examples/hello.rs has {lines} lines");
}

/// REQ id → deprecated, read from oec/spec/requirements/*.yaml.
fn catalog() -> HashMap<String, bool> {
    let dir = manifest_dir().join("../../../spec/requirements");
    let mut reqs = HashMap::new();
    for entry in fs::read_dir(&dir).unwrap() {
        let path = entry.unwrap().path();
        if path.extension().is_none_or(|e| e != "yaml") {
            continue;
        }
        let mut current: Option<String> = None;
        for line in fs::read_to_string(&path).unwrap().lines() {
            if let Some(id) = line.strip_prefix("  - id: ") {
                current = Some(id.trim().to_owned());
                reqs.insert(id.trim().to_owned(), false);
            } else if let Some(status) = line.strip_prefix("    status: ") {
                if let Some(id) = &current {
                    reqs.insert(id.clone(), status.trim() == "deprecated");
                }
            }
        }
    }
    assert!(reqs.len() > 400, "requirements catalog not found at {}", dir.display());
    reqs
}

fn walk(dir: &Path, files: &mut Vec<PathBuf>) {
    for entry in fs::read_dir(dir).unwrap() {
        let path = entry.unwrap().path();
        if path.is_dir() {
            if path.file_name().is_some_and(|n| n != "target") {
                walk(&path, files);
            }
        } else if path
            .extension()
            .is_some_and(|e| ["rs", "h", "c", "md"].iter().any(|x| e == *x))
        {
            files.push(path);
        }
    }
}

/// Complete REQ ids (`REQ-<AREA>-NNN`) in a text.
fn req_ids(text: &str) -> Vec<String> {
    let mut ids = Vec::new();
    let bytes = text.as_bytes();
    let mut i = 0;
    while let Some(pos) = text[i..].find("REQ-") {
        let start = i + pos;
        let mut end = start + 4;
        while end < bytes.len() && (bytes[end].is_ascii_uppercase() || bytes[end] == b'-') {
            end += 1;
        }
        let digits = bytes[end..].iter().take_while(|b| b.is_ascii_digit()).count();
        if digits == 3 && bytes[end - 1] == b'-' {
            ids.push(text[start..end + 3].to_owned());
        }
        i = end.max(start + 4);
    }
    ids
}

/// Every REQ id cited anywhere in `oec/sdk/` (sketches, headers, design
/// documents) exists in the catalog and is not deprecated.
#[test]
fn cited_req_ids_exist_and_are_active() {
    let reqs = catalog();
    let mut files = Vec::new();
    walk(&manifest_dir().join("../.."), &mut files);
    let mut bad = Vec::new();
    let mut cited = 0;
    for file in &files {
        for id in req_ids(&fs::read_to_string(file).unwrap()) {
            cited += 1;
            match reqs.get(&id) {
                None => bad.push(format!("{}: unknown {id}", file.display())),
                Some(true) => bad.push(format!("{}: deprecated {id}", file.display())),
                Some(false) => {}
            }
        }
    }
    assert!(cited > 100, "only {cited} REQ citations found");
    assert!(bad.is_empty(), "{}", bad.join("\n"));
}
