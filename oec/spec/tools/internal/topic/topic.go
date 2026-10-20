// Package topic loads the formal topic grammar (topic/topic.abnf) and the
// topic columns of the Method x Resource matrix, keeps both consistent,
// generates the derived regular expressions (topic/topic-patterns.json) and
// analyses single topics for the vector checks.
package topic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/OI4/oec/spec/tools/internal/abnf"
	"github.com/OI4/oec/spec/tools/internal/il"
	"github.com/OI4/oec/spec/tools/internal/schema"
	"github.com/OI4/oec/spec/tools/internal/yamlstrict"
)

// Paths inside the spec directory.
const (
	GrammarFile  = "topic/topic.abnf"
	PatternsFile = "topic/topic-patterns.json"
	MatrixFile   = "matrix/method-resource.yaml"
	MatrixSchema = "matrix/method-resource.schema.json"
)

// Rule names the tool relies on.
const (
	RuleTopic     = "topic"
	RuleStructure = "topic-structure"
)

// Presence of Source or Filter in a topic form.
const (
	Required  = "required"
	Optional  = "optional"
	Forbidden = "forbidden"
)

// Combination is one row of matrix/method-resource.yaml.
type Combination struct {
	Method     string   `json:"method"`
	Resource   string   `json:"resource"`
	Kind       string   `json:"kind"`
	Allowed    bool     `json:"allowed"`
	Rule       string   `json:"rule,omitempty"`
	Source     string   `json:"source,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	FilterRule string   `json:"filter_rule,omitempty"`
	Refs       []string `json:"refs"`
	Notes      string   `json:"notes,omitempty"`
	Payload    *Payload `json:"payload,omitempty"`
}

// Payload holds the payload columns of an allowed combination (C-028): the
// message type, the DataSetClassId and DataSetWriterId rules, the content of
// the message, the helper DataSets it may carry and how it is answered.
type Payload struct {
	Status      string   `json:"status"`
	MessageType string   `json:"message_type"`
	ClassID     string   `json:"class_id"`
	WriterID    string   `json:"writer_id"`
	Content     string   `json:"content"`
	Helpers     []string `json:"helpers,omitempty"`
	Answer      string   `json:"answer"`
	Reqs        []string `json:"reqs"`
	Notes       string   `json:"notes,omitempty"`
}

// Matrix is matrix/method-resource.yaml.
type Matrix struct {
	Baseline     string        `json:"baseline"`
	Combinations []Combination `json:"combinations"`
}

// Spec holds the grammar, its compiled regexes and the matrix.
type Spec struct {
	Grammar *abnf.Grammar
	Matrix  Matrix
	regex   map[string]*regexp.Regexp
	pattern map[string]string
}

// Load reads and checks the grammar file and the schema-validated matrix.
func Load(root string) (*Spec, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GrammarFile)))
	if err != nil {
		return nil, err
	}
	g, err := abnf.Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", GrammarFile, err)
	}
	for _, r := range []string{RuleTopic, RuleStructure, "method", "resource", "service", "filter-level"} {
		if !g.Has(r) {
			return nil, fmt.Errorf("%s: rule %s is missing", GrammarFile, r)
		}
	}
	s := &Spec{Grammar: g, regex: map[string]*regexp.Regexp{}, pattern: map[string]string{}}
	for _, name := range g.Order {
		p, err := g.Regex(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", GrammarFile, err)
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("%s: rule %s: generated regex does not compile (RE2): %w", GrammarFile, name, err)
		}
		s.pattern[name], s.regex[name] = p, re
	}
	v, err := yamlstrict.LoadFile(filepath.Join(root, filepath.FromSlash(MatrixFile)))
	if err != nil {
		return nil, err
	}
	val, err := schema.Compile(filepath.Join(root, filepath.FromSlash(MatrixSchema)))
	if err != nil {
		return nil, err
	}
	if err := val.Validate(v); err != nil {
		return nil, fmt.Errorf("%s: %w", MatrixFile, err)
	}
	if err := yamlstrict.Decode(v, &s.Matrix); err != nil {
		return nil, fmt.Errorf("%s: %w", MatrixFile, err)
	}
	return s, nil
}

// Pattern returns the generated regex of a rule.
func (s *Spec) Pattern(rule string) string { return s.pattern[strings.ToLower(rule)] }

// match runs the ABNF interpreter and the generated RE2 regex and reports a
// disagreement as error.
func (s *Spec) match(rule, in string) (bool, error) {
	ok, err := s.Grammar.Match(rule, in)
	if err != nil {
		return false, err
	}
	if re := s.regex[strings.ToLower(rule)]; re != nil && re.MatchString(in) != ok {
		return false, fmt.Errorf("rule %s: ABNF matcher (%v) and generated regex (%v) disagree on %q", rule, ok, !ok, in)
	}
	return ok, nil
}

// PatternsFileContent renders topic/topic-patterns.json.
func (s *Spec) PatternsFileContent() (string, error) {
	type entry struct {
		Rule    string `json:"rule"`
		Pattern string `json:"pattern"`
	}
	doc := struct {
		Comment  string  `json:"$comment"`
		Source   string  `json:"source"`
		Baseline string  `json:"baseline"`
		Dialect  string  `json:"dialect"`
		Patterns []entry `json:"patterns"`
	}{
		Comment:  "Generated by 'spectool topic' from topic.abnf - do not edit.",
		Source:   "topic.abnf",
		Baseline: s.Matrix.Baseline,
		Dialect: "Common subset of ECMA-262 (compile with the u flag), RE2 (Go) and Rust regex: " +
			"anchored with ^ and $, non-capturing groups only, \\xHH escapes, no look-around or back-references. " +
			"Each pattern matches the complete string; in Python use re.fullmatch because $ also matches before a final newline. " +
			"Character classes that cover more than U+00FF are written as negated classes. " +
			"The patterns check the syntax only; AppId and Source additionally need the Identification Link check of MG 4.1.",
	}
	for _, name := range s.Grammar.Order {
		doc.Patterns = append(doc.Patterns, entry{Rule: s.Grammar.Rules[name].Name, Pattern: s.pattern[name]})
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Parsed is the decomposition of a valid topic.
type Parsed struct {
	ServiceType  string
	AppID        string
	AppIDIL      string
	Method       string
	Resource     string
	Source       string
	SourceIL     string
	Filter       string   // wire form of all Filter levels, joined by "/"
	FilterValues []string // percent-decoded Filter levels
}

// Failure kinds of Analyze.
const (
	FailGrammar = "grammar"
	FailIL      = "identification-link"
)

// Warning codes of Analyze.
const (
	WarnWhitespace = "whitespace"
	WarnControl    = "control-character"
)

// Result is the verdict for one topic.
type Result struct {
	Valid    bool
	Rule     string
	Failure  string
	Detail   string
	Parsed   *Parsed
	Warnings []string
}

// Analyze checks one topic: grammar (ABNF interpreter and generated regex
// must agree), the matching combination and the Identification Links of
// AppId and Source. An error reports an inconsistency of the tooling.
func (s *Spec) Analyze(topic string) (Result, error) {
	ok, err := s.match(RuleTopic, topic)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		st, err := s.match(RuleStructure, topic)
		if err != nil {
			return Result{}, err
		}
		detail := "does not match the topic structure of MG 7"
		if st {
			detail = "matches the generic structure, but not an allowed Method x Resource form"
		}
		return Result{Failure: FailGrammar, Detail: detail}, nil
	}
	var rules []string
	for _, c := range s.Matrix.Combinations {
		if !c.Allowed {
			continue
		}
		m, err := s.match(c.Rule, topic)
		if err != nil {
			return Result{}, err
		}
		if m {
			rules = append(rules, c.Rule)
		}
	}
	if len(rules) != 1 {
		return Result{}, fmt.Errorf("topic %q matches %d combination rules %v", topic, len(rules), rules)
	}
	lv := strings.Split(topic, "/")
	p := &Parsed{ServiceType: lv[1], AppID: lv[2], Method: lv[3], Resource: lv[4]}
	if len(lv) > 5 {
		p.Source = lv[5]
	}
	if len(lv) > 6 {
		p.Filter = strings.Join(lv[6:], "/")
		for _, l := range lv[6:] {
			p.FilterValues = append(p.FilterValues, Decode(l))
		}
	}
	res := Result{Valid: true, Rule: rules[0], Parsed: p}
	check := func(name, enc string, out *string) {
		if enc == "" || !res.Valid {
			return
		}
		dec, v, err := il.CheckTopicElement(enc)
		*out = dec
		switch {
		case err != nil:
			res.Valid, res.Failure, res.Detail = false, FailIL, name+": "+err.Error()
		case v != nil:
			res.Valid, res.Failure, res.Detail = false, FailIL, fmt.Sprintf("%s %q: %v", name, dec, v)
		}
	}
	check("AppId", p.AppID, &p.AppIDIL)
	check("Source", p.Source, &p.SourceIL)
	res.Warnings = warnings(p)
	return res, nil
}

// Decode percent-decodes one Filter level (MG 7.7).
func Decode(level string) string {
	return strings.NewReplacer("%2B", "+", "%23", "#", "%2F", "/", "%25", "%").Replace(level)
}

func warnings(p *Parsed) []string {
	var ws, ctl bool
	for _, v := range p.FilterValues {
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
