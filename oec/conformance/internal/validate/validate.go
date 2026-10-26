// Package validate is D7 validate (ADR-0019 §5): it checks one message – a
// topic and its payload – offline against the baseline: the topic grammar
// and the Identification Links of the topic, strict JSON, the entry schema
// of the topic (R-1, R-5), the R-4 mutation probes and the rule layer
// (R-2, R-3, R-6).
package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/OI4/oec/conformance/internal/rules"
	"github.com/OI4/oec/conformance/internal/schemas"
	"github.com/OI4/oec/conformance/internal/topic"
)

// ObjectMetaData names the bare DataSetMetaDataType (the predefined
// MetaData of a resource, MG B2) as the object of a message without topic.
const ObjectMetaData = "DataSetMetaDataType"

// Validator holds the compiled baseline.
type Validator struct {
	Topics  *topic.Spec
	Schemas *schemas.Set
}

// New loads the embedded baseline.
func New() (*Validator, error) {
	ts, err := topic.Load()
	if err != nil {
		return nil, err
	}
	ss, err := schemas.Load()
	if err != nil {
		return nil, err
	}
	return &Validator{Topics: ts, Schemas: ss}, nil
}

// Baseline returns the guideline revision of the embedded data.
func (v *Validator) Baseline() string { return v.Topics.Baseline }

// Message is one message to validate.
type Message struct {
	Topic    string          `json:"topic,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Role     string          `json:"role,omitempty"`     // birth, close or will (R-6)
	Object   string          `json:"object,omitempty"`   // DataSetMetaDataType: a bare object without topic
	Tolerant bool            `json:"tolerant,omitempty"` // tolerant receiver flavor (ADR-0019 §6)
}

// Result is the verdict on one message.
type Result struct {
	Valid    bool              `json:"valid"`
	Reqs     []string          `json:"reqs,omitempty"`
	Findings []finding.Finding `json:"findings,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Topic    *topic.Parsed     `json:"topic,omitempty"`
	Schema   string            `json:"schema,omitempty"`
}

// ErrUsage reports a message that cannot be validated as given.
var ErrUsage = errors.New("usage")

// Validate checks one message. An error wrapping ErrUsage reports an
// inconsistent request; any other error is internal.
func (v *Validator) Validate(m Message) (Result, error) {
	var res Result
	if m.Role != "" {
		if _, ok := rules.Roles[m.Role]; !ok {
			return res, fmt.Errorf("%w: unknown message role %q (birth, close, will)", ErrUsage, m.Role)
		}
	}
	switch {
	case m.Object != "" && m.Object != ObjectMetaData:
		return res, fmt.Errorf("%w: unknown object %q (%s)", ErrUsage, m.Object, ObjectMetaData)
	case m.Object != "" && (m.Topic != "" || m.Role != ""):
		return res, fmt.Errorf("%w: an object has neither topic nor role", ErrUsage)
	case m.Object == "" && m.Topic == "":
		return res, fmt.Errorf("%w: a message needs a topic", ErrUsage)
	}
	var t *topic.Parsed
	if m.Topic != "" {
		tr, err := v.Topics.Check(m.Topic)
		if err != nil {
			return res, err
		}
		res.Warnings = tr.Warnings
		for _, f := range tr.Findings {
			res.Findings = append(res.Findings, finding.Finding{Req: f.Req, Layer: finding.LayerTopic, Pointer: f.Element, Detail: f.Detail})
		}
		if !tr.Valid {
			return v.done(res), nil
		}
		t = tr.Parsed
		res.Topic = t
	}
	if m.Payload == nil {
		return v.done(res), nil
	}
	inst, err := Parse(m.Payload)
	if err != nil {
		res.Findings = append(res.Findings, finding.Finding{Req: "REQ-CMP-005", Layer: finding.LayerParse, Detail: "the payload is no strict JSON: " + err.Error()})
		return v.done(res), nil
	}
	rule := ""
	if t != nil {
		rule = t.Rule
	}
	res.Schema = schemas.Entry(rule, m.Object != "", m.Tolerant)
	fs, err := v.Schemas.Validate(res.Schema, inst)
	if err != nil {
		return res, err
	}
	res.Findings = append(res.Findings, fs...)
	if t == nil {
		res.Findings = append(res.Findings, rules.CheckMetaDataType(inst)...)
	} else {
		rt := rules.Topic{ServiceType: t.ServiceType, AppIL: t.AppIDIL, Method: t.Method, Resource: t.Resource, SourceIL: t.SourceIL, Filter: t.Filter}
		res.Findings = append(res.Findings, rules.Check(rt, inst, m.Role, v.Topics)...)
	}
	return v.done(res), nil
}

func (v *Validator) done(res Result) Result {
	res.Findings = dedupe(res.Findings)
	finding.Sort(res.Findings)
	res.Reqs = finding.Reqs(res.Findings)
	res.Valid = len(res.Findings) == 0
	return res
}

// dedupe drops a rule finding the schema layer already reports with the
// same requirement at the same pointer.
func dedupe(fs []finding.Finding) []finding.Finding {
	seen := map[string]bool{}
	for _, f := range fs {
		if f.Layer == finding.LayerSchema {
			seen[f.Req+"|"+f.Pointer] = true
		}
	}
	out := fs[:0]
	for _, f := range fs {
		if f.Layer != finding.LayerRule || !seen[f.Req+"|"+f.Pointer] {
			out = append(out, f)
		}
	}
	return out
}

// Parse decodes strict JSON (RFC 8259): UTF-8, one value, no duplicate keys;
// numbers stay json.Number.
func Parse(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("invalid UTF-8")
	}
	if err := checkDuplicates(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON value")
	}
	return v, nil
}

// checkDuplicates walks the tokens and rejects an object with a repeated key.
func checkDuplicates(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	type frame struct {
		keys    map[string]bool // nil for an array
		wantKey bool
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		d, isDelim := tok.(json.Delim)
		if isDelim && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if n := len(stack); n > 0 && stack[n-1].keys != nil {
			top := stack[n-1]
			if top.wantKey {
				k := tok.(string)
				if top.keys[k] {
					return fmt.Errorf("duplicate key %q", k)
				}
				top.keys[k] = true
				top.wantKey = false
				continue
			}
			top.wantKey = true
		}
		if isDelim {
			f := &frame{}
			if d == '{' {
				f.keys, f.wantKey = map[string]bool{}, true
			}
			stack = append(stack, f)
		}
	}
}
