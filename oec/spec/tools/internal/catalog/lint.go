package catalog

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/guideline"
)

// LintOptions provide optional cross-checks.
type LintOptions struct {
	// Inventory items by key; nil skips inventory checks.
	Inventory map[string]guideline.Item
	// Normalized guideline text per section ref; nil skips quote/ref checks.
	Sections map[string]string
}

var rank = map[string]int{"none": 0, "MAY": 1, "SHOULD": 2, "MUST": 3, "SHOULD NOT": 2, "MUST NOT": 3}

func negative(o string) bool { return strings.HasSuffix(o, " NOT") }

// Lint checks all rules that JSON Schema cannot express. It returns all
// findings; an empty result means the catalog is consistent.
func (s *Spec) Lint(opt LintOptions) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	// Area registry.
	codes := map[string]bool{}
	sectionOwner := map[string]string{}
	for _, a := range s.Areas.Areas {
		if codes[a.Code] {
			add("%s: duplicate area %s", AreasFile, a.Code)
		}
		codes[a.Code] = true
		for _, sec := range a.Sections {
			if o, dup := sectionOwner[sec]; dup {
				add("%s: section %q assigned to %s and %s", AreasFile, sec, o, a.Code)
			}
			sectionOwner[sec] = a.Code
			if opt.Sections != nil && !sectionExists(opt.Sections, sec) {
				add("%s: area %s: section %q does not exist in the guideline", AreasFile, a.Code, sec)
			}
		}
	}
	files := map[string]bool{}
	for _, f := range s.Files {
		files[f.Area] = true
		if f.Path != RequirementsDir+"/"+AreaFileName(f.Area) {
			add("%s: area %s must be stored in %s", f.Path, f.Area, AreaFileName(f.Area))
		}
		if !codes[f.Area] {
			add("%s: area %s is not registered in %s", f.Path, f.Area, AreasFile)
		}
		if f.Baseline != s.Areas.Baseline {
			add("%s: baseline %s differs from %s (%s)", f.Path, f.Baseline, AreasFile, s.Areas.Baseline)
		}
	}
	for _, a := range s.Areas.Areas {
		if a.Status != "planned" && !files[a.Code] {
			add("%s: area %s has status %s but no %s", AreasFile, a.Code, a.Status, AreaFileName(a.Code))
		}
	}
	if s.Informative.Baseline != "" && s.Informative.Baseline != s.Areas.Baseline {
		add("%s: baseline %s differs from %s (%s)", InformativeFile, s.Informative.Baseline, AreasFile, s.Areas.Baseline)
	}

	// Requirements.
	ids := map[string]bool{}
	mapped := map[string]string{}
	for _, f := range s.Files {
		last := 0
		for _, r := range f.Requirements {
			at := f.Path + ": " + r.ID
			prefix := "REQ-" + f.Area + "-"
			if !strings.HasPrefix(r.ID, prefix) {
				add("%s: id must start with %s", at, prefix)
			} else if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, prefix)); err != nil {
				add("%s: id number is not numeric", at)
			} else {
				if n <= last {
					add("%s: ids must be ascending", at)
				}
				last = n
			}
			if ids[r.ID] {
				add("%s: duplicate id", at)
			}
			ids[r.ID] = true
			lintLevels(r, at, add)
			refs := map[string]bool{}
			for _, ref := range r.Refs {
				refs[ref] = true
				if opt.Sections != nil && (strings.HasPrefix(ref, "MG ") || strings.HasPrefix(ref, "RIG ")) && !sectionExists(opt.Sections, ref) {
					add("%s: ref %q does not exist in the guideline", at, ref)
				}
			}
			for _, sc := range r.Scope {
				if opt.Sections != nil && !sectionExists(opt.Sections, sc) {
					add("%s: scope %q does not exist in the guideline", at, sc)
				}
			}
			for _, q := range r.Quotes {
				if !refs[q.Ref] {
					add("%s: quote ref %q is not listed in refs", at, q.Ref)
				}
				if opt.Sections != nil {
					text, ok := opt.Sections[q.Ref]
					if !ok {
						add("%s: quote ref %q has no guideline file", at, q.Ref)
					} else if !strings.Contains(text, guideline.NormalizeForCompare(q.Text)) {
						add("%s: quote not found verbatim in %s: %q", at, q.Ref, q.Text)
					}
				}
			}
			for _, k := range r.Inventory {
				if opt.Inventory != nil {
					if _, ok := opt.Inventory[k]; !ok {
						add("%s: inventory key %s does not exist (guideline changed? regenerate and remap)", at, k)
					}
				}
				mapped[k] = r.ID
			}
		}
	}
	for _, f := range s.Files {
		for _, r := range f.Requirements {
			if r.ReplacedBy != "" && !ids[r.ReplacedBy] {
				add("%s: %s: replaced_by %s does not exist", f.Path, r.ID, r.ReplacedBy)
			}
		}
	}

	// Informative classification.
	info := map[string]bool{}
	for _, it := range s.Informative.Items {
		if info[it.Key] {
			add("%s: duplicate key %s", InformativeFile, it.Key)
		}
		info[it.Key] = true
		if id, ok := mapped[it.Key]; ok {
			add("%s: key %s is informative but also mapped by %s", InformativeFile, it.Key, id)
		}
		if opt.Inventory != nil {
			if _, ok := opt.Inventory[it.Key]; !ok {
				add("%s: key %s does not exist in the inventory", InformativeFile, it.Key)
			}
		}
	}
	return errs
}

func lintLevels(r Requirement, at string, add func(string, ...any)) {
	if r.Levels == nil {
		return
	}
	l := []string{r.Levels.L1, r.Levels.L2, r.Levels.L3}
	if l[2] != r.Keyword {
		add("%s: keyword %s must equal the Level 3 obligation %s", at, r.Keyword, l[2])
	}
	pos, neg := false, false
	for i, o := range l {
		if o == "none" {
			continue
		}
		if negative(o) {
			neg = true
		} else {
			pos = true
		}
		if i > 0 && rank[o] < rank[l[i-1]] {
			add("%s: levels must not weaken from L%d (%s) to L%d (%s)", at, i, l[i-1], i+1, o)
		}
	}
	if pos && neg {
		add("%s: levels mix positive and negative obligations", at)
	}
}

// sectionExists reports whether a section ref exists as a guideline file or
// as a parent of one (e.g. "MG 8.2" when only 8.2.x files exist).
func sectionExists(sections map[string]string, ref string) bool {
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

// AreaOf returns the area owning a section ref (longest assigned prefix at a
// section boundary) or "" if none.
func (s *Spec) AreaOf(section string) string {
	best, bestLen := "", -1
	for _, a := range s.Areas.Areas {
		for _, sec := range a.Sections {
			if guideline.SectionWithin(section, sec) && len(sec) > bestLen {
				best, bestLen = a.Code, len(sec)
			}
		}
	}
	return best
}
