#!/bin/sh
# The Podman run of images.yml (ADR-0032 §6, C-107): what of "Getting
# started" and the live D7/D8 run works with Podman on Linux, as the user
# of the runner (rootless). Runs after images.sh, getting-started.sh and
# live-infra.sh, which used Docker:
#
#   1. podman info; the DNS plugin of the network backend (containers of
#      compose find each other by name: dnsname for CNI, aardvark-dns for
#      netavark; installed with apt on a GitHub runner, else a hint); the
#      API socket of the user (podman system service);
#   2. load: the images of the demos and the registry from Docker
#      (docker save | podman load; the build.sh scripts need docker buildx);
#   3. d8-run: D8 oec-infra run --level 3 on the logger image through the
#      Podman socket (--daemon); --listen and --broker-host name an address
#      of the host, since the gateway of the rootless bridge is none;
#   4. getting-started: getting-started.sh with OEC_COMPOSE="podman compose";
#   5. live: live-infra.sh with OEC_COMPOSE="podman compose" and
#      OEC_LIVE_RUN_LISTEN (TestRunDev as in 3).
#
#   oec/ci/podman.sh [<report dir>]
#
# For 4 and 5 a wrapper "podman" first in PATH adds oec/dev/compose.podman.yaml
# after compose.yaml (Docker Compose refuses healthcheck.start_interval
# against Podman 4.x), and a ticker runs the health checks of starting
# containers (Podman schedules them with systemd timers; without a user
# systemd none run and compose --wait never ends).
#
# Every step runs; the table at the end says what works. A step listed in
# OEC_PODMAN_LIMITS (default: none) is a known limit: its failure is
# reported, not fatal; any other failing step fails the script. Logs go to
# <report dir>/podman-*.log. It removes the images it loaded into Podman
# and stops the API service and the ticker it started; never a prune.
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
# podman version --format differs between 4.x and 5.x; podman --version does not
echo "podman: $(podman --version)"
podman info --format 'podman: rootless {{.Host.Security.Rootless}}, network backend {{.Host.NetworkBackend}}, cgroups {{.Host.CgroupsVersion}}, storage {{.Store.GraphDriverName}}' || true
podman compose version 2>/dev/null | head -3 || true

backend="$(podman info --format '{{.Host.NetworkBackend}}')"
case $backend in
cni) dns=dnsname pkg=golang-github-containernetworking-plugin-dnsname dirs="/usr/lib/cni /opt/cni/bin /usr/libexec/cni" ;;
*) dns=aardvark-dns pkg=aardvark-dns dirs="/usr/libexec/podman /usr/lib/podman /usr/local/libexec/podman" ;;
esac
found() {
	for d in $dirs; do [ -x "$d/$dns" ] && return 0; done
	return 1
}
if ! found; then
	if [ "${GITHUB_ACTIONS:-}" = true ] && sudo -n true 2>/dev/null; then
		echo "podman: installing $pkg ($dns, network backend $backend)"
		sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends "$pkg" >/dev/null
	fi
	found || { echo "podman: no $dns for the network backend $backend (package $pkg): containers of compose cannot resolve each other" >&2; exit 1; }
fi
echo "podman: $dns ok (network backend $backend)"
# an address of the host for the MQTT test endpoint of D8 run
addr="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p')"
[ -n "$addr" ] || addr="$(hostname -I | awk '{print $1}')"
[ -n "$addr" ] || { echo "podman: no address of the host" >&2; exit 1; }
echo "podman: host address $addr"

sock="$(podman info --format '{{.Host.RemoteSocket.Path}}')"
sock="${sock#unix://}"
service=""
ticker=""
wrap="$(mktemp -d)"
cleanup() {
	[ -z "$ticker" ] || kill "$ticker" 2>/dev/null || true
	for i in $images; do podman image rm "$i" >/dev/null 2>&1 || true; done
	[ -z "$service" ] || kill "$service" 2>/dev/null || true
	rm -rf "$wrap"
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
		--daemon "unix://$sock" --listen "$addr" --broker-host "$addr" "oi4/oec-logger:$demos")
}
# the wrapper: podman compose with compose.podman.yaml after compose.yaml
# (or both when no -f is given); anything else goes to podman unchanged
real="$(command -v podman)"
cat >"$wrap/podman" <<EOF
#!/bin/sh
[ "\${1:-}" = compose ] || exec "$real" "\$@"
shift
seen=0
for a do
	shift
	set -- "\$@" "\$a"
	if [ "\$a" = compose.yaml ]; then set -- "\$@" -f "$oec/dev/compose.podman.yaml"; seen=1; fi
done
[ "\$seen" = 1 ] || set -- -f compose.yaml -f "$oec/dev/compose.podman.yaml" "\$@"
exec "$real" compose "\$@"
EOF
chmod +x "$wrap/podman"
(
	while :; do
		for c in $(podman ps -q --filter health=starting 2>/dev/null); do
			podman healthcheck run "$c" >/dev/null 2>&1 || true
		done
		sleep 2
	done
) &
ticker=$!
# load reads from the Docker engine: before DOCKER_HOST names the Podman socket
step load load
export DOCKER_HOST="unix://$sock" CONTAINER_HOST="unix://$sock"
step d8-run d8run
step getting-started env PATH="$wrap:$PATH" OEC_COMPOSE="podman compose" "$here/getting-started.sh"
step live env PATH="$wrap:$PATH" OEC_COMPOSE="podman compose" OEC_LIVE_RUN_LISTEN="$addr" "$here/live-infra.sh"

echo "== podman: results (known limits: ${limits:-none})"
printf '%s' "$results" | tee "$out/podman-results.txt"
[ "$fatal" = 0 ] || { echo "podman: a step outside the known limits failed" >&2; exit 1; }
echo "podman ok"
