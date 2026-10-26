// Command schematest is the test harness of ADR-0019. It builds the schema
// bundle (strict source + generated tolerant flavor), checks the REQ tracing
// of the "$comment"s against the catalog of oec/spec, routes the payload
// vectors of oec/spec by their topic to an entry schema (topic form, option
// B) and the bare DataSetMetaDataType vectors to the predefined resource
// MetaData, validates every valid vector with a topic against the
// topic-agnostic message schema (option A), routes the broker-json vectors
// to the platform schema of broker.json (RIG 4.1.1),
// validates them with santhosh-tekuri/jsonschema (draft 2020-12), maps every
// error to the REQ ids in the nearest "$comment" of the evaluation path and
// compares the result with the vector (expected, violates). It writes the
// bundle and the validation cases for a second, independent validator to
// -dist.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	schema "github.com/OI4/oec/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

func main() {
	spec := flag.String("spec", "../../spec", "path of oec/spec")
	expect := flag.String("expect", "../tests/rule-layer.yaml", "expectation list of the rule-layer cases")
	dist := flag.String("dist", "../dist", "output directory of the bundle and the cases")
	verbose := flag.Bool("v", false, "print the errors of every rejected vector")
	release := flag.String("release", "../release", "directory of the release manifests")
	changelog := flag.String("changelog", "../CHANGELOG.md", "changelog of the schema bundle")
	freeze := flag.Bool("write-release", false, "write the manifest release/<schema.Version>.json of the current tree and exit")
	flag.Parse()
	var err error
	if *freeze {
		err = runWriteRelease(*release)
	} else {
		err = run(*spec, *expect, *dist, *release, *changelog, *verbose)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "schematest:", err)
		os.Exit(1)
	}
}

type vector struct {
	ID       string   `yaml:"id"`
	Topic    string   `yaml:"topic"`
	Payload  string   `yaml:"payload"`
	Expected string   `yaml:"expected"`
	Mode     string   `yaml:"mode"`
	Object   string   `yaml:"object"`
	Violates []string `yaml:"violates"`
	Content  string   `yaml:"content"` // broker-json vectors
}

// brokerSchema is the schema of the platform file broker.json (RIG 4.1.1),
// relative to schema.BaseURI.
const brokerSchema = "platform/broker.schema.json"

// expectation partitions the violated REQs of an invalid vector whose
// violation the entry schema cannot see completely (ADR-0019 §8): Schema are
// the REQs the entry schema reports, Rule the REQs only the rule layer of
// D7 validate detects.
type expectation struct {
	Vector string   `yaml:"vector"`
	Schema []string `yaml:"schema"`
	Rule   []string `yaml:"rule"`
	Rules  []string `yaml:"rules"`
	Reason string   `yaml:"reason"`
}

type pattern struct {
	Rule    string `json:"rule"`
	Pattern string `json:"pattern"`
	re      *regexp.Regexp
}

// testCase is one validation of the cases file for the second validator.
// Topic, Mode, Object, File and Reqs describe the case for the oracle
// cross-check (spectool crosscheck, ADR-0019 amendment C-041); the
// validators ignore them.
type testCase struct {
	ID       string          `json:"id"`
	Schema   string          `json:"schema"`
	Instance json.RawMessage `json:"instance"`
	Valid    bool            `json:"valid"`
	Topic    string          `json:"topic,omitempty"`
	Mode     string          `json:"mode,omitempty"`   // "tolerant" for the tolerant flavor
	Object   string          `json:"object,omitempty"` // DataSetMetaDataType for a bare MetaData
	File     string          `json:"file,omitempty"`   // broker.json for the platform file
	Reqs     []string        `json:"reqs,omitempty"`   // REQs the schema reports for an invalid case
}

// tolerantMode is the Mode of a case of the tolerant flavor.
func tolerantMode(m string) string {
	if m == "tolerant" {
		return m
	}
	return ""
}

type result struct {
	valid bool
	reqs  []string
	msgs  []string
}

