package docscheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSitePages is the gate: the pages of oec/docs/docs pass.
func TestSitePages(t *testing.T) {
	problems, err := Check("../../docs")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
	pages, err := Pages("../../docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 6 {
		t.Fatalf("only %d pages: %v", len(pages), pages)
	}
	// The getting-started page is the root of the site (slug /), otherwise
	// the Docusaurus build reports "/" as a broken link.
	if !strings.Contains(read(t, "../../docs/intro.md"), "\nslug: /\n") {
		t.Error("docs/intro.md needs slug: /")
	}
}

// TestSiteLinksSeen proves the link scanner sees the README links of the
// existing pages (a silent scanner would pass everything).
func TestSiteLinksSeen(t *testing.T) {
	body := read(t, "../../docs/registry/index.md")
	n := len(reLink.FindAllString(stripCode(body), -1))
	if n < 5 {
		t.Fatalf("registry/index.md: %d links seen", n)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Image (ADR-0029 §12)":                  "image-adr-0029-12",
		"Compose profile `registry` (C-090)":    "compose-profile-registry-c-090",
		"HTTP: REST, SSE, TLS (ADR-0029 §7)":    "http-rest-sse-tls-adr-0029-7",
		"With the OEC Registry":                 "with-the-oec-registry",
		"Images (C-084)":                        "images-c-084",
		"Größe und Ü":                           "größe-und-ü",
		"a_b – c.d":                             "a_b--cd",
		"[Link text](x.md) and **bold**":        "link-text-and-bold",
		"Run `oec-check monitor` (D7) and more": "run-oec-check-monitor-d7-and-more",
	} {
		if got := Slug(headingText(in)); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnchors(t *testing.T) {
	ids := Anchors("# Title\n\n## Usage\n\n```\n## not a heading\n```\n\n## Usage\n\n## Usage\n\n### Custom {#my-id}\n\n<a id=\"html-id\"></a>\n")
	for _, want := range []string{"title", "usage", "usage-1", "usage-2", "my-id", "html-id"} {
		if !ids[want] {
			t.Errorf("anchor %q missing in %v", want, ids)
		}
	}
	if ids["not-a-heading"] || ids["custom"] {
		t.Errorf("unexpected anchors %v", ids)
	}
}

// tree builds oec/{docs/docs,dev}/… in a temporary folder and returns the
// docs folder.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	base := map[string]string{
		"oec/dev/README.md":            "# Dev\n\n## Compose profile `demos` (C-084)\n",
		"oec/dev/compose.yaml":         "services: {}\n",
		"oec/docs/README.md":           "# Docs\n",
		"outside.md":                   "# Outside\n",
		"oec/docs/docs/intro.md":       "---\ntitle: Start\ndescription: d\nslug: /\n---\n\n# Start\n\n[Demos](demos/index.md)\n",
		"oec/docs/docs/demos/index.md": "---\ntitle: Demos\nsidebar_position: 1\ndescription: d\n---\n\n# Demos\n\n## Run all three\n",
	}
	for k, v := range files {
		base[k] = v
	}
	for k, v := range base {
		if v == "" {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(k))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "oec", "docs", "docs")
}

func page(body string) string {
	return "---\ntitle: P\ndescription: d\n---\n\n# P\n\n" + body + "\n"
}

