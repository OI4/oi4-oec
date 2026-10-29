#!/bin/sh
# Opt-in live run of `oec-check monitor` and `oec-infra bus` against the
# Mosquitto of the dev environment (oec/dev, C-051; C-052; C-071). Not part
# of the gate oec/check.sh: it pulls the pinned image and needs a container
# engine.
#
#   oec/conformance/live.sh
#
# Steps: generate the dev PKI if missing (go run ./pki in oec/dev), start the
# broker (compose up --wait), run `go test ./cmd/oec-check -run
# TestMonitorDevBroker` and `go test ./cmd/oec-infra -run TestBusDevBroker`
# with OEC_LIVE=1, stop the broker and remove its volume (compose down -v).
#
# Environment:
#   OEC_COMPOSE    compose command (default "docker compose"; e.g.
#                  "podman compose" or "podman-compose")
#   OEC_LIVE_KEEP=1  leave the broker running afterwards
#   OEC_LIVE_OUT   directory for the recorded captures (default: a temporary
#                  directory of the test)
#
# Without a running container engine the script prints a note and exits 0.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
dev=$(cd "$here/../dev" && pwd)
compose=${OEC_COMPOSE:-docker compose}
engine=${compose%% *}
case $engine in
    *-compose) engine=${engine%-compose} ;;
esac

if ! command -v "$engine" >/dev/null 2>&1 || ! "$engine" info >/dev/null 2>&1; then
    echo "live: no running container engine ($engine): skipped"
    exit 0
fi

cd "$dev"
if [ ! -f generated/.oec-dev-pki ]; then
    echo "== dev PKI"
    GOTOOLCHAIN=${GOTOOLCHAIN:-auto} go run ./pki
fi

echo "== broker ($compose)"
# shellcheck disable=SC2086 # $compose may be two words
$compose -f compose.yaml up -d --wait
if [ "${OEC_LIVE_KEEP:-}" != 1 ]; then
    # shellcheck disable=SC2086
    trap '$compose -f "$dev/compose.yaml" down -v >/dev/null 2>&1 || true' EXIT
fi

echo "== oec-check monitor against the dev broker"
cd "$here"
OEC_LIVE=1 OEC_DEV_DIR="$dev" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestMonitorDevBroker$' ./cmd/oec-check

echo "== oec-infra bus against the dev broker"
OEC_LIVE=1 OEC_DEV_DIR="$dev" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestBusDevBroker$' ./cmd/oec-infra

echo "live: all tests passed"
