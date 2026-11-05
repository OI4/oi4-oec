package pipelines

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
