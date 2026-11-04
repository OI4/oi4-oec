package pipelines

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OI4/oec/ci/gha"
)

const (
	oec       = "../.."    // oec/ from oec/ci/pipelines
	workspace = "../../.." // the workspace repository
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// shVar returns the value of name=value in a shell file.
func shVar(t *testing.T, path, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^` + name + `="?([^"\n]*)"?$`).FindStringSubmatch(read(t, path))
	if m == nil {
		t.Fatalf("%s: no %s=", path, name)
	}
	return m[1]
}

func TestPolicy(t *testing.T) {
	for _, w := range Workflows() {
		for _, err := range gha.Check(w, Policy()) {
			t.Error(err)
		}
	}
}

// TestGenerated: the files under .github/workflows are the rendering of
// Workflows (cigen -check does the same in gate step 11).
func TestGenerated(t *testing.T) {
	for _, w := range Workflows() {
		want, err := gha.Render(w)
		if err != nil {
			t.Fatal(err)
		}
		got := read(t, filepath.Join(workspace, ".github/workflows", w.File))
		if got != string(want) {
			t.Errorf("%s is outdated: cd oec/ci && go run ./cmd/cigen", w.File)
		}
		for _, bad := range []string{"pull_request_target", "workflow_run", "self-hosted", "permissions: write", "write-all"} {
			if bytes.Contains(want, []byte(bad)) {
				t.Errorf("%s contains %q", w.File, bad)
			}
		}
	}
}

