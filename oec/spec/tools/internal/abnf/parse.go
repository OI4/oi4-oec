// Package abnf implements the subset of ABNF (RFC 5234 with the case-sensitive
// strings of RFC 7405) needed for the OEC topic grammar: it parses a rule
// list, matches strings against a rule, compiles non-recursive rules into
// regular expressions of the common ECMA-262 / RE2 / Rust dialect and
// generates random strings for cross-checks. Prose values (<...>) are not
// supported.
package abnf

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is an element of a rule definition.
type Node interface{ node() }

// Alt is an alternation (a / b).
type Alt struct{ Items []Node }

// Concat is a concatenation (a b).
type Concat struct{ Items []Node }

// Repeat is a repetition n*m element; Max < 0 means unbounded.
type Repeat struct {
	Min, Max int
	Item     Node
}

// Ref references another rule (names are case-insensitive, stored lowercase).
type Ref struct{ Name string }

// Literal is a character string; RFC 5234 quoted strings are case-insensitive,
// %s"..." (RFC 7405) and concatenated numeric values are case-sensitive.
type Literal struct {
	Text          string
	CaseSensitive bool
}

// Range is a single code point or a code point range of a numeric value.
type Range struct{ Lo, Hi rune }

func (Alt) node()     {}
func (Concat) node()  {}
func (Repeat) node()  {}
func (Ref) node()     {}
func (Literal) node() {}
func (Range) node()   {}

// Rule is one named rule.
type Rule struct {
	Name string // as written in the first definition
	Node Node
	Line int
}

// Grammar is a parsed rule list. Rules holds the user rules and the core
// rules of RFC 5234 appendix B.1 that the user did not define.
type Grammar struct {
	Rules map[string]*Rule
	Order []string // lowercase names of user rules in definition order
}

// Parse parses an ABNF rule list and checks that every referenced rule is
// defined and that no rule is recursive.
func Parse(src string) (*Grammar, error) {
	g := &Grammar{Rules: map[string]*Rule{}}
	if err := g.parseInto(src, false); err != nil {
		return nil, err
	}
	if err := g.parseInto(coreRules, true); err != nil {
		return nil, fmt.Errorf("core rules: %w", err)
	}
	if err := g.check(); err != nil {
		return nil, err
	}
	return g, nil
}

// RFC 5234 appendix B.1 without LWSP.
const coreRules = `
ALPHA  = %x41-5A / %x61-7A
BIT    = "0" / "1"
CHAR   = %x01-7F
CR     = %x0D
CRLF   = CR LF
CTL    = %x00-1F / %x7F
DIGIT  = %x30-39
DQUOTE = %x22
HEXDIG = DIGIT / "A" / "B" / "C" / "D" / "E" / "F"
HTAB   = %x09
LF     = %x0A
OCTET  = %x00-FF
SP     = %x20
VCHAR  = %x21-7E
WSP    = SP / HTAB
`

type logicalLine struct {
	text string
	line int
}

func (g *Grammar) parseInto(src string, core bool) error {
	for _, l := range logicalLines(src) {
		name, rest, incremental, err := splitRule(l.text)
		if err != nil {
			return fmt.Errorf("line %d: %w", l.line, err)
		}
		key := strings.ToLower(name)
		if core {
			if _, ok := g.Rules[key]; ok {
				continue
			}
		}
		p := &parser{src: rest}
		n, err := p.alternation()
		if err != nil {
			return fmt.Errorf("line %d: rule %s: %w", l.line, name, err)
		}
		p.skipWS()
		if p.pos < len(p.src) {
			return fmt.Errorf("line %d: rule %s: unexpected %q", l.line, name, p.src[p.pos:])
		}
		existing, ok := g.Rules[key]
		switch {
		case incremental && !ok:
			return fmt.Errorf("line %d: incremental alternative =/ for undefined rule %s", l.line, name)
		case incremental:
			existing.Node = Alt{Items: append(alternatives(existing.Node), alternatives(n)...)}
		case ok:
			return fmt.Errorf("line %d: rule %s already defined in line %d", l.line, name, existing.Line)
		default:
			g.Rules[key] = &Rule{Name: name, Node: n, Line: l.line}
			if !core {
				g.Order = append(g.Order, key)
			}
		}
	}
	return nil
}

