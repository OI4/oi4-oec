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
	s = strings.ReplaceAll(s, "**", "")
	s = escapePattern.ReplaceAllString(s, "$1")
	s = emphasisPattern.ReplaceAllString(s, "$1$2")
	s = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&", "&nbsp;", " ").Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// NormalizeForCompare makes text comparable independent of formatting:
// cleaned, without backticks and table pipes, typographic quotes unified.
func NormalizeForCompare(s string) string {
	s = CleanInline(s)
	s = strings.NewReplacer("`", "", "|", " ", "’", "'", "‘", "'", "“", `"`, "”", `"`, "–", "-", "—", "-").Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
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
