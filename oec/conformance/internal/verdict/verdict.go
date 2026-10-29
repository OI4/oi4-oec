// Package verdict is the verdict model of the D7/D8 compliance tools
// (DESIGN.md §5): outcomes, the severity of a requirement at the declared
// level, the aggregation of observations into test results and the exit
// code.
package verdict

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/OI4/oec/spec"
	"go.yaml.in/yaml/v3"
)

// Outcome of a requirement or a test (DESIGN.md §5.1).
type Outcome string

// Outcomes.
const (
	Pass   Outcome = "pass"
	Info   Outcome = "info"
	Skip   Outcome = "skip"
	Warn   Outcome = "warn"
	Fail   Outcome = "fail"
	Manual Outcome = "manual"
)

var rank = map[Outcome]int{Pass: 0, Manual: 0, Info: 1, Skip: 2, Warn: 3, Fail: 4}

// Worse reports whether a is worse than b (fail > warn > skip > info > pass).
func Worse(a, b Outcome) bool { return rank[a] > rank[b] }

// Requirement is the part of an oec/spec requirement the verdict needs.
type Requirement struct {
	ID           string            `yaml:"id" json:"id"`
	Title        string            `yaml:"title" json:"title"`
	Keyword      string            `yaml:"keyword" json:"keyword"`
	Levels       map[string]string `yaml:"levels" json:"levels,omitempty"`
	Status       string            `yaml:"status" json:"status"`
	Verification []string          `yaml:"verification" json:"verification"`
}

// Requirements is the requirements catalog by id.
type Requirements map[string]Requirement

