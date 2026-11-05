package guideline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFilesSkipsInformativeParts(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"docs/guideline/index.md",
		"docs/guideline/list-of-tables.md",
		"docs/guideline/appendix-a/a4-glossary.md",
		"docs/guideline/appendix-a/a5-authors.md",
		"docs/guideline/appendix-a/a6-history.md",
		"docs/guideline/appendix-a/a6-history/index.md",
		"docs/guideline/appendix-a/a6-history/a6-1-revision-1-0.md",
		"docs/guideline/appendix-b/b1.md",
		"docs/infrastructure-guideline/index.md",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# Title\n\nThe client must connect.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := func(examples bool) []string {
		files, err := loadFiles(root, examples)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range files {
			got = append(got, f.Path)
		}
		return got
	}
	want := []string{
		"docs/guideline/appendix-a/a4-glossary.md",
		"docs/guideline/index.md",
		"docs/infrastructure-guideline/index.md",
	}
	if got := paths(false); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadFiles:\ngot  %q\nwant %q", got, want)
	}
	want = []string{
		"docs/guideline/appendix-a/a4-glossary.md",
		"docs/guideline/appendix-b/b1.md",
		"docs/guideline/index.md",
		"docs/infrastructure-guideline/index.md",
	}
	if got := paths(true); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadFilesWithExamples:\ngot  %q\nwant %q", got, want)
	}
}
