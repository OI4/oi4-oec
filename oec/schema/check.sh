#!/bin/sh
# Local CI of oec/schema (ADR-0019): bundle + Go validator (santhosh-tekuri/jsonschema),
# then the second, independent validator (Rust jsonschema crate) on the same cases.
set -eu
cd "$(dirname "$0")/tools"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting" >&2; exit 1; }
GOTOOLCHAIN=auto go vet ./...
GOTOOLCHAIN=auto go run ./cmd/schematest -spec ../../spec -dist ../dist
cd rust-check
cargo run --release --quiet --locked -- ../../dist
