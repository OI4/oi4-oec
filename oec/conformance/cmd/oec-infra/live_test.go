package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance"
)

// TestBusDevBroker is the opt-in live run of `oec-infra bus` against the
// Mosquitto of oec/dev (oec/conformance/live.sh sets OEC_LIVE=1 and
// OEC_DEV_DIR). Port 8883: the probe logs in with the probe credentials, the
// unprivileged client with the credentials of the sample storage
// oec-dev-app. Port 8884 (client certificate required): the probe presents
// its client certificate, the unprivileged client is the sample storage
// oec-dev-cert. The dev ACL cannot bind the AppId level to a user and the
// applications read their own Set/Config, so REQ-RES-CONFIG-026 warns; there
// is no OEC Registry before P7, so T8-BUS-03 skips (C-071).
func TestBusDevBroker(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev := os.Getenv("OEC_DEV_DIR")
	if dev == "" {
		t.Fatal("OEC_DEV_DIR not set")
	}
	gen := filepath.Join(dev, "generated")
	probe := filepath.Join(gen, "probe")
	runs := []struct {
		name string
		args []string
		exit int
		want map[string]string
	}{
		{"8883", []string{"--broker", "mqtts://localhost:8883", "--bus-storage", filepath.Join(gen, "roots", "oec-dev-app"),
			"--username", "oec-probe", "--password-file", filepath.Join(probe, "password"),
			"--credentials-2", filepath.Join(gen, "roots", "oec-dev-app", "run", "secrets", "oi4", "credentials"), "--registry-wait", "5s"},
			exitInconclusive, map[string]string{"T8-BUS-01": "pass", "T8-BUS-02": "warn", "T8-BUS-03": "skip"}},
		{"8884", []string{"--broker", "mqtts://localhost:8884", "--ca", filepath.Join(gen, "broker", "ca.pem"),
			"--cert", filepath.Join(probe, "probe.pem"), "--key", filepath.Join(probe, "probe.key"),
			"--bus-storage-2", filepath.Join(gen, "roots", "oec-dev-cert"), "--max-packet-size", "262144", "--tests", "T8-BUS-0[12]"},
			exitPass, map[string]string{"T8-BUS-01": "pass", "T8-BUS-02": "warn"}},
	}
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			code, out, errOut := call(t, append(append([]string{"bus"}, r.args...), "--format", "json")...)
			t.Logf("%s%s", out, errOut)
			if code != r.exit {
				t.Errorf("exit %d, want %d", code, r.exit)
			}
			var rep busReport
			if err := json.Unmarshal([]byte(out), &rep); err != nil {
				t.Fatal(err)
			}
			if strings.Join(rep.Broker.TLS, ",") != "1.2,1.3" || rep.Broker.MaxPacketSize != 262144 {
				t.Errorf("broker %+v", rep.Broker)
			}
			got := map[string]string{}
			for _, tc := range rep.Tests {
				got[tc.ID] = tc.Outcome
			}
			for id, want := range r.want {
				if got[id] != want {
					t.Errorf("%s: %s, want %s", id, got[id], want)
				}
			}
			if len(got) != len(r.want) {
				t.Errorf("tests %v, want %v", got, r.want)
			}
		})
	}
}

// liveReq returns the outcome of a requirement in a host report.
func liveReq(r hostReport, test, req string) string {
	for _, tc := range r.Tests {
		if tc.ID != test {
			continue
		}
		for _, q := range tc.Reqs {
			if q.Req == req {
				return q.Outcome
			}
		}
	}
	return ""
}

// TestHostDevRoots is the opt-in live run of `oec-infra host` on this
// computer against the sample roots of oec/dev (live.sh): the storages pass,
// D8 connects to the dev broker with the login of the root (REQ-PLT-007);
// there is no OEC Registry before P7, so REQ-PLT-008 fails and T8-HST-04
// fails (C-072). The mounts are not judged (the roots are plain
// directories).
func TestHostDevRoots(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev := os.Getenv("OEC_DEV_DIR")
	if dev == "" {
		t.Fatal("OEC_DEV_DIR not set")
	}
	for _, root := range []string{"oec-dev-app", "oec-dev-cert"} {
		t.Run(root, func(t *testing.T) {
			code, out, errOut := call(t, "host", "--root", filepath.Join(dev, "generated", "roots", root), "--registry-wait", "3s", "--format", "json")
			t.Logf("%s%s", out, errOut)
			r, got := outcomes(t, out)
			if code != 1 || got["T8-HST-01"] != "pass" || got["T8-HST-02"] != "pass" || got["T8-HST-04"] != "fail" {
				t.Errorf("exit %d, tests %v", code, got)
			}
			if o := liveReq(r, "T8-HST-04", "REQ-PLT-007"); o != "pass" {
				t.Errorf("REQ-PLT-007 %s", o)
			}
			if o := liveReq(r, "T8-HST-04", "REQ-PLT-008"); o != "fail" {
				t.Errorf("REQ-PLT-008 %s (an OEC Registry in oec/dev? then update this test)", o)
			}
		})
	}
}

