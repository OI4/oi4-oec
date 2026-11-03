package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/host"
	"github.com/OI4/oec/conformance/internal/host/hosttest"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
)

type hostReport struct {
	Mode  string `json:"mode"`
	Input string `json:"input"`
	Exit  int    `json:"exit"`
	Host  struct {
		Container string `json:"container"`
		Storages  []struct {
			Name  string `json:"name"`
			Mount string `json:"mount"`
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		} `json:"storages"`
		Platform *struct {
			OS      string `json:"os"`
			Machine string `json:"machine"`
		} `json:"platform"`
		Engine *struct {
			Endpoint string `json:"endpoint"`
			Product  string `json:"product"`
		} `json:"engine"`
		Containers []struct {
			Name string `json:"name"`
		} `json:"containers"`
	} `json:"host"`
	Tests []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
		Reqs    []struct {
			Req     string `json:"req"`
			Outcome string `json:"outcome"`
		} `json:"reqs"`
	} `json:"tests"`
}

// The fixture storages are plain directories, no mount points: measured
// facts of this system, but D8 not in a container, so that the tests also
// pass when they run in a container (oec/ci/linux.sh). TestHostInContainer
// sets InContainer.
func init() {
	hostFacts = func() *host.Facts {
		f := host.Local()
		f.InContainer = false
		return f
	}
}

func hostRoot(t *testing.T) hosttest.Root {
	t.Helper()
	return hosttest.New(t, mqtttest.NewPKI(t), hosttest.Options{Port: 8883, Credentials: "app:app-secret", Certificate: true})
}

func outcomes(t *testing.T, out string) (hostReport, map[string]string) {
	t.Helper()
	var r hostReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := map[string]string{}
	for _, tc := range r.Tests {
		got[tc.ID] = tc.Outcome
	}
	return r, got
}

// TestHostRun: the storages of a conforming root pass offline; the report
// lists the container and the storages (the platform only with T8-HST-04).
func TestHostRun(t *testing.T) {
	root := hostRoot(t)
	rep := filepath.Join(t.TempDir(), "host.json")
	code, out, errOut := call(t, "host", "--root", root.Dir, "--offline", "--tests", "T8-HST-0[12]", "--format", "json", "--report", "json="+rep)
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	r, got := outcomes(t, out)
	if r.Mode != "host" || r.Input != root.Dir || r.Host.Container != hosttest.Container || len(r.Host.Storages) != 3 || r.Host.Platform != nil {
		t.Errorf("report %+v", r)
	}
	if got["T8-HST-01"] != "pass" || got["T8-HST-02"] != "pass" || len(got) != 2 {
		t.Errorf("tests %v", got)
	}
	for _, s := range r.Host.Storages {
		for _, f := range s.Files {
			if f.Name == "" {
				t.Errorf("storage %s: file without name", s.Name)
			}
		}
	}
	if b, err := os.ReadFile(rep); err != nil || !strings.Contains(string(b), `"mode": "host"`) {
		t.Errorf("--report: %v", err)
	}
	// Text output: no secret of the storages.
	code, out, _ = call(t, "host", "--root", root.Dir, "--offline", "--tests", "T8-HST-0[12]")
	if code != exitPass || !strings.Contains(out, "T8-HST-02") || strings.Contains(out, "app-secret") {
		t.Errorf("text: exit %d\n%s", code, out)
	}
}

// TestHostAll runs all host tests offline: T8-HST-04 depends on this host,
// REQ-PLT-007/008 are inconclusive offline, T8-HST-03 is inconclusive
// without an Engine API at --daemon.
func TestHostAll(t *testing.T) {
	root := hostRoot(t)
	code, out, errOut := call(t, "host", "--root", root.Dir, "--offline", "--daemon", "tcp://127.0.0.1:1", "--format", "json")
	r, got := outcomes(t, out)
	if got["T8-HST-01"] != "pass" || got["T8-HST-02"] != "pass" || got["T8-HST-03"] != "skip" || got["T8-HST-04"] == "" || got["T8-HST-04"] == "pass" {
		t.Errorf("exit %d, tests %v\n%s", code, got, errOut)
	}
	if code == exitPass {
		t.Errorf("exit %d: offline must not pass T8-HST-04", code)
	}
	if r.Host.Platform == nil || r.Host.Platform.OS != runtime.GOOS || r.Host.Platform.Machine == "" {
		t.Errorf("platform %+v", r.Host.Platform)
	}
}

