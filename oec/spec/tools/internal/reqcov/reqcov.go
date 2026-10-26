// Package reqcov reports the vector coverage of the requirements verified by
// the schemas and D7 validate (ADR-0019 §8/§9, ROADMAP P2 exit 2 and 3): a
// reviewed positive and a reviewed negative vector per requirement, umbrella
// requirements derived from their details, the layer that detects each
// negative payload vector and the entry schema of every allowed matrix row.
package reqcov

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/schema"
	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/vectors"
	"github.com/OI4/oec/spec/tools/internal/yamlstrict"
)

const (
	ImplicationsFile   = "coverage/implications.yaml"
	ImplicationsSchema = "coverage/implications.schema.json"
	ReportFile         = "coverage/vectors.md"
	// Relative to the schema directory (oec/schema).
	ExpectationsFile = "tests/rule-layer.yaml"
	EntrySchemaDir   = "v2/topics"
)

// WarningReqs maps a warning code of a valid topic vector to the requirement
// whose recommendation the topic does not follow (MG 7.7 "should not"). The
// vector is a negative vector of that requirement with layer "warning" and no
// positive vector of it (C-040).
var WarningReqs = map[string]string{topic.WarnWhitespace: "REQ-TOP-020"}

// Implication maps an umbrella requirement to its details.
type Implication struct {
	Umbrella  string   `json:"umbrella"`
	Details   []string `json:"details"`
	Remainder string   `json:"remainder"`
	Notes     string   `json:"notes,omitempty"`
}

// Implications is coverage/implications.yaml.
type Implications struct {
	Implications []Implication `json:"implications"`
}

// Expectation is one entry of the expectation list of the schema test suite.
type Expectation struct {
	Vector string   `json:"vector"`
	Schema []string `json:"schema"`
	Rule   []string `json:"rule"`
	Rules  []string `json:"rules"`
	Reason string   `json:"reason"`
}

// Expectations is oec/schema/tests/rule-layer.yaml.
type Expectations struct {
	Expectations []Expectation `json:"expectations"`
}

// LoadImplications reads and schema-validates the implication map (absent = empty).
func LoadImplications(root string) (*Implications, error) {
	im := &Implications{}
	p := filepath.Join(root, filepath.FromSlash(ImplicationsFile))
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return im, nil
	}
	v, err := yamlstrict.LoadFile(p)
	if err != nil {
		return nil, err
	}
	val, err := schema.Compile(filepath.Join(root, filepath.FromSlash(ImplicationsSchema)))
	if err != nil {
		return nil, err
	}
	if err := val.Validate(v); err != nil {
		return nil, fmt.Errorf("%s: %w", ImplicationsFile, err)
	}
	if err := yamlstrict.Decode(v, im); err != nil {
		return nil, fmt.Errorf("%s: %w", ImplicationsFile, err)
	}
	return im, nil
}

// LoadExpectations reads the expectation list of the schema test suite
// (absent = empty); the schema harness validates it in full.
func LoadExpectations(schemaDir string) (*Expectations, error) {
	ex := &Expectations{}
	p := filepath.Join(schemaDir, filepath.FromSlash(ExpectationsFile))
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return ex, nil
	}
	v, err := yamlstrict.LoadFile(p)
	if err != nil {
		return nil, err
	}
	if err := yamlstrict.Decode(v, ex); err != nil {
		return nil, fmt.Errorf("%s: %w", ExpectationsFile, err)
	}
	return ex, nil
}

// Neg is one negative vector of a requirement with the layer that detects it.
type Neg struct {
	ID       string
	Reviewed bool
	Layer    string // schema, rule (expectation list), R-1 (topic and IL vectors) or warning (WarningReqs)
}

// Entry is the coverage of one requirement.
type Entry struct {
	Req      catalog.Requirement
	Area     string
	Pos      []string // reviewed positive vectors
	PosDraft []string
	Neg      []Neg
	Umbrella *Implication
}

func (e *Entry) reviewedNeg() int {
	n := 0
	for _, g := range e.Neg {
		if g.Reviewed {
			n++
		}
	}
	return n
}

// Row is an allowed matrix row and its entry schema.
type Row struct {
	Method, Resource, Rule string
	Schema                 bool
}

// Result is the vector coverage.
type Result struct {
	Entries  []*Entry // schema/d7-validate requirements, by id
	Rows     []Row
	Problems []string // structural problems (always reported)
	Gaps     []string // P2 exit gate (reported with -gate)
	all      map[string]*Entry
	state    map[string]string
}

