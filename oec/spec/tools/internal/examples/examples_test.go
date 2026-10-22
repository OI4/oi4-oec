package examples

import (
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/vectors"
)

func TestExtractAndCompute(t *testing.T) {
	files := []guideline.File{{
		Doc: "MG", Section: "B1.1", Path: "b1.md",
		Lines: strings.Split(strings.Join([]string{
			"# B1.1 Example",
			"```json",
			`{"A": 1, "B": {"C": 2}}`,
			"```",
			"Fragment:",
			"```json",
			`"B": {"C": 2}`,
			"```",
			"````text",
			"```nested```",
			"````",
			"```json",
			`{"Template": "<placeholder>"}`,
			"```",
		}, "\n"), "\n"),
	}}
	blocks := Extract(files)
	if len(blocks) != 4 {
		t.Fatalf("want 4 blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Line != 2 || blocks[0].Lang != "json" || blocks[0].Ref != "MG B1.1" || !strings.HasPrefix(blocks[0].Key, "MG-B1.1-") {
		t.Fatalf("unexpected first block %+v", blocks[0])
	}
	if blocks[2].Text != "```nested```" || blocks[2].Lang != "text" {
		t.Fatalf("longer fence not handled: %+v", blocks[2])
	}
	vf := []vectors.File{{Kind: "payload", Vectors: []vectors.Vector{
		{ID: "V-1", Origin: vectors.Origin{Kind: "guideline", Ref: "MG B1.1"}, Payload: `{"A": 1, "B": {"C": 2}}`},
		{ID: "V-2", Origin: vectors.Origin{Kind: "constructed"}, Payload: `{"Template": "<placeholder>"}`},
	}}}
	ex := ExplainedFile{Items: []Explained{
		{Keys: []string{blocks[0].Key}, Reason: "covered anyway"},
		{Keys: []string{blocks[3].Key, "MG-B1.1-00000000"}, Reason: "template"},
	}}
	r := Compute(blocks, vf, ex)
	if got := r.Blocks[0].Vectors; len(got) != 1 || got[0] != "V-1" {
		t.Fatalf("block 0 not reproduced by V-1: %+v", r.Blocks[0])
	}
	if got := r.Blocks[1].Fragment; len(got) != 1 || got[0] != "V-1" {
		t.Fatalf("member fragment not found in V-1: %+v", r.Blocks[1])
	}
	if r.Blocks[3].Covered() {
		t.Fatal("constructed vectors must not cover guideline examples")
	}
	want := []string{"drop the explanation", "has no golden vector", "is no code block"}
	if len(r.Problems) != len(want) {
		t.Fatalf("want %d problems, got %v", len(want), r.Problems)
	}
	for i, w := range want {
		if !strings.Contains(r.Problems[i], w) {
			t.Fatalf("problem %d: want %q, got %q", i, w, r.Problems[i])
		}
	}
	rep := r.Report()
	if !strings.Contains(rep, "| MG | 0 |") || !strings.Contains(rep, "| MG Appendix B | 4 | 2 | 1 | 1 |") || !strings.Contains(rep, "fragment of V-1") {
		t.Fatalf("unexpected report:\n%s", rep)
	}
}

func TestKeysAreStable(t *testing.T) {
	f := guideline.File{Doc: "RIG", Section: "4.2", Lines: []string{"```", "x", "```", "", "```", "x", "```"}}
	a := Extract([]guideline.File{f})
	f.Lines = append([]string{"intro", ""}, f.Lines...)
	b := Extract([]guideline.File{f})
	if a[0].Key != b[0].Key || a[1].Key != b[1].Key || a[0].Key == a[1].Key {
		t.Fatalf("keys must depend on content and order only: %v %v", a, b)
	}
}
