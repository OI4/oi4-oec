// Package topic checks an OEC v2 topic (MG 7) for D7 validate: it diagnoses
// the first failing element with the REQ it violates, decodes the
// Identification Links of AppId and Source and parses the topic into its
// elements. The data come from the spec baseline (oec/spec): the generated
// topic patterns and the Method x Resource matrix (ADR-0019 §5).
package topic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/spec"
	"go.yaml.in/yaml/v3"
)

// Namespace is the first topic level (MG 7.1).
const Namespace = "Oi4v2"

// Failure kinds of a topic.
const (
	FailGrammar = "grammar"
	FailIL      = "identification-link"
)

// Warning codes of a valid topic.
const (
	WarnControl    = "control-character"
	WarnWhitespace = "whitespace"
)

// Presence of Source or Filter in a combination.
const (
	Required  = "required"
	Optional  = "optional"
	Forbidden = "forbidden"
)

// Combination is one row of the Method x Resource matrix.
type Combination struct {
	Method     string `yaml:"method"`
	Resource   string `yaml:"resource"`
	Kind       string `yaml:"kind"`
	Allowed    bool   `yaml:"allowed"`
	Rule       string `yaml:"rule"`
	Source     string `yaml:"source"`
	Filter     string `yaml:"filter"`
	FilterRule string `yaml:"filter_rule"`
}

// Spec holds the compiled topic data of the baseline.
type Spec struct {
	Baseline     string
	Combinations []Combination
	patterns     map[string]*regexp.Regexp
	formRules    []string
}

// Load reads the topic patterns and the matrix from the embedded baseline.
func Load() (*Spec, error) {
	raw, err := spec.FS.ReadFile(spec.PatternsFile)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Baseline string `json:"baseline"`
		Patterns []struct {
			Rule    string `json:"rule"`
			Pattern string `json:"pattern"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.PatternsFile, err)
	}
	s := &Spec{Baseline: doc.Baseline, patterns: map[string]*regexp.Regexp{}}
	for _, p := range doc.Patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: rule %s: %w", spec.PatternsFile, p.Rule, err)
		}
		s.patterns[p.Rule] = re
	}
	raw, err = spec.FS.ReadFile(spec.MatrixFile)
	if err != nil {
		return nil, err
	}
	var m struct {
		Baseline     string        `yaml:"baseline"`
		Combinations []Combination `yaml:"combinations"`
	}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.MatrixFile, err)
	}
	if m.Baseline != s.Baseline {
		return nil, fmt.Errorf("baseline of %s (%s) differs from %s (%s)", spec.MatrixFile, m.Baseline, spec.PatternsFile, s.Baseline)
	}
	s.Combinations = m.Combinations
	for _, c := range s.Combinations {
		if !c.Allowed {
			continue
		}
		if s.patterns[c.Rule] == nil {
			return nil, fmt.Errorf("no pattern for rule %s of %s %s", c.Rule, c.Method, c.Resource)
		}
		if c.FilterRule != "" && s.patterns[c.FilterRule] == nil {
			return nil, fmt.Errorf("no pattern for filter rule %s", c.FilterRule)
		}
		s.formRules = append(s.formRules, c.Rule)
	}
	for _, r := range []string{"topic", "service-type", "method", "resource", "service", "filter-level"} {
		if s.patterns[r] == nil {
			return nil, fmt.Errorf("%s: rule %s missing", spec.PatternsFile, r)
		}
	}
	return s, nil
}

func (s *Spec) match(rule, in string) bool { return s.patterns[rule].MatchString(in) }

// Match reports whether in matches the topic pattern rule; an unknown rule
// matches nothing.
func (s *Spec) Match(rule, in string) bool {
	re := s.patterns[rule]
	return re != nil && re.MatchString(in)
}

// Combination returns the matrix row of a Method and a Resource or service.
func (s *Spec) Combination(method, resource string) (Combination, bool) {
	for _, c := range s.Combinations {
		if c.Method == method && c.Resource == resource {
			return c, true
		}
	}
	return Combination{}, false
}

// Parsed is the decomposition of a topic whose grammar is valid.
type Parsed struct {
	Rule         string   `json:"rule"`
	ServiceType  string   `json:"service_type"`
	AppID        string   `json:"app_id"`
	AppIDIL      string   `json:"app_id_il"`
	Method       string   `json:"method"`
	Resource     string   `json:"resource"`
	Source       string   `json:"source,omitempty"`
	SourceIL     string   `json:"source_il,omitempty"`
	Filter       string   `json:"filter,omitempty"`
	FilterValues []string `json:"filter_values,omitempty"`
}

// Finding is one violated requirement of a topic.
type Finding struct {
	Req     string `json:"req"`
	Element string `json:"element"`
	Detail  string `json:"detail"`
}

// Result is the verdict for one topic.
type Result struct {
	Valid    bool      `json:"valid"`
	Failure  string    `json:"failure,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
	Parsed   *Parsed   `json:"parsed,omitempty"`
	Warnings []string  `json:"warnings,omitempty"`
}

// Reqs returns the REQ ids of the findings.
func (r Result) Reqs() []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range r.Findings {
		if !seen[f.Req] {
			seen[f.Req] = true
			out = append(out, f.Req)
		}
	}
	return out
}

