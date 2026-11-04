#!/bin/sh
# Checks of the micro SDK D9 (ADR-0030), offline, no Docker, no ESP-IDF:
#   host (Go module, standard library only): gofmt, go vet, go test – the
#   MQTT client (with -race) and the host runner cmd/oecm-host (the core
#   with a Go PAL over cgo) under D7 `oec-check test --level 1` and, as the
#   simulated sensor of examples/sensor, `--level 2` and `--level 3` (with a stored
#   ReferenceDesignation) on the test
#   broker go-testbroker of oec/sdk/rust (oec-check of oec/conformance built
#   into a temporary directory, ADR-0006; OECM_REQUIRE=1: a missing tool
#   fails); the C vector header tests/vectors_gen.h equals the output of
#   cmd/oecm-vecgen on oec/spec/export/vectors (G-1.4);
#   core (C17, CMake build in target/host, warnings are errors): the Unity
#   tests and the replay of the fuzz seed corpora fuzz/corpus (fuzz/run.sh
#   fuzzes with libFuzzer, opt-in) under AddressSanitizer + UBSan (ctest), clang --analyze with
#   warnings as errors over src/, the undefined symbols of the core
#   (built -ffreestanding -fno-builtin) against the allowlist of ADR-0030
#   §2 (no heap, no stdio), with GCC 14/15 (if installed) every source of
#   src/ compiled once;
#   payloads: tests/emit.c writes the messages of two Level 1 device
#   lifecycles (Will, Birth, Health, Close), of a Level 2 lifecycle with
#   requests (Get, Set) and of a Level 3 lifecycle (ReferenceDesignation,
#   NewDataSetWriterId) as JSONL, D7 `oec-check validate` checks
#   them; the host example examples/host/hello_device.c runs;
#   arduino: arduino/package.sh assembles the Arduino library (C-097) with
#   the unchanged sources of the core and the ESP-IDF port, every example
#   with its sketch and the sensor, the placeholders of arduino_secrets.h
#   empty; oecm.h and oecm_sensor.h compile as C++17 (the sketches are C++).
# Needs cmake, cargo and a C17 compiler with sanitizers (CC; clang, or GCC
# with OECM_ANALYZER=clang for the analysis).
set -eu
root="$(cd "$(dirname "$0")" && pwd)"
export GOTOOLCHAIN=auto
case "$(uname -s)" in
Darwin) leaks=0 ;; # LeakSanitizer is not supported by Apple clang
*) leaks=1 ;;
esac
export ASAN_OPTIONS="${ASAN_OPTIONS:-detect_leaks=$leaks:halt_on_error=1}"
export UBSAN_OPTIONS="${UBSAN_OPTIONS:-print_stacktrace=1:halt_on_error=1}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "micro: oec-check, go-testbroker"
rust="$root/../rust"
(cd "$root/../../conformance" && go build -o "$tmp/oec-check" ./cmd/oec-check)
(cd "$rust" && cargo build --offline --locked --quiet -p oec-ffi --example go-testbroker)
target="$(cd "$rust" && cargo metadata --offline --locked --format-version 1 --no-deps | sed -n 's/.*"target_directory":"\([^"]*\)".*/\1/p')"

echo "micro/host: gofmt, vet, test (D7 --level 1, 2 and 3), oecm-vecgen -check"
cd "$root/host"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
go vet ./...
go test -race ./mqtt
OECM_REQUIRE=1 OEC_D7="$tmp/oec-check" OEC_TESTBROKER="$target/debug/examples/go-testbroker" go test ./...
go run ./cmd/oecm-vecgen -check

echo "micro: cmake build (target/host), ctest (ASan + UBSan)"
cd "$root"
cmake -S . -B target/host -DCMAKE_BUILD_TYPE=Debug >/dev/null
cmake --build target/host --parallel 4 >/dev/null
(cd target/host && ctest --output-on-failure)

# OECM_ANALYZER: the clang of the analysis when CC is GCC (micro.yml, C-106).
analyzer="${OECM_ANALYZER:-${CC:-cc}}"
echo "micro: $(basename "$analyzer") --analyze src/"
for f in src/*.c; do
	"$analyzer" --analyze -std=c17 -Iinclude -Isrc -Xanalyzer -analyzer-werror -o /dev/null "$f"
done

echo "micro: undefined symbols of the core (ADR-0030 §2)"
allowed="memcmp memcpy memmove memset strchr strlen"
defined="$(nm -g target/host/liboecm_plain.a | awk 'NF == 3 && $2 != "U" { sub(/^_/, "", $3); print $3 }' | sort -u)"
nm -u target/host/liboecm_plain.a | awk 'NF == 1 && $1 !~ /:$/ { sub(/^_/, "", $1); print $1 }' | sort -u >"$tmp/undefined"
bad=""
while read -r sym; do
	case " $allowed " in *" $sym "*) continue ;; esac
	printf '%s\n' "$defined" | grep -qx "$sym" && continue
	bad="$bad $sym"
done <"$tmp/undefined"
test -z "$bad" || { echo "micro core uses symbols outside the allowlist:$bad" >&2; exit 1; }

gcc="$(command -v gcc-15 || command -v gcc-14 || true)"
if [ -n "$gcc" ]; then
	echo "micro: $(basename "$gcc") -O2 (warnings are errors)"
	for f in src/*.c; do
		"$gcc" -std=c17 -O2 -Wall -Wextra -Wpedantic -Wconversion -Wsign-conversion -Wshadow \
			-Wcast-qual -Wstrict-prototypes -Wmissing-prototypes -Wvla -Wformat=2 -Werror \
			-Iinclude -Isrc -c -o "$tmp/gcc.o" "$f"
	done
fi

echo "micro: payloads of the core → D7 oec-check validate"
target/host/oecm-emit "$tmp/messages.jsonl"
"$tmp/oec-check" validate "$tmp/messages.jsonl"

echo "micro: examples/host/hello_device"
target/host/hello_device >"$tmp/hello.txt"
grep -q '^pub Oi4v2/OTConnector/.*/Pub/MAM/' "$tmp/hello.txt" || { cat "$tmp/hello.txt"; echo "hello_device: no Birth" >&2; exit 1; }

echo "micro: arduino/package.sh (library layout, C-097), headers as C++17"
arduino/package.sh "$tmp/arduino" >/dev/null
lib="$tmp/arduino/OECMicro"
for f in src/*.c src/*.h include/oecm.h port/esp-idf/oecm/oecm_esp.c port/esp-idf/oecm/include/oecm_esp.h \
	arduino/OECMicro.h; do
	cmp -s "$f" "$lib/src/$(basename "$f")" || { echo "arduino: $f not packaged unchanged" >&2; exit 1; }
done
for p in name version author maintainer sentence paragraph category url architectures includes; do
	grep -q "^$p=." "$lib/library.properties" || { echo "arduino: library.properties without $p" >&2; exit 1; }
done
for ex in arduino/examples/*/; do
	n="$(basename "$ex")"
	for f in "$n.ino" oecm_sensor.c oecm_sensor.h arduino_secrets.h; do
		test -f "$lib/examples/$n/$f" || { echo "arduino: example $n without $f" >&2; exit 1; }
	done
	if grep -E '^#define SECRET_(WIFI_SSID|WIFI_PASS|CA_PEM|USERNAME|PASSWORD|CLIENT_CERT_PEM|CLIENT_KEY_PEM) ' "$ex/arduino_secrets.h" |
		grep -vE ' ""$' | grep -q .; then
		echo "arduino: ${ex%/}/arduino_secrets.h has values, not placeholders" >&2
		exit 1
	fi
done
printf '#include "oecm.h"\n#include "oecm_sensor.h"\n' |
	"${CXX:-c++}" -std=c++17 -fsyntax-only -Wall -Wextra -Wpedantic -Werror -Iinclude -Iexamples/sensor -x c++ -
