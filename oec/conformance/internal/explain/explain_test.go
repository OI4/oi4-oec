package explain

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "rewrite catalog/COVERAGE.md")

func load(t *testing.T) *Index {
	t.Helper()
	ix, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestSource(t *testing.T) {
	for ref, want := range map[string]string{
		"MG 9.1":     "Messaging Guideline 9.1",
		"RIG 4.1.1":  "Runtime & Infrastructure Guideline 4.1.1",
		"GL-ADR-020": "guideline ADR GL-ADR-020",
		"SF-062":     "spec finding SF-062",
		"MG A3":      "Messaging Guideline A3",
		"ADR-0019":   "ADR-0019",
	} {
		if got := Source(ref); got != want {
			t.Errorf("Source(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestIndex(t *testing.T) {
	ix := load(t)
	for _, f := range []Filter{{Patterns: []string{"["}}, {Mode: "image"}, {Level: 4}, {Needs: "docker"}, {Status: "done"}, {Req: "REQ-NOPE-001"}} {
		if ix.Check(f) == nil {
			t.Errorf("Check(%+v) = nil", f)
		}
	}
	if got := ix.List(Filter{Level: 2, Mode: "test", Needs: "none", Patterns: []string{"T7-SET-0?"}}); len(got) != 8 {
		t.Errorf("List: %d tests", len(got))
	}
	if got := ix.List(Filter{Level: 1, Patterns: []string{"T7-SET-03"}}); len(got) != 0 {
		t.Errorf("T7-SET-03 is a level-3 test: %v", got)
	}
	tc, ok := ix.Test("T7-ADR-01")
	if !ok {
		t.Fatal("T7-ADR-01 missing")
	}
	var labels []string
	for _, f := range ix.TestFixtures(tc) {
		labels = append(labels, f.Fixture+" "+f.Outcome)
	}
	for _, want := range []string{"sim T7-ADR-01/own-app fail", "sim T7-ADR-01/silent skip", "sim none " + PassOrNA} {
		if !slices.Contains(labels, want) {
			t.Errorf("TestFixtures(T7-ADR-01) misses %q: %v", want, labels)
		}
	}
	// a positive fixture without the peer does not run T7-ADR-01
	if slices.Contains(labels, "sim T7-GET-07/empty "+PassOrNA) {
		t.Errorf("TestFixtures(T7-ADR-01) lists a fixture without peer: %v", labels)
	}
	if r, ok := ix.Manual("REQ-LIFE-011"); !ok || r == "" {
		t.Error("REQ-LIFE-011 is manual")
	}
	if k := ValidateKinds(ix.Reqs["REQ-ACT-002"]); !slices.Equal(k, []string{"schema", "d7-validate"}) {
		t.Errorf("ValidateKinds(REQ-ACT-002) = %v", k)
	}
	if fx := ix.ReqFixtures("REQ-TOP-010"); len(fx) == 0 {
		t.Error("REQ-TOP-010 has fixtures")
	}
}

// TestCoverageReport keeps catalog/COVERAGE.md up to date (-update
// rewrites it).
func TestCoverageReport(t *testing.T) {
	got := Coverage(load(t))
	file := filepath.Join("..", "..", CoverageFile)
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date: go test ./internal/explain -run TestCoverageReport -update", CoverageFile)
	}
	for _, s := range [][]byte{[]byte("| `d7-monitor` |"), []byte("| `d7-test` |"), []byte("| T7-ADR-01 | test | 1 | peer | implemented |")} {
		if !bytes.Contains(got, s) {
			t.Errorf("coverage misses %q", s)
		}
	}
}
