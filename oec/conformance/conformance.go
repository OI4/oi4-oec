// Package conformance holds the data files of the compliance tools that
// live outside a Go package directory: the JSON Schemas of the D7 report
// (report.schema.json, report version 1, DESIGN.md §7) and of the D8 report
// (infra-report.schema.json, report version 1, DESIGN.md §9) and the
// indexes of the self-test fixtures (testdata/sim/index.yaml,
// testdata/captures/index.yaml and testdata/images/index.yaml, DESIGN.md
// §10), which the explain commands and the catalog coverage report show.
// It holds data and their loaders only and imports no package of the tools.
package conformance

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// ReportSchema is the JSON Schema (2020-12) of the JSON report of oec-check
// monitor and test, report version 1.
//
//go:embed report.schema.json
var ReportSchema []byte

// ReportSchemaID is the $id of ReportSchema.
const ReportSchemaID = "https://schemas.openindustry4.com/oec/tools/oec-check/report.schema.json"

// InfraReportSchema is the JSON Schema (2020-12) of the JSON report of
// oec-infra, report version 1.
//
//go:embed infra-report.schema.json
var InfraReportSchema []byte

// InfraReportSchemaID is the $id of InfraReportSchema.
const InfraReportSchemaID = "https://schemas.openindustry4.com/oec/tools/oec-infra/report.schema.json"

type compiled struct {
	once   sync.Once
	schema *jsonschema.Schema
	err    error
}

var reportSchema, infraReportSchema compiled

func (c *compiled) validate(name, id string, raw, doc []byte) error {
	c.once.Do(func() {
		s, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			c.err = err
			return
		}
		comp := jsonschema.NewCompiler()
		comp.DefaultDraft(jsonschema.Draft2020)
		comp.AssertFormat()
		if err := comp.AddResource(id, s); err != nil {
			c.err = err
			return
		}
		c.schema, c.err = comp.Compile(id)
	})
	if c.err != nil {
		return fmt.Errorf("%s: %w", name, c.err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("the report is no JSON document: %w", err)
	}
	return c.schema.Validate(v)
}

// ValidateReport validates one JSON report of monitor or test against
// ReportSchema (formats such as date-time are asserted).
func ValidateReport(doc []byte) error {
	return reportSchema.validate("report.schema.json", ReportSchemaID, ReportSchema, doc)
}

// ValidateInfraReport validates one JSON report of oec-infra against
// InfraReportSchema (formats such as date-time are asserted).
func ValidateInfraReport(doc []byte) error {
	return infraReportSchema.validate("infra-report.schema.json", InfraReportSchemaID, InfraReportSchema, doc)
}

//go:embed testdata/sim/index.yaml
var simIndex []byte

//go:embed testdata/captures/index.yaml
var captureIndex []byte

//go:embed testdata/images/index.yaml
var imageIndex []byte

//go:embed testdata/bus/index.yaml
var busIndex []byte

// Expect is the expected verdict of one test in a fixture: the outcome and
// the requirements with this outcome.
type Expect struct {
	Outcome string   `yaml:"outcome"`
	Reqs    []string `yaml:"reqs"`
}

// Fixture is one entry of a fixture index.
type Fixture struct {
	Kind     string            `yaml:"-"` // sim | capture | image | bus
	Name     string            `yaml:"-"` // the fault (sim) or the file (capture)
	Fault    string            `yaml:"fault"`
	File     string            `yaml:"file"`
	Status   string            `yaml:"status"`
	Positive bool              `yaml:"positive"`
	Tests    []string          `yaml:"tests"` // sim, bus: the selected tests (patterns); empty: every test
	Peer     bool              `yaml:"peer"`
	Announce bool              `yaml:"announce"`
	Expect   map[string]Expect `yaml:"expect"`
	Notes    string            `yaml:"notes"`
}

