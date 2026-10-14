package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/guideline"
)

// specRoot is the real oec/spec directory (schemas are taken from there).
const specRoot = "../../.."

const areasYAML = `baseline: 49b6fe2
areas:
  - code: CMP
    title: Compliance
    cycle: C-014
    status: drafted
    sections: [MG 1]
  - code: RES-MAM
    title: MAM
    cycle: C-018
    status: planned
    sections: [MG 5]
`

const validReq = `  - id: REQ-CMP-001
    title: Publish MAM
    statement: A component publishes MAM for itself.
    keyword: MUST
    domain: messaging
    levels: {L1: MUST, L2: MUST, L3: MUST}
    applies_to: [component]
    refs: [MG 1.3]
    quotes:
      - ref: MG 1.3
        text: Publish MAM for itself
    verification: [d7-monitor]
    inventory: [MG-1.3-aaaaaaaa]
    status: draft
`

func setup(t *testing.T, cmp string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{CatalogSchema, AreasSchema, InformativeSchema} {
		data, err := os.ReadFile(filepath.Join(specRoot, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		write(t, dir, f, string(data))
	}
	write(t, dir, AreasFile, areasYAML)
	write(t, dir, "requirements/cmp.yaml", "area: CMP\nbaseline: 49b6fe2\nrequirements:\n"+cmp)
	write(t, dir, InformativeFile, "baseline: 49b6fe2\nitems:\n  - key: MG-1.3-bbbbbbbb\n    reason: Informative example sentence.\n")
	return dir
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func opts() LintOptions {
	return LintOptions{
		Inventory: map[string]guideline.Item{"MG-1.3-aaaaaaaa": {}, "MG-1.3-bbbbbbbb": {}},
		Sections:  map[string]string{"MG 1.3": "Required Function Publish MAM for itself and all subordinate assets", "MG 5": "x"},
	}
}

func TestValidCatalog(t *testing.T) {
	s, err := Load(setup(t, validReq))
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.Lint(opts()); len(errs) > 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	if s.AreaOf("MG 1.3") != "CMP" || s.AreaOf("MG 10") != "" {
		t.Fatal("AreaOf mismatch")
	}
}

func TestSchemaViolations(t *testing.T) {
	cases := map[string][2]string{
		"unknown field":      {"status: draft", "status: draft\n    maturity: high"},
		"bad keyword":        {"keyword: MUST", "keyword: REQUIRED"},
		"missing levels":     {"    levels: {L1: MUST, L2: MUST, L3: MUST}\n", ""},
		"platform levels":    {"domain: messaging", "domain: platform"},
		"implicit no SF-035": {"keyword: MUST", "keyword: MUST\n    implicit: true"},
		"bad ref":            {"refs: [MG 1.3]", "refs: [chapter 1]"},
		"bad verification":   {"[d7-monitor]", "[d9]"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(validReq, c[0]) {
				t.Fatalf("fixture lacks %q", c[0])
			}
			_, err := Load(setup(t, strings.Replace(validReq, c[0], c[1], 1)))
			if err == nil {
				t.Fatal("expected schema error")
			}
		})
	}
}

func TestLintRules(t *testing.T) {
	second := strings.Replace(validReq, "REQ-CMP-001", "REQ-CMP-002", 1)
	cases := map[string]struct {
		cmp, want string
	}{
		"keyword vs L3":      {strings.Replace(validReq, "keyword: MUST", "keyword: SHOULD", 1), "Level 3"},
		"weakening":          {strings.Replace(validReq, "L2: MUST", "L2: MAY", 1), "weaken"},
		"mixed polarity":     {strings.Replace(validReq, "L1: MUST,", "L1: MUST NOT,", 1), "mix"},
		"wrong prefix":       {strings.Replace(validReq, "REQ-CMP-001", "REQ-IL-001", 1), "start with REQ-CMP-"},
		"not ascending":      {second + validReq, "ascending"},
		"quote ref":          {strings.Replace(validReq, "      - ref: MG 1.3", "      - ref: MG 1.2", 1), "not listed in refs"},
		"quote text":         {strings.Replace(validReq, "Publish MAM for itself", "Publish Health", 1), "not found verbatim"},
		"unknown inventory":  {strings.Replace(validReq, "aaaaaaaa", "cccccccc", 1), "does not exist"},
		"informative+mapped": {strings.Replace(validReq, "aaaaaaaa", "bbbbbbbb", 1), "also mapped"},
		"unknown ref":        {strings.Replace(validReq, "refs: [MG 1.3]", "refs: [MG 1.3, MG 9.9]", 1), "MG 9.9"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, err := Load(setup(t, c.cmp))
			if err != nil {
				t.Fatal(err)
			}
			errs := s.Lint(opts())
			for _, e := range errs {
				if strings.Contains(e.Error(), c.want) {
					return
				}
			}
			t.Fatalf("expected error containing %q, got %v", c.want, errs)
		})
	}
}

func TestDraftedAreaNeedsFile(t *testing.T) {
	dir := setup(t, validReq)
	write(t, dir, AreasFile, strings.Replace(areasYAML, "status: planned", "status: drafted", 1))
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := s.Lint(opts())
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "res-mam.yaml") {
		t.Fatalf("expected missing file error, got %v", errs)
	}
}
