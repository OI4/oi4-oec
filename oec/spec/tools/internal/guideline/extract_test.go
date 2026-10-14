package guideline

import (
	"reflect"
	"strings"
	"testing"
)

const sample = `---
title: "9.9 Sample"
---

# 9.9 Sample

The publisher must send the message. It is answered with an e.g. empty reply.
This sentence is plain prose.

:::tip RECOMMENDED
Use short topics.
:::

### Field

Value Range: ` + "`<String>`" + `  
Type: String  
Requirement: Mandatory

| Method | Level 1 | Level 2 |
|:---|:---:|:---:|
| ` + "`Pub`" + ` | Mandatory | Mandatory |
| ` + "`Set`" + ` | – | Where applicable |

- A client can reconnect.

` + "```json\n{\"must\": true}\n```" + `

*Table 1: Should not appear*
`

func file() File {
	return File{Doc: "MG", Path: "docs/guideline/x.md", Section: "9.9", Lines: strings.Split(sample, "\n")}
}

func TestExtract(t *testing.T) {
	items := Extract(file())
	type row struct{ kind, basis, text string }
	var got []row
	for _, it := range items {
		got = append(got, row{it.Kind, it.Basis, it.Text})
	}
	want := []row{
		{KindSentence, BasisKeyword, "The publisher must send the message."},
		{KindSentence, BasisImplicit, "It is answered with an e.g. empty reply."},
		{KindSentence, BasisAdmonition, "Use short topics."},
		{KindField, BasisField, "Value Range: `<String>`; Type: String; Requirement: Mandatory"},
		{KindTableRow, BasisKeyword, "Method: `Pub` | Level 1: Mandatory | Level 2: Mandatory"},
		{KindTableRow, BasisTable, "Method: `Set` | Level 1: – | Level 2: Where applicable"},
		{KindListItem, BasisKeyword, "A client can reconnect."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
	if items[3].Heading != "Field" || items[3].Keywords[0] != "MUST" {
		t.Errorf("field item: %+v", items[3])
	}
	if items[2].Keywords[0] != "SHOULD" || items[2].Admonition != "tip" {
		t.Errorf("admonition item: %+v", items[2])
	}
	if items[6].Keywords[0] != "MAY?" {
		t.Errorf("can must map to MAY?: %+v", items[6])
	}
}

func TestKeysStableAgainstLineShifts(t *testing.T) {
	a := Extract(file())
	f := file()
	f.Lines = append([]string{f.Lines[0], f.Lines[1], f.Lines[2], "", ""}, f.Lines[3:]...)
	b := Extract(f)
	for i := range a {
		if a[i].Key != b[i].Key {
			t.Fatalf("key changed for %q", a[i].Text)
		}
		if a[i].Line == b[i].Line {
			t.Fatalf("line should differ for %q", a[i].Text)
		}
	}
}

func TestDuplicateTextsGetDistinctKeys(t *testing.T) {
	f := File{Doc: "MG", Section: "1", Lines: []string{"A must be B.", "", "A must be B."}}
	items := Extract(f)
	if len(items) != 2 || items[0].Key == items[1].Key {
		t.Fatalf("expected two distinct keys, got %+v", items)
	}
}

func TestSplitSentences(t *testing.T) {
	got := SplitSentences("Use e.g. MQTT. Then `x. Y` stays. (Note) Last one? Yes.")
	want := []string{"Use e.g. MQTT.", "Then `x. Y` stays.", "(Note) Last one?", "Yes."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCleanInline(t *testing.T) {
	in := `See [7.1 Topic](../7.md) and ![img](a.png) **bold** *it* NORMAL\_0 a &lt;br/&gt; b`
	want := "See 7.1 Topic and bold it NORMAL_0 a b"
	if got := CleanInline(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCompareSections(t *testing.T) {
	in := []string{"A1", "10", "2", "1.10", "1.2", "index", "1"}
	want := []string{"index", "1", "1.2", "1.10", "2", "10", "A1"}
	for i := 0; i < len(in); i++ {
		for j := i + 1; j < len(in); j++ {
			if CompareSections(in[j], in[i]) < 0 {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
	if !reflect.DeepEqual(in, want) {
		t.Fatalf("got %v want %v", in, want)
	}
	if !SectionWithin("MG 7.2.1", "MG 7.2") || SectionWithin("MG 7.20", "MG 7.2") {
		t.Fatal("SectionWithin must respect section boundaries")
	}
}
