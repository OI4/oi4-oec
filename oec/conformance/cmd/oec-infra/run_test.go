package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/run/runtest"
	"github.com/OI4/oec/conformance/internal/run/simapp"
)

type runReport struct {
	Mode  string `json:"mode"`
	Input string `json:"input"`
	Exit  int    `json:"exit"`
	Run   struct {
		Engine struct {
			Endpoint string `json:"endpoint"`
		} `json:"engine"`
		Image struct {
			Ref  string `json:"ref"`
			Arch string `json:"arch"`
		} `json:"image"`
		Endpoint struct {
			Listen        string `json:"listen"`
			BrokerHost    string `json:"broker_host"`
			MaxPacketSize int    `json:"max_packet_size"`
		} `json:"endpoint"`
		Phases []struct {
			Name    string `json:"name"`
			Removed bool   `json:"removed"`
		} `json:"phases"`
	} `json:"run"`
	Tests []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
	} `json:"tests"`
}

func runOutcomes(t *testing.T, out string) (runReport, map[string]string) {
	t.Helper()
	var r runReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := map[string]string{}
	for _, tc := range r.Tests {
		got[tc.ID] = tc.Outcome
	}
	return r, got
}

var sandbox = []string{"--listen", "127.0.0.1", "--wait", "3s", "--settle", "300ms"}

// TestRunCommand: the good test image passes in the fake engine; every
// sandbox container is removed and the report validates.
func TestRunCommand(t *testing.T) {
	s := enginetest.Start(t, runtest.Engine(t))
	ref := runtest.Ref(simapp.Good)
	rep := filepath.Join(t.TempDir(), "run.json")
	args := append(append([]string{"run", "--daemon", s.Endpoint(), "--format", "json", "--report", "json=" + rep, "--max-packet-size", "100000"}, sandbox...), ref)
	code, out, errOut := call(t, args...)
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	r, got := runOutcomes(t, out)
	if r.Mode != "run" || r.Input != ref || r.Run.Engine.Endpoint != s.Endpoint() || r.Run.Image.Ref != ref || r.Run.Image.Arch != "arm64" ||
		r.Run.Endpoint.Listen != "127.0.0.1" || r.Run.Endpoint.BrokerHost != "oec-d8-broker" || r.Run.Endpoint.MaxPacketSize != 100000 || len(r.Run.Phases) != 3 {
		t.Errorf("report %+v", r)
	}
	for _, p := range r.Run.Phases {
		if !p.Removed {
			t.Errorf("phase %s not removed", p.Name)
		}
	}
	if got["T8-RUN-01"] != "pass" || got["T8-RUN-02"] != "pass" || got["T8-RUN-03"] != "pass" || len(got) != 3 {
		t.Errorf("tests %v", got)
	}
	if b, err := os.ReadFile(rep); err != nil || !strings.Contains(string(b), `"mode": "run"`) {
		t.Errorf("--report: %v", err)
	}
	code, out, _ = call(t, append(append([]string{"run", "--daemon", s.Endpoint()}, sandbox...), ref)...)
	if code != exitPass || !strings.Contains(out, "phase credentials") {
		t.Errorf("text: exit %d\n%s", code, out)
	}
}

// TestRunCommandFail: a faulty image fails with exit code 1; a selection
// runs only its tests.
func TestRunCommandFail(t *testing.T) {
	s := enginetest.Start(t, runtest.Engine(t))
	code, out, errOut := call(t, append(append([]string{"run", "--daemon", s.Endpoint(), "--format", "json", "--tests", "T8-RUN-03"}, sandbox...), runtest.Ref(simapp.SerialOther))...)
	if code != 1 {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	r, got := runOutcomes(t, out)
	if got["T8-RUN-03"] != "fail" || len(got) != 1 || len(r.Run.Phases) != 2 {
		t.Errorf("tests %v, %d phases", got, len(r.Run.Phases))
	}
}

func TestRunUsage(t *testing.T) {
	s := enginetest.Start(t, runtest.Engine(t))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"run"}, "exactly one image"},
		{[]string{"run", "a", "b"}, "exactly one image"},
		{[]string{"run", "--level", "4", "x"}, "--level"},
		{[]string{"run", "--format", "xml", "x"}, "--format"},
		{[]string{"run", "--wait", "0s", "x"}, "positive"},
		{[]string{"run", "--max-packet-size", "0", "x"}, "--max-packet-size"},
		{[]string{"run", "--broker-host", "", "x"}, "--broker-host"},
		{[]string{"run", "--tests", "T8-CTR-*", "x"}, "selects no run test"},
		{[]string{"run", "--daemon", s.Endpoint(), "oec-d8-sim:none"}, "no such image"},
		{[]string{"run", "--daemon", "tcp://192.0.2.1:2375", "x"}, "remote"},
	} {
		code, _, errOut := call(t, tc.args...)
		if code != exitUsage || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: exit %d, %s", tc.args, code, errOut)
		}
	}
}
