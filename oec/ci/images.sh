#!/bin/sh
# The image job of images.yml (ADR-0032 §6, C-107): builds the images of
# the three demos, of the OEC Registry and of the compliance tools for
# linux/amd64 and linux/arm64 with their opt-in build scripts
# (demos/images/build.sh, registry/images/build.sh,
# conformance/images/build.sh), each judged by D8 `oec-infra image` on its
# OCI archive and – the applications – by D8 `oec-infra run` on the image of
# the native platform. The native images stay loaded as oi4/oec-*:$VERSION
# (default 0.1.0, the tags of oec/dev/compose.yaml) for getting-started.sh
# and live-infra.sh.
#
#   oec/ci/images.sh [<output dir>]   (default dist/images)
#
# The output dir keeps the D8 reports, build logs and smoke outputs; the OCI
# archives are deleted after their D8 run (disk space; never uploaded: the
# images are not published, Q-011). Every build runs even if an earlier one
# failed; the script fails if one did. Runs on a Linux or macOS host with
# Docker (buildx) and Go; it pushes nothing and never prunes.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
out="${1:-dist/images}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
docker version --format 'docker: client {{.Client.Version}}, server {{.Server.Version}}'
docker buildx version
failed=
for b in demos registry conformance; do
	echo "== $b/images/build.sh"
	start=$(date +%s)
	rc=0
	"$oec/$b/images/build.sh" "$out" || rc=$?
	echo "$b: exit $rc in $(($(date +%s) - start)) s"
	[ "$rc" = 0 ] || failed="$failed $b"
	rm -f "$out"/*.oci.tar
	df -h / | tail -1 | awk '{print "disk: free " $4}'
done
echo "== D8 verdicts"
for f in "$out"/*.d8-*.txt; do
	[ -e "$f" ] || continue
	printf '%s: %s\n' "$(basename "$f" .txt)" "$(grep '^summary:' "$f" || echo 'no summary')"
done
docker image ls --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep '^oi4/oec-' || true
if [ -n "$failed" ]; then
	echo "images: failed:$failed (reports in $out)" >&2
	exit 1
fi
echo "images ok: reports in $out"