func TestCheckGood(t *testing.T) {
	docs := tree(t, map[string]string{"oec/docs/docs/p.md": page(strings.Join([]string{
		"[a](intro.md) [b](demos/index.md#run-all-three) [c](#p)",
		"[d](../../dev/README.md#compose-profile-demos-c-084) [e](../../dev/compose.yaml) [f](../../dev/)",
		"[g](https://example.org/x) [h](mailto:x@example.org) ![i](demos/index.md)",
		"`[not a link](nowhere.md)`",
		"```\n[not a link](nowhere.md) https://oi4.github.io/oi4-oec-documentation/\n```",
		"[ref]: ../../dev/README.md",
	}, "\n"))})
	problems, err := Check(docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestCheckBad(t *testing.T) {
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"no front matter":     {map[string]string{"oec/docs/docs/p.md": "# P\n"}, "no front matter"},
		"no title":            {map[string]string{"oec/docs/docs/p.md": "---\ndescription: d\n---\n\n# P\n"}, `non-empty "title"`},
		"no description":      {map[string]string{"oec/docs/docs/p.md": "---\ntitle: P\n---\n\n# P\n"}, `non-empty "description"`},
		"bad position":        {map[string]string{"oec/docs/docs/p.md": "---\ntitle: P\ndescription: d\nsidebar_position: x\n---\n\n# P\n"}, "sidebar_position"},
		"no h1":               {map[string]string{"oec/docs/docs/p.md": "---\ntitle: P\ndescription: d\n---\n\n## P\n"}, "level-1 heading"},
		"second root":         {map[string]string{"oec/docs/docs/p.md": "---\ntitle: P\ndescription: d\nslug: /\n---\n\n# P\n"}, "second page with slug /"},
		"missing page":        {map[string]string{"oec/docs/docs/p.md": page("[x](nope.md)")}, "does not exist"},
		"missing anchor":      {map[string]string{"oec/docs/docs/p.md": page("[x](demos/index.md#nope)")}, "no heading with the anchor #nope"},
		"missing own anchor":  {map[string]string{"oec/docs/docs/p.md": page("[x](#nope)")}, "no heading with the anchor #nope"},
		"missing repo file":   {map[string]string{"oec/docs/docs/p.md": page("[x](../../dev/nope.md)")}, "does not exist"},
		"missing repo anchor": {map[string]string{"oec/docs/docs/p.md": page("[x](../../dev/README.md#nope)")}, "no heading with the anchor"},
		"anchor in non-md":    {map[string]string{"oec/docs/docs/p.md": page("[x](../../dev/compose.yaml#x)")}, "needs a Markdown target"},
		"outside oec":         {map[string]string{"oec/docs/docs/p.md": page("[x](../../../outside.md)")}, "leaves the site"},
		"site folder":         {map[string]string{"oec/docs/docs/p.md": page("[x](../README.md)")}, "leaves the site"},
		"folder in docs":      {map[string]string{"oec/docs/docs/p.md": page("[x](demos/)")}, "names a folder"},
		"absolute":            {map[string]string{"oec/docs/docs/p.md": page("[x](/demos)")}, "site-absolute"},
		"scheme":              {map[string]string{"oec/docs/docs/p.md": page("[x](ftp://x)")}, "unsupported scheme"},
		"empty":               {map[string]string{"oec/docs/docs/p.md": page("[x]()")}, "empty link"},
		"guideline link":      {map[string]string{"oec/docs/docs/p.md": page("[MG](https://oi4.github.io/oi4-oec-documentation/docs/guideline/)")}, "G-4.11"},
		"guideline bare":      {map[string]string{"oec/docs/docs/p.md": page("see https://github.com/OI4/oi4-oec-documentation")}, "G-4.11"},
		"guideline ref":       {map[string]string{"oec/docs/docs/p.md": page("[mg]: https://oi4.github.io/x")}, "G-4.11"},
		"guideline html":      {map[string]string{"oec/docs/docs/p.md": page(`<a href="https://oi4.github.io/x">MG</a>`)}, "G-4.11"},
		"no root page":        {map[string]string{"oec/docs/docs/intro.md": "---\ntitle: Start\ndescription: d\n---\n\n# Start\n"}, "no page with slug /"},
		"bad ref target":      {map[string]string{"oec/docs/docs/p.md": page("[r]: nope.md")}, "does not exist"},
	} {
		t.Run(name, func(t *testing.T) {
			problems, err := Check(tree(t, c.files))
			if err != nil {
				t.Fatal(err)
			}
			var all []string
			for _, p := range problems {
				all = append(all, p.String())
			}
			if !strings.Contains(strings.Join(all, "\n"), c.want) {
				t.Fatalf("want a problem with %q, got %q", c.want, all)
			}
		})
	}
}

func TestLineNumbers(t *testing.T) {
	problems, err := Check(tree(t, map[string]string{"oec/docs/docs/p.md": page("line 8\n\n[x](nope.md)")}))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].File != "p.md" || problems[0].Line != 10 {
		t.Fatalf("got %v", problems)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
