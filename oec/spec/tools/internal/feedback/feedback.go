// Package feedback generates the feedback package for the work group
// (C-110): every spec finding of findings/SPEC-FINDINGS.md with its decision
// and the guideline commits that implement it, and every other change of the
// guideline documents between the base and the target commit. The input is
// the register, the hand-maintained attribution findings/feedback/attribution.tsv
// and the history of the guideline repository; the output is a set of
// Markdown files in findings/feedback/ next to the attribution.
package feedback

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Base is the guideline commit before the changes of the OEC v2.0 work: the
// Docusaurus guideline with GL-ADR-001 … 011 as the project took it over
// (C-006). The first spec-finding commit follows it.
const Base = "5f070bb"

// Dir is the directory of the package in the guideline repository.
const Dir = "findings/feedback"

// AttributionFile is the hand-maintained input in Dir.
const AttributionFile = Dir + "/attribution.tsv"

// RegisterFile is the spec-findings register.
const RegisterFile = "findings/SPEC-FINDINGS.md"

// Paths are the guideline documents the package covers.
var Paths = []string{
	"docs/guideline",
	"docs/infrastructure-guideline",
	"docs/process-description",
	"docs/intro.md",
	"rfcs_adrs",
	"static/img/guideline",
}

// MaxInline is the largest word diff (lines) shown inline for one commit.
const MaxInline = 400

// Classes of changes without a finding or GL-ADR, in the order of the report.
var Classes = []string{"normative", "wording", "editorial"}

var classInfo = map[string]string{
	"normative": "Changes that state, change or make precise a requirement without a spec finding (edits of the product owner). Please review them like a finding.",
	"wording":   "Changes of informative text or of the wording; no requirement changed.",
	"editorial": "Layout, links, headings, numbering and figures aligned to the text; the meaning is unchanged.",
}

// Finding is one row of the register.
type Finding struct {
	ID       string // SF-001
	Area     string
	Finding  string // open-table rows (6 columns)
	Location string
	Proposal string
	Outcome  string // closed-table rows (4 columns)
	Status   string
}

var idRe = regexp.MustCompile(`^SF-\d{3}$`)

// ParseRegister reads the rows of both tables of the register.
func ParseRegister(src string) ([]Finding, error) {
	var out []Finding
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(line, "| SF-") {
			continue
		}
		cells := splitRow(line)
		var f Finding
		switch len(cells) {
		case 6:
			f = Finding{ID: cells[0], Area: cells[1], Finding: cells[2], Location: cells[3], Proposal: cells[4], Status: cells[5]}
		case 4:
			f = Finding{ID: cells[0], Area: cells[1], Outcome: cells[2], Status: cells[3]}
		default:
			return nil, fmt.Errorf("%s:%d: %d cells, want 4 or 6", RegisterFile, i+1, len(cells))
		}
		if !idRe.MatchString(f.ID) {
			return nil, fmt.Errorf("%s:%d: bad id %q", RegisterFile, i+1, f.ID)
		}
		if seen[f.ID] {
			return nil, fmt.Errorf("%s:%d: duplicate %s", RegisterFile, i+1, f.ID)
		}
		seen[f.ID] = true
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no findings", RegisterFile)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// splitRow splits a table row at the pipes outside code spans; an escaped
// pipe stays in the cell.
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	var cur strings.Builder
	tick := 0 // length of the open code span delimiter, 0: outside
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case c == '`':
			n := 1
			for i+n < len(line) && line[i+n] == '`' {
				n++
			}
			switch {
			case tick == 0 && strings.Contains(line[i+n:], strings.Repeat("`", n)):
				tick = n
			case tick == n:
				tick = 0
			}
			cur.WriteString(line[i : i+n])
			i += n - 1
		case c == '|' && tick == 0:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

// Entry is one line of the attribution.
type Entry struct {
	Line   int
	Commit string   // abbreviated hash, 7 characters
	Refs   []string // SF-xxx and GL-ADR-0xx, the first is the home page
	Class  string   // normative, wording or editorial (no Refs)
	Note   string
}

var (
	commitRe = regexp.MustCompile(`^[0-9a-f]{7}$`)
	refRe    = regexp.MustCompile(`^(SF-\d{3}|GL-ADR-0\d{2})$`)
)

// ParseAttribution reads attribution.tsv: commit, ids or one class, note.
func ParseAttribution(src string) ([]Entry, error) {
	var out []Entry
	seen := map[string]int{}
	var errs []string
	for i, line := range strings.Split(src, "\n") {
		n := i + 1
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			errs = append(errs, fmt.Sprintf("line %d: %d columns, want 3 (commit, ids or class, note)", n, len(cols)))
			continue
		}
		e := Entry{Line: n, Commit: cols[0], Note: strings.TrimSpace(cols[2])}
		if !commitRe.MatchString(e.Commit) {
			errs = append(errs, fmt.Sprintf("line %d: commit %q is not a 7-character abbreviated hash", n, e.Commit))
		}
		if prev, dup := seen[e.Commit]; dup {
			errs = append(errs, fmt.Sprintf("line %d: commit %s already attributed in line %d", n, e.Commit, prev))
		}
		seen[e.Commit] = n
		if isClass(cols[1]) {
			e.Class = cols[1]
			if e.Note == "" {
				errs = append(errs, fmt.Sprintf("line %d: class %s needs a note", n, e.Class))
			}
		} else {
			dup := map[string]bool{}
			for _, r := range strings.Split(cols[1], ",") {
				switch {
				case !refRe.MatchString(r):
					errs = append(errs, fmt.Sprintf("line %d: %q is neither SF-xxx, GL-ADR-0xx nor a class (%s)", n, r, strings.Join(Classes, ", ")))
				case dup[r]:
					errs = append(errs, fmt.Sprintf("line %d: %s twice", n, r))
				}
				dup[r] = true
				e.Refs = append(e.Refs, r)
			}
		}
		out = append(out, e)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s:\n  %s", AttributionFile, strings.Join(errs, "\n  "))
	}
	return out, nil
}

