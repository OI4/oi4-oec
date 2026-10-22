// Package examples relates the code blocks of the guideline (MG + RIG,
// including the examples of MG Appendix B) to the golden vectors: every
// block is either reproduced by a vector of its section, a JSON fragment of a
// golden payload vector, or explained in coverage/examples.yaml (ADR-0018,
// C-031).
package examples

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/payload"
	"github.com/OI4/oec/spec/tools/internal/schema"
	"github.com/OI4/oec/spec/tools/internal/vectors"
	"github.com/OI4/oec/spec/tools/internal/yamlstrict"
)

const (
	File   = "coverage/examples.yaml"
	Schema = "coverage/examples.schema.json"
)

// Block is one fenced code block of a guideline file.
type Block struct {
	Key  string
	Ref  string // section, e.g. "MG B2.2"
	Path string // file relative to the guideline repository
	Line int    // 1-based line of the opening fence
	Lang string
	Text string
}

// Extract returns the fenced code blocks of the files in file order. Keys
// are derived from the content (like inventory keys), not from line numbers.
func Extract(files []guideline.File) []Block {
	var out []Block
	for _, f := range files {
		seen := map[string]int{}
		var cur *Block
		var body []string
		var fence string
		for i, l := range f.Lines {
			t := strings.TrimSpace(l)
			if cur == nil {
				if strings.HasPrefix(t, "```") {
					fence = t[:len(t)-len(strings.TrimLeft(t, "`"))]
					cur = &Block{Ref: f.Ref(), Path: f.Path, Line: i + 1, Lang: strings.TrimSpace(t[len(fence):])}
					body = nil
				}
				continue
			}
			if strings.HasPrefix(t, fence) && strings.Trim(t, "`") == "" {
				cur.Text = strings.Join(body, "\n")
				base := strings.Join([]string{f.Doc, f.Section, "example", guideline.NormalizeCodeForCompare(cur.Text)}, "\x1f")
				seen[base]++
				if n := seen[base]; n > 1 {
					base += "\x1f" + strconv.Itoa(n)
				}
				sum := sha256.Sum256([]byte(base))
				cur.Key = f.Doc + "-" + f.Section + "-" + hex.EncodeToString(sum[:4])
				out = append(out, *cur)
				cur = nil
				continue
			}
			body = append(body, l)
		}
	}
	return out
}

// Explained is one entry of coverage/examples.yaml.
type Explained struct {
	Keys   []string `json:"keys"`
	Reason string   `json:"reason"`
}

// ExplainedFile is coverage/examples.yaml.
type ExplainedFile struct {
	Baseline string      `json:"baseline"`
	Items    []Explained `json:"items"`
}

// Status of one block.
type Status struct {
	Block
	Vectors  []string // golden vectors of the block's section taken from it
	Fragment []string // golden payload vectors containing the block as a JSON value
	Reason   string
}

// Covered reports whether a golden vector reproduces the block.
func (s Status) Covered() bool { return len(s.Vectors) > 0 || len(s.Fragment) > 0 }

// Result is the coverage of all blocks.
type Result struct {
	Blocks   []Status
	Problems []string
}

type golden struct {
	id, ref, text string
	parsed        any
}

