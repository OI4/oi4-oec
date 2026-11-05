#!/bin/sh
# Publishes a release assembled by release.sh (ADR-0034, C-110) on the
# channels of C-109 D-3 (a):
#
#   oec/ci/publish.sh <dir> [--publish]
#
# 1. Checks <dir>: SHA256SUMS covers every file and matches, the version of
#    RELEASE-NOTES.md, the source archive.
# 2. Prepares the release repository OI4/oec (C-104 D-3 (a): oec/ is its
#    root) from the source archive: one commit on its main; in every Go
#    module the replace directives to sibling modules are replaced by the
#    release version and go.sum is completed (go mod tidy against the local
#    repository, repeated until it no longer changes); the tags v<v> and
#    <module directory>/v<v> (ADR-0033 §5). Checked from the local
#    repository with an empty module cache: go list -mod=readonly and
#    go mod verify in every module, go install …/cmd/oec-check@v<v> and
#    …/cmd/oec-infra@v<v>.
# 3. Checks the images: every oec-<image>-<v>.oci.tar is an OCI index with
#    linux/amd64 and linux/arm64.
# 4. Writes the body of the GitHub Release v<v>: RELEASE-NOTES.md and where
#    to get the release.
#
# Without --publish (the dry run of every release.yml run): nothing leaves
# a temporary directory, no token is needed (the network only for the Go
# modules of other authors); the commands that would write to GitHub or
# GHCR are printed (remote). With --publish (job publish of release.yml,
# environment release): only on the tag build of a release
# (GITHUB_REF_TYPE=tag, GITHUB_REF_NAME=oec-v<version>, not a snapshot),
# with OEC_RELEASE_TOKEN (write access to the contents of OI4/oec and to
# the packages of the organization OI4) and OEC_RELEASE_USER (the account of
# the token); then, in this order, so that the release appears last: pushes
# main and the tags of OI4/oec (atomic; main follows the main of OI4/oec, a
# tag that exists with the same tree is kept), copies the images to
# ghcr.io/oi4/oec-<image>:<v> (skopeo copy --all --preserve-digests) and
# creates the GitHub Release with every file of <dir>. Needs git, Go,
# skopeo for the image archives, and gh with --publish.
# OEC_PUBLISH_KEEP=<new dir> keeps the prepared repository (local tries).
set -eu
if [ $# -lt 1 ] || [ $# -gt 2 ]; then
	echo "usage: publish.sh <dir> [--publish]" >&2
	exit 2
fi
mode=dry
if [ $# = 2 ]; then
	test "$2" = --publish || { echo "usage: publish.sh <dir> [--publish]" >&2; exit 2; }
	mode=publish
fi
repo="OI4/oec"
registry="ghcr.io/oi4"
# The images of release.sh (TestPublish checks the list).
images="logger weather config registry conformance"
url="https://github.com/$repo"

die() {
	echo "publish: $*" >&2
	exit 1
}
# remote <command>...: a command that writes to GitHub or GHCR – run with
# --publish, printed in a dry run
remote() {
	if [ "$mode" = publish ]; then
		"$@"
	else
		echo "publish: dry run, would run: $*"
	fi
}
for c in git go; do
	command -v "$c" >/dev/null || die "no $c"
done
test -d "$1" || die "no directory $1"
out="$(cd "$1" && pwd)"
export GOTOOLCHAIN=auto LC_ALL=C

# 1. The release.
for f in SHA256SUMS RELEASE-NOTES.md; do
	test -f "$out/$f" || die "$out is not an assembled release (release.sh assemble): no $f"
done
if command -v sha256sum >/dev/null; then sha="sha256sum"; else sha="shasum -a 256"; fi
# shellcheck disable=SC2086 # sha may be two words
(cd "$out" && $sha -c --quiet SHA256SUMS) || die "$out does not match its SHA256SUMS"
listed="$(sed 's/^[0-9a-f]\{64\}  //' "$out/SHA256SUMS" | sort)"
present="$(cd "$out" && find . -type f ! -name SHA256SUMS | sed 's|^\./||' | sort)"
test "$listed" = "$present" || die "the files of $out are not those of SHA256SUMS"
version="$(sed -n '1s/^# OEC //p' "$out/RELEASE-NOTES.md")"
echo "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-snapshot\.g[0-9a-f]{12})?$' ||
	die "RELEASE-NOTES.md: no release or snapshot version: $version"
# shellcheck disable=SC2016 # the backquotes are Markdown
commit="$(sed -n 's/^| commit | `\([0-9a-f]\{40\}\)` (\(.*\)) |$/\1/p' "$out/RELEASE-NOTES.md")"
# shellcheck disable=SC2016
date="$(sed -n 's/^| commit | `\([0-9a-f]\{40\}\)` (\(.*\)) |$/\2/p' "$out/RELEASE-NOTES.md")"
if [ -z "$commit" ] || [ -z "$date" ]; then die "RELEASE-NOTES.md: no commit"; fi
source="$out/oec-$version-source.tar.gz"
test -f "$source" || die "no oec-$version-source.tar.gz"
if [ "$mode" = publish ]; then
	: "${GITHUB_REF_TYPE:=}" "${GITHUB_REF_NAME:=}"
	if [ "$GITHUB_REF_TYPE" != tag ] || [ "$GITHUB_REF_NAME" != "oec-v$version" ]; then
		die "--publish only on the tag build oec-v$version (GITHUB_REF_TYPE=$GITHUB_REF_TYPE, GITHUB_REF_NAME=$GITHUB_REF_NAME)"
	fi
	case "$version" in *-*) die "--publish: $version is a snapshot" ;; esac
	test -n "${OEC_RELEASE_TOKEN:-}" || die "--publish: no OEC_RELEASE_TOKEN (secret of the environment release)"
	test -n "${OEC_RELEASE_USER:-}" || die "--publish: no OEC_RELEASE_USER (variable of the environment release)"
	command -v gh >/dev/null || die "no gh"
	command -v skopeo >/dev/null || die "no skopeo"
fi
echo "publish: $mode of $version (commit $commit) from $out to $url and $registry"

tmp="$(mktemp -d)"
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
if [ -n "${OEC_PUBLISH_KEEP:-}" ]; then
	test ! -e "$OEC_PUBLISH_KEEP" || die "OEC_PUBLISH_KEEP: $OEC_PUBLISH_KEEP exists"
	mkdir -p "$OEC_PUBLISH_KEEP"
	work="$(cd "$OEC_PUBLISH_KEEP" && pwd)"
else
	work="$tmp/repo"
	mkdir "$work"
fi
# git without the configuration of the host; the go command reaches the
# release repository through insteadOf (GIT_CONFIG_COUNT reaches its git).
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ALLOW_PROTOCOL=file:https
export GIT_AUTHOR_NAME="OEC release" GIT_AUTHOR_EMAIL="41898282+github-actions[bot]@users.noreply.github.com"
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"
export GIT_AUTHOR_DATE="$date" GIT_COMMITTER_DATE="$date"
local_git() {
	GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="url.file://$work.insteadOf" GIT_CONFIG_VALUE_0="$url" "$@"
}
# auth_git <command>...: git with the token as HTTP header (never in an
# argument or a URL)
auth_git() {
	GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="http.https://github.com/.extraheader" \
		GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf '%s:%s' "$OEC_RELEASE_USER" "$OEC_RELEASE_TOKEN" | base64 | tr -d '\n')" "$@"
}

