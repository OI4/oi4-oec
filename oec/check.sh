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
#      static library, the latter under AddressSanitizer + UBSan) and the
#      D9 sketch oecm.h compiles as C17 (ADR-0022, from C-042);
#   6. dev: gofmt, vet, tests of the dev PKI generator and the lint of the
#      Mosquitto and compose configuration (C-051). No container engine: the
#      interop tests against the broker are opt-in (oec/dev/interop.sh).
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

printf '\nall checks passed\n'