// Compute classifies every block.
func Compute(blocks []Block, vfiles []vectors.File, ex ExplainedFile) *Result {
	var gs []golden
	for _, f := range vfiles {
		if f.Kind != "payload" {
			continue
		}
		for _, v := range f.Vectors {
			if v.Origin.Kind != "guideline" {
				continue
			}
			g := golden{id: v.ID, ref: v.Origin.Ref, text: v.Payload}
			if v.Original != "" {
				g.text = v.Original
			}
			if p, err := payload.Parse([]byte(v.Payload)); err == nil {
				g.parsed = p
			}
			gs = append(gs, g)
		}
	}
	res := &Result{}
	reasons := map[string]string{}
	for _, it := range ex.Items {
		for _, k := range it.Keys {
			if _, dup := reasons[k]; dup {
				res.Problems = append(res.Problems, fmt.Sprintf("%s: duplicate key %s", File, k))
			}
			reasons[k] = it.Reason
		}
	}
	known := map[string]bool{}
	for _, b := range blocks {
		known[b.Key] = true
		st := Status{Block: b, Reason: reasons[b.Key]}
		norm := guideline.NormalizeCodeForCompare(b.Text)
		frag, members := parseFragment(b.Text)
		for _, g := range gs {
			switch {
			case g.ref == b.Ref && strings.Contains(norm, guideline.NormalizeCodeForCompare(g.text)):
				st.Vectors = append(st.Vectors, g.id)
			case g.parsed == nil:
			case frag != nil && containsValue(g.parsed, frag),
				members != nil && containsMembers(g.parsed, members):
				st.Fragment = append(st.Fragment, g.id)
			}
		}
		switch {
		case st.Covered() && st.Reason != "":
			res.Problems = append(res.Problems, fmt.Sprintf("%s: example %s (%s) is reproduced by %s – drop the explanation", File, b.Key, b.Ref, strings.Join(append(append([]string{}, st.Vectors...), st.Fragment...), ", ")))
		case !st.Covered() && st.Reason == "":
			res.Problems = append(res.Problems, fmt.Sprintf("example %s (%s:%d) has no golden vector and no explanation in %s", b.Key, b.Path, b.Line, File))
		}
		res.Blocks = append(res.Blocks, st)
	}
	var stale []string
	for k := range reasons {
		if !known[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		res.Problems = append(res.Problems, fmt.Sprintf("%s: key %s is no code block of the guideline", File, k))
	}
	return res
}

// parseFragment parses a block as a JSON object or array (frag) or as a
// list of object members such as `"Payload": {...}` (members).
func parseFragment(text string) (frag any, members map[string]any) {
	if p, err := payload.Parse([]byte(text)); err == nil {
		switch p.(type) {
		case map[string]any, []any:
			return p, nil
		}
		return nil, nil
	}
	if p, err := payload.Parse([]byte("{" + text + "}")); err == nil {
		if m, ok := p.(map[string]any); ok && len(m) > 0 {
			return nil, m
		}
	}
	return nil, nil
}

// containsMembers reports whether an object in doc has all members of m
// with equal values.
func containsMembers(doc any, m map[string]any) bool {
	switch d := doc.(type) {
	case map[string]any:
		all := true
		for k, v := range m {
			if x, ok := d[k]; !ok || !reflect.DeepEqual(x, v) {
				all = false
				break
			}
		}
		if all {
			return true
		}
		for _, x := range d {
			if containsMembers(x, m) {
				return true
			}
		}
	case []any:
		for _, x := range d {
			if containsMembers(x, m) {
				return true
			}
		}
	}
	return false
}

// containsValue reports whether v occurs in doc as a whole JSON value.
func containsValue(doc, v any) bool {
	if reflect.DeepEqual(doc, v) {
		return true
	}
	switch d := doc.(type) {
	case map[string]any:
		for _, x := range d {
			if containsValue(x, v) {
				return true
			}
		}
	case []any:
		for _, x := range d {
			if containsValue(x, v) {
				return true
			}
		}
	}
	return false
}

// Counts of one document part.
type Counts struct{ Blocks, Covered, Explained, Open int }

// Summary counts the blocks per document part ("MG", "MG Appendix B", "RIG").
func (r *Result) Summary() ([]string, map[string]*Counts) {
	order := []string{"MG", "MG Appendix B", "RIG"}
	m := map[string]*Counts{}
	for _, o := range order {
		m[o] = &Counts{}
	}
	for _, s := range r.Blocks {
		part := strings.SplitN(s.Ref, " ", 2)[0]
		if strings.HasPrefix(s.Ref, "MG B") {
			part = "MG Appendix B"
		}
		c := m[part]
		if c == nil {
			c = &Counts{}
			m[part] = c
			order = append(order, part)
		}
		c.Blocks++
		switch {
		case s.Covered():
			c.Covered++
		case s.Reason != "":
			c.Explained++
		default:
			c.Open++
		}
	}
	return order, m
}

// Report renders the examples section of coverage/report.md.
func (r *Result) Report() string {
	var b strings.Builder
	b.WriteString("\n## Guideline examples\n\n")
	b.WriteString("Every fenced code block of MG and RIG (including MG Appendix B) is reproduced by a golden vector of its section, is a JSON fragment of a golden payload vector, or is explained in `coverage/examples.yaml`.\n\n")
	b.WriteString("| Part | Blocks | Golden vector | Explained | Open |\n|---|---:|---:|---:|---:|\n")
	order, m := r.Summary()
	var t Counts
	for _, o := range order {
		c := m[o]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", o, c.Blocks, c.Covered, c.Explained, c.Open)
		t.Blocks += c.Blocks
		t.Covered += c.Covered
		t.Explained += c.Explained
		t.Open += c.Open
	}
	fmt.Fprintf(&b, "| **Total** | %d | %d | %d | %d |\n", t.Blocks, t.Covered, t.Explained, t.Open)
	b.WriteString("\n| Example | Section | Line | Language | Golden vectors / reason |\n|---|---|---:|---|---|\n")
	for _, s := range r.Blocks {
		lang := s.Lang
		if lang == "" {
			lang = "–"
		}
		var parts []string
		if len(s.Vectors) > 0 {
			parts = append(parts, strings.Join(s.Vectors, ", "))
		}
		if len(s.Fragment) > 0 {
			parts = append(parts, "fragment of "+strings.Join(s.Fragment, ", "))
		}
		what := strings.Join(parts, "; ")
		switch {
		case what == "" && s.Reason != "":
			what = "explained: " + strings.ReplaceAll(s.Reason, "|", "\\|")
		case what == "":
			what = "**open**"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %s | %s |\n", s.Key, s.Ref, s.Line, lang, what)
	}
	return b.String()
}

// Load reads and schema-validates coverage/examples.yaml below root; a
// missing file yields an empty list.
func Load(root string) (ExplainedFile, error) {
	var ex ExplainedFile
	p := filepath.Join(root, filepath.FromSlash(File))
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return ex, nil
	}
	v, err := yamlstrict.LoadFile(p)
	if err != nil {
		return ex, err
	}
	val, err := schema.Compile(filepath.Join(root, filepath.FromSlash(Schema)))
	if err != nil {
		return ex, err
	}
	if err := val.Validate(v); err != nil {
		return ex, fmt.Errorf("%s: %w", File, err)
	}
	if err := yamlstrict.Decode(v, &ex); err != nil {
		return ex, fmt.Errorf("%s: %w", File, err)
	}
	return ex, nil
}
