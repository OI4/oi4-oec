package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/probe"
	"github.com/OI4/oec/conformance/internal/proxy"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// MonitorVersion is the version of the JSON document of a monitor run.
const MonitorVersion = 1

// Monitor is the report of oec-check monitor and test (DESIGN.md §7): the
// run, the participants seen on the bus and one verdict per catalog test. A
// test run (mode test) adds the identity of D7 and its requests.
type Monitor struct {
	Report         int                     `json:"report"` // MonitorVersion
	Tool           string                  `json:"tool"`
	Version        string                  `json:"version"`
	Schemas        string                  `json:"schemas"`
	Baseline       string                  `json:"baseline"`
	Catalog        string                  `json:"catalog"` // baseline of the test catalog
	Command        []string                `json:"command"`
	Mode           string                  `json:"mode"`       // live | replay | test
	Attachment     string                  `json:"attachment"` // bus | proxy
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
	Tester         string                  `json:"tester,omitempty"`       // test: PublisherId of D7 in the requests
	Requests       []probe.Request         `json:"requests,omitempty"`     // test: the stimuli in order
	Restore        *probe.Restore          `json:"restore,omitempty"`      // test: the state of the CUT after the write tests
	Announced      []probe.Announced       `json:"announced,omitempty"`    // test: the components D7 announced (--announce, --peer)
	ProxyListen    string                  `json:"proxy_listen,omitempty"` // test, proxy attachment: address of the proxy
	Connections    []proxy.Conn            `json:"connections,omitempty"`  // test, proxy attachment: the connection attempts of the CUT (no payloads, no password)
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

func summaryLine(summary map[verdict.Outcome]int) string {
	var parts []string
	for _, o := range outcomeOrder {
		if n := summary[o]; n > 0 {
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
	if e.Image != "" {
		at = append(at, e.Image)
	}
	if e.Container != "" {
		at = append(at, e.Container)
	}
	if e.Path != "" {
		at = append(at, e.Path)
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
	src, cmd := r.Broker, "monitor"
	if r.Mode == "replay" {
		src = r.Capture
	}
	if r.Mode == "test" {
		cmd = "test"
	}
	fmt.Fprintf(&b, "%s %s %s (%s %s), level %d, schemas %s, baseline %s\n", r.Tool, r.Version, cmd, r.Mode, src, r.Level, r.Schemas, r.Baseline)
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
	if r.Mode == "test" {
		answered := 0
		for _, q := range r.Requests {
			if q.Replies > 0 {
				answered++
			}
		}
		fmt.Fprintf(&b, "tester %s: %d request(s), %d answered\n", r.Tester, len(r.Requests), answered)
		for _, a := range r.Announced {
			fmt.Fprintf(&b, "announced %s %s: %d request(s) received, %d answered\n", a.Role, a.PublisherID, a.Requests, a.Answered)
		}
		if rs := r.Restore; rs != nil {
			b.WriteString(rs.Summary() + "\n")
		}
		if r.Attachment == "proxy" {
			fmt.Fprintf(&b, "proxy %s: %d connection attempt(s)\n", r.ProxyListen, len(r.Connections))
			for _, c := range r.Connections {
				fmt.Fprintf(&b, "  #%d %s", c.ID, c.Phase)
				if c.TLSVersion != "" {
					fmt.Fprintf(&b, " %s", c.TLSVersion)
				}
				if c.HandshakeErr != "" {
					fmt.Fprintf(&b, " handshake failed (%s)", c.HandshakeErr)
				}
				if c.Connect != nil {
					fmt.Fprintf(&b, " CONNECT %s/%d ClientId %q", c.Connect.ProtocolName, c.Connect.Level, c.Connect.ClientID)
				}
				if c.Connack >= 0 {
					fmt.Fprintf(&b, " CONNACK %d", c.Connack)
				}
				fmt.Fprintf(&b, ", %d publish(es)", len(c.Publishes))
				if c.Dropped {
					b.WriteString(", dropped")
				}
				b.WriteString("\n")
			}
		}
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
	writeTests(&b, r.Tests)
	fmt.Fprintf(&b, "summary: %s; result %s (exit %d)\n", summaryLine(r.Summary), exitText[r.Exit], r.Exit)
	_, err := io.WriteString(w, b.String())
	return err
}

// writeTests writes one line per test and the evidence of every requirement
// that is not fulfilled (text reports of oec-check and oec-infra).
func writeTests(b *strings.Builder, tests []verdict.TestResult) {
	for _, t := range tests {
		fmt.Fprintf(b, "  %-5s %s  %s", t.Outcome, t.ID, t.Title)
		if t.Reason != "" {
			fmt.Fprintf(b, " (%s)", t.Reason)
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
			fmt.Fprintf(b, "        %s %s (%s)", q.Outcome, id, ob)
			if q.Inconclusive {
				b.WriteString(" inconclusive")
			}
			if q.Reason != "" {
				fmt.Fprintf(b, ": %s", q.Reason)
			}
			b.WriteString("\n")
			if q.Outcome == verdict.Skip {
				continue
			}
			for _, e := range q.Evidence {
				fmt.Fprintf(b, "          - %s\n", evidenceText(e))
			}
		}
	}
}

// JUnit writes the report as JUnit XML (DESIGN.md §7): one testsuite per
// catalog group, one testcase per test, the requirements as properties;
// fail → failure, skip/manual → skipped, warn/info as system-out.
func (r *Monitor) JUnit(w io.Writer) error {
	return writeJUnit(w, r.Tool, r.Started, []junitProperty{{"level", fmt.Sprint(r.Level)}, {"mode", r.Mode}, {"schemas", r.Schemas}, {"baseline", r.Baseline}}, r.Tests)
}
