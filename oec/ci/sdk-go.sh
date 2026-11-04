#!/bin/sh
# The Go SDK on one runner of the SDK platform matrix (sdk.yml, ADR-0032
# §6, C-105), like the Go part of oec/sdk/check.sh: liboec_ffi.a into
# oec/sdk/go/lib/<GOOS>_<GOARCH>/ (go run ./cmd/oec-buildlib), the Rust
# helper go-testbroker and the D7 CLI oec-check, go vet, go test -race (the
# D7 runs and the example runs are Unix only) and the examples.
#
# Windows: cgo needs MinGW, so the library is built for the Rust target
# x86_64-pc-windows-gnu (rustup target add; the gcc of MinGW in PATH, aws-lc
# with its prebuilt NASM objects); amd64 only. With <dir>, the prebuilt
# library of that target goes to <dir>/liboec_ffi-<target>/ (ffi-lib.sh);
# on the other runners sdk-rust.sh builds it. OEC_CI_CC (sdk.yml: clang on
# Linux, as in gate.yml) becomes CC on Unix. Needs network or a fetched
# registry and module cache.
set -eu
test $# -le 1 || { echo "usage: sdk-go.sh [<dir>]" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
sdk="$(cd "$here/../sdk" && pwd)"
out=""
if [ $# -eq 1 ]; then
	# Resolved before cd "$sdk/go": sdk.yml passes the relative dist/sdk (C-105 addendum 2).
	mkdir -p "$1"
	out="$(cd "$1" && pwd)"
fi
export GOTOOLCHAIN=auto CGO_ENABLED=1
goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
exe=""
target=""
if [ "$goos" = windows ]; then
	test "$goarch" = amd64 || { echo "sdk-go: windows/$goarch: no cgo toolchain (MinGW) on the runner" >&2; exit 1; }
	exe=".exe"
	target="x86_64-pc-windows-gnu"
	export AWS_LC_SYS_PREBUILT_NASM=1
	rustup target add "$target"
elif [ -n "${OEC_CI_CC:-}" ]; then
	export CC="$OEC_CI_CC"
fi
cd "$sdk/go"
echo "sdk-go: $goos/$goarch, $(go version), $(gcc --version 2>/dev/null | head -1 || echo "no gcc")"
if [ -n "$target" ]; then
	go run ./cmd/oec-buildlib -target "$target"
else
	go run ./cmd/oec-buildlib
fi
(cd "$sdk/rust" && cargo build --locked --quiet -p oec-ffi --example go-testbroker)
broker="${CARGO_TARGET_DIR:-$sdk/rust/target}/debug/examples/go-testbroker$exe"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
(cd "$sdk/../conformance" && go build -o "$work/oec-check$exe" ./cmd/oec-check)

echo "sdk-go: go vet, go test -race"
go vet ./...
GODEBUG=cgocheck=1 OEC_GO_REQUIRE_BROKER=1 OEC_GO_TESTBROKER="$broker" OEC_GO_D7="$work/oec-check$exe" \
	go test -race -count=1 -timeout 30m ./...
echo "sdk-go: examples"
go build -o "$work/" ./examples/...
ls "$work"

if [ -n "$out" ] && [ -n "$target" ]; then
	"$here/ffi-lib.sh" "$out" "$target" >/dev/null
	echo "sdk-go: $out/liboec_ffi-$target"
fi
