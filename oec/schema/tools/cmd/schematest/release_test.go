package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	schema "github.com/OI4/oec/schema"
)

func TestLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"2.0.0", "2.0.1", true},
		{"2.0.1", "2.0.0", false},
		{"2.0.0", "2.1.0-dev", true},
		{"2.1.0-dev", "2.1.0", true},
		{"2.1.0", "2.1.0-dev", false},
		{"2.9.0", "2.10.0", true},
		{"2.0.0", "2.0.0", false},
	} {
		if got := less(c.a, c.b); got != c.want {
			t.Errorf("less(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestCheckRelease checks the freeze rules for a release version and for a
// pre-release version, independently of the current schema.Version.
func TestCheckRelease(t *testing.T) {
	const rel, dev = "2.0.0", "2.0.1-dev"
	docs, err := bundle()
	if err != nil {
		t.Fatal(err)
	}
	changelog := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("# Changelog\n\n## Unreleased\n\n## "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cur, err := current(docs)
	if err != nil {
		t.Fatal(err)
	}
	cur.Version, cur.Baseline = rel, schema.Baseline
	write := func(t *testing.T, m manifest) string {
		dir := t.TempDir()
		if err := writeJSON(filepath.Join(dir, m.Version+".json"), m); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	check := func(t *testing.T, version, dir, specBaseline string) string {
		problems, err := checkVersion(version, schema.Baseline, docs, dir, changelog, specBaseline)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(problems, "\n")
	}
	tests := []struct {
		name   string
		mutate func(m *manifest)
		want   string // substring of a problem, "" for none
	}{
		{"frozen tree", func(*manifest) {}, ""},
		{"file changed", func(m *manifest) { m.Files[0].SHA256 = strings.Repeat("0", 64) }, "changed after the release"},
		{"$defs name removed", func(m *manifest) { m.Files[0].Defs = append(m.Files[0].Defs, "Gone") }, "#/$defs/Gone was removed"},
		{"file removed", func(m *manifest) { m.Files = append(m.Files, manifestFile{Path: "gone.schema.json"}) }, "gone.schema.json was removed"},
		{"file added", func(m *manifest) { m.Files = m.Files[1:] }, "is not part of the release"},
		{"other baseline", func(m *manifest) { m.Baseline = "0000000" }, "bump schema.Version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := cur
			m.Files = append([]manifestFile(nil), cur.Files...)
			m.Files[0].Defs = append([]string(nil), cur.Files[0].Defs...)
			tc.mutate(&m)
			got := check(t, rel, write(t, m), schema.Baseline)
			switch {
			case tc.want == "" && got != "":
				t.Fatalf("unexpected problems:\n%s", got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Fatalf("problems %q, want one containing %q", got, tc.want)
			}
		})
	}
	t.Run("no manifest", func(t *testing.T) {
		if got := check(t, rel, t.TempDir(), schema.Baseline); !strings.Contains(got, "has no manifest") || strings.Contains(got, "\n") {
			t.Fatalf("problems %q", got)
		}
	})
	t.Run("other spec baseline", func(t *testing.T) {
		if got := check(t, rel, write(t, cur), "fffffff"); !strings.Contains(got, "the spec catalog is at") || strings.Contains(got, "\n") {
			t.Fatalf("problems %q", got)
		}
	})
	t.Run("pre-release above the release", func(t *testing.T) {
		m := cur
		m.Baseline = "0000000" // the pre-release may have a new baseline and changed files
		m.Files = m.Files[1:]
		if got := check(t, dev, write(t, m), schema.Baseline); got != "" {
			t.Fatalf("unexpected problems:\n%s", got)
		}
	})
	t.Run("pre-release without any release", func(t *testing.T) {
		if got := check(t, dev, t.TempDir(), schema.Baseline); got != "" {
			t.Fatalf("unexpected problems:\n%s", got)
		}
	})
	t.Run("pre-release not above the release", func(t *testing.T) {
		m := cur
		m.Version = "2.0.1"
		if got := check(t, dev, write(t, m), schema.Baseline); !strings.Contains(got, "is not above the release 2.0.1") {
			t.Fatalf("problems %q", got)
		}
	})
	t.Run("released $defs name removed in a pre-release", func(t *testing.T) {
		m := cur
		m.Files = append([]manifestFile(nil), cur.Files...)
		m.Files[0].Defs = append(append([]string(nil), cur.Files[0].Defs...), "Gone")
		if got := check(t, dev, write(t, m), schema.Baseline); !strings.Contains(got, "#/$defs/Gone was removed") {
			t.Fatalf("problems %q", got)
		}
	})
}