# 2. The release repository.
tar -xzf "$source" -C "$work" --strip-components=1
git -C "$work" init -q -b main
modules="$(cd "$work" && find . -name go.mod ! -path '*/node_modules/*' | sed 's|^\./||; s|/*go\.mod$||' | sort)"
test -n "$modules" || die "no Go module in the source archive"
tags="v$version"
edited=""
for d in $modules; do
	test -n "$d" || die "a go.mod at the root of oec/: no module directory for the tag"
	tags="$tags $d/v$version"
	json="$(cd "$work/$d" && go mod edit -json | tr -d ' \t\n')"
	case "$json" in *"\"Module\":{\"Path\":\"github.com/$repo/$d\"}"*) ;; *) die "$d/go.mod: module path is not github.com/$repo/$d" ;; esac
	olds="$(echo "$json" | grep -o '"Old":{"Path":"[^"]*"' | sed 's/.*"Path":"//; s/"$//' || true)"
	for o in $olds; do
		case "$o" in
		"github.com/$repo/"*) (cd "$work/$d" && go mod edit -dropreplace="$o" -require="$o@v$version") ;;
		*) die "$d/go.mod: replace of $o (only sibling modules are replaced)" ;;
		esac
		case " $edited " in *" $d "*) ;; *) edited="$edited $d" ;; esac
	done
	case "$(cd "$work/$d" && go mod edit -json | tr -d ' \t\n')" in *'"Replace":'*) die "$d/go.mod: a replace directive is left" ;; esac
