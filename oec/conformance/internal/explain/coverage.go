package explain

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/catalog"
)

// CoverageFile is the catalog coverage report, relative to the module root.
const CoverageFile = "catalog/COVERAGE.md"

// intro is the head of the section of a tool (%s: the spec baseline).
var intro = map[string]string{
	"oec-check": "# D7 catalog coverage\n\n" +
		"Generated from `catalog/d7.yaml`, the requirements catalog of `oec/spec` (baseline `%s`) and the indexes of the\n" +
		"self-test fixtures (`testdata/sim/index.yaml`, `testdata/captures/index.yaml`) – do not edit; regenerate with\n" +
		"`go test ./internal/explain -run TestCoverageReport -update` (the gate fails if this file is out of date).\n" +
		"`oec-check list` and `oec-check explain` show the same data.\n\n",
	"oec-infra": "# D8 catalog coverage\n\n" +
		"Generated from `catalog/d8.yaml`, the requirements catalog of `oec/spec` (baseline `%s`) and the indexes of the\n" +
		"self-test fixtures (`testdata/images/index.yaml`, `testdata/bus/index.yaml`, `testdata/hosts/index.yaml`,\n" +
		"`testdata/containers/index.yaml`, `testdata/run/index.yaml`) – generated with the D7 part above.\n" +
		"`oec-infra list` and `oec-infra explain` show the same data.\n\n",
}

// testsIntro explains the fixture columns of the test table of a tool.
var testsIntro = map[string]string{
	"oec-check": "Fixtures by the outcome their index states for the test: `fail`; `warn`/`info` (the verdict of a SHOULD or a\n" +
		"heuristic rule); `skip`; `positive` – positive fixtures that run the test without stating an outcome (pass or not\n" +
		"applicable). A negative fixture is one with fail, warn or info.\n\n",
	"oec-infra": "Fixtures by the outcome their index states for the test: `fail`; `warn`/`info` (the verdict of a SHOULD or a\n" +
		"heuristic rule); `skip`; `positive` – positive fixtures that run the test without stating an outcome or with `pass`\n" +
		"(pass or not applicable); `manual` – fixtures that state requirements with a manual verdict (a measured value for\n" +
		"a human to judge), whatever the outcome of the test. A negative fixture is one with fail, warn or info. Level `–`:\n" +
		"no requirement of the test is graded by level; they apply with their keyword (DESIGN.md §9).\n\n",
}

// Coverage renders the catalog coverage report (markdown), one section per
// index (D7, then D8): the tests by status, mode and need, the active
// requirements per verification kind of the tool and how they are checked,
// every test with its requirements and fixtures, the manual requirements
// and the requirements checked by tests.
func Coverage(ixs ...*Index) []byte {
	var b bytes.Buffer
	for i, ix := range ixs {
		if i > 0 {
			b.WriteString("\n")
		}
		section(&b, ix)
	}
	return b.Bytes()
}

func section(b *bytes.Buffer, ix *Index) {
	p := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }
	tool := ix.Tool
	manualCol := tool.Name == D8.Name
	p(intro[tool.Name], ix.Catalog.Baseline)

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
	for _, m := range tool.Modes {
		p("| mode `%s` | %d |\n", m, mode[m])
	}
	for _, n := range tool.Needs {
		p("| needs `%s` | %d |\n", n, needs[n])
	}

	ids := make([]string, 0, len(ix.Reqs))
	for id, r := range ix.Reqs {
		if r.Status != "deprecated" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	validateCol := slices.ContainsFunc(tool.Kinds, func(k Kind) bool { return k.Validate })
	p("\n| Active requirements by verification kind | REQs | checked by a test | `manual` |")
	if validateCol {
		p(" by `oec-check validate` |")
	}
	p(" not covered |\n|---|---:|---:|---:|")
	if validateCol {
		p("---:|")
	}
	p("---:|\n")
	none := 0
	for _, id := range ids {
		r := ix.Reqs[id]
		if !slices.ContainsFunc(r.Verification, func(v string) bool {
			return slices.ContainsFunc(tool.Kinds, func(k Kind) bool { return k.Name == v })
		}) {
			none++
		}
	}
	for _, k := range tool.Kinds {
		n, tested, manual, validate, open := 0, 0, 0, 0, 0
		for _, id := range ids {
			r := ix.Reqs[id]
			if !slices.Contains(r.Verification, k.Name) {
				continue
			}
			n++
			_, isManual := ix.Manual(id)
			switch {
			case k.Validate:
				validate++
			case slices.ContainsFunc(ix.Catalog.Tests, func(t catalog.Test) bool { return slices.Contains(k.Modes, t.Mode) && slices.Contains(t.Reqs, id) }):
				tested++
			case isManual:
				manual++
			default:
				open++
			}
		}
		if validateCol {
			p("| `%s` | %d | %d | %d | %d | %d |\n", k.Name, n, tested, manual, validate, open)
		} else {
			p("| `%s` | %d | %d | %d | %d |\n", k.Name, n, tested, manual, open)
		}
	}
	if validateCol {
		p("| none of these (%s, manual or other tools) | %d | | | | |\n", tool.Other, none)
	} else {
		p("| none of these (%s, manual or other tools) | %d | | | |\n", tool.Other, none)
	}

	kinds, pos, rev := map[string]int{}, 0, 0
	for _, f := range ix.Fixtures {
		kinds[f.Kind]++
		if f.Positive {
			pos++
		}
		if f.Status == "reviewed" {
			rev++
		}
	}
	var counts []string
	for _, k := range tool.Fixtures {
		counts = append(counts, fmt.Sprintf("%d %s", kinds[k], k))
	}
	p("\nFixtures: %s; %d positive; %d of %d reviewed.\n", strings.Join(counts, ", "), pos, rev, len(ix.Fixtures))
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
	b.WriteString(testsIntro[tool.Name])
	p("| Test | Mode | Level | Needs | Status | Requirements | fail | warn/info | skip | positive |")
	if manualCol {
		p(" manual |")
	}
	p("\n|---|---|---:|---|---|---|---:|---:|---:|---:|")
	if manualCol {
		p("---:|")
	}
	p("\n")
	for _, t := range ix.Catalog.Tests {
		c := ix.FixtureCounts(t)
		n := strings.Join(t.Needs, ", ")
		if n == "" {
			n = "–"
		}
		level := "–"
		if t.Level > 0 {
			level = fmt.Sprint(t.Level)
		}
		p("| %s | %s | %s | %s | %s | %s | %d | %d | %d | %d |", t.ID, t.Mode, level, n, t.Status, strings.Join(t.Reqs, " "), c.Fail, c.WarnInfo, c.Skip, c.Positive)
		if manualCol {
			p(" %d |", c.Manual)
		}
		p("\n")
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
}

// FixtureCounts counts the fixtures of a test by outcome; Manual counts
// the fixtures that state a manual verdict for one of its requirements
// (D8), whatever the outcome.
type FixtureCounts struct{ Fail, WarnInfo, Skip, Positive, Manual int }

// Negative is the number of fixtures with fail, warn or info.
func (c FixtureCounts) Negative() int { return c.Fail + c.WarnInfo }

// FixtureCounts counts the fixtures of the test (TestFixtures) by outcome.
// A positive fixture that states pass counts as positive (D8: a pass with
// a manual verdict, as in internal/host).
func (ix *Index) FixtureCounts(t catalog.Test) FixtureCounts {
	var c FixtureCounts
	for _, f := range ix.refs(t) {
		if len(f.Manual) > 0 {
			c.Manual++
		}
		switch f.Outcome {
		case "pass":
			if f.positive {
				c.Positive++
			}
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
