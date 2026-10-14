// Package schema validates loaded YAML values against JSON Schema 2020-12 files.
package schema

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates values against one compiled schema.
type Validator struct {
	path   string
	schema *jsonschema.Schema
}

// Compile loads and compiles the schema file at path (draft 2020-12 enforced).
func Compile(path string) (*Validator, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	url := "file://" + filepath.ToSlash(abs)
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Validator{path: path, schema: s}, nil
}

// Validate returns a readable error listing all schema violations.
func (v *Validator) Validate(value any) error {
	err := v.schema.Validate(value)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		return fmt.Errorf("schema %s: %s", filepath.Base(v.path), flatten(ve))
	}
	return err
}

func flatten(ve *jsonschema.ValidationError) string {
	out := ve.BasicOutput()
	msg := ""
	for _, u := range out.Errors {
		if u.Error == nil {
			continue
		}
		msg += fmt.Sprintf("\n  at %q: %s", u.InstanceLocation, u.Error.String())
	}
	if msg == "" {
		return ve.Error()
	}
	return msg
}