// Check diagnoses one topic. An error reports an inconsistency between the
// stepwise diagnosis and the generated form patterns (an internal error).
func (s *Spec) Check(t string) (Result, error) {
	res, p := s.grammar(t)
	var rules []string
	for _, r := range s.formRules {
		if s.match(r, t) {
			rules = append(rules, r)
		}
	}
	switch {
	case res.Failure == "" && len(rules) != 1:
		return Result{}, fmt.Errorf("topic %q passes the element checks but matches %d form rules %v", t, len(rules), rules)
	case res.Failure != "" && len(rules) != 0:
		return Result{}, fmt.Errorf("topic %q fails %v but matches the form rule %s", t, res.Reqs(), rules[0])
	case res.Failure != "":
		return res, nil
	}
	if !s.match("topic", t) {
		return Result{}, fmt.Errorf("topic %q matches form rule %s, but not the rule topic", t, rules[0])
	}
	p.Rule = rules[0]
	res.Parsed = p
	ilCheck := func(name, dec string) {
		for _, v := range ilink.Validate(dec) {
			res.Findings = append(res.Findings, Finding{Req: v.Req, Element: name, Detail: fmt.Sprintf("Identification Link %q: %s", dec, v.Detail)})
		}
	}
	ilCheck("AppId", p.AppIDIL)
	if p.Source != "" {
		ilCheck("Source", p.SourceIL)
	}
	if len(res.Findings) > 0 {
		res.Failure = FailIL
	} else {
		res.Valid = true
	}
	res.Warnings = warnings(p.FilterValues)
	return res, nil
}

