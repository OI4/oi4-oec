package validate

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/OI4/oec/conformance/internal/testvec"
)

func vector(t *testing.T, id string) testvec.Vector {
	t.Helper()
	vs, err := testvec.Load("payload")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("no vector %s", id)
	return testvec.Vector{}
}

// mutate returns the payload with the value at the reference tokens replaced.
func mutate(t *testing.T, payload string, tokens []string, nv any) json.RawMessage {
	t.Helper()
	v, err := Parse([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	node := v
	for i, k := range tokens {
		last := i == len(tokens)-1
		switch n := node.(type) {
		case map[string]any:
			if last {
				n[k] = nv
			}
			node = n[k]
		case []any:
			j, _ := strconv.Atoi(k)
			if last {
				n[j] = nv
			}
			node = n[j]
		}
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestR4: a value the schema accepts only by its number value or its syntax
// is reported with the REQs the schema reports for a clearly invalid value
// at the same place.
func TestR4(t *testing.T) {
	val := load(t)
	health, md := vector(t, "V-PAY-003"), vector(t, "V-PAY-011")
	enumValue := []string{"EnumDataTypes", "0", "EnumDefinition", "Fields", "0", "Value"}
	for _, c := range []struct {
		name     string
		v        testvec.Vector
		tokens   []string
		r4, bad  any
		wantReqs []string // optional: expected REQs
	}{
		{"integer 1.0", health, []string{"Messages", "0", "DataSetWriterId"}, json.Number("3456.0"), json.Number("3456.5"), nil},
		{"integer 1e2", health, []string{"Messages", "0", "SequenceNumber"}, json.Number("1e2"), json.Number("100.5"), nil},
		{"negative zero", health, []string{"Messages", "0", "SequenceNumber"}, json.Number("-0"), json.Number("-1"), nil},
		{"calendar", health, []string{"Messages", "0", "Timestamp"}, "2021-02-30T22:25:10.094Z", "2021-13-16T22:25:10.094Z", nil},
		{"int64 range", md, enumValue, "9223372036854775808", "x", []string{"REQ-MSG-044"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := Message{Topic: c.v.Topic, Object: c.v.Object}
			ptr := finding.Pointer(c.tokens)
			m.Payload = mutate(t, c.v.Payload, c.tokens, c.bad)
			bad, err := val.Validate(m)
			if err != nil {
				t.Fatal(err)
			}
			m.Payload = mutate(t, c.v.Payload, c.tokens, c.r4)
			got, err := val.Validate(m)
			if err != nil {
				t.Fatal(err)
			}
			want := atPointer(bad.Findings, ptr, finding.LayerSchema)
			if len(want) == 0 {
				t.Fatalf("the invalid value yields no schema finding at %s: %s", ptr, dump(bad.Findings))
			}
			if c.wantReqs != nil && !slices.Equal(want, c.wantReqs) {
				t.Errorf("schema reqs %v, want %v", want, c.wantReqs)
			}
			if r := atPointer(got.Findings, ptr, finding.LayerRule); !slices.Equal(r, want) || len(got.Findings) != len(r) {
				t.Errorf("R-4 reqs %v, want %v:\n%s", r, want, dump(got.Findings))
			}
		})
	}
}

func atPointer(fs []finding.Finding, ptr, layer string) []string {
	var sel []finding.Finding
	for _, f := range fs {
		if f.Pointer == ptr && f.Layer == layer {
			sel = append(sel, f)
		}
	}
	return finding.Reqs(sel)
}