func runWriteRelease(dir string) error {
	docs, err := bundle()
	if err != nil {
		return err
	}
	if err := writeRelease(docs, dir); err != nil {
		return err
	}
	fmt.Printf("wrote %s/%s.json\n", dir, schema.Version)
	return nil
}

func run(specDir, expectFile, distDir, releaseDir, changelog string, verbose bool) error {
	docs, err := bundle()
	if err != nil {
		return err
	}
	specBaseline, err := loadBaseline(filepath.Join(specDir, "requirements", "areas.yaml"))
	if err != nil {
		return err
	}
	problems, err := checkRelease(docs, releaseDir, changelog, specBaseline)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "FAIL", p)
		}
		return fmt.Errorf("%d release problems (ADR-0019 amendment C-041)", len(problems))
	}
	fmt.Printf("release: schemas %s (baseline %s) frozen, no released $id or $defs name removed\n", schema.Version, schema.Baseline)
	if err := writeBundle(distDir, docs); err != nil {
		return err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	for id, doc := range docs {
		if err := c.AddResource(id, doc); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	for id := range docs { // every file compiles (meta-schema check included)
		if _, err := c.Compile(id); err != nil {
			return err
		}
	}
	reqs, err := loadRequirements(filepath.Join(specDir, "requirements"))
	if err != nil {
		return err
	}
	if problems := checkComments(docs, reqs); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "FAIL", p)
		}
		return fmt.Errorf("%d $comment problems", len(problems))
	}
	patterns, err := loadPatterns(filepath.Join(specDir, "topic", "topic-patterns.json"))
	if err != nil {
		return err
	}
	vectors, err := loadVectors(filepath.Join(specDir, "vectors", "payload"))
	if err != nil {
		return err
	}
	expects, err := loadExpectations(expectFile, vectors)
	if err != nil {
		return err
	}

	var cases []testCase
	var failures []string
	routed, agnostic, skipped := 0, 0, 0
	fmt.Println("| Vector | Topic form | Mode | Expected (oracle) | Entry schema (B) | Generic schema (A) |")
	fmt.Println("|--------|------------|------|-------------------|------------------|--------------------|")
	for _, v := range vectors {
		base := schema.BaseURI
		mode := "strict"
		if v.Mode == "tolerant" {
			base, mode = schema.TolerantBaseURI, "tolerant"
		}
		generic := base + "message.schema.json"
		rule := route(patterns, v.Topic)
		entry := base + "topics/" + rule + ".schema.json"
		if v.Object == "DataSetMetaDataType" {
			// a bare MetaData is the predefined MetaData of a resource (MG B2)
			rule = "(bare) ResourceMetaData"
			entry = base + "metadata.schema.json#/$defs/ResourceMetaData"
			generic = base + "metadata.schema.json#/$defs/DataSetMetaData"
		}
		if rule == "" {
			if v.Topic == "" || v.Expected != "valid" {
				skipped++
				continue
			}
			// option A: every valid message passes the topic-agnostic schema
			agnostic++
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(v.Payload))
			if err != nil {
				return fmt.Errorf("%s: %w", v.ID, err)
			}
			a, err := validate(c, docs, generic, inst)
			if err != nil {
				return err
			}
			cases = append(cases, testCase{ID: v.ID + "/message", Schema: generic, Instance: compact(v.Payload), Valid: a.valid,
				Topic: v.Topic, Mode: tolerantMode(v.Mode), Reqs: a.reqs})
			if !a.valid {
				failures = append(failures, fmt.Sprintf("%s: valid vector rejected by %s: %v", v.ID, generic, a.msgs))
			}
			continue
		}
		routed++
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(v.Payload))
		if err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
		b, err := validate(c, docs, entry, inst)
		if err != nil {
			return err
		}
		a, err := validate(c, docs, generic, inst)
		if err != nil {
			return err
		}
		raw := compact(v.Payload)
		cases = append(cases,
			testCase{ID: v.ID + "/entry", Schema: entry, Instance: raw, Valid: b.valid,
				Topic: v.Topic, Mode: tolerantMode(v.Mode), Object: v.Object, Reqs: b.reqs},
			testCase{ID: v.ID + "/message", Schema: generic, Instance: raw, Valid: a.valid,
				Topic: v.Topic, Mode: tolerantMode(v.Mode), Object: v.Object, Reqs: a.reqs})

		want := v.Expected
		if want == "invalid" {
			want = "invalid " + strings.Join(v.Violates, ", ")
		}
		fmt.Printf("| %s | %s | %s | %s | %s | %s |\n", v.ID, rule, mode, want, verdict(b), verdict(a))
		if verbose {
			for _, m := range append(b.msgs, a.msgs...) {
				fmt.Println("    ", m)
			}
		}
		switch {
		case v.Expected == "valid" && !b.valid:
			failures = append(failures, fmt.Sprintf("%s: valid vector rejected by %s: %v", v.ID, entry, b.msgs))
		case v.Expected == "valid" && !a.valid:
			failures = append(failures, fmt.Sprintf("%s: valid vector rejected by %s: %v", v.ID, generic, a.msgs))
		case v.Expected == "invalid":
			// right reason: the entry schema reports exactly the REQs not
			// assigned to the rule layer by the expectation list
			want := slices.Clone(v.Violates)
			if e, ok := expects[v.ID]; ok {
				want = slices.Clone(e.Schema)
			}
			sort.Strings(want)
			switch {
			case len(want) == 0 && !b.valid:
				failures = append(failures, fmt.Sprintf("%s: rule-layer vector rejected by %s: %v", v.ID, entry, b.msgs))
			case len(want) > 0 && b.valid:
				failures = append(failures, fmt.Sprintf("%s: invalid vector accepted by %s", v.ID, entry))
			case len(want) > 0 && !slices.Equal(b.reqs, want):
				failures = append(failures, fmt.Sprintf("%s: %s reports %v, want %v: %v", v.ID, entry, b.reqs, want, b.msgs))
			}
		}
	}
	fmt.Printf("\n%d payload vectors routed to an entry schema (bare MetaData included), %d further valid vectors checked with the message schema only, %d without schema check yet (invalid without entry schema, or no topic).\n", routed, agnostic, skipped)
	brokers, err := loadVectors(filepath.Join(specDir, "vectors", "platform"))
	if err != nil {
		return err
	}
	bc, bf, err := runBroker(c, docs, brokers, verbose)
	if err != nil {
		return err
	}
	cases, failures = append(cases, bc...), append(failures, bf...)
	pc, pf, err := runProbes(c, docs, append(slices.Clone(vectors), brokers...))
	if err != nil {
		return err
	}
	cases, failures = append(cases, pc...), append(failures, pf...)
	if err := writeJSON(filepath.Join(distDir, "cases.json"), cases); err != nil {
		return err
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "FAIL", f)
		}
		return fmt.Errorf("%d failures", len(failures))
	}
	fmt.Println("ok")
	return nil
}

