package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/verdict"
)

type junitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitMessage struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Text    string `xml:",chardata"`
}

type junitCase struct {
	Name       string          `xml:"name,attr"`
	Classname  string          `xml:"classname,attr"`
	Properties []junitProperty `xml:"properties>property,omitempty"`
	Failure    *junitMessage   `xml:"failure,omitempty"`
	Skipped    *junitMessage   `xml:"skipped,omitempty"`
	SystemOut  string          `xml:"system-out,omitempty"`
}

type junitSuite struct {
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Skipped    int             `xml:"skipped,attr"`
	Timestamp  string          `xml:"timestamp,attr"`
	Properties []junitProperty `xml:"properties>property,omitempty"`
	Cases      []junitCase     `xml:"testcase"`
}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

// group returns the catalog group of a test id (T7-MON-01 → MON).
func group(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) == 3 {
		return parts[1]
	}
	return id
}

// writeJUnit writes test results as JUnit XML (DESIGN.md §7), shared by the
// reports of oec-check and oec-infra: one testsuite per catalog group, one
// testcase per test, the requirements as properties; fail → failure,
// skip/manual → skipped, warn/info as system-out.
func writeJUnit(w io.Writer, tool string, started time.Time, props []junitProperty, tests []verdict.TestResult) error {
	byGroup := map[string]*junitSuite{}
	var names []string
	for _, t := range tests {
		g := group(t.ID)
		s := byGroup[g]
		if s == nil {
			s = &junitSuite{Name: tool + "." + g, Timestamp: started.UTC().Format(time.RFC3339), Properties: props}
			byGroup[g] = s
			names = append(names, g)
		}
		c := junitCase{Name: t.ID + " " + t.Title, Classname: tool + "." + g}
		var out []string
		for _, q := range t.Reqs {
			c.Properties = append(c.Properties, junitProperty{"req", q.Req + "=" + string(q.Outcome)})
			if q.Outcome == verdict.Pass || q.Outcome == verdict.Skip {
				continue
			}
			for _, e := range q.Evidence {
				out = append(out, fmt.Sprintf("%s %s: %s", q.Outcome, q.Req, evidenceText(e)))
			}
		}
		text := strings.Join(out, "\n")
		switch t.Outcome {
		case verdict.Fail:
			c.Failure = &junitMessage{Message: "requirement violated", Type: "fail", Text: text}
			s.Failures++
		case verdict.Skip, verdict.Manual:
			c.Skipped = &junitMessage{Message: t.Reason}
			if c.Skipped.Message == "" {
				var rs []string
				for _, q := range t.Reqs {
					if q.Reason != "" {
						rs = append(rs, q.Req+": "+q.Reason)
					}
				}
				c.Skipped.Message = strings.Join(rs, "; ")
			}
			s.Skipped++
		default:
			c.SystemOut = text
		}
		s.Tests++
		s.Cases = append(s.Cases, c)
	}
	sort.Strings(names)
	doc := junitSuites{Name: tool}
	for _, g := range names {
		s := byGroup[g]
		doc.Tests += s.Tests
		doc.Failures += s.Failures
		doc.Skipped += s.Skipped
		doc.Suites = append(doc.Suites, *s)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
