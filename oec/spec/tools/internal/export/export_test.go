package export

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/vectors"
)

// The exported JSON decodes to exactly the data spectool loaded from YAML,
// so SDK test harnesses see the same vectors as 'spectool lint'.
func TestFilesRoundTrip(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	ts, err := topic.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := vectors.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Files(ts, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(files)+1 {
		t.Fatalf("%d exported files, want %d", len(out), len(files)+1)
	}
	for _, f := range files {
		name := VectorName(f.Path)
		content, ok := out[name]
		if !ok {
			t.Fatalf("%s: no export %s", f.Path, name)
		}
		var got vectorFile
		if err := json.Unmarshal([]byte(content), &got); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := f
		want.Path = ""
		if got.Source != f.Path || !strings.Contains(got.Comment, "do not edit") {
			t.Errorf("%s: header %+v", name, got.header)
		}
		if !reflect.DeepEqual(got.File, want) {
			t.Errorf("%s: round trip differs from %s", name, f.Path)
		}
	}
	var m matrixFile
	if err := json.Unmarshal([]byte(out[MatrixFile]), &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Matrix, ts.Matrix) {
		t.Error("matrix round trip differs")
	}
}

func TestVectorName(t *testing.T) {
	if got := VectorName("vectors/il/dnp-encoding.yaml"); got != "export/vectors/il-dnp-encoding.json" {
		t.Errorf("got %s", got)
	}
}
