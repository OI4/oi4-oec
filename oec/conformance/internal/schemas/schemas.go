// Package schemas is the schema layer of D7 validate (ADR-0019 §5): it
// compiles the embedded JSON Schema files of D1 in the strict and the
// tolerant flavor, validates a message against its entry schema and maps
// every error to the REQ ids of the schema comments. The R-4 probes add the
// number and calendar semantics JSON Schema cannot express.
package schemas

import (
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/OI4/oec/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const draft2020 = "https://json-schema.org/draft/2020-12/schema"

// Set is the compiled schema bundle of D1.
type Set struct {
	docs     map[string]any
	compiler *jsonschema.Compiler
	mu       sync.Mutex
	compiled map[string]*jsonschema.Schema
}

// Load reads the embedded schema files and derives the tolerant flavor.
func Load() (*Set, error) {
	s := &Set{docs: map[string]any{}, compiled: map[string]*jsonschema.Schema{}}
	err := fs.WalkDir(schema.FS, "v2", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".schema.json") {
			return err
		}
		f, err := schema.FS.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		doc, err := jsonschema.UnmarshalJSON(f)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		m, ok := doc.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: no JSON object", p)
		}
		rel := strings.TrimPrefix(p, "v2/")
		if id, _ := m["$id"].(string); id != schema.BaseURI+rel {
			return fmt.Errorf("%s: $id %q, want %q", p, id, schema.BaseURI+rel)
		}
		if m["$schema"] != draft2020 {
			return fmt.Errorf("%s: $schema is not draft 2020-12", p)
		}
		s.docs[schema.BaseURI+rel] = m
		tol := withoutClosures(m).(map[string]any)
		tol["$id"] = schema.TolerantBaseURI + rel
		s.docs[schema.TolerantBaseURI+rel] = tol
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(s.docs) == 0 {
		return nil, fmt.Errorf("no schema files embedded")
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	ids := make([]string, 0, len(s.docs))
	for id := range s.docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := c.AddResource(id, s.docs[id]); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
	}
	s.compiler = c
	return s, nil
}

// withoutClosures derives the tolerant flavor of REQ-MSG-046: every
// "unevaluatedProperties" (used for closures only, ADR-0019 §3) is dropped;
// keys forbidden explicitly ("not": {}) stay forbidden.
func withoutClosures(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			if k != "unevaluatedProperties" {
				out[k] = withoutClosures(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = withoutClosures(e)
		}
		return out
	}
	return v
}

// Has reports whether the bundle contains the schema file rel (below v2/).
func (s *Set) Has(rel string) bool { return s.docs[schema.BaseURI+rel] != nil }

// Count returns the number of schema files (strict flavor).
func (s *Set) Count() int { return len(s.docs) / 2 }

// Entry returns the URL of an entry schema: the topic form rule ("" = the
// topic-agnostic message schema) or the bare predefined MetaData.
func Entry(rule string, bareMetaData, tolerant bool) string {
	base := schema.BaseURI
	if tolerant {
		base = schema.TolerantBaseURI
	}
	switch {
	case bareMetaData:
		return base + "metadata.schema.json#/$defs/ResourceMetaData"
	case rule == "":
		return base + "message.schema.json"
	}
	return base + "topics/" + rule + ".schema.json"
}

func (s *Set) compile(id string) (*jsonschema.Schema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sch, ok := s.compiled[id]; ok {
		return sch, nil
	}
	sch, err := s.compiler.Compile(id)
	if err != nil {
		return nil, err
	}
	s.compiled[id] = sch
	return sch, nil
}

type leaf struct {
	locs []string
	err  *jsonschema.ValidationError
}

func (l leaf) key() string {
	return finding.Pointer(l.err.InstanceLocation) + "|" + l.err.SchemaURL + "|" + strings.Join(l.err.ErrorKind.KeywordPath(), "/")
}

func (s *Set) leaves(id string, inst any) ([]leaf, error) {
	sch, err := s.compile(id)
	if err != nil {
		return nil, err
	}
	err = sch.Validate(inst)
	if err == nil {
		return nil, nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return nil, err
	}
	return collect(ve, nil), nil
}

func collect(e *jsonschema.ValidationError, path []string) []leaf {
	locs := append(slices.Clone(path), e.SchemaURL)
	if len(e.Causes) == 0 {
		return []leaf{{locs: locs, err: e}}
	}
	var out []leaf
	for _, c := range e.Causes {
		out = append(out, collect(c, locs)...)
	}
	return out
}

// Validate validates a parsed instance (numbers as json.Number) against the
// entry schema id and returns the schema findings and the R-4 findings.
func (s *Set) Validate(id string, inst any) ([]finding.Finding, error) {
	ls, err := s.leaves(id, inst)
	if err != nil {
		return nil, err
	}
	var out []finding.Finding
	for _, l := range ls {
		reqs := s.reqs(l)
		ptr := finding.Pointer(l.err.InstanceLocation)
		msg := fmt.Sprintf("%s (%s)", l.err.ErrorKind.LocalizedString(printer), strings.Join(l.err.ErrorKind.KeywordPath(), "/"))
		if len(reqs) == 0 {
			out = append(out, finding.Finding{Layer: finding.LayerSchema, Pointer: ptr, Detail: msg})
		}
		for _, r := range reqs {
			out = append(out, finding.Finding{Req: r, Layer: finding.LayerSchema, Pointer: ptr, Detail: msg})
		}
	}
	r4, err := s.probe(id, inst, ls)
	if err != nil {
		return nil, err
	}
	return append(out, r4...), nil
}

var reqID = regexp.MustCompile(`^REQ-[A-Z]+(?:-[A-Z]+)*-[0-9]{3}$`)

// reqs returns the REQ ids of the deepest "$comment" with leading REQ ids
// on the evaluation path of a leaf, including its keyword location.
func (s *Set) reqs(l leaf) []string {
	locs := slices.Clone(l.locs)
	if kp := l.err.ErrorKind.KeywordPath(); len(kp) > 0 {
		locs = append(locs, l.err.SchemaURL+"/"+strings.Join(kp, "/"))
	}
	var cur []string
	for _, loc := range locs {
		base, frag, _ := strings.Cut(loc, "#")
		node := s.docs[base]
		if node == nil {
			continue
		}
		frag, _ = url.PathUnescape(frag)
		var tokens []string
		if frag != "" {
			tokens = strings.Split(strings.TrimPrefix(frag, "/"), "/")
		}
		for i := 0; node != nil; i++ {
			if m, ok := node.(map[string]any); ok {
				if c, ok := m["$comment"].(string); ok {
					if ids := leadingReqs(c); len(ids) > 0 {
						cur = ids
					}
				}
			}
			if i == len(tokens) {
				break
			}
			node = step(node, tokens[i])
		}
	}
	return cur
}

func leadingReqs(comment string) []string {
	var ids []string
	for _, f := range strings.Fields(comment) {
		f = strings.TrimRight(f, ",;:")
		if !reqID.MatchString(f) {
			break
		}
		ids = append(ids, f)
	}
	return ids
}

func step(node any, token string) any {
	token = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
	switch t := node.(type) {
	case map[string]any:
		return t[token]
	case []any:
		i, err := strconv.Atoi(token)
		if err != nil || i < 0 || i >= len(t) {
			return nil
		}
		return t[i]
	}
	return nil
}
