package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/engine/enginetest"
)

type containerReport struct {
	Mode      string `json:"mode"`
	Input     string `json:"input"`
	Level     int    `json:"level"`
	Exit      int    `json:"exit"`
	Container struct {
		Engine struct {
			Endpoint string `json:"endpoint"`
			Product  string `json:"product"`
		} `json:"engine"`
		Containers []struct {
			Name     string `json:"name"`
			Hostname string `json:"hostname"`
		} `json:"containers"`
	} `json:"container"`
	Tests []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
	} `json:"tests"`
}

// fakeEngine serves the recorded containers (no Docker daemon).
func fakeEngine(t *testing.T) (*enginetest.Server, enginetest.Recording) {
	t.Helper()
	rec := enginetest.Recorded(t)
	return enginetest.Start(t, rec.Engine(rec.App, rec.Host, rec.Stopped, rec.Sidecar)), rec
}

func containerOutcomes(t *testing.T, out string) (containerReport, map[string]string) {
	t.Helper()
	var r containerReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := map[string]string{}
	for _, tc := range r.Tests {
		got[tc.ID] = tc.Outcome
	}
	return r, got
}

func recName(d enginetest.Doc) string {
	return strings.TrimPrefix(enginetest.Get(d, "Name").(string), "/")
}

// TestContainerRun: the recorded application container passes; the report
// names the engine and the container and contains no mount source.
func TestContainerRun(t *testing.T) {
	s, rec := fakeEngine(t)
	app := recName(rec.App)
	rep := filepath.Join(t.TempDir(), "container.json")
	code, out, errOut := call(t, "container", "--daemon", s.Endpoint(), "--format", "json", "--report", "json="+rep, app)
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	r, got := containerOutcomes(t, out)
	if r.Mode != "container" || r.Input != app || r.Level != 3 || r.Container.Engine.Endpoint != s.Endpoint() ||
		len(r.Container.Containers) != 1 || r.Container.Containers[0].Hostname != app {
		t.Errorf("report %+v", r)
	}
	if got["T8-CTR-01"] != "pass" || got["T8-CTR-02"] != "pass" || got["T8-CTR-03"] != "pass" || len(got) != 3 {
		t.Errorf("tests %v", got)
	}
	if strings.Contains(out, "/srv/oec") {
		t.Error("the report contains a mount source")
	}
	if b, err := os.ReadFile(rep); err != nil || !strings.Contains(string(b), `"mode": "container"`) {
		t.Errorf("--report: %v", err)
	}
	code, out, _ = call(t, "container", "--daemon", s.Endpoint(), app)
	if code != exitPass || !strings.Contains(out, "T8-CTR-02") || !strings.Contains(out, app) {
		t.Errorf("text: exit %d\n%s", code, out)
	}
	for _, q := range s.Requests() {
		if strings.Contains(q, "/start") || strings.Contains(q, "/exec") {
			t.Errorf("request %s", q)
		}
	}
}

// TestContainerFail: the recorded container on the host network with a
// bind mount on /opt/oi4/licenses fails; two containers in one run; at
// level 1 T8-CTR-01 is skipped.
func TestContainerFail(t *testing.T) {
	s, rec := fakeEngine(t)
	host, app := recName(rec.Host), recName(rec.App)
	code, out, errOut := call(t, "container", "--daemon", s.Endpoint(), "--format", "json", app, host)
	r, got := containerOutcomes(t, out)
	if code != 1 || got["T8-CTR-01"] != "warn" || got["T8-CTR-02"] != "fail" || got["T8-CTR-03"] != "fail" ||
		r.Input != app+", "+host || len(r.Container.Containers) != 2 {
		t.Errorf("exit %d, tests %v, report %+v\n%s", code, got, r, errOut)
	}
	code, out, _ = call(t, "container", "--daemon", s.Endpoint(), "--level", "1", "--format", "json", host)
	if _, got := containerOutcomes(t, out); code != 1 || got["T8-CTR-01"] != "skip" {
		t.Errorf("--level 1: exit %d, tests %v", code, got)
	}
}

// TestContainerNoEngine: without an Engine API every requirement is
// inconclusive (exit 4, 0 with --allow-skip).
func TestContainerNoEngine(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.sock")
	code, out, errOut := call(t, "container", "--daemon", missing, "--format", "json", "app")
	r, got := containerOutcomes(t, out)
	if code != exitInconclusive || got["T8-CTR-02"] != "skip" || r.Container.Engine.Endpoint != "unix://"+filepath.ToSlash(missing) && !strings.HasSuffix(r.Container.Engine.Endpoint, "missing.sock") {
		t.Errorf("exit %d, tests %v, engine %+v\n%s", code, got, r.Container.Engine, errOut)
	}
	if code, _, _ := call(t, "container", "--daemon", missing, "--allow-skip", "app"); code != exitPass {
		t.Errorf("--allow-skip: exit %d", code)
	}
}

func TestContainerUsage(t *testing.T) {
	s, _ := fakeEngine(t)
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(unknown, []byte("root: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"container"}, "one or more container"},
		{[]string{"container", "--daemon", s.Endpoint(), "oec-no-such-container"}, "no such container"},
		{[]string{"container", "--level", "0", "x"}, "--level"},
		{[]string{"container", "--tests", "T8-HST-*", "x"}, "selects no container test"},
		{[]string{"container", "--timeout", "0s", "x"}, "positive"},
		{[]string{"container", "--format", "pdf", "x"}, "--format"},
		{[]string{"container", "--config", unknown, "x"}, `unknown key "root"`},
	} {
		code, out, errOut := call(t, c.args...)
		if code != exitUsage || !strings.Contains(out+errOut, c.msg) {
			t.Errorf("%v: exit %d, want %d with %q\n%s%s", c.args, code, exitUsage, c.msg, out, errOut)
		}
	}
	if code, out, _ := call(t, "help"); code != exitPass || !strings.Contains(out, "oec-infra container") {
		t.Errorf("help: %d", code)
	}
}
