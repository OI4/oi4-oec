package topic

import (
	"testing"
)

const specDir = "../../.."

func load(t *testing.T) *Spec {
	t.Helper()
	s, err := Load(specDir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCheck(t *testing.T) {
	s := load(t)
	for _, err := range s.Check(CheckOptions{Samples: 100, Seed: 1}) {
		t.Error(err)
	}
}

func TestAnalyze(t *testing.T) {
	s := load(t)
	const p = "cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU"
	const c = "Y29tcGFueS5pZC9BODdCQUQzNEg"
	cases := []struct {
		topic, rule, failure string
		values               []string
		warnings             int
	}{
		{"Oi4v2/OTConnector/" + p + "/Get/MAM/" + c, "get-mam", "", nil, 0},
		{"Oi4v2/OTConnector/" + p + "/Pub/Event/" + c + "/Syslog/Notice", "pub-event", "", []string{"Syslog", "Notice"}, 0},
		{"Oi4v2/OTConnector/" + p + "/Pub/Data/" + c + "/a%2Fb%25c d", "pub-data", "", []string{"a/b%c d"}, 1},
		{"Oi4v2/OTConnector/" + p + "/Pub/Data/" + c + "/a%20b", "", FailGrammar, nil, 0},
		{"Oi4v2/OTConnector/" + p + "/Get/Event", "", FailGrammar, nil, 0},
		{"Oi4v2/OTConnector/Q29tcGFueS5pZC9B/Get/MAM", "get-mam", FailIL, nil, 0},
	}
	for _, tc := range cases {
		res, err := s.Analyze(tc.topic)
		if err != nil {
			t.Fatal(err)
		}
		if res.Rule != tc.rule || res.Failure != tc.failure || len(res.Warnings) != tc.warnings {
			t.Errorf("Analyze(%q) = %+v", tc.topic, res)
			continue
		}
		if tc.values != nil && (res.Parsed == nil || len(res.Parsed.FilterValues) != len(tc.values) || res.Parsed.FilterValues[0] != tc.values[0]) {
			t.Errorf("Analyze(%q): filter values %v", tc.topic, res.Parsed.FilterValues)
		}
	}
}

func TestPatterns(t *testing.T) {
	s := load(t)
	out, err := s.PatternsFileContent()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 1000 {
		t.Fatalf("patterns too short: %d", len(out))
	}
}

func TestCheckDetectsMatrixDrift(t *testing.T) {
	for _, mutate := range []func(c *Combination){
		func(c *Combination) { c.Source = Required },
		func(c *Combination) { c.Filter, c.FilterRule = Optional, "tag" },
		func(c *Combination) { c.Allowed, c.Rule, c.Source, c.Filter = false, "", "", "" },
	} {
		s := load(t)
		for i := range s.Matrix.Combinations {
			if c := &s.Matrix.Combinations[i]; c.Method == "Pub" && c.Resource == "MAM" {
				mutate(c)
			}
		}
		if errs := s.Check(CheckOptions{Samples: 50, Seed: 1}); len(errs) == 0 {
			t.Errorf("mutated matrix passed the check: %+v", s.Matrix.Combinations[0])
		}
	}
	s := load(t)
	for i := range s.Matrix.Combinations {
		if c := &s.Matrix.Combinations[i]; c.Method == "Get" && c.Resource == "Event" {
			c.Allowed, c.Rule, c.Source, c.Filter, c.FilterRule = true, "get-event", Required, Required, "event-filter"
		}
	}
	if errs := s.Check(CheckOptions{Samples: 10, Seed: 1}); len(errs) == 0 {
		t.Error("allowed row without grammar rule passed the check")
	}
}
