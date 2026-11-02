package bus

import (
	"encoding/json"
	"testing"

	"github.com/OI4/oec/conformance/internal/bus/registrytest"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/validate"
)

// TestComponentMessages: every message a probe component of T8-BUS-03
// publishes – Birth (MAM, Health), the second Health, the Close message
// and the Will – and the Get of D8 to the Registry are conforming OEC
// messages (D1 schemas and rules of D7 validate), so a strict Registry
// accepts them (C-086: the WriterGroupName, MG 8.2.3, REQ-MSG-016, was
// missing, and the SDK consumer of the OEC Registry D5 dropped the
// messages).
func TestComponentMessages(t *testing.T) {
	v, err := validate.New()
	if err != nil {
		t.Fatal(err)
	}
	c := newComponent("oi4.org/oec-infra/D8/test-1")
	for _, tc := range []struct {
		name, role string
		m          mqttwire.Message
	}{
		{"birth MAM", "birth", c.message("MAM", writerMAM, c.mam())},
		{"birth Health", "", c.message("Health", writerHealth, health("NORMAL_0", 100))},
		{"second Health", "", c.message("Health", writerHealth, health("NORMAL_0", 99))},
		{"close", "close", c.message("Health", writerHealth, health("NORMAL_0", 0))},
		{"will", "will", c.message("Health", writerHealth, health("FAILURE_1", 0))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check(t, v, tc.m.Topic, tc.m.Payload, tc.role)
		})
	}
	p := &registryProbe{il: "oi4.org/Registry/SIM/1"}
	for _, resource := range []string{"MAM", "Health"} {
		for _, source := range []string{"", c.il} {
			t.Run("Get "+resource+" "+source, func(t *testing.T) {
				check(t, v, p.topic("Get", resource, source), request(c.mk.next(), c.pid, resource), "")
			})
		}
	}
	t.Run("Status Event of the simulated Registry", func(t *testing.T) {
		m := registrytest.StatusBad(p.il, "1-Registry/"+p.il, "1-Utility/oi4.org/oec-infra/D8/test")
		check(t, v, m.Topic, m.Payload, "")
	})
}

func check(t *testing.T, v *validate.Validator, topic string, payload []byte, role string) {
	t.Helper()
	res, err := v.Validate(validate.Message{Topic: topic, Payload: json.RawMessage(payload), Role: role})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Errorf("%s: %+v\n%s", topic, res.Findings, payload)
	}
}