// LoadRequirements reads the requirements catalog embedded in oec/spec.
func LoadRequirements() (Requirements, error) {
	paths, err := fs.Glob(spec.FS, spec.RequirementsGlob)
	if err != nil {
		return nil, err
	}
	out := Requirements{}
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

// Obligation returns the keyword of the requirement at the level (1 … 3):
// its levels entry, or its keyword if it is not graded (no levels). It is
// "none" if the requirement does not apply at the level and "" for an
// unknown id.
func (rs Requirements) Obligation(id string, level int) string {
	r, ok := rs[id]
	if !ok {
		return ""
	}
	if len(r.Levels) == 0 {
		return r.Keyword
	}
	if o, ok := r.Levels[fmt.Sprintf("L%d", level)]; ok {
		return o
	}
	return "none"
}

// Severity maps an obligation to the outcome of a violation (DESIGN.md
// §5.2): MUST/MUST NOT → fail, SHOULD/SHOULD NOT → warn, MAY → info; ""
// for "none" (not judged). A finding without requirement (e.g. a schema
// keyword not mapped to a REQ) is a failure: the entry schemas are
// normative.
func Severity(obligation string) Outcome {
	switch obligation {
	case "MUST", "MUST NOT", "SHALL", "SHALL NOT", "REQUIRED":
		return Fail
	case "SHOULD", "SHOULD NOT", "RECOMMENDED":
		return Warn
	case "MAY", "OPTIONAL":
		return Info
	case "none":
		return ""
	}
	return Fail
}

// Mandatory reports whether the obligation is MUST or MUST NOT.
func Mandatory(obligation string) bool { return Severity(obligation) == Fail && obligation != "" }

// Kind of an observation.
type Kind string

// Kinds of observations.
const (
	Checked   Kind = "checked"   // the rule had something to check
	Violation Kind = "violation" // the requirement is violated; severity from the REQ
	Warning   Kind = "warning"   // a validate warning: always warn
	Note      Kind = "note"      // a hint or heuristic: always info
	Skipped   Kind = "skipped"   // the rule could not be judged; Inconclusive marks a gap of the run
)

// Evidence points into the observed traffic.
type Evidence struct {
	Participant string `json:"participant,omitempty"`
	Line        int    `json:"line,omitempty"` // line of the capture (1-based)
	Topic       string `json:"topic,omitempty"`
	Pointer     string `json:"pointer,omitempty"`
	// Image and Path locate D8 image findings: the platform image and the
	// layout location or file path (oec-infra; D7 does not set them).
	Image  string `json:"image,omitempty"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail"`
}

// Observation is one result of a rule for one requirement.
type Observation struct {
	Test         string
	Req          string
	Kind         Kind
	Inconclusive bool // Skipped: the requirement could not be judged in this run (observation too short, connection not observed)
	Evidence
}

// ReqResult is the verdict on one requirement of a test.
type ReqResult struct {
	Req          string     `json:"req"`
	Obligation   string     `json:"obligation,omitempty"`
	Outcome      Outcome    `json:"outcome"`
	Inconclusive bool       `json:"inconclusive,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Evidence     []Evidence `json:"evidence,omitempty"`
}

// TestResult is the verdict on one catalog test.
type TestResult struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Level   int         `json:"level,omitempty"`
	Status  string      `json:"status"`
	Outcome Outcome     `json:"outcome"`
	Reason  string      `json:"reason,omitempty"`
	Reqs    []ReqResult `json:"reqs,omitempty"`
}

// Test describes a catalog test for Build.
type Test struct {
	ID, Title, Status string
	Level             int
	Reqs              []string
}

// MaxEvidence limits the evidence entries per requirement in a result.
const MaxEvidence = 20

// Build aggregates the observations into one result per test at the
// declared level. Tests that are not implemented are listed with outcome
// skip (status planned) and do not affect the exit code; tests above the
// declared level are skipped. Observations of requirements a test does not
// list (e.g. the requirements of validate findings in T7-MON-01) are added
// to that test.
func Build(tests []Test, obs []Observation, reqs Requirements, level int) []TestResult {
	byTest := map[string][]Observation{}
	for _, o := range obs {
		byTest[o.Test] = append(byTest[o.Test], o)
	}
	var out []TestResult
	for _, t := range tests {
		tr := TestResult{ID: t.ID, Title: t.Title, Level: t.Level, Status: t.Status}
		switch {
		case t.Status != "implemented":
			tr.Outcome, tr.Reason = Skip, "not implemented yet (status "+t.Status+")"
		case t.Level > level:
			tr.Outcome, tr.Reason = Skip, fmt.Sprintf("level %d is above the declared level %d", t.Level, level)
		default:
			ids := append([]string{}, t.Reqs...)
			for _, o := range byTest[t.ID] {
				if o.Req != "" && !contains(ids, o.Req) || o.Req == "" && !contains(ids, "") {
					ids = append(ids, o.Req)
				}
			}
			for _, id := range ids {
				var mine []Observation
				for _, o := range byTest[t.ID] {
					if o.Req == id {
						mine = append(mine, o)
					}
				}
				tr.Reqs = append(tr.Reqs, judge(id, mine, reqs, level))
			}
			// The test outcome is the worst of its judged requirements
			// (§5.2); a requirement that is not applicable (skip, not
			// inconclusive) does not make a test worse than pass.
			tr.Outcome = Pass
			skips := 0
			for _, r := range tr.Reqs {
				if r.Outcome == Skip {
					skips++
					if !r.Inconclusive {
						continue
					}
				}
				if Worse(r.Outcome, tr.Outcome) {
					tr.Outcome = r.Outcome
				}
			}
			if skips == len(tr.Reqs) {
				tr.Outcome = Skip
			}
		}
		out = append(out, tr)
	}
	return out
}

func contains(s []string, x string) bool {
	for _, e := range s {
		if e == x {
			return true
		}
	}
	return false
}

func judge(id string, obs []Observation, reqs Requirements, level int) ReqResult {
	r := ReqResult{Req: id, Obligation: reqs.Obligation(id, level)}
	if id == "" {
		r.Obligation = ""
	}
	if id != "" && r.Obligation == "none" {
		r.Outcome, r.Reason = Skip, fmt.Sprintf("%s does not apply at level %d", id, level)
		return r
	}
	sev := Severity(r.Obligation)
	var bad, notes, skips, incon []Evidence
	checked := false
	worst := Pass
	for _, o := range obs {
		switch o.Kind {
		case Checked:
			checked = true
		case Violation, Warning:
			s := sev
			if o.Kind == Warning {
				s = Warn
			}
			if Worse(s, worst) {
				worst = s
			}
			bad = append(bad, o.Evidence)
		case Note:
			notes = append(notes, o.Evidence)
		case Skipped:
			if o.Inconclusive {
				incon = append(incon, o.Evidence)
			} else {
				skips = append(skips, o.Evidence)
			}
		}
	}
	switch {
	case len(bad) > 0:
		r.Outcome, r.Evidence = worst, append(bad, notes...)
		if worst == Info && len(notes) == 0 {
			r.Outcome = Info
		}
	case len(incon) > 0:
		r.Outcome, r.Inconclusive, r.Evidence = Skip, Mandatory(r.Obligation), append(incon, notes...)
		r.Reason = reasons(incon)
	case len(notes) > 0:
		r.Outcome, r.Evidence = Info, notes
	case checked:
		r.Outcome = Pass
	case len(skips) > 0:
		r.Outcome, r.Reason, r.Evidence = Skip, reasons(skips), skips
	default:
		r.Outcome, r.Reason = Skip, "not applicable: nothing to check was observed"
	}
	if len(r.Evidence) > MaxEvidence {
		n := len(r.Evidence) - MaxEvidence
		r.Evidence = append(r.Evidence[:MaxEvidence:MaxEvidence], Evidence{Detail: fmt.Sprintf("… %d more", n)})
	}
	return r
}

func reasons(ev []Evidence) string {
	seen := map[string]bool{}
	var out []string
	for _, e := range ev {
		if !seen[e.Detail] {
			seen[e.Detail] = true
			out = append(out, e.Detail)
		}
	}
	sort.Strings(out)
	if len(out) > 3 {
		out = append(out[:3], fmt.Sprintf("… %d more", len(out)-3))
	}
	return strings.Join(out, "; ")
}

// Exit codes of the tools (DESIGN.md §5.3).
const (
	ExitPass         = 0
	ExitFail         = 1
	ExitUsage        = 2
	ExitInternal     = 3
	ExitInconclusive = 4
)

// Exit returns the exit code of a run: 1 with any failed requirement, 4 if
// a mandatory requirement could not be judged (unless allowSkip), else 0.
func Exit(results []TestResult, allowSkip bool) int {
	inconclusive := false
	for _, t := range results {
		for _, r := range t.Reqs {
			if r.Outcome == Fail {
				return ExitFail
			}
			inconclusive = inconclusive || r.Inconclusive
		}
	}
	if inconclusive && !allowSkip {
		return ExitInconclusive
	}
	return ExitPass
}

// Summary counts the outcomes of the implemented tests.
func Summary(results []TestResult) map[Outcome]int {
	out := map[Outcome]int{}
	for _, t := range results {
		if t.Status == "implemented" {
			out[t.Outcome]++
		}
	}
	return out
}