// Runs reports whether the fixture runs the test (its test selection).
func (f Fixture) Runs(id string) bool {
	if len(f.Tests) == 0 {
		return true
	}
	for _, p := range f.Tests {
		if ok, _ := path.Match(p, id); ok {
			return true
		}
	}
	return false
}

func decode(raw []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(v)
}

// SimFixtures returns the entries of testdata/sim/index.yaml in order.
func SimFixtures() ([]Fixture, error) {
	var ix struct {
		Fixtures []Fixture `yaml:"fixtures"`
	}
	if err := decode(simIndex, &ix); err != nil {
		return nil, fmt.Errorf("testdata/sim/index.yaml: %w", err)
	}
	for i := range ix.Fixtures {
		f := &ix.Fixtures[i]
		if f.Fault == "" || f.File != "" {
			return nil, fmt.Errorf("testdata/sim/index.yaml: entry %d: a sim fixture names a fault and no file", i+1)
		}
		f.Kind, f.Name = "sim", f.Fault
	}
	return ix.Fixtures, nil
}

// CaptureFixtures returns the entries of testdata/captures/index.yaml in
// order.
func CaptureFixtures() ([]Fixture, error) {
	var ix struct {
		Captures []Fixture `yaml:"captures"`
	}
	if err := decode(captureIndex, &ix); err != nil {
		return nil, fmt.Errorf("testdata/captures/index.yaml: %w", err)
	}
	for i := range ix.Captures {
		f := &ix.Captures[i]
		if f.File == "" || f.Fault != "" || len(f.Tests) > 0 || f.Peer || f.Announce {
			return nil, fmt.Errorf("testdata/captures/index.yaml: entry %d: a capture fixture names a file and nothing of a sim run", i+1)
		}
		f.Kind, f.Name = "capture", f.File
	}
	return ix.Captures, nil
}

// ImageFixtures returns the entries of testdata/images/index.yaml (the
// image archives of oec-infra image) in order.
func ImageFixtures() ([]Fixture, error) {
	var ix struct {
		Images []Fixture `yaml:"images"`
	}
	if err := decode(imageIndex, &ix); err != nil {
		return nil, fmt.Errorf("testdata/images/index.yaml: %w", err)
	}
	for i := range ix.Images {
		f := &ix.Images[i]
		if f.File == "" || f.Fault != "" || len(f.Tests) > 0 || f.Peer || f.Announce {
			return nil, fmt.Errorf("testdata/images/index.yaml: entry %d: an image fixture names a file and nothing of a sim run", i+1)
		}
		f.Kind, f.Name = "image", f.File
	}
	return ix.Images, nil
}

// BusFixtures returns the entries of testdata/bus/index.yaml (the broker
// scenarios of oec-infra bus) in order. They are not part of Fixtures:
// internal/explain stays D7-only until P5b increment 6 (A-248).
func BusFixtures() ([]Fixture, error) {
	var ix struct {
		Scenarios []Fixture `yaml:"scenarios"`
	}
	if err := decode(busIndex, &ix); err != nil {
		return nil, fmt.Errorf("testdata/bus/index.yaml: %w", err)
	}
	for i := range ix.Scenarios {
		f := &ix.Scenarios[i]
		if f.Fault == "" || f.File != "" || f.Peer || f.Announce {
			return nil, fmt.Errorf("testdata/bus/index.yaml: entry %d: a bus fixture names a fault and no file", i+1)
		}
		f.Kind, f.Name = "bus", f.Fault
	}
	return ix.Scenarios, nil
}

// Fixtures returns the sim fixtures followed by the capture fixtures.
func Fixtures() ([]Fixture, error) {
	s, err := SimFixtures()
	c, cerr := CaptureFixtures()
	if err = errors.Join(err, cerr); err != nil {
		return nil, err
	}
	return append(s, c...), nil
}

// Label is the short name of a fixture: "sim <fault>" or "capture <file>".
func (f Fixture) Label() string { return f.Kind + " " + strings.TrimSpace(f.Name) }
