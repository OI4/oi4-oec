//! Embeds the D1 schema tree (ADR-0019 §7, ADR-0023, ADR-0026 amendment C-047).
//!
//! Always: `schema::VERSION` and `schema::BASELINE` of the embedded tree, read
//! from `oec/schema/schema.go` (the working version, e.g. `2.0.1-dev`, not the
//! newest release manifest). With the feature `strict-validation`: one
//! `include_str!` per strict source file `oec/schema/v2/**/*.schema.json`, so
//! the binary needs no schema file at run time. The tolerant flavor is derived
//! in memory (`schema::Flavor::Tolerant`), not embedded.

use std::fmt::Write as _;
use std::path::{Path, PathBuf};
use std::{env, fs};

fn collect(dir: &Path, out: &mut Vec<PathBuf>) {
    for e in fs::read_dir(dir).expect("read oec/schema/v2") {
        let p = e.expect("dir entry").path();
        if p.is_dir() {
            collect(&p, out);
        } else if p.to_string_lossy().ends_with(".schema.json") {
            out.push(p);
        }
    }
}

/// Value of `const <name> = "<value>"` in schema.go (the build script stays
/// dependency-free; the constants are fixed by oec/schema/schema.go).
fn go_const(text: &str, name: &str) -> String {
    let key = format!("const {name} = \"");
    let start = text
        .find(&key)
        .unwrap_or_else(|| panic!("oec/schema/schema.go has no {key}"))
        + key.len();
    text[start..].split('"').next().unwrap().to_owned()
}

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let schema = manifest_dir
        .join("../../../schema")
        .canonicalize()
        .expect("oec/schema");
    let go = schema.join("schema.go");
    let v2 = schema.join("v2");
    println!("cargo:rerun-if-changed={}", go.display());
    println!("cargo:rerun-if-changed={}", v2.display());

    let text = fs::read_to_string(&go).expect("read oec/schema/schema.go");
    let version = go_const(&text, "Version");
    let baseline = go_const(&text, "Baseline");
    let base_uri = go_const(&text, "BaseURI");
    let mut src = String::new();
    let tolerant = format!("{base_uri}tolerant/");
    writeln!(
        src,
        "/// Version of the embedded schema tree (`oec/schema/schema.go`, ADR-0019 §6).\npub const VERSION: &str = {version:?};"
    )
    .unwrap();
    writeln!(
        src,
        "/// Guideline commit the embedded schema tree is derived from (`oec/schema/schema.go`, ADR-0018).\npub const BASELINE: &str = {baseline:?};"
    )
    .unwrap();
    writeln!(
        src,
        "/// `$id` prefix of every strict schema (ADR-0019 §6).\npub const BASE_URI: &str = {base_uri:?};"
    )
    .unwrap();
    writeln!(
        src,
        "/// `$id` prefix of the tolerant flavor (REQ-MSG-046), derived in memory.\npub const TOLERANT_BASE_URI: &str = {tolerant:?};"
    )
    .unwrap();

    if env::var_os("CARGO_FEATURE_STRICT_VALIDATION").is_some() {
        let mut files = Vec::new();
        collect(&v2, &mut files);
        files.sort();
        src.push_str("/// The strict schema sources: path below `v2/` and text.\n");
        src.push_str("pub(crate) static SOURCES: &[(&str, &str)] = &[\n");
        for f in &files {
            let rel = f
                .strip_prefix(&v2)
                .unwrap()
                .to_string_lossy()
                .replace('\\', "/");
            println!("cargo:rerun-if-changed={}", f.display());
            writeln!(
                src,
                "    ({rel:?}, include_str!({:?})),",
                f.display().to_string()
            )
            .unwrap();
        }
        src.push_str("];\n");
    }
    fs::write(
        PathBuf::from(env::var("OUT_DIR").unwrap()).join("embedded.rs"),
        src,
    )
    .unwrap();
}
