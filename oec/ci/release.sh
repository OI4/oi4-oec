#!/bin/sh
# Builds the release of the OEC deliverables from one commit (release.yml,
# ADR-0033, C-109) with the existing scripts, into the flat directory <dir>:
#
#   oec/ci/release.sh <dir> [<part>...]
#
# Parts (default: source binaries micro schema sbom ffi assemble):
#   source    oec-<v>-source.tar.gz: git archive of oec/ at the commit
#   binaries  oec-check-<v>-<os>-<arch>, oec-infra-<v>-<os>-<arch> (binaries.sh)
#   micro     OECMicro-<v>.zip, the Arduino library (sdk/micro/arduino/package.sh)
#   schema    oec-schema-<schema version>.tar.gz, both flavors (schematest)
#   sbom      sbom/ (sbom.sh)
#   ffi       liboec_ffi-<target>/ for OEC_CI_TARGET or the host (ffi-lib.sh)
#   images    oec-<image>-<v>.oci.tar (linux/amd64 + linux/arm64) and
#             image-reports/ with the D8 reports (the image build.sh scripts;
#             needs Docker with buildx – loads oi4/oec-*:<v> and removes them)
#   assemble  checks that every part comes from this commit, packs
#             liboec_ffi-<target>/, sbom/ and image-reports/, writes
#             RELEASE-NOTES.md and SHA256SUMS
#
# <v> is oec/VERSION on a tag build (GITHUB_REF_TYPE=tag, GITHUB_REF_NAME
# oec-v<version>), which also requires a clean tree, the CHANGELOG.md section
# of the version without "unreleased", a released schema bundle and every
# part; otherwise the build is a snapshot <version>-snapshot.g<commit>.
# Archives are reproducible (cmd/pack, git archive with SOURCE_DATE_EPOCH =
# the commit time); D7/D8 and the libraries are reproducible with the same
# toolchain and paths. Nothing is published: the workflow uploads <dir> as
# an artifact (Q-011). Needs git, Go, and for the parts their scripts' tools.
set -eu
test $# -ge 1 || { echo "usage: release.sh <dir> [source|binaries|micro|schema|sbom|ffi|images|assemble]..." >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
# shellcheck source-path=SCRIPTDIR source=versions.sh
. "$here/versions.sh"
mkdir -p "$1"
out="$(cd "$1" && pwd)"
shift
parts="${*:-source binaries micro schema sbom ffi assemble}"
# The targets of the prebuilt C ABI library: the SDK matrix (sdk.yml, sbom.sh).
ffi_targets="x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu armv7-unknown-linux-gnueabihf x86_64-apple-darwin aarch64-apple-darwin x86_64-pc-windows-msvc aarch64-pc-windows-msvc x86_64-pc-windows-gnu"
images="logger weather config registry conformance"

die() {
	echo "release: $*" >&2
	exit 1
}
version="$(cat "$oec/VERSION")"
echo "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || die "oec/VERSION: not MAJOR.MINOR.PATCH: $version"
commit="$(git -C "$oec" rev-parse HEAD)"
SOURCE_DATE_EPOCH="$(git -C "$oec" log -1 --format=%ct HEAD)"
export SOURCE_DATE_EPOCH GOTOOLCHAIN=auto
schema_version="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$oec/schema/schema.go")"
baseline="$(sed -n 's/^const Baseline = "\(.*\)"$/\1/p' "$oec/schema/schema.go")"
dirty="$(git -C "$oec" status --porcelain --untracked-files=no -- . | head -1)"
if [ "${GITHUB_REF_TYPE:-}" = tag ]; then
	test "${GITHUB_REF_NAME:-}" = "oec-v$version" || die "tag ${GITHUB_REF_NAME:-} does not name oec/VERSION (oec-v$version)"
	test -z "$dirty" || die "a tag build needs a clean tree"
	heading="$(grep -E "^## $(echo "$version" | sed 's/\./\\./g')( |$)" "$oec/CHANGELOG.md" || true)"
	test -n "$heading" || die "oec/CHANGELOG.md has no section ## $version"
	if echo "$heading" | grep -qi unreleased; then die "oec/CHANGELOG.md: $version is still unreleased"; fi
	case "$schema_version" in *-*) die "schema bundle $schema_version is not released (ADR-0019 §6)" ;; esac
	test -f "$oec/schema/release/$schema_version.json" || die "no manifest schema/release/$schema_version.json"
	kind=release
	v="$version"
else
	test -z "$dirty" || [ "${OEC_RELEASE_ALLOW_DIRTY:-}" = 1 ] ||
		die "uncommitted changes under oec/ (commit them, or OEC_RELEASE_ALLOW_DIRTY=1 for a try)"
	kind=snapshot
	v="$version-snapshot.g$(git -C "$oec" rev-parse --short=12 HEAD)"
