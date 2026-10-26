package topic

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OI4/oec/conformance/internal/testvec"
)

func TestVectors(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	vs, err := testvec.Load("topic")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) == 0 {
		t.Fatal("no topic vectors")
	}
	for _, v := range vs {
		r, err := s.Check(v.Topic)
		if err != nil {
			t.Errorf("%s: %v", v.ID, err)
			continue
		}
		if v.Expected == "valid" {
			if !r.Valid {
				t.Errorf("%s: valid topic rejected: %v", v.ID, r.Findings)
				continue
			}
			want := map[string]string{}
			for k, x := range v.Parse {
				if k != "filter_values" {
					want[k] = x.(string)
				}
			}
			p := r.Parsed
			got := map[string]string{"rule": p.Rule, "service_type": p.ServiceType, "app_id": p.AppID, "app_id_il": p.AppIDIL, "method": p.Method, "resource": p.Resource}
			for k, x := range map[string]string{"source": p.Source, "source_il": p.SourceIL, "filter": p.Filter} {
				if x != "" {
					got[k] = x
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: parse %v, want %v", v.ID, got, want)
			}
			var wantFV []string
			if fv, ok := v.Parse["filter_values"].([]any); ok {
				for _, x := range fv {
					wantFV = append(wantFV, x.(string))
				}
			}
			if !slices.Equal(p.FilterValues, wantFV) {
				t.Errorf("%s: filter values %q, want %q", v.ID, p.FilterValues, wantFV)
			}
			if !slices.Equal(r.Warnings, v.Warnings) {
				t.Errorf("%s: warnings %v, want %v", v.ID, r.Warnings, v.Warnings)
			}
			continue
		}
		if r.Valid {
			t.Errorf("%s: invalid topic accepted", v.ID)
			continue
		}
		if r.Failure != v.Fails {
			t.Errorf("%s: failure %s, want %s", v.ID, r.Failure, v.Fails)
		}
		got := r.Reqs()
		slices.Sort(got)
		want := slices.Clone(v.Violates)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: violates %v, want %v (%v)", v.ID, got, want, r.Findings)
		}
	}
}
