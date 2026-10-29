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

// restartOn restarts the simulated CUT once the restart hook created flag.
func restartOn(ctx context.Context, t *testing.T, s *sim.Sim, flag string) {
	for ctx.Err() == nil {
		if _, err := os.Stat(flag); err == nil {
			if err := s.Restart(ctx); err != nil {
				t.Errorf("restart: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRestartHook runs --stop-cmd and --start-cmd in this order and
// reports a failing command.
func TestRestartHook(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	var errOut bytes.Buffer
	f := &monitorFlags{stopCmd: "echo stop >> '" + log + "'", startCmd: "echo start >> '" + log + "'", restartWait: time.Second}
	hook, err := f.restartHook(&errOut)
	if err != nil || hook == nil {
		t.Fatal(hook != nil, err)
	}
	if err := hook(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(log); string(b) != "stop\nstart\n" {
		t.Errorf("commands ran %q", b)
	}
	f = &monitorFlags{restartCmd: "exit 3", restartWait: time.Second}
	if hook, _ = f.restartHook(&errOut); hook == nil || hook(context.Background()) == nil {
		t.Error("a failing restart command is not reported")
	}
	if hook, err = (&monitorFlags{restartWait: time.Second}).restartHook(&errOut); hook != nil || err != nil {
		t.Errorf("no restart flags: %v %v", hook != nil, err)
	}
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
		{sim.None, true, []string{"--level", "3", "--peer", "--announce"}, exitValid, "pass  T7-ADR-01"},
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
				// The restart hook creates a file; the test restarts the
				// simulated CUT when it appears.
				flag := filepath.Join(dir, fmt.Sprintf("restart-%d", i))
				args = append(args, "--proxy-listen", "127.0.0.1:"+port, "--proxy-ca", p.CAFile, "--proxy-ca-key", p.CAKeyFile, "--proxy-wait", "30s",
					"--restart-cmd", "touch '"+flag+"'", "--restart-wait", "30s")
				cfg.Broker = "mqtts://localhost:" + port
				go func() {
					for {
						var err error
						if s, err = sim.Start(ctx, cfg, c.fault); err == nil || ctx.Err() != nil {
							started <- err
							if err == nil {
								go restartOn(ctx, t, s, flag)
							}
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
				Announced []struct {
					Role        string
					PublisherID string `json:"publisher_id"`
					Requests    int
					Answered    int
				}
			}
			if err := json.Unmarshal(raw, &rep); err != nil || rep.Mode != "test" || !strings.HasPrefix(rep.Tester, "Utility/oi4.org/oec-check/D7/") || len(rep.Requests) == 0 {
				t.Errorf("report %+v %v", rep, err)
			}
			if c.proxy && (rep.Attachment != "proxy" || len(rep.Connections) < 5 || !strings.Contains(out, "proxy 127.0.0.1:")) {
				t.Errorf("proxy attachment: %s, %d connection(s)\n%s", rep.Attachment, len(rep.Connections), out)
			}
			if c.proxy && (!strings.Contains(out, "pass  T7-LIFE-04") || !strings.Contains(out, "pass  T7-SET-10") || !strings.Contains(errOut, "restart hook: touch")) {
				t.Errorf("restart hook:\n%s%s", out, errOut)
			}
			if c.fault == sim.None && (len(rep.Announced) != 2 || rep.Announced[0].Role != "tester" || rep.Announced[0].PublisherID != rep.Tester ||
				rep.Announced[1].Role != "peer" || !strings.HasPrefix(rep.Announced[1].PublisherID, "Utility/oi4.org/oec-check/Peer/") ||
				rep.Announced[1].Requests == 0 || !strings.Contains(out, "announced peer Utility/oi4.org/oec-check/Peer/")) {
				t.Errorf("--peer --announce: %+v\n%s", rep.Announced, out)
			}
			if c.fault == sim.ReplyTo && (len(rep.Announced) != 0 || !strings.Contains(out, "T7-ADR-01")) {
				t.Errorf("announced without --peer, --announce: %+v", rep.Announced)
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
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--restart-cmd", "true", "--stop-cmd", "true", "--start-cmd", "true"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--stop-cmd", "true"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--start-cmd", "true"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--restart-cmd", "true", "--restart-wait", "0s"},
		{"monitor", "--broker", "mqtts://localhost", "--restart-cmd", "true"},
		{"monitor", "--broker", "mqtts://localhost", "--peer"},
		{"test", "--broker", "mqtts://localhost", "--app", "a.com/M/P/S", "--announce", "--tester-il", "a.com/M/P"},
	} {
		if code, out, errOut := exec(t, "", args...); code != exitUsage {
			t.Errorf("%v: exit %d\n%s%s", args, code, out, errOut)
		}
	}
}
