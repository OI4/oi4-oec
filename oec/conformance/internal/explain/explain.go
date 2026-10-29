// Package explain is the catalog view of oec-check list and explain and of
// the catalog coverage report (DESIGN.md §2, §6; C-069). It joins the D7
// test catalog (catalog/d7.yaml), the requirements catalog embedded in
// oec/spec and the indexes of the self-test fixtures (package conformance).
// The fixtures are shown as their indexes state them (the expected
// verdicts); the self-tests check that the runs give these verdicts.
package explain

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/spec"
)

// Requirement is one requirement of oec/spec (requirements/*.yaml).
type Requirement struct {
	ID           string            `yaml:"id" json:"id"`
	Title        string            `yaml:"title" json:"title"`
	Statement    string            `yaml:"statement" json:"statement"`
	Keyword      string            `yaml:"keyword" json:"keyword"`
	Domain       string            `yaml:"domain" json:"domain,omitempty"`
	Levels       map[string]string `yaml:"levels" json:"levels,omitempty"`
	AppliesTo    []string          `yaml:"applies_to" json:"applies_to,omitempty"`
	Condition    string            `yaml:"condition" json:"condition,omitempty"`
	Refs         []string          `yaml:"refs" json:"refs"`
	Quotes       []Quote           `yaml:"quotes" json:"quotes,omitempty"`
	Verification []string          `yaml:"verification" json:"verification"`
	Status       string            `yaml:"status" json:"status"`
	Notes        string            `yaml:"notes" json:"notes,omitempty"`
}

// Quote is a quotation of the guideline that a requirement rests on.
type Quote struct {
	Ref  string `yaml:"ref" json:"ref"`
	Text string `yaml:"text" json:"text"`
}

// FixtureRef is the expectation of one fixture for one test: the outcome
// stated in its index (PassOrNA for a positive fixture that runs the test
// without stating an outcome) and the requirements with this outcome.
type FixtureRef struct {
	Fixture string   `json:"fixture"` // "sim <fault>" or "capture <file>"
	Test    string   `json:"test"`
	Outcome string   `json:"outcome"`
	Reqs    []string `json:"reqs,omitempty"`
	Status  string   `json:"status"` // review status of the fixture
}

// PassOrNA is the outcome of a positive fixture without an expectation for
// the test.
const PassOrNA = "pass or not applicable"

// Index is the joined view.
type Index struct {
	Catalog  *catalog.Catalog
	Reqs     map[string]Requirement
	Fixtures []conformance.Fixture
}

// Load reads the embedded catalogs and fixture indexes.
func Load() (*Index, error) {
	c, err := catalog.Load(catalog.D7File)
	if err != nil {
		return nil, err
	}
	for i := range c.Tests {
		if c.Tests[i].Needs == nil {
			c.Tests[i].Needs = []string{} // JSON: [] rather than null
		}
	}
	reqs, err := loadRequirements()
	if err != nil {
		return nil, err
	}
	fx, err := conformance.Fixtures()
	if err != nil {
		return nil, err
	}
	return &Index{Catalog: c, Reqs: reqs, Fixtures: fx}, nil
}