// TestVersions: the pins of versions.sh agree with the gate scripts and the
// Cargo packages.
func TestVersions(t *testing.T) {
	versions := filepath.Join(oec, "ci/versions.sh")
	if got, want := shVar(t, versions, "cbindgen_version"), shVar(t, filepath.Join(oec, "sdk/check.sh"), "cbindgen_version"); got != want {
		t.Errorf("versions.sh cbindgen %s, sdk/check.sh %s", got, want)
	}
	rust := shVar(t, versions, "rust_version")
	n := 0
	err := filepath.WalkDir(oec, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "target" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.Name() != "Cargo.toml" {
			return nil
		}
		m := regexp.MustCompile(`(?m)^rust-version = "([^"]+)"`).FindStringSubmatch(read(t, path))
		if m == nil {
			return nil
		}
		n++
		if rust != m[1] && !strings.HasPrefix(rust, m[1]+".") {
			t.Errorf("%s: rust-version %s, versions.sh rust_version %s", path, m[1], rust)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Errorf("only %d Cargo.toml with rust-version found", n)
	}
	if got := shVar(t, versions, "guideline_ref"); got != GuidelineRef {
		t.Errorf("versions.sh guideline_ref %s, workflows %s", got, GuidelineRef)
	}
	for _, v := range []string{"cargo_deny_version", "actionlint_version"} {
		if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(shVar(t, versions, v)) {
			t.Errorf("versions.sh %s is not a release version", v)
		}
	}
	if !regexp.MustCompile(`^ubuntu:24\.04@sha256:[0-9a-f]{64}$`).MatchString(shVar(t, versions, "ubuntu_image")) {
		t.Error("versions.sh ubuntu_image must be ubuntu:24.04 pinned by digest (runner ubuntu-24.04)")
	}
}

// TestScripts: every oec/… path the workflows run or read exists; the
// scripts are executable.
func TestScripts(t *testing.T) {
	path := regexp.MustCompile(`oec/[A-Za-z0-9_./-]+`)
	for _, w := range Workflows() {
		var refs []string
		for _, c := range w.Commands() {
			refs = append(refs, strings.Fields(c)[0])
		}
		for _, j := range w.Jobs {
			for _, s := range j.Steps {
				for _, kv := range s.With {
					if kv.Key == "go-version-file" || kv.Key == "cache-dependency-path" {
						refs = append(refs, kv.Value)
					}
				}
			}
		}
		for _, ref := range refs {
			for _, p := range path.FindAllString(ref, -1) {
				matches, _ := filepath.Glob(filepath.Join(workspace, p))
				if strings.Contains(p, "**") {
					continue // a glob of setup-go, checked by the go.sum files below
				}
				if len(matches) != 1 {
					t.Errorf("%s: %s does not exist", w.File, p)
					continue
				}
				if strings.HasSuffix(p, ".sh") {
					if fi, err := os.Stat(matches[0]); err != nil || fi.Mode()&0o111 == 0 {
						t.Errorf("%s: %s is not executable", w.File, p)
					}
				}
			}
		}
	}
	if m, _ := filepath.Glob(filepath.Join(oec, "*/go.sum")); len(m) == 0 {
		if m, _ = filepath.Glob(filepath.Join(oec, "*/*/go.sum")); len(m) == 0 {
			t.Error("no go.sum under oec/ for the setup-go cache")
		}
	}
}

// TestRunners: the platforms of ROADMAP P9 and ADR-0032 §6.
func TestRunners(t *testing.T) {
	platforms := func(w gha.Workflow, job string) map[string]bool {
		got := map[string]bool{}
		for _, j := range w.Jobs {
			if j.ID != job {
				continue
			}
			for _, os := range j.MatrixOS() {
				for _, r := range Runners {
					if r.Label == os {
						got[r.OS+"/"+r.Arch] = true
					}
				}
			}
		}
		return got
	}
	need := map[string][]string{
		"gate":        {"linux/amd64", "linux/arm64", "darwin/arm64"},
		"conformance": {"linux/amd64", "linux/arm64", "darwin/arm64", "darwin/amd64", "windows/amd64", "windows/arm64"},
	}
	jobs := map[string]gha.Workflow{"gate": Gate(), "conformance": Conformance()}
	jobID := map[string]string{"gate": "gate", "conformance": "test"}
	for name, ps := range need {
		got := platforms(jobs[name], jobID[name])
		for _, p := range ps {
			if !got[p] {
				t.Errorf("%s: no runner for %s", name, p)
			}
		}
	}
}

// TestGateRunsCheck: the gate job runs the local gate itself, after the
// tool setup, with the guideline next to oec/.
func TestGateRunsCheck(t *testing.T) {
	cmds := strings.Join(Gate().Commands(), "\n")
	if !strings.HasSuffix(cmds, "oec/check.sh\noec/ci/lint.sh") {
		t.Errorf("gate.yml must end the gate job with oec/check.sh, got:\n%s", cmds)
	}
	// The runner first frees disk space (the cargo targets of the gate do
	// not fit on the ubuntu-24.04 runner otherwise), then the runner and
	// linux.sh install the same system packages, among them the
	// compiler-rt runtime of clang (ASan/UBSan of sdk/check.sh).
	if !strings.HasPrefix(cmds, "oec/ci/disk.sh\noec/ci/packages.sh\noec/ci/setup-tools.sh\n") {
		t.Errorf("the gate job must start with oec/ci/disk.sh, packages.sh, then setup-tools.sh, got:\n%s", cmds)
	}
	if v, _ := gha.Get(Gate().Jobs[0].Env, "CARGO_INCREMENTAL"); v != "0" {
		t.Errorf("gate job: CARGO_INCREMENTAL = %q, want 0 (no incremental cache on the runner disk)", v)
	}
	// disk.sh only deletes on a GitHub-hosted Linux runner, never locally
	// or in the container of linux.sh.
	disk := read(t, filepath.Join(oec, "ci/disk.sh"))
	for _, g := range []string{`"${RUNNER_ENVIRONMENT:-}" != github-hosted`, `"$(uname -s)" != Linux`} {
		if !strings.Contains(disk, g) {
			t.Errorf("disk.sh lacks the guard %s", g)
		}
	}
	if !strings.Contains(read(t, filepath.Join(oec, "ci/linux.sh")), "/src/oec/ci/packages.sh") {
		t.Error("linux.sh does not install the packages of packages.sh")
	}
	pk := read(t, filepath.Join(oec, "ci/packages.sh"))
	for _, p := range []string{" clang ", " libclang-rt-dev ", " cmake "} {
		if !strings.Contains(pk, p) {
			t.Errorf("packages.sh does not install%s", p)
		}
	}
	var guideline bool
	for _, s := range Gate().Jobs[0].Steps {
		if p, _ := gha.Get(s.With, "path"); p == GuidelinePath {
			d, _ := gha.Get(s.With, "fetch-depth")
			guideline = d == "0"
		}
	}
	if !guideline {
		t.Error("gate job: no guideline checkout with fetch-depth 0 at " + GuidelinePath)
	}
	if !strings.Contains(read(t, filepath.Join(oec, "docs/check.sh")), "/../../"+GuidelinePath+"/") {
		t.Error("the gate no longer reads the guideline next to the workspace")
	}
}
