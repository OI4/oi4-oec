#!/bin/sh
# The vulnerability check of the dependencies (ADR-0032 amendment C-108),
# with network, never in the gate: govulncheck on every Go module (only
# vulnerabilities in called code fail; the module-level findings are in the
# output), cargo deny check advisories with the fresh RustSec database on
# every Cargo.lock outside the spikes (without the fallback of the gate to
# the cached database) and npm audit of the Registry Web UI (package-lock.json,
# production and development packages). Every check runs; the outputs go to
# <dir>; it fails at the end if one check failed. govulncheck is a tool,
# not a dependency: go run at the version of versions.sh, built with the
# toolchain of the checked module (an older toolchain cannot load a newer
# standard library).
set -eu
test $# -eq 1 || { echo "usage: audit.sh <dir>" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
# shellcheck source-path=SCRIPTDIR source=versions.sh
. "$here/versions.sh"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
command -v npm >/dev/null || { echo "audit: npm is missing" >&2; exit 1; }
failed=""

# check <name> <command...>: runs the command, its output into <dir>/<name>.txt
check() {
	name="$1"
	shift
	if "$@" >"$out/$name.txt" 2>&1; then
		echo "audit: $name ok"
	else
		echo "audit: $name FAILED (see $name.txt)"
		tail -n 40 "$out/$name.txt"
		failed="$failed $name"
	fi
}

for mod in $(cd "$oec" && find . -name go.mod -not -path '*/node_modules/*' -not -path '*/target/*' | sort); do
	dir="$(dirname "${mod#./}")"
	go="$(cd "$oec/$dir" && GOTOOLCHAIN=auto go env GOVERSION)"
	# shellcheck disable=SC2016 # expanded by the inner sh
	check "govulncheck-$(echo "$dir" | tr / -)" sh -c 'cd "$1" && GOTOOLCHAIN="$2" go run "golang.org/x/vuln/cmd/govulncheck@v$3" -show verbose ./...' \
		- "$oec/$dir" "$go" "$govulncheck_version"
done
for lock in $(cd "$oec" && find . -name Cargo.lock -not -path '*/target/*' -not -path '*/spike/*' | sort); do
	dir="$(dirname "${lock#./}")"
	check "cargo-deny-$(echo "$dir" | tr / -)" cargo deny --log-level error --config "$oec/sdk/rust/deny.toml" \
		--manifest-path "$oec/$dir/Cargo.toml" check advisories
done
# shellcheck disable=SC2016 # expanded by the inner sh
check npm-audit-registry-ui sh -c 'cd "$1" && npm audit --package-lock-only' - "$oec/registry/ui"
test -z "$failed" || { echo "audit: failed:$failed" >&2; exit 1; }
echo "audit: all checks passed"
