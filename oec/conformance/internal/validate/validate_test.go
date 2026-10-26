package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/OI4/oec/conformance/internal/testvec"
	"go.yaml.in/yaml/v3"
)

func load(t *testing.T) *Validator {
	t.Helper()
	v, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestPayloadVectors: D7 reports no finding for a valid payload vector and
// exactly the requirements in violates for an invalid one.
func TestPayloadVectors(t *testing.T) {
	val := load(t)
	vs, err := testvec.Load("payload")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Run(v.ID, func(t *testing.T) {
			m := Message{Topic: v.Topic, Payload: json.RawMessage(v.Payload), Role: v.Role, Object: v.Object, Tolerant: v.Mode == "tolerant"}
			res, err := val.Validate(m)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range res.Findings {
				if f.Req == "" {
					t.Errorf("finding without REQ: %+v", f)
				}
			}
			switch v.Expected {
			case "valid":
				if !res.Valid {
					t.Errorf("valid vector: %s", dump(res.Findings))
				}
			case "invalid":
				want := slices.Clone(v.Violates)
				slices.Sort(want)
				if !slices.Equal(res.Reqs, want) {
					t.Errorf("reqs %v, violates %v:\n%s", res.Reqs, want, dump(res.Findings))
				}
			}
		})
	}
}

func dump(fs []finding.Finding) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString("    " + f.Layer + " " + f.Req + " " + f.Pointer + ": " + f.Detail + "\n")
	}
	return b.String()
}

func TestParse(t *testing.T) {
	for in, ok := range map[string]bool{
		`{"a":1}`:                           true,
		`{"a":{"b":1,"c":[{"b":2}]},"b":3}`: true,
		`{"a":1,"a":2}`:                     false,
		`{"a":[{"b":1,"b":1}]}`:             false,
		`{"a":1} x`:                         false,
		`{"a":1}{}`:                         false,
		"{\"a\":\"\xff\"}":                  false,
		`[1,2,{"x":"x"}]`:                   true,
	} {
		if _, err := Parse([]byte(in)); (err == nil) != ok {
			t.Errorf("Parse(%q): err %v", in, err)
		}
	}
}

// TestLayerPartition: the schema layer and the rule layer report exactly the
// partition of the expectation list of the schema test suite (ADR-0019 §8);
// a vector not listed there is detected by the schema layer alone.
func TestLayerPartition(t *testing.T) {
	val := load(t)
	raw, err := os.ReadFile(filepath.Join(testvec.SpecRoot(), "..", "schema", "tests", "rule-layer.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Expectations []struct {
			Vector string   `yaml:"vector"`
			Schema []string `yaml:"schema"`
			Rule   []string `yaml:"rule"`
		} `yaml:"expectations"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	type split struct{ schema, rule []string }
	want := map[string]split{}
	for _, e := range doc.Expectations {
		want[e.Vector] = split{sorted(e.Schema), sorted(e.Rule)}
	}
	vs, err := testvec.Load("payload")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Expected != "invalid" {
			continue
		}
		res, err := val.Validate(Message{Topic: v.Topic, Payload: json.RawMessage(v.Payload), Role: v.Role, Object: v.Object, Tolerant: v.Mode == "tolerant"})
		if err != nil {
			t.Fatal(err)
		}
		var sf, rf []finding.Finding
		for _, f := range res.Findings {
			if f.Layer == finding.LayerRule {
				rf = append(rf, f)
			} else {
				sf = append(sf, f)
			}
		}
		w, ok := want[v.ID]
		if !ok {
			w = split{schema: sorted(v.Violates)}
		}
		if got := finding.Reqs(sf); !slices.Equal(got, w.schema) {
			t.Errorf("%s: schema layer %v, want %v", v.ID, got, w.schema)
		}
		if got := finding.Reqs(rf); !slices.Equal(got, w.rule) {
			t.Errorf("%s: rule layer %v, want %v:\n%s", v.ID, got, w.rule, dump(rf))
		}
		delete(want, v.ID)
	}
	for id := range want {
		t.Errorf("expectation %s names no invalid payload vector", id)
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	if len(s) == 0 {
		return nil
	}
	return s
}
