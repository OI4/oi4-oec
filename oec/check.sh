#!/bin/sh
# Local CI of the monorepo (ADR-0019 amendment C-041): the P2 gate, extended
# by the P3 sketches in C-042 and the Rust SDK in P4 (the Rust API sketch was
# deleted in C-060, the C ABI oec-ffi added in C-061).
#   1. spec/tools: gofmt, vet, tests and the spectool checks, including the
#      coverage gate (every schema/d7-validate REQ has a reviewed positive and
#      negative vector, every allowed matrix row has its entry schema);
#   2. schema/check.sh: bundle, release freeze, both validators;
#   3. spectool crosscheck: oracle vs schema on every case of schema/dist;
#   4. conformance: gofmt, vet, tests;
#   5. sdk/check.sh: the Rust SDK workspace (fmt, clippy, tests against the
#      shared vectors, doc, cargo-deny; ADR-0026, from C-046; every REQ id
#      cited under oec/sdk/ is active, oec/tests/citations.rs), the C ABI
#      oec-ffi (from C-061, ADR-0021: oec.h equals the cbindgen output, the
#      C examples and the C test program run against the shared and the
#      static library, the latter under AddressSanitizer + UBSan); the D9
#      sketch oecm.h of C-042 was replaced by the micro SDK in C-092;
#   6. dev: gofmt, vet, tests of the dev PKI generator and the lint of the
#      Mosquitto and compose configuration (C-051). No container engine: the
#      interop tests against the broker are opt-in (oec/dev/interop.sh);
#   7. demos/check.sh: the demo services of P6 (ADR-0028, from C-079): the
#      logger (C17, unit tests and integration tests under AddressSanitizer
#      + UBSan against the in-process TLS test broker, D7 test --level 2)
#      and the weather service (Go on the Go SDK, from C-080: unit tests,
#      integration tests with -race, D7 test --level 3).
#   8. registry/check.sh: the OEC Registry D5 of P7 (ADR-0029, from C-086):
#      Rust package on the SDK crate oec (fmt, clippy, tests incl. the
#      AsyncAPI document against its meta-schema, cargo-deny), Go
#      integration tests with -race against the test broker: D8 bus
#      T8-BUS-03, D7 test --level 3, HTTP/SSE/TLS and the embedded Web UI,
#      and the offline npm policy of the Web UI (internal/uipolicy).
#   9. sdk/micro/check.sh: the micro SDK D9 of P4b (ADR-0030, from C-092):
#      C17 core with the Unity tests against the shared vectors under
#      AddressSanitizer + UBSan, vector header freshness, clang --analyze,
#      the symbol allowlist of the core, GCC 14/15 if installed, the
#      emitted payloads validated by D7 `oec-check validate`, and the Go
#      host runner under D7 `oec-check test --level 1, 2 and 3` (C-093 …
#      C-095).
# The inventory check reads the guideline repository next to this repository
# (../../oi4-oec-documentation) at the baseline of spec/requirements/areas.yaml.
set -eu
root="$(cd "$(dirname "$0")" && pwd)"
export GOTOOLCHAIN=auto

gocheck() {
	test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
	go vet ./...
	go test ./...
}

step() { printf '\n== %s\n' "$*"; }

baseline="$(sed -n 's/^baseline: *"\{0,1\}\([0-9a-f]*\)"\{0,1\} *$/\1/p' "$root/spec/requirements/areas.yaml")"
test -n "$baseline" || { echo "no baseline in spec/requirements/areas.yaml" >&2; exit 1; }

step "spec/tools: gofmt, vet, test"
cd "$root/spec/tools"
gocheck
spectool() { go run ./cmd/spectool "$@" -spec ..; }
step "spectool lint"; spectool lint
step "spectool render -check"; spectool render -check
step "spectool topic -check"; spectool topic -check
step "spectool coverage -check -gate"; spectool coverage -check -gate
step "spectool inventory -check -rev $baseline"; spectool inventory -check -rev "$baseline"

step "schema/check.sh"
"$root/schema/check.sh"

step "spectool crosscheck"
cd "$root/spec/tools"
spectool crosscheck

step "conformance: gofmt, vet, test"
cd "$root/conformance"
gocheck

step "sdk/check.sh (Rust SDK, C headers)"
"$root/sdk/check.sh"

step "dev: gofmt, vet, test (dev PKI, broker configuration)"
cd "$root/dev"
gocheck

step "demos/check.sh (demo services)"
"$root/demos/check.sh"

step "registry/check.sh (OEC Registry D5)"
"$root/registry/check.sh"

step "sdk/micro/check.sh (micro SDK D9)"
"$root/sdk/micro/check.sh"

printf '\nall checks passed\n'
