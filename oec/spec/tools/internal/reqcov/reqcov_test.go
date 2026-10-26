package reqcov

import (
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/vectors"
)

func req(id string, ver ...string) catalog.Requirement {
	return catalog.Requirement{ID: id, Verification: ver, Status: "reviewed"}
}

func TestCompute(t *testing.T) {
	s := &catalog.Spec{Files: []catalog.AreaFile{{Area: "MSG", Requirements: []catalog.Requirement{
		req("REQ-MSG-001", "schema", "d7-validate"),
		req("REQ-MSG-002", "schema"),
		req("REQ-MSG-003", "d7-validate"),
		req("REQ-MSG-004", "schema", "d7-validate"),
		req("REQ-MSG-005", "manual"),
		req("REQ-MSG-006", "schema", "d7-validate"),
	}}}}
	files := []vectors.File{
		{Kind: "payload", Vectors: []vectors.Vector{
			{ID: "V-PAY-001", Expected: "valid", Status: "reviewed", Reqs: []string{"REQ-MSG-001", "REQ-MSG-002", "REQ-MSG-003"}},
			{ID: "V-PAY-002", Expected: "invalid", Status: "reviewed", Violates: []string{"REQ-MSG-001"}},
			{ID: "V-PAY-003", Expected: "invalid", Status: "reviewed", Violates: []string{"REQ-MSG-002", "REQ-MSG-003"}},
			{ID: "V-PAY-004", Expected: "invalid", Status: "draft", Violates: []string{"REQ-MSG-006"}},
		}},
		{Kind: "topic", Vectors: []vectors.Vector{
			{ID: "V-TOP-001", Expected: "invalid", Status: "reviewed", Violates: []string{"REQ-MSG-001"}},
		}},
	}
	im := &Implications{Implications: []Implication{
		{Umbrella: "REQ-MSG-004", Details: []string{"REQ-MSG-001", "REQ-MSG-002"}, Remainder: "none"},
		{Umbrella: "REQ-MSG-006", Details: []string{"REQ-MSG-004", "REQ-MSG-009"}, Remainder: "none"},
	}}
	ex := &Expectations{Expectations: []Expectation{
		{Vector: "V-PAY-003", Schema: []string{"REQ-MSG-002"}, Rule: []string{"REQ-MSG-003"}, Rules: []string{"R-2"}, Reason: "x"},
		{Vector: "V-PAY-002", Rule: []string{"REQ-MSG-002"}, Rules: []string{"R-2"}, Reason: "x"},
		{Vector: "V-PAY-001", Rule: []string{"REQ-MSG-001"}, Rules: []string{"R-2"}, Reason: "x"},
	}}
	ts := &topic.Spec{Matrix: topic.Matrix{Combinations: []topic.Combination{
		{Method: "Pub", Resource: "Health", Allowed: true, Rule: "pub-health"},
		{Method: "Del", Resource: "Health", Allowed: false},
	}}}
	r := Compute(s, files, ts, im, ex, t.TempDir())
	if len(r.Entries) != 5 {
		t.Fatalf("want 5 schema/d7-validate REQs, got %d", len(r.Entries))
	}
	for id, want := range map[string]string{
		"REQ-MSG-001": "covered", "REQ-MSG-002": "covered", "REQ-MSG-003": "covered",
		"REQ-MSG-004": "derived", "REQ-MSG-006": "details open (1 of 2)",
	} {
		if got := r.State(id); got != want {
			t.Errorf("%s: state %q, want %q", id, got, want)
		}
	}
	var layers []string
	for _, e := range r.Entries {
		for _, g := range e.Neg {
			layers = append(layers, e.Req.ID+"/"+g.ID+"/"+g.Layer)
		}
	}
	if got := strings.Join(layers, " "); got != "REQ-MSG-001/V-PAY-002/schema REQ-MSG-001/V-TOP-001/R-1 REQ-MSG-002/V-PAY-003/schema REQ-MSG-003/V-PAY-003/rule REQ-MSG-006/V-PAY-004/schema" {
		t.Errorf("layers: %s", got)
	}
	probs := strings.Join(r.Problems, "\n")
	for _, want := range []string{
		"unknown detail REQ-MSG-009",
		"V-PAY-002: schema + rule",
		"V-PAY-001 is not an invalid payload vector",
	} {
		if !strings.Contains(probs, want) {
			t.Errorf("problem %q missing in:\n%s", want, probs)
		}
	}
	if len(r.Rows) != 1 || r.Rows[0].Schema || r.MissingSchemas() != 1 {
		t.Errorf("rows: %+v", r.Rows)
	}
	if len(r.Gaps) != 2 {
		t.Errorf("gaps: %v", r.Gaps)
	}
	rep := r.Report()
	for _, want := range []string{"| **Total** | 5 | 3 | 1 | 0 | 0 | 1 |", "V-PAY-004*", "V-TOP-001 (R-1)", "| Pub | Health | `pub-health` | **missing** |"} {
		if !strings.Contains(rep, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func TestWarning(t *testing.T) {
	s := &catalog.Spec{Files: []catalog.AreaFile{{Area: "TOP", Requirements: []catalog.Requirement{
		req("REQ-TOP-020", "d7-validate"),
		req("REQ-TOP-022", "d7-validate"),
	}}}}
	files := []vectors.File{{Kind: "topic", Vectors: []vectors.Vector{
		{ID: "V-TOP-011", Expected: "valid", Status: "reviewed", Reqs: []string{"REQ-TOP-020", "REQ-TOP-022"}, Warnings: []string{"control-character", "whitespace"}},
		{ID: "V-TOP-137", Expected: "valid", Status: "draft", Reqs: []string{"REQ-TOP-020"}},
	}}}
	r := Compute(s, files, &topic.Spec{}, &Implications{}, &Expectations{}, t.TempDir())
	e := r.all["REQ-TOP-020"]
	if len(e.Pos) != 0 || len(e.PosDraft) != 1 || len(e.Neg) != 1 || e.Neg[0] != (Neg{ID: "V-TOP-011", Reviewed: true, Layer: "warning"}) {
		t.Errorf("REQ-TOP-020: pos %v, draft %v, neg %+v", e.Pos, e.PosDraft, e.Neg)
	}
	if got := r.State("REQ-TOP-020"); got != "negative only" {
		t.Errorf("REQ-TOP-020: state %q", got)
	}
	if got := r.State("REQ-TOP-022"); got != "positive only" {
		t.Errorf("REQ-TOP-022: state %q", got)
	}
	if !strings.Contains(r.Report(), "V-TOP-011 (warning)") {
		t.Error("report lacks the warning layer")
	}
}

func TestChecks(t *testing.T) {
	s := &catalog.Spec{Files: []catalog.AreaFile{{Area: "MSG", Requirements: []catalog.Requirement{
		req("REQ-MSG-001", "schema"),
		req("REQ-MSG-002", "schema"),
		req("REQ-MSG-003", "manual"),
	}}}}
	files := []vectors.File{{Kind: "payload", Vectors: []vectors.Vector{
		{ID: "V-PAY-001", Expected: "invalid", Status: "reviewed", Violates: []string{"REQ-MSG-001", "REQ-MSG-003"}},
	}}}
	im := &Implications{Implications: []Implication{
		{Umbrella: "REQ-MSG-001", Details: []string{"REQ-MSG-002"}, Remainder: "none"},
		{Umbrella: "REQ-MSG-002", Details: []string{"REQ-MSG-001"}, Remainder: "none"},
		{Umbrella: "REQ-MSG-003", Details: []string{"REQ-MSG-003"}, Remainder: "none"},
		{Umbrella: "REQ-MSG-002", Details: []string{"REQ-MSG-001"}, Remainder: "none"},
	}}
	ex := &Expectations{Expectations: []Expectation{
		{Vector: "V-PAY-001", Schema: []string{"REQ-MSG-003"}, Rule: []string{"REQ-MSG-001"}, Rules: []string{"R-2"}, Reason: "x"},
	}}
	r := Compute(s, files, &topic.Spec{}, im, ex, t.TempDir())
	probs := strings.Join(r.Problems, "\n")
	for _, want := range []string{
		"REQ-MSG-001 is part of an implication cycle",
		"umbrella REQ-MSG-003 is deprecated or has neither schema nor d7-validate",
		"REQ-MSG-003 is its own detail",
		"umbrella REQ-MSG-002 listed twice",
		"rule-layer REQ-MSG-001 lacks verification d7-validate",
		"schema-layer REQ-MSG-003 lacks verification schema",
	} {
		if !strings.Contains(probs, want) {
			t.Errorf("problem %q missing in:\n%s", want, probs)
		}
	}
}
