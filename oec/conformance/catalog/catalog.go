// Package catalog holds the test catalogs of the compliance tools D7
// (oec-check, d7.yaml) and D8 (oec-infra, d8.yaml) – ADR-0025. A catalog
// entry names the requirements of oec/spec it checks; the tests of this
// package keep the catalogs consistent with the requirements catalog.
package catalog

import (
	"bytes"
	"embed"
	"fmt"

	"go.yaml.in/yaml/v3"
)

//go:embed d7.yaml d8.yaml
var files embed.FS

// Files of the two catalogs.
const (
	D7File = "d7.yaml"
	D8File = "d8.yaml"
)

// Catalog is the test catalog of one tool.
type Catalog struct {
	Tool     string   `yaml:"tool"`
	Baseline string   `yaml:"baseline"`
	Tests    []Test   `yaml:"tests"`
	Manual   []Manual `yaml:"manual"`
}

// Test is one planned or implemented test.
type Test struct {
	ID     string   `yaml:"id" json:"id"`
	Title  string   `yaml:"title" json:"title"`
	Mode   string   `yaml:"mode" json:"mode"`
	Level  int      `yaml:"level" json:"level"` // 0 = not graded (platform requirements only)
	Needs  []string `yaml:"needs" json:"needs"`
	Reqs   []string `yaml:"reqs" json:"reqs"`
	Status string   `yaml:"status" json:"status"`
	Notes  string   `yaml:"notes" json:"notes,omitempty"`
}

// Manual names a requirement of the tool's verification kind that no test
// checks, with the reason.
type Manual struct {
	Req    string `yaml:"req" json:"req"`
	Reason string `yaml:"reason" json:"reason"`
}

// Load reads one embedded catalog (D7File or D8File), rejecting unknown keys.
func Load(name string) (*Catalog, error) {
	raw, err := files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &c, nil
}