// Selected tells whether a requirement is in the scope of the report.
func Selected(r catalog.Requirement) bool {
	return r.Status != "deprecated" && (has(r.Verification, "schema") || has(r.Verification, "d7-validate"))
}

// Compute relates the vectors, the implication map, the expectation list and
// the entry schemas in schemaDir to the catalog.
func Compute(s *catalog.Spec, files []vectors.File, ts *topic.Spec, im *Implications, ex *Expectations, schemaDir string) *Result {
	r := &Result{all: map[string]*Entry{}, state: map[string]string{}}
	for _, f := range s.Files {
		for _, q := range f.Requirements {
			e := &Entry{Req: q, Area: f.Area}
			r.all[q.ID] = e
			if Selected(q) {
				r.Entries = append(r.Entries, e)
			}
		}
	}
	sort.Slice(r.Entries, func(i, j int) bool { return r.Entries[i].Req.ID < r.Entries[j].Req.ID })
	ruleOf := map[string]map[string]bool{}
	for _, x := range ex.Expectations {
		ruleOf[x.Vector] = set(x.Rule)
	}
	vecs := map[string]vectors.Vector{}
	kinds := map[string]string{}
	for _, f := range files {
		for _, v := range f.Vectors {
			vecs[v.ID], kinds[v.ID] = v, f.Kind
			reviewed := v.Status == "reviewed"
			if v.Expected == "valid" {
				warned := map[string]bool{}
				if f.Kind == "topic" {
					for _, w := range v.Warnings {
						if id := WarningReqs[w]; id != "" && !warned[id] {
							warned[id] = true
							if e := r.all[id]; e != nil {
								e.Neg = append(e.Neg, Neg{ID: v.ID, Reviewed: reviewed, Layer: "warning"})
							}
						}
					}
				}
				for _, id := range v.Reqs {
					if e := r.all[id]; e != nil && !warned[id] {
						if reviewed {
							e.Pos = append(e.Pos, v.ID)
						} else {
							e.PosDraft = append(e.PosDraft, v.ID)
						}
					}
				}
				continue
			}
			for _, id := range v.Violates {
				e := r.all[id]
				if e == nil {
					continue
				}
				layer := "schema"
				switch {
				case f.Kind != "payload":
					layer = "R-1"
				case ruleOf[v.ID][id]:
					layer = "rule"
				}
				e.Neg = append(e.Neg, Neg{ID: v.ID, Reviewed: reviewed, Layer: layer})
			}
		}
	}
	r.checkImplications(im)
	r.checkExpectations(ex, vecs, kinds)
	for _, c := range ts.Matrix.Combinations {
		if !c.Allowed || c.Rule == "" {
			continue
		}
		_, err := os.Stat(filepath.Join(schemaDir, filepath.FromSlash(EntrySchemaDir), c.Rule+".schema.json"))
		r.Rows = append(r.Rows, Row{Method: c.Method, Resource: c.Resource, Rule: c.Rule, Schema: err == nil})
	}
	for _, e := range r.Entries {
		if st := r.State(e.Req.ID); st != "covered" && st != "derived" {
			r.Gaps = append(r.Gaps, fmt.Sprintf("%s: %s", e.Req.ID, st))
		}
	}
	for _, row := range r.Rows {
		if !row.Schema {
			r.Gaps = append(r.Gaps, fmt.Sprintf("%s %s: no entry schema %s/%s.schema.json", row.Method, row.Resource, EntrySchemaDir, row.Rule))
		}
	}
	return r
}

// MissingSchemas counts the allowed matrix rows without an entry schema.
func (r *Result) MissingSchemas() int {
	n := 0
	for _, row := range r.Rows {
		if !row.Schema {
			n++
		}
	}
	return n
}

