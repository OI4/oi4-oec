package schemas

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printer = message.NewPrinter(language.English)

// R-4 (ADR-0019 §5): JSON Schema compares numbers by value and strings by
// syntax. The oracle additionally requires the integer syntax for integers
// (1.0, 1e2 and -0 are no unsigned integers), the calendar validity of a
// DateTime (no 30 February) and the Int64 range of a 19-digit Int64 string.
// The probes find such values, replace them by a value the schema itself
// rejects at the same place and report the REQs of the new errors of the
// expected keyword at that place. A value at a place without such a
// constraint (a free string, a Double) yields no finding.

type probe struct {
	tokens  []string
	value   any
	matches func(kind any) bool
	detail  string
}

var (
	plainInteger = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)$`)
	dateTimeLike = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}[Tt]`)
	int64Like    = regexp.MustCompile(`^-?[1-9][0-9]{18}$`)
)

func isType(k any) bool    { _, ok := k.(*kind.Type); return ok }
func isPattern(k any) bool { _, ok := k.(*kind.Pattern); return ok }
func isMinimum(k any) bool {
	switch k.(type) {
	case *kind.Minimum, *kind.ExclusiveMinimum:
		return true
	}
	return false
}

// probes returns the R-4 candidates of an instance.
func probes(v any, tokens []string) []probe {
	var out []probe
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			out = append(out, probes(e, append(append([]string(nil), tokens...), k))...)
		}
	case []any:
		for i, e := range t {
			out = append(out, probes(e, append(append([]string(nil), tokens...), strconv.Itoa(i)))...)
		}
	case json.Number:
		s := string(t)
		r, ok := new(big.Rat).SetString(s)
		if !ok || !r.IsInt() {
			break
		}
		if !plainInteger.MatchString(s) {
			half := new(big.Rat).Add(r, big.NewRat(1, 2))
			out = append(out, probe{tokens, json.Number(half.FloatString(1)), isType,
				fmt.Sprintf("the integer %s is not written in integer syntax", s)})
		}
		if r.Sign() == 0 && s[0] == '-' {
			out = append(out, probe{tokens, json.Number("-1"), isMinimum,
				fmt.Sprintf("%s is no unsigned integer", s)})
		}
	case string:
		if dateTimeLike.MatchString(t) && len(t) > 7 {
			if _, err := time.Parse(time.RFC3339Nano, t); err != nil {
				out = append(out, probe{tokens, t[:5] + "13" + t[7:], isPattern,
					fmt.Sprintf("%q is no valid date and time (calendar)", t)})
			}
		}
		if int64Like.MatchString(t) {
			if _, err := strconv.ParseInt(t, 10, 64); err != nil {
				out = append(out, probe{tokens, t + "0", isPattern,
					fmt.Sprintf("%s exceeds the Int64 range", t)})
			}
		}
	}
	return out
}

func replace(v any, tokens []string, nv any) any {
	if len(tokens) == 0 {
		return nv
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = e
		}
		out[tokens[0]] = replace(t[tokens[0]], tokens[1:], nv)
		return out
	case []any:
		out := append([]any(nil), t...)
		i, _ := strconv.Atoi(tokens[0])
		out[i] = replace(t[i], tokens[1:], nv)
		return out
	}
	return v
}

func (s *Set) probe(id string, inst any, orig []leaf) ([]finding.Finding, error) {
	ps := probes(inst, nil)
	if len(ps) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, l := range orig {
		seen[l.key()] = true
	}
	var out []finding.Finding
	for _, p := range ps {
		ls, err := s.leaves(id, replace(inst, p.tokens, p.value))
		if err != nil {
			return nil, err
		}
		ptr := finding.Pointer(p.tokens)
		reported := map[string]bool{}
		for _, l := range ls {
			if seen[l.key()] || finding.Pointer(l.err.InstanceLocation) != ptr || !p.matches(l.err.ErrorKind) {
				continue
			}
			for _, r := range s.reqs(l) {
				if !reported[r] {
					reported[r] = true
					out = append(out, finding.Finding{Req: r, Layer: finding.LayerRule, Pointer: ptr, Detail: p.detail + " (R-4)"})
				}
			}
		}
	}
	return out, nil
}
