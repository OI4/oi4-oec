package gha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The renderer writes a small, fixed subset of YAML: block mappings and
// sequences, plain scalars where they are unambiguous, double-quoted
// scalars (JSON strings, valid YAML) otherwise, literal blocks for
// multi-line strings and trailing comments.

type node interface{}

type pair struct {
	k string
	v node
}

type (
	mapping []pair
	seq     []node
	null    struct{}
	block   string
	scalar  struct {
		s       string
		comment string
		raw     bool // a bool or an integer, written as is
	}
)

func str(s string) scalar { return scalar{s: s} }

func boolean(v bool) scalar { return scalar{s: strconv.FormatBool(v), raw: true} }

func integer(v int) scalar { return scalar{s: strconv.Itoa(v), raw: true} }

func kvs(list []KV) mapping {
	m := make(mapping, 0, len(list))
	for _, kv := range list {
		m = append(m, pair{kv.Key, text(kv.Value)})
	}
	return m
}

// text is a scalar, or a literal block if s has more than one line.
func text(s string) node {
	if strings.Contains(s, "\n") {
		return block(s)
	}
	return str(s)
}

// Render returns the YAML text of w.
func Render(w Workflow) ([]byte, error) {
	var b bytes.Buffer
	for _, c := range w.Comment {
		if c == "" {
			b.WriteString("#\n")
		} else {
			b.WriteString("# " + c + "\n")
		}
	}
	r := renderer{b: &b}
	r.mapping(w.node(), 0)
	if r.err != nil {
		return nil, r.err
	}
	return b.Bytes(), nil
}

func (w Workflow) node() mapping {
	var on mapping
	var push mapping
	if len(w.PushBranches) > 0 {
		push = append(push, pair{"branches", strs(w.PushBranches)})
	}
	if len(w.PushTags) > 0 {
		push = append(push, pair{"tags", strs(w.PushTags)})
	}
	if len(push) > 0 {
		on = append(on, pair{"push", push})
	}
	if w.PullRequest {
		on = append(on, pair{"pull_request", null{}})
	}
	if w.Dispatch {
		on = append(on, pair{"workflow_dispatch", null{}})
	}
	m := mapping{
		{"name", str(w.Name)},
		{"on", on},
		{"permissions", kvs(w.Permissions)},
		{"concurrency", mapping{
			{"group", str(w.Concurrency.Group)},
			{"cancel-in-progress", boolean(w.Concurrency.CancelInProgress)},
		}},
		{"defaults", mapping{{"run", mapping{{"shell", str("bash")}}}}},
	}
	if len(w.Env) > 0 {
		m = append(m, pair{"env", kvs(w.Env)})
	}
	jobs := mapping{}
	for _, j := range w.Jobs {
		jobs = append(jobs, pair{j.ID, j.node()})
	}
	return append(m, pair{"jobs", jobs})
}

func strs(list []string) seq {
	s := seq{}
	for _, v := range list {
		s = append(s, str(v))
	}
	return s
}

func (j Job) node() mapping {
	m := mapping{{"name", str(j.Name)}}
	if len(j.Needs) > 0 {
		m = append(m, pair{"needs", strs(j.Needs)})
	}
	m = append(m,
		pair{"runs-on", str(j.RunsOn)},
		pair{"timeout-minutes", integer(j.TimeoutMinutes)},
	)
	if len(j.Matrix) > 0 {
		include := seq{}
		for _, e := range j.Matrix {
			include = append(include, kvs(e))
		}
		m = append(m, pair{"strategy", mapping{
			{"fail-fast", boolean(j.FailFast)},
			{"matrix", mapping{{"include", include}}},
		}})
	}
	if len(j.Env) > 0 {
		m = append(m, pair{"env", kvs(j.Env)})
	}
	steps := seq{}
	for _, s := range j.Steps {
		steps = append(steps, s.node())
	}
	return append(m, pair{"steps", steps})
}

func (s Step) node() mapping {
	m := mapping{{"name", str(s.Name)}}
	if s.If != "" {
		m = append(m, pair{"if", str(s.If)})
	}
	if !s.Uses.IsZero() {
		m = append(m, pair{"uses", scalar{s: s.Uses.Ref(), comment: s.Uses.Tag}})
	}
	if len(s.With) > 0 {
		m = append(m, pair{"with", kvs(s.With)})
	}
	if len(s.Env) > 0 {
		m = append(m, pair{"env", kvs(s.Env)})
	}
	if s.Run != "" {
		m = append(m, pair{"run", text(s.Run)})
	}
	return m
}

