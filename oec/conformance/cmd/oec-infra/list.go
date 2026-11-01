package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/OI4/oec/conformance/internal/explain"
)

// list and explain follow oec-check list/explain (C-069); the D7 command
// stays unchanged (D7 is closed), so the code is adapted here (C-075).

// parseInterleaved parses flags that may follow the arguments
// ("list T8-RUN-* --needs docker").
func parseInterleaved(fl *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	for {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		if fl.NArg() == 0 {
			return rest, nil
		}
		rest = append(rest, fl.Arg(0))
		args = fl.Args()[1:]
	}
}

func usageError(stderr io.Writer, format string, a ...any) int {
	fmt.Fprintf(stderr, "%s: "+format+"\n", append([]any{tool}, a...)...)
	return exitUsage
}

func loadIndex(stderr io.Writer) (*explain.Index, bool) {
	ix, err := explain.LoadInfra()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return nil, false
	}
	return ix, true
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	fmt.Fprintf(stdout, "%s\n", raw)
	return exitPass
}

// level is the level of a test in the listing: "-" for a test that is not
// graded (its requirements apply with their keyword, DESIGN.md §9).
func level(l int) string {
	if l == 0 {
		return "-"
	}
	return strconv.Itoa(l)
}

// runList lists the tests of the D8 catalog (DESIGN.md §2).
func runList(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("list", flag.ContinueOnError)
	fl.SetOutput(stderr)
	var f explain.Filter
	format := fl.String("format", "text", "text|json")
	fl.StringVar(&f.Mode, "mode", "", "image|run|container|host|bus")
	fl.IntVar(&f.Level, "level", 0, "tests that run at level 1|2|3 (ungraded tests run at every level)")
	fl.StringVar(&f.Req, "req", "", "tests that check the requirement")
	fl.StringVar(&f.Needs, "needs", "", "docker|host-access|credentials-2|none")
	fl.StringVar(&f.Status, "status", "", "planned|implemented")
	patterns, err := parseInterleaved(fl, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	f.Patterns = patterns
	if *format != "text" && *format != "json" {
		return usageError(stderr, "unknown format %q (text, json)", *format)
	}
	levelSet := false
	fl.Visit(func(x *flag.Flag) { levelSet = levelSet || x.Name == "level" })
	if levelSet && f.Level == 0 {
		return usageError(stderr, "--level must be 1, 2 or 3")
	}
	ix, ok := loadIndex(stderr)
	if !ok {
		return exitInternal
	}
	if err := ix.Check(f); err != nil {
		return usageError(stderr, "%v", err)
	}
	tests := ix.List(f)
	if *format == "json" {
		return writeJSON(stdout, stderr, tests)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tMODE\tLEVEL\tSTATUS\tNEEDS\tTITLE\tREQS")
	impl := 0
	for _, t := range tests {
		needs := strings.Join(t.Needs, ",")
		if needs == "" {
			needs = "-"
		}
		if t.Status == "implemented" {
			impl++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Mode, level(t.Level), t.Status, needs, t.Title, strings.Join(t.Reqs, " "))
	}
	tw.Flush()
	if len(tests) == len(ix.Catalog.Tests) {
		manual := "requirements"
		if len(ix.Catalog.Manual) == 1 {
			manual = "requirement"
		}
		fmt.Fprintf(stdout, "%d tests (%d implemented, %d planned), %d manual %s (oec-infra explain REQ-…)\n",
			len(tests), impl, len(tests)-impl, len(ix.Catalog.Manual), manual)
	} else {
		fmt.Fprintf(stdout, "%d of %d tests (%d implemented, %d planned)\n", len(tests), len(ix.Catalog.Tests), impl, len(tests)-impl)
	}
	return exitPass
}

// testView is the JSON of explain for a test (level 0: not graded).
type testView struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	Mode         string                `json:"mode"`
	Level        int                   `json:"level"`
	Needs        []string              `json:"needs"`
	Status       string                `json:"status"`
	Notes        string                `json:"notes,omitempty"`
	Requirements []explain.Requirement `json:"requirements"`
	Fixtures     []explain.FixtureRef  `json:"fixtures"`
}

// reqView is the JSON of explain for a requirement.
type reqView struct {
	explain.Requirement
	Tests    []string             `json:"tests"`
	OecCheck []string             `json:"oec_check,omitempty"` // verification kinds of oec-check (D7)
	Manual   string               `json:"manual,omitempty"`
	Fixtures []explain.FixtureRef `json:"fixtures"`
}

// d7Kinds returns the verification kinds of the requirement that oec-check
// covers.
func d7Kinds(r explain.Requirement) []string {
	var out []string
	for _, k := range explain.D7.Kinds {
		if slices.Contains(r.Verification, k.Name) {
			out = append(out, k.Name)
		}
	}
	return out
}

// runExplain explains a test or a requirement (DESIGN.md §2).
func runExplain(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("explain", flag.ContinueOnError)
	fl.SetOutput(stderr)
	format := fl.String("format", "text", "text|json")
	ids, err := parseInterleaved(fl, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	if len(ids) != 1 {
		return usageError(stderr, "explain takes one test id (T8-…) or requirement id (REQ-…)")
	}
	if *format != "text" && *format != "json" {
		return usageError(stderr, "unknown format %q (text, json)", *format)
	}
	ix, ok := loadIndex(stderr)
	if !ok {
		return exitInternal
	}
	id := strings.ToUpper(ids[0])
	if t, ok := ix.Test(id); ok {
		v := testView{ID: t.ID, Title: t.Title, Mode: t.Mode, Level: t.Level, Needs: t.Needs, Status: t.Status, Notes: t.Notes,
			Requirements: []explain.Requirement{}, Fixtures: ix.TestFixtures(t)}
		for _, r := range t.Reqs {
			v.Requirements = append(v.Requirements, ix.Reqs[r])
		}
		if v.Fixtures == nil {
			v.Fixtures = []explain.FixtureRef{}
		}
		if *format == "json" {
			return writeJSON(stdout, stderr, v)
		}
		writeTest(stdout, v)
		return exitPass
	}
	if r, ok := ix.Reqs[id]; ok {
		v := reqView{Requirement: r, Tests: ix.TestsOf(id), OecCheck: d7Kinds(r), Fixtures: ix.ReqFixtures(id)}
		v.Manual, _ = ix.Manual(id)
		if v.Fixtures == nil {
			v.Fixtures = []explain.FixtureRef{}
		}
		if *format == "json" {
			return writeJSON(stdout, stderr, v)
		}
		writeReq(stdout, v)
		return exitPass
	}
	if strings.HasPrefix(id, "T7-") {
		return usageError(stderr, "%s is not a test of oec-infra (oec-check explain %s)", id, id)
	}
	return usageError(stderr, "unknown test or requirement %q (oec-infra list shows the tests)", ids[0])
}

// wrap writes the text in lines of at most 78 characters with the indent.
func wrap(w io.Writer, indent, text string) {
	line := indent
	for _, word := range strings.Fields(text) {
		if len(line) > len(indent) && len(line)+1+len(word) > 78 {
			fmt.Fprintln(w, line)
			line = indent
		}
		if len(line) > len(indent) {
			line += " "
		}
		line += word
	}
	if len(line) > len(indent) {
		fmt.Fprintln(w, line)
	}
}

func levels(l map[string]string) string {
	if len(l) == 0 {
		return "not graded"
	}
	var out []string
	for _, k := range []string{"L1", "L2", "L3"} {
		if o, ok := l[k]; ok {
			out = append(out, k+" "+o)
		}
	}
	return strings.Join(out, ", ")
}

func sourcesOf(refs []string) string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = explain.Source(r)
	}
	return strings.Join(out, "; ")
}

