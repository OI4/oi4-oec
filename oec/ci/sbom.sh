#!/bin/sh
# Makes the SBOM (SPDX 2.3 JSON), the license texts and the license report
# of every deliverable into <dir>/<deliverable>/ (sbom.spdx.json, <id>.txt,
# LICENSES.md) with oec-sbom (ADR-0028 §4, ADR-0032 amendment C-108). No
# build: the crates come from cargo tree over the targets of the SDK matrix,
# the Go modules from go list -deps over the platforms of the binaries.
# The image SBOMs (with the Debian base) stay in the images
# (/opt/oi4/licenses). supply-chain.yml uploads <dir> as a workflow
# artifact; no release (Q-011). Needs Go and cargo with the crates fetched
# (offline); VERSION (default 0.1.0) is the version of the deliverables
# whose manifest states none.
set -eu
test $# -eq 1 || { echo "usage: sbom.sh <dir>" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
# shellcheck source=versions.sh
. "$here/versions.sh"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
version="${VERSION:-0.1.0}"
export GOTOOLCHAIN=auto
if [ -z "${SOURCE_DATE_EPOCH:-}" ] && git -C "$oec" rev-parse HEAD >/dev/null 2>&1; then
	SOURCE_DATE_EPOCH="$(git -C "$oec" log -1 --format=%ct)"
	export SOURCE_DATE_EPOCH
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
(cd "$oec/demos" && go build -o "$tmp/oec-sbom" ./internal/sbom/cmd/oec-sbom)
texts="$oec/demos/images/licenses"
golicenses="$oec/conformance/images/go-licenses.json"
targets="x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu armv7-unknown-linux-gnueabihf x86_64-apple-darwin aarch64-apple-darwin x86_64-pc-windows-msvc aarch64-pc-windows-msvc x86_64-pc-windows-gnu"
platforms="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

# cargo_tree <manifest dir> <crate>: the crates linked into <crate>
cargo_tree() {
	f="$tmp/$2.cargo"
	set -- --locked --offline --manifest-path "$oec/$1/Cargo.toml" -p "$2" \
		--edges normal,no-proc-macro --prefix none --format '{p}|{l}'
	for t in $targets; do set -- "$@" --target "$t"; done
	cargo tree "$@" >"$f"
	echo "$f"
}

# go_list <module dir> <cgo 0|1> <packages...>: the Go modules of the
# packages on every platform
go_list() {
	dir="$1" cgo="$2"
	shift 2
	f="$tmp/$(echo "$dir $*" | tr -c 'a-zA-Z0-9\n' '_').golist"
	: >"$f"
	for p in $platforms; do
		(cd "$oec/$dir" && CGO_ENABLED="$cgo" GOOS="${p%/*}" GOARCH="${p#*/}" go list -deps \
			-f '{{with .Module}}{{.Path}}|{{.Version}}|{{with .Replace}}{{.Path}}|{{.Version}}{{end}}{{end}}' "$@") >>"$f"
	done
	echo "$f"
}

# go_version <module dir>: the Go toolchain of the module (its standard library)
go_version() {
	(cd "$oec/$1" && go env GOVERSION)
}

# manifest_version <Cargo.toml>: the version of the package or workspace
manifest_version() {
	sed -n 's/^version = "\(.*\)"$/\1/p' "$1" | head -n 1
}

# sbom <deliverable> <version> <oec-sbom flags...>
sbom() {
	name="$1" v="$2"
	shift 2
	rm -rf "${out:?}/$name"
	"$tmp/oec-sbom" -name "$name" -version "$v" -license MIT -texts "$texts" \
		-out "$out/$name" -report "$out/$name/LICENSES.md" "$@"
	n="$(grep -c '"SPDXID": "SPDXRef-Package-' "$out/$name/sbom.spdx.json")"
	echo "sbom: $name $v, $n components"
}

rust_sdk="$(manifest_version "$oec/sdk/rust/Cargo.toml")"
ffi="$(cargo_tree sdk/rust oec-ffi)"
sbom oec-rust-sdk "$rust_sdk" -cargo "$(cargo_tree sdk/rust oec)"
sbom oec-ffi "$rust_sdk" -cargo "$ffi" -rust "$rust_version"
sbom oec-go-sdk "$version" -golist "$(go_list sdk/go 1 .)" -cargo "$ffi" -rust "$rust_version"
sbom oec-micro-sdk "$(sed -n 's/^version=//p' "$oec/sdk/micro/arduino/library.properties")"
for cmd in oec-check oec-infra; do
	sbom "$cmd" "$version" -golist "$(go_list conformance 0 "./cmd/$cmd")" -golicenses "$golicenses" \
		-gostd "$(go_version conformance)"
done
sbom oec-logger "$version" -cargo "$ffi" -rust "$rust_version"
sbom oec-weather "$version" -golist "$(go_list demos 1 ./weather)" -gostd "$(go_version demos)" \
	-cargo "$ffi" -rust "$rust_version"
sbom oec-config "$(manifest_version "$oec/demos/config/Cargo.toml")" \
	-cargo "$(cargo_tree demos/config oec-config)" -rust "$rust_version"
sbom oec-registry "$(manifest_version "$oec/registry/Cargo.toml")" \
	-cargo "$(cargo_tree registry oec-registry)" -rust "$rust_version" -npm "$oec/registry/ui/package-lock.json"
sbom oec-schema "$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$oec/schema/schema.go")"
