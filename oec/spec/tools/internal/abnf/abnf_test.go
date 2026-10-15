package abnf

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

const testGrammar = `
; comment line
greeting = %s"Hi" SP name   ; case-sensitive
           [ "!" ]
name     = 1*8( ALPHA / "-" ) / %x22 1*DIGIT %x22
word     = "abc"
word     =/ %x31.32.33
wide     = %x20-22 / %x24-10FFFF
enc      = 1*( wide / %s"%24" )
both     = "a" / "ab"
pair     = both "b"
quads    = *3(2ALPHA) [DIGIT]
`

func mustParse(t *testing.T, src string) *Grammar {
	t.Helper()
	g, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestMatch(t *testing.T) {
	g := mustParse(t, testGrammar)
	cases := []struct {
		rule, in string
		want     bool
	}{
		{"greeting", "Hi Bob", true},
		{"greeting", "Hi Bob!", true},
		{"greeting", "hi Bob", false},
		{"greeting", "Hi \"42\"", true},
		{"greeting", "Hi abcdefghi", false},
		{"word", "ABC", true},
		{"word", "123", true},
		{"word", "12", false},
		{"enc", "a b\u00e4\U0001F600#", false},
		{"enc", "a b\u00e4\U0001F600%24", true},
		{"pair", "ab", true},
		{"pair", "abb", true},
		{"PAIR", "abbb", false},
	}
	for _, c := range cases {
		got, err := g.Match(c.rule, c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("Match(%s, %q) = %v, want %v", c.rule, c.in, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"a = b",
		"a = b\nb = a",
		"a = <prose>",
		"a = \"x\"\na = \"y\"",
		"a =/ \"x\"",
		"a = 3*2\"x\"",
		"a = (\"x\"",
		"a = %q41",
		"= \"x\"",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("Parse(%q): expected error", src)
		}
	}
}

func TestRegexText(t *testing.T) {
	g := mustParse(t, testGrammar)
	want := map[string]string{
		"word":  `^(?:[Aa][Bb][Cc]|123)$`,
		"wide":  `^(?:[^\x00-\x1F#])$`,
		"name":  `^(?:[\x2DA-Za-z]{1,8}|"[0-9]+")$`,
		"quads": `^(?:(?:[A-Za-z]{2}){0,3}[0-9]?)$`,
	}
	for rule, w := range want {
		got, err := g.Regex(rule)
		if err != nil {
			t.Fatal(err)
		}
		if got != w {
			t.Errorf("Regex(%s) = %s, want %s", rule, got, w)
		}
	}
}

// TestRegexAgreesWithMatcher cross-checks generated regexes against the
// interpreter on random samples and on mutated samples.
func TestRegexAgreesWithMatcher(t *testing.T) {
	g := mustParse(t, testGrammar)
	gen := &Generator{G: g, Rand: rand.New(rand.NewSource(1))}
	for _, rule := range g.Order {
		src, err := g.Regex(rule)
		if err != nil {
			t.Fatal(err)
		}
		re := regexp.MustCompile(src)
		for i := 0; i < 300; i++ {
			s, err := gen.Generate(rule)
			if err != nil {
				t.Fatal(err)
			}
			ok, _ := g.Match(rule, s)
			if !ok || !re.MatchString(s) {
				t.Fatalf("%s: sample %q: matcher=%v regex=%v", rule, s, ok, re.MatchString(s))
			}
			for _, m := range mutate(gen.Rand, s) {
				ok, _ := g.Match(rule, m)
				if ok != re.MatchString(m) {
					t.Fatalf("%s: mutant %q: matcher=%v regex=%v", rule, m, ok, !ok)
				}
			}
		}
	}
}

func mutate(r *rand.Rand, s string) []string {
	rs := []rune(s)
	alphabet := []rune("aZ09 -!\"$%#/\x00\u00e4\U0001F600")
	var out []string
	if len(rs) > 0 {
		i := r.Intn(len(rs))
		out = append(out, string(rs[:i])+string(rs[i+1:]))
		c := append([]rune(nil), rs...)
		c[i] = alphabet[r.Intn(len(alphabet))]
		out = append(out, string(c))
	}
	i := r.Intn(len(rs) + 1)
	out = append(out, string(rs[:i])+string(alphabet[r.Intn(len(alphabet))])+string(rs[i:]))
	return append(out, strings.ToUpper(s), strings.ToLower(s))
}

func TestRegexRejectsUnrepresentable(t *testing.T) {
	g := mustParse(t, "a = %x100\nb = %x00-10FFFF\nc = 1001\"x\"")
	for _, r := range []string{"a", "b", "c"} {
		if _, err := g.Regex(r); err == nil {
			t.Errorf("Regex(%s): expected error", r)
		}
	}
}