type renderer struct {
	b   *bytes.Buffer
	err error
}

func (r *renderer) fail(format string, a ...any) {
	if r.err == nil {
		r.err = fmt.Errorf(format, a...)
	}
}

func (r *renderer) indent(n int) { r.b.WriteString(strings.Repeat(" ", n)) }

func (r *renderer) mapping(m mapping, ind int) {
	for _, p := range m {
		r.indent(ind)
		r.entry(p, ind)
	}
}

// entry writes "key:" and the value; the indentation is already written.
func (r *renderer) entry(p pair, ind int) {
	if !plainKey(p.k) {
		r.fail("key %q is not a plain key", p.k)
	}
	r.b.WriteString(p.k + ":")
	switch v := p.v.(type) {
	case null:
		r.b.WriteString("\n")
	case mapping:
		if len(v) == 0 {
			r.b.WriteString(" {}\n")
			return
		}
		r.b.WriteString("\n")
		r.mapping(v, ind+2)
	case seq:
		if len(v) == 0 {
			r.b.WriteString(" []\n")
			return
		}
		r.b.WriteString("\n")
		r.seq(v, ind+2)
	case block:
		r.block(string(v), ind+2)
	case scalar:
		if v.raw {
			r.b.WriteString(" " + v.s)
		} else {
			r.b.WriteString(" " + Quote(v.s))
		}
		if v.comment != "" {
			r.b.WriteString(" # " + v.comment)
		}
		r.b.WriteString("\n")
	default:
		r.fail("unknown node %T", v)
	}
}

func (r *renderer) seq(s seq, ind int) {
	for _, item := range s {
		r.indent(ind)
		r.b.WriteString("- ")
		switch v := item.(type) {
		case mapping:
			if len(v) == 0 {
				r.b.WriteString("{}\n")
				continue
			}
			r.entry(v[0], ind+2)
			r.mapping(v[1:], ind+2)
		case scalar:
			r.b.WriteString(Quote(v.s) + "\n")
		default:
			r.fail("unsupported sequence item %T", v)
		}
	}
}

// block writes s as a literal block scalar with the indentation ind.
func (r *renderer) block(s string, ind int) {
	chomp := "-"
	if strings.HasSuffix(s, "\n") {
		chomp = ""
		s = strings.TrimSuffix(s, "\n")
	}
	if strings.HasSuffix(s, "\n") {
		r.fail("literal block with trailing empty lines: %q", s)
	}
	lines := strings.Split(s, "\n")
	if strings.HasPrefix(lines[0], " ") || strings.HasPrefix(lines[0], "\t") {
		r.fail("literal block starts with white space: %q", s)
	}
	r.b.WriteString(" |" + chomp + "\n")
	for _, l := range lines {
		if strings.TrimRight(l, " \t") != l {
			r.fail("literal block line with trailing white space: %q", l)
		}
		if l != "" {
			r.indent(ind)
			r.b.WriteString(l)
		}
		r.b.WriteString("\n")
	}
}

func plainKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case i > 0 && (c >= '0' && c <= '9' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// Quote returns s as a plain scalar if that reads back as the same string
// in YAML 1.1 and 1.2, otherwise as a double-quoted scalar.
func Quote(s string) string {
	if plain(s) {
		return s
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// date matches the start of a YAML 1.1 timestamp.
var date = regexp.MustCompile(`^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}`)

// reserved are the YAML 1.1 and 1.2 words of other types than string.
var reserved = map[string]bool{
	"true": true, "false": true, "yes": true, "no": true, "on": true, "off": true,
	"y": true, "n": true, "null": true, "~": true,
}

func plain(s string) bool {
	if s == "" || reserved[strings.ToLower(s)] {
		return false
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return false
	}
	if _, err := strconv.ParseInt(s, 0, 64); err == nil {
		return false
	}
	if date.MatchString(s) {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '$', c == '/':
		case c == '(':
		case i > 0 && strings.ContainsRune(".-+@=,*'|&!<>(){}[] ", c):
		default:
			return false
		}
	}
	return !strings.HasSuffix(s, " ") && !strings.Contains(s, "  ")
}