func writeFixtures(w io.Writer, fx []explain.FixtureRef, withTest bool) {
	if len(fx) == 0 {
		fmt.Fprintln(w, "fixtures: none")
		return
	}
	fmt.Fprintln(w, "fixtures (expected verdicts of the self-tests):")
	for _, f := range fx {
		line := "  " + f.Fixture + ": " + f.Outcome
		if withTest {
			line += " " + f.Test
		}
		if len(f.Reqs) > 0 && !withTest {
			line += " " + strings.Join(f.Reqs, ", ")
		}
		if len(f.Manual) > 0 {
			line += ", manual " + strings.Join(f.Manual, ", ")
		}
		fmt.Fprintf(w, "%s (%s)\n", line, f.Status)
	}
}

func writeTest(w io.Writer, v testView) {
	needs := "none"
	if len(v.Needs) > 0 {
		needs = strings.Join(v.Needs, ", ")
	}
	graded := "not graded"
	if v.Level > 0 {
		graded = "level " + strconv.Itoa(v.Level)
	}
	fmt.Fprintf(w, "%s – %s\nmode %s, %s, needs %s, %s\n", v.ID, v.Title, v.Mode, graded, needs, v.Status)
	if v.Notes != "" {
		fmt.Fprintln(w)
		wrap(w, "", v.Notes)
	}
	fmt.Fprintln(w, "\nrequirements:")
	for _, r := range v.Requirements {
		fmt.Fprintf(w, "  %s – %s\n", r.ID, r.Title)
		fmt.Fprintf(w, "    %s (%s); %s\n", r.Keyword, levels(r.Levels), sourcesOf(r.Refs))
		wrap(w, "    ", r.Statement)
	}
	fmt.Fprintln(w)
	writeFixtures(w, v.Fixtures, false)
}

func writeReq(w io.Writer, v reqView) {
	fmt.Fprintf(w, "%s – %s\n%s (%s), status %s\n", v.ID, v.Title, v.Keyword, levels(v.Levels), v.Status)
	if v.Domain != "" || len(v.AppliesTo) > 0 {
		fmt.Fprintf(w, "domain %s, applies to %s\n", v.Domain, strings.Join(v.AppliesTo, ", "))
	}
	if v.Condition != "" {
		wrap(w, "", "condition: "+v.Condition)
	}
	fmt.Fprintln(w, "statement:")
	wrap(w, "  ", v.Statement)
	fmt.Fprintf(w, "refs: %s\n", sourcesOf(v.Refs))
	for _, q := range v.Quotes {
		wrap(w, "  ", explain.Source(q.Ref)+": “"+q.Text+"”")
	}
	fmt.Fprintf(w, "verification: %s\n", strings.Join(v.Verification, ", "))
	if len(v.Tests) > 0 {
		fmt.Fprintf(w, "tests: %s\n", strings.Join(v.Tests, ", "))
	}
	if v.Manual != "" {
		wrap(w, "", "manual: "+v.Manual)
	}
	if len(v.Tests) == 0 && v.Manual == "" {
		if len(v.OecCheck) > 0 {
			fmt.Fprintf(w, "not checked by oec-infra (oec-check explain %s)\n", v.ID)
		} else {
			fmt.Fprintln(w, "not checked by oec-infra")
		}
	}
	if v.Notes != "" {
		wrap(w, "", "notes: "+v.Notes)
	}
	writeFixtures(w, v.Fixtures, true)
}
