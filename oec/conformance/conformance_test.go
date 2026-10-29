package conformance

import (
	"encoding/json"
	"strings"
	"testing"
)

// minimal is a valid replay report of version 1.
const minimal = `{
  "report": 1, "tool": "oec-check", "version": "dev", "schemas": "2.0.1-dev", "baseline": "eb7889c",
  "catalog": "eb7889c", "command": ["oec-check", "monitor", "--replay", "x.jsonl"], "mode": "replay",
  "attachment": "bus", "capture": "x.jsonl", "level": 3, "self_assessment": true,
  "started": "2026-10-29T10:00:00Z", "ended": "2026-10-29T10:01:00.5Z", "observed": "1m0.5s",
  "messages": 2, "namespace": 2,
  "participants": [{"id": "OTConnector/a.com/M/P/S", "service_type": "OTConnector", "app_id": "a.com/M/P/S",
    "messages": 2, "invalid": 0, "births": 1, "wills": 0, "closes": 0,
    "first": "2026-10-29T10:00:00Z", "last": "2026-10-29T10:00:01Z"}],
  "tests": [{"id": "T7-MON-05", "title": "Health", "level": 1, "status": "implemented", "outcome": "fail",
    "reqs": [{"req": "REQ-TOP-017", "obligation": "MUST", "outcome": "fail",
      "evidence": [{"participant": "OTConnector/a.com/M/P/S", "line": 2, "topic": "Oi4v2/x", "detail": "gap"}]},
      {"req": "REQ-RES-HEALTH-004", "obligation": "MUST", "outcome": "skip", "inconclusive": true, "reason": "not observable"}]},
    {"id": "T7-MON-09", "title": "x", "level": 3, "status": "implemented", "outcome": "skip", "reason": "level 3 is above the declared level 2"}],
  "summary": {"fail": 1, "skip": 1},
  "exit": 1
}`

// testReport is a valid test report through the proxy.
const testReport = `{
  "report": 1, "tool": "oec-check", "version": "dev", "schemas": "2.0.1-dev", "baseline": "eb7889c",
  "catalog": "eb7889c", "command": ["oec-check", "test"], "mode": "test", "attachment": "proxy",
  "broker": "mqtts://localhost:8883", "insecure": true, "level": 2, "app": "a.com/M/P/S", "self_assessment": true,
  "started": "2026-10-29T10:00:00Z", "ended": "2026-10-29T10:01:00Z", "observed": "1m0s",
  "messages": 0, "namespace": 0, "participants": [],
  "tester": "Utility/oi4.org/oec-check/D7/0123456789ab",
  "requests": [{"test": "T7-GET-01", "topic": "Oi4v2/x/Get/MAM", "message_id": "m1", "sent": "2026-10-29T10:00:01Z", "replies": 1}],
  "restore": {"checked": true, "restored": false, "changed": ["- Config a", "+ Config b"], "note": "x"},
  "announced": [{"role": "tester", "publisher_id": "Utility/oi4.org/oec-check/D7/0123456789ab", "requests": 1, "answered": 1},
    {"role": "peer", "publisher_id": "Utility/oi4.org/oec-check/Peer/0123456789ab", "requests": 0, "answered": 0}],
  "proxy_listen": "127.0.0.1:18883",
  "connections": [{"id": 0, "phase": "relay", "remote": "127.0.0.1:5000", "accepted": "2026-10-29T10:00:00Z",
    "tls_version": "TLS 1.3", "client_certificate": false,
    "connect": {"protocol_name": "MQTT", "protocol_level": 4, "clean_session": true, "keep_alive": 30, "client_id": "c",
      "will": {"topic": "Oi4v2/x", "qos": 0, "retain": false}, "username": "u", "has_username": true, "has_password": true},
    "connect_time": "2026-10-29T10:00:00Z", "connack": 0,
    "publishes": [{"topic": "Oi4v2/x", "qos": 1, "retain": false, "time": "2026-10-29T10:00:00Z"}],
    "subscribed": "2026-10-29T10:00:00Z", "disconnect": false, "dropped": true, "closed": "2026-10-29T10:00:02Z"},
    {"id": 1, "phase": "tls1.1", "remote": "127.0.0.1:5001", "accepted": "2026-10-29T10:00:03Z", "client_certificate": false,
     "handshake_error": "protocol version", "connack": -1, "disconnect": false, "dropped": false}],
  "tests": [], "summary": {}, "error": "interrupted", "exit": 4
}`

