package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// MonitorVersion is the version of the JSON document of a monitor run.
const MonitorVersion = 1

// Monitor is the report of oec-check monitor (DESIGN.md §7): the run, the
// participants seen on the bus and one verdict per catalog test.
type Monitor struct {
	Report         int                     `json:"report"` // MonitorVersion
	Tool           string                  `json:"tool"`
	Version        string                  `json:"version"`
	Schemas        string                  `json:"schemas"`
	Baseline       string                  `json:"baseline"`
	Catalog        string                  `json:"catalog"` // baseline of the test catalog
	Command        []string                `json:"command"`
	Mode           string                  `json:"mode"`       // live | replay
	Attachment     string                  `json:"attachment"` // bus
	Capture        string                  `json:"capture,omitempty"`
	Broker         string                  `json:"broker,omitempty"`
	Insecure       bool                    `json:"insecure,omitempty"` // broker certificate not verified
	Level          int                     `json:"level"`
	App            string                  `json:"app,omitempty"`
	SelfAssessment bool                    `json:"self_assessment"`
	Started        time.Time               `json:"started"`
	Ended          time.Time               `json:"ended"`
	Observed       string                  `json:"observed"`
	Messages       int                     `json:"messages"`
	Namespace      int                     `json:"namespace"` // messages in the Oi4v2 namespace
	Participants   []monitor.Participant   `json:"participants"`
	Tests          []verdict.TestResult    `json:"tests"`
	Summary        map[verdict.Outcome]int `json:"summary"`
	Error          string                  `json:"error,omitempty"` // why the observation stopped early
	Exit           int                     `json:"exit"`
}

// JSON writes the report as one JSON document.
func (r *Monitor) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

var outcomeOrder = []verdict.Outcome{verdict.Pass, verdict.Info, verdict.Warn, verdict.Fail, verdict.Skip, verdict.Manual}

func (r *Monitor) summaryLine() string {
	var parts []string
	for _, o := range outcomeOrder {
		if n := r.Summary[o]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, o))
		}
	}
	if len(parts) == 0 {
		return "no implemented test"
	}
	return strings.Join(parts, ", ")
}

func evidenceText(e verdict.Evidence) string {
	var at []string
	if e.Line > 0 {
		at = append(at, fmt.Sprintf("line %d", e.Line))
	}
	if e.Topic != "" {
		at = append(at, e.Topic)
	}
	if e.Pointer != "" {
		at = append(at, e.Pointer)
	}
	s := e.Detail
	if len(at) > 0 {
		s += " [" + strings.Join(at, " ") + "]"
	}
	return s
}

var exitText = map[int]string{0: "PASS", 1: "FAIL", 4: "INCONCLUSIVE"}

// Text writes the participants, one line per test and the evidence of every
// requirement that is not fulfilled.
func (r *Monitor) Text(w io.Writer) error {
	var b strings.Builder
	src := r.Broker
	if r.Mode == "replay" {
		src = r.Capture
	}
	fmt.Fprintf(&b, "%s %s monitor (%s %s), level %d, schemas %s, baseline %s\n", r.Tool, r.Version, r.Mode, src, r.Level, r.Schemas, r.Baseline)
	if r.Insecure {
		b.WriteString("WARNING: the broker certificate was not verified (--insecure, dev only)\n")
	}
	if r.App != "" {
		fmt.Fprintf(&b, "component under test: %s\n", r.App)
	}
	fmt.Fprintf(&b, "observed %s from %s: %d message(s), %d in the Oi4v2 namespace\n", r.Observed, r.Started.Format(time.RFC3339), r.Messages, r.Namespace)
	if r.Error != "" {
		fmt.Fprintf(&b, "stopped early: %s\n", r.Error)
	}
	b.WriteString("participants:\n")
	if len(r.Participants) == 0 {
		b.WriteString("  none\n")
	}
	for _, p := range r.Participants {
		fmt.Fprintf(&b, "  %s: %d message(s), %d invalid, %d birth(s), %d will(s), %d close(s)", p.ID, p.Messages, p.Invalid, p.Births, p.Wills, p.Closes)
		if len(p.Sources) > 0 {
			fmt.Fprintf(&b, ", %d subordinate source(s)", len(p.Sources))
		}
		b.WriteString("\n")
	}
	b.WriteString("tests:\n")
	for _, t := range r.Tests {
		fmt.Fprintf(&b, "  %-5s %s  %s", t.Outcome, t.ID, t.Title)
		if t.Reason != "" {
			fmt.Fprintf(&b, " (%s)", t.Reason)
		}
		b.WriteString("\n")
		for _, q := range t.Reqs {
			if q.Outcome == verdict.Pass || q.Outcome == verdict.Skip && !q.Inconclusive {
				continue
			}
			ob := q.Obligation
			if ob == "" {
				ob = "-"
			}
			id := q.Req
			if id == "" {
				id = "(no requirement)"
			}
			fmt.Fprintf(&b, "        %s %s (%s)", q.Outcome, id, ob)
			if q.Inconclusive {
				b.WriteString(" inconclusive")
			}
			if q.Reason != "" {
				fmt.Fprintf(&b, ": %s", q.Reason)
			}
			b.WriteString("\n")
			if q.Outcome == verdict.Skip {
				continue
			}
			for _, e := range q.Evidence {
				fmt.Fprintf(&b, "          - %s\n", evidenceText(e))
			}
		}
	}
	fmt.Fprintf(&b, "summary: %s; result %s (exit %d)\n", r.summaryLine(), exitText[r.Exit], r.Exit)
	_, err := io.WriteString(w, b.String())
	return err
}

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

// JUnit writes the report as JUnit XML (DESIGN.md §7): one testsuite per
// catalog group, one testcase per test, the requirements as properties;
// fail → failure, skip/manual → skipped, warn/info as system-out.
func (r *Monitor) JUnit(w io.Writer) error {
	byGroup := map[string]*junitSuite{}
	var names []string
	for _, t := range r.Tests {
		g := group(t.ID)
		s := byGroup[g]
		if s == nil {
			s = &junitSuite{Name: r.Tool + "." + g, Timestamp: r.Started.UTC().Format(time.RFC3339),
				Properties: []junitProperty{{"level", fmt.Sprint(r.Level)}, {"mode", r.Mode}, {"schemas", r.Schemas}, {"baseline", r.Baseline}}}
			byGroup[g] = s
			names = append(names, g)
		}
		c := junitCase{Name: t.ID + " " + t.Title, Classname: r.Tool + "." + g}
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
	doc := junitSuites{Name: r.Tool}
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
