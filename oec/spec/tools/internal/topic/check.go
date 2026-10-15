package topic

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/abnf"
	"github.com/OI4/oec/spec/tools/internal/il"
)

// CheckOptions control the sample-based consistency check.
type CheckOptions struct {
	Samples int   // random topics per allowed combination
	Seed    int64 // fixed seed keeps the check reproducible
}

// Check verifies that the matrix and the grammar describe the same topic
// forms: closed sets, rule names, the "topic" alternatives and, based on
// generated and constructed topics, the presence of Source and Filter.
func (s *Spec) Check(opt CheckOptions) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	methods, err := s.literals("method")
	if err != nil {
		return []error{err}
	}
	resources, err := s.literals("resource")
	if err != nil {
		return []error{err}
	}
	services, err := s.literals("service")
	if err != nil {
		return []error{err}
	}
	kind := map[string]string{}
	for _, r := range resources {
		kind[r] = "resource"
	}
	for _, r := range services {
		kind[r] = "service"
	}

	seen := map[string]bool{}
	allowed := map[string]Combination{}
	for _, c := range s.Matrix.Combinations {
		at := fmt.Sprintf("%s: %s/%s", MatrixFile, c.Method, c.Resource)
		key := c.Method + "/" + c.Resource
		if seen[key] {
			add("%s: duplicate row", at)
		}
		seen[key] = true
		if k, ok := kind[c.Resource]; !ok {
			add("%s: %s is neither a resource nor a service of %s", at, c.Resource, GrammarFile)
		} else if k != c.Kind {
			add("%s: kind %s, but the grammar lists it as %s", at, c.Kind, k)
		}
		if !c.Allowed {
			continue
		}
		if want := strings.ToLower(c.Method + "-" + c.Resource); c.Rule != want {
			add("%s: rule %s must be named %s", at, c.Rule, want)
		}
		if !s.Grammar.Has(c.Rule) {
			add("%s: rule %s is not defined in %s", at, c.Rule, GrammarFile)
			continue
		}
		switch {
		case c.Filter == Forbidden && c.FilterRule != "":
			add("%s: filter_rule given although the Filter is forbidden", at)
		case c.Filter != Forbidden && c.FilterRule == "":
			add("%s: filter_rule missing", at)
		case c.FilterRule != "" && !s.Grammar.Has(c.FilterRule):
			add("%s: filter_rule %s is not defined in %s", at, c.FilterRule, GrammarFile)
			continue
		}
		if c.Source == Forbidden && c.Filter != Forbidden {
			add("%s: a Filter requires a Source", at)
		}
		allowed[c.Rule] = c
	}
	for _, m := range methods {
		for r := range kind {
			if !seen[m+"/"+r] {
				add("%s: row %s/%s is missing", MatrixFile, m, r)
			}
		}
	}

	alts, err := s.refs(RuleTopic)
	if err != nil {
		add("%v", err)
	}
	var missing, extra []string
	for _, a := range alts {
		if _, ok := allowed[a]; !ok {
			extra = append(extra, a)
		}
	}
	for r := range allowed {
		if !contains(alts, r) {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		add("%s: rule topic lacks the allowed combination(s) %s", GrammarFile, strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		add("%s: rule topic contains %s, not allowed in %s", GrammarFile, strings.Join(extra, ", "), MatrixFile)
	}
	if len(errs) > 0 {
		return errs
	}
	errs = append(errs, s.checkSamples(opt)...)
	return append(errs, s.checkConstructed()...)
}

// checkSamples generates random topics per allowed combination.
func (s *Spec) checkSamples(opt CheckOptions) []error {
	var errs []error
	gen := &abnf.Generator{G: s.Grammar, Rand: rand.New(rand.NewSource(opt.Seed))}
	for _, c := range s.Matrix.Combinations {
		if !c.Allowed {
			continue
		}
		at := fmt.Sprintf("%s/%s (%s)", c.Method, c.Resource, c.Rule)
		count := map[string]int{}
		for i := 0; i < opt.Samples; i++ {
			t, err := gen.Generate(c.Rule)
			if err != nil {
				return append(errs, err)
			}
			if err := s.checkSample(c, t, i < 10); err != nil {
				errs = append(errs, fmt.Errorf("%s: sample %q: %w", at, t, err))
				break
			}
			lv := strings.Split(t, "/")
			count[presenceKey("source", len(lv) > 5)]++
			count[presenceKey("filter", len(lv) > 6)]++
		}
		for _, el := range []struct{ name, want string }{{"source", c.Source}, {"filter", c.Filter}} {
			with, without := count[el.name+"+"], count[el.name+"-"]
			ok := el.want == Required && without == 0 ||
				el.want == Forbidden && with == 0 ||
				el.want == Optional && with > 0 && without > 0
			if !ok {
				errs = append(errs, fmt.Errorf("%s: matrix says %s is %s, but %d of %d samples contain it", at, el.name, el.want, with, with+without))
			}
		}
	}
	return errs
}

func presenceKey(name string, present bool) string {
	if present {
		return name + "+"
	}
	return name + "-"
}

func (s *Spec) checkSample(c Combination, t string, unique bool) error {
	for _, r := range []string{c.Rule, RuleTopic, RuleStructure} {
		ok, err := s.match(r, t)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("does not match rule %s", r)
		}
	}
	lv := strings.Split(t, "/")
	if lv[3] != c.Method || lv[4] != c.Resource {
		return fmt.Errorf("levels 4/5 are %s/%s", lv[3], lv[4])
	}
	if len(lv) > 6 {
		ok, err := s.match(c.FilterRule, strings.Join(lv[6:], "/"))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("Filter does not match %s", c.FilterRule)
		}
	}
	if unique {
		res, err := s.Analyze(t)
		if err != nil {
			return err
		}
		if res.Rule != c.Rule {
			return fmt.Errorf("analysed as %s", res.Rule)
		}
	}
	return nil
}

