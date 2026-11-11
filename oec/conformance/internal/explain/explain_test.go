package explain

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func loadInfra(t *testing.T) *Index {
	t.Helper()
	ix, err := LoadInfra()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// TestInfraIndex: the D8 view has its own modes, needs and fixtures
// (C-075).
func TestInfraIndex(t *testing.T) {
	ix := loadInfra(t)
	if ix.Tool.Name != "oec-infra" || len(ix.Catalog.Tests) != 17 {
		t.Fatalf("D8 index: tool %q, %d tests", ix.Tool.Name, len(ix.Catalog.Tests))
	}
	for _, f := range []Filter{{Mode: "monitor"}, {Needs: "peer"}, {Level: 4}, {Req: "REQ-NOPE-001"}} {
		if ix.Check(f) == nil {
			t.Errorf("Check(%+v) = nil", f)
		}
	}
	for _, f := range []Filter{{Mode: "image"}, {Mode: "run"}, {Needs: "docker"}, {Needs: "host-access"}, {Needs: "credentials-2"}, {Needs: "none"}} {
		if err := ix.Check(f); err != nil {
			t.Errorf("Check(%+v) = %v", f, err)
		}
	}
	ids := func(f Filter) []string {
		var out []string
		for _, tc := range ix.List(f) {
			out = append(out, tc.ID)
		}
		return out
	}
	if got := ids(Filter{Needs: "docker"}); !slices.Equal(got, []string{"T8-RUN-01", "T8-RUN-02", "T8-RUN-03", "T8-CTR-01", "T8-CTR-02", "T8-CTR-03", "T8-HST-03"}) {
		t.Errorf("needs docker: %v", got)
	}
	if got := ids(Filter{Needs: "none", Mode: "bus"}); !slices.Equal(got, []string{"T8-BUS-01", "T8-BUS-03"}) {
		t.Errorf("bus without needs: %v", got)
	}
	// an ungraded (platform) test runs at every level
	if got := ids(Filter{Level: 1, Mode: "host"}); len(got) != 4 {
		t.Errorf("host tests at level 1: %v", got)
	}
	if _, ok := ix.Test("T7-ADR-01"); ok {
		t.Error("the D8 index knows a D7 test")
	}
	labels := func(id string) []string {
		tc, ok := ix.Test(id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		var out []string
		for _, f := range ix.TestFixtures(tc) {
			out = append(out, f.Fixture+" "+f.Outcome)
		}
		return out
	}
	for id, want := range map[string][]string{
		"T8-IMG-01": {"image bad-digest.tar fail", "image good.tar " + PassOrNA},
		"T8-RUN-02": {"run good " + PassOrNA},
		"T8-HST-04": {"host none pass", "host T8-HST-04/ram-reserved pass"},
		"T8-CTR-02": {"container none " + PassOrNA},
		"T8-BUS-02": {"bus none " + PassOrNA},
	} {
		got := labels(id)
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("TestFixtures(%s) misses %q: %v", id, w, got)
			}
		}
		// a fixture of another mode never runs the test
		for _, l := range got {
			if !strings.HasPrefix(l, ix.mode(id)+" ") {
				t.Errorf("TestFixtures(%s) lists %q", id, l)
			}
		}
	}
	tc, _ := ix.Test("T8-HST-04")
	var manual []string
	for _, f := range ix.TestFixtures(tc) {
		if f.Fixture == "host none" {
			manual = f.Manual
		}
	}
	if !slices.Equal(manual, []string{"REQ-PLT-003"}) {
		t.Errorf("host none: manual %v", manual)
	}
	// ram-reserved states pass with a manual verdict but is no positive
	// fixture (as in internal/host); none is
	if c := ix.FixtureCounts(tc); c.Positive == 0 || c.Manual == 0 || c.Negative() == 0 {
		t.Errorf("T8-HST-04: %+v", c)
	}
	if r, ok := ix.Manual("REQ-CMP-012"); !ok || r == "" {
		t.Error("REQ-CMP-012 is manual in D8")
	}
	if _, ok := ix.Manual("REQ-LIFE-011"); ok {
		t.Error("REQ-LIFE-011 is a D7 manual entry")
	}
	if fx := ix.ReqFixtures("REQ-PLT-003"); len(fx) == 0 {
		t.Error("REQ-PLT-003 has fixtures (manual verdicts)")
	}
	if fx := ix.ReqFixtures("REQ-TOP-010"); len(fx) != 0 {
		t.Errorf("REQ-TOP-010 has D8 fixtures: %v", fx)
	}
	if got := ix.TestsOf("REQ-CNT-009"); !slices.Equal(got, []string{"T8-RUN-02"}) {
		t.Errorf("TestsOf(REQ-CNT-009) = %v", got)
	}
}

// mode returns the mode of a test of the catalog.
func (ix *Index) mode(id string) string {
	tc, _ := ix.Test(id)
	return tc.Mode
}

// TestFixtureCoverage: every implemented test has a negative fixture
// (fail, warn or info) and a positive one (P5 and P5b exit criterion,
// DESIGN.md §10) – D7 over the sim and capture indexes, D8 over the
// image, bus, host, container and run indexes.
func TestFixtureCoverage(t *testing.T) {
	for _, ix := range []*Index{load(t), loadInfra(t)} {
		for _, tc := range ix.Catalog.Tests {
			if tc.Status != "implemented" {
				continue
			}
			if c := ix.FixtureCounts(tc); c.Negative() == 0 || c.Positive == 0 {
				t.Errorf("%s: %d negative and %d positive fixtures, want at least one of each", tc.ID, c.Negative(), c.Positive)
			}
		}
	}
}

// TestCoverageReport keeps catalog/COVERAGE.md up to date (-update
// rewrites it).
func TestCoverageReport(t *testing.T) {
	d7 := load(t)
	got := Coverage(d7, loadInfra(t))
	// the D7 part is the D7 report alone, unchanged by the D8 part
	if !bytes.HasPrefix(got, Coverage(d7)) {
		t.Error("the D7 part depends on the D8 catalog")
	}
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
	for _, s := range [][]byte{
		[]byte("| `d7-monitor` |"), []byte("| `d7-test` |"), []byte("| T7-ADR-01 | test | 1 | peer | implemented |"),
		[]byte("\n# D8 catalog coverage\n"), []byte("| `d8` |"), []byte("| T8-HST-03 | host | – | host-access, docker | implemented |"),
		[]byte("| mode `image` | 4 |"), []byte("| needs `docker` | 7 |"), []byte("Fixtures: 19 image, 31 bus, 69 host, 26 container, 20 run;"),
		[]byte("| REQ-CMP-012 |"),
	} {
		if !bytes.Contains(got, s) {
			t.Errorf("coverage misses %q", s)
		}
	}
}
