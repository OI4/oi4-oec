package inventory

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/guideline"
)

func TestMarshalRoundTrip(t *testing.T) {
	in := &File{Baseline: "49b6fe2", Note: "n", Items: []guideline.Item{
		{Key: "MG-1-00000000", Section: "MG 1", File: "a.md", Line: 3, Kind: "sentence", Basis: "keyword",
			Keywords: []string{"MUST"}, Terms: []string{"must"}, Text: "yes"},
		{Key: "MG-1-00000001", Section: "MG 1", File: "a.md", Line: 4, Kind: "sentence", Basis: "implicit",
			Text: "2026-10-14"},
		{Key: "MG-1-00000002", Section: "MG 1", File: "a.md", Line: 5, Kind: "table-row", Basis: "table",
			Text: "null: ~ | on: off | 0x1F"},
	}}
	p := filepath.Join(t.TempDir(), "inv.yaml")
	if err := in.Write(p); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch:\n%#v\n%#v", in, out)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}