// checkConstructed builds, for every row, the topic without Source, with
// Source and with Source and Filter and compares the verdict of rule
// "topic" with the matrix.
func (s *Spec) checkConstructed() []error {
	var errs []error
	gen := &abnf.Generator{G: s.Grammar, Rand: rand.New(rand.NewSource(7))}
	base := "Oi4v2/OTConnector/" + il.Encode("provider.com/FieldDataService/FDS-001/1200-0345") + "/"
	src := il.Encode("company.id/A87BAD34H")
	for _, c := range s.Matrix.Combinations {
		filter := "x"
		if c.FilterRule != "" {
			f, err := gen.Generate(c.FilterRule)
			if err != nil {
				return append(errs, err)
			}
			filter = f
		}
		v0 := base + c.Method + "/" + c.Resource
		cases := []struct {
			topic string
			want  bool
		}{
			{v0, c.Allowed && c.Source != Required},
			{v0 + "/" + src, c.Allowed && c.Source != Forbidden && c.Filter != Required},
			{v0 + "/" + src + "/" + filter, c.Allowed && c.Filter != Forbidden},
		}
		for _, tc := range cases {
			for _, r := range []string{RuleTopic, RuleStructure} {
				got, err := s.match(r, tc.topic)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				want := tc.want || r == RuleStructure
				if got != want {
					errs = append(errs, fmt.Errorf("%s/%s: rule %s on %q = %v, matrix expects %v", c.Method, c.Resource, r, tc.topic, got, want))
				}
			}
		}
	}
	return errs
}

// literals returns the string alternatives of a rule that consists only of
// case-sensitive literals.
func (s *Spec) literals(rule string) ([]string, error) {
	var out []string
	for _, n := range alternatives(s.Grammar.Rules[rule].Node) {
		l, ok := n.(abnf.Literal)
		if !ok || !l.CaseSensitive {
			return nil, fmt.Errorf("%s: rule %s must be an alternation of %%s\"...\" literals", GrammarFile, rule)
		}
		out = append(out, l.Text)
	}
	return out, nil
}

// refs returns the rule names of a rule that is an alternation of references.
func (s *Spec) refs(rule string) ([]string, error) {
	var out []string
	for _, n := range alternatives(s.Grammar.Rules[rule].Node) {
		r, ok := n.(abnf.Ref)
		if !ok {
			return nil, fmt.Errorf("%s: rule %s must be an alternation of rule names", GrammarFile, rule)
		}
		out = append(out, r.Name)
	}
	return out, nil
}

func alternatives(n abnf.Node) []abnf.Node {
	if a, ok := n.(abnf.Alt); ok {
		return a.Items
	}
	return []abnf.Node{n}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
