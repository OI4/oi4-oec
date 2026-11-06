package pipelines

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OI4/oec/ci/gha"
)

var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// releaseVersion returns the release version of oec/VERSION (ADR-0033).
func releaseVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(oec, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	v := string(b)
	if !strings.HasSuffix(v, "\n") || strings.Count(v, "\n") != 1 || !semver.MatchString(strings.TrimSuffix(v, "\n")) {
		t.Fatalf("oec/VERSION must be one line MAJOR.MINOR.PATCH, got %q", v)
	}
	return strings.TrimSuffix(v, "\n")
}

// TestReleaseVersion: every manifest and constant that states the version
// of a deliverable of the release train states oec/VERSION (ADR-0033 §2,
// C-109); the schema bundle, the report formats and the micro SDK API keep
// their own versions.
func TestReleaseVersion(t *testing.T) {
	v := releaseVersion(t)
	match := func(file, re string) {
		t.Helper()
		m := regexp.MustCompile(re).FindStringSubmatch(read(t, filepath.Join(oec, file)))
		if m == nil {
			t.Errorf("%s: no version (%s)", file, re)
		} else if m[1] != v {
			t.Errorf("%s: version %s, oec/VERSION %s", file, m[1], v)
		}
	}
	match("sdk/rust/Cargo.toml", `(?m)^\[workspace\.package\]\nversion = "([^"]+)"`)
	match("demos/config/Cargo.toml", `(?m)^name = "oec-config"\n(?:.*\n)?version = "([^"]+)"`)
	match("registry/Cargo.toml", `(?m)^name = "oec-registry"\n(?:.*\n)?version = "([^"]+)"`)
	match("sdk/micro/arduino/library.properties", `(?m)^version=(.+)$`)
	match("demos/logger/src/main.c", `(?m)^#define LOGGER_VERSION "([^"]+)"$`)
	match("demos/weather/main.go", `(?m)^const version = "([^"]+)"$`)
	match("demos-smart-sensors/esp32-dht22/CMakeLists.txt", `(?m)^set\(PROJECT_VER "([^"]+)"\)$`)

	jsonVersion := func(file string, path ...string) {
		t.Helper()
		var doc any
		if err := json.Unmarshal([]byte(read(t, filepath.Join(oec, file))), &doc); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, k := range path {
			m, _ := doc.(map[string]any)
			doc = m[k]
		}
		if doc != v {
			t.Errorf("%s %s: version %v, oec/VERSION %s", file, strings.Join(path, "."), doc, v)
		}
	}
	jsonVersion("registry/ui/package.json", "version")
	jsonVersion("registry/ui/package-lock.json", "version")
	jsonVersion("registry/ui/package-lock.json", "packages", "", "version")
	jsonVersion("registry/api/openapi.json", "info", "version")
	jsonVersion("registry/api/asyncapi.json", "info", "version")

	// The defaults of the image builds, the image tags and the SBOMs:
	// ARG VERSION= in the Dockerfiles, ${VERSION:-…}, ${OEC_DEMOS_VERSION:-…}
	// and ${OEC_REGISTRY_VERSION:-…} in scripts, compose files and tests.
	defaults := regexp.MustCompile(`(?:ARG VERSION=|\b(?:VERSION|OEC_DEMOS_VERSION|OEC_REGISTRY_VERSION):-)([0-9][0-9A-Za-z.+-]*)`)
	n := 0
	err := filepath.WalkDir(oec, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "target", "node_modules", "managed_components", "dist", "build", "generated":
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if d.Name() != "Dockerfile" && ext != ".sh" && ext != ".yaml" && ext != ".yml" && ext != ".go" {
			return nil
		}
		for _, m := range defaults.FindAllStringSubmatch(read(t, path), -1) {
			n++
			if m[1] != v {
				rel, _ := filepath.Rel(oec, path)
				t.Errorf("oec/%s: default version %s, oec/VERSION %s", rel, m[1], v)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// 5 Dockerfiles x 2, 3 image build.sh, sbom.sh, podman.sh x 2,
	// live-infra.sh, compose.yaml x 4, conformance/live.sh, dev/pki test x 2
	if n < 24 {
		t.Errorf("only %d version defaults found", n)
	}

	// D7/D8 report the version they were built with (-X main.version).
	for _, f := range []string{"ci/binaries.sh", "conformance/Dockerfile"} {
		if !strings.Contains(read(t, filepath.Join(oec, f)), "-X main.version=") {
			t.Errorf("%s does not set main.version of oec-check/oec-infra", f)
		}
	}
	// The schema bundle keeps its own version 2.MINOR.PATCH (ADR-0019 §6).
	if !regexp.MustCompile(`(?m)^const Version = "2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?"$`).MatchString(read(t, filepath.Join(oec, "schema/schema.go"))) {
		t.Error("schema/schema.go: Version is not 2.MINOR.PATCH")
	}
}

// TestRelease: release.yml (ADR-0033, C-109) builds every deliverable of
// the release train from one commit – on a tag oec-v* or by hand, never on
// a branch or a pull request – with release.sh, without caches; the prebuilt
// library for each target of the SDK matrix; the last job waits for every
// other one, downloads their parts and assembles one workflow artifact with
// SHA256SUMS and the release notes. Nothing is published (Q-011).
func TestRelease(t *testing.T) {
	w := Release()
	if w.File != "release.yml" {
		t.Errorf("release workflow file %s", w.File)
	}
	var found bool
	for _, x := range Workflows() {
		found = found || x.File == w.File
	}
	if !found {
		t.Error("Workflows() lacks release.yml")
	}
	if strings.Join(w.PushTags, " ") != "oec-v*" || len(w.PushBranches) != 0 || w.PullRequest || !w.Dispatch {
		t.Errorf("release triggers: tags %v, branches %v, pull_request %v, dispatch %v", w.PushTags, w.PushBranches, w.PullRequest, w.Dispatch)
	}
	if len(w.Permissions) != 1 || w.Permissions[0] != (gha.KV{Key: "contents", Value: "read"}) {
		t.Errorf("release permissions %v, want contents: read only", w.Permissions)
	}

	// The ffi matrix is the target list of release.sh and of sbom.sh, each
	// target on the runner that builds it in sdk.yml.
	script := read(t, filepath.Join(oec, "ci/release.sh"))
	targets := strings.Fields(shVar(t, filepath.Join(oec, "ci/release.sh"), "ffi_targets"))
	if want := strings.Fields(shVar(t, filepath.Join(oec, "ci/sbom.sh"), "targets")); strings.Join(targets, " ") != strings.Join(want, " ") {
		t.Errorf("release.sh ffi_targets %v, sbom.sh targets %v", targets, want)
	}
	sdkRunner := map[string]string{}
	for _, j := range SDK().Jobs {
		for _, e := range j.Matrix {
			os, _ := gha.Get(e, "os")
			tg, _ := gha.Get(e, "target")
			if os != "" && tg != "" {
				sdkRunner[tg] = os
			}
		}
	}
	sdkRunner["armv7-unknown-linux-gnueabihf"] = job(t, SDK(), "armv7").RunsOn
	ffi := job(t, w, "ffi")
	if len(ffi.Matrix) != len(targets) {
		t.Errorf("ffi: %d matrix entries, release.sh has %d targets", len(ffi.Matrix), len(targets))
	}
	for i, e := range ffi.Matrix {
		os, _ := gha.Get(e, "os")
		tg, _ := gha.Get(e, "target")
		if i < len(targets) && tg != targets[i] {
			t.Errorf("ffi matrix %d: target %s, release.sh %s", i, tg, targets[i])
		}
		if sdkRunner[tg] != os {
			t.Errorf("ffi %s on %s, sdk.yml builds it on %q", tg, os, sdkRunner[tg])
		}
	}
	if v, _ := gha.Get(ffi.Env, "OEC_CI_TARGET"); v != "${{ matrix.target }}" {
		t.Errorf("ffi: OEC_CI_TARGET %q", v)
	}

	want := map[string]struct {
		needs, cmds, artifact string
	}{
		"ffi": {"", "oec/ci/disk.sh\noec/ci/packages.sh\noec/ci/packages.sh --armv7\noec/ci/setup-tools.sh --toolchain\noec/ci/release.sh dist/release ffi",
			"oec-release-ffi-${{ matrix.target }}"},
		"dist": {"", "oec/ci/disk.sh\noec/ci/setup-tools.sh --toolchain\noec/ci/fetch.sh\noec/ci/release.sh dist/release source binaries micro schema sbom",
			"oec-release-dist"},
		"images":  {"", "oec/ci/disk.sh\noec/ci/release.sh dist/release images", "oec-release-images"},
		"release": {"ffi dist images", "oec/ci/release.sh dist/release assemble\noec/ci/publish.sh dist/release", "oec-release"},
	}
	if n := len(w.Jobs); n != len(want)+1 || w.Jobs[n-2].ID != "release" || w.Jobs[n-1].ID != "publish" {
		t.Errorf("release: jobs %d, the last must be release and publish (TestPublish)", n)
	}
	for id, x := range want {
		j := job(t, w, id)
		if got := strings.Join(j.Needs, " "); got != x.needs {
			t.Errorf("%s: needs %q, want %q", id, got, x.needs)
		}
		if cmds := strings.Join(gha.Workflow{Jobs: []gha.Job{j}}.Commands(), "\n"); cmds != x.cmds {
			t.Errorf("%s job commands:\n%s\nwant:\n%s", id, cmds, x.cmds)
		}
		for _, s := range j.Steps {
			if s.Uses == Cache {
				t.Errorf("%s: a cache in a release job", id)
			}
			if s.Uses == SetupGo {
				if c, _ := gha.Get(s.With, "cache"); c != "false" {
					t.Errorf("%s: setup-go with its module cache", id)
				}
			}
		}
		ups := uploads(j)
		if len(ups) != 1 || j.Steps[len(j.Steps)-1].Uses != UploadArtifact {
			t.Fatalf("%s: %d upload steps, want 1 as the last step", id, len(ups))
		}
		if n, _ := gha.Get(ups[0].With, "name"); n != x.artifact {
			t.Errorf("%s: artifact %q, want %q", id, n, x.artifact)
		}
		if p, _ := gha.Get(ups[0].With, "path"); p != "dist/release" {
			t.Errorf("%s: artifact path %q, want dist/release", id, p)
		}
		if f, _ := gha.Get(ups[0].With, "if-no-files-found"); f != "error" {
			t.Errorf("%s: if-no-files-found %q, want error", id, f)
		}
	}
	// The release job takes the parts of this run only.
	var download *gha.Step
	rel := job(t, w, "release")
	for i, s := range rel.Steps {
		if s.Uses == DownloadArtifact {
			download = &rel.Steps[i]
		}
	}
	if download == nil {
		t.Fatal("release: no download of the parts")
	}
	for k, v := range map[string]string{"pattern": "oec-release-*", "path": "dist/release", "merge-multiple": "true"} {
		if got, _ := gha.Get(download.With, k); got != v {
			t.Errorf("release: download %s %q, want %q", k, got, v)
		}
	}
	if _, ok := gha.Get(download.With, "run-id"); ok {
		t.Error("release: parts of another run")
	}

	// release.sh: the parts, the checks of a tag build, reproducible
	// archives, the checksums; it builds with the existing scripts and
	// publishes nothing.
	for _, s := range []string{"oec-v$version", "SOURCE_DATE_EPOCH", "archive --format=tar.gz", "--mtime",
		"/binaries.sh", "/ffi-lib.sh", "arduino/package.sh", "/sbom.sh", "cmd/schematest",
		"for b in demos registry conformance", "/images/build.sh",
		"./cmd/pack", "SHA256SUMS", "RELEASE-NOTES.md", "CHANGELOG.md", "release/$schema_version.json"} {
		if !strings.Contains(script, s) {
			t.Errorf("release.sh lacks %s", s)
		}
	}
	if regexp.MustCompile(`(?m)^[^#]*\b(push|login|publish|upload)\b`).MatchString(script) {
		t.Error("release.sh publishes (Q-011)")
	}
	// D7/D8 without VCS stamping: an unrelated change in the working tree
	// (vcs.modified) would change their bytes; the version names the commit.
	if !strings.Contains(read(t, filepath.Join(oec, "ci/binaries.sh")), "-buildvcs=false") {
		t.Error("binaries.sh: D7/D8 depend on the state of the working tree (-buildvcs=false)")
	}
	if !regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(releaseVersion(t)) + `\b`).MatchString(read(t, filepath.Join(oec, "CHANGELOG.md"))) {
		t.Error("oec/CHANGELOG.md has no section for oec/VERSION")
	}
}
