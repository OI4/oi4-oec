// Package testvec loads the test vectors of the spec baseline
// (oec/spec/vectors) for the tests of D7. It is used by tests only.
package testvec

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"go.yaml.in/yaml/v3"
)

// Vector is the union of the vector fields D7 tests use.
type Vector struct {
	ID       string   `yaml:"id"`
	Expected string   `yaml:"expected"`
	Status   string   `yaml:"status"`
	Reqs     []string `yaml:"reqs"`
	Violates []string `yaml:"violates"`

	Topic    string         `yaml:"topic"`
	Fails    string         `yaml:"fails"`
	Parse    map[string]any `yaml:"parse"`
	Warnings []string       `yaml:"warnings"`
	IL       *string        `yaml:"il"`
	Encoded  *string        `yaml:"encoded"`
	Codes    []string       `yaml:"violations"`
	Input    string         `yaml:"input"`
	Output   string         `yaml:"output"`
	Payload  string         `yaml:"payload"`
	Object   string         `yaml:"object"`
	Role     string         `yaml:"role"`
	Mode     string         `yaml:"mode"`
	Extra    map[string]any `yaml:",inline"`
}

// SpecRoot returns the directory oec/spec.
func SpecRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "spec")
}

// Load returns the vectors of all YAML files in vectors/<dir>, sorted by id.
func Load(dir string) ([]Vector, error) {
	files, err := filepath.Glob(filepath.Join(SpecRoot(), "vectors", dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Vector
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Vectors []Vector `yaml:"vectors"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		out = append(out, doc.Vectors...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