done
echo "publish: Go modules: $(echo "$modules" | tr '\n' ' ')(replaced:$edited)"
proxy="file://$(go env GOMODCACHE)/cache/download,$(go env GOPROXY)"
retag() {
	git -C "$work" add -A
	git -C "$work" commit -q --allow-empty -m "$1"
	for t in $tags; do
		git -C "$work" tag -f "$t" >/dev/null
	done
}
i=0
while :; do
	i=$((i + 1))
	test "$i" -le 5 || die "go.sum does not settle after 5 rounds"
	retag "round $i"
	for d in $edited; do
		(cd "$work/$d" && local_git env GOMODCACHE="$tmp/modcache-$i" GOFLAGS=-mod=mod GOPRIVATE="github.com/$repo" \
			GOPROXY="$proxy" go mod tidy) || die "$d: go mod tidy against the release repository failed"
	done
	test -n "$(git -C "$work" status --porcelain)" || break
done
tree="$(git -C "$work" write-tree)"
message="OEC $version

From $commit of OI4/oi4-oec (oec/), release oec-v$version."
parent=""
if [ "$mode" = publish ]; then
	main="$(auth_git git ls-remote "$url.git" refs/heads/main)" || die "cannot read $url"
	parent="$(echo "$main" | cut -f1)"
	if [ -n "$parent" ]; then
		auth_git git -C "$work" fetch -q "$url.git" "$parent"
	fi
else
	echo "publish: dry run: the commit has no parent (with --publish it follows main of $url)"
fi
if [ -n "$parent" ]; then
	final="$(git -C "$work" commit-tree "$tree" -p "$parent" -m "$message")"
else
	final="$(git -C "$work" commit-tree "$tree" -m "$message")"
fi
git -C "$work" update-ref refs/heads/main "$final"
git -C "$work" reset -q --hard main
for t in $tags; do
	git -C "$work" tag -f "$t" "$final" >/dev/null
done
git -C "$work" gc -q --prune=now

v_cache="$tmp/modcache-verify"
for d in $modules; do
	(cd "$work/$d" && local_git env GOMODCACHE="$v_cache" GOFLAGS=-mod=readonly GOPRIVATE="github.com/$repo" GOPROXY="$proxy" \
		sh -c 'go list -deps -test ./... >/dev/null && go mod verify >/dev/null') || die "$d: go list/go mod verify of the release failed"
done
# go install …@version runs outside a module: the toolchain of the module.
toolchain="$(sed -n 's/^toolchain //p' "$work/conformance/go.mod")"
test -n "$toolchain" || die "conformance/go.mod: no toolchain line"
(cd "$tmp" && local_git env GOMODCACHE="$v_cache" GOBIN="$tmp/bin" GOFLAGS= GOPRIVATE="github.com/$repo" GOPROXY="$proxy" \
	GOTOOLCHAIN="$toolchain" go install "github.com/$repo/conformance/cmd/oec-check@v$version" "github.com/$repo/conformance/cmd/oec-infra@v$version") ||
	die "go install …/cmd/oec-check@v$version from the release repository failed"
"$tmp/bin/oec-check" version
"$tmp/bin/oec-infra" version
echo "publish: release repository: commit $final, tree $tree, tags: $tags"

