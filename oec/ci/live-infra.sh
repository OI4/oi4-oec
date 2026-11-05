#!/bin/sh
# The live run of images.yml (ADR-0032 §6, C-107; ADR-0025 amendment C-074
# item 7): oec/conformance/live.sh with the OEC Registry of the compose
# profile registry (OEC_LIVE_REGISTRY=1) – D7 `oec-check monitor`, D8
# `oec-infra bus`, `host`, `container` and `run` (TestRunDev: every fault
# of the simulated application in sandbox containers) against the broker of
# oec/dev and the container engine of a Linux host.
#
#   oec/ci/live-infra.sh
#
# Unlike live.sh it fails without a running container engine (live.sh then
# skips), and it fails without the image oi4/oec-registry:$OEC_REGISTRY_VERSION
# (default 0.1.0; images.sh). OEC_COMPOSE as in live.sh.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
compose=${OEC_COMPOSE:-docker compose}
engine=${compose%% *}
case $engine in
*-compose) engine=${engine%-compose} ;;
esac
"$engine" info >/dev/null 2>&1 || { echo "live-infra: no running container engine ($engine)" >&2; exit 1; }
registry="oi4/oec-registry:${OEC_REGISTRY_VERSION:-0.1.0}"
"$engine" image inspect "$registry" >/dev/null 2>&1 ||
	{ echo "live-infra: no image $registry (oec/ci/images.sh)" >&2; exit 1; }
OEC_LIVE_REGISTRY=1 "$oec/conformance/live.sh"
