package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
)

const captures = "../../testdata/captures/"

func TestMonitorReplayExitCodes(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
	}{
		{[]string{"--replay", captures + "lifecycle-pass.jsonl"}, exitValid},
		{[]string{"--replay", captures + "heartbeat-gap.jsonl"}, exitInvalid},
		{[]string{"--replay", captures + "invalid-message.jsonl"}, exitInvalid},
		// 30 s observation: the heartbeat (MUST) is inconclusive.
		{[]string{"--replay", captures + "reconnect-pass.jsonl"}, exitInconclusive},
		{[]string{"--replay", captures + "reconnect-pass.jsonl", "--allow-skip"}, exitValid},
		// A failing test wins over an inconclusive one.
		{[]string{"--replay", captures + "heartbeat-gap.jsonl", "--allow-skip"}, exitInvalid},
		// --app selects the component under test: B has no violation.
		{[]string{"--replay", captures + "request-id-pending.jsonl", "--app", "consumer.com/MEService/MES-7/7322"}, exitInvalid},
		{[]string{"--replay", captures + "heartbeat-gap.jsonl", "--app", "consumer.com/MEService/MES-7/7322"}, exitInconclusive},
		{[]string{"--replay", captures + "heartbeat-gap.jsonl", "--app", "OTConnector/provider.com/FieldDataService/FDS-001/1200-0345"}, exitInvalid},
		// usage errors
		{[]string{"--replay", captures + "missing.jsonl"}, exitUsage},
		{[]string{"--replay", captures + "index.yaml"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "--record", "x.jsonl"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "--insecure"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "--level", "4"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "--format", "xml"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "--report", "pdf=x.pdf"}, exitUsage},
		{[]string{"--replay", captures + "lifecycle-pass.jsonl", "extra"}, exitUsage},
		{[]string{}, exitUsage},
		{[]string{"--broker", "mqtt://localhost:1883"}, exitUsage},
		{[]string{"--broker", "mqtts://localhost:8883", "--password-file", "pw"}, exitUsage},
		{[]string{"--broker", "mqtts://localhost:8883", "--cert", "c.pem"}, exitUsage},
	} {
		code, out, errOut := exec(t, "", append([]string{"monitor"}, c.args...)...)
		if code != c.code {
			t.Errorf("%v: exit %d, want %d\n%s%s", c.args, code, c.code, out, errOut)
		}
	}
}

// TestReplayReportSchema replays every capture fixture through the CLI at
// every level, with and without the component under test; exec validates
// each JSON report against report.schema.json (C-069).
func TestReplayReportSchema(t *testing.T) {
	fixtures, err := conformance.CaptureFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		for _, level := range []string{"1", "2", "3"} {
			args := []string{"monitor", "--replay", captures + f.File, "--format", "json", "--level", level}
			if level == "2" {
				args = append(args, "--app", "OTConnector/provider.com/FieldDataService/FDS-001/1200-0345")
			}
			code, out, errOut := exec(t, "", args...)
			if code != exitValid && code != exitInvalid && code != exitInconclusive || !strings.HasPrefix(out, "{") {
				t.Errorf("%v: exit %d\n%s", args, code, errOut)
			}
		}
	}
}

func TestMonitorReports(t *testing.T) {
	dir := t.TempDir()
	js, junit, text := filepath.Join(dir, "r.json"), filepath.Join(dir, "r.xml"), filepath.Join(dir, "r.txt")
	code, out, errOut := exec(t, "", "monitor", "--replay", captures+"heartbeat-gap.jsonl", "--format", "json",
		"--report", "junit="+junit, "--report", "text="+text, "--report", "json="+js, "--level", "2")
	if code != exitInvalid {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	file, err := os.ReadFile(js)
	if err != nil {
		t.Fatal(err)
	}
	if string(file) != out {
		t.Error("JSON report file differs from standard output")
	}
	var r struct {
		Report       int
		Mode         string
		Level        int
		Capture      string
		Messages     int
		Exit         int
		Participants []struct{ ID string }
		Tests        []struct {
			ID      string
			Outcome string
		}
		Summary map[string]int
	}
	if err := json.Unmarshal(file, &r); err != nil {
		t.Fatal(err)
	}
	outcome := map[string]string{}
	for _, x := range r.Tests {
		outcome[x.ID] = x.Outcome
	}
	if r.Report != 1 || r.Mode != "replay" || r.Level != 2 || r.Exit != exitInvalid || r.Messages != 5 || len(r.Participants) != 1 ||
		outcome["T7-MON-05"] != "fail" || outcome["T7-MON-03"] != "pass" || r.Summary["fail"] != 1 {
		t.Errorf("JSON report: %+v", r)
	}
	x, err := os.ReadFile(junit)
	if err != nil {
		t.Fatal(err)
	}
	var suites struct {
		Tests    int `xml:"tests,attr"`
		Failures int `xml:"failures,attr"`
	}
	if err := xml.Unmarshal(x, &suites); err != nil {
		t.Fatal(err)
	}
	if suites.Tests != 19 || suites.Failures != 1 {
		t.Errorf("JUnit: %+v", suites)
	}
	tx, err := os.ReadFile(text)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tx), "fail  T7-MON-05") || !strings.Contains(string(tx), "result FAIL (exit 1)") {
		t.Errorf("text report:\n%s", tx)
	}
}

func TestMonitorConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "c.yaml")
	os.WriteFile(cfg, []byte("allow-skip: true\nlevel: 2\n"), 0o644)
	if code, _, errOut := exec(t, "", "monitor", "--config", cfg, "--replay", captures+"reconnect-pass.jsonl"); code != exitValid {
		t.Errorf("config allow-skip: exit %d %s", code, errOut)
	}
	// the command line wins over the file
	if code, out, _ := exec(t, "", "monitor", "--config", cfg, "--level", "3", "--replay", captures+"lifecycle-pass.jsonl"); code != exitValid || !strings.Contains(out, "level 3") {
		t.Errorf("flag over config: exit %d\n%s", code, out)
	}
	os.WriteFile(cfg, []byte("password: secret\n"), 0o644)
	if code, _, errOut := exec(t, "", "monitor", "--config", cfg, "--replay", captures+"lifecycle-pass.jsonl"); code != exitUsage || !strings.Contains(errOut, `unknown key "password"`) {
		t.Errorf("unknown key: exit %d %s", code, errOut)
	}
}

func TestValidateCapture(t *testing.T) {
	code, out, errOut := exec(t, "", "validate", captures+"lifecycle-pass.jsonl")
	if code != exitValid || !strings.Contains(out, "lifecycle-pass.jsonl:2") {
		t.Errorf("valid capture: exit %d\n%s%s", code, out, errOut)
	}
	if code, _, _ := exec(t, "", "validate", captures+"invalid-message.jsonl"); code != exitInvalid {
		t.Errorf("invalid capture: exit %d", code)
	}
}

// TestMonitorLive runs monitor against the in-process test broker (loopback
// only): a simulated component publishes the records of a fixture.
func TestMonitorLive(t *testing.T) {
	p := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, p, "probe", "secret", false)
	dir := t.TempDir()
	pw := filepath.Join(dir, "password")
	os.WriteFile(pw, []byte("secret\n"), 0o600)
	data, err := os.ReadFile(captures + "lifecycle-pass.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := capture.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	go func() {
		// wait for the subscription of the monitor, then publish
		for b.Subscribers(subscription) == 0 {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		pub, err := mqttwire.Dial(ctx, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "probe", Password: []byte("secret")})
		if err != nil {
			t.Error(err)
			return
		}
		defer pub.Close()
		for _, r := range fixture.Records {
			if err := pub.Publish(ctx, mqttwire.Message{Topic: r.Topic, Payload: r.Payload, QoS: r.QoS}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	rec, js := filepath.Join(dir, "rec.jsonl"), filepath.Join(dir, "r.json")
	code, out, errOut := exec(t, "", "monitor", "--broker", b.URL(), "--ca", p.CAFile, "--username", "probe", "--password-file", pw,
		"--duration", "2s", "--record", rec, "--report", "json="+js)
	if code != exitInconclusive || !strings.Contains(out, "observation too short") {
		t.Fatalf("live: exit %d\n%s%s", code, out, errOut)
	}
	got, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	c, err := capture.Read(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Records) != len(fixture.Records) || c.Trailer == nil || c.Header.Broker != b.URL() || c.Header.SubscribeQoS != 1 || c.Header.Insecure {
		t.Fatalf("recording: %d of %d records, header %+v, trailer %+v", len(c.Records), len(fixture.Records), c.Header, c.Trailer)
	}
	for i, r := range c.Records {
		if r.Topic != fixture.Records[i].Topic || !bytes.Equal(r.Payload, fixture.Records[i].Payload) {
			t.Errorf("record %d: %s", i, r.Topic)
		}
	}
	// the recording replays to the same verdicts (time-based rules aside)
	code2, out2, _ := exec(t, "", "monitor", "--replay", rec)
	if code2 != exitInconclusive || strings.Count(out2, "pass  T7-MON") != strings.Count(out, "pass  T7-MON") {
		t.Errorf("replay of the recording: exit %d\n%s\nlive:\n%s", code2, out2, out)
	}
	if strings.Contains(string(got), "secret") {
		t.Error("the password is in the capture")
	}
	rep, _ := os.ReadFile(js)
	if strings.Contains(string(rep), "secret\"") || !strings.Contains(string(rep), `"mode": "live"`) {
		t.Errorf("report:\n%s", rep)
	}

	// --insecure: connects with a wrong CA, warns and marks the report
	code, out, errOut = exec(t, "", "monitor", "--broker", b.URL(), "--ca", p.OtherCAFile, "--username", "probe", "--password-file", pw,
		"--duration", "200ms", "--insecure", "--format", "json", "--allow-skip")
	if code != exitValid || !strings.Contains(errOut, "WARNING: --insecure") || !strings.Contains(out, `"insecure": true`) {
		t.Errorf("insecure: exit %d\n%s%s", code, out, errOut)
	}
	// wrong CA without --insecure, wrong password: broker not reachable
	for _, args := range [][]string{
		{"--broker", b.URL(), "--ca", p.OtherCAFile, "--username", "probe", "--password-file", pw},
		{"--broker", b.URL(), "--ca", p.CAFile, "--username", "other", "--password-file", pw},
	} {
		if code, out, errOut := exec(t, "", append([]string{"monitor", "--duration", "200ms"}, args...)...); code != exitInternal {
			t.Errorf("%v: exit %d\n%s%s", args, code, out, errOut)
		}
	}
	// connection lost: the observation stops early and says why
	b = mqtttest.NewBroker(t, p, "probe", "secret", false)
	go func() {
		for b.Subscribers(subscription) == 0 && ctx.Err() == nil {
			time.Sleep(10 * time.Millisecond)
		}
		b.Close()
	}()
	code, out, errOut = exec(t, "", "monitor", "--broker", b.URL(), "--ca", p.CAFile, "--username", "probe", "--password-file", pw,
		"--duration", "20s", "--format", "json", "--allow-skip")
	if code != exitValid || !strings.Contains(errOut, "connection lost") || !strings.Contains(out, `"error": "connection lost`) {
		t.Errorf("lost: exit %d\n%s%s", code, out, errOut)
	}
}
