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

// TestPathsIgnore (C-111 D-3 (a)): pushes and pull requests that only change
// the project documents (project/) start no workflow; the release workflow
// (tags) has no path filter. The gate never reads project/ – only spectool
// rebaseline names the living project documents.
func TestPathsIgnore(t *testing.T) {
	for _, w := range Workflows() {
		got := strings.Join(w.PathsIgnore, " ")
		switch {
		case len(w.PushBranches) > 0 && got != "project/**":
			t.Errorf("%s: paths-ignore %q, want project/**", w.File, got)
		case len(w.PushBranches) == 0 && got != "":
			t.Errorf("%s: paths-ignore %q on a workflow without a branch trigger", w.File, got)
		}
	}
	allowed := map[string]bool{"spec/tools/cmd/spectool/main.go": true, "ci/pipelines/pipelines.go": true}
	err := filepath.WalkDir(oec, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "target", "build", ".git", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".sh") && !(strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")) {
			return nil
		}
		rel, _ := filepath.Rel(oec, path)
		if allowed[filepath.ToSlash(rel)] {
			return nil
		}
		if strings.Contains(read(t, path), "project/") {
			t.Errorf("oec/%s mentions project/ – documentation-only pushes skip the CI (paths-ignore)", filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
	for _, v := range []string{"cargo_deny_version", "actionlint_version", "govulncheck_version"} {
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
		"sdk/rust":    {"linux/amd64", "linux/arm64", "darwin/arm64", "darwin/amd64", "windows/amd64", "windows/arm64"},
		"sdk/go":      {"linux/amd64", "linux/arm64", "darwin/arm64", "darwin/amd64", "windows/amd64"},
		"sdk/armv7":   {"linux/amd64"},
		"micro/host":  {"linux/amd64", "linux/arm64", "windows/amd64"},
		"images":      {"linux/amd64", "linux/arm64"},
	}
	jobs := map[string]gha.Workflow{"gate": Gate(), "conformance": Conformance(),
		"sdk/rust": SDK(), "sdk/go": SDK(), "sdk/armv7": SDK(), "micro/host": Micro(), "images": Images()}
	jobID := map[string]string{"gate": "gate", "conformance": "test",
		"sdk/rust": "rust", "sdk/go": "go", "sdk/armv7": "armv7", "micro/host": "host", "images": "images"}
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

// rustTarget is the Rust host target of a runner platform.
var rustTarget = map[string]string{
	"linux/amd64":   "x86_64-unknown-linux-gnu",
	"linux/arm64":   "aarch64-unknown-linux-gnu",
	"darwin/arm64":  "aarch64-apple-darwin",
	"darwin/amd64":  "x86_64-apple-darwin",
	"windows/amd64": "x86_64-pc-windows-msvc",
	"windows/arm64": "aarch64-pc-windows-msvc",
}

func runner(t *testing.T, label string) Runner {
	t.Helper()
	for _, r := range Runners {
		if r.Label == label {
			return r
		}
	}
	t.Fatalf("no runner %s", label)
	return Runner{}
}

func job(t *testing.T, w gha.Workflow, id string) gha.Job {
	t.Helper()
	for _, j := range w.Jobs {
		if j.ID == id {
			return j
		}
	}
	t.Fatalf("%s: no job %s", w.File, id)
	return gha.Job{}
}

// uploads returns the artifact names and paths of the upload steps of j.
func uploads(j gha.Job) (steps []gha.Step) {
	for _, s := range j.Steps {
		if s.Uses == UploadArtifact {
			steps = append(steps, s)
		}
	}
	return steps
}

// TestSDK: the SDK platform matrix of ADR-0032 §6 (C-105) – the matrix
// target of each runner is its Rust host target (sdk-rust.sh checks it on
// the runner), every job ends with its script and uploads the prebuilt
// library liboec_ffi-<target> built for that target; the Go SDK on Windows
// links the MinGW library; ARMv7 is a cross build with the cross packages.
func TestSDK(t *testing.T) {
	w := SDK()
	if w.File != "sdk.yml" {
		t.Errorf("SDK workflow file %s", w.File)
	}
	upload := func(j gha.Job, name, path, cond string) {
		t.Helper()
		ups := uploads(j)
		if len(ups) != 1 {
			t.Fatalf("job %s: %d upload steps, want 1", j.ID, len(ups))
		}
		n, _ := gha.Get(ups[0].With, "name")
		p, _ := gha.Get(ups[0].With, "path")
		f, _ := gha.Get(ups[0].With, "if-no-files-found")
		if n != name || p != path || f != "error" || ups[0].If != cond {
			t.Errorf("job %s: upload %q from %q (if-no-files-found %q, if %q), want %q from %q, error, if %q",
				j.ID, n, p, f, ups[0].If, name, path, cond)
		}
	}
	cmds := func(j gha.Job) string {
		return strings.Join(gha.Workflow{Jobs: []gha.Job{j}}.Commands(), "\n")
	}

	rust := job(t, w, "rust")
	for _, e := range rust.Matrix {
		os, _ := gha.Get(e, "os")
		target, _ := gha.Get(e, "target")
		r := runner(t, os)
		if want := rustTarget[r.OS+"/"+r.Arch]; target != want {
			t.Errorf("rust: %s has target %s, want %s", os, target, want)
		}
	}
	if v, _ := gha.Get(rust.Env, "OEC_CI_TARGET"); v != "${{ matrix.target }}" {
		t.Errorf("rust: OEC_CI_TARGET = %q, want the matrix target", v)
	}
	if _, ok := gha.Get(rust.Env, "CC"); ok {
		t.Error("rust: no CC in the job env (it would select the compiler of aws-lc on Windows); sdk-rust.sh takes OEC_CI_CC")
	}
	if c := cmds(rust); !strings.HasSuffix(c, "oec/ci/setup-tools.sh --toolchain\noec/ci/sdk-rust.sh dist/sdk") {
		t.Errorf("rust job commands:\n%s", c)
	}
	upload(rust, "liboec_ffi-${{ matrix.target }}", "dist/sdk/liboec_ffi-${{ matrix.target }}", "")

	gojob := job(t, w, "go")
	if c := cmds(gojob); !strings.HasSuffix(c, "oec/ci/setup-tools.sh --toolchain\noec/ci/sdk-go.sh dist/sdk") {
		t.Errorf("go job commands:\n%s", c)
	}
	upload(gojob, "liboec_ffi-x86_64-pc-windows-gnu", "dist/sdk/liboec_ffi-x86_64-pc-windows-gnu", "runner.os == 'Windows'")
	if !strings.Contains(read(t, filepath.Join(oec, "ci/sdk-go.sh")), `target="x86_64-pc-windows-gnu"`) {
		t.Error("sdk-go.sh does not build the MinGW library x86_64-pc-windows-gnu on Windows")
	}

	armv7 := job(t, w, "armv7")
	if c := cmds(armv7); !strings.Contains(c, "oec/ci/packages.sh --armv7\n") ||
		!strings.HasSuffix(c, "oec/ci/sdk-armv7.sh dist/sdk") {
		t.Errorf("armv7 job commands:\n%s", c)
	}
	upload(armv7, "liboec_ffi-armv7-unknown-linux-gnueabihf", "dist/sdk/liboec_ffi-armv7-unknown-linux-gnueabihf", "")

	// The SDK jobs share no cache entry with the gate (whose entry also
	// holds cbindgen and cargo-deny).
	for _, j := range w.Jobs {
		for _, s := range j.Steps {
			if s.Uses != Cache {
				continue
			}
			if k, _ := gha.Get(s.With, "key"); !strings.HasPrefix(k, "oec-sdk-") {
				t.Errorf("job %s: cache key %s, want the prefix oec-sdk-", j.ID, k)
			}
		}
	}
}

// TestMicro: the micro SDK workflow of ADR-0032 §6 (C-106) – the host
// checks on Linux amd64/arm64 and Windows with GCC (MinGW), the ESP-IDF
// builds and the QEMU D7 run in the Espressif container pinned by digest,
// the Arduino builds with the pinned arduino-cli on both cores; download
// caches only, with their own prefix, and no artifacts (firmware).
func TestMicro(t *testing.T) {
	w := Micro()
	if w.File != "micro.yml" {
		t.Errorf("micro workflow file %s", w.File)
	}
	cmds := func(j gha.Job) string {
		return strings.Join(gha.Workflow{Jobs: []gha.Job{j}}.Commands(), "\n")
	}
	for _, j := range w.Jobs {
		if ups := uploads(j); len(ups) != 0 {
			t.Errorf("job %s uploads %d artifacts; micro.yml keeps no firmware", j.ID, len(ups))
		}
		for _, s := range j.Steps {
			if s.Uses != Cache {
				continue
			}
			if k, _ := gha.Get(s.With, "key"); !strings.HasPrefix(k, "oec-micro-") {
				t.Errorf("job %s: cache key %s, want the prefix oec-micro-", j.ID, k)
			}
			if p, _ := gha.Get(s.With, "path"); p != "~/.cargo/registry\n" {
				t.Errorf("job %s: cache path %q, want the crate downloads only", j.ID, p)
			}
		}
	}

	host := job(t, w, "host")
	for _, e := range host.Matrix {
		if cc, _ := gha.Get(e, "cc"); cc != "gcc" {
			os, _ := gha.Get(e, "os")
			t.Errorf("host: %s with cc %s, want gcc (the gate runs clang)", os, cc)
		}
	}
	if v, _ := gha.Get(host.Env, "OEC_CI_CC"); v != "${{ matrix.cc }}" {
		t.Errorf("host: OEC_CI_CC = %q, want the matrix cc", v)
	}
	if c := cmds(host); !strings.HasSuffix(c, "\noec/ci/micro-host.sh") {
		t.Errorf("host job commands:\n%s", c)
	}
	hostSh := read(t, filepath.Join(oec, "ci/micro-host.sh"))
	for _, s := range []string{"OECM_ANALYZER=clang", "sdk/micro/check.sh", "-DOECM_SANITIZE=OFF", "MinGW Makefiles", "go test -race"} {
		if !strings.Contains(hostSh, s) {
			t.Errorf("micro-host.sh lacks %q", s)
		}
	}
	if !strings.Contains(read(t, filepath.Join(oec, "sdk/micro/check.sh")), `"${OECM_ANALYZER:-${CC:-cc}}"`) {
		t.Error("sdk/micro/check.sh does not take the analyzer from OECM_ANALYZER (default CC)")
	}

	idf := job(t, w, "esp-idf")
	if idf.RunsOn != "ubuntu-24.04" {
		t.Errorf("esp-idf runs on %s, want ubuntu-24.04 (the amd64 image)", idf.RunsOn)
	}
	if c := cmds(idf); !strings.HasPrefix(c, "oec/ci/disk.sh\n") || !strings.HasSuffix(c, "\noec/ci/micro-esp-idf.sh") {
		t.Errorf("esp-idf job commands:\n%s", c)
	}
	idfSh := read(t, filepath.Join(oec, "ci/micro-esp-idf.sh"))
	for _, s := range []string{"$espressif_idf_image", "esp32 esp32s3 qemu", "OECM_QEMU_BUILD", "OECM_QEMU=",
		"TestQEMUD7Level1", "TestQEMUD7Level2", "TestQEMUD7Level3", "--init"} {
		if !strings.Contains(idfSh, s) {
			t.Errorf("micro-esp-idf.sh lacks %q", s)
		}
	}

	ard := job(t, w, "arduino")
	var cores []string
	for _, e := range ard.Matrix {
		c, _ := gha.Get(e, "core")
		cores = append(cores, c)
	}
	if strings.Join(cores, " ") != "3.3 4.0" {
		t.Errorf("arduino: cores %v, want 3.3 and 4.0", cores)
	}
	if v, _ := gha.Get(ard.Env, "OECM_ARDUINO_CORE"); v != "${{ matrix.core }}" {
		t.Errorf("arduino: OECM_ARDUINO_CORE = %q, want the matrix core", v)
	}
	if c := cmds(ard); !strings.HasPrefix(c, "oec/ci/disk.sh\n") || !strings.HasSuffix(c, "\noec/ci/micro-arduino.sh") {
		t.Errorf("arduino job commands:\n%s", c)
	}
	ardSh := read(t, filepath.Join(oec, "ci/micro-arduino.sh"))
	for _, s := range []string{"/arduino/package.sh", "sha256", "esp32:esp32:esp32s3", "esp32:esp32:esp32c3", "esp32:esp32:esp32c6"} {
		if !strings.Contains(ardSh, s) {
			t.Errorf("micro-arduino.sh lacks %q", s)
		}
	}

	versions := filepath.Join(oec, "ci/versions.sh")
	pins := map[string]string{
		"espressif_idf_image":             `^espressif/idf:v6\.1@sha256:[0-9a-f]{64}$`,
		"arduino_cli_version":             `^1\.5\.1$`,
		"arduino_esp32_3_3":               `^3\.3\.[0-9]+$`,
		"arduino_esp32_4_0":               `^4\.0\.0(-RC[0-9]+)?$`,
		"arduino_cli_sha256_linux_amd64":  `^[0-9a-f]{64}$`,
		"arduino_cli_sha256_linux_arm64":  `^[0-9a-f]{64}$`,
		"arduino_cli_sha256_darwin_arm64": `^[0-9a-f]{64}$`,
		"m5unified_version":               `^[0-9]+\.[0-9]+\.[0-9]+$`,
		"m5gfx_version":                   `^[0-9]+\.[0-9]+\.[0-9]+$`,
	}
	for name, re := range pins {
		if v := shVar(t, versions, name); !regexp.MustCompile(re).MatchString(v) {
			t.Errorf("versions.sh %s=%s does not match %s", name, v, re)
		}
	}
	// The ESP-IDF version of the image is the one of ADR-0030 §8.
	if !strings.Contains(read(t, filepath.Join(oec, "sdk/micro/README.md")), "ESP-IDF v6.1") {
		t.Error("sdk/micro/README.md no longer names ESP-IDF v6.1 (the image of versions.sh)")
	}
}

// TestImages: the image workflow of ADR-0032 §6 (C-107) – on Linux amd64
// and arm64 the multi-arch images of the demos, the registry and the
// compliance tools with D8 (images.sh), the newcomer run of "Getting
// started" (getting-started.sh), the live run of D7/D8 with the registry
// (live-infra.sh) and, on amd64, Podman (podman.sh); no image leaves the
// runner (Q-011): only the D8 reports and logs are uploaded, never an OCI
// archive, and nothing logs in to a registry or pushes.
func TestImages(t *testing.T) {
	w := Images()
	if w.File != "images.yml" {
		t.Errorf("images workflow file %s", w.File)
	}
	j := job(t, w, "images")
	var oses []string
	for _, os := range j.MatrixOS() {
		if r := runner(t, os); r.OS != "linux" {
			t.Errorf("images: runner %s is not Linux (no Linux containers elsewhere)", os)
		}
		oses = append(oses, os)
	}
	if strings.Join(oses, " ") != "ubuntu-24.04 ubuntu-24.04-arm" {
		t.Errorf("images: runners %v, want ubuntu-24.04 and ubuntu-24.04-arm", oses)
	}
	cmds := strings.Join(gha.Workflow{Jobs: []gha.Job{j}}.Commands(), "\n")
	want := "oec/ci/disk.sh\noec/ci/images.sh dist/images\noec/ci/getting-started.sh\noec/ci/live-infra.sh\noec/ci/podman.sh dist/images"
	if cmds != want {
		t.Errorf("images job commands:\n%s\nwant:\n%s", cmds, want)
	}
	for _, s := range j.Steps {
		if s.Run == "oec/ci/podman.sh dist/images" && s.If != "matrix.os == 'ubuntu-24.04'" {
			t.Errorf("podman step if %q, want the amd64 runner only", s.If)
		}
	}
	ups := uploads(j)
	if len(ups) != 1 {
		t.Fatalf("images: %d upload steps, want 1 (the reports)", len(ups))
	}
	if ups[0].If != "always()" {
		t.Errorf("reports upload if %q, want always()", ups[0].If)
	}
	if p, _ := gha.Get(ups[0].With, "path"); p != "dist/images\n!dist/images/*.oci.tar\n" {
		t.Errorf("reports upload path %q: the reports only, no OCI archive", p)
	}
	if n, _ := gha.Get(ups[0].With, "name"); n != "oec-images-reports-${{ matrix.os }}" {
		t.Errorf("reports artifact name %q", n)
	}
	for _, f := range []string{"ci/images.sh", "ci/getting-started.sh", "ci/live-infra.sh", "ci/podman.sh",
		"demos/images/build.sh", "registry/images/build.sh", "conformance/images/build.sh"} {
		s := read(t, filepath.Join(oec, f))
		if regexp.MustCompile(`(?m)^[^#]*\b(docker|podman|buildah|skopeo|oras)\s+(push|login)\b`).MatchString(s) ||
			regexp.MustCompile(`(?m)^[^#]*(push=true|--push\b|type=registry)`).MatchString(s) {
			t.Errorf("%s pushes or logs in to a registry (Q-011)", f)
		}
		if regexp.MustCompile(`(?m)^[^#]*\bprune\b`).MatchString(s) {
			t.Errorf("%s prunes", f)
		}
	}
	imagesSh := read(t, filepath.Join(oec, "ci/images.sh"))
	for _, s := range []string{"demos registry conformance", `rm -f "$out"/*.oci.tar`} {
		if !strings.Contains(imagesSh, s) {
			t.Errorf("images.sh lacks %q", s)
		}
	}
	gs := read(t, filepath.Join(oec, "ci/getting-started.sh"))
	// generated/ is reused: podman.sh runs it a second time on the runner;
	// the Set waits until the Config is writable (SubscriptionList, SF-107) (C-107).
	for _, s := range []string{"[ -f generated/.oec-dev-pki ]", `.context == \"weather\" and .writable`, "go run ./pki", "--env-file generated/demos.env --profile demos --profile registry",
		"up -d --wait", "down -v", "/api/set", "-station-4"} {
		if !strings.Contains(gs, s) {
			t.Errorf("getting-started.sh lacks %q", s)
		}
	}
	if !strings.Contains(read(t, filepath.Join(oec, "ci/live-infra.sh")), `OEC_LIVE_REGISTRY=1 "$oec/conformance/live.sh"`) {
		t.Error("live-infra.sh does not run live.sh with the registry")
	}
}

// TestSupplyChain: the supply chain workflow (ADR-0032 amendment C-108) –
// the job sbom makes the SBOM, license texts and license report of every
// deliverable with oec/ci/sbom.sh and uploads them as a workflow artifact
// (no release, Q-011); the job audit runs the vulnerability check
// oec/ci/audit.sh with network and uploads its outputs also on failure.
// Both on the hosted Linux runner, with the triggers of every workflow (no
// schedule yet: the weekly run comes with the first release, C-108 D-3 (a)).
func TestSupplyChain(t *testing.T) {
	w := SupplyChain()
	if w.File != "supply-chain.yml" {
		t.Errorf("supply chain workflow file %s", w.File)
	}
	var found bool
	for _, x := range Workflows() {
		found = found || x.File == w.File
	}
	if !found {
		t.Error("Workflows() lacks supply-chain.yml")
	}
	want := map[string]struct {
		cmds, name, path, cond string
	}{
		"sbom":  {"oec/ci/setup-tools.sh --toolchain\noec/ci/fetch.sh\noec/ci/sbom.sh dist/sbom", "oec-sbom", "dist/sbom", ""},
		"audit": {"oec/ci/setup-tools.sh\noec/ci/audit.sh dist/audit", "oec-audit", "dist/audit", "always()"},
	}
	if len(w.Jobs) != len(want) {
		t.Errorf("supply chain: %d jobs, want %d", len(w.Jobs), len(want))
	}
	for id, x := range want {
		j := job(t, w, id)
		if j.RunsOn != "ubuntu-24.04" {
			t.Errorf("%s: runs on %s, want ubuntu-24.04", id, j.RunsOn)
		}
		if cmds := strings.Join(gha.Workflow{Jobs: []gha.Job{j}}.Commands(), "\n"); cmds != x.cmds {
			t.Errorf("%s job commands:\n%s\nwant:\n%s", id, cmds, x.cmds)
		}
		ups := uploads(j)
		if len(ups) != 1 {
			t.Fatalf("%s: %d upload steps, want 1", id, len(ups))
		}
		if n, _ := gha.Get(ups[0].With, "name"); n != x.name {
			t.Errorf("%s: artifact %q, want %q", id, n, x.name)
		}
		if p, _ := gha.Get(ups[0].With, "path"); p != x.path {
			t.Errorf("%s: artifact path %q, want %q", id, p, x.path)
		}
		if ups[0].If != x.cond {
			t.Errorf("%s: upload if %q, want %q", id, ups[0].If, x.cond)
		}
		if j.Steps[len(j.Steps)-1].Uses != UploadArtifact {
			t.Errorf("%s: the upload is not the last step", id)
		}
	}
	// sbom.sh: every deliverable of ROADMAP P9 increment 5, nothing built
	// or pushed; audit.sh: the three checks, govulncheck as a pinned tool.
	sb := read(t, filepath.Join(oec, "ci/sbom.sh"))
	for _, d := range []string{"oec-rust-sdk", "oec-ffi", "oec-go-sdk", "oec-micro-sdk", "oec-check", "oec-infra",
		"oec-logger", "oec-weather", "oec-config", "oec-registry", "oec-schema", "-npm", "-report"} {
		if !strings.Contains(sb, d) {
			t.Errorf("sbom.sh lacks %s", d)
		}
	}
	au := read(t, filepath.Join(oec, "ci/audit.sh"))
	for _, s := range []string{`golang.org/x/vuln/cmd/govulncheck@v$3`, "$govulncheck_version", "check advisories", "npm audit"} {
		if !strings.Contains(au, s) {
			t.Errorf("audit.sh lacks %s", s)
		}
	}
	if strings.Contains(au, "--frozen") || strings.Contains(au, "--disable-fetch") {
		t.Error("audit.sh must check with the fresh advisory database")
	}
	for _, s := range []string{sb, au} {
		if regexp.MustCompile(`(?m)^[^#]*\b(push|login|publish)\b`).MatchString(s) {
			t.Error("a supply chain script publishes (Q-011)")
		}
	}
}