fi
echo "release: $kind $v, commit $commit, schema $schema_version, parts: $parts"
test ! -e "$out/RELEASE-NOTES.md" || die "$out is an assembled release; use a new directory"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
# mark <part>: the record of a part, checked by assemble
mark() {
	mkdir -p "$out/parts"
	printf 'commit=%s\nversion=%s\n' "$commit" "$v" >"$out/parts/$1"
	echo "release: $1 done"
}
pack() {
	if [ ! -x "$tmp/pack" ]; then
		(cd "$here" && go build -o "$tmp/pack" ./cmd/pack)
	fi
	"$tmp/pack" -mtime "$SOURCE_DATE_EPOCH" "$@"
}
if command -v sha256sum >/dev/null; then sha="sha256sum"; else sha="shasum -a 256"; fi
# quiet <log> <command>...: the output of a script into the log, its tail on failure
quiet() {
	log="$1"
	shift
	"$@" >"$log" 2>&1 || { tail -40 "$log" >&2; die "failed: $*"; }
}

part_source() {
	# from the top level: in a subdirectory, git archive limits the tree to it
	prefix="$(git -C "$oec" rev-parse --show-prefix)"
	git -C "$(git -C "$oec" rev-parse --show-toplevel)" archive --format=tar.gz --prefix="oec-$v/" --mtime="@$SOURCE_DATE_EPOCH" \
		-o "$out/oec-$v-source.tar.gz" "$commit:$prefix"
	mark source
}

part_binaries() {
	quiet "$tmp/binaries.log" env VERSION="$v" "$here/binaries.sh" "$tmp/bin"
	for f in "$tmp"/bin/oec-check-* "$tmp"/bin/oec-infra-*; do
		name="$(basename "$f" | sed "s/^\(oec-[a-z]*\)-/\1-$v-/")"
		mv "$f" "$out/$name"
	done
	mark binaries
}

part_micro() {
	quiet "$tmp/micro.log" "$oec/sdk/micro/arduino/package.sh" "$tmp/arduino"
	pack -prefix OECMicro/ -o "$out/OECMicro-$v.zip" "$tmp/arduino/OECMicro"
	mark micro
}

part_schema() {
	quiet "$tmp/schema.log" go -C "$oec/schema/tools" run ./cmd/schematest -spec "$oec/spec" \
		-expect "$oec/schema/tests/rule-layer.yaml" -release "$oec/schema/release" \
		-changelog "$oec/schema/CHANGELOG.md" -dist "$tmp/schema-dist"
	s="$tmp/oec-schema-$schema_version"
	mkdir -p "$s"
	cp -R "$tmp/schema-dist/schemas.openindustry4.com" "$s/"
	cp "$oec/schema/CHANGELOG.md" "$oec/LICENSE" "$s/"
	pack -prefix "oec-schema-$schema_version/" -o "$out/oec-schema-$schema_version.tar.gz" "$s"
	mark schema
}

part_sbom() {
	rm -rf "$out/sbom"
	quiet "$tmp/sbom.log" env VERSION="$v" "$here/sbom.sh" "$out/sbom"
	mark sbom
}

part_ffi() {
	host="$(rustc -vV | sed -n 's/^host: //p')"
	target="${OEC_CI_TARGET:-$host}"
	case " $ffi_targets " in *" $target "*) ;; *) die "ffi: $target is not a target of the release" ;; esac
	case "$target" in
	armv7-unknown-linux-gnueabihf)
		command -v arm-linux-gnueabihf-gcc >/dev/null || die "ffi: no arm-linux-gnueabihf-gcc (oec/ci/packages.sh --armv7)"
		export CC_armv7_unknown_linux_gnueabihf=arm-linux-gnueabihf-gcc
		export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_GNUEABIHF_LINKER=arm-linux-gnueabihf-gcc
		;;
	esac
	if [ "$target" = "$host" ]; then
		"$here/ffi-lib.sh" "$out" >/dev/null
	else
		rustup target add "$target"
		"$here/ffi-lib.sh" "$out" "$target" >/dev/null
	fi
	mark "ffi-$target"
}

