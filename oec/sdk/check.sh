#!/bin/sh
# Checks of the SDKs (ADR-0026; C ABI ADR-0021) and of the remaining P3 sketch
# (oecm.h, ADR-0022):
#   0. rust (Cargo workspace of D2: oec-core from C-046, oec from C-048,
#      oec-ffi from C-061):
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
#      ADR-0023 amendment C-050; oec-ffi as well from C-061). cargo-deny fetches the RustSec
#      advisory DB; without network it falls back to the cached DB (a real
#      violation fails both runs). Needs cargo-deny
#      (cargo install cargo-deny --locked).
#      The examples of oec (oec/examples: hello, config, device_source,
#      consumer, demo) are built by clippy --all-targets; oec/tests/citations.rs
#      checks hello world ≤ 20 lines and that every REQ id cited under
#      oec/sdk/ exists and is not deprecated (moved from the P3 API sketch,
#      deleted in C-060, ADR-0026 §1);
#      rust/oec-ffi (C ABI of D2, C-061, ADR-0021) is a workspace member:
#      its unit tests and tests/c.rs run with the workspace; tests/c.rs
#      compiles examples/{hello,logger}.c and tests/c/abi.c with the system
#      C compiler against the shared library and against the static library
#      under AddressSanitizer + UBSan and runs them against the in-process
#      TLS test broker (OEC_FFI_REQUIRE_SANITIZERS=1: a toolchain without
#      sanitizers fails instead of skipping; set it to 0 to skip there);
#   1. rust/oec-ffi/include/oec.h is the output of cbindgen 0.29.4 (pinned,
#      CLI only, ADR-0023 amendment C-061): regenerated into a temporary
#      file and compared; the header with the examples and tests/c/*.c
#      compile as C17 without warnings; oecm.h (D9 sketch) and its
#      examples as well (syntax only – no implementation);
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
cbindgen_version="0.29.4"
export OEC_FFI_REQUIRE_SANITIZERS="${OEC_FFI_REQUIRE_SANITIZERS:-1}"

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
cargo clippy --offline --locked -p oec -p oec-ffi --all-targets --quiet --no-default-features --features=crypto-ring -- -D warnings
command -v cargo-deny >/dev/null || { echo "cargo-deny missing: cargo install cargo-deny --locked" >&2; exit 1; }
cargo deny --log-level error check || {
	echo "cargo deny: retrying with the cached advisory DB (no network?)" >&2
	cargo deny --log-level error --frozen check
}

echo "rust/oec-ffi: oec.h = cbindgen $cbindgen_version output; examples + tests/c (C17)"
command -v cbindgen >/dev/null || { echo "cbindgen missing: cargo install cbindgen --version $cbindgen_version --locked" >&2; exit 1; }
test "$(cbindgen --version)" = "cbindgen $cbindgen_version" || {
	echo "cbindgen $cbindgen_version required, found: $(cbindgen --version)" >&2; exit 1; }
header="$(mktemp)"
(cd "$root/rust/oec-ffi" && cbindgen --quiet --config cbindgen.toml --output "$header")
diff -u "$root/rust/oec-ffi/include/oec.h" "$header" || {
	rm -f "$header"
	echo "rust/oec-ffi/include/oec.h is outdated: run cbindgen --config cbindgen.toml --output include/oec.h in rust/oec-ffi" >&2
	exit 1
}
rm -f "$header"
for f in "$root"/rust/oec-ffi/examples/*.c "$root"/rust/oec-ffi/tests/c/*.c; do
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
