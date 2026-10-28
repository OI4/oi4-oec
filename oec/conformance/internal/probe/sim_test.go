package probe

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/sim"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

const simDir = "../../testdata/sim"

type simIndex struct {
	Fixtures []struct {
		Fault    string `yaml:"fault"`
		Status   string `yaml:"status"`
		Positive bool   `yaml:"positive"`
		Expect   map[string]struct {
			Outcome verdict.Outcome `yaml:"outcome"`
			Reqs    []string        `yaml:"reqs"`
		} `yaml:"expect"`
		Notes string `yaml:"notes"`
	} `yaml:"fixtures"`
}

func loadSimIndex(t *testing.T) simIndex {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(simDir, "index.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var ix simIndex
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&ix); err != nil {
		t.Fatal(err)
	}
	return ix
}

// simTimeout is the answer timeout of the fixture runs: long enough for a
// loaded machine (19 fixtures run in parallel in C-063), short enough
// for the negative waits. A wall-clock jump (sleep of the machine) during a
// run fails the heartbeat check of T7-MON-05: rerun.
const simTimeout = 3 * time.Second

// runSim runs oec-check test (monitor + runner) against the simulated CUT
// with the fault on a fresh test broker.
func runSim(t *testing.T, v *validate.Validator, reqs verdict.Requirements, fault sim.Fault, level int) ([]verdict.TestResult, Session) {
	t.Helper()
	pki := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, pki, "", "", false)
	cfg := mqttwire.Config{Broker: b.URL(), CAFile: pki.CAFile}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	started := time.Now().UTC()
	e := monitor.New(v, monitor.DefaultOptions(), started)
	observe := func(m mqttwire.Message) error {
		return e.Observe(monitor.Message{Topic: m.Topic, Payload: m.Payload, Size: len(m.Payload), Received: m.Received.UTC(), QoS: m.QoS, Retain: m.Retain})
	}
	// The CUT starts after the subscription, so the monitor sees its Birth.
	var c *sim.Sim
	start := func(ctx context.Context) error {
		var err error
		c, err = sim.Start(ctx, cfg, fault)
		return err
	}
	app := sim.ServiceType + "/" + sim.AppID
	s, err := RunBus(ctx, cfg, Options{App: app, Level: level, Timeout: simTimeout, Subscribed: start}, reqs, observe)
	if c != nil {
		c.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	if s.Cause != nil {
		t.Fatal(s.Cause)
	}
	mon := e.Finish(time.Now().UTC())
	// D7's own requests conform, except the deliberate stimuli that strict
	// validation rejects: the unknown key of T7-GET-09 and the undefined
	// query parameter of T7-GET-07 (REQ-MSG-001), the Query on Get MAM
	// (REQ-QRY-001) and the Query on Get ReferenceDesignation with Source
	// (REQ-RES-REFDES-013) of T7-GET-07. Against a faulty CUT a test may
	// stop before its stimuli: then a part of them.
	want := []string{"REQ-MSG-001", "REQ-MSG-001", "REQ-QRY-001", "REQ-RES-REFDES-013"}
	var own []string
	for _, o := range mon.Observations {
		if o.Participant == s.Probe.Tester && o.Kind == verdict.Violation {
			own = append(own, o.Req)
			t.Logf("tester: %s %s %s %s", o.Test, o.Req, o.Topic, o.Detail)
		}
	}
	sort.Strings(own)
	if fault == "none" && !slices.Equal(own, want) || !subset(own, want) {
		t.Errorf("violations of the requests of D7 %v, want %v (the deliberate stimuli of T7-GET-07 and T7-GET-09)", own, want)
	}
	res, _, err := Judge(app, level, nil, mon, s, reqs)
	if err != nil {
		t.Fatal(err)
	}
	return res, s
}

