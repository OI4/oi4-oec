#!/bin/sh
# The Go checks of D7 oec-check and D8 oec-infra as in gate step 4 (gofmt,
# vet, tests of oec/conformance) for conformance.yml on the Linux, macOS and
# Windows runners (ADR-0032 §6; on Windows in Git Bash).
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
export GOTOOLCHAIN=auto
cd "$here/../conformance"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
go vet ./...
go test ./...
