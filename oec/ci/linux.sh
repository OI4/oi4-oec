#!/bin/sh
# Opt-in (Docker, network, about 1 to 2 hours): runs the job "gate" of
# gate.yml in an Ubuntu 24.04 container (versions.sh; the architecture of
# the Docker host) against the committed state: clones HEAD of this
# repository and the branch guideline_ref of ../../oi4-oec-documentation,
# installs the system packages (packages.sh --go; GOTOOLCHAIN=auto fetches
# the Go toolchain of go.mod) and rustup, then runs setup-tools.sh, fetch.sh and
# oec/check.sh as a non-root user with CC=clang, as on the runner
# (ADR-0032 §8). Removes its container on exit and the Ubuntu image if it
# pulled it. Never part of the gate.
#
# Behind a TLS-intercepting proxy, set OEC_CI_CA_CERTS to a PEM file with
# the proxy's root certificate(s); it is mounted read-only and added to the
# container's trust store (never commit such a file).
#
# OEC_CI_CPUS (default 4, the vCPUs of a hosted Linux runner) limits the
# container; cargo and Go follow the quota, the links of the Rust tests stay
# within the memory of a small Docker VM, and the run time estimates the
# runner's.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/versions.sh"
ws="$(cd "$here/../.." && pwd)"
test -d "$ws/oi4-oec-documentation/.git" || { echo "linux.sh: no guideline repository at $ws/oi4-oec-documentation" >&2; exit 1; }
name="oec-ci-linux"
pulled=0
docker image inspect "$ubuntu_image" >/dev/null 2>&1 || pulled=1
cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	if [ "$pulled" = 1 ]; then docker image rm "$ubuntu_image" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

ca_mount=""
if [ -n "${OEC_CI_CA_CERTS:-}" ]; then
	test -f "$OEC_CI_CA_CERTS" || { echo "linux.sh: OEC_CI_CA_CERTS: no file $OEC_CI_CA_CERTS" >&2; exit 1; }
	ca_mount="-v $(cd "$(dirname "$OEC_CI_CA_CERTS")" && pwd)/$(basename "$OEC_CI_CA_CERTS"):/oec-extra-ca.pem:ro"
fi

# The steps of gate.yml, as the user ci (not root: file permission checks
# behave as on the runner).
job='set -eu
git config --global --add safe.directory "*"
git clone --quiet /src "$HOME/w"
git clone --quiet --branch "$GUIDELINE_REF" /src/oi4-oec-documentation "$HOME/w/oi4-oec-documentation"
cd "$HOME/w"
echo "linux.sh: workspace $(git rev-parse --short HEAD), guideline $(git -C oi4-oec-documentation rev-parse --short HEAD), $(uname -m)"
export GOTOOLCHAIN=auto CARGO_TERM_COLOR=never CC=clang
export PATH="$HOME/.oec-tools/bin:$HOME/.cargo/bin:$PATH"
oec/ci/setup-tools.sh
oec/ci/fetch.sh
oec/check.sh'

# shellcheck disable=SC2086 # ca_mount is empty or one -v option
# --init: an init process reaps the orphans of the restart hooks (D7 test
# --start-cmd starts the application in the background), as systemd does on
# the runner; without it a stopped application stays a zombie and the
# kill -0 loop of the stop hook never ends.
docker run --name "$name" --rm --init --cpus "${OEC_CI_CPUS:-4}" \
	-v "$ws:/src:ro" $ca_mount \
	-e GUIDELINE_REF="$guideline_ref" -e JOB="$job" \
	"$ubuntu_image" bash -euc '
sh /src/oec/ci/packages.sh --go
if [ -f /oec-extra-ca.pem ]; then
	cp /oec-extra-ca.pem /usr/local/share/ca-certificates/oec-extra-ca.crt
	update-ca-certificates >/dev/null
	echo "linux.sh: extra CA certificates from OEC_CI_CA_CERTS added"
fi
useradd --create-home ci
printf "%s\n" "$JOB" >/tmp/job.sh
runuser -u ci -- env HOME=/home/ci USER=ci GUIDELINE_REF="$GUIDELINE_REF" sh /tmp/job.sh'