part_images() {
	command -v docker >/dev/null || die "images: no docker"
	mkdir -p "$out/image-reports"
	for b in demos registry conformance; do
		echo "release: $b/images/build.sh"
		rc=0
		VERSION="$v" "$oec/$b/images/build.sh" "$tmp/images" >"$out/image-reports/$b.log" 2>&1 || rc=$?
		VERSION="$v" "$oec/$b/images/build.sh" --remove >/dev/null 2>&1 || true
		test "$rc" = 0 || { tail -40 "$out/image-reports/$b.log" >&2; die "images: $b/images/build.sh failed"; }
	done
	for i in $images; do
		test -f "$tmp/images/oec-$i.oci.tar" || die "images: no oec-$i.oci.tar"
		mv "$tmp/images/oec-$i.oci.tar" "$out/oec-$i-$v.oci.tar"
	done
	mv "$tmp"/images/* "$out/image-reports/"
	mark images
}

# section <file> <heading>: the lines of the section "## <heading>…" without
# its heading, up to the next "## "
section() {
	awk -v h="## $2" '
		/^## / { p = (index($0, h) == 1 && (length($0) == length(h) || substr($0, length(h) + 1, 1) == " ")); next }
		p' "$1"
}

part_assemble() {
	test -d "$out/parts" || die "assemble: no parts in $out"
	for p in "$out"/parts/*; do
		test "$(cat "$p")" = "$(printf 'commit=%s\nversion=%s' "$commit" "$v")" ||
			die "assemble: $(basename "$p") comes from another commit or version: $(tr '\n' ' ' <"$p")"
	done
	if [ "$kind" = release ]; then
		for p in source binaries micro schema sbom images; do
			test -f "$out/parts/$p" || die "assemble: part $p is missing"
		done
		for t in $ffi_targets; do
			test -f "$out/parts/ffi-$t" || die "assemble: part ffi-$t is missing"
		done
	fi
	rustc_lines=""
	for d in "$out"/liboec_ffi-*/; do
		test -d "$d" || continue
		t="$(basename "$d")"
		t="${t#liboec_ffi-}"
		(cd "$d" && $sha -c SHA256SUMS >/dev/null) || die "assemble: liboec_ffi-$t does not match its SHA256SUMS"
		rustc_lines="$rustc_lines- \`$t\`: $(sed -n 's/^rustc: //p' "$d/BUILDINFO")
"
		case "$t" in *-windows-*) ext=zip ;; *) ext=tar.gz ;; esac
		pack -prefix "liboec_ffi-$v-$t/" -o "$out/liboec_ffi-$v-$t.$ext" "$d"
		rm -rf "$d"
	done
	if [ -d "$out/sbom" ]; then
		pack -prefix "oec-$v-sbom/" -o "$out/oec-$v-sbom.tar.gz" "$out/sbom"
		rm -rf "$out/sbom"
	fi
	if [ -d "$out/image-reports" ]; then
		pack -prefix "oec-$v-image-reports/" -o "$out/oec-$v-image-reports.tar.gz" "$out/image-reports"
		rm -rf "$out/image-reports"
	fi
	rm -rf "$out/parts"
	extra="$(cd "$out" && find . -mindepth 2 | head -1)"
	test -z "$extra" || die "assemble: $out is not flat ($extra)"

	notes="$out/RELEASE-NOTES.md"
	{
		echo "# OEC $v"
		echo
		if [ "$kind" = release ]; then
			echo "Release $version of the OEC deliverables (tag \`oec-v$version\`)."
		else
			echo "Snapshot of the next version $version (not a release): built from a commit without a release tag."
		fi
		echo
		echo "| | |"
		echo "|---|---|"
		echo "| commit | \`$commit\` ($(git -C "$oec" log -1 --format=%cI HEAD)) |"
		echo "| guideline baseline | \`$baseline\` |"
		echo "| JSON Schema bundle | $schema_version |"
		echo "| D7/D8 report format | $(sed -n 's/^const MonitorVersion = //p' "$oec/conformance/internal/report/monitor.go") / $(sed -n 's/^const InfraVersion = //p' "$oec/conformance/internal/report/infra.go") |"
		echo "| micro SDK C API | \`OECM_API_VERSION\` $(sed -n 's/^#define OECM_API_VERSION \([0-9]*\)u$/\1/p' "$oec/sdk/micro/include/oecm.h") |"
		echo "| Go toolchain | $(sed -n 's/^toolchain //p' "$oec/conformance/go.mod") |"
		echo "| Rust toolchain | $rust_version |"
		echo
		echo "Workflow artifact only: nothing of it is published (no GitHub Release, no registry, no package index). Check the files with \`sha256sum -c SHA256SUMS\`."
		echo
		echo "## Changes"
		section "$oec/CHANGELOG.md" "$version"
		echo
		echo "## JSON Schema bundle $schema_version"
		case "$schema_version" in
		*-*) section "$oec/schema/CHANGELOG.md" "Unreleased" ;;
		*) section "$oec/schema/CHANGELOG.md" "$schema_version" ;;
		esac
		if [ -n "$rustc_lines" ]; then
			echo
			echo "## Prebuilt C ABI libraries"
			echo
			printf '%s' "$rustc_lines"
		fi
		echo
		echo "## Files"
		echo
		(cd "$out" && find . -type f ! -name RELEASE-NOTES.md | sed 's|^\./||' | LC_ALL=C sort) | while IFS= read -r f; do
			echo "- \`$f\`"
		done
		echo "- \`SHA256SUMS\`"
	} >"$notes"
	(cd "$out" && find . -type f ! -name SHA256SUMS | sed 's|^\./||' | LC_ALL=C sort >"$tmp/files")
	# shellcheck disable=SC2086 # sha may be two words
	(cd "$out" && xargs $sha <"$tmp/files" >SHA256SUMS)
	echo "release: $out ($(wc -l <"$out/SHA256SUMS" | tr -d ' ') files)"
}

for p in $parts; do
	case "$p" in
	source | binaries | micro | schema | sbom | ffi | images | assemble) "part_$p" ;;
	*) die "unknown part $p" ;;
	esac
done