// runBroker validates the content of every broker-json vector against the
// platform schema of broker.json. broker.json has no closure (SF-100 (a),
// proposed), so the tolerant flavor equals the strict one and is not
// checked separately. An invalid vector must be rejected with exactly its
// violated REQs; there is no rule layer for this file.
func runBroker(c *jsonschema.Compiler, docs map[string]any, vectors []vector, verbose bool) ([]testCase, []string, error) {
	var cases []testCase
	var failures []string
	entry := schema.BaseURI + brokerSchema
	fmt.Println("\n| Vector | File | Expected (oracle) | Schema |")
	fmt.Println("|--------|------|-------------------|--------|")
	for i := range vectors {
		v := &vectors[i]
		v.Payload = v.Content // base of the broker.json probes
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(v.Content))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", v.ID, err)
		}
		r, err := validate(c, docs, entry, inst)
		if err != nil {
			return nil, nil, err
		}
		cases = append(cases, testCase{ID: v.ID + "/file", Schema: entry, Instance: compact(v.Content), Valid: r.valid,
			File: "broker.json", Reqs: r.reqs})
		want := v.Expected
		if want == "invalid" {
			want = "invalid " + strings.Join(v.Violates, ", ")
		}
		fmt.Printf("| %s | broker.json | %s | %s |\n", v.ID, want, verdict(r))
		if verbose {
			for _, m := range r.msgs {
				fmt.Println("    ", m)
			}
		}
		violates := slices.Clone(v.Violates)
		sort.Strings(violates)
		switch {
		case v.Expected == "valid" && !r.valid:
			failures = append(failures, fmt.Sprintf("%s: valid vector rejected by %s: %v", v.ID, entry, r.msgs))
		case v.Expected == "invalid" && r.valid:
			failures = append(failures, fmt.Sprintf("%s: invalid vector accepted by %s", v.ID, entry))
		case v.Expected == "invalid" && !slices.Equal(r.reqs, violates):
			failures = append(failures, fmt.Sprintf("%s: %s reports %v, want %v: %v", v.ID, entry, r.reqs, violates, r.msgs))
		}
	}
	fmt.Printf("\n%d broker-json vectors validated against %s.\n", len(vectors), brokerSchema)
	return cases, failures, nil
}

