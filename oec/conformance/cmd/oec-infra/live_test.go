package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
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

// liveEngine returns the Engine API endpoint of the engine of live.sh:
// DOCKER_HOST/CONTAINER_HOST, else the endpoint of the current Docker
// context or the socket of Podman.
func liveEngine(t *testing.T, engine string) string {
	t.Helper()
	for _, env := range []string{"DOCKER_HOST", "CONTAINER_HOST"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	args := []string{"context", "inspect", "--format", "{{.Endpoints.docker.Host}}"}
	if engine == "podman" {
		args = []string{"info", "--format", "{{.Host.RemoteSocket.Path}}"}
	}
	out, err := exec.Command(engine, args...).Output()
	if err != nil {
		t.Fatalf("%s %v: %v", engine, args, err)
	}
	return strings.TrimSpace(string(out))
}

func liveUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// TestContainerDev is the opt-in live run of `oec-infra container` and of
// T8-HST-03 (`oec-infra host --tests T8-HST-03`) against the engine of
// live.sh (C-073). It starts two containers from the image of the dev
// broker (entrypoint sleep): a conforming application
// oec-live-app-<uuid> (hostname = name, the bridge network of oec/dev, own
// volumes on /etc/oi4/app and /opt/oi4/app, a read-only bind on
// /run/secrets/oi4) and a faulty one (host network, hostname other than
// the name, an underscore, a bind on /opt/oi4/licenses, the /etc/oi4/app
// volume of the application). The dev broker itself is no OI4 container
// (RIG 4.2): its hostname "mosquitto" differs from the compose name, so
// REQ-CNT-019 fails there. The containers and volumes are removed
// afterwards.
func TestContainerDev(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev, engine, broker := os.Getenv("OEC_DEV_DIR"), os.Getenv("OEC_ENGINE"), os.Getenv("OEC_BROKER_CONTAINER")
	if dev == "" || engine == "" || broker == "" {
		t.Fatal("OEC_DEV_DIR, OEC_ENGINE or OEC_BROKER_CONTAINER not set")
	}
	cmd := func(name string, args ...string) string {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	ep := liveEngine(t, engine)
	image := cmd(engine, "inspect", "-f", "{{.Config.Image}}", broker)
	brokerName := strings.TrimPrefix(cmd(engine, "inspect", "-f", "{{.Name}}", broker), "/")
	network := strings.Fields(cmd(engine, "inspect", "-f", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}", broker))[0]
	id := liveUUID(t)
	app, bad := "oec-live-app-"+id, "oec-live-bad_"+id[:8]
	cfg, data := "oec-live-cfg-"+id[:8], "oec-live-data-"+id[:8]
	secrets, lic := t.TempDir(), t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command(engine, "rm", "-f", app, bad).Run()
		_ = exec.Command(engine, "volume", "rm", "-f", cfg, data).Run()
	})
	cmd(engine, "run", "-d", "--name", app, "--hostname", app, "--network", network,
		"-v", cfg+":/etc/oi4/app", "-v", data+":/opt/oi4/app", "-v", secrets+":/run/secrets/oi4:ro",
		"--entrypoint", "sleep", image, "600")
	cmd(engine, "run", "-d", "--name", bad, "--hostname", "oec-live-bad", "--network", "host",
		"-v", lic+":/opt/oi4/licenses:ro", "-v", cfg+":/etc/oi4/app",
		"--entrypoint", "sleep", image, "600")

	for _, c := range []struct {
		label, name string
		exit        int
		want        map[string]string
	}{
		{"app", app, exitPass, map[string]string{"T8-CTR-01": "pass", "T8-CTR-02": "pass", "T8-CTR-03": "pass"}},
		{"bad", bad, 1, map[string]string{"T8-CTR-01": "warn", "T8-CTR-02": "fail", "T8-CTR-03": "fail"}},
		{"broker", brokerName, 1, map[string]string{"T8-CTR-01": "pass", "T8-CTR-02": "fail", "T8-CTR-03": "pass"}},
	} {
		t.Run("container/"+c.label, func(t *testing.T) {
			code, out, errOut := call(t, "container", "--daemon", ep, "--format", "json", c.name)
			t.Logf("%s%s", out, errOut)
			_, got := containerOutcomes(t, out)
			if code != c.exit {
				t.Errorf("exit %d, want %d", code, c.exit)
			}
			for id, want := range c.want {
				if got[id] != want {
					t.Errorf("%s: %s, want %s", id, got[id], want)
				}
			}
			if strings.Contains(out, secrets) || strings.Contains(out, lic) || strings.Contains(out, "/var/lib/") {
				t.Error("a mount source in the report")
			}
		})
	}

	root := filepath.Join(dev, "generated", "roots", "oec-dev-app")
	storages := func(t *testing.T, exit int, outcome, failing string) {
		code, out, errOut := call(t, "host", "--root", root, "--offline", "--daemon", ep, "--tests", "T8-HST-03", "--format", "json")
		t.Logf("%s%s", out, errOut)
		r, got := outcomes(t, out)
		if code != exit || got["T8-HST-03"] != outcome {
			t.Errorf("exit %d, T8-HST-03 %s; want %d, %s", code, got["T8-HST-03"], exit, outcome)
		}
		if failing != "" && liveReq(r, "T8-HST-03", failing) != "fail" {
			t.Errorf("%s %s, want fail", failing, liveReq(r, "T8-HST-03", failing))
		}
	}
	t.Run("host/shared", func(t *testing.T) { storages(t, 1, "fail", "REQ-CNT-011") })
	cmd(engine, "rm", "-f", bad)
	t.Run("host/own", func(t *testing.T) { storages(t, exitPass, "pass", "") })
}
