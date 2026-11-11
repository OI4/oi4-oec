package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// listIDs returns the test ids of a text listing (first column of every
// row after the header, without the closing count line).
func listIDs(out string) []string {
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(l); len(f) > 0 && strings.HasPrefix(f[0], "T8-") {
			ids = append(ids, f[0])
		}
	}
	return ids
}

func TestList(t *testing.T) {
	code, out, errs := call(t, "list")
	if code != exitPass || errs != "" {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	ids := listIDs(out)
	if len(ids) != 17 || ids[0] != "T8-IMG-01" || !strings.Contains(out, "17 tests (17 implemented, 0 planned), 1 manual requirement (oec-infra explain REQ-…)") {
		t.Fatalf("list: %d tests\n%s", len(ids), out)
	}
	// an ungraded test shows "-" as level
	if !strings.Contains(out, "T8-HST-03") || !strings.Contains(out, "host-access,docker") {
		t.Errorf("list misses T8-HST-03 with its needs:\n%s", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) > 2 && f[0] == "T8-BUS-01" && f[2] != "-" {
			t.Errorf("T8-BUS-01 is not graded: %q", l)
		}
	}

	for _, c := range []struct {
		name string
		args []string
		want []string // all ids, in order
		n    int      // or only the count
	}{
		{name: "docker", args: []string{"--needs", "docker"}, want: []string{"T8-RUN-01", "T8-RUN-02", "T8-RUN-03", "T8-CTR-01", "T8-CTR-02", "T8-CTR-03", "T8-HST-03"}},
		{name: "second credentials", args: []string{"--needs", "credentials-2"}, want: []string{"T8-BUS-02"}},
		{name: "offline", args: []string{"--needs", "none"}, want: []string{"T8-IMG-01", "T8-IMG-02", "T8-IMG-03", "T8-IMG-04", "T8-BUS-01", "T8-BUS-03"}},
		{name: "pattern", args: []string{"T8-RUN-*"}, want: []string{"T8-RUN-01", "T8-RUN-02", "T8-RUN-03"}},
		{name: "pattern after flag", args: []string{"--mode", "host", "T8-HST-0[12]"}, want: []string{"T8-HST-01", "T8-HST-02"}},
		{name: "image", args: []string{"--mode", "image"}, n: 4},
		{name: "level 1", args: []string{"--level", "1", "--mode", "image"}, want: []string{"T8-IMG-01"}},
		{name: "ungraded at level 1", args: []string{"--level", "1", "--mode", "bus"}, n: 3},
		{name: "planned", args: []string{"--status", "planned"}, n: 0},
		{name: "req", args: []string{"--req", "REQ-CNT-003"}, want: []string{"T8-HST-01", "T8-BUS-01"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out, errs := call(t, append([]string{"list"}, c.args...)...)
			if code != exitPass || errs != "" {
				t.Fatalf("exit %d, stderr %q", code, errs)
			}
			got := listIDs(out)
			if c.want != nil && strings.Join(got, " ") != strings.Join(c.want, " ") || c.want == nil && len(got) != c.n {
				t.Errorf("got %v\n%s", got, out)
			}
		})
	}

	code, out, _ = call(t, "list", "--mode", "container", "--format", "json")
	if code != exitPass {
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
	if err := json.Unmarshal([]byte(out), &tests); err != nil || len(tests) != 3 {
		t.Fatalf("json: %v\n%s", err, out)
	}
	for _, tc := range tests {
		if tc.Mode != "container" || tc.Status != "implemented" || len(tc.Reqs) == 0 || len(tc.Needs) != 1 {
			t.Errorf("json: %+v", tc)
		}
	}
	if code, out, _ = call(t, "list", "--needs", "none", "--mode", "bus", "--format", "json"); code != exitPass || !strings.Contains(out, `"needs": []`) {
		t.Errorf("json: needs of a test without needs is not []: exit %d\n%s", code, out)
	}

	for _, args := range [][]string{
		{"--mode", "monitor"}, {"--level", "4"}, {"--level", "0"}, {"--needs", "peer"}, {"--status", "done"},
		{"--format", "junit"}, {"--req", "REQ-NOPE-001"}, {"[T8"}, {"--bogus"},
	} {
		if code, _, errs := call(t, append([]string{"list"}, args...)...); code != exitUsage || errs == "" {
			t.Errorf("list %v: exit %d, stderr %q", args, code, errs)
		}
	}
	if code, out, errs := call(t, "list", "-h"); code != exitPass || out != "" || !strings.Contains(errs, "-needs") {
		t.Errorf("list -h: exit %d, stderr %q", code, errs)
	}
	if _, out, _ := call(t, "help"); !strings.Contains(out, "oec-infra list [--mode image|run|container|host|bus]") || !strings.Contains(out, "oec-infra explain [--format text|json] T8-…|REQ-…") {
		t.Errorf("usage misses list or explain:\n%s", out)
	}
}

func TestExplain(t *testing.T) {
	for _, c := range []struct {
		id   string
		want []string
	}{
		{"T8-RUN-02", []string{
			"T8-RUN-02 – ",
			"mode run, level 2, needs docker, implemented",
			"REQ-CNT-009", "MUST", // a requirement with its keyword
			"Runtime & Infrastructure Guideline",
			"run no-credentials: fail REQ-CNT-009",
			"run good: pass or not applicable",
		}},
		{"t8-hst-04", []string{
			"mode host, not graded, needs host-access, implemented",
			"host none: pass, manual REQ-PLT-003",
			"host T8-HST-04/ram-reserved: pass, manual REQ-PLT-001, REQ-PLT-003",
		}},
		{"T8-IMG-03", []string{"image no-sbom.tar: fail", "image good.tar: pass or not applicable"}},
		{"REQ-CNT-009", []string{
			"REQ-CNT-009 – ", "statement:", "refs:", "Runtime & Infrastructure Guideline", "verification: ",
			"tests: T8-RUN-02", "run no-credentials: fail T8-RUN-02",
		}},
		{"REQ-PLT-003", []string{"tests: T8-HST-04", "host none: pass T8-HST-04, manual REQ-PLT-003"}},
		{"REQ-CMP-012", []string{"manual: A rule for D8 itself"}},
		// a D7 requirement: oec-infra does not check it and names oec-check
		{"REQ-TOP-010", []string{"not checked by oec-infra (oec-check explain REQ-TOP-010)", "fixtures: none"}},
	} {
		t.Run(c.id, func(t *testing.T) {
			code, out, errs := call(t, "explain", c.id)
			if code != exitPass || errs != "" {
				t.Fatalf("exit %d, stderr %q", code, errs)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in\n%s", w, out)
				}
			}
		})
	}

	code, out, _ := call(t, "explain", "--format", "json", "REQ-PLT-003")
	var req struct {
		ID        string   `json:"id"`
		Statement string   `json:"statement"`
		Refs      []string `json:"refs"`
		Tests     []string `json:"tests"`
		Fixtures  []struct {
			Fixture string   `json:"fixture"`
			Test    string   `json:"test"`
			Outcome string   `json:"outcome"`
			Manual  []string `json:"manual"`
		} `json:"fixtures"`
	}
	if code != exitPass || json.Unmarshal([]byte(out), &req) != nil || req.ID != "REQ-PLT-003" || req.Statement == "" || len(req.Refs) == 0 || len(req.Tests) != 1 || len(req.Fixtures) == 0 || len(req.Fixtures[0].Manual) == 0 {
		t.Errorf("json: exit %d\n%s", code, out)
	}
	if strings.Contains(out, `"validate"`) {
		t.Errorf("json: a D8 explanation names oec-check validate\n%s", out)
	}
	code, out, _ = call(t, "explain", "T8-BUS-02", "--format", "json")
	var tc struct {
		ID           string `json:"id"`
		Level        int    `json:"level"`
		Needs        []string
		Requirements []struct {
			ID string `json:"id"`
		} `json:"requirements"`
		Fixtures []struct {
			Fixture string `json:"fixture"`
		} `json:"fixtures"`
	}
	if code != exitPass || json.Unmarshal([]byte(out), &tc) != nil || tc.ID != "T8-BUS-02" || tc.Level != 0 || len(tc.Requirements) != 9 || len(tc.Fixtures) == 0 {
		t.Errorf("json: exit %d\n%s", code, out)
	}

	for _, args := range [][]string{{}, {"T8-NOPE-01"}, {"T7-ADR-01"}, {"REQ-NOPE-001"}, {"T8-IMG-01", "T8-IMG-02"}, {"--format", "junit", "T8-IMG-01"}} {
		if code, _, errs := call(t, append([]string{"explain"}, args...)...); code != exitUsage || errs == "" {
			t.Errorf("explain %v: exit %d, stderr %q", args, code, errs)
		}
	}
	if _, _, errs := call(t, "explain", "T7-ADR-01"); !strings.Contains(errs, "oec-check explain") {
		t.Errorf("explain of a D7 test does not point to oec-check: %q", errs)
	}
}