// TestHostInContainer: in a container, a storage that is no mount point
// fails its storage REQ (RIG 4.1: the platform maps a host-defined path to
// it); on Linux only, where the mount table is read.
func TestHostInContainer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("mount table: Linux only")
	}
	saved := hostFacts
	t.Cleanup(func() { hostFacts = saved })
	hostFacts = func() *host.Facts {
		f := saved()
		f.InContainer = true
		return f
	}
	root := hostRoot(t)
	code, out, errOut := call(t, "host", "--root", root.Dir, "--offline", "--tests", "T8-HST-0[12]", "--format", "json")
	r, got := outcomes(t, out)
	if code != 1 || got["T8-HST-01"] != "fail" || got["T8-HST-02"] != "fail" {
		t.Errorf("exit %d, tests %v\n%s", code, got, errOut)
	}
	failed := map[string]bool{}
	for _, tc := range r.Tests {
		for _, q := range tc.Reqs {
			if q.Outcome == "fail" {
				failed[q.Req] = true
			}
		}
	}
	for _, req := range []string{"REQ-CNT-001", "REQ-CNT-006", "REQ-CNT-008"} {
		if !failed[req] {
			t.Errorf("%s not failed: %v", req, failed)
		}
	}
	for _, s := range r.Host.Storages {
		if s.Mount != "none" {
			t.Errorf("storage %s: mount %q", s.Name, s.Mount)
		}
	}
}

// TestHostFail: a root without broker.json fails T8-HST-01 (exit 1).
func TestHostFail(t *testing.T) {
	root := hostRoot(t)
	hosttest.Remove(t, filepath.Join(root.MQTT, "broker.json"))
	code, out, errOut := call(t, "host", "--root", root.Dir, "--offline", "--tests", "T8-HST-01", "--format", "json")
	if _, got := outcomes(t, out); code != 1 || got["T8-HST-01"] != "fail" {
		t.Errorf("exit %d, tests %v\n%s", code, got, errOut)
	}
}

// TestHostStorageFlags: the storages can be given one by one; the
// container name overrides etc/hostname (another name: no <container>.pem,
// REQ-CNT-007 SHOULD → warn).
func TestHostStorageFlags(t *testing.T) {
	root := hostRoot(t)
	empty := t.TempDir()
	code, out, errOut := call(t, "host", "--root", empty, "--mqtt-storage", root.MQTT, "--certs-storage", root.Certs,
		"--secrets-storage", root.Secrets, "--container", hosttest.Container, "--offline", "--tests", "T8-HST-0[12]", "--format", "json")
	if r, got := outcomes(t, out); code != exitPass || r.Host.Container != hosttest.Container {
		t.Errorf("exit %d, tests %v\n%s", code, got, errOut)
	}
	code, out, _ = call(t, "host", "--root", root.Dir, "--container", "other", "--offline", "--tests", "T8-HST-02", "--format", "json")
	if _, got := outcomes(t, out); code != exitPass || got["T8-HST-02"] != "warn" {
		t.Errorf("--container other: exit %d, tests %v", code, got)
	}
}

