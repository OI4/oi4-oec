package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/bus/registrytest"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
)

// busEnv is a conforming test broker (as the bus fixture "none") with a
// simulated OEC Registry and the files of a run.
type busEnv struct {
	b                     *mqtttest.Broker
	p                     *mqtttest.PKI
	dir, password, creds2 string
}

// restricted is the ACL of the unprivileged user app: Get only (Pub and
// Reply only under its own AppId, which it does not use here), Set/Config
// neither subscribed nor delivered.
func restricted(user, topic string, k mqtttest.Access) bool {
	if user != "app" {
		return true
	}
	lv := strings.Split(topic, "/")
	if len(lv) < 5 {
		return k != mqtttest.Write
	}
	if k == mqtttest.Write {
		return lv[3] == "Get"
	}
	return lv[3] != "Set" || lv[4] != "Config"
}

func newBusEnv(t *testing.T, open bool) *busEnv {
	t.Helper()
	e := &busEnv{p: mqtttest.NewPKI(t), dir: t.TempDir()}
	o := mqtttest.Options{Username: "oec-probe", Password: "probe-secret",
		Users: map[string]string{"app": "app-secret", "registry": "registry-secret"}, ACL: restricted,
		MaxPacket: 262144 + 65544, MaxPayload: 262144}
	if open {
		o.ACL = nil
	}
	e.b = mqtttest.NewBrokerWith(t, e.p, o)
	registrytest.Start(t, mqttwire.Config{Broker: e.b.URL(), CAFile: e.p.CAFile, Username: "registry", Password: []byte("registry-secret")},
		"oi4.org/Registry/SIM/1", registrytest.None, 100*time.Millisecond)
	e.password = e.file(t, "password", "probe-secret\n")
	e.creds2 = e.file(t, "credentials", base64.StdEncoding.EncodeToString([]byte("app:app-secret"))+"\n")
	return e
}

