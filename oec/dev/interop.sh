#!/bin/sh
# Opt-in interoperability tests of the Rust SDK against the Mosquitto of the
# dev environment (oec/dev, C-051, ADR-0026 amendment C-051). Not part of the
# gate oec/check.sh: this script pulls the pinned image and needs a
# container engine.
#
#   oec/dev/interop.sh
#
# Steps: generate the dev PKI if missing (go run ./pki), start the broker
# (compose up --wait), run `cargo test -p oec --test interop` with
# OEC_INTEROP=1, stop the broker and remove its volume (compose down -v).
#
# Environment:
#   OEC_COMPOSE        compose command (default "docker compose"; e.g.
#                      "podman compose" or "podman-compose")
#   OEC_INTEROP_KEEP=1 leave the broker running afterwards
#
# Without a running container engine the script prints a note and exits 0.
set -eu

dev=$(cd "$(dirname "$0")" && pwd)
compose=${OEC_COMPOSE:-docker compose}
engine=${compose%% *}
case $engine in
    *-compose) engine=${engine%-compose} ;;
esac

if ! command -v "$engine" >/dev/null 2>&1 || ! "$engine" info >/dev/null 2>&1; then
    echo "interop: no running container engine ($engine): skipped"
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
if [ "${OEC_INTEROP_KEEP:-}" != 1 ]; then
    # shellcheck disable=SC2086
    trap '$compose -f "$dev/compose.yaml" down -v >/dev/null 2>&1 || true' EXIT
fi

echo "== interop tests"
cd "$dev/../sdk/rust"
OEC_INTEROP=1 OEC_DEV_DIR="$dev" OEC_COMPOSE="$compose" \
    cargo test --locked -p oec --test interop -- --test-threads=1

echo "interop: all tests passed"
