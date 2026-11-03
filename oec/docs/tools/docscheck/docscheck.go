// Package docscheck checks the pages of the OEC documentation site
// (oec/docs/docs, ADR-0031) without node and without network: the front
// matter of every page, every Markdown link and anchor (into the site and
// into the files of the repository folder oec/), and the rule of
// GUARDRAILS G-4.11 that the site has no hyperlinks to the guideline site.
package docscheck

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Problem is one finding, with the page relative to the docs folder.
type Problem struct {
	File string
	Line int
	Msg  string
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg)
	}
	return fmt.Sprintf("%s: %s", p.File, p.Msg)
}

// forbidden are the hosts and path parts of the guideline site and its
// repository (G-4.11: guideline sections are referenced by number).
var forbidden = []string{"oi4.github.io", "oi4-oec-documentation"}

// Check checks every page below docs (the folder oec/docs/docs). Links that
// leave the site must name an existing file or folder of the repository
// folder oec (the parent of docs/..); the site build turns them into
// repository links. Links to anything outside oec, to oec/docs itself
// outside the site, site-absolute links and unknown schemes are problems.
func Check(docs string) ([]Problem, error) {
	docs, err := filepath.Abs(docs)
	if err != nil {
		return nil, err
	}
	site := filepath.Dir(docs)
	oec := filepath.Dir(site)
	c := &checker{docs: docs, site: site, oec: oec, anchors: map[string]map[string]bool{}}
	var pages []string
	err = filepath.WalkDir(docs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isPage(p) {
			pages = append(pages, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return []Problem{{File: ".", Msg: "no pages"}}, nil
	}
	slices.Sort(pages)
	for _, p := range pages {
		if err := c.page(p); err != nil {
			return nil, err
		}
	}
	if !c.rootPage {
		c.problems = append(c.problems, Problem{File: ".", Msg: "no page with slug / (the root of the site)"})
	}
	return c.problems, nil
}

func isPage(p string) bool { return strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".mdx") }

type checker struct {
	docs, site, oec string
	anchors         map[string]map[string]bool
	problems        []Problem
	rootPage        bool
}

func (c *checker) add(file string, line int, format string, a ...any) {
	rel, err := filepath.Rel(c.docs, file)
	if err != nil {
		rel = file
	}
	c.problems = append(c.problems, Problem{File: filepath.ToSlash(rel), Line: line, Msg: fmt.Sprintf(format, a...)})
}

var (
	reLink    = regexp.MustCompile(`!?\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*<?([^)\s>]*)>?(?:\s+"[^"]*")?\s*\)`)
	reRefDef  = regexp.MustCompile(`(?m)^ {0,3}\[[^\]]+\]:\s*<?(\S+?)>?(?:\s+"[^"]*")?\s*$`)
	reHTMLRef = regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*["']([^"']*)["']`)
	reBareURL = regexp.MustCompile(`(?i)\bhttps?://[^\s)<>"'\]]+`)
	reKey     = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$`)
)

func (c *checker) page(file string) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	fm, body, bodyLine, ok := splitFrontMatter(text)
	if !ok {
		c.add(file, 1, "no front matter (--- … ---)")
	} else {
		c.frontMatter(file, fm)
	}
	if !strings.Contains(stripCode(body), "\n# ") && !strings.HasPrefix(stripCode(body), "# ") {
		c.add(file, 0, "no level-1 heading")
	}
	plain := stripCode(body)
	line := func(off int) int { return bodyLine + strings.Count(plain[:off], "\n") }
	var urls []struct {
		u   string
		off int
	}
	for _, m := range reLink.FindAllStringSubmatchIndex(plain, -1) {
		urls = append(urls, struct {
			u   string
			off int
		}{plain[m[2]:m[3]], m[0]})
	}
	for _, re := range []*regexp.Regexp{reRefDef, reHTMLRef} {
		for _, m := range re.FindAllStringSubmatchIndex(plain, -1) {
			urls = append(urls, struct {
				u   string
				off int
			}{plain[m[2]:m[3]], m[0]})
		}
	}
	for _, m := range reBareURL.FindAllStringIndex(plain, -1) {
		if guideline(plain[m[0]:m[1]]) {
			c.add(file, line(m[0]), "hyperlink to the guideline site %q (G-4.11: reference guideline sections by number)", plain[m[0]:m[1]])
		}
	}
	for _, u := range urls {
		c.link(file, line(u.off), u.u)
	}
	return nil
}

func guideline(u string) bool {
	l := strings.ToLower(u)
	for _, f := range forbidden {
		if strings.Contains(l, f) {
			return true
		}
	}
	return false
}

// splitFrontMatter returns the front matter lines, the body and the line
// number of the first body line.
func splitFrontMatter(text string) (fm []string, body string, bodyLine int, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, 1, false
	}
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return lines[1:i], strings.Join(lines[i+1:], "\n"), i + 2, true
		}
	}
	return nil, text, 1, false
}

func (c *checker) frontMatter(file string, fm []string) {
	keys := map[string]string{}
	for i, l := range fm {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "#") {
			continue
		}
		m := reKey.FindStringSubmatch(l)
		if m == nil {
			c.add(file, i+2, "front matter line %q is not `key: value`", l)
			continue
		}
		if _, dup := keys[m[1]]; dup {
			c.add(file, i+2, "front matter key %q twice", m[1])
		}
		keys[m[1]] = strings.Trim(strings.TrimSpace(m[2]), `"'`)
	}
	for _, k := range []string{"title", "description"} {
		if keys[k] == "" {
			c.add(file, 1, "front matter needs a non-empty %q", k)
		}
	}
	if v, ok := keys["sidebar_position"]; ok {
		if n, err := strconv.Atoi(v); err != nil || n < 0 {
			c.add(file, 1, "sidebar_position %q is not a non-negative integer", v)
		}
	}
	if v, ok := keys["slug"]; ok {
		if !strings.HasPrefix(v, "/") || strings.Contains(v, "..") {
			c.add(file, 1, "slug %q must start with / and not contain ..", v)
		}
		if v == "/" {
			if c.rootPage {
				c.add(file, 1, "a second page with slug /")
			}
			c.rootPage = true
		}
	}
}