func verdict(r result) string {
	if r.valid {
		return "valid"
	}
	return "invalid " + strings.Join(r.reqs, ", ")
}

// bundle returns the strict source files and the generated tolerant flavor,
// keyed by $id.
func bundle() (map[string]any, error) {
	docs := map[string]any{}
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
		m, _ := doc.(map[string]any)
		id, _ := m["$id"].(string)
		if want := schema.BaseURI + strings.TrimPrefix(p, "v2/"); id != want {
			return fmt.Errorf("%s: $id %q, want %q", p, id, want)
		}
		if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			return fmt.Errorf("%s: $schema is not draft 2020-12", p)
		}
		docs[id] = doc
		tol, _ := tolerant(doc).(map[string]any)
		tol["$id"] = schema.TolerantBaseURI + strings.TrimPrefix(id, schema.BaseURI)
		docs[tol["$id"].(string)] = tol
		return nil
	})
	return docs, err
}

// tolerant derives the tolerant flavor (unknown-key tolerance of
// REQ-MSG-046, like the oracle mode "tolerant"): a deep copy without the
// closures. "unevaluatedProperties" is used for closures only (ADR-0019 §3).
// Keys that are explicitly forbidden ("not": {}) stay forbidden.
func tolerant(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			if k == "unevaluatedProperties" {
				continue
			}
			out[k] = tolerant(e)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = tolerant(e)
		}
		return out
	}
	return v
}

// writeBundle writes the bundle below dir/<host>/; files of an earlier run
// are removed first, so that no stale $id reaches the second validator.
func writeBundle(dir string, docs map[string]any) error {
	u, err := url.Parse(schema.BaseURI)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dir, u.Host)); err != nil {
		return err
	}
	for id, doc := range docs {
		p := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(id, "https://")))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := writeJSON(p, doc); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(p string, v any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0o644)
}

func compact(s string) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return json.RawMessage(s)
	}
	return buf.Bytes()
}

func loadPatterns(p string) ([]pattern, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var f struct {
		Patterns []pattern `json:"patterns"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	var out []pattern
	for _, pt := range f.Patterns {
		if _, err := fs.Stat(schema.FS, "v2/topics/"+pt.Rule+".schema.json"); err != nil {
			continue // no entry schema for this rule
		}
		pt.re, err = regexp.Compile(pt.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", pt.Rule, err)
		}
		out = append(out, pt)
	}
	return out, nil
}

// route returns the topic form whose entry schema validates the payload of
// a message on topic ("" = no entry schema).
func route(patterns []pattern, topic string) string {
	for _, p := range patterns {
		if p.re.MatchString(topic) {
			return p.Rule
		}
	}
	return ""
}

func loadExpectations(p string, vectors []vector) (map[string]expectation, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Expectations []expectation `yaml:"expectations"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	byID := map[string]vector{}
	for _, v := range vectors {
		byID[v.ID] = v
	}
	out := map[string]expectation{}
	for _, e := range doc.Expectations {
		v, ok := byID[e.Vector]
		switch {
		case !ok:
			return nil, fmt.Errorf("%s: unknown vector %s", p, e.Vector)
		case v.Expected != "invalid":
			return nil, fmt.Errorf("%s: %s is not an invalid vector", p, e.Vector)
		case len(e.Rule) == 0 || len(e.Rules) == 0 || e.Reason == "":
			return nil, fmt.Errorf("%s: %s needs rule REQs, rules and a reason", p, e.Vector)
		}
		got := append(slices.Clone(e.Schema), e.Rule...)
		want := slices.Clone(v.Violates)
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			return nil, fmt.Errorf("%s: %s: schema + rule = %v, vector violates %v", p, e.Vector, got, want)
		}
		if _, dup := out[e.Vector]; dup {
			return nil, fmt.Errorf("%s: %s listed twice", p, e.Vector)
		}
		out[e.Vector] = e
	}
	return out, nil
}

