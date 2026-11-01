#!/bin/sh
# Opt-in container build of the Go SDK examples for linux amd64 and arm64
# (ADR-0027, P4c exit criterion 4, C-078). Needs Docker and network (base
# images, crates, Debian packages). Creates only its own objects and
# removes them: one container per step (--rm), the base images it pulled
# itself; no image is built, so no build cache is left.
#
#   oec/sdk/go/cross/run.sh [<output dir>]
#
# Then runs each binary once on its platform (linux/amd64 emulated by
# Docker): without a container storage it must end with the bootstrap
# error of the SDK, which shows that it loads and runs the Rust core.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/../../.." && pwd)"
rust_image="docker.io/library/rust:1.89-bookworm"
go_image="docker.io/library/golang:1.27-bookworm"
run_image="docker.io/library/debian:bookworm-slim"
out="${1:-$(mktemp -d)}"
work="$(mktemp -d)"
pulled=""
name="oec-go-build-$$"

for img in "$rust_image" "$go_image" "$run_image"; do
	docker image inspect "$img" >/dev/null 2>&1 || pulled="$pulled $img"
done
cleanup() {
	docker rm -f -v "$name-go" >/dev/null 2>&1 || true
	for img in $pulled; do docker image rm "$img" >/dev/null 2>&1 || true; done
	rm -rf "$work"
}
trap cleanup EXIT

docker create --name "$name-go" "$go_image" >/dev/null
docker cp -q "$name-go:/usr/local/go" "$work/go"
docker rm -v "$name-go" >/dev/null

docker run --rm --name "$name" -v "$oec:/oec:ro" -v "$work/go:/usr/local/go:ro" \
	-v "$out:/out" -v "$here/build.sh:/build.sh:ro" "$rust_image" sh /build.sh

for arch in amd64 arm64; do
	for ex in hello source; do
		msg="$(docker run --rm --name "$name-$arch-$ex" --platform "linux/$arch" -e OEC_ROOT=/nonexistent \
			-v "$out/linux_$arch:/x:ro" "$run_image" "/x/$ex" 2>&1 || true)"
		echo "linux/$arch $ex: $msg"
		case "$msg" in
		*"oec: bootstrap"*) ;;
		*) echo "linux/$arch $ex: no bootstrap error" >&2; exit 1 ;;
		esac
	done
	# per platform: a later pull of the other platform would leave this
	# one untagged
	case "$pulled" in *"$run_image"*) docker image rm "$run_image" >/dev/null ;; esac
done
echo "go cross build ok: $out"
