package ilink

import (
	"slices"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/testvec"
)

func TestVectors(t *testing.T) {
	vs, err := testvec.Load("il")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, v := range vs {
		switch {
		case strings.HasPrefix(v.ID, "V-DNP-"):
			n++
			if got := DnpEncode(v.Input); got != v.Output {
				t.Errorf("%s: DnpEncode(%q) = %q, want %q", v.ID, v.Input, got, v.Output)
			}
		case strings.HasPrefix(v.ID, "V-IL-"):
			n++
			var codes, reqs []string
			add := func(vs ...Violation) {
				for _, x := range vs {
					codes = append(codes, x.Code)
					if !slices.Contains(reqs, x.Req) {
						reqs = append(reqs, x.Req)
					}
				}
			}
			s := ""
			if v.IL != nil {
				s = *v.IL
			}
			if v.Encoded != nil {
				dec, viol := CheckElement(*v.Encoded)
				if viol != nil {
					add(*viol)
				} else {
					if v.IL != nil && dec != *v.IL {
						t.Errorf("%s: decodes to %q, want %q", v.ID, dec, *v.IL)
					}
					s = dec
				}
			}
			if codes == nil {
				add(Validate(s)...)
			}
			if v.Expected == "valid" {
				if codes != nil {
					t.Errorf("%s: valid link rejected: %v", v.ID, codes)
				}
				if v.Encoded != nil && v.IL != nil && Encode(*v.IL) != *v.Encoded {
					t.Errorf("%s: Encode = %q, want %q", v.ID, Encode(*v.IL), *v.Encoded)
				}
				continue
			}
			if !slices.Equal(codes, v.Codes) {
				t.Errorf("%s: codes %v, want %v", v.ID, codes, v.Codes)
			}
			slices.Sort(reqs)
			want := slices.Clone(v.Violates)
			slices.Sort(want)
			if !slices.Equal(reqs, want) {
				t.Errorf("%s: reqs %v, want %v", v.ID, reqs, want)
			}
		}
	}
	if n == 0 {
		t.Fatal("no vectors")
	}
}