// subset reports whether the sorted a is a sub-multiset of the sorted b.
func subset(a, b []string) bool {
	i := 0
	for _, x := range b {
		if i < len(a) && a[i] == x {
			i++
		}
	}
	return i == len(a)
}

// TestSimFixtures runs every entry of testdata/sim/index.yaml and compares
// the verdicts at level 3 (DESIGN.md §10).
func TestSimFixtures(t *testing.T) {
	v, err := validate.New()
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	ix := loadSimIndex(t)
	faults := map[string]bool{}
	for _, f := range sim.Faults {
		faults[string(f)] = true
	}
	faults["none"] = true
	type outcome struct {
		ran                bool
		positive, negative []string
	}
	results := make([]outcome, len(ix.Fixtures))
	t.Run("run", func(t *testing.T) {
		for i, fx := range ix.Fixtures {
			if !faults[fx.Fault] {
				t.Errorf("%s: unknown fault", fx.Fault)
				continue
			}
			delete(faults, fx.Fault)
			if fx.Status != "draft" && fx.Status != "reviewed" {
				t.Errorf("%s: status %q", fx.Fault, fx.Status)
			}
			t.Run(fx.Fault, func(t *testing.T) {
				t.Parallel()
				fault := sim.Fault(fx.Fault)
				if fx.Fault == "none" {
					fault = sim.None
				}
				res, s := runSim(t, v, reqs, fault, 3)
				results[i].ran = true
				t.Logf("%d requests", len(s.Probe.Requests))
				for _, tr := range res {
					if tr.Status != "implemented" {
						continue
					}
					t.Logf("%s %s %s", tr.ID, tr.Outcome, tr.Reason)
					for _, r := range tr.Reqs {
						t.Logf("  %s %s %s", r.Req, r.Outcome, r.Reason)
					}
					want, listed := fx.Expect[tr.ID]
					if !listed {
						if tr.Outcome != verdict.Pass && tr.Outcome != verdict.Skip {
							t.Errorf("%s: outcome %s, want pass or skip (%+v)", tr.ID, tr.Outcome, tr.Reqs)
						}
						if fx.Positive && tr.Outcome == verdict.Pass {
							results[i].positive = append(results[i].positive, tr.ID)
						}
						continue
					}
					results[i].negative = append(results[i].negative, tr.ID)
					if tr.Outcome != want.Outcome {
						t.Errorf("%s: outcome %s, want %s (%+v)", tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
					}
					var got []string
					for _, r := range tr.Reqs {
						if r.Outcome == tr.Outcome {
							got = append(got, r.Req)
						}
					}
					sort.Strings(got)
					w := slices.Clone(want.Reqs)
					sort.Strings(w)
					if !slices.Equal(got, w) {
						t.Errorf("%s: requirements with outcome %s %v, want %v (%+v)", tr.ID, tr.Outcome, got, w, tr.Reqs)
					}
				}
			})
		}
	})
	if !t.Failed() {
		for f := range faults {
			t.Errorf("fault %s has no fixture in index.yaml", f)
		}
	}
	positive, negative := map[string]bool{}, map[string]bool{}
	for _, r := range results {
		if !r.ran {
			return // filtered with -run: no coverage check
		}
		for _, id := range r.positive {
			positive[id] = true
		}
		for _, id := range r.negative {
			negative[id] = true
		}
	}
	for _, id := range Implemented {
		if !positive[id] {
			t.Errorf("%s has no positive fixture (DESIGN.md §10)", id)
		}
		if !negative[id] {
			t.Errorf("%s has no negative fixture (DESIGN.md §10)", id)
		}
	}
}

// TestImplemented keeps Implemented equal to the catalog status.
func TestImplemented(t *testing.T) {
	tests, err := Tests()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tc := range tests {
		if tc.Status == "implemented" {
			got = append(got, tc.ID)
		}
	}
	if !slices.Equal(got, Implemented) {
		t.Errorf("catalog implemented %v, runner %v", got, Implemented)
	}
}
