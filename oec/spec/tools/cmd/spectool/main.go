// Command spectool maintains the OEC v2 spec baseline in oec/spec (ADR-0018).
//
//	spectool lint      [-spec DIR] [-guideline DIR]   check catalog, matrix, topic grammar and vectors
//	spectool render    [-spec DIR] [-check]           generate requirements/view/*.md,
//	                                                  matrix/view/*.md and vectors/view/*.md
//	spectool inventory [-spec DIR] [-guideline DIR] [-rev REV] [-check]
//	                                                  extract normative-statement candidates
//	spectool coverage  [-spec DIR] [-check]           generate coverage/report.md
//	spectool topic     [-spec DIR] [-check] [TOPIC...] generate topic/topic-patterns.json;
//	                                                  with arguments: analyse the given topics
//	spectool review    [-spec DIR] [-to reviewed|draft] [-n] ID...
//	                                                  set the status after a PO review (requirements,
//	                                                  vectors, areas, matrix payload columns; ranges
//	                                                  such as REQ-QRY-001..004), then render, coverage, lint
//	spectool rebaseline [-spec DIR] [-guideline DIR] [-rev REV] [-n]
//	                                                  move the spec to a new guideline commit: inventory,
//	                                                  baseline in all sources, topic patterns, render,
//	                                                  coverage, lint; lists the project docs to update
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/coverage"
	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/inventory"
	"github.com/OI4/oec/spec/tools/internal/maintain"
	"github.com/OI4/oec/spec/tools/internal/render"
	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/vectors"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "lint":
		err = cmdLint(os.Args[2:])
	case "render":
		err = cmdRender(os.Args[2:])
	case "inventory":
		err = cmdInventory(os.Args[2:])
	case "coverage":
		err = cmdCoverage(os.Args[2:])
	case "topic":
		err = cmdTopic(os.Args[2:])
	case "review":
		err = cmdReview(os.Args[2:])
	case "rebaseline":
		err = cmdRebaseline(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "spectool:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: spectool lint|render|inventory|coverage|topic|review|rebaseline [flags]  (spectool <cmd> -h for flags)")
}

type common struct {
	spec      string
	guideline string
}

func flags(name string) (*flag.FlagSet, *common) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	c := &common{}
	fs.StringVar(&c.spec, "spec", ".", "spec directory (oec/spec)")
	fs.StringVar(&c.guideline, "guideline", "", "guideline repository (default: <spec>/../../oi4-oec-documentation)")
	return fs, c
}

func (c *common) guidelineDir() string {
	if c.guideline != "" {
		return c.guideline
	}
	return filepath.Join(c.spec, "..", "..", "oi4-oec-documentation")
}

func (c *common) path(rel string) string { return filepath.Join(c.spec, filepath.FromSlash(rel)) }

