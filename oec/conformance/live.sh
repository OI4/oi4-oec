#!/bin/sh
# Opt-in live run of `oec-check monitor`, `oec-infra bus`, `oec-infra
# host`, `oec-infra container` and `oec-infra run` against the Mosquitto of
# the dev environment and the container engine (oec/dev, C-051; C-052;
# C-071; C-072; C-073; C-074; C-090). Not part
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
# broker itself and T8-HST-03; they are removed afterwards) and `-run
# TestRunDev` (loads the test images oec-d8-sim:<fault> of the simulated
# application, runs every fault in oec-d8-run-* sandbox containers and
# removes the images by tag; never a prune) with OEC_LIVE=1, stop the
# broker and remove its volume (compose down -v). Containers of the live
# tests are removed with their anonymous volumes (rm -v, C-075).
#
# With OEC_LIVE_REGISTRY=1 (C-090, A-418) the OEC Registry of the compose
# profile registry starts after the monitor run (its Gets would change the
# recording) from the local image oi4/oec-registry (oec/registry/images/
# build.sh; never pulled): `oec-infra bus` discovers it and T8-BUS-03
# passes, `oec-infra host` finds it (REQ-PLT-008). compose down stops it.
#
# Environment:
#   OEC_COMPOSE    compose command (default "docker compose"; e.g.
#                  "podman compose" or "podman-compose")
#   OEC_LIVE_KEEP=1  leave the broker running afterwards
#   OEC_LIVE_REGISTRY=1  start the OEC Registry too (profile registry)
#   OEC_LIVE_OUT   directory for the recorded captures (default: a temporary
#                  directory of the test)
#   OEC_LIVE_RUN_LISTEN  --listen and --broker-host of TestRunDev (C-107;
#                  rootless Podman: an address of the host, oec/ci/podman.sh)
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

profile=
if [ "${OEC_LIVE_REGISTRY:-}" = 1 ]; then
    image=oi4/oec-registry:${OEC_REGISTRY_VERSION:-0.1.0}
    if ! "$engine" image inspect "$image" >/dev/null 2>&1; then
        echo "live: OEC_LIVE_REGISTRY=1 needs the local image $image (oec/registry/images/build.sh)" >&2
        exit 1
    fi
    if [ ! -d generated/roots/oec-registry ] || ! grep -q '^OEC_REGISTRY_NAME=' generated/demos.env; then
        echo "live: generated/ has no storage of the registry: go run ./pki -force (in oec/dev)" >&2
        exit 1
    fi
    profile="--env-file generated/demos.env --profile registry"
fi

echo "== broker ($compose)"
# shellcheck disable=SC2086 # $compose may be two words
$compose -f compose.yaml up -d --wait
if [ "${OEC_LIVE_KEEP:-}" != 1 ]; then
    # shellcheck disable=SC2086
    trap 'cd "$dev" && $compose -f compose.yaml $profile down -v >/dev/null 2>&1 || true' EXIT
fi

echo "== oec-check monitor against the dev broker"
cd "$here"
OEC_LIVE=1 OEC_DEV_DIR="$dev" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestMonitorDevBroker$' ./cmd/oec-check

if [ -n "$profile" ]; then
    echo "== OEC Registry (compose profile registry)"
    # shellcheck disable=SC2086
    (cd "$dev" && $compose -f compose.yaml $profile up -d --wait)
fi

echo "== oec-infra bus against the dev broker"
OEC_LIVE=1 OEC_LIVE_REGISTRY="${OEC_LIVE_REGISTRY:-}" OEC_DEV_DIR="$dev" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestBusDevBroker$' ./cmd/oec-infra

echo "== oec-infra host on the sample roots of oec/dev"
# shellcheck disable=SC2086
broker=$(cd "$dev" && $compose -f compose.yaml ps -q mosquitto)
OEC_LIVE=1 OEC_LIVE_REGISTRY="${OEC_LIVE_REGISTRY:-}" OEC_DEV_DIR="$dev" OEC_ENGINE="$engine" OEC_BROKER_CONTAINER="$broker" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestHostDev(Roots|Container)$' ./cmd/oec-infra

echo "== oec-infra container and T8-HST-03 on containers of the engine"
OEC_LIVE=1 OEC_DEV_DIR="$dev" OEC_ENGINE="$engine" OEC_BROKER_CONTAINER="$broker" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -run '^TestContainerDev$' ./cmd/oec-infra

echo "== oec-infra run with the test images of the simulated application"
OEC_LIVE=1 OEC_ENGINE="$engine" GOTOOLCHAIN=${GOTOOLCHAIN:-auto} \
    go test -count=1 -v -timeout 20m -run '^TestRunDev$' ./cmd/oec-infra

echo "live: all tests passed"
