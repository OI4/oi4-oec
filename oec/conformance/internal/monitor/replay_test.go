package monitor

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

type fixtureIndex struct {
	Captures []struct {
		File     string `yaml:"file"`
		Status   string `yaml:"status"`
		Positive bool   `yaml:"positive"`
		Expect   map[string]struct {
			Outcome verdict.Outcome `yaml:"outcome"`
			Reqs    []string        `yaml:"reqs"`
		} `yaml:"expect"`
		Notes string `yaml:"notes"`
	} `yaml:"captures"`
}

func loadIndex(t *testing.T) fixtureIndex {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "index.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var ix fixtureIndex
	dec := yaml.NewDecoder(bytesReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&ix); err != nil {
		t.Fatal(err)
	}
	return ix
}

func replayFile(t *testing.T, v *validate.Validator, name string) Result {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := capture.Read(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	res, err := Replay(v, c, DefaultOptions())
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

// TestFixtures replays every capture fixture and compares the verdicts at
// level 3 with testdata/captures/index.yaml.
func TestFixtures(t *testing.T) {
	v, err := validate.New()
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	tests, err := Tests()
	if err != nil {
		t.Fatal(err)
	}
	ix := loadIndex(t)
	positive, negative := map[string]bool{}, map[string]bool{}
	files := map[string]bool{}
	for _, c := range ix.Captures {
		files[c.File] = true
		if c.Status != "draft" && c.Status != "reviewed" {
			t.Errorf("%s: status %q", c.File, c.Status)
		}
		res := replayFile(t, v, c.File)
		for _, tr := range verdict.Build(tests, res.Observations, reqs, 3) {
			if tr.Status != "implemented" {
				continue
			}
			want, listed := c.Expect[tr.ID]
			if !listed {
				if tr.Outcome != verdict.Pass && tr.Outcome != verdict.Skip {
					t.Errorf("%s %s: outcome %s, want pass or skip (%v)", c.File, tr.ID, tr.Outcome, tr.Reqs)
				}
				if c.Positive && tr.Outcome == verdict.Pass {
					positive[tr.ID] = true
				}
				continue
			}
			negative[tr.ID] = true
			if tr.Outcome != want.Outcome {
				t.Errorf("%s %s: outcome %s, want %s (%v)", c.File, tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
			}
			var got []string
			for _, r := range tr.Reqs {
				if r.Outcome == tr.Outcome {
					got = append(got, r.Req)
				}
			}
			sort.Strings(got)
			w := slices.Clone(want.Reqs)
			sort.Strings(w)
			if !slices.Equal(got, w) {
				t.Errorf("%s %s: requirements with outcome %s %v, want %v", c.File, tr.ID, tr.Outcome, got, w)
			}
		}
	}
	for _, id := range Implemented {
		if !positive[id] {
			t.Errorf("%s has no positive fixture (DESIGN.md §10)", id)
		}
		if !negative[id] {
			t.Errorf("%s has no negative fixture (DESIGN.md §10)", id)
		}
	}
	all, _ := filepath.Glob(filepath.Join(fixtureDir, "*.jsonl"))
	for _, f := range all {
		if !files[filepath.Base(f)] {
			t.Errorf("%s is not in index.yaml", f)
		}
	}
}

// TestImplemented keeps Implemented equal to the catalog status.
func TestImplemented(t *testing.T) {
	tests, err := Tests()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tc := range tests {
		if tc.Status == "implemented" {
			got = append(got, tc.ID)
		}
	}
	if !slices.Equal(got, Implemented) {
		t.Errorf("catalog implemented %v, engine %v", got, Implemented)
	}
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
