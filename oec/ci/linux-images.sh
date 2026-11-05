#!/bin/sh
# Opt-in (Docker, network, about 30 to 60 minutes): runs the job "images" of
# images.yml (ADR-0032 §6 and §8, C-107) on a fresh Ubuntu 24.04
# (versions.sh; the architecture of the Docker host) against the committed
# state – the newcomer path of "Getting started" with the packages of the
# distribution. A privileged container with a Docker engine of its own
# (docker.io, docker-buildx, docker-compose-v2 of Ubuntu; its storage in an
# anonymous volume of the container) clones HEAD of this repository,
# installs Go of the distribution (GOTOOLCHAIN=auto fetches the toolchain of
# go.mod), curl and jq, and runs as the non-root user ci (group docker)
# oec/ci/images.sh, getting-started.sh and live-infra.sh. With
# OEC_CI_PODMAN=1 it then runs oec/ci/podman.sh (podman of Ubuntu, rootless
# as the user ci, with the DNS plugin dnsname of its CNI backend; storage
# driver vfs on an ext4 volume: in this nested container – Docker Desktop
# and its VM – every rootless container of the overlay driver failed with
# "exec …: Invalid argument", C-107; a runner needs none of this). Nothing reaches the Docker engine of the host except the
# container itself: removed on exit with its volume, and the Ubuntu image if
# the script pulled it. Never part of the gate.
#
#   oec/ci/linux-images.sh [<report dir>]   (D8 reports and logs copied there)
#
# OEC_CI_CA_CERTS and OEC_CI_CPUS as in linux.sh.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/versions.sh"
ws="$(cd "$here/../.." && pwd)"
reports="${1:-}"
name="oec-ci-images"
pulled=0
docker image inspect "$ubuntu_image" >/dev/null 2>&1 || pulled=1
cleanup() {
	docker rm -f -v "$name" >/dev/null 2>&1 || true
	if [ "$pulled" = 1 ]; then docker image rm "$ubuntu_image" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

ca_mount=""
if [ -n "${OEC_CI_CA_CERTS:-}" ]; then
	test -f "$OEC_CI_CA_CERTS" || { echo "linux-images.sh: OEC_CI_CA_CERTS: no file $OEC_CI_CA_CERTS" >&2; exit 1; }
	ca_mount="-v $(cd "$(dirname "$OEC_CI_CA_CERTS")" && pwd)/$(basename "$OEC_CI_CA_CERTS"):/oec-extra-ca.pem:ro"
fi
out_mount=""
if [ -n "$reports" ]; then
	mkdir -p "$reports"
	out_mount="-v $(cd "$reports" && pwd):/reports"
fi

# The steps of the job images of images.yml, as the user ci.
job='set -eu
git config --global --add safe.directory "*"
git clone --quiet /src "$HOME/w"
cd "$HOME/w"
echo "linux-images.sh: workspace $(git rev-parse --short HEAD), $(uname -m)"
export GOTOOLCHAIN=auto
rc=0
oec/ci/images.sh dist/images || rc=$?
if [ "$rc" = 0 ]; then
	oec/ci/getting-started.sh || rc=$?
fi
if [ "$rc" = 0 ]; then
	oec/ci/live-infra.sh || rc=$?
fi
if [ "$rc" = 0 ] && [ "$PODMAN" = 1 ]; then
	oec/ci/podman.sh dist/images || rc=$?
fi
if [ -d /reports ]; then cp -R dist/images/. /reports/; fi
exit "$rc"'

# shellcheck disable=SC2086 # ca_mount and out_mount are empty or one -v option
docker run --name "$name" --privileged --cpus "${OEC_CI_CPUS:-4}" \
	-v /var/lib/docker -v /var/lib/containers \
	-v "$ws:/src:ro" $ca_mount $out_mount \
	-e JOB="$job" -e PODMAN="${OEC_CI_PODMAN:-0}" \
	"$ubuntu_image" bash -euc '
if [ -f /oec-extra-ca.pem ]; then
	apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ca-certificates >/dev/null
	cp /oec-extra-ca.pem /usr/local/share/ca-certificates/oec-extra-ca.crt
	update-ca-certificates >/dev/null
fi
apt-get update -qq
pkgs="ca-certificates curl git jq golang-go docker.io docker-buildx docker-compose-v2 iproute2"
[ "$PODMAN" = 1 ] && pkgs="$pkgs podman uidmap slirp4netns passt fuse-overlayfs dbus-user-session golang-github-containernetworking-plugin-dnsname"
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends $pkgs >/dev/null
echo "linux-images.sh: $(docker --version), $(docker buildx version), $(docker compose version), $(go version)"
dockerd >/var/log/dockerd.log 2>&1 &
n=0
until docker info >/dev/null 2>&1; do
	n=$((n + 1)); [ "$n" -lt 60 ] || { tail -20 /var/log/dockerd.log; exit 1; }
	sleep 1
done
useradd --create-home --groups docker ci
if [ "$PODMAN" = 1 ]; then
	mkdir -p /home/ci/.config/containers /var/lib/containers/ci
	printf "[storage]\ndriver = \"vfs\"\ngraphroot = \"/var/lib/containers/ci\"\n" >/home/ci/.config/containers/storage.conf
	chown -R ci:ci /home/ci/.config /var/lib/containers/ci
fi
printf "%s\n" "$JOB" >/tmp/job.sh
runuser -u ci -- env HOME=/home/ci USER=ci PODMAN="$PODMAN" sh /tmp/job.sh'
