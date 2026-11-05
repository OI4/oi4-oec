#!/bin/sh
# Opt-in build of the image of the compliance tools D7 oec-check and D8
# oec-infra (ADR-0025 §10, amendment C-107) with docker buildx, as the demo
# and registry images (oec/demos/images/build.sh); not part of the gate
# (needs Docker and network: base and builder images, Go modules). Creates
# only its own objects:
#
#   oec/conformance/images/build.sh [<output dir>]   build, check with D8, load
#   oec/conformance/images/build.sh --remove         remove the loaded image
#
# It builds the image for linux/amd64 and linux/arm64 as the OCI archive
# <output dir>/oec-conformance.oci.tar, runs D8 `oec-infra image` on it at
# level 3 (reports next to it), loads the image of the native platform as
# oi4/oec-conformance:$VERSION into Docker and runs both tools from the
# loaded image (oec-check list; oec-infra image on the archive, mounted
# read-only). D8 `run` does not apply: the tools are no OEC application (no
# MQTT client of an OEC asset). A D8 verdict other than pass or a failing
# smoke run makes the script fail. The builds run in a buildx builder of
# their own (driver docker-container) with BuildKit pinned by tag and
# digest, which is removed at the end with its cache; a BuildKit image the
# script pulled is removed as well. Nothing is pruned.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/../.." && pwd)"
version="${VERSION:-0.1.0}"
image="oi4/oec-conformance:$version"
# BuildKit v0.34.1 (Apache-2.0), multi-arch index digest of 2026-11-01 (as the demos)
buildkit="docker.io/moby/buildkit:v0.34.1@sha256:99368f3e46177c6405b9740e4461d91cee004128daeecca78d4ceac50ec1da5c"

if [ "${1:-}" = "--remove" ]; then
	docker image rm "$image" 2>/dev/null || true
	exit 0
fi

out="${1:-$(mktemp -d)}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
case "$(uname -m)" in
x86_64 | amd64) native=amd64 ;;
arm64 | aarch64) native=arm64 ;;
*) echo "unsupported host $(uname -m)" >&2; exit 1 ;;
esac
epoch="$(git -C "$oec" log -1 --format=%ct)"
builder="oec-conformance-$$"
tmp="$(mktemp -d)"
had_buildkit=0
docker image inspect "$buildkit" >/dev/null 2>&1 && had_buildkit=1
cleanup() {
	docker buildx rm "$builder" >/dev/null 2>&1 || true
	if [ "$had_buildkit" = 0 ]; then
		id="$(docker image inspect -f '{{.Id}}' "$buildkit" 2>/dev/null || true)"
		[ -z "$id" ] || docker image rm "$id" >/dev/null 2>&1 || true
	fi
	rm -rf "$tmp"
}
trap cleanup EXIT

(cd "$oec/conformance" && GOTOOLCHAIN=auto go build -o "$tmp/oec-infra" ./cmd/oec-infra)
docker buildx create --name "$builder" --driver docker-container --driver-opt "image=$buildkit" >/dev/null
d8=0
echo "== oec-conformance: linux/amd64 + linux/arm64"
start=$(date +%s)
docker buildx build --builder "$builder" --progress plain -f "$oec/conformance/Dockerfile" \
	--platform linux/amd64,linux/arm64 --build-arg VERSION="$version" \
	--build-arg SOURCE_DATE_EPOCH="$epoch" \
	-o "type=oci,dest=$out/oec-conformance.oci.tar" "$oec" >"$out/oec-conformance.build.log" 2>&1 ||
	{ tail -40 "$out/oec-conformance.build.log" >&2; exit 1; }
echo "built in $(($(date +%s) - start)) s: $(du -h "$out/oec-conformance.oci.tar" | cut -f1) $out/oec-conformance.oci.tar"
rc=0
"$tmp/oec-infra" image --level 3 --report "json=$out/oec-conformance.d8-image.json" \
	"$out/oec-conformance.oci.tar" >"$out/oec-conformance.d8-image.txt" || rc=$?
echo "D8 oec-infra image --level 3: exit $rc ($out/oec-conformance.d8-image.txt)"
[ "$rc" = 0 ] || d8=1
old="$(docker image inspect -f '{{.Id}}' "$image" 2>/dev/null || true)"
docker buildx build --builder "$builder" --progress quiet -f "$oec/conformance/Dockerfile" \
	--platform "linux/$native" --build-arg VERSION="$version" \
	--build-arg SOURCE_DATE_EPOCH="$epoch" \
	-t "$image" --load "$oec" >/dev/null
new="$(docker image inspect -f '{{.Id}}' "$image")"
if [ -n "$old" ] && [ "$old" != "$new" ]; then
	# the image of an earlier run lost its tag; without -f it stays if it has another
	docker image rm "$old" >/dev/null 2>&1 || true
fi
echo "loaded $image (linux/$native): $(docker image inspect -f '{{.Size}}' "$image") bytes"
# smoke run of both tools from the loaded image (no network needed)
rc=0
docker run --rm --network none "$image" oec-check list >"$out/oec-conformance.smoke-check.txt" 2>&1 || rc=$?
echo "smoke: oec-check list in the image: exit $rc"
[ "$rc" = 0 ] || d8=1
rc=0
docker run --rm --network none -v "$out/oec-conformance.oci.tar:/work/image.oci.tar:ro" "$image" \
	oec-infra image --level 3 /work/image.oci.tar >"$out/oec-conformance.smoke-infra.txt" 2>&1 || rc=$?
echo "smoke: oec-infra image --level 3 in the image on its own archive: exit $rc"
[ "$rc" = 0 ] || d8=1
[ "$d8" = 0 ] || {
	echo "D8 or the smoke run did not pass (see $out); the loaded image stays for a look ($0 --remove)" >&2
	exit 1
}
echo "image ok: $out (remove the loaded image with $0 --remove)"
