package probe

import (
	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// AllTests returns the tests a test run judges: the passive tests of
// monitor and the active tests of test, in catalog order per mode.
func AllTests() ([]verdict.Test, error) {
	mt, err := monitor.Tests()
	if err != nil {
		return nil, err
	}
	tt, err := Tests()
	if err != nil {
		return nil, err
	}
	return append(mt, tt...), nil
}

// TestRunPrefix marks a monitor requirement that a test run cannot judge.
const TestRunPrefix = "test run: "

// Judge builds the verdict of a test run (ADR-0025 amendment C-062): the
// monitor observations of the CUT plus the observations of the runner. A
// requirement the passive checks cannot judge in a short test run (connect
// not observed, observation shorter than the heartbeat, MaxPacketSize
// unknown) is a plain skip, not an inconclusive one: a level claim needs a
// monitor run of at least MinDuration besides the test run. Tests that
// patterns does not select are skipped.
func Judge(app string, level int, patterns []string, mon monitor.Result, s Session, reqs verdict.Requirements) ([]verdict.TestResult, []monitor.Participant, error) {
	tests, err := AllTests()
	if err != nil {
		return nil, nil, err
	}
	_, id := SplitApp(app)
	obs, parts := monitor.ForApp(id, mon.Observations, mon.Participants)
	for i := range obs {
		if obs[i].Kind == verdict.Skipped && obs[i].Inconclusive {
			obs[i].Inconclusive = false
			obs[i].Detail = TestRunPrefix + obs[i].Detail
		}
	}
	if !s.Probe.Found && len(parts) == 0 {
		obs = monitor.NotFound(tests, id)
	} else {
		obs = append(obs, s.Probe.Observations...)
	}
	res := verdict.Build(tests, obs, reqs, level)
	for i := range res {
		if !Selected(patterns, res[i].ID) {
			res[i] = verdict.TestResult{ID: res[i].ID, Title: res[i].Title, Level: res[i].Level, Status: res[i].Status, Outcome: verdict.Skip, Reason: "not selected (--tests)"}
		}
	}
	return res, parts, nil
}
