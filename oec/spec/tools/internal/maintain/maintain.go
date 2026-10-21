// Package maintain performs the recurring bookkeeping edits on the spec
// sources: status changes after a PO review (requirements, vectors, areas,
// payload columns of the Method x Resource matrix) and the baseline bump of a
// re-baseline. Edits are line based so that comments and formatting of the
// hand-written YAML files stay untouched.
package maintain

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	requirementsDir = "requirements"
	vectorsDir      = "vectors"
	areasFile       = "requirements/areas.yaml"
	matrixFile      = "matrix/method-resource.yaml"
	inventoryFile   = "coverage/inventory.yaml"
	grammarFile     = "topic/topic.abnf"
)

// Status targets accepted by SetStatus.
const (
	Reviewed = "reviewed"
	Draft    = "draft"
)

// MatrixAll selects the payload columns of all matrix rows.
const MatrixAll = "matrix"

// Change is one status edit.
type Change struct {
	File string
	ID   string
	From string
	To   string
}

// Unchanged reports whether the entry already had the target status.
func (c Change) Unchanged() bool { return c.From == c.To }

var rangeRe = regexp.MustCompile(`^(.*?)(\d+)\.\.(.*?)(\d+)$`)

// ExpandIDs expands ranges such as "REQ-QRY-001..004" or
// "V-PAY-026..V-PAY-052" into single ids; arguments may also be comma
// separated. Other arguments are kept as given.
func ExpandIDs(args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		for _, part := range strings.Split(a, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if !strings.Contains(part, "..") {
				out = append(out, part)
				continue
			}
			m := rangeRe.FindStringSubmatch(part)
			if m == nil || (m[3] != "" && m[3] != m[1]) {
				return nil, fmt.Errorf("invalid range %q (use PREFIX-NNN..MMM or PREFIX-NNN..PREFIX-MMM)", part)
			}
			from, _ := strconv.Atoi(m[2])
			to, _ := strconv.Atoi(m[4])
			if to < from {
				return nil, fmt.Errorf("invalid range %q (end before start)", part)
			}
			for n := from; n <= to; n++ {
				out = append(out, fmt.Sprintf("%s%0*d", m[1], len(m[2]), n))
			}
		}
	}
	return out, nil
}

type file struct {
	rel     string
	lines   []string
	changed bool
}

type entry struct {
	f    *file
	line int // line of the status value
	kind string
}

const (
	kindItem   = "item"   // requirement or vector: draft | reviewed
	kindArea   = "area"   // area: planned | drafted | reviewed
	kindMatrix = "matrix" // payload column of a matrix row: draft | reviewed
)

var (
	itemRe   = regexp.MustCompile(`^(\s*)- (id|code): "?([^"\s#]+)"?\s*(#.*)?$`)
	statusRe = regexp.MustCompile(`^(\s*)status:(\s*)("?)([A-Za-z-]+)("?)(\s*(#.*)?)$`)
	matrixRe = regexp.MustCompile(`method: ([A-Za-z]+), resource: ([A-Za-z0-9]+),`)
	payStRe  = regexp.MustCompile(`payload: \{status: ([a-z]+)`)
)

func readFile(root, rel string) (*file, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	return &file{rel: rel, lines: strings.Split(string(data), "\n")}, nil
}

func (f *file) write(root string) error {
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(f.rel)), []byte(strings.Join(f.lines, "\n")), 0o644)
}

