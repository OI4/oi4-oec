//! Embeds the strict schema sources `oec/schema/v2/**/*.schema.json` and the
//! version of the newest release manifest `oec/schema/release/<v>.json`.
//! The generated `$OUT_DIR/embedded.rs` holds one `include_str!` per file, so
//! the binary needs no schema files at run time (ADR-0019 §7).

use std::path::{Path, PathBuf};
use std::{env, fs};

fn collect(dir: &Path, out: &mut Vec<PathBuf>) {
    for e in fs::read_dir(dir).expect("read schema dir") {
        let p = e.expect("dir entry").path();
        if p.is_dir() {
            collect(&p, out);
        } else if p.to_string_lossy().ends_with(".schema.json") {
            out.push(p);
        }
    }
}

/// Value of `"key": "value"` in a flat JSON text (the build script stays
/// dependency-free; the manifest format is fixed by oec/schema/release).
fn field(text: &str, key: &str) -> String {
    let k = format!("\"{key}\": \"");
    let start = text.find(&k).unwrap_or_else(|| panic!("manifest has no {key}")) + k.len();
    text[start..].split('"').next().unwrap().to_string()
}

fn version_key(v: &str) -> Vec<u64> {
    v.split(|c: char| !c.is_ascii_digit()).filter(|s| !s.is_empty()).map(|s| s.parse().unwrap()).collect()
}

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let schema = manifest_dir.join("../../../../schema").canonicalize().expect("oec/schema");
    let v2 = schema.join("v2");
    let release = schema.join("release");
    println!("cargo:rerun-if-changed={}", v2.display());
    println!("cargo:rerun-if-changed={}", release.display());

    let mut files = Vec::new();
    collect(&v2, &mut files);
    files.sort();

    let mut manifests: Vec<PathBuf> = fs::read_dir(&release)
        .expect("release dir")
        .map(|e| e.unwrap().path())
        .filter(|p| p.extension().is_some_and(|x| x == "json"))
        .collect();
    manifests.sort_by_key(|p| version_key(&p.file_stem().unwrap().to_string_lossy()));
    let newest = fs::read_to_string(manifests.last().expect("a release manifest")).unwrap();

    let mut src = String::new();
    src.push_str(&format!("pub const SCHEMA_VERSION: &str = {:?};\n", field(&newest, "version")));
    src.push_str(&format!("pub const SCHEMA_BASELINE: &str = {:?};\n", field(&newest, "baseline")));
    src.push_str("pub static SCHEMAS: &[(&str, &str)] = &[\n");
    for f in &files {
        let rel = f.strip_prefix(&v2).unwrap().to_string_lossy().replace('\\', "/");
        assert!(newest.contains(&format!("\"path\": \"{rel}\"")), "{rel} is not in the release manifest");
        println!("cargo:rerun-if-changed={}", f.display());
        src.push_str(&format!("    ({rel:?}, include_str!({:?})),\n", f.display().to_string()));
    }
    src.push_str("];\n");
    fs::write(PathBuf::from(env::var("OUT_DIR").unwrap()).join("embedded.rs"), src).unwrap();
}
