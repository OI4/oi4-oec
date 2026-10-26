package catalog

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/OI4/oec/spec"
	"go.yaml.in/yaml/v3"
)

type requirement struct {
	ID           string            `yaml:"id"`
	Status       string            `yaml:"status"`
	Verification []string          `yaml:"verification"`
	Levels       map[string]string `yaml:"levels"`
}

func requirements(t *testing.T) (map[string]requirement, string) {
	t.Helper()
	paths, err := fs.Glob(spec.FS, spec.RequirementsGlob)
	if err != nil {
		t.Fatal(err)
	}
	reqs := map[string]requirement{}
	baseline := ""
	for _, p := range paths {
		raw, err := fs.ReadFile(spec.FS, p)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Baseline     string        `yaml:"baseline"`
			Requirements []requirement `yaml:"requirements"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if doc.Requirements == nil {
			continue // areas.yaml
		}
		baseline = doc.Baseline
		for _, r := range doc.Requirements {
			reqs[r.ID] = r
		}
	}
	return reqs, baseline
}

// minLevel is the lowest level at which the requirement applies, 0 if it is
// not graded.
func minLevel(r requirement) int {
	for i, l := range []string{"L1", "L2", "L3"} {
		if o, ok := r.Levels[l]; ok && o != "none" {
			return i + 1
		}
	}
	return 0
}

type toolRules struct {
	file   string
	tool   string
	prefix string
	modes  map[string]string // mode -> verification kind it covers
	needs  []string
}

var tools = []toolRules{
	{D7File, "oec-check", "T7", map[string]string{"monitor": "d7-monitor", "test": "d7-test"}, []string{"proxy", "restart-hook", "peer"}},
	{D8File, "oec-infra", "T8", map[string]string{"image": "d8", "run": "d8", "container": "d8", "host": "d8", "bus": "d8"}, []string{"docker", "host-access", "credentials-2"}},
}

var testID = regexp.MustCompile(`^T[78]-[A-Z]+-[0-9]{2}$`)

// TestCatalogs checks each catalog against the requirements catalog and
// that every d7-monitor, d7-test and d8 requirement is covered (ADR-0025).
func TestCatalogs(t *testing.T) {
	reqs, baseline := requirements(t)
	ids := map[string]bool{}
	for _, tr := range tools {
		c, err := Load(tr.file)
		if err != nil {
			t.Fatal(err)
		}
		if c.Tool != tr.tool {
			t.Errorf("%s: tool %q, want %q", tr.file, c.Tool, tr.tool)
		}
		if c.Baseline != baseline {
			t.Errorf("%s: baseline %q, requirements have %q", tr.file, c.Baseline, baseline)
		}
		covered := map[string]map[string]bool{} // kind -> req
		for _, kind := range tr.modes {
			covered[kind] = map[string]bool{}
		}
		for _, tc := range c.Tests {
			where := fmt.Sprintf("%s %s", tr.file, tc.ID)
			if !testID.MatchString(tc.ID) || !strings.HasPrefix(tc.ID, tr.prefix+"-") {
				t.Errorf("%s: bad id", where)
			}
			if ids[tc.ID] {
				t.Errorf("%s: duplicate id", where)
			}
			ids[tc.ID] = true
			if tc.Title == "" || len(tc.Reqs) == 0 {
				t.Errorf("%s: title and reqs are required", where)
			}
			if tc.Status != "planned" && tc.Status != "implemented" {
				t.Errorf("%s: status %q", where, tc.Status)
			}
			kind, ok := tr.modes[tc.Mode]
			if !ok {
				t.Errorf("%s: mode %q", where, tc.Mode)
				continue
			}
			for _, n := range tc.Needs {
				if !slices.Contains(tr.needs, n) {
					t.Errorf("%s: need %q", where, n)
				}
			}
			need := 0 // lowest level at which one of its requirements applies
			for _, id := range tc.Reqs {
				r, ok := reqs[id]
				switch {
				case !ok:
					t.Errorf("%s: unknown %s", where, id)
					continue
				case r.Status == "deprecated":
					t.Errorf("%s: %s is deprecated", where, id)
				case !slices.Contains(r.Verification, kind):
					t.Errorf("%s: %s has no verification %s", where, id, kind)
				}
				covered[kind][id] = true
				if l := minLevel(r); l > 0 && (need == 0 || l < need) {
					need = l
				}
			}
			switch {
			case need == 0 && tc.Level != 0:
				t.Errorf("%s: level %d, but only ungraded requirements", where, tc.Level)
			case need > 0 && tc.Level != need:
				t.Errorf("%s: level %d, the first of its requirements applies at level %d", where, tc.Level, need)
			}
		}
		manual := map[string]bool{}
		for _, m := range c.Manual {
			r, ok := reqs[m.Req]
			if !ok || !slices.Contains(r.Verification, "manual") || m.Reason == "" {
				t.Errorf("%s manual %s: needs an existing requirement with verification manual and a reason", tr.file, m.Req)
			}
			manual[m.Req] = true
		}
		var missing []string
		for id, r := range reqs {
			if r.Status == "deprecated" {
				continue
			}
			for kind := range covered {
				if slices.Contains(r.Verification, kind) && !covered[kind][id] && !manual[id] {
					missing = append(missing, kind+" "+id)
				}
			}
		}
		sort.Strings(missing)
		for _, m := range missing {
			t.Errorf("%s: not covered: %s", tr.file, m)
		}
	}
}