// yamlFiles lists the hand-written YAML sources below dir (relative to root),
// excluding the generated inventory.
func yamlFiles(root, dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".yaml" {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); rel != inventoryFile {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// index maps every requirement/vector id, area code and matrix row
// ("Method/Resource") to its status line.
func index(root string) (map[string]entry, []*file, error) {
	rels, err := yamlFiles(root, requirementsDir)
	if err != nil {
		return nil, nil, err
	}
	vrels, err := yamlFiles(root, vectorsDir)
	if err != nil {
		return nil, nil, err
	}
	rels = append(rels, vrels...)
	idx := map[string]entry{}
	var files []*file
	for _, rel := range rels {
		f, err := readFile(root, rel)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, f)
		for i, l := range f.lines {
			m := itemRe.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			kind := kindItem
			if m[2] == "code" {
				if rel != areasFile {
					continue
				}
				kind = kindArea
			}
			id := m[3]
			st := statusLine(f.lines, i, len(m[1]))
			if st < 0 {
				return nil, nil, fmt.Errorf("%s:%d: %s has no status field", rel, i+1, id)
			}
			if prev, dup := idx[id]; dup {
				return nil, nil, fmt.Errorf("%s: id %s also defined in %s", rel, id, prev.f.rel)
			}
			idx[id] = entry{f: f, line: st, kind: kind}
		}
	}
	mf, err := readFile(root, matrixFile)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, mf)
	for i, l := range mf.lines {
		m := matrixRe.FindStringSubmatch(l)
		if m == nil || !payStRe.MatchString(l) {
			continue
		}
		idx[m[1]+"/"+m[2]] = entry{f: mf, line: i, kind: kindMatrix}
	}
	return idx, files, nil
}

// statusLine returns the index of the "status:" field of the list item that
// starts at line start with the given indentation, or -1.
func statusLine(lines []string, start, indent int) int {
	for j := start + 1; j < len(lines); j++ {
		l := lines[j]
		if strings.TrimSpace(l) == "" {
			continue
		}
		lead := len(l) - len(strings.TrimLeft(l, " "))
		if lead <= indent {
			return -1
		}
		if lead == indent+2 && statusRe.MatchString(l) {
			return j
		}
	}
	return -1
}

func (e entry) get() string {
	l := e.f.lines[e.line]
	if e.kind == kindMatrix {
		return payStRe.FindStringSubmatch(l)[1]
	}
	return statusRe.FindStringSubmatch(l)[4]
}

func (e entry) set(v string) {
	l := e.f.lines[e.line]
	if e.kind == kindMatrix {
		e.f.lines[e.line] = payStRe.ReplaceAllString(l, "payload: {status: "+v)
	} else {
		m := statusRe.FindStringSubmatch(l)
		e.f.lines[e.line] = m[1] + "status:" + m[2] + m[3] + v + m[5] + m[6]
	}
	e.f.changed = true
}

// transition returns the expected source and the target status of a kind.
func transition(kind, to string) (from, target string) {
	switch {
	case kind == kindArea && to == Reviewed:
		return "drafted", "reviewed"
	case kind == kindArea:
		return "reviewed", "drafted"
	case to == Reviewed:
		return "draft", "reviewed"
	default:
		return "reviewed", "draft"
	}
}

// SetStatus sets the given requirements, vectors, areas ("MSG") and payload
// columns of matrix rows ("Pub/MAM", or MatrixAll for all rows) to the target
// status (Reviewed or Draft; areas use drafted instead of draft). Entries that
// already have the target status are reported as unchanged. An area can only
// be set to reviewed if none of its requirements stays draft. Nothing is
// written if any problem is found or dryRun is set.
func SetStatus(root string, ids []string, to string, dryRun bool) ([]Change, error) {
	if to != Reviewed && to != Draft {
		return nil, fmt.Errorf("unknown target status %q (use %s or %s)", to, Reviewed, Draft)
	}
	idx, files, err := index(root)
	if err != nil {
		return nil, err
	}
	var sel []string
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			sel = append(sel, id)
		}
	}
	for _, id := range ids {
		if id == MatrixAll {
			var rows []string
			for k, e := range idx {
				if e.kind == kindMatrix {
					rows = append(rows, k)
				}
			}
			sort.Slice(rows, func(i, j int) bool { return idx[rows[i]].line < idx[rows[j]].line })
			for _, r := range rows {
				add(r)
			}
			continue
		}
		if _, ok := idx[id]; !ok {
			return nil, fmt.Errorf("unknown id %q (requirement, vector, area code, matrix row Method/Resource or %q)", id, MatrixAll)
		}
		add(id)
	}
	var changes []Change
	var problems []string
	for _, id := range sel {
		e := idx[id]
		from, target := transition(e.kind, to)
		cur := e.get()
		switch cur {
		case target:
		case from:
			e.set(target)
		default:
			problems = append(problems, fmt.Sprintf("%s: %s has status %s, expected %s", e.f.rel, id, cur, from))
			continue
		}
		changes = append(changes, Change{File: e.f.rel, ID: id, From: cur, To: target})
	}
	if to == Reviewed {
		problems = append(problems, draftInReviewedAreas(sel, idx, files)...)
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("nothing written: %s", strings.Join(problems, "; "))
	}
	if !dryRun {
		for _, f := range files {
			if f.changed {
				if err := f.write(root); err != nil {
					return nil, err
				}
			}
		}
	}
	return changes, nil
}