func (e *busEnv) file(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(e.dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// args are the flags of a fast run against the environment.
func (e *busEnv) args(extra ...string) []string {
	return append([]string{"bus", "--broker", e.b.URL(), "--ca", e.p.CAFile, "--username", "oec-probe", "--password-file", e.password,
		"--plain-port", "0", "--max-packet-size", "262144", "--timeout", "1s", "--registry-wait", "3s"}, extra...)
}

type busReport struct {
	Mode   string `json:"mode"`
	Input  string `json:"input"`
	Exit   int    `json:"exit"`
	Broker struct {
		URL         string   `json:"url"`
		Probe       string   `json:"probe"`
		TLS         []string `json:"tls"`
		Certificate struct {
			Names []string `json:"names"`
		} `json:"certificate"`
		MaxPacketSize int    `json:"max_packet_size"`
		Registry      string `json:"registry"`
	} `json:"broker"`
	Tests []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
	} `json:"tests"`
}

func TestBusRun(t *testing.T) {
	e := newBusEnv(t, false)
	text := filepath.Join(e.dir, "report.txt")
	code, out, errOut := call(t, e.args("--credentials-2", e.creds2, "--format", "json", "--report", "text="+text)...)
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	var r busReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Mode != "bus" || r.Input != e.b.URL() || r.Broker.URL != e.b.URL() || !strings.HasPrefix(r.Broker.Probe, "oi4.org/oec-infra/D8/") ||
		strings.Join(r.Broker.TLS, ",") != "1.2,1.3" || r.Broker.MaxPacketSize != 262144 || r.Broker.Registry != "oi4.org/Registry/SIM/1" ||
		len(r.Broker.Certificate.Names) == 0 || len(r.Tests) != 3 {
		t.Errorf("report %+v", r)
	}
	for _, tc := range r.Tests {
		if tc.Outcome != "pass" {
			t.Errorf("%s: %s", tc.ID, tc.Outcome)
		}
	}
	if strings.Contains(out, "probe-secret") || strings.Contains(out, "app-secret") {
		t.Error("the report holds a password")
	}
	txt, err := os.ReadFile(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"oec-infra dev bus (" + e.b.URL() + "), level 3", "probe AppId oi4.org/oec-infra/D8/", "TLS 1.2, 1.3", "MaxPacketSize 262144", "OEC Registry oi4.org/Registry/SIM/1", "T8-BUS-02"} {
		if !strings.Contains(string(txt), want) {
			t.Errorf("text report without %q:\n%s", want, txt)
		}
	}
}

func TestBusFail(t *testing.T) {
	e := newBusEnv(t, true)
	code, out, errOut := call(t, e.args("--credentials-2", e.creds2, "--tests", "T8-BUS-02", "--format", "json")...)
	if code != 1 {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	var r busReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Tests) != 1 || r.Tests[0].ID != "T8-BUS-02" || r.Tests[0].Outcome != "fail" || !strings.Contains(out, `"topic": "Oi4v2/Utility/`) {
		t.Errorf("report %s", out)
	}
}

// TestBusStorage runs from a container storage (RIG 4.1) with a --config
// file; without an unprivileged client T8-BUS-02 is inconclusive (exit 4,
// --allow-skip: 0).
func TestBusStorage(t *testing.T) {
	e := newBusEnv(t, false)
	_, port, _ := net.SplitHostPort(e.b.Addr)
	n, _ := strconv.Atoi(port)
	root := filepath.Join(e.dir, "root")
	ca, err := os.ReadFile(e.p.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	e.file(t, "root/etc/oi4/mqtt/broker.json", fmt.Sprintf(`{"Address": "localhost", "SecurePort": %d, "MaxPacketSize": 262144}`, n))
	e.file(t, "root/etc/oi4/mqtt/broker_ca.pem", string(ca))
	e.file(t, "root/run/secrets/oi4/credentials", base64.StdEncoding.EncodeToString([]byte("oec-probe:probe-secret")))
	cfg := e.file(t, "bus.yaml", "bus-storage: "+root+"\nplain-port: 0\ntimeout: 1s\ntests: [T8-BUS-01, T8-BUS-02]\n")
	code, out, errOut := call(t, "bus", "--config", cfg, "--format", "json")
	if code != exitInconclusive || !strings.Contains(out, `"inconclusive": true`) {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	code, out, errOut = call(t, "bus", "--config", cfg, "--allow-skip", "--tests", "T8-BUS-02", "--format", "junit")
	if code != exitPass || !strings.Contains(out, "<testsuite") || !strings.Contains(out, "T8-BUS-02") || strings.Contains(out, "T8-BUS-01") {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	// The unprivileged client from a second container storage.
	e.file(t, "root2/etc/oi4/mqtt/broker.json", fmt.Sprintf(`{"Address": "localhost", "SecurePort": %d}`, n))
	e.file(t, "root2/etc/oi4/mqtt/broker_ca.pem", string(ca))
	e.file(t, "root2/run/secrets/oi4/credentials", base64.StdEncoding.EncodeToString([]byte("app:app-secret")))
	code, out, errOut = call(t, "bus", "--config", cfg, "--bus-storage-2", filepath.Join(e.dir, "root2"), "--tests", "T8-BUS-02", "--format", "json")
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
}

func TestBusUsage(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("bm8tY29sb24="), 0o600); err != nil { // "no-colon"
		t.Fatal(err)
	}
	unknown := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(unknown, []byte("duration: 1s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := "mqtts://localhost:1"
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"bus"}, "--broker or --bus-storage"},
		{[]string{"bus", "--broker", b, "x"}, "no arguments"},
		{[]string{"bus", "--broker", "http://x"}, "mqtts"},
		{[]string{"bus", "--broker", b, "--cert", "c.pem"}, "--cert and --key"},
		{[]string{"bus", "--broker", b, "--password-file", bad}, "--username"},
		{[]string{"bus", "--broker", b, "--credentials-2", bad, "--bus-storage-2", dir}, "exclude each other"},
		{[]string{"bus", "--broker", b, "--credentials-2", bad}, "<username>:<password>"},
		{[]string{"bus", "--broker", b, "--bus-storage-2", dir}, "--bus-storage-2"},
		{[]string{"bus", "--bus-storage", dir}, "container storage"},
		{[]string{"bus", "--broker", b, "--tests", "T8-IMG-*"}, "selects no bus test"},
		{[]string{"bus", "--broker", b, "--plain-port", "x"}, "not a port"},
		{[]string{"bus", "--broker", b, "--timeout", "0s"}, "positive"},
		{[]string{"bus", "--broker", b, "--format", "pdf"}, "--format"},
		{[]string{"bus", "--config", unknown}, `unknown key "duration"`},
	} {
		code, out, errOut := call(t, c.args...)
		if code != exitUsage || !strings.Contains(out+errOut, c.msg) {
			t.Errorf("%v: exit %d, want %d with %q\n%s%s", c.args, code, exitUsage, c.msg, out, errOut)
		}
	}
}

// TestBusUnreachable: no broker – every test is inconclusive (exit 4).
func TestBusUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	code, out, errOut := call(t, "bus", "--broker", "mqtts://"+addr, "--plain-port", "0", "--timeout", "500ms", "--format", "json")
	if code != exitInconclusive || strings.Contains(out, `"outcome": "fail"`) {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
}
