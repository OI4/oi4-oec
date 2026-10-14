package yamlstrict

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAccepts(t *testing.T) {
	src := `
id: REQ-CMP-001
count: 3
ratio: 0.5
flag: true
none: null
quoted: "yes"
date: "2026-10-14"
list:
  - a
  - "b"
text: |
  multi
  line
`
	got, err := Load([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"id": "REQ-CMP-001", "count": json.Number("3"), "ratio": json.Number("0.5"),
		"flag": true, "none": nil, "quoted": "yes", "date": "2026-10-14",
		"list": []any{"a", "b"}, "text": "multi\nline\n",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"anchor":          "a: &x 1\nb: 2\n",
		"alias":           "a: &x 1\nb: *x\n",
		"tag":             "a: !!str 1\n",
		"custom tag":      "a: !foo bar\n",
		"merge":           "base: {a: 1}\nother:\n  <<: {a: 1}\n",
		"duplicate":       "a: 1\na: 2\n",
		"int key":         "1: a\n",
		"yaml11 bool":     "a: yes\n",
		"yaml11 bool off": "a: Off\n",
		"timestamp":       "a: 2026-10-14\n",
		"hex":             "a: 0x1F\n",
		"octal":           "a: 0o17\n",
		"inf":             "a: .inf\n",
		"bool True":       "a: True\n",
		"null tilde":      "a: ~\n",
		"two docs":        "a: 1\n---\nb: 2\n",
		"empty":           "",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load([]byte(src)); err == nil {
				t.Fatalf("expected error for %q", src)
			}
		})
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	v, err := Load([]byte("id: x\nextra: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID string `json:"id"`
	}
	err = Decode(v, &out)
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}
