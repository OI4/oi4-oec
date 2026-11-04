#!/bin/sh
# The Rust SDK on one runner of the SDK platform matrix (sdk.yml, ADR-0032
# §6, C-105): clippy and the tests of the workspace oec/sdk/rust (oec-core,
# oec, oec-ffi; the C tests of oec-ffi on Unix, with the sanitizers) for the
# host target, then the prebuilt library <dir>/liboec_ffi-<target>/
# (ffi-lib.sh) and a C smoke test: examples/hello.c linked to that static
# library with its system libraries, run without a container storage.
#
# Native builds only: the target is the host of rustc (OEC_CI_TARGET, if
# set, must equal it). Windows x86-64 builds aws-lc with its prebuilt NASM
# objects (no NASM on the runner image); Windows arm64 needs clang-cl in
# PATH (LLVM) for aws-lc. OEC_CI_CC (sdk.yml: clang on Linux, as in
# gate.yml) becomes CC on Unix – the C compiler of the C tests, the smoke
# test and the C parts of the crates; on Windows it is only the compiler of
# the smoke test (default clang of LLVM), so that aws-lc keeps its own
# choice. Needs the crates (network or a fetched registry).
set -eu
test $# -eq 1 || { echo "usage: sdk-rust.sh <dir>" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
rust="$here/../sdk/rust"
mkdir -p "$1"
out="$(cd "$1" && pwd)"

host="$(rustc -vV | sed -n 's/^host: //p')"
if [ -n "${OEC_CI_TARGET:-}" ] && [ "$OEC_CI_TARGET" != "$host" ]; then
	echo "sdk-rust: the runner builds $host, the matrix expects $OEC_CI_TARGET" >&2
	exit 1
fi
case "$host" in
x86_64-pc-windows-*) export AWS_LC_SYS_PREBUILT_NASM=1 ;;
esac
case "$host" in
*-windows-*) smoke_cc="${OEC_CI_CC:-clang}" ;;
*)
	export OEC_FFI_REQUIRE_SANITIZERS="${OEC_FFI_REQUIRE_SANITIZERS:-1}"
	if [ -n "${OEC_CI_CC:-}" ]; then export CC="$OEC_CI_CC"; fi
	smoke_cc="${CC:-cc}"
	;;
esac
echo "sdk-rust: $host, $(rustc --version)"

cd "$rust"
echo "sdk-rust: cargo clippy"
cargo clippy --locked --all-targets --quiet -- -D warnings
echo "sdk-rust: cargo test"
cargo test --locked --quiet

pkg="$("$here/ffi-lib.sh" "$out" | tail -1)"
echo "sdk-rust: C smoke test (examples/hello.c, static)"
smoke="$out/.smoke-$host"
rm -rf "$smoke"
mkdir -p "$smoke"
native="$(cat "$pkg/native-static-libs.txt")"
cflags="-std=c17 -Wall -Wextra -Wpedantic -Werror -I $pkg/include"
case "$host" in
*-windows-msvc)
	# LLVM clang for the MSVC target with the DLL CRT of Rust
	# (/defaultlib:msvcrt of native-static-libs); getenv is no error. The
	# system libraries as -l<name>: the clang driver takes a bare
	# kernel32.lib for an input file in the working directory, -lkernel32
	# goes to the linker, which searches the SDK paths (C-105 addendum 2).
	libs="$(printf '%s\n' "$native" | tr ' ' '\n' | sed -n 's/^\(.*\)\.[Ll][Ii][Bb]$/-l\1/p' | tr '\n' ' ')"
	exe="$smoke/hello.exe"
	# shellcheck disable=SC2086 # flag lists
	"$smoke_cc" $cflags -D_CRT_SECURE_NO_WARNINGS -fms-runtime-lib=dll \
		"$rust/oec-ffi/examples/hello.c" "$pkg/lib/oec_ffi.lib" $libs -o "$exe"
	;;
*)
	exe="$smoke/hello"
	# shellcheck disable=SC2086
	"$smoke_cc" $cflags "$rust/oec-ffi/examples/hello.c" "$pkg/lib/liboec_ffi.a" $native -o "$exe"
	;;
esac
# No container storage: the bootstrap fails with its error (REQ-CNT-021);
# that proves the program links and runs the SDK.
set +e
OEC_ROOT="$smoke/no-root" "$exe" >"$smoke/stdout" 2>"$smoke/stderr"
status=$?
set -e
if [ "$status" != 1 ] || ! grep -q '^build: bootstrap: ' "$smoke/stderr"; then
	echo "sdk-rust: hello exited with $status, expected 1 and a bootstrap error; stderr:" >&2
	cat "$smoke/stderr" >&2
	exit 1
fi
sed 's/^/sdk-rust: hello: /' "$smoke/stderr"
rm -rf "$smoke"
echo "sdk-rust: $pkg"