func (r *Result) checkImplications(im *Implications) {
	seen := map[string]bool{}
	for _, m := range im.Implications {
		u := r.all[m.Umbrella]
		switch {
		case u == nil:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: unknown umbrella %s", ImplicationsFile, m.Umbrella))
			continue
		case seen[m.Umbrella]:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: umbrella %s listed twice", ImplicationsFile, m.Umbrella))
			continue
		case !Selected(u.Req):
			r.Problems = append(r.Problems, fmt.Sprintf("%s: umbrella %s is deprecated or has neither schema nor d7-validate", ImplicationsFile, m.Umbrella))
		}
		seen[m.Umbrella] = true
		m := m
		u.Umbrella = &m
		for _, d := range m.Details {
			de := r.all[d]
			switch {
			case de == nil:
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %s: unknown detail %s", ImplicationsFile, m.Umbrella, d))
			case d == m.Umbrella:
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %s is its own detail", ImplicationsFile, d))
			case de.Req.Status == "deprecated":
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %s: detail %s is deprecated", ImplicationsFile, m.Umbrella, d))
			}
		}
	}
	color := map[string]int{} // 1 = on the path, 2 = done
	var visit func(id string) bool
	visit = func(id string) bool {
		switch color[id] {
		case 1:
			return true
		case 2:
			return false
		}
		color[id] = 1
		cyc := false
		if e := r.all[id]; e != nil && e.Umbrella != nil {
			for _, d := range e.Umbrella.Details {
				cyc = visit(d) || cyc
			}
		}
		color[id] = 2
		return cyc
	}
	for _, m := range im.Implications {
		if r.all[m.Umbrella] != nil && visit(m.Umbrella) {
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %s is part of an implication cycle", ImplicationsFile, m.Umbrella))
		}
	}
}

func (r *Result) checkExpectations(ex *Expectations, vecs map[string]vectors.Vector, kinds map[string]string) {
	for _, x := range ex.Expectations {
		v, ok := vecs[x.Vector]
		if !ok || kinds[x.Vector] != "payload" || v.Expected != "invalid" {
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %s is not an invalid payload vector", ExpectationsFile, x.Vector))
			continue
		}
		got := append(append([]string{}, x.Schema...), x.Rule...)
		if !sameSet(got, v.Violates) {
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %s: schema + rule = %v, the vector violates %v", ExpectationsFile, x.Vector, got, v.Violates))
		}
		for _, id := range x.Rule {
			if e := r.all[id]; e != nil && !has(e.Req.Verification, "d7-validate") {
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %s: rule-layer %s lacks verification d7-validate", ExpectationsFile, x.Vector, id))
			}
		}
		for _, id := range x.Schema {
			if e := r.all[id]; e != nil && !has(e.Req.Verification, "schema") {
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %s: schema-layer %s lacks verification schema", ExpectationsFile, x.Vector, id))
			}
		}
	}
}

// State is the coverage state of a requirement: covered (reviewed positive
// and negative vector), derived (umbrella whose details are all covered or
// derived), positive only, negative only, none, details open (umbrella),
// cycle or unknown.
func (r *Result) State(id string) string {
	if st, ok := r.state[id]; ok {
		if st == "" {
			return "cycle" // reported by checkImplications
		}
		return st
	}
	e := r.all[id]
	if e == nil {
		return "unknown"
	}
	r.state[id] = ""
	p, n := len(e.Pos) > 0, e.reviewedNeg() > 0
	st := "none"
	switch {
	case p && n:
		st = "covered"
	case p:
		st = "positive only"
	case n:
		st = "negative only"
	}
	if st != "covered" && e.Umbrella != nil {
		open := 0
		for _, d := range e.Umbrella.Details {
			if ds := r.State(d); ds != "covered" && ds != "derived" {
				open++
			}
		}
		if open == 0 {
			st = "derived"
		} else {
			st = fmt.Sprintf("details open (%d of %d)", open, len(e.Umbrella.Details))
		}
	}
	r.state[id] = st
	return st
}

