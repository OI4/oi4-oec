#!/bin/sh
# The host checks of the micro SDK on one runner of micro.yml (ADR-0032 §6,
# C-106).
#
# Linux, macOS: oec/sdk/micro/check.sh (gate step 9) with the compiler
# OEC_CI_CC (micro.yml: gcc; default cc) for CMake, the sanitizers and cgo
# – the gate runs it with clang – and clang for the analysis
# (OECM_ANALYZER=clang). The crates of go-testbroker are fetched first
# (check.sh builds offline).
#
# Windows (Git Bash, the MinGW gcc of the runner; amd64): the portable part
# of check.sh. The D7 runs of the Go host runner (internal/d7run: /bin/sh
# restart hooks, POSIX signals) and the sanitizers (MinGW has no ASan) are
# Unix only, and so are clang --analyze and the symbol allowlist (properties
# of the sources, checked on Linux). What runs: gofmt, go vet and
# go test -race of the portable packages, go build of the commands (cgo),
# oecm-vecgen -check, the CMake build with -DOECM_SANITIZE=OFF and ctest,
# GCC -O2 with the warnings of the gate (LLP64: 32-bit long), the payloads
# of the emitter under oec-check validate, hello_device, arduino/package.sh
# and the headers as C++17. Needs network or a filled module cache.
set -eu
test $# -eq 0 || { echo "usage: micro-host.sh" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
micro="$(cd "$here/../sdk/micro" && pwd)"
export GOTOOLCHAIN=auto
cc="${OEC_CI_CC:-cc}"

case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN*) ;;
*)
	echo "micro-host: $(uname -s)/$(uname -m), CC=$cc, analyzer clang"
	cxx=c++
	if [ "$cc" = gcc ]; then cxx=g++; fi
	cargo fetch --locked --quiet --manifest-path "$here/../sdk/rust/Cargo.toml"
	CC="$cc" CXX="$cxx" OECM_ANALYZER=clang "$micro/check.sh"
	echo "micro-host: ok"
	exit 0
	;;
esac

test "$(go env GOARCH)" = amd64 || { echo "micro-host: windows/$(go env GOARCH): no MinGW on the runner" >&2; exit 1; }
export CC=gcc CXX=g++ CGO_ENABLED=1
command -v mingw32-make >/dev/null || { echo "micro-host: no mingw32-make (MinGW) in PATH" >&2; exit 1; }
echo "micro-host: windows, $(command -v gcc): $(gcc --version | head -1), $(cmake --version | head -1)"
# A Windows path (C:/…): the same name for bash and the native programs.
tmp="$(cygpath -m "$(mktemp -d)")"
trap 'rm -rf "$tmp"' EXIT
(cd "$here/../conformance" && go build -o "$tmp/oec-check.exe" ./cmd/oec-check)

echo "micro-host: host (gofmt, vet and go test -race of ./mqtt and ./cmd/oecm-vecgen, build, oecm-vecgen -check)"
cd "$micro/host"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
go vet ./mqtt ./cmd/oecm-vecgen
go test -race -count=1 ./mqtt ./cmd/oecm-vecgen
go build -o "$tmp/" ./cmd/...
go run ./cmd/oecm-vecgen -check

echo "micro-host: cmake (MinGW Makefiles, OECM_SANITIZE=OFF), ctest"
cd "$micro"
build="$tmp/build"
cmake -S . -B "$build" -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Debug -DOECM_SANITIZE=OFF >/dev/null
cmake --build "$build" --parallel 4 >/dev/null
(cd "$build" && ctest --output-on-failure)

echo "micro-host: gcc -O2 (warnings are errors)"
for f in src/*.c; do
	gcc -std=c17 -O2 -Wall -Wextra -Wpedantic -Wconversion -Wsign-conversion -Wshadow \
		-Wcast-qual -Wstrict-prototypes -Wmissing-prototypes -Wvla -Wformat=2 -Werror \
		-Iinclude -Isrc -c -o "$tmp/gcc.o" "$f"
done

echo "micro-host: payloads of the core → D7 oec-check validate, hello_device"
"$build/oecm-emit.exe" "$tmp/messages.jsonl"
"$tmp/oec-check.exe" validate "$tmp/messages.jsonl"
"$build/hello_device.exe" >"$tmp/hello.txt"
grep -q '^pub Oi4v2/OTConnector/.*/Pub/MAM/' "$tmp/hello.txt" || { cat "$tmp/hello.txt"; echo "hello_device: no Birth" >&2; exit 1; }

echo "micro-host: arduino/package.sh, headers as C++17"
arduino/package.sh "$tmp/arduino" >/dev/null
test -f "$tmp/arduino/OECMicro/src/oecm.h"
printf '#include "oecm.h"\n#include "oecm_sensor.h"\n' |
	g++ -std=c++17 -fsyntax-only -Wall -Wextra -Wpedantic -Werror -Iinclude -Iexamples/sensor -x c++ -
echo "micro-host: ok (windows; D7, sanitizers, analyzer and symbol allowlist run on Linux)"
