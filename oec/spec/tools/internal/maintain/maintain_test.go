package maintain

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const areasYAML = `# areas
baseline: "abc1234"
areas:
  - code: QRY
    title: Query
    cycle: C-027
    status: drafted   # keep this comment
    sections: [MG 8.3.13]
  - code: CNT
    title: Container
    cycle: C-030
    status: planned
    sections: [RIG 2]
`

const qryYAML = `# header
area: QRY
baseline: "abc1234"
requirements:
  - id: REQ-QRY-001
    title: one
    statement: >-
      text with
      status: draft inside a folded scalar
    status: draft

  - id: REQ-QRY-002
    title: two
    status: draft
`

const vecYAML = `kind: payload
baseline: "abc1234"
vectors:
  - id: V-PAY-053
    expected: valid
    status: draft
    payload: |-
      {
        "status": "draft"
      }
  - id: V-PAY-054
    status: reviewed
`

const matrixYAML = `baseline: "abc1234"
combinations:
  - {method: Pub, resource: MAM, kind: resource, allowed: true, payload: {status: draft, content: datasets}}
  - {method: Set, resource: MAM, kind: resource, allowed: false}
  - {method: Get, resource: MAM, kind: resource, allowed: true, payload: {status: draft, content: none}}
`

func tree(t *testing.T) string {
	return writeTree(t, map[string]string{
		"requirements/areas.yaml":          areasYAML,
		"requirements/qry.yaml":            qryYAML,
		"vectors/payload/guideline.yaml":   vecYAML,
		"matrix/method-resource.yaml":      matrixYAML,
		"coverage/informative.yaml":        "baseline: \"abc1234\"\nitems: []\n",
		"coverage/inventory.yaml":          "baseline: abc1234\nitems: []\n",
		"topic/topic.abnf":                 "; Source of truth: MG @ abc1234 - MG 7\nrule = \"abc1234\"\n",
		"requirements/catalog.schema.json": "{}",
	})
}

func TestExpandIDs(t *testing.T) {
	got, err := ExpandIDs([]string{"REQ-QRY-001..003", "V-PAY-053..V-PAY-054,MSG", "Pub/MAM"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"REQ-QRY-001", "REQ-QRY-002", "REQ-QRY-003", "V-PAY-053", "V-PAY-054", "MSG", "Pub/MAM"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, bad := range []string{"REQ-QRY-003..001", "REQ-QRY-001..V-PAY-002", "REQ..X"} {
		if _, err := ExpandIDs([]string{bad}); err == nil {
			t.Errorf("%s: expected error", bad)
		}
	}
}

func TestSetStatusReviewed(t *testing.T) {
	root := tree(t)
	ids, _ := ExpandIDs([]string{"REQ-QRY-001..002", "QRY", "V-PAY-053..054", "matrix"})
	ch, err := SetStatus(root, ids, Reviewed, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range ch {
		got = append(got, c.ID+":"+c.From+">"+c.To)
	}
	want := []string{"REQ-QRY-001:draft>reviewed", "REQ-QRY-002:draft>reviewed", "QRY:drafted>reviewed",
		"V-PAY-053:draft>reviewed", "V-PAY-054:reviewed>reviewed", "Pub/MAM:draft>reviewed", "Get/MAM:draft>reviewed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if !ch[4].Unchanged() {
		t.Error("V-PAY-054 should be unchanged")
	}
	q := read(t, root, "requirements/qry.yaml")
	if strings.Count(q, "    status: reviewed") != 2 || !strings.Contains(q, "      status: draft inside a folded scalar") {
		t.Errorf("qry.yaml:\n%s", q)
	}
	if a := read(t, root, "requirements/areas.yaml"); !strings.Contains(a, "    status: reviewed   # keep this comment") || !strings.Contains(a, "    status: planned") {
		t.Errorf("areas.yaml:\n%s", a)
	}
	if v := read(t, root, "vectors/payload/guideline.yaml"); !strings.Contains(v, `"status": "draft"`) || strings.Count(v, "    status: reviewed") != 2 {
		t.Errorf("vectors:\n%s", v)
	}
	if m := read(t, root, "matrix/method-resource.yaml"); strings.Count(m, "payload: {status: reviewed") != 2 {
		t.Errorf("matrix:\n%s", m)
	}
}

func TestSetStatusProblemsWriteNothing(t *testing.T) {
	for name, ids := range map[string][]string{
		"area with draft requirement": {"REQ-QRY-001", "QRY"},
		"planned area":                {"CNT"},
		"unknown id":                  {"REQ-QRY-009"},
		"matrix row without payload":  {"Set/MAM"},
	} {
		root := tree(t)
		if _, err := SetStatus(root, ids, Reviewed, false); err == nil {
			t.Errorf("%s: expected error", name)
		}
		if read(t, root, "requirements/qry.yaml") != qryYAML || read(t, root, "requirements/areas.yaml") != areasYAML {
			t.Errorf("%s: files were written", name)
		}
	}
}

func TestSetStatusDraftAndDryRun(t *testing.T) {
	root := tree(t)
	ch, err := SetStatus(root, []string{"V-PAY-054"}, Draft, true)
	if err != nil || len(ch) != 1 || ch[0].To != "draft" {
		t.Fatalf("%v %v", ch, err)
	}
	if read(t, root, "vectors/payload/guideline.yaml") != vecYAML {
		t.Error("dry run wrote the file")
	}
	if _, err := SetStatus(root, []string{"V-PAY-054"}, Draft, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, root, "vectors/payload/guideline.yaml"), "  - id: V-PAY-054\n    status: draft") {
		t.Error("V-PAY-054 not set to draft")
	}
	if _, err := SetStatus(root, []string{"X"}, "accepted", false); err == nil {
		t.Error("unknown target accepted")
	}
}

func TestRebaseline(t *testing.T) {
	root := tree(t)
	changed, err := Rebaseline(root, "abc1234", "def5678", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"requirements/areas.yaml", "requirements/qry.yaml", "vectors/payload/guideline.yaml",
		"coverage/informative.yaml", "matrix/method-resource.yaml", "topic/topic.abnf"}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("got %v\nwant %v", changed, want)
	}
	if !strings.Contains(read(t, root, "requirements/qry.yaml"), "baseline: \"def5678\"\n") {
		t.Error("qry.yaml not rebased")
	}
	if g := read(t, root, "topic/topic.abnf"); !strings.Contains(g, "MG @ def5678") || !strings.Contains(g, `rule = "abc1234"`) {
		t.Errorf("abnf:\n%s", g)
	}
	if inv := read(t, root, "coverage/inventory.yaml"); !strings.Contains(inv, "abc1234") {
		t.Error("generated inventory must be left to the generator")
	}
}

func TestMentions(t *testing.T) {
	long := strings.Repeat("ä", 80) + " baseline `abc1234` " + strings.Repeat("ö", 80)
	root := writeTree(t, map[string]string{"A.md": "x\nCurrent baseline: `abc1234`\n" + long + "\n"})
	got := Mentions(root, []string{"A.md", "missing.md"}, "abc1234")
	if len(got) != 2 || got[0] != "A.md:2: Current baseline: `abc1234`" || !strings.Contains(got[1], "`abc1234`") || !strings.HasPrefix(got[1], "A.md:3: …") {
		t.Fatalf("%q", got)
	}
}
