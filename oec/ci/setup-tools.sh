#!/bin/sh
# Installs the tools of the gate in the versions of versions.sh on a CI
# runner or in the container of linux.sh (ADR-0032 §5): the Rust toolchain
# with clippy and rustfmt (rustup; installed with --default-toolchain none
# if missing), cbindgen and cargo-deny (cargo install --locked into
# $OEC_TOOLS, default ~/.oec-tools, which gate.yml caches). On GitHub Actions
# the tool directories are appended to GITHUB_PATH (as Windows paths on
# Windows). --toolchain installs only the Rust toolchain (sdk.yml,
# micro.yml). Needs network.
set -eu
toolchain_only=0
case "${1:-}" in
"") ;;
--toolchain) toolchain_only=1 ;;
*) echo "usage: setup-tools.sh [--toolchain]" >&2; exit 2 ;;
esac
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/versions.sh"
tools="${OEC_TOOLS:-$HOME/.oec-tools}"

if ! command -v rustup >/dev/null; then
	if [ ! -x "$HOME/.cargo/bin/rustup" ]; then
		echo "setup-tools: installing rustup"
		curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs |
			sh -s -- -y --quiet --no-modify-path --profile minimal --default-toolchain none
	fi
	PATH="$HOME/.cargo/bin:$PATH"
fi
echo "setup-tools: rust $rust_version (clippy, rustfmt)"
rustup toolchain install "$rust_version" --profile minimal -c clippy -c rustfmt --no-self-update
rustup default "$rust_version"

path_entry() {
	if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s\n' "$1"; fi
}
if [ "$toolchain_only" = 1 ]; then
	if [ -n "${GITHUB_PATH:-}" ]; then
		path_entry "$HOME/.cargo/bin" >>"$GITHUB_PATH"
	fi
	rustc --version
	cargo --version
	exit 0
fi

PATH="$tools/bin:$PATH"
install() {
	if [ "$("$tools/bin/$1" --version 2>/dev/null || true)" = "$1 $2" ]; then
		echo "setup-tools: $1 $2 (cached)"
		return
	fi
	echo "setup-tools: cargo install $1 $2"
	cargo install --locked --quiet --root "$tools" "$1" --version "$2"
	test "$("$tools/bin/$1" --version)" = "$1 $2"
}
install cbindgen "$cbindgen_version"
install cargo-deny "$cargo_deny_version"

if [ -n "${GITHUB_PATH:-}" ]; then
	{
		path_entry "$HOME/.cargo/bin"
		path_entry "$tools/bin"
	} >>"$GITHUB_PATH"
fi
rustc --version
cargo --version
