package verdict

import "testing"

func TestLoadRequirementsAndObligation(t *testing.T) {
	rs, err := LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 400 {
		t.Fatalf("only %d requirements", len(rs))
	}
	cases := []struct {
		id    string
		level int
		want  string
	}{
		{"REQ-TOP-017", 1, "MUST"},
		{"REQ-LIFE-014", 1, "SHOULD NOT"},
		{"REQ-LIFE-014", 2, "MUST NOT"},
		{"REQ-MQTT-018", 3, "MUST NOT"},
		{"REQ-NOPE-001", 1, ""},
	}
	for _, c := range cases {
		if got := rs.Obligation(c.id, c.level); got != c.want {
			t.Errorf("Obligation(%s, %d) = %q, want %q", c.id, c.level, got, c.want)
		}
	}
	rs["REQ-X-001"] = Requirement{ID: "REQ-X-001", Keyword: "MAY"}
	rs["REQ-X-002"] = Requirement{ID: "REQ-X-002", Keyword: "MUST", Levels: map[string]string{"L3": "MUST"}}
	if got := rs.Obligation("REQ-X-001", 2); got != "MAY" {
		t.Errorf("ungraded: %q", got)
	}
	if got := rs.Obligation("REQ-X-002", 1); got != "none" {
		t.Errorf("not applicable: %q", got)
	}
}

func TestSeverity(t *testing.T) {
	for in, want := range map[string]Outcome{"MUST": Fail, "MUST NOT": Fail, "SHOULD": Warn, "SHOULD NOT": Warn, "MAY": Info, "none": "", "": Fail} {
		if got := Severity(in); got != want {
			t.Errorf("Severity(%q) = %q, want %q", in, got, want)
		}
	}
	if Mandatory("SHOULD") || !Mandatory("MUST NOT") || Mandatory("") {
		t.Error("Mandatory")
	}
}

func reqs() Requirements {
	return Requirements{
		"REQ-A": {ID: "REQ-A", Keyword: "MUST", Levels: map[string]string{"L1": "MUST", "L2": "MUST", "L3": "MUST"}},
		"REQ-B": {ID: "REQ-B", Keyword: "SHOULD", Levels: map[string]string{"L1": "SHOULD", "L2": "MUST", "L3": "MUST"}},
		"REQ-C": {ID: "REQ-C", Keyword: "MUST", Levels: map[string]string{"L3": "MUST"}},
	}
}

func TestBuild(t *testing.T) {
	tests := []Test{
		{ID: "T1", Status: "implemented", Level: 1, Reqs: []string{"REQ-A", "REQ-B"}},
		{ID: "T2", Status: "planned", Level: 1, Reqs: []string{"REQ-A"}},
		{ID: "T3", Status: "implemented", Level: 3, Reqs: []string{"REQ-C"}},
		{ID: "T4", Status: "implemented", Level: 1, Reqs: []string{"REQ-A"}},
		{ID: "T5", Status: "implemented", Level: 1, Reqs: []string{"REQ-A", "REQ-C"}},
	}
	obs := []Observation{
		{Test: "T1", Req: "REQ-A", Kind: Checked},
		{Test: "T1", Req: "REQ-B", Kind: Violation, Evidence: Evidence{Detail: "b"}},
		{Test: "T4", Req: "REQ-A", Kind: Skipped, Inconclusive: true, Evidence: Evidence{Detail: "too short"}},
		{Test: "T5", Req: "REQ-A", Kind: Checked},
		{Test: "T5", Req: "", Kind: Violation, Evidence: Evidence{Detail: "schema"}},
	}
	got := Build(tests, obs, reqs(), 1)
	want := map[string]Outcome{"T1": Warn, "T2": Skip, "T3": Skip, "T4": Skip, "T5": Fail}
	for _, r := range got {
		if r.Outcome != want[r.ID] {
			t.Errorf("%s: %s, want %s (%+v)", r.ID, r.Outcome, want[r.ID], r)
		}
	}
	if Exit(got, false) != ExitFail {
		t.Error("exit with fail")
	}
	if !got[3].Reqs[0].Inconclusive || Exit(got[:4], false) != ExitInconclusive || Exit(got[:4], true) != ExitPass {
		t.Errorf("inconclusive: %+v", got[3])
	}
	// At level 2 REQ-B is MUST: T1 fails.
	if r := Build(tests[:1], obs, reqs(), 2); r[0].Outcome != Fail {
		t.Errorf("level 2: %+v", r[0])
	}
	if s := Summary(got); s[Warn] != 1 || s[Skip] != 2 || s[Fail] != 1 {
		t.Errorf("summary %v", s)
	}
}

func TestJudgeNotesAndEvidenceLimit(t *testing.T) {
	var obs []Observation
	for i := 0; i < MaxEvidence+5; i++ {
		obs = append(obs, Observation{Test: "T", Req: "REQ-A", Kind: Violation, Evidence: Evidence{Detail: "x"}})
	}
	r := Build([]Test{{ID: "T", Status: "implemented", Level: 1, Reqs: []string{"REQ-A"}}}, obs, reqs(), 1)[0].Reqs[0]
	if r.Outcome != Fail || len(r.Evidence) != MaxEvidence+1 {
		t.Errorf("%s %d", r.Outcome, len(r.Evidence))
	}
	r = Build([]Test{{ID: "T", Status: "implemented", Level: 1, Reqs: []string{"REQ-A"}}}, []Observation{{Test: "T", Req: "REQ-A", Kind: Note, Evidence: Evidence{Detail: "hint"}}}, reqs(), 1)[0].Reqs[0]
	if r.Outcome != Info {
		t.Errorf("note: %s", r.Outcome)
	}
	r = Build([]Test{{ID: "T", Status: "implemented", Level: 1, Reqs: []string{"REQ-A"}}}, nil, reqs(), 1)[0].Reqs[0]
	if r.Outcome != Skip || r.Inconclusive {
		t.Errorf("nothing observed: %+v", r)
	}
}

func TestTestOutcomeIgnoresNotApplicable(t *testing.T) {
	tests := []Test{{ID: "T", Status: "implemented", Level: 1, Reqs: []string{"REQ-A", "REQ-B"}}}
	pass := []Observation{{Test: "T", Req: "REQ-A", Kind: Checked}}
	if r := Build(tests, pass, reqs(), 1)[0]; r.Outcome != Pass || r.Reqs[1].Outcome != Skip {
		t.Errorf("a not applicable requirement keeps pass: %+v", r)
	}
	incon := append(pass, Observation{Test: "T", Req: "REQ-B", Kind: Skipped, Inconclusive: true})
	if r := Build(tests, incon, reqs(), 2)[0]; r.Outcome != Skip || !r.Reqs[1].Inconclusive {
		t.Errorf("an inconclusive MUST requirement makes the test skip: %+v", r)
	}
	if r := Build(tests, nil, reqs(), 1)[0]; r.Outcome != Skip {
		t.Errorf("nothing observed: %+v", r)
	}
}
