package explain

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/catalog"
)

// d7Kinds are the verification kinds of D7: validate checks schema and
// d7-validate, the catalog tests d7-monitor and d7-test (ADR-0019,
// ADR-0025).
var d7Kinds = []string{"d7-monitor", "d7-test", "schema", "d7-validate"}

// CoverageFile is the catalog coverage report, relative to the module root.
const CoverageFile = "catalog/COVERAGE.md"

// Coverage renders the catalog coverage report (markdown): the tests by
// status, mode and need, the active requirements per D7 verification kind
// and how they are checked, every test with its requirements and fixtures,
// the manual requirements and the requirements checked by tests.
func Coverage(ix *Index) []byte {
	var b bytes.Buffer
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("# D7 catalog coverage\n\n")
	p("Generated from `catalog/d7.yaml`, the requirements catalog of `oec/spec` (baseline `%s`) and the indexes of the\n", ix.Catalog.Baseline)
	p("self-test fixtures (`testdata/sim/index.yaml`, `testdata/captures/index.yaml`) – do not edit; regenerate with\n")
	p("`go test ./internal/explain -run TestCoverageReport -update` (the gate fails if this file is out of date).\n")
	p("`oec-check list` and `oec-check explain` show the same data.\n\n")

	p("## Summary\n\n")
	status, mode, needs := map[string]int{}, map[string]int{}, map[string]int{}
	for _, t := range ix.Catalog.Tests {
		status[t.Status]++
		mode[t.Mode]++
		if len(t.Needs) == 0 {
			needs["none"]++
		}
		for _, n := range t.Needs {
			needs[n]++
		}
	}
	p("| Tests | Count |\n|---|---:|\n")
	p("| all | %d |\n", len(ix.Catalog.Tests))
	for _, s := range Statuses {
		p("| status `%s` | %d |\n", s, status[s])
	}
	for _, m := range Modes {
		p("| mode `%s` | %d |\n", m, mode[m])
	}
	for _, n := range Needs {
		p("| needs `%s` | %d |\n", n, needs[n])
	}

	ids := make([]string, 0, len(ix.Reqs))
	for id, r := range ix.Reqs {
		if r.Status != "deprecated" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	p("\n| Active requirements by verification kind | REQs | checked by a test | `manual` | by `oec-check validate` | not covered |\n|---|---:|---:|---:|---:|---:|\n")
	none := 0
	for _, id := range ids {
		r := ix.Reqs[id]
		if !slices.ContainsFunc(r.Verification, func(k string) bool { return slices.Contains(d7Kinds, k) }) {
			none++
		}
	}
	for _, k := range d7Kinds {
		n, tested, manual, validate, open := 0, 0, 0, 0, 0
		for _, id := range ids {
			r := ix.Reqs[id]
			if !slices.Contains(r.Verification, k) {
				continue
			}
			n++
			_, isManual := ix.Manual(id)
			switch {
			case k == "schema" || k == "d7-validate":
				validate++
			case slices.ContainsFunc(ix.Catalog.Tests, func(t catalog.Test) bool { return t.Mode == modeOf(k) && slices.Contains(t.Reqs, id) }):
				tested++
			case isManual:
				manual++
			default:
				open++
			}
		}
		p("| `%s` | %d | %d | %d | %d | %d |\n", k, n, tested, manual, validate, open)
	}
	p("| none of these (D8, manual or other tools) | %d | | | | |\n", none)

	sims, caps, pos, rev := 0, 0, 0, 0
	for _, f := range ix.Fixtures {
		if f.Kind == "sim" {
			sims++
		} else {
			caps++
		}
		if f.Positive {
			pos++
		}
		if f.Status == "reviewed" {
			rev++
		}
	}
	p("\nFixtures: %d sim, %d capture; %d positive; %d of %d reviewed.\n", sims, caps, pos, rev, len(ix.Fixtures))
	neg, posT := 0, 0
	for _, t := range ix.Catalog.Tests {
		c := ix.FixtureCounts(t)
		if c.Negative() > 0 {
			neg++
		}
		if c.Positive > 0 {
			posT++
		}
	}
	p("Tests with a negative fixture (fail, warn or info): %d of %d; with a positive fixture: %d of %d.\n", neg, len(ix.Catalog.Tests), posT, len(ix.Catalog.Tests))

	p("\n## Tests\n\n")
	p("Fixtures by the outcome their index states for the test: `fail`; `warn`/`info` (the verdict of a SHOULD or a\n")
	p("heuristic rule); `skip`; `positive` – positive fixtures that run the test without stating an outcome (pass or not\n")
	p("applicable). A negative fixture is one with fail, warn or info.\n\n")
	p("| Test | Mode | Level | Needs | Status | Requirements | fail | warn/info | skip | positive |\n|---|---|---:|---|---|---|---:|---:|---:|---:|\n")
	for _, t := range ix.Catalog.Tests {
		c := ix.FixtureCounts(t)
		n := strings.Join(t.Needs, ", ")
		if n == "" {
			n = "–"
		}
		p("| %s | %s | %d | %s | %s | %s | %d | %d | %d | %d |\n", t.ID, t.Mode, t.Level, n, t.Status, strings.Join(t.Reqs, " "), c.Fail, c.WarnInfo, c.Skip, c.Positive)
	}

	p("\n## Manual requirements\n\n| Requirement | Reason |\n|---|---|\n")
	for _, m := range ix.Catalog.Manual {
		p("| %s | %s |\n", m.Req, strings.ReplaceAll(m.Reason, "|", "\\|"))
	}

	p("\n## Requirements checked by tests\n\n| Requirement | Keyword | Verification | Tests | Fixtures with a stated outcome |\n|---|---|---|---|---:|\n")
	for _, id := range ids {
		tests := ix.TestsOf(id)
		if len(tests) == 0 {
			continue
		}
		r := ix.Reqs[id]
		p("| %s | %s | %s | %s | %d |\n", id, r.Keyword, strings.Join(r.Verification, ", "), strings.Join(tests, " "), len(ix.ReqFixtures(id)))
	}
	return b.Bytes()
}

// FixtureCounts counts the fixtures of a test by outcome.
type FixtureCounts struct{ Fail, WarnInfo, Skip, Positive int }

// Negative is the number of fixtures with fail, warn or info.
func (c FixtureCounts) Negative() int { return c.Fail + c.WarnInfo }

// FixtureCounts counts the fixtures of the test (TestFixtures) by outcome.
func (ix *Index) FixtureCounts(t catalog.Test) FixtureCounts {
	var c FixtureCounts
	for _, f := range ix.TestFixtures(t) {
		switch f.Outcome {
		case "fail":
			c.Fail++
		case "warn", "info":
			c.WarnInfo++
		case "skip":
			c.Skip++
		case PassOrNA:
			c.Positive++
		}
	}
	return c
}

func modeOf(kind string) string {
	if kind == "d7-monitor" {
		return "monitor"
	}
	return "test"
}