# 3. The images.
image_lines=""
n=0
for i in $images; do
	f="$out/oec-$i-$version.oci.tar"
	if [ ! -f "$f" ]; then
		test "$mode" = dry || die "no oec-$i-$version.oci.tar"
		echo "publish: dry run: no oec-$i-$version.oci.tar in $out, image $i skipped"
		continue
	fi
	command -v skopeo >/dev/null || die "no skopeo"
	skopeo inspect --raw "oci-archive:$f" >"$tmp/index.json" || die "$f: not an OCI archive"
	index="$(tr -d ' \t\n' <"$tmp/index.json")"
	case "$index" in *'"manifests":['*) ;; *) die "$f: no multi-platform image (no image index)" ;; esac
	for a in amd64 arm64; do
		case "$index" in *"\"architecture\":\"$a\",\"os\":\"linux\""* | *"\"os\":\"linux\",\"architecture\":\"$a\""*) ;; *) die "$f: no linux/$a" ;; esac
	done
	digest="$(skopeo manifest-digest "$tmp/index.json")"
	image_lines="$image_lines- \`$registry/oec-$i:$version\` (\`$digest\`)
"
	n=$((n + 1))
done
echo "publish: $n images checked"

# 4. The body of the GitHub Release.
notes="$tmp/notes.md"
{
	cat "$out/RELEASE-NOTES.md"
	echo
	echo "## Where to get it"
	echo
	echo "- The files: the assets of this release; check them with \`sha256sum -c SHA256SUMS\`."
	echo "- The Go modules: the tags \`<module directory>/v$version\` of this repository, e.g. \`go install github.com/$repo/conformance/cmd/oec-check@v$version\`. The Go SDK needs the prebuilt \`liboec_ffi\` of the same version (cgo)."
	if [ -n "$image_lines" ]; then
		echo "- The images (linux/amd64, linux/arm64):"
		echo
		printf '%s' "$image_lines"
	fi
} >"$notes"

# Publication, in this order: the repository, the images, the release.
if [ "$mode" = publish ]; then
	gh release view "v$version" --repo "$repo" >/dev/null 2>&1 && die "the GitHub Release v$version of $repo exists"
	auth="$tmp/auth.json"
	(
		umask 077
		printf '{"auths":{"ghcr.io":{"auth":"%s"}}}\n' "$(printf '%s:%s' "$OEC_RELEASE_USER" "$OEC_RELEASE_TOKEN" | base64 | tr -d '\n')" >"$auth"
	)
	GH_TOKEN="$OEC_RELEASE_TOKEN"
	export GH_TOKEN
else
	auth="<authfile>"
fi
refspecs=""
if [ "$mode" = publish ]; then
	existing="$(auth_git git ls-remote --tags "$url.git")" || die "cannot read the tags of $url"
fi
for t in $tags; do
	remote_sha="$(echo "${existing:-}" | awk -v r="refs/tags/$t" '$2 == r { print $1 }')"
	if [ -n "$remote_sha" ]; then
		auth_git git -C "$work" fetch -q "$url.git" "refs/tags/$t"
		test "$(git -C "$work" rev-parse "$remote_sha^{tree}")" = "$tree" || die "the tag $t of $url exists with another tree"
		echo "publish: tag $t exists with the same tree, kept"
	else
		refspecs="$refspecs refs/tags/$t:refs/tags/$t"
	fi
done
if [ -n "$refspecs" ]; then
	# shellcheck disable=SC2086 # refspecs is a word list
	remote auth_git git -C "$work" push --atomic "$url.git" "$final:refs/heads/main" $refspecs
fi
for i in $images; do
	f="$out/oec-$i-$version.oci.tar"
	test -f "$f" || continue
	remote skopeo copy --all --preserve-digests --retry-times 3 --authfile "$auth" "oci-archive:$f" "docker://$registry/oec-$i:$version"
done
# shellcheck disable=SC2046 # the file names have no spaces
remote gh release create "v$version" --repo "$repo" --title "OEC $version" --notes-file "$notes" --verify-tag $(find "$out" -type f | sort)
if [ "$mode" = publish ]; then
	echo "publish: published $version: $url/releases/tag/v$version"
else
	echo "publish: dry run of $version passed, nothing published"
fi
