#!/bin/sh
# The ESP-IDF part of the micro SDK for micro.yml (ADR-0032 §6, C-106),
# opt-in locally (Docker, network): in the Espressif ESP-IDF container of
# versions.sh (pinned by digest, ADR-0030 §8) examples/esp-idf/build.sh
# builds the variants esp32 esp32s3 qemu (warnings of the oecm component are
# errors there), then D7 `oec-check test --level 1/2/3` runs against the
# example firmware in the Espressif QEMU of the image (cmd/oecm-qemu,
# TestQEMUD7Level1/2/3 with OECM_REQUIRE=1; a skipped test fails).
#
# The Linux tools are built on the host first: oec-check and the test binary
# of cmd/oecm-qemu with Go (CGO_ENABLED=0, for the architecture of the
# Docker host), go-testbroker with cargo on a Linux host of the same
# architecture (the image is Ubuntu 24.04 with glibc 2.39, as the runner),
# or OEC_TESTBROKER = a prebuilt Linux go-testbroker (e.g. on macOS). The
# workspace is mounted at its own path (the test binary finds oec/ there),
# the container runs with the uid of the caller and --init (it reaps the
# orphans of the D7 restart hooks). Behind a TLS-intercepting proxy,
# OEC_CI_CA_CERTS = a PEM file with its root certificate(s) for the
# download of esp-mqtt from the ESP Component Registry (never commit it).
# Output: oec/sdk/micro/target/esp-idf/<variant>/ (git-ignored, deleted
# and rebuilt; no artifact). Removes its container and the image if it pulled it.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
ws="$(cd "$here/../.." && pwd)"
micro="$ws/oec/sdk/micro"

if [ "${1:-}" = --in-container ]; then
	# In the container (the entrypoint of the image has run export.sh).
	tools="$2"
	mkdir -p "$HOME"
	git config --global --add safe.directory '*'
	if [ -f "$tools/extra-ca.pem" ]; then
		cat /etc/ssl/certs/ca-certificates.crt "$tools/extra-ca.pem" >"$HOME/ca.pem"
		export SSL_CERT_FILE="$HOME/ca.pem" REQUESTS_CA_BUNDLE="$HOME/ca.pem"
	fi
	echo "micro-esp-idf: $(idf.py --version), $(uname -m), $(qemu-system-xtensa --version | head -1)"
	# From scratch: a local build of another host would not configure here.
	rm -rf "$micro/target/esp-idf"
	"$micro/examples/esp-idf/build.sh" esp32 esp32s3 qemu
	echo "micro-esp-idf: D7 --level 1/2/3 against the firmware in QEMU (cmd/oecm-qemu)"
	cd "$micro/host/cmd/oecm-qemu"
	status=0
	OECM_REQUIRE=1 OECM_QEMU_BUILD="$micro/target/esp-idf/qemu" OECM_QEMU="$(command -v qemu-system-xtensa)" \
		OECM_QEMU_PYTHON="$(command -v python)" OEC_D7="$tools/oec-check" OEC_TESTBROKER="$tools/go-testbroker" \
		"$tools/oecm-qemu.test" -test.run '^TestQEMUD7Level[123]$' -test.v -test.count=1 -test.timeout 60m \
		>"$HOME/d7.log" 2>&1 || status=$?
	cat "$HOME/d7.log"
	test "$status" = 0 || { echo "micro-esp-idf: QEMU D7 failed" >&2; exit 1; }
	for t in TestQEMUD7Level1 TestQEMUD7Level2 TestQEMUD7Level3; do
		grep -q "^--- PASS: $t " "$HOME/d7.log" || { echo "micro-esp-idf: $t did not pass" >&2; exit 1; }
	done
	echo "micro-esp-idf: ok"
	exit 0
fi

test $# -eq 0 || { echo "usage: micro-esp-idf.sh" >&2; exit 2; }
. "$here/versions.sh"
export GOTOOLCHAIN=auto
command -v docker >/dev/null || { echo "micro-esp-idf: no docker" >&2; exit 1; }
name="oec-ci-esp-idf"
pulled=0
docker image inspect "$espressif_idf_image" >/dev/null 2>&1 || pulled=1
tools="$(mktemp -d)"
cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	if [ "$pulled" = 1 ]; then docker image rm "$espressif_idf_image" >/dev/null 2>&1 || true; fi
	rm -rf "$tools"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

case "$(docker info --format '{{.Architecture}}')" in
x86_64 | amd64) goarch=amd64 machine=x86_64 ;;
aarch64 | arm64) goarch=arm64 machine=aarch64 ;;
*) echo "micro-esp-idf: unsupported Docker architecture" >&2; exit 1 ;;
esac
echo "micro-esp-idf: tools for linux/$goarch"
(cd "$ws/oec/conformance" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -o "$tools/oec-check" ./cmd/oec-check)
(cd "$micro/host" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -o "$tools/oecm-qemu.test" ./cmd/oecm-qemu)
if [ -n "${OEC_TESTBROKER:-}" ]; then
	cp "$OEC_TESTBROKER" "$tools/go-testbroker"
elif [ "$(uname -s)" = Linux ] && [ "$(uname -m)" = "$machine" ]; then
	rust="$ws/oec/sdk/rust"
	(cd "$rust" && cargo build --locked --quiet -p oec-ffi --example go-testbroker)
	cp "${CARGO_TARGET_DIR:-$rust/target}/debug/examples/go-testbroker" "$tools/"
else
	echo "micro-esp-idf: set OEC_TESTBROKER to a go-testbroker built for linux/$goarch" >&2
	exit 1
fi
if [ -n "${OEC_CI_CA_CERTS:-}" ]; then
	cp "$OEC_CI_CA_CERTS" "$tools/extra-ca.pem"
fi
chmod -R a+rX "$tools"

docker run --name "$name" --rm --init \
	-v "$ws:$ws" -v "$tools:$tools:ro" -w "$ws" \
	--user "$(id -u):$(id -g)" -e HOME=/tmp/oec-home \
	"$espressif_idf_image" sh "$here/micro-esp-idf.sh" --in-container "$tools"
