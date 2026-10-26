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

func TestCheckRelease(t *testing.T) {
	docs, err := bundle()
	if err != nil {
		t.Fatal(err)
	}
	changelog := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("# Changelog\n\n## "+schema.Version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cur, err := current(docs)
	if err != nil {
		t.Fatal(err)
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
			dir := t.TempDir()
			m := cur
			m.Files = append([]manifestFile(nil), cur.Files...)
			m.Files[0].Defs = append([]string(nil), cur.Files[0].Defs...)
			tc.mutate(&m)
			if err := writeJSON(filepath.Join(dir, schema.Version+".json"), m); err != nil {
				t.Fatal(err)
			}
			problems, err := checkRelease(docs, dir, changelog, schema.Baseline)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(problems, "\n")
			switch {
			case tc.want == "" && got != "":
				t.Fatalf("unexpected problems:\n%s", got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Fatalf("problems %q, want one containing %q", got, tc.want)
			}
		})
	}
	t.Run("no manifest", func(t *testing.T) {
		problems, err := checkRelease(docs, t.TempDir(), changelog, schema.Baseline)
		if err != nil {
			t.Fatal(err)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "has no manifest") {
			t.Fatalf("problems %q", problems)
		}
	})
	t.Run("other spec baseline", func(t *testing.T) {
		dir := t.TempDir()
		if err := writeJSON(filepath.Join(dir, schema.Version+".json"), cur); err != nil {
			t.Fatal(err)
		}
		problems, err := checkRelease(docs, dir, changelog, "fffffff")
		if err != nil {
			t.Fatal(err)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "the spec catalog is at") {
			t.Fatalf("problems %q", problems)
		}
	})
}
