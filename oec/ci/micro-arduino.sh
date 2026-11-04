#!/bin/sh
# The Arduino library of the micro SDK on one core for micro.yml (ADR-0032
# §6, C-106), opt-in locally (network, several GB of disk):
#
#   micro-arduino.sh [3.3|4.0]   (default: OECM_ARDUINO_CORE)
#
# arduino-cli of versions.sh (the release archive, SHA-256 checked) with an
# own configuration below OECM_ARDUINO_HOME (default ~/.oec-arduino: data,
# downloads, sketchbook; never the default locations), the Arduino-ESP32
# core of versions.sh (3.3: the newest 3.3.x; 4.0: 4.0.0, or its newest
# release candidate from the development index until 4.0.0 is released),
# M5Unified and M5GFX for the M5Stack example. The library comes from
# arduino/package.sh; its examples compile with --warnings all, and a
# warning in the library or a sketch fails: Sensor on ESP32 and ESP32-S3,
# M5Stack on M5Stack Core2 and CoreS3; Sensor on ESP32-C3 and ESP32-C6 is
# best effort (a failure is reported, it does not fail the run). The
# downloads of the core are deleted after the install (disk); no artifact.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/versions.sh"
micro="$(cd "$here/../sdk/micro" && pwd)"
core="${1:-${OECM_ARDUINO_CORE:-}}"
case "$core" in
3.3) version="$arduino_esp32_3_3" ;;
4.0) version="$arduino_esp32_4_0" ;;
*) echo "usage: micro-arduino.sh 3.3|4.0 (or OECM_ARDUINO_CORE)" >&2; exit 2 ;;
esac
case "$(uname -s)/$(uname -m)" in
Linux/x86_64) platform=Linux_64bit sum="$arduino_cli_sha256_linux_amd64" ;;
Linux/aarch64 | Linux/arm64) platform=Linux_ARM64 sum="$arduino_cli_sha256_linux_arm64" ;;
Darwin/arm64) platform=macOS_ARM64 sum="$arduino_cli_sha256_darwin_arm64" ;;
*) echo "micro-arduino: no pinned arduino-cli for $(uname -s)/$(uname -m)" >&2; exit 1 ;;
esac
home="${OECM_ARDUINO_HOME:-$HOME/.oec-arduino}"
mkdir -p "$home"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cli="$home/arduino-cli-$arduino_cli_version/arduino-cli"
if [ ! -x "$cli" ]; then
	archive="arduino-cli_${arduino_cli_version}_$platform.tar.gz"
	echo "micro-arduino: download $archive"
	curl -fsSL -o "$work/$archive" "https://github.com/arduino/arduino-cli/releases/download/v$arduino_cli_version/$archive"
	if command -v sha256sum >/dev/null; then
		got="$(sha256sum "$work/$archive" | cut -d' ' -f1)"
	else
		got="$(shasum -a 256 "$work/$archive" | cut -d' ' -f1)"
	fi
	test "$got" = "$sum" || { echo "micro-arduino: $archive: sha256 $got, want $sum" >&2; exit 1; }
	mkdir -p "$(dirname "$cli")"
	tar -xzf "$work/$archive" -C "$(dirname "$cli")" arduino-cli
fi
cat >"$home/arduino-cli.yaml" <<EOF
board_manager:
    additional_urls:
        - https://espressif.github.io/arduino-esp32/package_esp32_index.json
        - https://espressif.github.io/arduino-esp32/package_esp32_dev_index.json
directories:
    data: $home/data
    downloads: $home/staging
    user: $home/user
EOF
a() { "$cli" --config-file "$home/arduino-cli.yaml" "$@"; }
echo "micro-arduino: $(a version), esp32:esp32@$version"
a core update-index >/dev/null
a core install "esp32:esp32@$version"
rm -rf "$home/staging"
a lib update-index >/dev/null
a lib install --no-deps "M5GFX@$m5gfx_version" "M5Unified@$m5unified_version"
a core list
a lib list

"$micro/arduino/package.sh" "$work/lib"
lib="$work/lib/OECMicro"
failed=""
# compile <example> <fqbn> <required: 1, best effort: 0>
compile() {
	board="${2##*:}"
	log="$work/$1-$board.log"
	if a compile --fqbn "$2" --warnings all --library "$lib" --build-path "$work/build/$1-$board" \
		"$lib/examples/$1" >"$log" 2>&1; then
		ok=1
	else
		ok=0
	fi
	# A warning in the library (OECMicro/) or in a file of the sketch.
	own="$(grep 'warning:' "$log" | grep -E 'OECMicro/|/sketch/' || true)"
	if [ "$ok" = 1 ] && [ -z "$own" ]; then
		echo "micro-arduino: $1 $2: $(grep '^Sketch uses' "$log" || true)"
		return
	fi
	cat "$log"
	if [ "$3" = 1 ]; then
		failed="$failed $1/$board"
	else
		echo "::warning::micro-arduino: best effort: $1 on $2 failed (esp32:esp32@$version)"
	fi
}
compile Sensor esp32:esp32:esp32 1
compile Sensor esp32:esp32:esp32s3 1
compile M5Stack esp32:esp32:m5stack_core2 1
compile M5Stack esp32:esp32:m5stack_cores3 1
compile Sensor esp32:esp32:esp32c3 0
compile Sensor esp32:esp32:esp32c6 0
test -z "$failed" || { echo "micro-arduino: failed or with warnings:$failed" >&2; exit 1; }
echo "micro-arduino: ok (esp32:esp32@$version)"
