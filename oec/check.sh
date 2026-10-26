#!/bin/sh
# Local CI of the monorepo (ADR-0019 amendment C-041): the P2 gate.
#   1. spec/tools: gofmt, vet, tests and the spectool checks, including the
#      coverage gate (every schema/d7-validate REQ has a reviewed positive and
#      negative vector, every allowed matrix row has its entry schema);
#   2. schema/check.sh: bundle, release freeze, both validators;
#   3. spectool crosscheck: oracle vs schema on every case of schema/dist;
#   4. conformance: gofmt, vet, tests.
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

printf '\nall checks passed\n'