func isClass(s string) bool {
	for _, c := range Classes {
		if s == c {
			return true
		}
	}
	return false
}

// File is one file changed by a commit.
type File struct {
	Path    string // path at the commit (the new path of a rename)
	OldPath string // the old path of a rename, else empty
	Added   string // numstat; "-" for binary files
	Deleted string
	Diff    string // word diff of a Markdown file (hunks only), else empty
}

// Commit is one non-merge commit of the range that changes the documents.
type Commit struct {
	Hash  string // full
	Date  string // author date, YYYY-MM-DD
	Files []File
}

// Short is the 7-character abbreviation used in the package.
func (c Commit) Short() string { return c.Hash[:7] }

// DiffLines counts the lines of the word diffs.
func (c Commit) DiffLines() int {
	n := 0
	for _, f := range c.Files {
		n += strings.Count(f.Diff, "\n")
	}
	return n
}

// ADR is a guideline ADR at the target.
type ADR struct {
	ID    string // GL-ADR-012
	Path  string // rfcs_adrs/ADR-012_….md
	Title string
}

// History is what the package reads from the guideline repository.
type History struct {
	Base, Target         string // full hashes
	BaseDate, TargetDate string
	Commits              []Commit // chronological
	MergesWithChanges    []string // merges of the range that change the documents themselves
	ADRs                 []ADR
	ShortStat            string // git diff --shortstat base target -- Paths
}

// Package is the input of Build.
type Package struct {
	History     *History
	Findings    []Finding
	Attribution []Entry
}