// loadBaseline returns the guideline baseline of the spec catalog.
func loadBaseline(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	var a struct {
		Baseline string `yaml:"baseline"`
	}
	if err := yaml.Unmarshal(data, &a); err != nil {
		return "", fmt.Errorf("%s: %w", p, err)
	}
	if a.Baseline == "" {
		return "", fmt.Errorf("%s: no baseline", p)
	}
	return a.Baseline, nil
}

func loadVectors(dir string) ([]vector, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []vector
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Vectors []vector `yaml:"vectors"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, doc.Vectors...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func validate(c *jsonschema.Compiler, docs map[string]any, id string, inst any) (result, error) {
	sch, err := c.Compile(id)
	if err != nil {
		return result{}, err
	}
	err = sch.Validate(inst)
	if err == nil {
		return result{valid: true}, nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return result{}, err
	}
	var r result
	seen := map[string]bool{}
	for _, l := range leaves(ve, nil) {
		reqs := l.reqs(docs)
		for _, q := range reqs {
			if !seen[q] {
				seen[q] = true
				r.reqs = append(r.reqs, q)
			}
		}
		r.msgs = append(r.msgs, fmt.Sprintf("%s %s: %s %v", "/"+strings.Join(l.err.InstanceLocation, "/"), strings.Join(l.err.ErrorKind.KeywordPath(), "/"), l.err.Error(), reqs))
	}
	sort.Strings(r.reqs)
	return r, nil
}

type leaf struct {
	locs []string // schema locations on the evaluation path, root first
	err  *jsonschema.ValidationError
}

func leaves(e *jsonschema.ValidationError, path []string) []leaf {
	locs := append(slices.Clone(path), e.SchemaURL)
	if len(e.Causes) == 0 {
		return []leaf{{locs: locs, err: e}}
	}
	var out []leaf
	for _, c := range e.Causes {
		out = append(out, leaves(c, locs)...)
	}
	return out
}

var reqID = regexp.MustCompile(`^REQ-[A-Z]+(?:-[A-Z]+)*-[0-9]{3}$`)

// reqs maps an error to the REQ ids of the deepest "$comment" on its
// evaluation path. Only the REQ ids at the start of a comment count. A
// closure is a schema with its own comment ({"$comment": "REQ-MSG-001 …",
// "not": {}}), so an undefined key maps to the REQ of its closure.
func (l leaf) reqs(docs map[string]any) []string {
	kp := l.err.ErrorKind.KeywordPath()
	locs := slices.Clone(l.locs)
	if len(kp) > 0 {
		locs = append(locs, l.err.SchemaURL+"/"+strings.Join(kp, "/"))
	}
	var cur []string
	for _, loc := range locs {
		base, frag, _ := strings.Cut(loc, "#")
		doc := docs[base]
		if doc == nil {
			continue
		}
		frag, _ = url.PathUnescape(frag)
		tokens := strings.Split(strings.TrimPrefix(frag, "/"), "/")
		if frag == "" {
			tokens = nil
		}
		node := doc
		for i := 0; i <= len(tokens); i++ {
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
			node = child(node, tokens[i])
			if node == nil {
				break
			}
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

func child(node any, token string) any {
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
