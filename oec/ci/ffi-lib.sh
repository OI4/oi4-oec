#!/bin/sh
# Builds the prebuilt C ABI library of the Rust SDK for one target (sdk.yml,
# ADR-0032 §6, C-105): the release build of oec-ffi with the default
# features (crypto-aws-lc-rs), its system libraries (rustc --print
# native-static-libs) and the directory <dir>/liboec_ffi-<target>/ with
# include/oec.h, lib/liboec_ffi.a (lib/oec_ffi.lib for MSVC),
# native-static-libs.txt, BUILDINFO, LICENSE and SHA256SUMS. Static library
# only. The last line of the output is the directory. <target> defaults to
# the host of rustc; a cross target needs its rustup target and C toolchain
# (CC_<target>, CARGO_TARGET_<TARGET>_LINKER). Workflow artifact, no
# release (Q-011).
set -eu
test $# -eq 1 || test $# -eq 2 || { echo "usage: ffi-lib.sh <dir> [<target>]" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
rust="$oec/sdk/rust"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
host="$(rustc -vV | sed -n 's/^host: //p')"
target="${2:-$host}"
case "$target" in
x86_64-pc-windows-*) export AWS_LC_SYS_PREBUILT_NASM=1 ;;
esac
case "$target" in
*-windows-msvc) lib="oec_ffi.lib" ;;
*) lib="liboec_ffi.a" ;;
esac
target_dir="${CARGO_TARGET_DIR:-$rust/target}"
if [ "$target" = "$host" ] && [ $# -eq 1 ]; then
	built="$target_dir/release/$lib"
	set -- --release
else
	built="$target_dir/$target/release/$lib"
	set -- --release --target "$target"
fi

echo "ffi-lib: oec-ffi for $target (release)" >&2
log="$out/.rustc-$target.log"
(cd "$rust" && cargo rustc --locked "$@" -p oec-ffi --lib -- --print native-static-libs) 2>"$log" || {
	cat "$log" >&2
	rm -f "$log"
	exit 1
}
native="$(sed -n 's/^note: native-static-libs: //p' "$log" | tail -1)"
rm -f "$log"
test -n "$native" || { echo "ffi-lib: rustc printed no native-static-libs" >&2; exit 1; }
case "$target" in
# aws-lc uses winpthreads with MinGW; rustc does not print it (its own std
# links it). Static: no libwinpthread-1.dll at run time (as oec/sdk/go/link.go).
*-windows-gnu) native="$native -l:libpthread.a" ;;
esac
test -f "$built" || { echo "ffi-lib: no $built" >&2; exit 1; }

pkg="$out/liboec_ffi-$target"
rm -rf "$pkg"
mkdir -p "$pkg/include" "$pkg/lib"
cp "$rust/oec-ffi/include/oec.h" "$pkg/include/"
cp "$built" "$pkg/lib/"
cp "$oec/LICENSE" "$pkg/"
printf '%s\n' "$native" >"$pkg/native-static-libs.txt"
{
	echo "target: $target"
	echo "crate: oec-ffi $(sed -n 's/^version = "\(.*\)"/\1/p' "$rust/Cargo.toml" | head -1), default features (crypto-aws-lc-rs)"
	echo "commit: $(git -C "$oec" rev-parse HEAD 2>/dev/null || echo unknown)"
	echo "rustc: $(rustc --version)"
	echo "native-static-libs: $native"
} >"$pkg/BUILDINFO"
cd "$pkg"
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
# shellcheck disable=SC2086 # the command may be two words
$sum BUILDINFO LICENSE native-static-libs.txt include/oec.h "lib/$lib" >SHA256SUMS
cat SHA256SUMS >&2
echo "$pkg"
