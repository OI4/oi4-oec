// Package catalog loads and checks the requirements catalog in oec/spec
// (ADR-0018): area registry, area files and the informative classification.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/schema"
	"github.com/OI4/oec/spec/tools/internal/yamlstrict"
)

// Requirement is one catalog entry (catalog.schema.json).
type Requirement struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Statement    string   `json:"statement"`
	Keyword      string   `json:"keyword"`
	Implicit     bool     `json:"implicit,omitempty"`
	Domain       string   `json:"domain"`
	Levels       *Levels  `json:"levels,omitempty"`
	AppliesTo    []string `json:"applies_to"`
	Condition    string   `json:"condition,omitempty"`
	Scope        []string `json:"scope,omitempty"`
	Refs         []string `json:"refs"`
	Quotes       []Quote  `json:"quotes,omitempty"`
	Verification []string `json:"verification"`
	Inventory    []string `json:"inventory,omitempty"`
	Status       string   `json:"status"`
	ReplacedBy   string   `json:"replaced_by,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

// Levels holds the obligation per compliance level (MG 1.3).
type Levels struct {
	L1 string `json:"L1"`
	L2 string `json:"L2"`
	L3 string `json:"L3"`
}

// Quote is verbatim guideline text supporting a requirement.
type Quote struct {
	Ref  string `json:"ref"`
	Text string `json:"text"`
}

// AreaFile is one requirements/<area>.yaml.
type AreaFile struct {
	Area         string        `json:"area"`
	Baseline     string        `json:"baseline"`
	Requirements []Requirement `json:"requirements"`
	Path         string        `json:"-"`
}

// Area is one entry of requirements/areas.yaml.
type Area struct {
	Code     string   `json:"code"`
	Title    string   `json:"title"`
	Cycle    string   `json:"cycle"`
	Status   string   `json:"status"`
	Sections []string `json:"sections"`
	Notes    string   `json:"notes,omitempty"`
}

// Areas is requirements/areas.yaml.
type Areas struct {
	Baseline string `json:"baseline"`
	Areas    []Area `json:"areas"`
}

// InformativeItem classifies an inventory item as informative (MG 1.2).
type InformativeItem struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// Informative is coverage/informative.yaml.
type Informative struct {
	Baseline string            `json:"baseline"`
	Items    []InformativeItem `json:"items"`
}

// Spec is the loaded catalog.
type Spec struct {
	Root        string
	Areas       Areas
	Files       []AreaFile
	Informative Informative
}

// Paths inside the spec directory.
const (
	RequirementsDir   = "requirements"
	AreasFile         = "requirements/areas.yaml"
	AreasSchema       = "requirements/areas.schema.json"
	CatalogSchema     = "requirements/catalog.schema.json"
	InformativeFile   = "coverage/informative.yaml"
	InformativeSchema = "coverage/informative.schema.json"
	InventoryFile     = "coverage/inventory.yaml"
	ViewDir           = "requirements/view"
	ReportFile        = "coverage/report.md"
)

// AreaFileName returns the file name of an area ("RES-MAM" -> "res-mam.yaml").
func AreaFileName(code string) string { return strings.ToLower(code) + ".yaml" }

// Load reads and schema-validates all catalog files below root.
func Load(root string) (*Spec, error) {
	s := &Spec{Root: root}
	if err := loadValidated(root, AreasFile, AreasSchema, &s.Areas); err != nil {
		return nil, err
	}
	cat, err := schema.Compile(filepath.Join(root, filepath.FromSlash(CatalogSchema)))
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, RequirementsDir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") && e.Name() != "areas.yaml" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		rel := RequirementsDir + "/" + n
		v, err := yamlstrict.LoadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if err := cat.Validate(v); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		var af AreaFile
		if err := yamlstrict.Decode(v, &af); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		af.Path = rel
		s.Files = append(s.Files, af)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(InformativeFile))); err == nil {
		if err := loadValidated(root, InformativeFile, InformativeSchema, &s.Informative); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func loadValidated(root, file, schemaFile string, out any) error {
	v, err := yamlstrict.LoadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return err
	}
	val, err := schema.Compile(filepath.Join(root, filepath.FromSlash(schemaFile)))
	if err != nil {
		return err
	}
	if err := val.Validate(v); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if err := yamlstrict.Decode(v, out); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

// Requirements returns all requirements of all area files.
func (s *Spec) Requirements() []Requirement {
	var out []Requirement
	for _, f := range s.Files {
		out = append(out, f.Requirements...)
	}
	return out
}

// Area returns the registry entry of an area code.
func (s *Spec) Area(code string) (Area, bool) {
	for _, a := range s.Areas.Areas {
		if a.Code == code {
			return a, true
		}
	}
	return Area{}, false
}
