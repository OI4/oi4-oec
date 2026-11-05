// Package guideline reads the OEC guideline Markdown sources (MG + RIG) and
// extracts the candidates of normative statements (MG 1.2) for the inventory.
package guideline

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Document is one guideline document inside the guideline repository.
type Document struct {
	Code string // "MG" or "RIG"
	Dir  string // relative to the guideline repository root
}

// Documents are the two normative guideline documents (ADR-0001, G-1.1).
var Documents = []Document{
	{Code: "MG", Dir: "docs/guideline"},
	{Code: "RIG", Dir: "docs/infrastructure-guideline"},
}

// excluded are informative parts that never carry requirements: examples
// (Appendix B, MG 1.2), lists of figures/tables, authors and history (one
// file up to C-111, one page per revision since C-112).
var exampleDirs = map[string]bool{"appendix-b": true}
var excludedDirs = map[string]bool{"a6-history": true}
var excludedFiles = map[string]bool{
	"list-of-figures.md": true, "list-of-tables.md": true,
	"a5-authors.md": true, "a6-history.md": true,
}

var h1Pattern = regexp.MustCompile(`^#\s+(?:([0-9]+(?:\.[0-9]+)*|[A-Z][0-9]+(?:\.[0-9]+)*)\s+)?(.+?)\s*$`)

// File is one Markdown file of a guideline document.
type File struct {
	Doc     string
	Path    string // relative to the guideline repository root, forward slashes
	Section string // section number ("7.2.1", "A1") or slug for unnumbered files ("index", "preface")
	Title   string
	Lines   []string
}

// Ref returns the citation of the file's section, e.g. "MG 7.2.1".
func (f File) Ref() string { return f.Doc + " " + f.Section }

// LoadFiles reads all relevant Markdown files of MG and RIG below root.
func LoadFiles(root string) ([]File, error) { return loadFiles(root, false) }

// LoadFilesWithExamples is LoadFiles plus the informative examples of MG
// Appendix B, used to verify vectors taken from the examples.
func LoadFilesWithExamples(root string) ([]File, error) { return loadFiles(root, true) }

func loadFiles(root string, examples bool) ([]File, error) {
	var files []File
	for _, d := range Documents {
		base := filepath.Join(root, filepath.FromSlash(d.Dir))
		err := filepath.WalkDir(base, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				if excludedDirs[e.Name()] || exampleDirs[e.Name()] && !examples {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(p) != ".md" || excludedFiles[e.Name()] {
				return nil
			}
			f, err := readFile(root, base, p, d.Code)
			if err != nil {
				return err
			}
			files = append(files, f)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", d.Code, err)
		}
	}
	sort.Slice(files, func(i, j int) bool { return lessFile(files[i], files[j]) })
	return files, nil
}

func readFile(root, base, p, doc string) (File, error) {
	fh, err := os.Open(p)
	if err != nil {
		return File{}, err
	}
	defer fh.Close()
	var lines []string
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		return File{}, err
	}
	rel, _ := filepath.Rel(root, p)
	f := File{Doc: doc, Path: filepath.ToSlash(rel), Lines: lines}
	for _, l := range lines {
		if m := h1Pattern.FindStringSubmatch(l); m != nil {
			f.Section, f.Title = m[1], m[2]
			break
		}
	}
	if f.Section == "" {
		relDoc, _ := filepath.Rel(base, p)
		slug := strings.TrimSuffix(filepath.ToSlash(relDoc), ".md")
		slug = strings.TrimSuffix(slug, "/index")
		f.Section = strings.ToLower(strings.ReplaceAll(slug, " ", "-"))
	}
	return f, nil
}

func lessFile(a, b File) bool {
	if a.Doc != b.Doc {
		return docOrder(a.Doc) < docOrder(b.Doc)
	}
	if c := CompareSections(a.Section, b.Section); c != 0 {
		return c < 0
	}
	return a.Path < b.Path
}

func docOrder(code string) int {
	for i, d := range Documents {
		if d.Code == code {
			return i
		}
	}
	return len(Documents)
}

// CompareSections orders sections naturally: unnumbered slugs first, then
// numbered chapters (1, 1.1, 1.2, 2, …, 10), then appendices (A1, A2, …).
func CompareSections(a, b string) int {
	ka, kb := sectionKey(a), sectionKey(b)
	for i := 0; i < len(ka) && i < len(kb); i++ {
		if ka[i] != kb[i] {
			if ka[i] < kb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ka) < len(kb):
		return -1
	case len(ka) > len(kb):
		return 1
	}
	return strings.Compare(a, b)
}

func sectionKey(s string) []int {
	if s == "" {
		return []int{0}
	}
	group := 1
	if s[0] >= 'A' && s[0] <= 'Z' {
		group = 2
		s = s[1:]
	} else if s[0] < '0' || s[0] > '9' {
		return []int{0}
	}
	key := []int{group}
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		key = append(key, n)
	}
	return key
}

// SectionWithin reports whether section ref s (e.g. "MG 7.2.1") lies within
// prefix p (e.g. "MG 7.2" or "MG 7.2.1"), respecting section boundaries.
func SectionWithin(s, p string) bool {
	return s == p || strings.HasPrefix(s, p+".")
}