func alternatives(n Node) []Node {
	if a, ok := n.(Alt); ok {
		return a.Items
	}
	return []Node{n}
}

// logicalLines removes comments and joins continuation lines (lines that
// start with white space continue the previous rule).
func logicalLines(src string) []logicalLine {
	var out []logicalLine
	for i, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		text := stripComment(raw)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if (text[0] == ' ' || text[0] == '\t') && len(out) > 0 {
			out[len(out)-1].text += " " + strings.TrimSpace(text)
			continue
		}
		out = append(out, logicalLine{text: strings.TrimSpace(text), line: i + 1})
	}
	return out
}

func stripComment(s string) string {
	inQuote := false
	for i, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ';' && !inQuote:
			return strings.TrimRight(s[:i], " \t")
		}
	}
	return strings.TrimRight(s, " \t")
}

func splitRule(s string) (name, rest string, incremental bool, err error) {
	i := 0
	for i < len(s) && isNameChar(s[i], i == 0) {
		i++
	}
	if i == 0 {
		return "", "", false, fmt.Errorf("expected rule name in %q", s)
	}
	name = s[:i]
	r := strings.TrimLeft(s[i:], " \t")
	switch {
	case strings.HasPrefix(r, "=/"):
		return name, r[2:], true, nil
	case strings.HasPrefix(r, "="):
		return name, r[1:], false, nil
	}
	return "", "", false, fmt.Errorf("expected = or =/ after rule name %s", name)
}

func isNameChar(c byte, first bool) bool {
	alpha := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	if first {
		return alpha
	}
	return alpha || c >= '0' && c <= '9' || c == '-'
}

type parser struct {
	src string
	pos int
}

