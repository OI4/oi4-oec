package pipelines

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OI4/oec/ci/gha"
)

// TestPublish: the publishing step of release.yml (ADR-0034, C-110). Only
// the job publish, on a tag and behind the environment release (the PO
// approves each run), gets the write token; it publishes the assembled
// release of the same run with oec/ci/publish.sh --publish. Every release
// run checks the publication with a dry run (publish.sh without --publish).
func TestPublish(t *testing.T) {
	w := Release()
	p := job(t, w, "publish")
	if strings.Join(p.Needs, " ") != "release" || p.If != "github.ref_type == 'tag'" || p.Environment != ReleaseEnvironment || p.RunsOn != "ubuntu-24.04" {
		t.Errorf("publish: needs %v, if %q, environment %q, runs-on %s", p.Needs, p.If, p.Environment, p.RunsOn)
	}
	if ReleaseEnvironment != "release" || ReleaseSecret != "OEC_RELEASE_TOKEN" {
		t.Errorf("environment %s, secret %s", ReleaseEnvironment, ReleaseSecret)
	}
	if use := Policy().Secrets[ReleaseSecret]; use != (gha.SecretUse{Environment: ReleaseEnvironment, EnvKey: ReleaseSecret}) {
		t.Errorf("policy of %s: %v", ReleaseSecret, use)
	}
	if cmds := strings.Join(gha.Workflow{Jobs: []gha.Job{p}}.Commands(), "\n"); cmds != "oec/ci/publish.sh dist/release --publish" {
		t.Errorf("publish commands:\n%s", cmds)
	}
	var download, publish *gha.Step
	for i, s := range p.Steps {
		switch {
		case s.Uses == DownloadArtifact:
			download = &p.Steps[i]
		case s.Run != "":
			publish = &p.Steps[i]
		case s.Uses == Cache || s.Uses == UploadArtifact:
			t.Errorf("publish: step %s", s.Name)
		}
	}
	if download == nil || publish == nil {
		t.Fatal("publish: no download of the release or no publish step")
	}
	for k, v := range map[string]string{"name": "oec-release", "path": "dist/release"} {
		if got, _ := gha.Get(download.With, k); got != v {
			t.Errorf("publish: download %s %q, want %q", k, got, v)
		}
	}
	if _, ok := gha.Get(download.With, "run-id"); ok {
		t.Error("publish: a release of another run")
	}
	if want := []gha.KV{
		{Key: "OEC_RELEASE_TOKEN", Value: "${{ secrets.OEC_RELEASE_TOKEN }}"},
		{Key: "OEC_RELEASE_USER", Value: "${{ vars.OEC_RELEASE_USER }}"},
	}; len(publish.Env) != len(want) || publish.Env[0] != want[0] || publish.Env[1] != want[1] {
		t.Errorf("publish env %v, want %v", publish.Env, want)
	}
	// No other job anywhere has an environment or the token.
	for _, x := range Workflows() {
		for _, j := range x.Jobs {
			if j.Environment != "" && (x.File != w.File || j.ID != "publish") {
				t.Errorf("%s job %s: environment %s", x.File, j.ID, j.Environment)
			}
		}
		if y, err := gha.Render(x); err != nil || (strings.Contains(string(y), ReleaseSecret) && x.File != w.File) {
			t.Errorf("%s: %v or uses %s", x.File, err, ReleaseSecret)
		}
	}

	// publish.sh: checks the release, prepares the release repository (the
	// Go modules without replace, go.sum complete, go install from the
	// tags), the images and the GitHub Release; every command that writes
	// to GitHub or GHCR goes through remote, which only --publish on a tag
	// build of the version runs.
	path := filepath.Join(oec, "ci/publish.sh")
	script := read(t, path)
	for _, s := range []string{"SHA256SUMS", "RELEASE-NOTES.md", "-source.tar.gz", "-dropreplace", "go mod tidy", "go mod verify",
		"go install", "GOPRIVATE", "insteadOf", "oci-archive:", "skopeo inspect", "gh release create", "--verify-tag",
		`"$GITHUB_REF_TYPE" != tag`, "oec-v$version", "OEC_RELEASE_TOKEN", "--authfile", "http.https://github.com/.extraheader"} {
		if !strings.Contains(script, s) {
			t.Errorf("publish.sh lacks %s", s)
		}
	}
	if got := shVar(t, path, "repo"); got != "OI4/oec" {
		t.Errorf("publish.sh repo %q, want OI4/oec (C-109 D-3 (a))", got)
	}
	if got := shVar(t, path, "registry"); got != "ghcr.io/oi4" {
		t.Errorf("publish.sh registry %q, want ghcr.io/oi4", got)
	}
	if got, want := shVar(t, path, "images"), shVar(t, filepath.Join(oec, "ci/release.sh"), "images"); got != want {
		t.Errorf("publish.sh images %q, release.sh %q", got, want)
	}
	writes := regexp.MustCompile(`\bgit\b.*\bpush\b|docker://|\bgh\s+release\s+(create|upload|edit|delete)\b|\bgh\s+api\b`)
	remote := regexp.MustCompile(`^\s*remote\s`)
	n := 0
	for i, l := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "#") || !writes.MatchString(l) {
			continue
		}
		n++
		if !remote.MatchString(l) {
			t.Errorf("publish.sh:%d writes without remote: %s", i+1, strings.TrimSpace(l))
		}
	}
	if n < 3 {
		t.Errorf("publish.sh: %d remote writes, want the push, the images and the release", n)
	}
	if regexp.MustCompile(`(?m)^[^#]*(\bset\s+-[a-z]*x|\becho\b.*\$\{?OEC_RELEASE_TOKEN)`).MatchString(script) {
		t.Error("publish.sh may print the token")
	}
}
