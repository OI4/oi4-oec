#!/bin/sh
# Checks of the SDKs (ADR-0026) and of the remaining P3 sketches (ADR-0021, 0022):
#   0. rust (Cargo workspace of D2: oec-core from C-046, oec from C-048):
#      rustfmt, clippy (pedantic, warnings are errors), tests including the
#      shared test vectors of oec/spec/export (IL, topic, payload,
#      broker.json, lifecycle roles) and the MQTT codec/session tests with
#      proptest (C-049), the runtime of oec against an in-process TLS test
#      broker (C-050; loopback only, PKI generated per test; the interop
#      tests of oec/tests/interop.rs are compiled and linted but skip
#      without OEC_INTEROP=1, see oec/dev/interop.sh, C-051), rustdoc
#      without warnings, cargo-deny (licenses, advisories, bans, sources;
#      ADR-0023 §2). clippy, tests and rustdoc run twice: default features
#      and `--features strict-validation` (oec-core embeds oec/schema/v2
#      and validates with jsonschema, C-047); clippy once more for oec with
#      the crypto provider ring instead of aws-lc-rs (`crypto-ring`,
#      ADR-0023 amendment C-050). cargo-deny fetches the RustSec
#      advisory DB; without network it falls back to the cached DB (a real
#      violation fails both runs). Needs cargo-deny
#      (cargo install cargo-deny --locked).
#      The examples of oec (oec/examples: hello, config, device_source,
#      consumer, demo) are built by clippy --all-targets; oec/tests/citations.rs
#      checks hello world ≤ 20 lines and that every REQ id cited under
#      oec/sdk/ exists and is not deprecated (moved from the P3 API sketch,
#      deleted in C-060, ADR-0026 §1);
#   1. the C headers oec.h (D2 C ABI, rust/oec-ffi until increment 10
#      generates it) and oecm.h (D9) and their examples compile as C17
#      without warnings (syntax only – no implementation);
#   2. spikes (C-043, ADR-0023): rust/spike/schema-embed embeds oec/schema/v2
#      at compile time and reproduces every strict verdict of
#      oec/schema/dist/cases.json (needs schema/check.sh first, as in
#      oec/check.sh); rust/spike/mqtt-codec is the sans-IO MQTT 3.1.1 codec
#      of option (a) of Q-026, byte-exact against hand-encoded packets (the
#      product codec is oec-core::mqtt since C-049).
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
cargo clippy --offline --locked -p oec --all-targets --quiet --no-default-features --features=crypto-ring -- -D warnings
command -v cargo-deny >/dev/null || { echo "cargo-deny missing: cargo install cargo-deny --locked" >&2; exit 1; }
cargo deny --log-level error check || {
	echo "cargo deny: retrying with the cached advisory DB (no network?)" >&2
	cargo deny --log-level error --frozen check
}

echo "rust/oec-ffi: oec.h + examples (C17)"
for f in "$root"/rust/oec-ffi/examples/*.c; do
	# shellcheck disable=SC2086
	"$cc" $cflags -I "$root/rust/oec-ffi/include" "$f"
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
