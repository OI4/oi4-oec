package abnf

import (
	"fmt"
	"sort"
	"strings"
)

// Regex compiles a rule into an anchored regular expression ^(?:...)$ in the
// common dialect of ECMA-262 (with the u flag), RE2 (Go) and Rust regex:
// only non-capturing groups, character classes, bounded and unbounded
// quantifiers and \xHH escapes; no look-around, back-references or
// named groups. Character classes that cover more than U+00FF are emitted as
// negated classes of their complement (UTF-16 surrogates count as part of the
// set, they cannot occur in UTF-8 text). Rules must not be recursive.
func (g *Grammar) Regex(rule string) (string, error) {
	key := strings.ToLower(rule)
	if _, ok := g.Rules[key]; !ok {
		return "", fmt.Errorf("unknown rule %s", rule)
	}
	c := &compiler{g: g}
	body, _, err := c.compile(Ref{Name: key})
	if err != nil {
		return "", fmt.Errorf("rule %s: %w", rule, err)
	}
	return "^(?:" + body + ")$", nil
}

type prec int

const (
	precAtom prec = iota
	precConcat
	precAlt
)

type compiler struct{ g *Grammar }

// maxCount is the largest repetition count accepted by RE2 and Rust.
const maxCount = 1000

func (c *compiler) compile(n Node) (string, prec, error) {
	if set, ok := c.charset(n); ok {
		s, err := class(set)
		return s, precAtom, err
	}
	switch v := n.(type) {
	case Ref:
		return c.compile(c.g.Rules[v.Name].Node)
	case Literal:
		var b strings.Builder
		n := 0
		for _, r := range v.Text {
			n++
			if !v.CaseSensitive && isASCIILetter(r) {
				s, _ := class(normalize([]Range{{lower(r), lower(r)}, {upper(r), upper(r)}}))
				b.WriteString(s)
				continue
			}
			s, err := escapeOutside(r)
			if err != nil {
				return "", 0, err
			}
			b.WriteString(s)
		}
		if n == 1 {
			return b.String(), precAtom, nil
		}
		return b.String(), precConcat, nil
	case Concat:
		var b strings.Builder
		for _, it := range v.Items {
			s, p, err := c.compile(it)
			if err != nil {
				return "", 0, err
			}
			if p == precAlt {
				s = "(?:" + s + ")"
			}
			b.WriteString(s)
		}
		return b.String(), precConcat, nil
	case Alt:
		parts := make([]string, 0, len(v.Items))
		for _, it := range v.Items {
			s, _, err := c.compile(it)
			if err != nil {
				return "", 0, err
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, "|"), precAlt, nil
	case Repeat:
		if v.Max == 0 {
			return "", precConcat, nil
		}
		s, p, err := c.compile(v.Item)
		if err != nil {
			return "", 0, err
		}
		if v.Min == 1 && v.Max == 1 {
			return s, p, nil
		}
		if v.Min > maxCount || v.Max > maxCount {
			return "", 0, fmt.Errorf("repetition count above %d", maxCount)
		}
		if p != precAtom {
			s = "(?:" + s + ")"
		}
		switch {
		case v.Min == 0 && v.Max == 1:
			s += "?"
		case v.Min == 0 && v.Max < 0:
			s += "*"
		case v.Min == 1 && v.Max < 0:
			s += "+"
		case v.Max < 0:
			s += fmt.Sprintf("{%d,}", v.Min)
		case v.Min == v.Max:
			s += fmt.Sprintf("{%d}", v.Min)
		default:
			s += fmt.Sprintf("{%d,%d}", v.Min, v.Max)
		}
		// A quantified atom must be grouped before it is quantified again.
		return s, precConcat, nil
	}
	return "", 0, fmt.Errorf("cannot compile %T", n)
}

// charset returns the code point set of n if n matches exactly one character.
func (c *compiler) charset(n Node) ([]Range, bool) {
	switch v := n.(type) {
	case Range:
		return []Range{v}, true
	case Literal:
		r := []rune(v.Text)
		if len(r) != 1 {
			return nil, false
		}
		if !v.CaseSensitive && isASCIILetter(r[0]) {
			return normalize([]Range{{lower(r[0]), lower(r[0])}, {upper(r[0]), upper(r[0])}}), true
		}
		return []Range{{r[0], r[0]}}, true
	case Ref:
		return c.charset(c.g.Rules[v.Name].Node)
	case Alt:
		var all []Range
		for _, it := range v.Items {
			s, ok := c.charset(it)
			if !ok {
				return nil, false
			}
			all = append(all, s...)
		}
		return normalize(all), true
	case Repeat:
		if v.Min == 1 && v.Max == 1 {
			return c.charset(v.Item)
		}
	}
	return nil, false
}

func normalize(rs []Range) []Range {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Lo < rs[j].Lo })
	var out []Range
	for _, r := range rs {
		if len(out) > 0 && r.Lo <= out[len(out)-1].Hi+1 {
			if r.Hi > out[len(out)-1].Hi {
				out[len(out)-1].Hi = r.Hi
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

const (
	surrogateLo = 0xD800
	surrogateHi = 0xDFFF
	maxRune     = 0x10FFFF
)

// class emits a character class (or a single escaped character).
func class(set []Range) (string, error) {
	set = normalize(set)
	if len(set) == 1 && set[0].Lo == set[0].Hi {
		return escapeOutside(set[0].Lo)
	}
	negated := false
	if set[len(set)-1].Hi > 0xFF {
		set = complement(normalize(append(append([]Range(nil), set...), Range{surrogateLo, surrogateHi})))
		negated = true
		if len(set) == 0 {
			return "", fmt.Errorf("character set covers all code points")
		}
		if set[len(set)-1].Hi > 0xFF {
			return "", fmt.Errorf("character set and its complement both exceed U+00FF")
		}
	}
	var b strings.Builder
	b.WriteByte('[')
	if negated {
		b.WriteByte('^')
	}
	for _, r := range set {
		b.WriteString(escapeInside(r.Lo))
		switch {
		case r.Hi == r.Lo:
		case r.Hi == r.Lo+1:
			b.WriteString(escapeInside(r.Hi))
		default:
			b.WriteByte('-')
			b.WriteString(escapeInside(r.Hi))
		}
	}
	b.WriteByte(']')
	return b.String(), nil
}

func complement(set []Range) []Range {
	var out []Range
	next := rune(0)
	for _, r := range set {
		if r.Lo > next {
			out = append(out, Range{next, r.Lo - 1})
		}
		next = r.Hi + 1
	}
	if next <= maxRune {
		out = append(out, Range{next, maxRune})
	}
	return out
}

func isAlnum(r rune) bool { return isASCIILetter(r) || r >= '0' && r <= '9' }

func isASCIILetter(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }

func lower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// Characters that are literal without escaping in all three dialects.
const (
	safeOutside = " !\"#%&',-/:;<=>@_`~"
	safeInside  = " !\"#%',/:;<=>@_`"
	// Syntax characters: a backslash escape is valid in all three dialects
	// (ECMA-262 u-mode only allows identity escapes of these).
	syntaxChars = "\\^$.|?*+()[]{}"
)

func escapeOutside(r rune) (string, error) {
	switch {
	case isAlnum(r) || strings.ContainsRune(safeOutside, r):
		return string(r), nil
	case strings.ContainsRune(syntaxChars, r):
		return `\` + string(r), nil
	case r <= 0xFF:
		return fmt.Sprintf(`\x%02X`, r), nil
	}
	return "", fmt.Errorf("code point U+%04X cannot be written in the common regex dialect", r)
}

func escapeInside(r rune) string {
	if isAlnum(r) || strings.ContainsRune(safeInside, r) {
		return string(r)
	}
	return fmt.Sprintf(`\x%02X`, r)
}