func (p *parser) skipWS() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *parser) alternation() (Node, error) {
	var items []Node
	for {
		c, err := p.concatenation()
		if err != nil {
			return nil, err
		}
		items = append(items, c)
		p.skipWS()
		if p.peek() != '/' {
			break
		}
		p.pos++
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return Alt{Items: items}, nil
}

func (p *parser) concatenation() (Node, error) {
	var items []Node
	for {
		p.skipWS()
		c := p.peek()
		if c == 0 || c == '/' || c == ')' || c == ']' {
			break
		}
		r, err := p.repetition()
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	switch len(items) {
	case 0:
		return nil, fmt.Errorf("empty element at %q", p.src[p.pos:])
	case 1:
		return items[0], nil
	}
	return Concat{Items: items}, nil
}

func (p *parser) number() (int, bool) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if p.pos == start {
		return 0, false
	}
	n, _ := strconv.Atoi(p.src[start:p.pos])
	return n, true
}

func (p *parser) repetition() (Node, error) {
	minN, hasMin := p.number()
	repeat := hasMin
	maxN := minN
	if p.peek() == '*' {
		p.pos++
		repeat = true
		if !hasMin {
			minN = 0
		}
		if n, ok := p.number(); ok {
			maxN = n
		} else {
			maxN = -1
		}
	}
	e, err := p.element()
	if err != nil {
		return nil, err
	}
	if !repeat {
		return e, nil
	}
	if maxN >= 0 && maxN < minN {
		return nil, fmt.Errorf("repetition %d*%d: max below min", minN, maxN)
	}
	return Repeat{Min: minN, Max: maxN, Item: e}, nil
}

func (p *parser) element() (Node, error) {
	switch c := p.peek(); {
	case c == '(' || c == '[':
		p.pos++
		n, err := p.alternation()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		closing := byte(')')
		if c == '[' {
			closing = ']'
		}
		if p.peek() != closing {
			return nil, fmt.Errorf("missing %q at %q", closing, p.src[p.pos:])
		}
		p.pos++
		if c == '[' {
			return Repeat{Min: 0, Max: 1, Item: n}, nil
		}
		return n, nil
	case c == '"':
		return p.quoted(false)
	case c == '%':
		return p.percent()
	case c == '<':
		return nil, fmt.Errorf("prose values are not supported: %q", p.src[p.pos:])
	case isNameChar(c, true):
		start := p.pos
		for p.pos < len(p.src) && isNameChar(p.src[p.pos], p.pos == start) {
			p.pos++
		}
		return Ref{Name: strings.ToLower(p.src[start:p.pos])}, nil
	default:
		return nil, fmt.Errorf("unexpected %q", p.src[p.pos:])
	}
}

func (p *parser) quoted(caseSensitive bool) (Node, error) {
	p.pos++ // opening quote
	end := strings.IndexByte(p.src[p.pos:], '"')
	if end < 0 {
		return nil, fmt.Errorf("unterminated string at %q", p.src[p.pos:])
	}
	text := p.src[p.pos : p.pos+end]
	p.pos += end + 1
	for _, r := range text {
		if r < 0x20 || r > 0x7E {
			return nil, fmt.Errorf("quoted string %q contains a character outside %%x20-7E", text)
		}
	}
	if text == "" {
		return nil, fmt.Errorf("empty quoted string")
	}
	return Literal{Text: text, CaseSensitive: caseSensitive}, nil
}

func (p *parser) percent() (Node, error) {
	p.pos++
	kind := strings.ToLower(string(p.peek()))
	p.pos++
	switch kind {
	case "s", "i":
		if p.peek() != '"' {
			return nil, fmt.Errorf("expected quoted string after %%%s", kind)
		}
		return p.quoted(kind == "s")
	case "x", "d", "b":
	default:
		return nil, fmt.Errorf("unknown numeric value %%%s", kind)
	}
	base := map[string]int{"x": 16, "d": 10, "b": 2}[kind]
	digits := func() (rune, error) {
		start := p.pos
		for p.pos < len(p.src) && isDigit(p.src[p.pos], base) {
			p.pos++
		}
		if p.pos == start {
			return 0, fmt.Errorf("expected base-%d digits at %q", base, p.src[start:])
		}
		v, err := strconv.ParseInt(p.src[start:p.pos], base, 32)
		if err != nil || v > utf8.MaxRune {
			return 0, fmt.Errorf("numeric value %q out of range", p.src[start:p.pos])
		}
		return rune(v), nil
	}
	first, err := digits()
	if err != nil {
		return nil, err
	}
	switch p.peek() {
	case '-':
		p.pos++
		hi, err := digits()
		if err != nil {
			return nil, err
		}
		if hi < first {
			return nil, fmt.Errorf("range %X-%X is empty", first, hi)
		}
		return Range{Lo: first, Hi: hi}, nil
	case '.':
		seq := []rune{first}
		for p.peek() == '.' {
			p.pos++
			r, err := digits()
			if err != nil {
				return nil, err
			}
			seq = append(seq, r)
		}
		return Literal{Text: string(seq), CaseSensitive: true}, nil
	}
	return Range{Lo: first, Hi: first}, nil
}

func isDigit(c byte, base int) bool {
	switch base {
	case 2:
		return c == '0' || c == '1'
	case 10:
		return c >= '0' && c <= '9'
	}
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// check verifies that all references resolve and that no rule is recursive.
func (g *Grammar) check() error {
	state := map[string]int{} // 0 unvisited, 1 in progress, 2 done
	var visit func(name string, path []string) error
	var walk func(n Node, path []string) error
	walk = func(n Node, path []string) error {
		switch v := n.(type) {
		case Alt:
			for _, it := range v.Items {
				if err := walk(it, path); err != nil {
					return err
				}
			}
		case Concat:
			for _, it := range v.Items {
				if err := walk(it, path); err != nil {
					return err
				}
			}
		case Repeat:
			return walk(v.Item, path)
		case Ref:
			if _, ok := g.Rules[v.Name]; !ok {
				return fmt.Errorf("rule %s references undefined rule %s", path[len(path)-1], v.Name)
			}
			return visit(v.Name, path)
		}
		return nil
	}
	visit = func(name string, path []string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("recursive rule: %s -> %s", strings.Join(path, " -> "), name)
		case 2:
			return nil
		}
		state[name] = 1
		if err := walk(g.Rules[name].Node, append(path, name)); err != nil {
			return err
		}
		state[name] = 2
		return nil
	}
	for _, name := range g.Order {
		if err := visit(name, nil); err != nil {
			return err
		}
	}
	return nil
}

// Has reports whether the grammar defines a rule (case-insensitive).
func (g *Grammar) Has(name string) bool {
	_, ok := g.Rules[strings.ToLower(name)]
	return ok
}
