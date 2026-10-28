package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/sim"
)

// freePort returns a loopback port that was free a moment ago.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// TestTestLive runs oec-check test against the simulated CUT on a loopback
// broker (DESIGN.md §10): pass without a fault through the proxy attachment
// (§3.4), fail with one on the bus.
func TestTestLive(t *testing.T) {
	p := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, p, "probe", "secret", false)
	dir := t.TempDir()
	pw := filepath.Join(dir, "password")
	os.WriteFile(pw, []byte("secret\n"), 0o600)
	conn := []string{"--broker", b.URL(), "--ca", p.CAFile, "--username", "probe", "--password-file", pw, "--timeout", "2s"}
	app := sim.ServiceType + "/" + sim.AppID

	for i, c := range []struct {
		fault sim.Fault
		proxy bool
		args  []string
		code  int
		want  string
	}{
		{sim.None, true, []string{"--level", "3"}, exitValid, "pass  T7-LIFE-02"},
		{sim.ReplyTo, false, []string{"--level", "3", "--tests", "T7-GET-01"}, exitInvalid, "fail  T7-GET-01"},
	} {
		t.Run(string(c.fault), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cfg := mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "probe", Password: []byte("secret")}
			rec, js := filepath.Join(dir, fmt.Sprintf("%d.jsonl", i)), filepath.Join(dir, fmt.Sprintf("%d.json", i))
			args := append(append([]string{"test", "--app", app, "--record", rec, "--report", "json=" + js}, conn...), c.args...)
			var s *sim.Sim
			started := make(chan error, 1)
			if c.proxy {
				// The CUT connects to the proxy, which listens once
				// oec-check runs: retry until it is there.
				port := freePort(t)
				args = append(args, "--proxy-listen", "127.0.0.1:"+port, "--proxy-ca", p.CAFile, "--proxy-ca-key", p.CAKeyFile, "--proxy-wait", "30s")
				cfg.Broker = "mqtts://localhost:" + port
				go func() {
					for {
						var err error
						if s, err = sim.Start(ctx, cfg, c.fault); err == nil || ctx.Err() != nil {
							started <- err
							return
						}
						time.Sleep(50 * time.Millisecond)
					}
				}()
			} else {
				var err error
				if s, err = sim.Start(ctx, cfg, c.fault); err != nil {
					t.Fatal(err)
				}
				started <- nil
			}
			code, out, errOut := exec(t, "", args...)
			if err := <-started; err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if code != c.code || !strings.Contains(out, c.want) || !strings.Contains(out, "test (test ") {
				t.Fatalf("exit %d, want %d\n%s%s", code, c.code, out, errOut)
			}
			raw, _ := os.ReadFile(js)
			var rep struct {
				Mode        string
				Attachment  string
				Connections []struct{ Phase string }
				Tester      string
				Requests    []struct {
					Test    string
					Replies int
				}
			}
			if err := json.Unmarshal(raw, &rep); err != nil || rep.Mode != "test" || !strings.HasPrefix(rep.Tester, "Utility/oi4.org/oec-check/D7/") || len(rep.Requests) == 0 {
				t.Errorf("report %+v %v", rep, err)
			}
			if c.proxy && (rep.Attachment != "proxy" || len(rep.Connections) < 5 || !strings.Contains(out, "proxy 127.0.0.1:")) {
				t.Errorf("proxy attachment: %s, %d connection(s)\n%s", rep.Attachment, len(rep.Connections), out)
			}
			if strings.Contains(string(raw), "secret\"") {
				t.Error("the password is in the report")
			}
			data, _ := os.ReadFile(rec)
			cp, err := capture.Read(bytes.NewReader(data))
			if err != nil || cp.Trailer == nil || !strings.Contains(string(data), "/Get/MAM/") {
				t.Errorf("recording: %v", err)
			}
			if c.fault == sim.ReplyTo && !strings.Contains(out, "not selected (--tests)") {
				t.Errorf("--tests did not deselect:\n%s", out)
			}
		})
	}
}

func TestTestUsage(t *testing.T) {
	for _, args := range [][]string{
		{"test", "--broker", "mqtts://localhost"},
		{"test", "--broker", "mqtts://localhost", "--app", "Foo/a.com/M/P/S"},
		{"test", "--broker", "mqtts://localhost", "--app", "not an app id"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--tests", "T7-NOPE-*"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--replay", "x.jsonl"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--timeout", "0s"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--tester-il", "a b"},
		{"test", "--app", "a.com/M/P/S"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--proxy-listen", "127.0.0.1:0"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--proxy-ca", "ca.pem"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--proxy-listen", "127.0.0.1:0", "--proxy-ca", "nope.pem", "--proxy-ca-key", "nope.key"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--proxy-listen", "127.0.0.1:0", "--proxy-ca", "a", "--proxy-ca-key", "b", "--proxy-wait", "0s"},
	} {
		if code, out, errOut := exec(t, "", args...); code != exitUsage {
			t.Errorf("%v: exit %d\n%s%s", args, code, out, errOut)
		}
	}
}