func (c *checker) link(file string, line int, u string) {
	switch {
	case u == "":
		c.add(file, line, "empty link")
		return
	case guideline(u):
		c.add(file, line, "hyperlink to the guideline site %q (G-4.11: reference guideline sections by number)", u)
		return
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "mailto:"):
		return
	case strings.HasPrefix(u, "#"):
		c.anchor(file, line, file, u[1:], u)
		return
	case strings.HasPrefix(u, "/"):
		c.add(file, line, "site-absolute link %q: link the page file relatively", u)
		return
	case strings.Contains(u, ":"):
		c.add(file, line, "link %q: unsupported scheme", u)
		return
	case strings.Contains(u, "?"):
		c.add(file, line, "link %q: no query in file links", u)
		return
	}
	p, frag, _ := strings.Cut(u, "#")
	target := filepath.Join(filepath.Dir(file), filepath.FromSlash(p))
	inDocs := within(target, c.docs)
	if !inDocs && (within(target, c.site) || !within(target, c.oec)) {
		c.add(file, line, "link %q leaves the site and the repository folder oec/ (only pages of the site and files of oec/ outside oec/docs)", u)
		return
	}
	st, err := os.Stat(target)
	if err != nil {
		c.add(file, line, "link %q: %s does not exist", u, c.relOEC(target))
		return
	}
	if inDocs && st.IsDir() {
		c.add(file, line, "link %q names a folder: link its page file (index.md)", u)
		return
	}
	if frag != "" {
		if st.IsDir() || !isPage(target) {
			c.add(file, line, "link %q: an anchor needs a Markdown target", u)
			return
		}
		c.anchor(file, line, target, frag, u)
	}
}