// Problems checks the completeness: every commit of the range attributed
// exactly once, every id known, every finding with a commit (or resolved in
// the base, or obsolete), merges without changes of their own.
func (p *Package) Problems() []string {
	var out []string
	h := p.History
	commits := map[string]bool{}
	for _, c := range h.Commits {
		if commits[c.Short()] {
			out = append(out, fmt.Sprintf("commit abbreviation %s is ambiguous in the range", c.Short()))
		}
		commits[c.Short()] = true
	}
	findings := map[string]bool{}
	for _, f := range p.Findings {
		findings[f.ID] = true
	}
	adrs := map[string]bool{}
	for _, a := range h.ADRs {
		adrs[a.ID] = true
	}
	attributed := map[string]bool{}
	used := map[string]bool{}
	for _, e := range p.Attribution {
		attributed[e.Commit] = true
		if !commits[e.Commit] {
			out = append(out, fmt.Sprintf("%s:%d: commit %s is not a commit of %.7s..%.7s that changes the documents", AttributionFile, e.Line, e.Commit, h.Base, h.Target))
		}
		for _, r := range e.Refs {
			used[r] = true
			if strings.HasPrefix(r, "SF-") && !findings[r] {
				out = append(out, fmt.Sprintf("%s:%d: %s is not in the register", AttributionFile, e.Line, r))
			}
			if strings.HasPrefix(r, "GL-ADR-") && !adrs[r] {
				out = append(out, fmt.Sprintf("%s:%d: %s is not in rfcs_adrs at the target", AttributionFile, e.Line, r))
			}
		}
	}
	for _, c := range h.Commits {
		if !attributed[c.Short()] {
			out = append(out, fmt.Sprintf("commit %s (%s) changes the documents but is not attributed in %s", c.Short(), c.Date, AttributionFile))
		}
	}
	base := h.Base[:7]
	for _, f := range p.Findings {
		if used[f.ID] {
			continue
		}
		if !strings.HasPrefix(f.Status, "obsolete") && !strings.Contains(f.Status, base) {
			out = append(out, fmt.Sprintf("%s (%s) has no commit in the attribution and is not resolved in the base %s", f.ID, f.Status, base))
		}
	}
	for _, m := range h.MergesWithChanges {
		out = append(out, fmt.Sprintf("merge %.7s changes the documents itself; attribute its changes in a commit of their own", m))
	}
	return out
}

// Build renders the package: file name (relative to Dir) → content.
func (p *Package) Build() map[string]string {
	r := newRenderer(p)
	files := map[string]string{"README.md": r.readme(), "files.md": r.files()}
	for _, c := range Classes {
		files[c+".md"] = r.class(c)
	}
	for _, f := range p.Findings {
		files[f.ID+".md"] = r.finding(f)
	}
	for _, id := range r.adrIDs {
		files[id+".md"] = r.adr(id)
	}
	return files
}

type renderer struct {
	p       *Package
	commits map[string]Commit
	home    map[string]string   // commit → page id (SF-xxx, GL-ADR-0xx or a class)
	byRef   map[string][]Entry  // id → entries, chronological
	byClass map[string][]Entry  // class → entries, chronological
	coRefs  map[string][]string // id → the other ids of its commits
	adrIDs  []string            // GL-ADRs with commits
	adrs    map[string]ADR
}

