// Package vectors loads the language-neutral test vectors in oec/spec/vectors
// and recomputes every expected result with the reference checks: the topic
// grammar (ABNF interpreter + generated regex), the Identification Link rules,
// the dnp-encoding, the Oi4Identifier construction and the payload envelope.
package vectors

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/il"
	"github.com/OI4/oec/spec/tools/internal/schema"
	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/yamlstrict"
)

// Paths inside the spec directory.
const (
	Dir    = "vectors"
	Schema = "vectors/vectors.schema.json"
)

// Origin records where a vector comes from.
type Origin struct {
	Kind     string   `json:"kind"`
	Ref      string   `json:"ref,omitempty"`
	Original string   `json:"original,omitempty"`
	SF       []string `json:"sf,omitempty"`
}

// Parse is the expected decomposition of a valid topic.
type Parse struct {
	Rule         string   `json:"rule"`
	ServiceType  string   `json:"service_type"`
	AppID        string   `json:"app_id"`
	AppIDIL      string   `json:"app_id_il"`
	Method       string   `json:"method"`
	Resource     string   `json:"resource"`
	Source       string   `json:"source,omitempty"`
	SourceIL     string   `json:"source_il,omitempty"`
	Filter       string   `json:"filter,omitempty"`
	FilterValues []string `json:"filter_values,omitempty"`
}