func cmdLint(args []string) error {
	fs, c := flags("lint")
	noGuideline := fs.Bool("no-guideline", false, "skip checks against the guideline sources (refs, quotes)")
	_ = fs.Parse(args)
	s, err := catalog.Load(c.spec)
	if err != nil {
		return err
	}
	var opt catalog.LintOptions
	if inv, err := inventory.Load(c.path(catalog.InventoryFile)); err == nil {
		opt.Inventory = inv.ByKey()
		if inv.Baseline != s.Areas.Baseline {
			return fmt.Errorf("%s baseline %s differs from catalog baseline %s", catalog.InventoryFile, inv.Baseline, s.Areas.Baseline)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var examples map[string]string
	if !*noGuideline {
		files, err := guideline.LoadFiles(c.guidelineDir())
		if err != nil {
			return fmt.Errorf("%w (use -guideline or -no-guideline)", err)
		}
		opt.Sections = guideline.SectionTexts(files)
		all, err := guideline.LoadFilesWithExamples(c.guidelineDir())
		if err != nil {
			return err
		}
		examples = guideline.SectionTexts(all)
	}
	errs := s.Lint(opt)

	ts, err := topic.Load(c.spec)
	if err != nil {
		return err
	}
	if ts.Matrix.Baseline != s.Areas.Baseline {
		errs = append(errs, fmt.Errorf("%s: baseline %s differs from %s (%s)", topic.MatrixFile, ts.Matrix.Baseline, catalog.AreasFile, s.Areas.Baseline))
	}
	for _, ref := range matrixRefs(ts) {
		if opt.Sections != nil && !sectionKnown(opt.Sections, ref) {
			errs = append(errs, fmt.Errorf("%s: ref %q does not exist in the guideline", topic.MatrixFile, ref))
		}
	}
	errs = append(errs, ts.Check(topic.CheckOptions{Samples: 100, Seed: 1})...)

	vfiles, err := vectors.Load(c.spec)
	if err != nil {
		return err
	}
	reqs := map[string]bool{}
	for _, r := range s.Requirements() {
		reqs[r.ID] = true
	}
	areaStatus := map[string]string{}
	for _, a := range s.Areas.Areas {
		areaStatus[a.Code] = a.Status
	}
	errs = append(errs, ts.CheckPayload(topic.PayloadOptions{Reqs: reqs, AreaStatus: areaStatus})...)
	errs = append(errs, vectors.Check(vfiles, ts, vectors.Options{Baseline: s.Areas.Baseline, Reqs: reqs, Sections: examples})...)

	for _, e := range errs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d problem(s)", len(errs))
	}
	fmt.Printf("ok: %d area file(s), %d requirement(s), %d informative item(s)\n", len(s.Files), len(s.Requirements()), len(s.Informative.Items))
	n := vectors.Count(vfiles)
	fmt.Printf("ok: %s: %d combination(s); vectors: topic=%d identification-link=%d dnp-encoding=%d oi4identifier=%d payload=%d\n",
		topic.MatrixFile, len(ts.Matrix.Combinations), n["topic"], n["identification-link"], n["dnp-encoding"], n["oi4identifier"], n["payload"])
	return nil
}

func matrixRefs(ts *topic.Spec) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range ts.Matrix.Combinations {
		for _, r := range c.Refs {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out
}

func sectionKnown(sections map[string]string, ref string) bool {
	if _, ok := sections[ref]; ok {
		return true
	}
	for s := range sections {
		if guideline.SectionWithin(s, ref) {
			return true
		}
	}
	return false
}

func cmdTopic(args []string) error {
	fs, c := flags("topic")
	check := fs.Bool("check", false, "fail if topic/topic-patterns.json is outdated instead of writing it")
	_ = fs.Parse(args)
	ts, err := topic.Load(c.spec)
	if err != nil {
		return err
	}
	if fs.NArg() > 0 {
		for _, t := range fs.Args() {
			res, err := ts.Analyze(t)
			if err != nil {
				return err
			}
			if !res.Valid {
				fmt.Printf("invalid (%s, rule %s): %s\n  %s\n", res.Failure, orNone(res.Rule), res.Detail, t)
				continue
			}
			p := res.Parsed
			fmt.Printf("valid (%s): ServiceType=%s AppId=%q Method=%s Resource=%s", res.Rule, p.ServiceType, p.AppIDIL, p.Method, p.Resource)
			if p.Source != "" {
				fmt.Printf(" Source=%q", p.SourceIL)
			}
			if p.Filter != "" {
				fmt.Printf(" Filter=%q", p.FilterValues)
			}
			if len(res.Warnings) > 0 {
				fmt.Printf(" warnings=%v", res.Warnings)
			}
			fmt.Println()
		}
		return nil
	}
	if errs := ts.Check(topic.CheckOptions{Samples: 100, Seed: 1}); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		return fmt.Errorf("%d problem(s) between %s and %s", len(errs), topic.GrammarFile, topic.MatrixFile)
	}
	out, err := ts.PatternsFileContent()
	if err != nil {
		return err
	}
	return writeAll(c, map[string]string{topic.PatternsFile: out}, *check)
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func cmdRender(args []string) error {
	fs, c := flags("render")
	check := fs.Bool("check", false, "fail if generated views are outdated instead of writing them")
	_ = fs.Parse(args)
	s, err := catalog.Load(c.spec)
	if err != nil {
		return err
	}
	views := render.Views(s)
	ts, err := topic.Load(c.spec)
	if err != nil {
		return err
	}
	vfiles, err := vectors.Load(c.spec)
	if err != nil {
		return err
	}
	for rel, content := range render.ReviewViews(s, ts, vfiles) {
		views[rel] = content
	}
	for _, dir := range []string{catalog.ViewDir, path.Dir(render.MatrixView), render.VectorViewDir} {
		existing, _ := filepath.Glob(c.path(dir + "/*.md"))
		for _, p := range existing {
			rel := dir + "/" + filepath.Base(p)
			if _, ok := views[rel]; !ok {
				if *check {
					return fmt.Errorf("%s is stale (no source file)", rel)
				}
				if err := os.Remove(p); err != nil {
					return err
				}
			}
		}
	}
	return writeAll(c, views, *check)
}

func cmdInventory(args []string) error {
	fs, c := flags("inventory")
	rev := fs.String("rev", "", "guideline commit recorded as baseline (default: git HEAD of the guideline)")
	check := fs.Bool("check", false, "fail if coverage/inventory.yaml is outdated instead of writing it")
	_ = fs.Parse(args)
	inv, err := inventory.Generate(c.guidelineDir(), *rev)
	if err != nil {
		return err
	}
	data, err := inv.Marshal()
	if err != nil {
		return err
	}
	if err := writeAll(c, map[string]string{catalog.InventoryFile: string(data)}, *check); err != nil {
		return err
	}
	counts := map[string]int{}
	for _, it := range inv.Items {
		counts[strings.SplitN(it.Section, " ", 2)[0]+" "+it.Basis]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("%d item(s) at %s:", len(inv.Items), inv.Baseline)
	for _, k := range keys {
		fmt.Printf(" %s=%d", k, counts[k])
	}
	fmt.Println()
	return nil
}

func cmdCoverage(args []string) error {
	fs, c := flags("coverage")
	check := fs.Bool("check", false, "fail on coverage problems or an outdated report instead of writing it")
	_ = fs.Parse(args)
	s, err := catalog.Load(c.spec)
	if err != nil {
		return err
	}
	inv, err := inventory.Load(c.path(catalog.InventoryFile))
	if err != nil {
		return err
	}
	res := coverage.Compute(s, inv)
	if err := writeAll(c, map[string]string{catalog.ReportFile: res.Report()}, *check); err != nil {
		return err
	}
	for _, st := range res.Areas {
		if st.Total > 0 {
			fmt.Printf("%-11s %-8s items=%4d covered=%4d informative=%4d open=%4d\n", st.Area.Code, st.Area.Status, st.Total, st.Covered, st.Informative, len(st.Open))
		}
	}
	if *check && len(res.Problems) > 0 {
		return errors.New(strings.Join(res.Problems, "; "))
	}
	for _, p := range res.Problems {
		fmt.Fprintln(os.Stderr, "warning:", p)
	}
	return nil
}

func writeAll(c *common, files map[string]string, check bool) error {
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	var stale []string
	for _, rel := range names {
		p := c.path(rel)
		old, err := os.ReadFile(p)
		if err == nil && bytes.Equal(old, []byte(files[rel])) {
			continue
		}
		if check {
			stale = append(stale, rel)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(files[rel]), 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", rel)
	}
	if len(stale) > 0 {
		return fmt.Errorf("outdated generated file(s), run without -check: %s", strings.Join(stale, ", "))
	}
	return nil
}

// args returns the common flags for calling another subcommand.
func (c *common) args(extra ...string) []string {
	a := []string{"-spec", c.spec}
	if c.guideline != "" {
		a = append(a, "-guideline", c.guideline)
	}
	return append(a, extra...)
}

// regenerate writes the views and the coverage report and lints the result.
func (c *common) regenerate() error {
	if err := cmdRender(c.args()); err != nil {
		return err
	}
	if err := cmdCoverage(c.args()); err != nil {
		return err
	}
	return cmdLint(c.args())
}

func cmdReview(args []string) error {
	fs, c := flags("review")
	to := fs.String("to", maintain.Reviewed, "target status: reviewed (after a PO review) or draft (reopen)")
	dry := fs.Bool("n", false, "dry run: show the changes without writing")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: spectool review [flags] ID...\n"+
			"  ID: requirement (REQ-QRY-001), vector (V-PAY-053), area code (QRY; drafted <-> reviewed),\n"+
			"      matrix payload column (Pub/MAM) or \"matrix\" for all rows; ranges REQ-QRY-001..004, comma lists")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("no ids given")
	}
	ids, err := maintain.ExpandIDs(fs.Args())
	if err != nil {
		return err
	}
	changes, err := maintain.SetStatus(c.spec, ids, *to, *dry)
	if err != nil {
		return err
	}
	n := 0
	for _, ch := range changes {
		if ch.Unchanged() {
			fmt.Printf("  %-34s %s already %s\n", ch.File, ch.ID, ch.To)
			continue
		}
		n++
		fmt.Printf("  %-34s %s %s -> %s\n", ch.File, ch.ID, ch.From, ch.To)
	}
	if *dry {
		fmt.Printf("dry run: %d change(s), nothing written\n", n)
		return nil
	}
	fmt.Printf("%d status change(s) written\n", n)
	if n == 0 {
		return nil
	}
	return c.regenerate()
}

// projectDocs are the living project documents that name the current baseline
// (relative to the workspace root <spec>/../..).
var projectDocs = []string{"AGENTS.md", "project/STATUS.md", "project/GUARDRAILS.md", "project/ROADMAP.md", "oec/spec/README.md"}

func cmdRebaseline(args []string) error {
	fs, c := flags("rebaseline")
	rev := fs.String("rev", "", "new guideline commit; must be checked out (default: git HEAD of the guideline)")
	dry := fs.Bool("n", false, "dry run: show the files whose baseline would change")
	force := fs.Bool("force", false, "re-baseline even if only files outside docs/ and rfcs_adrs/ changed")
	_ = fs.Parse(args)
	s, err := catalog.Load(c.spec)
	if err != nil {
		return err
	}
	old := s.Areas.Baseline
	g := c.guidelineDir()
	git := func(a ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", g}, a...)...).Output()
		return strings.TrimSpace(string(out)), err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("cannot read the guideline HEAD in %s: %w", g, err)
	}
	if *rev == "" {
		*rev = "HEAD"
	}
	full, err := git("rev-parse", "--verify", *rev+"^{commit}")
	if err != nil {
		return fmt.Errorf("unknown guideline commit %q", *rev)
	}
	if full != head {
		return fmt.Errorf("guideline HEAD is %.7s, not %s – check out %s first (the inventory reads the working tree)", head, *rev, *rev)
	}
	if dirty, _ := git("status", "--porcelain", "--", "docs", "rfcs_adrs"); dirty != "" {
		return fmt.Errorf("guideline working tree has uncommitted changes in docs/ or rfcs_adrs/ – commit them first:\n%s", dirty)
	}
	short, err := git("rev-parse", "--short=7", full)
	if err != nil {
		return err
	}
	if short == old {
		fmt.Printf("spec baseline is already %s\n", old)
		return nil
	}
	if diff, err := git("diff", "--name-only", old, full, "--", "docs", "rfcs_adrs"); err == nil && diff == "" && !*force {
		fmt.Printf("guideline %s changes nothing in docs/ or rfcs_adrs/ since %s (e.g. register only) – no re-baseline needed (ADR-0001 amendment C-018); use -force to re-baseline anyway\n", short, old)
		return nil
	}
	changed, err := maintain.Rebaseline(c.spec, old, short, *dry)
	if err != nil {
		return err
	}
	for _, f := range changed {
		fmt.Printf("  baseline %s -> %s: %s\n", old, short, f)
	}
	if *dry {
		fmt.Printf("dry run: %d file(s), nothing written\n", len(changed))
		return nil
	}
	if err := cmdInventory(c.args("-rev", short)); err != nil {
		return err
	}
	if err := cmdTopic(c.args()); err != nil {
		return err
	}
	lintErr := c.regenerate()
	root := filepath.Join(c.spec, "..", "..")
	fmt.Printf("\nTo do by hand (baseline %s -> %s):\n", old, short)
	if lintErr != nil {
		fmt.Println("  - fix the lint problems above (remap inventory keys, sync golden vectors with edits/justification), then: spectool lint && spectool render && spectool coverage")
	}
	if m := maintain.Mentions(root, projectDocs, old); len(m) > 0 {
		fmt.Println("  - update the baseline in the living project docs (move", old, "to the previous baselines where listed):")
		for _, l := range m {
			fmt.Println("      " + l)
		}
	}
	fmt.Println("  - ADR-0001 amendment and the cycle log record the new baseline; commit catalog, generated files and docs separately")
	return lintErr
}
