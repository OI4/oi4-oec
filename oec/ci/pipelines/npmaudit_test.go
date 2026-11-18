package pipelines

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNpmAudit: ci/npm-audit.mjs judges the JSON of npm audit against an
// ignore list (one advisory id and its reason per line, like the ignore
// entries of deny.toml, ADR-0023 §2): an advisory that is not listed fails,
// a listed one passes with its reason, a listed one that is no longer
// reported only warns, and an unreadable report or a list entry without a
// reason fails.
func TestNpmAudit(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is missing")
	}
	const report = `{"auditReportVersion":2,"vulnerabilities":{
		"picomatch":{"name":"picomatch","severity":"high","via":[{"source":1,"name":"picomatch","title":"ReDoS","url":"https://github.com/advisories/GHSA-fc5q-39xc-vjpf","severity":"high","range":"<=4.0.7"}]},
		"fdir":{"name":"fdir","severity":"high","via":["picomatch"]}}}`
	dir := t.TempDir()
	run := func(stdin, list string) (string, int) {
		t.Helper()
		f := filepath.Join(dir, "ignore.txt")
		if err := os.WriteFile(f, []byte(list), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", filepath.Join(oec, "ci/npm-audit.mjs"), f)
		cmd.Stdin = strings.NewReader(stdin)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		code := 0
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return out.String(), code
	}
	for name, c := range map[string]struct {
		stdin, list string
		code        int
		want        string
	}{
		"not accepted": {report, "# none\n", 1, "GHSA-fc5q-39xc-vjpf high picomatch <=4.0.7: ReDoS – NOT ACCEPTED"},
		"accepted":     {report, "# build tool only\nGHSA-fc5q-39xc-vjpf build time only, no fix\n", 0, "accepted: build time only, no fix"},
		"stale":        {`{"auditReportVersion":2,"vulnerabilities":{}}`, "GHSA-fc5q-39xc-vjpf build time only\n", 0, "warning: GHSA-fc5q-39xc-vjpf is no longer reported"},
		"no reason":    {report, "GHSA-fc5q-39xc-vjpf\n", 2, "needs an advisory id and a reason"},
		"bad id":       {report, "CVE-2026-1 reason\n", 2, "needs an advisory id and a reason"},
		"npm error":    {`{"error":{"code":"ENOLOCK","summary":"no lockfile"}}`, "", 2, "no audit report"},
		"not json":     {"npm ERR! network", "", 2, "no audit report"},
	} {
		t.Run(name, func(t *testing.T) {
			out, code := run(c.stdin, c.list)
			if code != c.code || !strings.Contains(out, c.want) {
				t.Fatalf("exit %d, want %d; output does not contain %q:\n%s", code, c.code, c.want, out)
			}
		})
	}
}
