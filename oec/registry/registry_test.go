// Integration tests of the OEC Registry (ADR-0029 §11, A-387): the
// registry binary on the test broker, judged by D8 (`oec-infra bus`,
// T8-BUS-03) and D7 (`oec-check test`), both run as programs (ADR-0006).
package registry

import (
	"strings"
	"testing"

	"github.com/OI4/oec/registry/internal/regtest"
)

// il is the Identification Link of the registry in the tests: the product
// of src/main.rs and the hostname of the container storage, dnp-encoded
// (MG 4.1, SF-104).
const il = "oi4.org/OEC,20Registry/OEC-REGISTRY/reg-1"

// TestD8Registry: D8 T8-BUS-03 (REQ-REG-001, REQ-REG-002) passes against
// the registry – two probe components with Birth, a second Health, Close
// and Will; their MAM and last Health in the answers to Get MAM and Get
// Health with and without Source while available, neither afterwards.
// On the wire the registry subscribes to the MAM and Health of every
// participant and publishes only Pub of its own and Get to others
// (read-only, ADR-0008).
func TestD8Registry(t *testing.T) {
	b := regtest.StartBroker(t)
	root := b.Container(t, "reg-1")
	_, birth := regtest.StartRegistry(t, b, root, il)
	tests, code := regtest.D8Bus(t, b, il, "T8-BUS-03")
	if len(tests) != 1 || tests[0].ID != "T8-BUS-03" || tests[0].Outcome != "pass" || code != 0 {
		t.Errorf("exit %d, tests %+v", code, tests)
	}
	conn := birth.Conn
	gets := 0
	collecting := map[string]bool{"Oi4v2/+/+/Pub/MAM/#": false, "Oi4v2/+/+/Pub/Health/#": false}
	for _, s := range b.Events() {
		if s.Conn == conn && s.Seen == "subscribe" {
			for _, f := range s.Filters {
				if _, ok := collecting[f]; ok {
					collecting[f] = true
				}
			}
		}
		if s.Conn != conn || s.Seen != "publish" {
			continue
		}
		lv := strings.Split(s.Topic, "/")
		if len(lv) < 5 {
			t.Errorf("topic %s", s.Topic)
			continue
		}
		own := lv[1] == "Registry" && lv[2] == regtest.Topic(il)
		switch {
		case own && lv[3] == "Pub":
		case !own && lv[3] == "Get":
			gets++
		default:
			t.Errorf("the registry published on %s (read-only: Pub of its own, Get to others)", s.Topic)
		}
	}
	for f, ok := range collecting {
		if !ok {
			t.Errorf("the registry does not subscribe to %s (ADR-0029 §4)", f)
		}
	}
	t.Logf("%d Get requests of the registry to others", gets)
}

// TestD7Level2: D7 `oec-check test --level 2` against the registry
// (ADR-0029 §2, A-389): no test fails and the tests of d7Level2Pass pass.
func TestD7Level2(t *testing.T) {
	regtest.RunD7(t, regtest.D7{
		Exe:      regtest.Registry(t),
		Hostname: "reg-1",
		App:      "Registry/" + il,
		Level:    2,
		Pass:     d7Level2Pass,
		Report:   "OEC_REGISTRY_D7_REPORT",
	})
}

// d7Level2Pass: the 44 D7 tests that pass against the registry (C-086).
// D7 skips nine: T7-MON-07, T7-MON-17 and T7-ADR-02 judge requests of the
// CUT to others or a second namespace (none); T7-MON-09 and T7-MON-14
// judge Data, Metadata and cyclic publications (none); T7-SET-05 and
// T7-SET-10 need a Config DataSet and a ReferenceDesignation object (the
// registry has neither at L2); T7-SET-04 needs a Del D7 can restore;
// T7-SET-03 is Level 3. T7-GET-04 warns REQ-CMP-010 and T7-MON-04 reports
// REQ-CMP-045 (info): the registry lists D7 and its peer as available
// assets (MG A1), and D7 takes every further Source of Get MAM for a
// represented entity – the D7 refinement for registries of ADR-0029 §5
// (A-388, increment 2).
var d7Level2Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-13", "T7-MON-15", "T7-MON-16", "T7-MON-18", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-01", "T7-ADR-03",
	"T7-GET-01", "T7-GET-02", "T7-GET-03", "T7-GET-05", "T7-GET-06", "T7-GET-07", "T7-GET-08", "T7-GET-09",
	"T7-SET-01", "T7-SET-02", "T7-SET-06", "T7-SET-07", "T7-SET-08", "T7-SET-09", "T7-SET-11", "T7-SET-12",
	"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04",
}