func newRenderer(p *Package) *renderer {
	r := &renderer{p: p, commits: map[string]Commit{}, home: map[string]string{}, byRef: map[string][]Entry{},
		byClass: map[string][]Entry{}, coRefs: map[string][]string{}, adrs: map[string]ADR{}}
	order := map[string]int{}
	for i, c := range p.History.Commits {
		r.commits[c.Short()] = c
		order[c.Short()] = i
	}
	for _, a := range p.History.ADRs {
		r.adrs[a.ID] = a
	}
	entries := append([]Entry(nil), p.Attribution...)
	sort.SliceStable(entries, func(i, j int) bool { return order[entries[i].Commit] < order[entries[j].Commit] })
	co := map[string]map[string]bool{}
	for _, e := range entries {
		if e.Class != "" {
			r.home[e.Commit] = e.Class
			r.byClass[e.Class] = append(r.byClass[e.Class], e)
			continue
		}
		r.home[e.Commit] = e.Refs[0]
		for _, id := range e.Refs {
			r.byRef[id] = append(r.byRef[id], e)
			for _, o := range e.Refs {
				if o != id {
					if co[id] == nil {
						co[id] = map[string]bool{}
					}
					co[id][o] = true
				}
			}
		}
	}
	for id, m := range co {
		r.coRefs[id] = sortedKeys(m)
	}
	for id := range r.byRef {
		if strings.HasPrefix(id, "GL-ADR-") {
			r.adrIDs = append(r.adrIDs, id)
		}
	}
	sort.Strings(r.adrIDs)
	return r
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

const repoURL = "https://github.com/OI4/oi4-oec-documentation"

func (r *renderer) header(b *strings.Builder, title string) {
	fmt.Fprintf(b, "# %s\n\n", title)
	b.WriteString("<!-- Generated by spectool feedback (oec/spec/tools of the OEC v2.0 workspace) – do not edit; edit attribution.tsv or the register and regenerate. -->\n\n")
}

func pageLink(id string) string { return fmt.Sprintf("[%s](%s.md)", id, id) }

func (r *renderer) commitLink(short string) string {
	return fmt.Sprintf("[`%s`](%s.md#commit-%s)", short, r.home[short], short)
}

// commit renders one commit on page: in full on its home page, else as a
// link to it.
func (r *renderer) commit(b *strings.Builder, page string, e Entry) {
	c := r.commits[e.Commit]
	if c.Hash == "" {
		return
	}
	fmt.Fprintf(b, "### Commit %s\n\n", e.Commit)
	fmt.Fprintf(b, "[`%s`](%s/commit/%s), %s", e.Commit, repoURL, c.Hash, c.Date)
	var others []string
	for _, id := range e.Refs {
		if id != page {
			others = append(others, pageLink(id))
		}
	}
	if len(others) > 0 {
		fmt.Fprintf(b, "; also for %s", strings.Join(others, ", "))
	}
	b.WriteString(".")
	if e.Note != "" {
		fmt.Fprintf(b, " %s", e.Note)
	}
	b.WriteString("\n\n")
	if home := r.home[e.Commit]; home != page {
		fmt.Fprintf(b, "The changes of this commit are shown on the page of %s: %s.\n\n", pageLink(home), r.commitLink(e.Commit))
		return
	}
	b.WriteString("| File | + | − |\n|------|---|---|\n")
	for _, f := range c.Files {
		name := "`" + f.Path + "`"
		if f.OldPath != "" {
			name = "`" + f.OldPath + "` → `" + f.Path + "`"
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", cell(name), f.Added, f.Deleted)
	}
	b.WriteString("\n")
	n := c.DiffLines()
	switch {
	case n == 0:
		b.WriteString("No Markdown file changed (figures or other files only).\n\n")
	case n > MaxInline:
		fmt.Fprintf(b, "The word diff of this commit has %d lines and is not shown here. Show it with `git show --word-diff=plain %s -- %s` or on GitHub (link above).\n\n", n, e.Commit, strings.Join(Paths, " "))
	default:
		for _, f := range c.Files {
			if f.Diff == "" {
				continue
			}
			fmt.Fprintf(b, "`%s`\n\n", f.Path)
			fence := fenceFor(f.Diff)
			fmt.Fprintf(b, "%s\n%s%s\n\n", fence, f.Diff, fence)
		}
	}
}

// fenceFor returns a backtick fence longer than any backtick run in s.
func fenceFor(s string) string {
	longest, run := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

func (r *renderer) related(b *strings.Builder, id string) {
	co := r.coRefs[id]
	if len(co) == 0 {
		return
	}
	links := make([]string, len(co))
	for i, o := range co {
		links[i] = pageLink(o)
	}
	fmt.Fprintf(b, "Related by shared commits: %s.\n\n", strings.Join(links, ", "))
}

func (r *renderer) finding(f Finding) string {
	var b strings.Builder
	r.header(&b, f.ID+" – "+f.Area)
	b.WriteString("[Index](README.md)\n\n")
	fmt.Fprintf(&b, "**Status:** %s\n\n", f.Status)
	if f.Outcome != "" {
		fmt.Fprintf(&b, "## Outcome\n\n%s\n\n", f.Outcome)
	} else {
		fmt.Fprintf(&b, "## Finding\n\n%s\n\n## Location\n\n%s\n\n## Decision\n\n%s\n\n", f.Finding, f.Location, f.Proposal)
	}
	r.related(&b, f.ID)
	b.WriteString("## Changes of the guideline\n\n")
	es := r.byRef[f.ID]
	if len(es) == 0 {
		if strings.HasPrefix(f.Status, "obsolete") {
			b.WriteString("None: the finding became obsolete.\n")
		} else {
			fmt.Fprintf(&b, "None in the range: the finding was resolved in the base `%s`.\n", r.p.History.Base[:7])
		}
		return b.String()
	}
	for _, e := range es {
		r.commit(&b, f.ID, e)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (r *renderer) adr(id string) string {
	var b strings.Builder
	a := r.adrs[id]
	r.header(&b, id+" – "+a.Title)
	b.WriteString("[Index](README.md)\n\n")
	fmt.Fprintf(&b, "The guideline ADR is the file `%s`. The commits below implement or change it.\n\n", a.Path)
	r.related(&b, id)
	b.WriteString("## Changes of the guideline\n\n")
	for _, e := range r.byRef[id] {
		r.commit(&b, id, e)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (r *renderer) class(c string) string {
	var b strings.Builder
	r.header(&b, "Other changes: "+c)
	b.WriteString("[Index](README.md)\n\n")
	fmt.Fprintf(&b, "%s These changes implement neither a spec finding nor a guideline ADR (%d commits).\n\n", classInfo[c], len(r.byClass[c]))
	for _, e := range r.byClass[c] {
		r.commit(&b, c, e)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (r *renderer) files() string {
	var b strings.Builder
	r.header(&b, "Changed files")
	b.WriteString("[Index](README.md)\n\n")
	b.WriteString("Every file of the documents changed between the base and the target, with the commits that changed it and the page that shows each commit. A renamed file is listed under its new name.\n\n")
	byPath := map[string][]string{}
	renamed := map[string][]string{}
	for _, c := range r.p.History.Commits {
		s := c.Short()
		for _, f := range c.Files {
			byPath[f.Path] = append(byPath[f.Path], fmt.Sprintf("%s (%s)", r.commitLink(s), pageLink(r.home[s])))
			if f.OldPath != "" {
				renamed[f.Path] = append(renamed[f.Path], f.OldPath)
			}
		}
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	b.WriteString("| File | Commits |\n|------|---------|\n")
	for _, p := range paths {
		name := "`" + p + "`"
		for _, o := range renamed[p] {
			name += " (from `" + o + "`)"
		}
		fmt.Fprintf(&b, "| %s | %s |\n", cell(name), strings.Join(byPath[p], ", "))
	}
	return b.String()
}

func (r *renderer) readme() string {
	var b strings.Builder
	h := r.p.History
	base, target := h.Base[:7], h.Target[:7]
	r.header(&b, "Feedback package for the work group – OEC guidelines v2.0")
	b.WriteString("## What this is\n\n")
	b.WriteString("While implementing the OEC v2.0 guidelines – the OEC Messaging Guideline (MG) and the OEC Runtime & Infrastructure Guideline (RIG) – the OEC v2.0 project found ambiguities, contradictions and gaps in the text: the spec findings of the register [SPEC-FINDINGS.md](../SPEC-FINDINGS.md). The product owner decided every finding; the decided text was written into the guideline on the branch `feature/spec-findings`, partly together with guideline ADRs (`rfcs_adrs`). Besides, the product owner edited the guideline (layout, links, wording and a few requirements).\n\n")
	b.WriteString("This package lets the work group review and adopt these changes: every finding with its decision and the commits that implement it, and every other change of the guideline in the same range. It adds nothing new – it is a reading aid for the history of the branch. The finding pages copy the entries of the register as they are; they cite the cycles (C-xxx), requirements (REQ-…), decisions (D-x) and tools (D7, D8) of the OEC v2.0 project, which are not needed to judge the change of the guideline.\n\n")
	b.WriteString("## Range\n\n")
	fmt.Fprintf(&b, "- **Base:** [`%s`](%s/tree/%s) (%s) – the guideline as the project took it over, with GL-ADR-001 … 011.\n", base, repoURL, h.Base, h.BaseDate)
	fmt.Fprintf(&b, "- **Target:** [`%s`](%s/tree/%s) (%s) – the guideline baseline of the OEC v2.0 implementation.\n", target, repoURL, h.Target, h.TargetDate)
	b.WriteString("- **Documents:** MG (`docs/guideline`), RIG (`docs/infrastructure-guideline`), the informative OEC Process Description (`docs/process-description`), the guideline ADRs (`rfcs_adrs`), the figures (`static/img/guideline`) and the former welcome page of the site (`docs/intro.md`). Not included: the site code (`src`, `docusaurus.config.js`), the blog and the Community section (`docs/community`).\n")
	fmt.Fprintf(&b, "- **Complete diff:** %s – `git diff %s %s -- %s`, or on GitHub: [%s...%s](%s/compare/%s...%s).\n\n", h.ShortStat, base, target, strings.Join(Paths, " "), base, target, repoURL, base, target)
	b.WriteString("## How to read\n\n")
	b.WriteString("- Each finding has a page `SF-xxx.md`: area, finding, location and decision (older findings: the outcome), status, and the commits that implement it. A commit shows its changed files and the word diff of its Markdown files – `[-removed-]` and `{+added+}` – in hunks that start with an `@@` line with the line numbers.\n")
	b.WriteString("- A commit that implements several findings or GL-ADRs is shown once, on the page of the first; the other pages link to it.\n")
	fmt.Fprintf(&b, "- A commit whose word diff has more than %d lines (mostly renumbering and restructuring) is listed with its files only; figures are listed, not shown. The git command or the GitHub link shows them.\n", MaxInline)
	b.WriteString("- Each guideline ADR with commits in the range has a page `GL-ADR-0xx.md`.\n")
	b.WriteString("- The changes without a finding or GL-ADR are listed in three classes: [normative.md](normative.md) (please review like a finding), [wording.md](wording.md) and [editorial.md](editorial.md).\n")
	b.WriteString("- [files.md](files.md) lists every changed file with its commits.\n")
	b.WriteString("- Section numbers: GL-ADR-015 renumbered MG chapters 7–11 to 6–10 (the former chapter 6 became the Process Description). SF-001 … SF-034 cite the numbering of their time; later findings use the current numbering.\n\n")
	r.numbers(&b)
	b.WriteString("## Findings\n\n| Finding | Area | Status | Commits |\n|---------|------|--------|---------|\n")
	for _, f := range r.p.Findings {
		fmt.Fprintf(&b, "| [%s](%s.md) | %s | %s | %d |\n", f.ID, f.ID, cell(f.Area), statusWord(f.Status), len(r.byRef[f.ID]))
	}
	b.WriteString("\n## Guideline ADRs\n\n| GL-ADR | Title | Commits |\n|--------|-------|---------|\n")
	for _, id := range r.adrIDs {
		fmt.Fprintf(&b, "| [%s](%s.md) | %s | %d |\n", id, id, cell(r.adrs[id].Title), len(r.byRef[id]))
	}
	b.WriteString("\n## Completeness\n\n")
	b.WriteString("The OEC v2.0 workspace checks this package in its gate (`spectool feedback -check`): every commit of the range that changes the documents is attributed exactly once in [attribution.tsv](attribution.tsv) – to findings, GL-ADRs or one class with a note –, every id exists, every finding has a commit unless it was resolved in the base or became obsolete, merge commits carry no changes of their own, and the files of this package equal the rendering.\n")
	return b.String()
}

func (r *renderer) numbers(b *strings.Builder) {
	h := r.p.History
	status := map[string]int{}
	for _, f := range r.p.Findings {
		status[statusWord(f.Status)]++
	}
	var sf, adr int
	for _, c := range h.Commits {
		home := r.home[c.Short()]
		switch {
		case strings.HasPrefix(home, "SF-"):
			sf++
		case strings.HasPrefix(home, "GL-ADR-"):
			adr++
		}
	}
	b.WriteString("## Numbers\n\n")
	fmt.Fprintf(b, "- Findings: %d (%s).\n", len(r.p.Findings), joinCounts(status))
	fmt.Fprintf(b, "- Commits that change the documents: %d – %d shown on a finding page, %d on a GL-ADR page", len(h.Commits), sf, adr)
	for _, c := range Classes {
		fmt.Fprintf(b, ", %d %s", len(r.byClass[c]), c)
	}
	b.WriteString(".\n\n")
}

func joinCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = strconv.Itoa(m[k]) + " " + k
	}
	return strings.Join(parts, ", ")
}

func statusWord(s string) string {
	if i := strings.IndexAny(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}

func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