// grammar runs the element checks in topic order; the first failing stage
// determines the findings.
func (s *Spec) grammar(t string) (Result, *Parsed) {
	fail := func(f ...Finding) (Result, *Parsed) {
		return Result{Failure: FailGrammar, Findings: f}, nil
	}
	lv := strings.Split(t, "/")
	if lv[0] != Namespace {
		return fail(Finding{"REQ-TOP-001", "Namespace", fmt.Sprintf("the topic starts with %q, not with the namespace %s", lv[0], Namespace)})
	}
	if len(lv) < 5 {
		return fail(Finding{"REQ-TOP-001", "Topic", fmt.Sprintf("%d levels; a topic has at least Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>", len(lv))})
	}
	for i := 0; i < len(lv) && i < 6; i++ {
		if strings.ContainsAny(lv[i], "\r\n") {
			return fail(Finding{"REQ-TOP-001", "Topic", fmt.Sprintf("line break in topic level %d", i+1)})
		}
	}
	p := &Parsed{ServiceType: lv[1], AppID: lv[2], Method: lv[3], Resource: lv[4]}
	if !s.match("service-type", p.ServiceType) {
		return fail(Finding{"REQ-TOP-005", "ServiceType", fmt.Sprintf("%q is no ServiceType of MG 7.2", p.ServiceType)})
	}
	if p.AppID == "" {
		return fail(Finding{"REQ-TOP-009", "AppId", "the AppId level is empty"})
	}
	dec, v := ilink.CheckElement(p.AppID)
	if v != nil {
		return fail(Finding{v.Req, "AppId", v.Detail})
	}
	p.AppIDIL = dec
	if !s.match("method", p.Method) {
		return fail(Finding{"REQ-TOP-011", "Method", fmt.Sprintf("%q is no Method of MG 7.4", p.Method)})
	}
	if !s.match("resource", p.Resource) && !s.match("service", p.Resource) {
		return fail(Finding{"REQ-TOP-012", "Resource", fmt.Sprintf("%q is neither a Resource nor a common service of MG 7.5", p.Resource)})
	}
	c, ok := s.Combination(p.Method, p.Resource)
	if !ok || !c.Allowed {
		return fail(Finding{"REQ-TOP-013", "Method", fmt.Sprintf("%s %s is no allowed combination of MG 7.4/9", p.Method, p.Resource)})
	}
	hasSource, hasFilter := len(lv) > 5, len(lv) > 6
	if c.Source == Required && !hasSource {
		return fail(Finding{"REQ-TOP-013", "Source", fmt.Sprintf("%s %s requires a Source", p.Method, p.Resource)})
	}
	if c.Source == Forbidden && hasSource {
		return fail(Finding{"REQ-TOP-013", "Source", fmt.Sprintf("%s %s has no Source", p.Method, p.Resource)})
	}
	if hasSource {
		p.Source = lv[5]
		if p.Source == "" {
			return fail(Finding{"REQ-TOP-018", "Source", "the Source level is empty"})
		}
		dec, v := ilink.CheckElement(p.Source)
		if v != nil {
			return fail(Finding{v.Req, "Source", v.Detail})
		}
		p.SourceIL = dec
	}
	if c.Filter == Required && !hasFilter {
		return fail(Finding{"REQ-TOP-013", "Filter", fmt.Sprintf("%s %s requires a Filter", p.Method, p.Resource)})
	}
	if c.Filter == Forbidden && hasFilter {
		f := Finding{"REQ-TOP-013", "Filter", fmt.Sprintf("%s %s has no Filter", p.Method, p.Resource)}
		if c.Kind == "service" && p.Resource == "NewDataSetWriterId" {
			return fail(Finding{"REQ-ACT-020", "Filter", "the topic of the common service NewDataSetWriterId has no Filter"}, f)
		}
		return fail(f)
	}
	if !hasFilter {
		return Result{}, p
	}
	levels := lv[6:]
	p.Filter = strings.Join(levels, "/")
	if r, failed := s.filter(c, levels); failed {
		return r, nil
	}
	for _, l := range levels {
		p.FilterValues = append(p.FilterValues, Decode(l))
	}
	return Result{}, p
}

func (s *Spec) filter(c Combination, levels []string) (Result, bool) {
	fail := func(f ...Finding) (Result, bool) {
		return Result{Failure: FailGrammar, Findings: f}, true
	}
	var empty bool
	filled := make([]string, len(levels))
	for i, l := range levels {
		filled[i] = l
		if l == "" {
			empty, filled[i] = true, "x"
		}
	}
	if empty {
		f := []Finding{{"REQ-TOP-021", "Filter", "the Filter contains an empty level"}}
		if c.FilterRule != "" && s.match(c.FilterRule, strings.Join(filled, "/")) {
			f = append([]Finding{{"REQ-TOP-019", "Filter", "a Filter level of " + c.FilterRule + " consists of at least one character"}}, f...)
		}
		return fail(f...)
	}
	for _, l := range levels {
		if !s.match("filter-level", l) {
			return fail(Finding{"REQ-TOP-019", "Filter", fmt.Sprintf("Filter level %q contains a character that is not percent-encoded (MG 7.7)", l)})
		}
	}
	if c.FilterRule == "" || s.match(c.FilterRule, strings.Join(levels, "/")) {
		return Result{}, false
	}
	req := "REQ-TOP-022"
	if c.FilterRule == "event-filter" {
		req = "REQ-TOP-023"
	}
	return fail(Finding{req, "Filter", fmt.Sprintf("%q is no %s of %s %s", strings.Join(levels, "/"), c.FilterRule, c.Method, c.Resource)})
}

// Decode percent-decodes one Filter level (MG 7.7).
func Decode(level string) string {
	return strings.NewReplacer("%2B", "+", "%23", "#", "%2F", "/", "%25", "%").Replace(level)
}

// MatchFilterLevel reports whether l is a valid Filter level (MG 7.7).
func (s *Spec) MatchFilterLevel(l string) bool { return s.match("filter-level", l) }

func warnings(values []string) []string {
	var ws, ctl bool
	for _, v := range values {
		for _, r := range v {
			if r <= 0x1F || r >= 0x7F && r <= 0x9F {
				ctl = true
			}
			if unicode.IsSpace(r) {
				ws = true
			}
		}
	}
	var out []string
	if ctl {
		out = append(out, WarnControl)
	}
	if ws {
		out = append(out, WarnWhitespace)
	}
	return out
}
