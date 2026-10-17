package render

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/catalog"
	"github.com/OI4/oec/spec/tools/internal/topic"
	"github.com/OI4/oec/spec/tools/internal/vectors"
)

func TestTopicForm(t *testing.T) {
	ts, err := topic.Load(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for rule, want := range map[string]string{
		"pub-data":                "Oi4v2/<ServiceType>/<AppId>/Pub/Data[/<Source>[/<Tag>]]",
		"set-publicationlist":     "Oi4v2/<ServiceType>/<AppId>/Set/PublicationList/<Source>/<ResourceType>[/<Tag>]",
		"pub-event":               "Oi4v2/<ServiceType>/<AppId>/Pub/Event/<Source>/<EventCategory>/<EventLevel>",
		"call-newdatasetwriterid": "Oi4v2/<ServiceType>/<AppId>/Call/NewDataSetWriterId[/<Source>]",
		"no-such-rule":            "no-such-rule",
	} {
		if got := topicForm(ts.Grammar, rule); got != want {
			t.Errorf("topicForm(%s) = %q, want %q", rule, got, want)
		}
	}
}

func TestCode(t *testing.T) {
	for in, want := range map[string]string{
		"abc":   "`abc`",
		"":      "_(empty)_",
		"a\tb":  "`\"a\\tb\"` (escaped)",
		" a":    "`\" a\"` (escaped)",
		"a`b":   "`` \"a`b\" `` (escaped)",
		"Data/": "`Data/`",
	} {
		if got := code(in); got != want {
			t.Errorf("code(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReviewViews(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	s, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := topic.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := vectors.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	views := ReviewViews(s, ts, files)
	if len(views) != len(files)+2 {
		t.Fatalf("got %d views, want %d", len(views), len(files)+2)
	}
	m := views[MatrixView]
	for _, c := range ts.Matrix.Combinations {
		if c.Allowed && !strings.Contains(m, "`"+c.Rule+"`") {
			t.Errorf("matrix view misses rule %s", c.Rule)
		}
	}
	for _, f := range files {
		v := views[VectorViewDir+"/"+vectorViewName(f.Path)]
		for _, vec := range f.Vectors {
			if !strings.Contains(v, "\n## "+vec.ID+"\n") {
				t.Errorf("%s: view misses %s", f.Path, vec.ID)
			}
		}
	}
}
