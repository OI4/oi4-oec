#!/bin/sh
# ARMv7 Linux, best effort (Q-008), in the SDK platform matrix (sdk.yml,
# ADR-0032 §6, C-105): on a Linux amd64/arm64 host with the Debian/Ubuntu
# cross toolchain (packages.sh --armv7: gcc-arm-linux-gnueabihf and
# qemu-user) the Rust SDK is cross-built for armv7-unknown-linux-gnueabihf
# with the default features (aws-lc-rs) and also with crypto-ring; the
# tests of oec-core, oec (with both providers) and the Rust tests of oec-ffi
# run under qemu-arm (user-mode emulation; the C tests of oec-ffi need a native C
# compiler and stay with the other runners). Then the prebuilt library
# <dir>/liboec_ffi-armv7-unknown-linux-gnueabihf/ (ffi-lib.sh), the C smoke
# test of sdk-rust.sh under qemu-arm, and the Go SDK for linux/arm (GOARM=7):
# go vet and the examples, the example hello run once under qemu-arm.
set -eu
test $# -eq 1 || { echo "usage: sdk-armv7.sh <dir>" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
sdk="$(cd "$here/../sdk" && pwd)"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
target="armv7-unknown-linux-gnueabihf"
gcc="arm-linux-gnueabihf-gcc"
sysroot="/usr/arm-linux-gnueabihf"
command -v "$gcc" >/dev/null || { echo "sdk-armv7: no $gcc (oec/ci/packages.sh --armv7)" >&2; exit 1; }
command -v qemu-arm >/dev/null || { echo "sdk-armv7: no qemu-arm (oec/ci/packages.sh --armv7)" >&2; exit 1; }
export CC_armv7_unknown_linux_gnueabihf="$gcc"
export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_GNUEABIHF_LINKER="$gcc"
export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_GNUEABIHF_RUNNER="qemu-arm -L $sysroot"
export QEMU_LD_PREFIX="$sysroot"
rustup target add "$target"
echo "sdk-armv7: $(rustc --version), $($gcc --version | head -1), $(qemu-arm --version | head -1)"

cd "$sdk/rust"
echo "sdk-armv7: cargo clippy (aws-lc-rs and ring)"
cargo clippy --locked --target "$target" --all-targets --quiet -- -D warnings
cargo clippy --locked --target "$target" -p oec -p oec-ffi --all-targets --quiet \
	--no-default-features --features crypto-ring -- -D warnings
echo "sdk-armv7: cargo test under qemu-arm (oec-core, oec, oec-ffi without the C tests)"
cargo test --locked --target "$target" --quiet -p oec-core -p oec
cargo test --locked --target "$target" --quiet -p oec --no-default-features --features crypto-ring
cargo test --locked --target "$target" --quiet -p oec-ffi --lib

pkg="$("$here/ffi-lib.sh" "$out" "$target" | tail -1)"
echo "sdk-armv7: C smoke test (examples/hello.c, static, qemu-arm)"
smoke="$(mktemp -d)"
trap 'rm -rf "$smoke"' EXIT
# shellcheck disable=SC2046 # the library list
"$gcc" -std=c17 -Wall -Wextra -Wpedantic -Werror -I "$pkg/include" "$sdk/rust/oec-ffi/examples/hello.c" \
	"$pkg/lib/liboec_ffi.a" $(cat "$pkg/native-static-libs.txt") -o "$smoke/hello"
smoke_run() {
	set +e
	OEC_ROOT="$smoke/no-root" qemu-arm "$@" >"$smoke/stdout" 2>"$smoke/stderr"
	status=$?
	set -e
	if [ "$status" != 1 ] || ! grep -q 'bootstrap: ' "$smoke/stderr"; then
		echo "sdk-armv7: $1 exited with $status, expected 1 and a bootstrap error; stderr:" >&2
		cat "$smoke/stderr" >&2
		exit 1
	fi
	sed "s|^|sdk-armv7: $(basename "$1"): |" "$smoke/stderr"
}
smoke_run "$smoke/hello"

echo "sdk-armv7: Go SDK for linux/arm (GOARM=7)"
cd "$sdk/go"
export GOTOOLCHAIN=auto
go run ./cmd/oec-buildlib -target "$target"
export CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC="$gcc"
go vet ./...
go build -o "$smoke/" ./examples/...
smoke_run "$smoke/hello"
echo "sdk-armv7: $pkg"