// TestHostDevContainer is the opt-in live run of `oec-infra host` inside a
// Linux container (live.sh sets OEC_ENGINE and OEC_BROKER_CONTAINER): the
// Linux binary runs in the network namespace of the dev broker (broker.json
// says localhost) with the storages of a sample root mounted read-only
// (ro), once with the secret storage mounted read-write (REQ-CNT-008 fails).
func TestHostDevContainer(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev, engine, broker := os.Getenv("OEC_DEV_DIR"), os.Getenv("OEC_ENGINE"), os.Getenv("OEC_BROKER_CONTAINER")
	if dev == "" || engine == "" || broker == "" {
		t.Fatal("OEC_DEV_DIR, OEC_ENGINE or OEC_BROKER_CONTAINER not set")
	}
	cmd := func(name string, args ...string) string {
		t.Helper()
		out, err := exec.Command(name, args...).Output()
		if err != nil {
			t.Fatalf("%s %v: %v", name, args, err)
		}
		return strings.TrimSpace(string(out))
	}
	arch := cmd(engine, "info", "--format", "{{.Architecture}}")
	goarch := map[string]string{"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}[arch]
	if goarch == "" {
		t.Skipf("engine architecture %q", arch)
	}
	bin := filepath.Join(t.TempDir(), "oec-infra")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch, "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	image := cmd(engine, "inspect", "-f", "{{.Config.Image}}", broker)
	for _, c := range []struct {
		root, secrets string
		want          map[string]string
	}{
		{"oec-dev-app", "ro", map[string]string{"T8-HST-01": "pass", "T8-HST-02": "pass"}},
		{"oec-dev-cert", "ro", map[string]string{"T8-HST-01": "pass", "T8-HST-02": "pass"}},
		{"oec-dev-cert", "rw", map[string]string{"T8-HST-01": "pass", "T8-HST-02": "fail"}},
	} {
		t.Run(c.root+"-"+c.secrets, func(t *testing.T) {
			root := filepath.Join(dev, "generated", "roots", c.root)
			out, err := exec.Command(engine, "run", "--rm", "--network", "container:"+broker, "--entrypoint", "/oec-infra",
				"-v", bin+":/oec-infra:ro",
				"-v", filepath.Join(root, "etc", "oi4", "mqtt")+":/etc/oi4/mqtt:ro",
				"-v", filepath.Join(root, "etc", "oi4", "certs")+":/etc/oi4/certs:ro",
				"-v", filepath.Join(root, "run", "secrets", "oi4")+":/run/secrets/oi4:"+c.secrets,
				image, "host", "--container", c.root, "--registry-wait", "3s", "--format", "json").Output()
			t.Logf("%s", out)
			if len(out) == 0 {
				t.Fatalf("%s run: %v", engine, err)
			}
			if err := conformance.ValidateInfraReport(out); err != nil {
				t.Error(err)
			}
			r, got := outcomes(t, string(out))
			for id, want := range c.want {
				if got[id] != want {
					t.Errorf("%s: %s, want %s", id, got[id], want)
				}
			}
			if o := liveReq(r, "T8-HST-04", "REQ-PLT-003"); o != "manual" {
				t.Errorf("REQ-PLT-003 %s, want manual (Linux, patch level)", o)
			}
			if o := liveReq(r, "T8-HST-04", "REQ-PLT-007"); o != "pass" {
				t.Errorf("REQ-PLT-007 %s", o)
			}
			for _, s := range r.Host.Storages {
				want := "ro"
				if s.Name == "secrets" {
					want = c.secrets
				}
				if s.Mount != want {
					t.Errorf("storage %s mounted %q, want %q", s.Name, s.Mount, want)
				}
			}
		})
	}
}
