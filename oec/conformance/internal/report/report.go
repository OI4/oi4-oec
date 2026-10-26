// Package report writes the results of D7 validate as text or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/validate"
)

// Item is the result of one message of an input.
type Item struct {
	Source string `json:"source"`
	validate.Result
}

// Report is the JSON document of a run.
type Report struct {
	Tool     string `json:"tool"`
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
	Valid    bool   `json:"valid"`
	Count    int    `json:"count"`
	Invalid  int    `json:"invalid"`
	Items    []Item `json:"results"`
}

// New returns an empty report.
func New(tool, version, baseline string) *Report {
	return &Report{Tool: tool, Version: version, Baseline: baseline, Valid: true, Items: []Item{}}
}

// Add records one result.
func (r *Report) Add(source string, res validate.Result) {
	r.Items = append(r.Items, Item{Source: source, Result: res})
	r.Count++
	if !res.Valid {
		r.Invalid++
		r.Valid = false
	}
}

// JSON writes the report as one JSON document.
func (r *Report) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

// Text writes one block per message and a summary line.
func (r *Report) Text(w io.Writer) error {
	var b strings.Builder
	for _, it := range r.Items {
		verdict := "VALID"
		if !it.Valid {
			verdict = "INVALID"
		}
		fmt.Fprintf(&b, "%s: %s", it.Source, verdict)
		if it.Topic != nil {
			fmt.Fprintf(&b, " (%s)", it.Topic.Rule)
		}
		if len(it.Reqs) > 0 {
			fmt.Fprintf(&b, " %s", strings.Join(it.Reqs, " "))
		}
		b.WriteString("\n")
		for _, f := range it.Findings {
			req := f.Req
			if req == "" {
				req = "-"
			}
			at := ""
			if f.Pointer != "" {
				at = " " + f.Pointer
			}
			fmt.Fprintf(&b, "  [%s] %s%s: %s\n", f.Layer, req, at, f.Detail)
		}
		for _, wn := range it.Warnings {
			if id := topic.WarningReqs[wn]; id != "" {
				wn += " (" + id + ")"
			}
			fmt.Fprintf(&b, "  warning: %s\n", wn)
		}
	}
	fmt.Fprintf(&b, "%s %s, baseline %s: %d message(s), %d invalid\n", r.Tool, r.Version, r.Baseline, r.Count, r.Invalid)
	_, err := io.WriteString(w, b.String())
	return err
}
