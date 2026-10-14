// Package yamlstrict loads YAML restricted to a JSON-compatible subset (ADR-0018):
// one document, no anchors/aliases, no explicit tags, no merge keys, no duplicate
// or non-string keys, decimal numbers only, and no plain scalars that other YAML
// parsers would read differently (YAML 1.1 booleans, timestamps).
package yamlstrict

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

var (
	decimalInt   = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	decimalFloat = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][-+]?[0-9]+)?$`)
	yaml11Bool   = map[string]bool{
		"y": true, "yes": true, "n": true, "no": true, "on": true, "off": true,
	}
)

// LoadFile reads and converts a YAML file. See Load.
func LoadFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := Load(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

// Load parses data and returns JSON-compatible Go values: map[string]any,
// []any, string, json.Number, bool and nil.
func Load(data []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty document")
		}
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		return nil, errors.New("more than one YAML document")
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, errors.New("unexpected document structure")
	}
	return convert(doc.Content[0])
}

func convert(n *yaml.Node) (any, error) {
	if n.Anchor != "" {
		return nil, nodeErr(n, "anchors are not allowed")
	}
	if n.Style&yaml.TaggedStyle != 0 {
		return nil, nodeErr(n, "explicit tags are not allowed")
	}
	switch n.Kind {
	case yaml.AliasNode:
		return nil, nodeErr(n, "aliases are not allowed")
	case yaml.MappingNode:
		return convertMap(n)
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := convert(c)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.ScalarNode:
		return convertScalar(n)
	default:
		return nil, nodeErr(n, "unsupported node kind")
	}
}

func convertMap(n *yaml.Node) (any, error) {
	out := make(map[string]any, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind == yaml.ScalarNode && k.Tag == "!!merge" {
			return nil, nodeErr(k, "merge keys are not allowed")
		}
		if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
			return nil, nodeErr(k, "mapping keys must be strings")
		}
		if _, dup := out[k.Value]; dup {
			return nil, nodeErr(k, fmt.Sprintf("duplicate key %q", k.Value))
		}
		cv, err := convert(v)
		if err != nil {
			return nil, err
		}
		out[k.Value] = cv
	}
	return out, nil
}

func convertScalar(n *yaml.Node) (any, error) {
	plain := n.Style == 0
	switch n.Tag {
	case "!!str":
		if plain && yaml11Bool[strings.ToLower(n.Value)] {
			return nil, nodeErr(n, fmt.Sprintf("ambiguous plain scalar %q must be quoted", n.Value))
		}
		return n.Value, nil
	case "!!bool":
		if n.Value != "true" && n.Value != "false" {
			return nil, nodeErr(n, fmt.Sprintf("boolean must be written as true or false, got %q", n.Value))
		}
		return n.Value == "true", nil
	case "!!null":
		if n.Value != "null" {
			return nil, nodeErr(n, "null must be written as null")
		}
		return nil, nil
	case "!!int":
		if !decimalInt.MatchString(n.Value) {
			return nil, nodeErr(n, fmt.Sprintf("integer %q must be decimal", n.Value))
		}
		return json.Number(n.Value), nil
	case "!!float":
		if !decimalFloat.MatchString(n.Value) {
			return nil, nodeErr(n, fmt.Sprintf("number %q is not JSON-compatible", n.Value))
		}
		if f, err := strconv.ParseFloat(n.Value, 64); err != nil || math.IsInf(f, 0) {
			return nil, nodeErr(n, fmt.Sprintf("number %q out of range", n.Value))
		}
		return json.Number(n.Value), nil
	default:
		return nil, nodeErr(n, fmt.Sprintf("scalar type %s is not allowed (quote the value)", n.Tag))
	}
}

func nodeErr(n *yaml.Node, msg string) error {
	return fmt.Errorf("line %d, column %d: %s", n.Line, n.Column, msg)
}

// Decode converts a loaded value into a typed struct via JSON, rejecting unknown fields.
func Decode(v any, out any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}