// Vector is one test vector of any kind.
type Vector struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Expected string   `json:"expected"`
	Reqs     []string `json:"reqs,omitempty"`
	Violates []string `json:"violates,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Origin   Origin   `json:"origin"`
	Status   string   `json:"status"`
	Notes    string   `json:"notes,omitempty"`

	Topic    string   `json:"topic,omitempty"`
	Parse    *Parse   `json:"parse,omitempty"`
	Fails    string   `json:"fails,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	IL         *string  `json:"il,omitempty"`
	Encoded    string   `json:"encoded,omitempty"`
	Violations []string `json:"violations,omitempty"`

	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`

	ManufacturerURI string `json:"manufacturer_uri,omitempty"`
	Model           string `json:"model,omitempty"`
	ProductCode     string `json:"product_code,omitempty"`
	SerialNumber    string `json:"serial_number,omitempty"`
	Oi4Identifier   string `json:"oi4identifier,omitempty"`

	Payload  string `json:"payload,omitempty"`
	Object   string `json:"object,omitempty"` // payload is a bare object of this type, no message
	Mode     string `json:"mode,omitempty"`   // validation mode: strict (default) or tolerant
	Original string `json:"original,omitempty"`
	Edits    []Edit `json:"edits,omitempty"`
}

// Edit records one change of a payload vector against its guideline
// example (ADR-0018 amendments C-013 and C-023).
type Edit struct {
	Target string   `json:"target"` // "topic" or a JSON Pointer
	Kind   string   `json:"kind"`
	Reason string   `json:"reason"`
	SF     []string `json:"sf,omitempty"`
}

// File is one vector file.
type File struct {
	Kind        string   `json:"kind"`
	Baseline    string   `json:"baseline"`
	Description string   `json:"description"`
	Vectors     []Vector `json:"vectors"`
	Path        string   `json:"-"`
}

// Load reads and schema-validates all vector files below root/vectors.
func Load(root string) ([]File, error) {
	val, err := schema.Compile(filepath.Join(root, filepath.FromSlash(Schema)))
	if err != nil {
		return nil, err
	}
	var paths []string
	err = filepath.WalkDir(filepath.Join(root, Dir), func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() && strings.HasSuffix(p, ".yaml") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var files []File
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		v, err := yamlstrict.LoadFile(p)
		if err != nil {
			return nil, err
		}
		if err := val.Validate(v); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		var f File
		if err := yamlstrict.Decode(v, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		f.Path = rel
		files = append(files, f)
	}
	return files, nil
}

// Options provide the cross-references for Check.
type Options struct {
	Baseline string
	// Requirement ids of the catalog; nil skips the check.
	Reqs map[string]bool
	// Normalized guideline text per section ref, including Appendix B; nil
	// skips the origin check.
	Sections map[string]string
}

// Check recomputes all vectors and returns every mismatch.
func Check(files []File, ts *topic.Spec, opt Options) []error {
	var errs []error
	ids := map[string]string{}
	for _, f := range files {
		if f.Baseline != opt.Baseline {
			errs = append(errs, fmt.Errorf("%s: baseline %s differs from the catalog baseline %s", f.Path, f.Baseline, opt.Baseline))
		}
		for _, v := range f.Vectors {
			at := f.Path + ": " + v.ID
			add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(at+": "+format, a...)) }
			if prev, dup := ids[v.ID]; dup {
				add("duplicate id (also in %s)", prev)
			}
			ids[v.ID] = f.Path
			if opt.Reqs != nil {
				for _, r := range append(append([]string(nil), v.Reqs...), v.Violates...) {
					if !opt.Reqs[r] {
						add("requirement %s does not exist", r)
					}
				}
			}
			if f.Kind == "payload" {
				if err := checkPayload(ts, v, opt.Sections, add); err != nil {
					return append(errs, fmt.Errorf("%s: %w", at, err))
				}
				continue
			}
			checkOrigin(f.Kind, v, opt.Sections, add)
			switch f.Kind {
			case "topic":
				if err := checkTopic(ts, v, add); err != nil {
					return append(errs, fmt.Errorf("%s: %w", at, err))
				}
			case "identification-link":
				checkIL(v, add)
			case "dnp-encoding":
				if got := il.DnpEncode(v.Input); got != v.Output {
					add("dnp-encoding of %q is %q, vector says %q", v.Input, got, v.Output)
				}
			case "oi4identifier":
				checkOi4Identifier(v, add)
			}
		}
	}
	return errs
}

// subject is the value a guideline origin must contain verbatim.
func subject(kind string, v Vector) string {
	switch kind {
	case "topic":
		return v.Topic
	case "identification-link":
		if v.IL != nil {
			return *v.IL
		}
		return v.Encoded
	case "dnp-encoding":
		return v.Output
	case "oi4identifier":
		return v.Oi4Identifier
	}
	return ""
}

func checkOrigin(kind string, v Vector, sections map[string]string, add func(string, ...any)) {
	if v.Origin.Kind != "guideline" {
		return
	}
	text := subject(kind, v)
	if v.Origin.Original != "" {
		if v.Origin.Original == text {
			add("origin.original equals the vector value – drop it")
		}
		text = v.Origin.Original
	}
	if sections == nil || text == "" {
		return
	}
	sec, ok := sections[v.Origin.Ref]
	if !ok {
		add("origin %s has no guideline file", v.Origin.Ref)
		return
	}
	if !strings.Contains(sec, guideline.NormalizeForCompare(text)) {
		add("%q not found verbatim in %s", text, v.Origin.Ref)
	}
}

func checkTopic(ts *topic.Spec, v Vector, add func(string, ...any)) error {
	res, err := ts.Analyze(v.Topic)
	if err != nil {
		return err
	}
	if res.Valid != (v.Expected == "valid") {
		add("topic is %s (%s %s), vector expects %s", verdict(res.Valid), res.Failure, res.Detail, v.Expected)
		return nil
	}
	if !res.Valid {
		if res.Failure != v.Fails {
			add("topic fails the %s check (%s), vector says %s", res.Failure, res.Detail, v.Fails)
		}
		return nil
	}
	p := res.Parsed
	got := Parse{
		Rule: res.Rule, ServiceType: p.ServiceType, AppID: p.AppID, AppIDIL: p.AppIDIL,
		Method: p.Method, Resource: p.Resource, Source: p.Source, SourceIL: p.SourceIL,
		Filter: p.Filter, FilterValues: p.FilterValues,
	}
	if !reflect.DeepEqual(got, *v.Parse) {
		add("parse result differs:\n    computed %+v\n    vector   %+v", got, *v.Parse)
	}
	if !equalSet(res.Warnings, v.Warnings) {
		add("warnings %v, vector says %v", res.Warnings, v.Warnings)
	}
	return nil
}

func checkIL(v Vector, add func(string, ...any)) {
	var got []string
	switch {
	case v.IL != nil:
		for _, x := range il.Validate(*v.IL) {
			got = append(got, x.Code)
		}
		if v.Encoded != "" {
			if enc := il.Encode(*v.IL); enc != v.Encoded {
				add("encoding of %q is %q, vector says %q", *v.IL, enc, v.Encoded)
			}
		}
	default:
		_, viol, err := il.CheckTopicElement(v.Encoded)
		if err != nil {
			got = []string{"encoding"}
		}
		for _, x := range viol {
			got = append(got, x.Code)
		}
	}
	if (len(got) == 0) != (v.Expected == "valid") || !equalSet(got, v.Violations) {
		add("violations %v, vector says %s %v", got, v.Expected, v.Violations)
	}
}

func checkOi4Identifier(v Vector, add func(string, ...any)) {
	id, viol := il.Oi4Identifier(v.ManufacturerURI, v.Model, v.ProductCode, v.SerialNumber)
	var got []string
	for _, x := range viol {
		got = append(got, x.Code)
	}
	if (len(got) == 0) != (v.Expected == "valid") || !equalSet(got, v.Violations) {
		add("violations %v, vector says %s %v", got, v.Expected, v.Violations)
		return
	}
	if v.Expected == "valid" && id != v.Oi4Identifier {
		add("Oi4Identifier is %q, vector says %q", id, v.Oi4Identifier)
	}
}

func verdict(valid bool) string {
	if valid {
		return "valid"
	}
	return "invalid"
}

func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}

// Count returns the number of vectors per kind.
func Count(files []File) map[string]int {
	out := map[string]int{}
	for _, f := range files {
		out[f.Kind] += len(f.Vectors)
	}
	return out
}
