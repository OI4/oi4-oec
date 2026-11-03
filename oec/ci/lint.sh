#!/bin/sh
# actionlint on .github/workflows (ADR-0032 §7): a job of gate.yml, opt-in
# locally (network: go run fetches actionlint at the version of versions.sh
# into the Go module cache; it is a tool, not a dependency of a module).
# actionlint runs shellcheck on the run steps if shellcheck is installed.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/versions.sh"
export GOTOOLCHAIN=auto
cd "$here/../.."
go run "github.com/rhysd/actionlint/cmd/actionlint@v$actionlint_version" .github/workflows/*.yml
echo "actionlint $actionlint_version: .github/workflows ok"
