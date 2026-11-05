#!/bin/sh
# The newcomer run of images.yml (ADR-0032 §6, C-107): walks "Getting
# started" of the Community pages (oi4-oec-documentation/docs/community/
# index.md, "See OEC at work") on a fresh Linux host and checks what a
# newcomer sees, with curl and jq instead of a browser:
#
#   1. go run ./pki in oec/dev (only if generated/ is missing, as
#      live.sh), then compose up of the profiles demos and registry with
#      --env-file generated/demos.env and --wait;
#   2. the Web UIs of the logger (8781), weather (8782) and config (8783)
#      service and of the registry (https 5798, development CA) answer;
#   3. the registry lists the three demos and the three stations of the
#      weather service, all available (not itself: it lists the others);
#   4. the story step: the config service sets the stations of the weather
#      service from 3 to 4 (POST /api/set, Status good) and the registry
#      lists station 4.
#
#   oec/ci/getting-started.sh
#
# Needs the images oi4/oec-{logger,weather,config}:$OEC_DEMOS_VERSION and
# oi4/oec-registry:$OEC_REGISTRY_VERSION (default 0.1.0; images.sh or the
# build.sh scripts), Go, curl and jq. Compose down -v always runs at the end
# (OEC_GS_KEEP=1 keeps the stack). OEC_COMPOSE as in live.sh (default
# "docker compose", e.g. "podman compose"). It runs in the compose project
# oec-dev and publishes the ports of oec/dev on 127.0.0.1: not next to a
# running oec/dev stack.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
dev="$(cd "$here/../dev" && pwd)"
compose=${OEC_COMPOSE:-docker compose}
engine=${compose%% *}
case $engine in
*-compose) engine=${engine%-compose} ;;
esac
for t in "$engine" go curl jq; do
	command -v "$t" >/dev/null 2>&1 || { echo "getting-started: $t not found" >&2; exit 1; }
done
"$engine" info >/dev/null 2>&1 || { echo "getting-started: no running container engine ($engine)" >&2; exit 1; }
timeout="${OEC_GS_TIMEOUT:-180}"

cd "$dev"
if [ -f generated/.oec-dev-pki ]; then
	echo "== go run ./pki: generated/ exists, reused (as live.sh)"
else
	echo "== go run ./pki"
	GOTOOLCHAIN=${GOTOOLCHAIN:-auto} go run ./pki
fi
# shellcheck disable=SC1091
. ./generated/demos.env
profiles="--env-file generated/demos.env --profile demos --profile registry"
ok=0
cleanup() {
	if [ "$ok" = 0 ]; then
		echo "== getting-started failed: state and logs" >&2
		# shellcheck disable=SC2086
		$compose $profiles ps -a >&2 || true
		# shellcheck disable=SC2086
		$compose $profiles logs --no-color --tail 40 >&2 || true
	fi
	if [ "${OEC_GS_KEEP:-}" = 1 ]; then
		echo "getting-started: the stack keeps running (OEC_GS_KEEP=1)"
	else
		# shellcheck disable=SC2086
		$compose $profiles down -v >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

echo "== compose up (profiles demos and registry)"
start=$(date +%s)
# shellcheck disable=SC2086
$compose $profiles up -d --wait
echo "up in $(($(date +%s) - start)) s"

echo "== the Web UIs"
for p in 8781 8782 8783; do
	curl -fsS -o /dev/null "http://127.0.0.1:$p/"
	echo "http://127.0.0.1:$p/ ok"
done
curl -fsS -o /dev/null --cacert generated/broker/ca.pem https://localhost:5798/
echo "https://localhost:5798/ ok (development CA)"

assets() { curl -fsS --cacert generated/broker/ca.pem https://localhost:5799/api/assets; }
# poll <what> <command…>: repeats the command every 2 s until it succeeds
poll() {
	what="$1"
	shift
	n=0
	until "$@" >/dev/null 2>&1; do
		n=$((n + 2))
		if [ "$n" -ge "$timeout" ]; then
			echo "getting-started: $what: not within $timeout s" >&2
			return 1
		fi
		sleep 2
	done
	echo "$what: after $n s"
}
# listed <serial…>: the registry lists an available asset (online, MAM and
# Health known) for every serial number (the last element of its IL)
listed() {
	assets | jq -e --args '[.assets[] | select(.available) | .il | split("/") | last] as $s
		| ($ARGS.positional - $s) == []' "$@"
}

echo "== the registry lists the three demos and the stations"
poll "registry: logger, weather, config and stations 1 … 3 available" \
	listed "$OEC_LOGGER_NAME" "$OEC_WEATHER_NAME" "$OEC_CONFIG_NAME" \
	"$OEC_WEATHER_NAME-station-1" "$OEC_WEATHER_NAME-station-2" "$OEC_WEATHER_NAME-station-3" ||
	{ assets | jq -r '.assets[] | "  \(.availability) \(.il)"' >&2; exit 1; }
assets | jq -r '.assets[] | "  \(.availability) \(.kind) \(.il)"'

echo "== story: the config service sets the stations of the weather service to 4"
weather="oi4.org/OEC,20Weather/OEC-WEATHER/$OEC_WEATHER_NAME"
apps() { curl -fsS http://127.0.0.1:8783/api/apps; }
# the config service discovers the weather service and its Config DataSets
poll "config service: the weather service discovered" \
	sh -c 'curl -fsS http://127.0.0.1:8783/api/apps | jq -e --arg w "$1" "[.apps[] | select(.app == \$w) | .configs[]] | length > 0"' _ "$weather" ||
	{ apps >&2; exit 1; }
body="$(jq -cn --arg w "$weather" '{serviceType: "OTConnector", app: $w, source: $w, context: "weather",
	values: {common: {stations: "4"}}}')"
result="$(curl -fsS -X POST -H 'Content-Type: application/json' --data "$body" http://127.0.0.1:8783/api/set)"
echo "$result" | jq -c '{status, error}'
echo "$result" | jq -e '.status.good == true' >/dev/null || { echo "getting-started: Set not good" >&2; exit 1; }
poll "registry: station 4 of the weather service listed" listed "$OEC_WEATHER_NAME-station-4" ||
	{ assets | jq -r '.assets[] | "  \(.availability) \(.il)"' >&2; exit 1; }
ok=1
echo "getting-started ok"