// draftInReviewedAreas reports requirements that stay draft in an area that
// is set to reviewed (after the edits in memory).
func draftInReviewedAreas(sel []string, idx map[string]entry, files []*file) []string {
	var problems []string
	for _, id := range sel {
		if idx[id].kind != kindArea {
			continue
		}
		rel := requirementsDir + "/" + strings.ToLower(id) + ".yaml"
		for _, f := range files {
			if f.rel != rel {
				continue
			}
			var drafts []string
			for i, l := range f.lines {
				m := itemRe.FindStringSubmatch(l)
				if m == nil || m[2] != "id" {
					continue
				}
				if st := statusLine(f.lines, i, len(m[1])); st >= 0 && (entry{f: f, line: st, kind: kindItem}).get() == Draft {
					drafts = append(drafts, m[3])
				}
			}
			if len(drafts) > 0 {
				problems = append(problems, fmt.Sprintf("area %s cannot be reviewed, still draft: %s", id, strings.Join(drafts, ", ")))
			}
		}
	}
	return problems
}

// Rebaseline replaces the baseline old by new in all hand-written YAML
// sources (top-level "baseline:" key, always written quoted) and in the header
// comment of the topic grammar. The generated files (inventory, topic
// patterns, views, coverage report) are regenerated by the caller. It returns
// the changed files.
func Rebaseline(root, old, new string, dryRun bool) ([]string, error) {
	if old == "" || new == "" {
		return nil, fmt.Errorf("empty baseline")
	}
	var rels []string
	for _, dir := range []string{requirementsDir, vectorsDir, "coverage", "matrix"} {
		r, err := yamlFiles(root, dir)
		if err != nil {
			return nil, err
		}
		rels = append(rels, r...)
	}
	rels = append(rels, grammarFile)
	baseRe := regexp.MustCompile(`^baseline:\s*"?` + regexp.QuoteMeta(old) + `"?\s*$`)
	var changed []string
	for _, rel := range rels {
		f, err := readFile(root, rel)
		if err != nil {
			return nil, err
		}
		for i, l := range f.lines {
			if rel == grammarFile {
				if strings.HasPrefix(l, ";") && strings.Contains(l, "@ "+old) {
					f.lines[i] = strings.ReplaceAll(l, "@ "+old, "@ "+new)
					f.changed = true
				}
				continue
			}
			if baseRe.MatchString(l) {
				f.lines[i] = `baseline: "` + new + `"`
				f.changed = true
			}
		}
		if !f.changed {
			continue
		}
		changed = append(changed, rel)
		if !dryRun {
			if err := f.write(root); err != nil {
				return nil, err
			}
		}
	}
	return changed, nil
}

// Mentions returns "file:line: excerpt" for every line of the given files
// (relative to base) that contains s; missing files are skipped.
func Mentions(base string, rels []string, s string) []string {
	var out []string
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for i, l := range strings.Split(string(data), "\n") {
			if strings.Contains(l, s) {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, i+1, excerpt(strings.TrimSpace(l), s)))
			}
		}
	}
	return out
}

func excerpt(line, s string) string {
	r := []rune(line)
	if len(r) <= 100 {
		return line
	}
	k := len([]rune(line[:strings.Index(line, s)]))
	start := k - 40
	if start < 0 {
		start = 0
	}
	end := start + 100
	if end > len(r) {
		end = len(r)
	}
	out := string(r[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(r) {
		out += "…"
	}
	return out
}
