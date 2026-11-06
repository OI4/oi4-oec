package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance"
)

// listIDs returns the test ids of a text listing (first column of every
// row after the header, without the closing count line).
func listIDs(t *testing.T, out string) []string {
	t.Helper()
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(l); len(f) > 0 && strings.HasPrefix(f[0], "T7-") {
			ids = append(ids, f[0])
		}
	}
	return ids
}

func TestList(t *testing.T) {
	code, out, errs := exec(t, "", "list")
	if code != exitValid || errs != "" {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	ids := listIDs(t, out)
	if len(ids) != 55 || ids[0] != "T7-MON-01" || !strings.Contains(out, "55 tests (55 implemented, 0 planned), 4 manual requirements") {
		t.Fatalf("list: %d tests\n%s", len(ids), out)
	}
	if !strings.Contains(out, "T7-ADR-01") || !strings.Contains(out, "peer") {
		t.Errorf("list misses T7-ADR-01 with its need:\n%s", out)
	}

	for _, c := range []struct {
		name string
		args []string
		want []string // all ids, in order
		n    int      // or only the count
	}{
		{name: "need", args: []string{"--needs", "peer"}, want: []string{"T7-ADR-01"}},
		{name: "restart hook", args: []string{"--needs", "restart-hook"}, want: []string{"T7-LIFE-04", "T7-SET-10"}},
		{name: "pattern", args: []string{"T7-ACT-*"}, want: []string{"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04"}},
		{name: "two patterns", args: []string{"T7-ADR-01", "T7-MON-01"}, want: []string{"T7-MON-01", "T7-ADR-01"}},
		{name: "monitor", args: []string{"--mode", "monitor"}, n: 19},
		{name: "test without needs", args: []string{"--mode", "test", "--needs", "none"}, n: 26},
		{name: "planned", args: []string{"--status", "planned"}, n: 0},
		{name: "req", args: []string{"--req", "REQ-TOP-010"}, want: []string{"T7-MON-02", "T7-ADR-01"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out, errs := exec(t, "", append([]string{"list"}, c.args...)...)
			if code != exitValid || errs != "" {
				t.Fatalf("exit %d, stderr %q", code, errs)
			}
			got := listIDs(t, out)
			if c.want != nil && strings.Join(got, " ") != strings.Join(c.want, " ") || c.want == nil && len(got) != c.n {
				t.Errorf("got %v\n%s", got, out)
			}
		})
	}

	code, out, _ = exec(t, "", "list", "--level", "1", "--mode", "test", "--format", "json")
	if code != exitValid {
		t.Fatalf("json: exit %d", code)
	}
	var tests []struct {
		ID     string   `json:"id"`
		Mode   string   `json:"mode"`
		Level  int      `json:"level"`
		Needs  []string `json:"needs"`
		Reqs   []string `json:"reqs"`
		Status string   `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &tests); err != nil || len(tests) == 0 {
		t.Fatalf("json: %v\n%s", err, out)
	}
	for _, tc := range tests {
		if tc.Level > 1 || tc.Mode != "test" || tc.Status != "implemented" || len(tc.Reqs) == 0 || tc.Needs == nil {
			t.Errorf("json: %+v", tc)
		}
	}

	for _, args := range [][]string{
		{"--mode", "image"}, {"--level", "4"}, {"--needs", "docker"}, {"--status", "done"},
		{"--format", "junit"}, {"--req", "REQ-NOPE-001"}, {"[T7"}, {"--bogus"},
	} {
		if code, _, errs := exec(t, "", append([]string{"list"}, args...)...); code != exitUsage || errs == "" {
			t.Errorf("list %v: exit %d, stderr %q", args, code, errs)
		}
	}
}

func TestExplain(t *testing.T) {
	for _, c := range []struct {
		id   string
		want []string
	}{
		{"T7-ADR-01", []string{
			"T7-ADR-01 – Requests sent by the CUT address the destination",
			"mode test, level 1, needs peer, implemented",
			"REQ-TOP-010", "MUST", // a requirement with its keyword
			"Messaging Guideline",
			"sim T7-ADR-01/own-app: fail REQ-BUS-002, REQ-TOP-006, REQ-TOP-010",
			"sim T7-ADR-01/silent: skip",
			"sim none: pass or not applicable",
		}},
		{"t7-mon-05", []string{"T7-MON-05", "capture"}},
		{"REQ-TOP-010", []string{
			"REQ-TOP-010 – ", "statement:", "refs:", "Messaging Guideline", "verification:",
			"tests: T7-MON-02, T7-ADR-01", "sim T7-ADR-01/own-app: fail",
		}},
		{"REQ-LIFE-011", []string{"manual: Subscriber side"}},
		{"REQ-ACT-002", []string{"checked by oec-check validate (schema, d7-validate)"}},
	} {
		t.Run(c.id, func(t *testing.T) {
			code, out, errs := exec(t, "", "explain", c.id)
			if code != exitValid || errs != "" {
				t.Fatalf("exit %d, stderr %q", code, errs)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in\n%s", w, out)
				}
			}
		})
	}

	code, out, _ := exec(t, "", "explain", "--format", "json", "REQ-TOP-010")
	var req struct {
		ID        string   `json:"id"`
		Statement string   `json:"statement"`
		Refs      []string `json:"refs"`
		Tests     []string `json:"tests"`
		Fixtures  []struct {
			Fixture string `json:"fixture"`
			Test    string `json:"test"`
			Outcome string `json:"outcome"`
		} `json:"fixtures"`
	}
	if code != exitValid || json.Unmarshal([]byte(out), &req) != nil || req.ID != "REQ-TOP-010" || req.Statement == "" || len(req.Refs) == 0 || len(req.Tests) != 2 || len(req.Fixtures) == 0 {
		t.Errorf("json: exit %d\n%s", code, out)
	}
	code, out, _ = exec(t, "", "explain", "--format", "json", "T7-ADR-01")
	var tc struct {
		ID           string `json:"id"`
		Requirements []struct {
			ID string `json:"id"`
		} `json:"requirements"`
		Fixtures []struct {
			Fixture string `json:"fixture"`
		} `json:"fixtures"`
	}
	if code != exitValid || json.Unmarshal([]byte(out), &tc) != nil || tc.ID != "T7-ADR-01" || len(tc.Requirements) != 3 || len(tc.Fixtures) == 0 {
		t.Errorf("json: exit %d\n%s", code, out)
	}

	for _, args := range [][]string{{}, {"T7-NOPE-01"}, {"REQ-NOPE-001"}, {"T7-ADR-01", "T7-MON-01"}, {"--format", "junit", "T7-ADR-01"}} {
		if code, _, errs := exec(t, "", append([]string{"explain"}, args...)...); code != exitUsage || errs == "" {
			t.Errorf("explain %v: exit %d, stderr %q", args, code, errs)
		}
	}
}

func TestReportSchemaCommand(t *testing.T) {
	code, out, errs := exec(t, "", "report-schema")
	if code != exitValid || errs != "" || out != string(conformance.ReportSchema) {
		t.Errorf("report-schema: exit %d, stderr %q", code, errs)
	}
	if code, _, errs := exec(t, "", "report-schema", "x"); code != exitUsage || errs == "" {
		t.Errorf("report-schema x: exit %d", code)
	}
	code, out, _ = exec(t, "", "version")
	if code != exitValid || !strings.Contains(out, "catalog 605fbb7, report 1") {
		t.Errorf("version: %s", out)
	}
}
