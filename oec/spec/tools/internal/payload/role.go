package payload

import (
	"encoding/json"
	"fmt"
)

// Roles maps a declared message role (Q-020 (a), C-035) to the requirement
// on the content of such a message: Birth (MG 6.3.3, REQ-LIFE-003), Close
// (MG 6.3.4, REQ-LIFE-007) and Will (MG 6.3.5, REQ-LIFE-009). A role is not
// visible in a single message; it is declared by the vector (field role)
// or by the caller of D7 validate (--role).
var Roles = map[string]string{
	"birth": "REQ-LIFE-003",
	"close": "REQ-LIFE-007",
	"will":  "REQ-LIFE-009",
}

// CheckRole checks the single-message rules of a declared role in addition
// to CheckMode: the message is a Pub MAM (Birth) or Pub Health (Close, Will)
// with exactly one DataSetMessage; the MAM of a Birth is the one of the
// component itself (ProductInstanceUri = AppId), the Health of a Close is
// NORMAL_0 with HealthScore 0, the Health of a Will FAILURE_1 with
// HealthScore 0. The topic rules of the roles (Source = AppId, REQ-LIFE-002/
// 006/008) and the DataSetWriterId shared with the regular messages are
// multi-message or connection rules and stay with D7 monitor/test. Errors
// of the envelope are left to CheckMode.
func CheckRole(t Topic, msg any, role string) ([]Problem, error) {
	req, ok := Roles[role]
	if !ok {
		return nil, fmt.Errorf("unknown message role %q", role)
	}
	c := &checker{t: t}
	resource := "Health"
	if role == "birth" {
		resource = "MAM"
	}
	if t.Method != "Pub" || t.Resource != resource {
		c.add("", req, "a %s message is a Pub %s, not a %s %s", role, resource, t.Method, t.Resource)
		return c.sorted(), nil
	}
	m, _ := msg.(map[string]any)
	dsms, ok := m["Messages"].([]any)
	if !ok {
		return nil, nil
	}
	if len(dsms) != 1 {
		c.add("/Messages", req, "a %s message has exactly one DataSetMessage, not %d", role, len(dsms))
		return c.sorted(), nil
	}
	dm, _ := dsms[0].(map[string]any)
	p, ok := dm["Payload"].(map[string]any)
	if !ok {
		return nil, nil
	}
	const ptr = "/Messages/0/Payload"
	switch role {
	case "birth":
		if s, ok := p["ProductInstanceUri"].(string); ok && s != t.AppIL {
			c.add(join(ptr, "ProductInstanceUri"), req, "the Birth message carries the MAM of %q, not of the component itself %q", s, t.AppIL)
		}
	case "close":
		c.roleHealth(ptr, p, req, "NORMAL_0")
	case "will":
		c.roleHealth(ptr, p, req, "FAILURE_1")
	}
	return c.sorted(), nil
}

func (c *checker) roleHealth(ptr string, p map[string]any, req, health string) {
	if h, _ := p["Health"].(string); h != health {
		c.add(join(ptr, "Health"), req, "Health is %q, the message role requires %s", p["Health"], health)
	}
	n, ok := p["HealthScore"].(json.Number)
	if !ok {
		c.add(join(ptr, "HealthScore"), req, "HealthScore is missing or no number, the message role requires 0")
		return
	}
	if f, err := n.Float64(); err != nil || f != 0 {
		c.add(join(ptr, "HealthScore"), req, "HealthScore is %s, the message role requires 0", n)
	}
}