func (c *checker) relOEC(p string) string {
	if r, err := filepath.Rel(filepath.Dir(c.oec), p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

func within(p, dir string) bool {
	r, err := filepath.Rel(dir, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

func (c *checker) anchor(file string, line int, target, frag, u string) {
	ids, ok := c.anchors[target]
	if !ok {
		raw, err := os.ReadFile(target)
		if err != nil {
			c.add(file, line, "link %q: %v", u, err)
			return
		}
		_, body, _, _ := splitFrontMatter(strings.ReplaceAll(string(raw), "\r\n", "\n"))
		ids = Anchors(body)
		c.anchors[target] = ids
	}
	if !ids[frag] {
		c.add(file, line, "link %q: no heading with the anchor #%s in %s", u, frag, c.relOEC(target))
	}
}

var (
	reHeading  = regexp.MustCompile(`(?m)^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)
	reCustomID = regexp.MustCompile(`\s*\{#([^}\s]+)\}\s*$`)
	reHTMLID   = regexp.MustCompile(`(?i)<a\s[^>]*\b(?:id|name)\s*=\s*["']([^"']+)["']`)
	reMDLink   = regexp.MustCompile(`!?\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\([^)]*\)`)
	reTag      = regexp.MustCompile(`<[^>]+>`)
)

// Anchors returns the anchors of the headings of a Markdown body as GitHub
// and Docusaurus derive them (github-slugger, with -1, -2 … for repeated
// slugs; an explicit {#id} wins), plus the id/name of HTML anchors.
func Anchors(body string) map[string]bool {
	ids := map[string]bool{}
	seen := map[string]int{}
	plain := stripCodeBlocks(body)
	for _, m := range reHeading.FindAllStringSubmatch(plain, -1) {
		text := m[2]
		if cm := reCustomID.FindStringSubmatch(text); cm != nil {
			ids[cm[1]] = true
			continue
		}
		s := Slug(headingText(text))
		if n, dup := seen[s]; dup {
			seen[s] = n + 1
			s = s + "-" + strconv.Itoa(n+1)
			for ids[s] {
				seen[s]++
				s = s + "-" + strconv.Itoa(seen[s])
			}
		} else {
			seen[s] = 0
		}
		ids[s] = true
	}
	for _, m := range reHTMLID.FindAllStringSubmatch(plain, -1) {
		ids[m[1]] = true
	}
	return ids
}

// headingText is the text a reader sees: the content of code spans, the
// text of links, no emphasis markers and no HTML tags.
func headingText(s string) string {
	s = reMDLink.ReplaceAllString(s, "$1")
	s = reTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.TrimSpace(s)
}

// Slug is github-slugger: lower case, every character that is not a
// letter, a mark, a digit, a connector, '-' or a space removed, spaces
// replaced by '-'.
func Slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-', r == '_', unicode.IsLetter(r), unicode.IsMark(r), unicode.Is(unicode.Nd, r), unicode.Is(unicode.Nl, r), unicode.Is(unicode.No, r), unicode.Is(unicode.Pc, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

var reFence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// stripCodeBlocks blanks fenced code blocks (keeping the line count).
func stripCodeBlocks(s string) string {
	lines := strings.Split(s, "\n")
	fence := ""
	for i, l := range lines {
		if fence != "" {
			lines[i] = ""
			if m := reFence.FindStringSubmatch(l); m != nil && m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), string(fence[0]))) == "" {
				fence = ""
			}
			continue
		}
		if m := reFence.FindStringSubmatch(l); m != nil {
			fence = m[1]
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// stripCode blanks fenced code blocks and code spans (keeping offsets of
// lines), so their content is neither a link nor a URL.
func stripCode(s string) string {
	s = stripCodeBlocks(s)
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '`' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		run := s[i:j]
		end := strings.Index(s[j:], run)
		if end < 0 {
			b.WriteString(run)
			i = j
			continue
		}
		for _, ch := range s[i : j+end+len(run)] {
			if ch == '\n' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
		}
		i = j + end + len(run)
	}
	return b.String()
}

// Pages lists the pages below docs relative to it, in slash form.
func Pages(docs string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(docs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isPage(p) {
			r, _ := filepath.Rel(docs, p)
			out = append(out, path.Clean(filepath.ToSlash(r)))
		}
		return nil
	})
	slices.Sort(out)
	return out, err
}
