#!/bin/sh
# The Podman run of images.yml (ADR-0032 §6, C-107): what of "Getting
# started" and the live D7/D8 run works with Podman on Linux, as the user
# of the runner (rootless). Runs after images.sh, getting-started.sh and
# live-infra.sh, which used Docker:
#
#   1. podman info; the API socket of the user (podman system service);
#   2. load: the images of the demos and the registry from Docker
#      (docker save | podman load; the build.sh scripts need docker buildx);
#   3. d8-run: D8 oec-infra run --level 3 on the logger image through the
#      Podman socket (--daemon);
#   4. getting-started: getting-started.sh with OEC_COMPOSE="podman compose";
#   5. live: live-infra.sh with OEC_COMPOSE="podman compose".
#
#   oec/ci/podman.sh [<report dir>]
#
# Every step runs; the table at the end says what works. A step listed in
# OEC_PODMAN_LIMITS (default: the known limits of the last hosted run, see
# oec/ci/README.md) is a known limit: its failure is reported, not fatal;
# any other failing step fails the script. Logs go to <report dir>/podman-*.log.
# It removes the images it loaded into Podman and stops the API service it
# started; never a prune.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
out="${1:-dist/images}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
limits="${OEC_PODMAN_LIMITS-}"
demos="${OEC_DEMOS_VERSION:-0.1.0}"
registry="${OEC_REGISTRY_VERSION:-0.1.0}"
images="oi4/oec-logger:$demos oi4/oec-weather:$demos oi4/oec-config:$demos oi4/oec-registry:$registry"
for t in podman docker go; do
	command -v "$t" >/dev/null 2>&1 || { echo "podman: $t not found" >&2; exit 1; }
done
podman version --format 'podman: client {{.Client.Version}}, server {{.Server.Version}}'
podman info --format 'podman: rootless {{.Host.Security.Rootless}}, network backend {{.Host.NetworkBackend}}, cgroups {{.Host.CgroupsVersion}}, storage {{.Store.GraphDriverName}}'
podman compose version 2>/dev/null | head -3 || true

sock="$(podman info --format '{{.Host.RemoteSocket.Path}}')"
sock="${sock#unix://}"
service=""
cleanup() {
	for i in $images; do podman image rm "$i" >/dev/null 2>&1 || true; done
	[ -z "$service" ] || kill "$service" 2>/dev/null || true
}
trap cleanup EXIT
if ! curl -fsS --unix-socket "$sock" http://d/_ping >/dev/null 2>&1; then
	mkdir -p "$(dirname "$sock")"
	podman system service --time=0 "unix://$sock" >"$out/podman-service.log" 2>&1 &
	service=$!
	n=0
	until curl -fsS --unix-socket "$sock" http://d/_ping >/dev/null 2>&1; do
		n=$((n + 1))
		[ "$n" -lt 30 ] || { echo "podman: no API socket at $sock" >&2; exit 1; }
		sleep 1
	done
fi
echo "podman: API socket $sock"
export DOCKER_HOST="unix://$sock" CONTAINER_HOST="unix://$sock"

results=""
fatal=0
# step <name> <command…>: runs a step, logs to podman-<name>.log
step() {
	name="$1"
	shift
	echo "== podman: $name"
	start=$(date +%s)
	rc=0
	"$@" >"$out/podman-$name.log" 2>&1 || rc=$?
	secs=$(($(date +%s) - start))
	case " $limits " in
	*" $name "*) known=1 ;;
	*) known=0 ;;
	esac
	if [ "$rc" = 0 ]; then
		verdict="works"
		[ "$known" = 0 ] || verdict="works (listed as a limit: update OEC_PODMAN_LIMITS)"
	elif [ "$known" = 1 ]; then
		verdict="known limit (exit $rc)"
	else
		verdict="FAILS (exit $rc)"
		fatal=1
	fi
	tail -15 "$out/podman-$name.log"
	results="$results$(printf '%-16s %-6s %s' "$name" "${secs}s" "$verdict")
"
}

load() {
	# docker save | podman load: Docker loads only the native platform
	# shellcheck disable=SC2086
	docker save $images | podman load
	for i in $images; do podman image inspect --format '{{.Id}} {{.RepoTags}}' "$i"; done
}
d8run() {
	(cd "$oec/conformance" && GOTOOLCHAIN=${GOTOOLCHAIN:-auto} go run ./cmd/oec-infra run --level 3 \
		--daemon "unix://$sock" "oi4/oec-logger:$demos")
}
step load load
step d8-run d8run
step getting-started env OEC_COMPOSE="podman compose" "$here/getting-started.sh"
step live env OEC_COMPOSE="podman compose" "$here/live-infra.sh"

echo "== podman: results (known limits: ${limits:-none})"
printf '%s' "$results" | tee "$out/podman-results.txt"
[ "$fatal" = 0 ] || { echo "podman: a step outside the known limits failed" >&2; exit 1; }
echo "podman ok"
