package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	schema "github.com/OI4/oec/schema"
	"go.yaml.in/yaml/v3"
)

type requirement struct {
	ID           string   `yaml:"id"`
	Refs         []string `yaml:"refs"`
	Verification []string `yaml:"verification"`
	Status       string   `yaml:"status"`
}

func loadRequirements(dir string) (map[string]requirement, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	out := map[string]requirement{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Requirements []requirement `yaml:"requirements"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, r := range doc.Requirements {
			out[r.ID] = r
		}
	}
	return out, nil
}

// commentRefs matches the reference list right after the leading REQ ids of
// a "$comment": "REQ-… (MG 8.2.3, MG 7.7): …".
var commentRefs = regexp.MustCompile(`^(?:REQ-[A-Z]+(?:-[A-Z]+)*-[0-9]{3}[,;]?\s+)+\(([^)]*)\)`)

// checkComments checks the REQ tracing of ADR-0019 §1 against the catalog:
// every REQ named at the start of a "$comment" exists, is not deprecated and
// carries the verification method schema, and every reference in the
// parentheses after the ids is a ref of at least one of these REQs.
func checkComments(docs map[string]any, reqs map[string]requirement) []string {
	var problems []string
	ids := make([]string, 0, len(docs))
	for id := range docs {
		if !strings.HasPrefix(id, schema.TolerantBaseURI) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		walk(docs[id], "", func(ptr, c string) {
			names := leadingReqs(c)
			if len(names) == 0 {
				return
			}
			var refs []string
			for _, n := range names {
				r, ok := reqs[n]
				switch {
				case !ok:
					problems = append(problems, fmt.Sprintf("%s#%s: unknown %s", id, ptr, n))
				case r.Status == "deprecated":
					problems = append(problems, fmt.Sprintf("%s#%s: %s is deprecated", id, ptr, n))
				case !slices.Contains(r.Verification, "schema"):
					problems = append(problems, fmt.Sprintf("%s#%s: %s has no verification schema", id, ptr, n))
				}
				refs = append(refs, r.Refs...)
			}
			m := commentRefs.FindStringSubmatch(c)
			if m == nil {
				return
			}
			for _, ref := range strings.Split(m[1], ",") {
				if ref = strings.TrimSpace(ref); !slices.Contains(refs, ref) {
					problems = append(problems, fmt.Sprintf("%s#%s: %q is no ref of %v", id, ptr, ref, names))
				}
			}
		})
	}
	return problems
}

func walk(node any, ptr string, f func(ptr, comment string)) {
	switch t := node.(type) {
	case map[string]any:
		if c, ok := t["$comment"].(string); ok {
			f(ptr, c)
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(t[k], ptr+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(k), f)
		}
	case []any:
		for i, v := range t {
			walk(v, fmt.Sprintf("%s/%d", ptr, i), f)
		}
	}
}
