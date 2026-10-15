package abnf

import (
	"fmt"
	"math/rand"
	"strings"
)

// Generator produces random strings that match a rule. Unbounded repetitions
// add up to MaxExtra items beyond the minimum; large character ranges prefer
// printable ASCII so that samples stay readable.
type Generator struct {
	G        *Grammar
	Rand     *rand.Rand
	MaxExtra int
}

// Generate returns one random string matching rule.
func (gen *Generator) Generate(rule string) (string, error) {
	key := strings.ToLower(rule)
	if _, ok := gen.G.Rules[key]; !ok {
		return "", fmt.Errorf("unknown rule %s", rule)
	}
	var b strings.Builder
	gen.emit(&b, Ref{Name: key})
	return b.String(), nil
}

func (gen *Generator) emit(b *strings.Builder, n Node) {
	r := gen.Rand
	switch v := n.(type) {
	case Ref:
		gen.emit(b, gen.G.Rules[v.Name].Node)
	case Literal:
		for _, c := range v.Text {
			if !v.CaseSensitive && isASCIILetter(c) && r.Intn(2) == 0 {
				if c == lower(c) {
					c = upper(c)
				} else {
					c = lower(c)
				}
			}
			b.WriteRune(c)
		}
	case Range:
		b.WriteRune(gen.pick(v))
	case Alt:
		gen.emit(b, v.Items[r.Intn(len(v.Items))])
	case Concat:
		for _, it := range v.Items {
			gen.emit(b, it)
		}
	case Repeat:
		count := v.Min
		switch {
		case v.Max < 0:
			extra := gen.MaxExtra
			if extra <= 0 {
				extra = 3
			}
			count += r.Intn(extra + 1)
		case r.Intn(2) == 0:
			count += r.Intn(v.Max - v.Min + 1)
		default:
			count += r.Intn(min(v.Max-v.Min, 3) + 1)
		}
		for i := 0; i < count; i++ {
			gen.emit(b, v.Item)
		}
	}
}

func (gen *Generator) pick(v Range) rune {
	r := gen.Rand
	if v.Hi-v.Lo > 0xFF && r.Intn(5) != 0 {
		lo, hi := max(v.Lo, 0x20), min(v.Hi, 0x7E)
		if lo <= hi {
			return lo + rune(r.Intn(int(hi-lo+1)))
		}
	}
	if v.Lo >= surrogateLo && v.Hi <= surrogateHi {
		return v.Lo
	}
	for {
		c := v.Lo + rune(r.Intn(int(v.Hi-v.Lo+1)))
		if c < surrogateLo || c > surrogateHi {
			return c
		}
	}
}
