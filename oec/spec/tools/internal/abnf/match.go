package abnf

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Match reports whether the whole string s matches the rule. The matcher
// explores all alternatives (set of end positions), so ambiguous grammars are
// handled correctly.
func (g *Grammar) Match(rule, s string) (bool, error) {
	key := strings.ToLower(rule)
	if _, ok := g.Rules[key]; !ok {
		return false, fmt.Errorf("unknown rule %s", rule)
	}
	m := &matcher{g: g, in: []rune(s), memo: map[memoKey][]int{}}
	for _, end := range m.match(Ref{Name: key}, 0) {
		if end == len(m.in) {
			return true, nil
		}
	}
	return false, nil
}

type memoKey struct {
	rule string
	pos  int
}

type matcher struct {
	g    *Grammar
	in   []rune
	memo map[memoKey][]int
}

// match returns the sorted, de-duplicated end positions of n starting at pos.
func (m *matcher) match(n Node, pos int) []int {
	switch v := n.(type) {
	case Ref:
		k := memoKey{v.Name, pos}
		if r, ok := m.memo[k]; ok {
			return r
		}
		r := m.match(m.g.Rules[v.Name].Node, pos)
		m.memo[k] = r
		return r
	case Range:
		if pos < len(m.in) && m.in[pos] >= v.Lo && m.in[pos] <= v.Hi {
			return []int{pos + 1}
		}
		return nil
	case Literal:
		lit := []rune(v.Text)
		if pos+len(lit) > len(m.in) {
			return nil
		}
		for i, r := range lit {
			c := m.in[pos+i]
			if c == r || !v.CaseSensitive && r < 0x80 && c < 0x80 && unicode.ToLower(c) == unicode.ToLower(r) {
				continue
			}
			return nil
		}
		return []int{pos + len(lit)}
	case Alt:
		set := map[int]bool{}
		for _, it := range v.Items {
			for _, e := range m.match(it, pos) {
				set[e] = true
			}
		}
		return keys(set)
	case Concat:
		cur := []int{pos}
		for _, it := range v.Items {
			set := map[int]bool{}
			for _, p := range cur {
				for _, e := range m.match(it, p) {
					set[e] = true
				}
			}
			if len(set) == 0 {
				return nil
			}
			cur = keys(set)
		}
		return cur
	case Repeat:
		step := func(from []int) []int {
			set := map[int]bool{}
			for _, p := range from {
				for _, e := range m.match(v.Item, p) {
					set[e] = true
				}
			}
			return keys(set)
		}
		cur := []int{pos}
		for i := 0; i < v.Min; i++ {
			cur = step(cur)
			if len(cur) == 0 {
				return nil
			}
		}
		result := map[int]bool{}
		for _, p := range cur {
			result[p] = true
		}
		for i := v.Min; v.Max < 0 || i < v.Max; i++ {
			var next []int
			for _, p := range step(cur) {
				if !result[p] {
					result[p] = true
					next = append(next, p)
				}
			}
			if len(next) == 0 {
				break
			}
			cur = next
		}
		return keys(result)
	}
	panic(fmt.Sprintf("abnf: unknown node %T", n))
}

func keys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
