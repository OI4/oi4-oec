#!/bin/sh
# Opt-in live run of `oec-check monitor`, `oec-infra bus`, `oec-infra
# host` and `oec-infra container` against the Mosquitto of the dev
# environment (oec/dev, C-051; C-052; C-071; C-072; C-073). Not part
# of the gate oec/check.sh: it pulls the pinned image and needs a container
# engine.
#
#   oec/conformance/live.sh
#
# Steps: generate the dev PKI if missing (go run ./pki in oec/dev), start the
# broker (compose up --wait), run `go test ./cmd/oec-check -run
# TestMonitorDevBroker`, `go test ./cmd/oec-infra -run TestBusDevBroker` and
# `-run 'TestHostDev(Roots|Container)'` (the host run on this computer and in
# a Linux container in the network namespace of the broker) and `-run
# TestContainerDev` (two short-lived containers from the broker image, the
# broker itself and T8-HST-03; they are removed afterwards) with
# OEC_LIVE=1, stop the broker and remove its volume (compose down -v).
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

echo "== oec-infra host on the sample roots of oec/dev"
# shellcheck disable=SC2086
broker=$(cd "$dev" && $compose -f compose.yaml ps -q mosquitto)
OEC_LIVE=1 OEC_DEV_DIR="$dev" OEC_ENGINE="$engine" OEC_BROKER_CONTAINER="$broker" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestHostDev(Roots|Container)$' ./cmd/oec-infra

echo "== oec-infra container and T8-HST-03 on containers of the engine"
OEC_LIVE=1 OEC_DEV_DIR="$dev" OEC_ENGINE="$engine" OEC_BROKER_CONTAINER="$broker" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestContainerDev$' ./cmd/oec-infra

echo "live: all tests passed"