func loadRequirements() (map[string]Requirement, error) {
	paths, err := fs.Glob(spec.FS, spec.RequirementsGlob)
	if err != nil {
		return nil, err
	}
	out := map[string]Requirement{}
	for _, p := range paths {
		raw, err := fs.ReadFile(spec.FS, p)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Requirements []Requirement `yaml:"requirements"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for _, r := range doc.Requirements {
			out[r.ID] = r
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no requirements in %s", spec.RequirementsGlob)
	}
	return out, nil
}

// Values of the filters.
var (
	Modes    = []string{"monitor", "test"}
	Needs    = []string{"proxy", "restart-hook", "peer", "none"}
	Statuses = []string{"planned", "implemented"}
)

// Filter selects tests; zero values select every test.
type Filter struct {
	Patterns []string // test ids or path.Match patterns (any matches)
	Mode     string
	Level    int    // tests that run at this level (catalog level <= Level)
	Req      string // tests that check the requirement
	Needs    string // tests with this need; "none": tests without needs
	Status   string
}

// Check reports an invalid filter.
func (ix *Index) Check(f Filter) error {
	for _, p := range f.Patterns {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("bad test pattern %q", p)
		}
	}
	switch {
	case f.Mode != "" && !slices.Contains(Modes, f.Mode):
		return fmt.Errorf("--mode must be one of %s", strings.Join(Modes, ", "))
	case f.Level < 0 || f.Level > 3:
		return fmt.Errorf("--level must be 1, 2 or 3")
	case f.Needs != "" && !slices.Contains(Needs, f.Needs):
		return fmt.Errorf("--needs must be one of %s", strings.Join(Needs, ", "))
	case f.Status != "" && !slices.Contains(Statuses, f.Status):
		return fmt.Errorf("--status must be one of %s", strings.Join(Statuses, ", "))
	}
	if _, ok := ix.Reqs[f.Req]; f.Req != "" && !ok {
		return fmt.Errorf("unknown requirement %s", f.Req)
	}
	return nil
}

// List returns the tests of the catalog that pass the filter, in catalog
// order.
func (ix *Index) List(f Filter) []catalog.Test {
	out := []catalog.Test{}
	for _, t := range ix.Catalog.Tests {
		if f.match(t) {
			out = append(out, t)
		}
	}
	return out
}

func (f Filter) match(t catalog.Test) bool {
	if len(f.Patterns) > 0 && !slices.ContainsFunc(f.Patterns, func(p string) bool { ok, _ := path.Match(p, t.ID); return ok }) {
		return false
	}
	switch {
	case f.Mode != "" && t.Mode != f.Mode,
		f.Level > 0 && t.Level > f.Level,
		f.Req != "" && !slices.Contains(t.Reqs, f.Req),
		f.Status != "" && t.Status != f.Status,
		f.Needs == "none" && len(t.Needs) > 0,
		f.Needs != "" && f.Needs != "none" && !slices.Contains(t.Needs, f.Needs):
		return false
	}
	return true
}

// Test returns the catalog entry of a test.
func (ix *Index) Test(id string) (catalog.Test, bool) {
	for _, t := range ix.Catalog.Tests {
		if t.ID == id {
			return t, true
		}
	}
	return catalog.Test{}, false
}

// TestFixtures returns the expectations of the fixtures for the test: every
// fixture whose index states an outcome for it, then every positive
// fixture that runs it without stating one (a capture runs only the
// monitor tests; a sim fixture without peer skips the tests that need the
// peer).
func (ix *Index) TestFixtures(t catalog.Test) []FixtureRef {
	var stated, positive []FixtureRef
	for _, f := range ix.Fixtures {
		if e, ok := f.Expect[t.ID]; ok {
			stated = append(stated, FixtureRef{Fixture: f.Label(), Test: t.ID, Outcome: e.Outcome, Reqs: e.Reqs, Status: f.Status})
			continue
		}
		if f.Positive && f.Runs(t.ID) && (f.Kind == "sim" || t.Mode == "monitor") && (f.Peer || !slices.Contains(t.Needs, "peer")) {
			positive = append(positive, FixtureRef{Fixture: f.Label(), Test: t.ID, Outcome: PassOrNA, Status: f.Status})
		}
	}
	return append(stated, positive...)
}

// TestsOf returns the ids of the tests that check the requirement, in
// catalog order.
func (ix *Index) TestsOf(req string) []string {
	out := []string{}
	for _, t := range ix.Catalog.Tests {
		if slices.Contains(t.Reqs, req) {
			out = append(out, t.ID)
		}
	}
	return out
}

// Manual returns the reason why no test checks the requirement, if the
// catalog lists it as manual.
func (ix *Index) Manual(req string) (string, bool) {
	for _, m := range ix.Catalog.Manual {
		if m.Req == req {
			return m.Reason, true
		}
	}
	return "", false
}

// ReqFixtures returns the fixtures whose index states an outcome naming the
// requirement.
func (ix *Index) ReqFixtures(req string) []FixtureRef {
	var out []FixtureRef
	for _, f := range ix.Fixtures {
		ids := make([]string, 0, len(f.Expect))
		for id := range f.Expect {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if e := f.Expect[id]; slices.Contains(e.Reqs, req) {
				out = append(out, FixtureRef{Fixture: f.Label(), Test: id, Outcome: e.Outcome, Reqs: e.Reqs, Status: f.Status})
			}
		}
	}
	return out
}

// ValidateKinds returns the verification kinds of the requirement that
// oec-check validate checks (schema, d7-validate).
func ValidateKinds(r Requirement) []string {
	var out []string
	for _, k := range r.Verification {
		if k == "schema" || k == "d7-validate" {
			out = append(out, k)
		}
	}
	return out
}

var sources = map[string]string{
	"MG":     "Messaging Guideline",
	"RIG":    "Runtime & Infrastructure Guideline",
	"GL-ADR": "guideline ADR",
	"SF":     "spec finding",
}

// Source expands the abbreviation of a reference: "MG 9.1" → "Messaging
// Guideline 9.1", "GL-ADR-020" → "guideline ADR GL-ADR-020".
func Source(ref string) string {
	if head, rest, ok := strings.Cut(ref, " "); ok {
		if s, ok := sources[head]; ok {
			return s + " " + rest
		}
	}
	for _, p := range []string{"GL-ADR", "SF"} {
		if strings.HasPrefix(ref, p+"-") {
			return sources[p] + " " + ref
		}
	}
	return ref
}