// Report renders coverage/vectors.md.
func (r *Result) Report() string {
	var b strings.Builder
	b.WriteString("<!-- Generated by 'spectool coverage' – do not edit. -->\n# Vector coverage of the `schema`/`d7-validate` requirements\n\n")
	b.WriteString("Scope: every non-deprecated requirement whose verification contains `schema` or `d7-validate` (ADR-0019 §9). " +
		"**covered** = at least one reviewed valid vector names it in `reqs` and one reviewed invalid vector in `violates`; " +
		"**derived** = umbrella of `coverage/implications.yaml` whose details are all covered or derived. " +
		"Draft vectors are marked with `*` and do not count. The layer of a negative vector is `schema` (its entry schema reports the REQ, no mark), " +
		"`rule` (rule layer of D7 validate per `oec/schema/tests/rule-layer.yaml`), `R-1` (topic and Identification Link vectors) or `warning` " +
		"(a valid topic vector whose warning `whitespace` shows that the topic does not follow the recommendation of REQ-TOP-020; it is no positive vector of it). " +
		"P2 exit gate (ROADMAP P2 exit 2 and 3): `spectool coverage -check -gate`.\n\n")
	b.WriteString("## Summary\n\n| Area | REQs | Covered | Derived | Positive only | Negative only | None / details open |\n|---|---:|---:|---:|---:|---:|---:|\n")
	type sum struct{ n, cov, der, pos, neg, none int }
	var areas []string
	sums := map[string]*sum{}
	tot := &sum{}
	for _, e := range r.Entries {
		s := sums[e.Area]
		if s == nil {
			s = &sum{}
			sums[e.Area] = s
			areas = append(areas, e.Area)
		}
		for _, x := range []*sum{s, tot} {
			x.n++
			switch r.State(e.Req.ID) {
			case "covered":
				x.cov++
			case "derived":
				x.der++
			case "positive only":
				x.pos++
			case "negative only":
				x.neg++
			default:
				x.none++
			}
		}
	}
	sort.Strings(areas)
	for _, a := range areas {
		s := sums[a]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d |\n", a, s.n, s.cov, s.der, s.pos, s.neg, s.none)
	}
	fmt.Fprintf(&b, "| **Total** | %d | %d | %d | %d | %d | %d |\n", tot.n, tot.cov, tot.der, tot.pos, tot.neg, tot.none)
	if len(r.Problems) > 0 {
		b.WriteString("\n## Problems\n\n")
		for _, p := range r.Problems {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	with := 0
	for _, row := range r.Rows {
		if row.Schema {
			with++
		}
	}
	fmt.Fprintf(&b, "\n## Entry schemas per allowed matrix row\n\n%d of %d allowed rows have an entry schema (`oec/schema/%s/<rule>.schema.json`).\n\n| Method | Resource | Rule | Entry schema |\n|---|---|---|---|\n", with, len(r.Rows), EntrySchemaDir)
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s |\n", row.Method, row.Resource, row.Rule, yesNo(row.Schema))
	}
	b.WriteString("\n## Umbrella requirements\n\n| Umbrella | Details | Remainder (not covered by vectors) |\n|---|---|---|\n")
	var ums []*Entry
	for _, e := range r.all {
		if e.Umbrella != nil {
			ums = append(ums, e)
		}
	}
	sort.Slice(ums, func(i, j int) bool { return ums[i].Req.ID < ums[j].Req.ID })
	for _, e := range ums {
		var ds []string
		for _, d := range e.Umbrella.Details {
			ds = append(ds, fmt.Sprintf("%s (%s)", d, r.State(d)))
		}
		fmt.Fprintf(&b, "| %s (%s) | %s | %s |\n", e.Req.ID, r.State(e.Req.ID), strings.Join(ds, ", "), cell(e.Umbrella.Remainder))
	}
	b.WriteString("\n## Requirements\n\n| REQ | Verification | Status | Positive | Negative (layer) | Coverage |\n|---|---|---|---|---|---|\n")
	for _, e := range r.Entries {
		var neg []string
		for _, g := range e.Neg {
			s := g.ID
			if !g.Reviewed {
				s += "*"
			}
			if g.Layer != "schema" {
				s += " (" + g.Layer + ")"
			}
			neg = append(neg, s)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", e.Req.ID, strings.Join(e.Req.Verification, ", "), e.Req.Status,
			list(e.Pos, e.PosDraft), list(neg, nil), r.State(e.Req.ID))
	}
	return b.String()
}

func list(rev, draft []string) string {
	const max = 4
	out := append([]string{}, rev...)
	for _, s := range draft {
		out = append(out, s+"*")
	}
	if len(out) == 0 {
		return "–"
	}
	if len(out) > max {
		return fmt.Sprintf("%s … (%d)", strings.Join(out[:max], ", "), len(out))
	}
	return strings.Join(out, ", ")
}

func cell(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "|", "\\|") }

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "**missing**"
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func set(l []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range l {
		m[s] = true
	}
	return m
}

// sameSet compares a (without duplicates) and b as sets.
func sameSet(a, b []string) bool {
	x, y := set(a), set(b)
	if len(x) != len(a) || len(x) != len(y) {
		return false
	}
	for k := range x {
		if !y[k] {
			return false
		}
	}
	return true
}
