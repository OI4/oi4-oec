package probe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/schema"
)

// TestClasses checks the DataSetClassIds of the requests against the entry
// schemas get-<resource>.
func TestClasses(t *testing.T) {
	for res, id := range Classes {
		raw, err := schema.FS.ReadFile("v2/topics/get-" + strings.ToLower(res) + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Properties struct {
				DataSetClassID struct {
					Const string `json:"const"`
				} `json:"DataSetClassId"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Properties.DataSetClassID.Const != id {
			t.Errorf("%s: DataSetClassId %s, schema %s", res, id, doc.Properties.DataSetClassID.Const)
		}
	}
}

// TestTesterIL checks the generated identity of D7 (DESIGN.md §3.5).
func TestTesterIL(t *testing.T) {
	a, b := TesterIL(), TesterIL()
	if a == b {
		t.Errorf("two runs share the tester identity %s", a)
	}
	if v := ilink.Validate(a); len(v) > 0 {
		t.Errorf("%s: %v", a, v)
	}
}

func TestSplitApp(t *testing.T) {
	for _, c := range []struct{ in, st, app string }{
		{"OTConnector/a.com/M/P/S", "OTConnector", "a.com/M/P/S"},
		{"a.com/M/P/S", "", "a.com/M/P/S"},
		{"urn:x/OTConnector", "", "urn:x/OTConnector"},
	} {
		if st, app := SplitApp(c.in); st != c.st || app != c.app {
			t.Errorf("%s: %q %q, want %q %q", c.in, st, app, c.st, c.app)
		}
	}
}

func TestSelected(t *testing.T) {
	if !Selected(nil, "T7-GET-01") || !Selected([]string{"T7-GET-*"}, "T7-GET-05") || Selected([]string{"T7-GET-0[12]"}, "T7-GET-05") || !Selected([]string{"T7-MON-01", "T7-GET-09"}, "T7-GET-09") {
		t.Error("Selected")
	}
}
