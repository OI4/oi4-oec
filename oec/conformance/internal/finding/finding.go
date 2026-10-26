// Package finding defines the result unit of D7 validate.
package finding

import (
	"sort"
	"strings"
)

// Layers of D7 validate (ADR-0019 §5).
const (
	LayerTopic  = "topic"
	LayerParse  = "parse"
	LayerSchema = "schema"
	LayerRule   = "rule"
)

// Finding is one violated requirement of a message.
type Finding struct {
	Req     string `json:"req,omitempty"`
	Layer   string `json:"layer"`
	Pointer string `json:"pointer,omitempty"`
	Detail  string `json:"detail"`
}

// Reqs returns the distinct, sorted REQ ids of fs.
func Reqs(fs []Finding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		if f.Req != "" && !seen[f.Req] {
			seen[f.Req] = true
			out = append(out, f.Req)
		}
	}
	sort.Strings(out)
	return out
}

// Pointer returns the JSON Pointer (RFC 6901) of the reference tokens.
func Pointer(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	r := strings.NewReplacer("~", "~0", "/", "~1")
	var b strings.Builder
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(r.Replace(t))
	}
	return b.String()
}

// Sort orders findings by layer, pointer and REQ.
func Sort(fs []Finding) {
	order := map[string]int{LayerTopic: 0, LayerParse: 1, LayerSchema: 2, LayerRule: 3}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if order[a.Layer] != order[b.Layer] {
			return order[a.Layer] < order[b.Layer]
		}
		if a.Pointer != b.Pointer {
			return a.Pointer < b.Pointer
		}
		return a.Req < b.Req
	})
}
