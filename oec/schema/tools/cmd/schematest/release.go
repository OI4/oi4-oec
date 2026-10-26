package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	schema "github.com/OI4/oec/schema"
)

// manifest is release/<version>.json: the frozen state of a schema release
// (ADR-0019 §6 and amendment C-041). The $id of a file is Base + Path.
type manifest struct {
	Version  string         `json:"version"`
	Baseline string         `json:"baseline"`
	Base     string         `json:"base"`
	Files    []manifestFile `json:"files"`
}

type manifestFile struct {
	Path   string   `json:"path"`
	SHA256 string   `json:"sha256"` // of the canonical JSON (sorted keys, no white space)
	Defs   []string `json:"defs,omitempty"`
}

// versionPattern is a v2 bundle version: 2.MINOR.PATCH, optionally with a
// pre-release suffix for the work after a release (e.g. 2.1.0-dev).
var versionPattern = regexp.MustCompile(`^2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)

// current builds the manifest of the strict source files in docs.
func current(docs map[string]any) (manifest, error) {
	m := manifest{Version: schema.Version, Baseline: schema.Baseline, Base: schema.BaseURI}
	for id, doc := range docs {
		if strings.HasPrefix(id, schema.TolerantBaseURI) {
			continue
		}
		canon, err := json.Marshal(doc)
		if err != nil {
			return m, fmt.Errorf("%s: %w", id, err)
		}
		sum := sha256.Sum256(canon)
		f := manifestFile{Path: strings.TrimPrefix(id, schema.BaseURI), SHA256: hex.EncodeToString(sum[:])}
		if defs, ok := doc.(map[string]any)["$defs"].(map[string]any); ok {
			for d := range defs {
				f.Defs = append(f.Defs, d)
			}
			sort.Strings(f.Defs)
		}
		m.Files = append(m.Files, f)
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	return m, nil
}

// writeRelease freezes the current tree as release/<schema.Version>.json.
func writeRelease(docs map[string]any, dir string) error {
	if !versionPattern.MatchString(schema.Version) || strings.Contains(schema.Version, "-") {
		return fmt.Errorf("schema.Version %q is no release version 2.MINOR.PATCH", schema.Version)
	}
	p := filepath.Join(dir, schema.Version+".json")
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("%s exists: a released version is never rewritten", p)
	}
	m, err := current(docs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(p, m)
}

// checkRelease enforces the freeze rules (ADR-0019 amendment C-041):
//   - schema.Version is 2.MINOR.PATCH[-pre], schema.Baseline is the baseline
//     of the spec catalog;
//   - a release version has its manifest, which equals the current tree
//     (a released version is immutable), and a CHANGELOG section;
//   - a pre-release version has no manifest and is above every release;
//   - no $id and no $defs name of any release disappears (within v2, a
//     removal is a breaking change).
func checkRelease(docs map[string]any, dir, changelog, specBaseline string) ([]string, error) {
	var problems []string
	if !versionPattern.MatchString(schema.Version) {
		problems = append(problems, fmt.Sprintf("schema.Version %q is not 2.MINOR.PATCH[-pre]", schema.Version))
	}
	if schema.Baseline != specBaseline {
		problems = append(problems, fmt.Sprintf("schema.Baseline %q, the spec catalog is at %q", schema.Baseline, specBaseline))
	}
	cur, err := current(docs)
	if err != nil {
		return nil, err
	}
	files := map[string]manifestFile{}
	for _, f := range cur.Files {
		files[f.Path] = f
	}
	log, err := os.ReadFile(changelog)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pre := strings.Contains(schema.Version, "-")
	own := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m manifest
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		where := "release " + m.Version
		if e.Name() != m.Version+".json" || !versionPattern.MatchString(m.Version) || strings.Contains(m.Version, "-") {
			problems = append(problems, fmt.Sprintf("%s: version %q does not match the file name or is no release version", e.Name(), m.Version))
			continue
		}
		if !regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(m.Version) + `\b`).Match(log) {
			problems = append(problems, fmt.Sprintf("%s: no section \"## %s\" in %s", where, m.Version, filepath.Base(changelog)))
		}
		if m.Base != schema.BaseURI {
			problems = append(problems, fmt.Sprintf("%s: base %q, the bundle has %q", where, m.Base, schema.BaseURI))
		}
		for _, f := range m.Files {
			g, ok := files[f.Path]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: %s%s was removed", where, m.Base, f.Path))
				continue
			}
			for _, d := range f.Defs {
				if !slices.Contains(g.Defs, d) {
					problems = append(problems, fmt.Sprintf("%s: %s%s#/$defs/%s was removed", where, m.Base, f.Path, d))
				}
			}
		}
		switch {
		case m.Version == schema.Version:
			own = true
			problems = append(problems, diffRelease(m, cur)...)
		case pre && !less(m.Version, schema.Version):
			problems = append(problems, fmt.Sprintf("schema.Version %s is not above the release %s", schema.Version, m.Version))
		}
	}
	if !pre && !own {
		problems = append(problems, fmt.Sprintf("schema.Version %s has no manifest %s/%s.json (schematest -write-release)", schema.Version, filepath.Base(dir), schema.Version))
	}
	return problems, nil
}

// diffRelease reports every difference between the manifest of the current
// version and the tree: a released version is immutable.
func diffRelease(m, cur manifest) []string {
	var out []string
	where := "release " + m.Version
	if m.Baseline != cur.Baseline {
		out = append(out, fmt.Sprintf("%s: baseline %s, schema.Baseline is %s (bump schema.Version)", where, m.Baseline, cur.Baseline))
	}
	rel := map[string]manifestFile{}
	for _, f := range m.Files {
		rel[f.Path] = f
	}
	for _, f := range cur.Files {
		r, ok := rel[f.Path]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s: %s is not part of the release (bump schema.Version)", where, f.Path))
		case r.SHA256 != f.SHA256:
			out = append(out, fmt.Sprintf("%s: %s changed after the release (bump schema.Version)", where, f.Path))
		case !slices.Equal(r.Defs, f.Defs):
			out = append(out, fmt.Sprintf("%s: %s has other $defs than the release (bump schema.Version)", where, f.Path))
		}
	}
	return out
}

// less compares two versions 2.MINOR.PATCH[-pre] by semantic versioning
// precedence (a pre-release is below its release).
func less(a, b string) bool {
	pa, pb := versionPattern.FindStringSubmatch(a), versionPattern.FindStringSubmatch(b)
	for i := 1; i <= 2; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	switch {
	case pa[3] == pb[3]:
		return false
	case pa[3] == "":
		return false
	case pb[3] == "":
		return true
	}
	return pa[3] < pb[3]
}
