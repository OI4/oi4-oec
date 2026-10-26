#!/bin/sh
# Checks of the SDKs (ADR-0026 proposed) and of the P3 sketches (ADR-0020 … 0022):
#   0. rust (Cargo workspace of D2: oec-core from C-046, oec from C-048):
#      rustfmt, clippy (pedantic, warnings are errors), tests including the
#      shared test vectors of oec/spec/export (IL, topic, payload,
#      broker.json), rustdoc without warnings, cargo-deny (licenses,
#      advisories, bans, sources; ADR-0023 §2). clippy, tests and rustdoc
#      run twice: default features and `--features strict-validation`
#      (oec-core embeds oec/schema/v2 and validates with jsonschema, C-047). cargo-deny fetches the RustSec
#      advisory DB; without network it falls back to the cached DB (a real
#      violation fails both runs). Needs cargo-deny
#      (cargo install cargo-deny --locked).
#   1. rust/sketch: the public Rust API sketch of D2 compiles with all DX
#      examples; tests: hello world ≤ 20 lines, every REQ id cited under
#      oec/sdk/ exists and is not deprecated;
#   2. the C headers oec.h (D2 C ABI) and oecm.h (D9) and their examples
#      compile as C17 without warnings (syntax only – no implementation);
#   3. spikes (C-043, ADR-0023): rust/spike/schema-embed embeds oec/schema/v2
#      at compile time and reproduces every strict verdict of
#      oec/schema/dist/cases.json (needs schema/check.sh first, as in
#      oec/check.sh); rust/spike/mqtt-codec is the sans-IO MQTT 3.1.1 codec
#      of option (a) of Q-026, byte-exact against hand-encoded packets.
set -eu
root="$(cd "$(dirname "$0")" && pwd)"
cc="${CC:-cc}"
cflags="-std=c17 -Wall -Wextra -Wpedantic -Werror -fsyntax-only"

echo "rust: cargo fmt, clippy, test, doc, deny"
cd "$root/rust"
cargo fmt --all --check
for features in "" "--features=strict-validation"; do
	# shellcheck disable=SC2086
	cargo clippy --offline --locked --all-targets --quiet $features -- -D warnings
	# shellcheck disable=SC2086
	cargo test --offline --locked --quiet $features
	# shellcheck disable=SC2086
	RUSTDOCFLAGS="-D warnings" cargo doc --offline --locked --no-deps --quiet $features
done
command -v cargo-deny >/dev/null || { echo "cargo-deny missing: cargo install cargo-deny --locked" >&2; exit 1; }
cargo deny --log-level error check || {
	echo "cargo deny: retrying with the cached advisory DB (no network?)" >&2
	cargo deny --log-level error --frozen check
}

echo "rust/sketch: cargo test"
cd "$root/rust/sketch"
cargo test --offline --locked --quiet

echo "rust/sketch/c: oec.h + examples (C17)"
for f in "$root"/rust/sketch/c/examples/*.c; do
	# shellcheck disable=SC2086
	"$cc" $cflags -I "$root/rust/sketch/c/include" "$f"
done

echo "micro/sketch: oecm.h + examples (C17)"
for f in "$root"/micro/sketch/examples/*.c; do
	# shellcheck disable=SC2086
	"$cc" $cflags -I "$root/micro/sketch/include" "$f"
done

for spike in schema-embed mqtt-codec; do
	echo "rust/spike/$spike: cargo test"
	cd "$root/rust/spike/$spike"
	cargo test --offline --locked --quiet
done

echo "sdk checks ok"
