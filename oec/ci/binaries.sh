#!/bin/sh
# Cross-builds D7 oec-check and D8 oec-infra for windows/linux/darwin x
# amd64/arm64 into <dir> with SHA256SUMS (ADR-0032 §6): CGO_ENABLED=0,
# -trimpath, no VCS stamping (the state of the working tree would change the
# bytes) and an empty build id, so the same commit and Go toolchain give the
# same bytes. VERSION is the version the tools report (`oec-check
# version`, the reports; -X main.version), default dev; release.sh passes the
# release version (ADR-0033). conformance.yml uploads them as a workflow
# artifact; no release (Q-011).
set -eu
test $# -eq 1 || { echo "usage: binaries.sh <dir>" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
version="${VERSION:-dev}"
export GOTOOLCHAIN=auto CGO_ENABLED=0
cd "$here/../conformance"
for os in linux darwin windows; do
	for arch in amd64 arm64; do
		ext=""
		if [ "$os" = windows ]; then ext=".exe"; fi
		for cmd in oec-check oec-infra; do
			GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags="-buildid= -X main.version=$version" -o "$out/$cmd-$os-$arch$ext" "./cmd/$cmd"
		done
	done
done
cd "$out"
rm -f SHA256SUMS
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
# shellcheck disable=SC2086
$sum oec-check-* oec-infra-* >SHA256SUMS
cat SHA256SUMS
