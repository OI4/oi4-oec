package docscheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The gate: the pages of the Community section of the guideline site
// (oi4-oec-documentation/docs/community, next to this repository) and the
// links of the READMEs of oec/ to them pass.
const (
	communityDir = "../../../../oi4-oec-documentation/docs/community"
	oecDir       = "../../.."
)

func TestSitePages(t *testing.T) {
	problems, err := Check(communityDir, oecDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
	pages, err := Pages(communityDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 9 {
		t.Fatalf("only %d pages: %v", len(pages), pages)
	}
}

func TestReadmeLinks(t *testing.T) {
	problems, n, err := CheckReadmes(oecDir, communityDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
	if n < 5 {
		t.Fatalf("only %d README links to the Community pages seen", n)
	}
}

// TestSiteLinksSeen proves the link scanner sees the repository links of
// the existing pages (a silent scanner would pass everything).
func TestSiteLinksSeen(t *testing.T) {
	body := read(t, communityDir+"/registry/index.md")
	n := strings.Count(stripCode(body), "]("+RepoURL)
	if n < 5 {
		t.Fatalf("registry/index.md: %d repository links seen", n)
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

// tree builds oec/… and guide/docs/{community,guideline}/… in a temporary
// folder and returns the community and the oec folder.
func tree(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	base := map[string]string{
		"oec/dev/README.md":                     "# Dev\n\n## Compose profile `demos` (C-084)\n",
		"oec/dev/compose.yaml":                  "services: {}\n",
		"outside.md":                            "# Outside\n",
		"guide/docs/guideline/mg.md":            "# MG\n",
		"guide/docs/community/index.md":         "---\ntitle: Start\ndescription: d\n---\n\n# Start\n\n[Demos](demos/index.md)\n",
		"guide/docs/community/demos/index.md":   "---\ntitle: Demos\nsidebar_position: 1\ndescription: d\n---\n\n# Demos\n\n## Run all three\n",
		"guide/docs/community/demos/weather.md": "---\ntitle: Weather\ndescription: d\n---\n\n# Weather\n",
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
	return filepath.Join(root, "guide", "docs", "community"), filepath.Join(root, "oec")
}

func page(body string) string {
	return "---\ntitle: P\ndescription: d\n---\n\n# P\n\n" + body + "\n"
}

func TestCheckGood(t *testing.T) {
	docs, oec := tree(t, map[string]string{"guide/docs/community/p.md": page(strings.Join([]string{
		"[a](index.md) [b](demos/index.md#run-all-three) [c](#p)",
		"[d](" + RepoURL + "dev/README.md#compose-profile-demos-c-084) [e](" + RepoURL + "dev/compose.yaml) [f](" + RepoURL + "dev)",
		"[g](https://example.org/x) [h](mailto:x@example.org) ![i](demos/index.md)",
		"`[not a link](nowhere.md)` `{x}` `<T>`",
		"```\n[not a link](nowhere.md) https://oi4.github.io/oi4-oec-documentation/ {x} <T>\n```",
		"a < b, a <= b, 1 > 0, -> and <- are text",
		":::note\n\nA note.\n\n:::",
		"[ref]: " + RepoURL + "dev/README.md",
	}, "\n"))})
	problems, err := Check(docs, oec)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestCheckBad(t *testing.T) {
	const p = "guide/docs/community/p.md"
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"no front matter":     {map[string]string{p: "# P\n"}, "no front matter"},
		"no title":            {map[string]string{p: "---\ndescription: d\n---\n\n# P\n"}, `non-empty "title"`},
		"no description":      {map[string]string{p: "---\ntitle: P\n---\n\n# P\n"}, `non-empty "description"`},
		"bad position":        {map[string]string{p: "---\ntitle: P\ndescription: d\nsidebar_position: x\n---\n\n# P\n"}, "sidebar_position"},
		"slug":                {map[string]string{p: "---\ntitle: P\ndescription: d\nslug: /p\n---\n\n# P\n"}, "no slug"},
		"no h1":               {map[string]string{p: "---\ntitle: P\ndescription: d\n---\n\n## P\n"}, "level-1 heading"},
		"missing page":        {map[string]string{p: page("[x](nope.md)")}, "does not exist"},
		"missing anchor":      {map[string]string{p: page("[x](demos/index.md#nope)")}, "no heading with the anchor #nope"},
		"missing own anchor":  {map[string]string{p: page("[x](#nope)")}, "no heading with the anchor #nope"},
		"missing repo file":   {map[string]string{p: page("[x](" + RepoURL + "dev/nope.md)")}, "does not exist"},
		"missing repo anchor": {map[string]string{p: page("[x](" + RepoURL + "dev/README.md#nope)")}, "no heading with the anchor"},
		"anchor in non-md":    {map[string]string{p: page("[x](" + RepoURL + "dev/compose.yaml#x)")}, "needs a Markdown target"},
		"repo escape":         {map[string]string{p: page("[x](" + RepoURL + "../outside.md)")}, "leaves oec/"},
		"other repo url":      {map[string]string{p: page("[x](https://github.com/OI4/oi4-oec/tree/main/oec/dev)")}, "link files of oec/ as"},
		"relative into oec":   {map[string]string{p: page("[x](../../../oec/dev/README.md)")}, "leaves the Community section"},
		"relative guideline":  {map[string]string{p: page("[x](../guideline/mg.md)")}, "leaves the Community section"},
		"folder in docs":      {map[string]string{p: page("[x](demos/)")}, "names a folder"},
		"absolute":            {map[string]string{p: page("[x](/docs/community)")}, "site-absolute"},
		"scheme":              {map[string]string{p: page("[x](ftp://x)")}, "unsupported scheme"},
		"empty":               {map[string]string{p: page("[x]()")}, "empty link"},
		"guideline link":      {map[string]string{p: page("[MG](https://oi4.github.io/oi4-oec-documentation/docs/guideline/)")}, "G-4.11"},
		"guideline bare":      {map[string]string{p: page("see https://github.com/OI4/oi4-oec-documentation")}, "G-4.11"},
		"guideline ref":       {map[string]string{p: page("[mg]: https://oi4.github.io/x")}, "G-4.11"},
		"guideline html":      {map[string]string{p: page(`<a href="https://oi4.github.io/x">MG</a>`)}, "G-4.11"},
		"no root page":        {map[string]string{"guide/docs/community/index.md": ""}, "no index.md"},
		"bad ref target":      {map[string]string{p: page("[r]: nope.md")}, "does not exist"},
		"mdx brace":           {map[string]string{p: page("a {placeholder} here")}, "MDX"},
		"mdx tag":             {map[string]string{p: page("a <AppId> here")}, "MDX"},
		"mdx autolink":        {map[string]string{p: page("see <https://example.org>")}, "MDX"},
	} {
		t.Run(name, func(t *testing.T) {
			docs, oec := tree(t, c.files)
			problems, err := Check(docs, oec)
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

func TestReadmesGood(t *testing.T) {
	docs, oec := tree(t, map[string]string{"oec/dev/README.md": "# Dev\n\nSee [site](" + SiteURL + "), [demos](" + SiteURL + "/demos), [run](" + SiteURL + "/demos#run-all-three), [w](" + SiteURL + "/demos/weather).\n\n`" + SiteURL + "/nope` in code\n"})
	problems, n, err := CheckReadmes(oec, docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
	if n != 4 {
		t.Fatalf("%d links seen, want 4", n)
	}
}

func TestReadmesBad(t *testing.T) {
	for name, c := range map[string]struct{ link, want string }{
		"missing page":   {SiteURL + "/nope", "no page"},
		"missing anchor": {SiteURL + "/demos#nope", "no heading with the anchor #nope"},
		"md suffix":      {SiteURL + "/demos/weather.md", "no page"},
		"trailing slash": {SiteURL + "/demos/", "trailing slash"},
		"old site":       {"../docs/docs/registry/index.md", "moved"},
	} {
		t.Run(name, func(t *testing.T) {
			docs, oec := tree(t, map[string]string{"oec/dev/README.md": "# Dev\n\n[x](" + c.link + ")\n"})
			problems, _, err := CheckReadmes(oec, docs)
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
	problems, err := Check(tree(t, map[string]string{"guide/docs/community/p.md": page("line 8\n\n[x](nope.md)")}))
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
