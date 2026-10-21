package guideline

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	imagePattern    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkPattern     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	escapePattern   = regexp.MustCompile(`\\([\\_*\[\]()#+\-.!|{}<>])`)
	emphasisPattern = regexp.MustCompile(`(^|[\s(\[/"'])\*([^*\s](?:[^*]*[^*\s])?)\*`)
	htmlBreak       = regexp.MustCompile(`(?i)(&lt;|<)br\s*/?(&gt;|>)`)
	spaces          = regexp.MustCompile(`\s+`)
)

// CleanInline removes Markdown inline formatting that is irrelevant for the
// wording (images, link targets, escapes, emphasis, HTML breaks) and collapses
// whitespace. Inline code backticks are kept.
func CleanInline(s string) string {
	s = imagePattern.ReplaceAllString(s, "")
	for i := 0; i < 3 && linkPattern.MatchString(s); i++ {
		s = linkPattern.ReplaceAllString(s, "$1")
	}
	s = htmlBreak.ReplaceAllString(s, " ")
	s = outsideCode(s, func(t string) string {
		return strings.ReplaceAll(t, "**", "")
	})
	s = escapePattern.ReplaceAllString(s, "$1")
	s = outsideCode(s, func(t string) string {
		return emphasisPattern.ReplaceAllString(t, "$1$2")
	})
	s = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&", "&nbsp;", " ").Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// outsideCode applies f to the parts of s outside inline code spans, so that
// emphasis markers are not stripped from code such as `********`. An
// unbalanced backtick leaves the rest of s to f.
func outsideCode(s string, f func(string) string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i+1:], '`')
		if j < 0 {
			break
		}
		b.WriteString(f(s[:i]))
		b.WriteString(s[i : i+j+2])
		s = s[i+j+2:]
	}
	b.WriteString(f(s))
	return b.String()
}

// NormalizeForCompare makes text comparable independent of formatting:
// cleaned, without backticks and table pipes, typographic quotes unified.
func NormalizeForCompare(s string) string {
	s = CleanInline(s)
	s = strings.NewReplacer("`", "", "|", " ", "’", "'", "‘", "'", "“", `"`, "”", `"`, "–", "-", "—", "-").Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// NormalizeCodeForCompare normalizes the content of a code block (e.g. a JSON
// payload) like SectionTexts does: code is not stripped of emphasis markers,
// so values such as "********" survive.
func NormalizeCodeForCompare(s string) string {
	return NormalizeForCompare("`" + s + "`")
}

var abbreviations = map[string]bool{
	"e.g": true, "i.e": true, "etc": true, "vs": true, "cf": true, "fig": true,
	"no": true, "nr": true, "approx": true, "incl": true, "resp": true, "min": true, "max": true,
}

// SplitSentences splits a cleaned paragraph into sentences. A sentence ends
// at '.', '!' or '?' followed by whitespace and an uppercase letter, a
// backtick or an opening bracket – outside inline code and not after a
// known abbreviation.
func SplitSentences(p string) []string {
	var out []string
	inCode := false
	start := 0
	for i := 0; i < len(p); {
		r, size := utf8.DecodeRuneInString(p[i:])
		if r == '`' {
			inCode = !inCode
		}
		if !inCode && (r == '.' || r == '!' || r == '?') {
			j := i + size
			k := j
			for k < len(p) && p[k] == ' ' {
				k++
			}
			if k > j && k < len(p) {
				next, _ := utf8.DecodeRuneInString(p[k:])
				if (unicode.IsUpper(next) || next == '`' || next == '(' || next == '"') &&
					!(r == '.' && endsWithAbbreviation(p[start:i])) {
					out = append(out, strings.TrimSpace(p[start:j]))
					start = k
					i = k
					continue
				}
			}
		}
		i += size
	}
	if rest := strings.TrimSpace(p[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func endsWithAbbreviation(s string) bool {
	idx := strings.LastIndexAny(s, " (")
	word := strings.ToLower(s[idx+1:])
	return abbreviations[word]
}
