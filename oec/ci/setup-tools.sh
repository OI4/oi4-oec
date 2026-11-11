#!/bin/sh
# Installs the tools of the gate in the versions of versions.sh on a CI
# runner or in the container of linux.sh (ADR-0032 §5): the Rust toolchain
# with clippy and rustfmt (rustup; if missing: rustup-init of rustup_version,
# SHA-256 checked, --default-toolchain none), cbindgen and cargo-deny (cargo
# install --locked into $OEC_TOOLS, default ~/.oec-tools, which gate.yml
# caches). On GitHub Actions the tool directories are appended to
# GITHUB_PATH (as Windows paths on Windows). --toolchain installs only the
# Rust toolchain (sdk.yml, micro.yml). Needs network.
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

# rustup-init of rustup_version for this platform, checked against the
# SHA-256 of versions.sh before it runs (C-113 RT-3).
install_rustup() {
	exe=""
	case "$(uname -s)/$(uname -m)" in
	Linux/x86_64) target=x86_64-unknown-linux-gnu sum="$rustup_init_sha256_linux_amd64" ;;
	Linux/aarch64 | Linux/arm64) target=aarch64-unknown-linux-gnu sum="$rustup_init_sha256_linux_arm64" ;;
	Darwin/arm64) target=aarch64-apple-darwin sum="$rustup_init_sha256_darwin_arm64" ;;
	Darwin/x86_64) target=x86_64-apple-darwin sum="$rustup_init_sha256_darwin_amd64" ;;
	MINGW* | MSYS* | CYGWIN*)
		# Git Bash may run emulated on Windows on Arm: the runner's (or the
		# native) architecture, not uname -m.
		exe=.exe
		case "${RUNNER_ARCH:-${PROCESSOR_ARCHITEW6432:-${PROCESSOR_ARCHITECTURE:-}}}" in
		X64 | AMD64) target=x86_64-pc-windows-msvc sum="$rustup_init_sha256_windows_amd64" ;;
		ARM64) target=aarch64-pc-windows-msvc sum="$rustup_init_sha256_windows_arm64" ;;
		*) echo "setup-tools: no pinned rustup-init for Windows on ${RUNNER_ARCH:-${PROCESSOR_ARCHITECTURE:-?}}" >&2; exit 1 ;;
		esac
		;;
	*) echo "setup-tools: no pinned rustup-init for $(uname -s)/$(uname -m)" >&2; exit 1 ;;
	esac
	work="$(mktemp -d)"
	trap 'rm -rf "$work"' EXIT
	init="$work/rustup-init$exe"
	echo "setup-tools: installing rustup $rustup_version ($target)"
	curl --proto '=https' --tlsv1.2 -sSf -o "$init" \
		"https://static.rust-lang.org/rustup/archive/$rustup_version/$target/rustup-init$exe"
	if command -v sha256sum >/dev/null; then
		got="$(sha256sum "$init" | cut -d' ' -f1)"
	else
		got="$(shasum -a 256 "$init" | cut -d' ' -f1)"
	fi
	test "$got" = "$sum" || { echo "setup-tools: rustup-init $rustup_version ($target): sha256 $got, want $sum" >&2; exit 1; }
	chmod +x "$init"
	"$init" -y --quiet --no-modify-path --profile minimal --default-toolchain none
}
if ! command -v rustup >/dev/null; then
	if [ ! -x "$HOME/.cargo/bin/rustup" ]; then
		install_rustup
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
