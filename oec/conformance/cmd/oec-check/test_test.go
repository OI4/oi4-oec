package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/sim"
)

// TestTestLive runs oec-check test against the simulated CUT on a loopback
// broker (DESIGN.md §10): pass without a fault, fail with one.
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
		args  []string
		code  int
		want  string
	}{
		{sim.None, []string{"--level", "3"}, exitValid, "pass  T7-GET-01"},
		{sim.ReplyTo, []string{"--level", "3", "--tests", "T7-GET-01"}, exitInvalid, "fail  T7-GET-01"},
	} {
		t.Run(string(c.fault), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			s, err := sim.Start(ctx, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "probe", Password: []byte("secret")}, c.fault)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			rec, js := filepath.Join(dir, fmt.Sprintf("%d.jsonl", i)), filepath.Join(dir, fmt.Sprintf("%d.json", i))
			args := append(append([]string{"test", "--app", app, "--record", rec, "--report", "json=" + js}, conn...), c.args...)
			code, out, errOut := exec(t, "", args...)
			if code != c.code || !strings.Contains(out, c.want) || !strings.Contains(out, "test (test ") {
				t.Fatalf("exit %d, want %d\n%s%s", code, c.code, out, errOut)
			}
			raw, _ := os.ReadFile(js)
			var rep struct {
				Mode     string
				Tester   string
				Requests []struct {
					Test    string
					Replies int
				}
			}
			if err := json.Unmarshal(raw, &rep); err != nil || rep.Mode != "test" || !strings.HasPrefix(rep.Tester, "Utility/oi4.org/oec-check/D7/") || len(rep.Requests) == 0 {
				t.Errorf("report %+v %v", rep, err)
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
	} {
		if code, out, errOut := exec(t, "", args...); code != exitUsage {
			t.Errorf("%v: exit %d\n%s%s", args, code, out, errOut)
		}
	}
}