// TestHostConfig: flag values from a YAML file.
func TestHostConfig(t *testing.T) {
	root := hostRoot(t)
	cfg := filepath.Join(t.TempDir(), "host.yaml")
	if err := os.WriteFile(cfg, []byte("root: "+root.Dir+"\noffline: true\ntests: [T8-HST-01, T8-HST-02]\nformat: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := call(t, "host", "--config", cfg)
	if _, got := outcomes(t, out); code != exitPass || len(got) != 2 {
		t.Errorf("exit %d, tests %v\n%s", code, got, errOut)
	}
}

func TestHostUsage(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(unknown, []byte("broker: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"host", "x"}, "no arguments"},
		{[]string{"host", "--root", filepath.Join(dir, "missing")}, "no directory"},
		{[]string{"host", "--root", file}, "no directory"},
		{[]string{"host", "--root", dir, "--tests", "T8-BUS-*"}, "selects no host test"},
		{[]string{"host", "--root", dir, "--timeout", "0s"}, "positive"},
		{[]string{"host", "--root", dir, "--registry-wait", "-1s"}, "positive"},
		{[]string{"host", "--root", dir, "--format", "pdf"}, "--format"},
		{[]string{"host", "--config", unknown}, `unknown key "broker"`},
	} {
		code, out, errOut := call(t, c.args...)
		if code != exitUsage || !strings.Contains(out+errOut, c.msg) {
			t.Errorf("%v: exit %d, want %d with %q\n%s%s", c.args, code, exitUsage, c.msg, out, errOut)
		}
	}
	if code, out, _ := call(t, "help"); code != exitPass || !strings.Contains(out, "oec-infra host") {
		t.Errorf("help: %d", code)
	}
}

// TestHostDaemon: a container daemon at a unix socket given with --daemon
// is reported (no Docker daemon needed: any listener answers the dial).
func TestHostDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets: see internal/host TestDialDaemon")
	}
	dir, err := os.MkdirTemp("", "oec-d") // short path: sun_path limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	root := hostRoot(t)
	_, out, errOut := call(t, "host", "--root", root.Dir, "--offline", "--daemon", sock, "--tests", "T8-HST-04", "--format", "json")
	r, _ := outcomes(t, out)
	if !strings.Contains(out, "unix://"+sock) {
		t.Errorf("daemon not reported\n%s%s", out, errOut)
	}
	for _, tc := range r.Tests {
		for _, q := range tc.Reqs {
			if q.Req == "REQ-PLT-006" && q.Outcome != "pass" {
				t.Errorf("REQ-PLT-006 %s", q.Outcome)
			}
		}
	}
}

// TestHostStorages: T8-HST-03 through the fake Engine API – App and
// Sidecar pass, App and Stopped share a volume (exit 1), --self naming an
// unknown container is inconclusive (exit 4).
func TestHostStorages(t *testing.T) {
	root := hostRoot(t)
	rec := enginetest.Recorded(t)
	cases := []struct {
		name       string
		containers []enginetest.Doc
		args       []string
		exit       int
		outcome    string
	}{
		{"pass", []enginetest.Doc{rec.App, rec.Sidecar}, nil, exitPass, "pass"},
		{"shared", []enginetest.Doc{rec.App, rec.Stopped}, nil, 1, "fail"},
		{"self", []enginetest.Doc{rec.App, rec.Sidecar}, []string{"--self", "oec-d8"}, exitInconclusive, "skip"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := enginetest.Start(t, rec.Engine(tc.containers...))
			args := append([]string{"host", "--root", root.Dir, "--offline", "--daemon", srv.Endpoint(), "--tests", "T8-HST-03", "--format", "json"}, tc.args...)
			code, out, errOut := call(t, args...)
			r, got := outcomes(t, out)
			if code != tc.exit || got["T8-HST-03"] != tc.outcome || len(got) != 1 {
				t.Errorf("exit %d, tests %v\n%s%s", code, got, out, errOut)
			}
			if r.Host.Engine == nil || r.Host.Engine.Endpoint != srv.Endpoint() {
				t.Errorf("engine %+v", r.Host.Engine)
			}
			if tc.name != "self" && (len(r.Host.Containers) == 0 || strings.Contains(out, "/var/lib/docker") || strings.Contains(out, "/srv/oec")) {
				t.Errorf("containers %+v or a mount source in the report\n%s", r.Host.Containers, out)
			}
		})
	}
}