func patch(t *testing.T, doc string, edit func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReportSchema(t *testing.T) {
	if !strings.Contains(string(ReportSchema), `"const": 1`) {
		t.Error("the report schema does not pin report version 1")
	}
	for name, doc := range map[string]string{"minimal": minimal, "test": testReport} {
		if err := ValidateReport([]byte(doc)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	first := func(m map[string]any, key string) map[string]any { return m[key].([]any)[0].(map[string]any) }
	for name, bad := range map[string][]byte{
		"version 2":             patch(t, minimal, func(m map[string]any) { m["report"] = 2 }),
		"unknown key":           patch(t, minimal, func(m map[string]any) { m["extra"] = true }),
		"no exit":               patch(t, minimal, func(m map[string]any) { delete(m, "exit") }),
		"exit 2":                patch(t, minimal, func(m map[string]any) { m["exit"] = 2 }),
		"level 4":               patch(t, minimal, func(m map[string]any) { m["level"] = 4 }),
		"mode":                  patch(t, minimal, func(m map[string]any) { m["mode"] = "bus" }),
		"replay without file":   patch(t, minimal, func(m map[string]any) { delete(m, "capture") }),
		"replay with tester":    patch(t, minimal, func(m map[string]any) { m["tester"] = "Utility/x" }),
		"replay through proxy":  patch(t, minimal, func(m map[string]any) { m["attachment"] = "proxy" }),
		"started no date":       patch(t, minimal, func(m map[string]any) { m["started"] = "yesterday" }),
		"outcome":               patch(t, minimal, func(m map[string]any) { first(m, "tests")["outcome"] = "error" }),
		"test id":               patch(t, minimal, func(m map[string]any) { first(m, "tests")["id"] = "T8-BUS-01" }),
		"summary key":           patch(t, minimal, func(m map[string]any) { m["summary"] = map[string]any{"ok": 1} }),
		"participant key":       patch(t, minimal, func(m map[string]any) { first(m, "participants")["password"] = "x" }),
		"test without tester":   patch(t, testReport, func(m map[string]any) { delete(m, "tester") }),
		"test without app":      patch(t, testReport, func(m map[string]any) { delete(m, "app") }),
		"proxy without listen":  patch(t, testReport, func(m map[string]any) { delete(m, "proxy_listen") }),
		"bus with connections":  patch(t, testReport, func(m map[string]any) { m["attachment"] = "bus"; delete(m, "proxy_listen") }),
		"password in connect":   patch(t, testReport, func(m map[string]any) { first(m, "connections")["connect"].(map[string]any)["password"] = "x" }),
		"payload in publish":    patch(t, testReport, func(m map[string]any) { first(first(m, "connections"), "publishes")["payload"] = "x" }),
		"phase":                 patch(t, testReport, func(m map[string]any) { first(m, "connections")["phase"] = "tls1.0" }),
		"request phase":         patch(t, testReport, func(m map[string]any) { first(m, "requests")["test"] = "setup" }),
		"role":                  patch(t, testReport, func(m map[string]any) { first(m, "announced")["role"] = "observer" }),
		"request without reply": patch(t, testReport, func(m map[string]any) { delete(first(m, "requests"), "replies") }),
		"restore without check": patch(t, testReport, func(m map[string]any) { delete(m["restore"].(map[string]any), "checked") }),
	} {
		if err := ValidateReport(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := ValidateReport([]byte("{")); err == nil {
		t.Error("no JSON: accepted")
	}
}

func TestFixtureIndexes(t *testing.T) {
	sim, err := SimFixtures()
	if err != nil {
		t.Fatal(err)
	}
	caps, err := CaptureFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(sim) == 0 || len(caps) == 0 {
		t.Fatalf("%d sim, %d capture fixtures", len(sim), len(caps))
	}
	for _, f := range append(sim, caps...) {
		if f.Kind != "sim" && f.Kind != "capture" || f.Name == "" || f.Status != "draft" && f.Status != "reviewed" {
			t.Errorf("fixture %+v", f)
		}
		for id, e := range f.Expect {
			if !strings.HasPrefix(id, "T7-") || e.Outcome == "" {
				t.Errorf("%s: expect %s %+v", f.Name, id, e)
			}
		}
	}
	none := sim[0]
	if none.Name != "none" || !none.Positive || !none.Runs("T7-GET-01") || !none.Peer || !none.Announce {
		t.Errorf("first sim fixture %+v", none)
	}
	sel := Fixture{Tests: []string{"T7-MON-*", "T7-CON-*"}}
	if !sel.Runs("T7-CON-02") || sel.Runs("T7-GET-01") {
		t.Error("Runs ignores the test selection")
	}
}
