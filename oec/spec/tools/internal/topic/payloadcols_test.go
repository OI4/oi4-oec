package topic

import "testing"

func TestCheckPayload(t *testing.T) {
	s := load(t)
	for _, err := range s.CheckPayload(PayloadOptions{}) {
		t.Error(err)
	}
}

func TestCheckPayloadDetectsDrift(t *testing.T) {
	type mutation struct {
		method, resource string
		mutate           func(p *Payload)
	}
	for _, m := range []mutation{
		{"Pub", "MAM", func(p *Payload) { p.ClassID = "publisher-defined" }},
		{"Pub", "Data", func(p *Payload) { p.ClassID = "predefined" }},
		{"Get", "Metadata", func(p *Payload) { p.Content = "metadata-filled" }},
		{"Pub", "MAM", func(p *Payload) { p.Helpers = []string{"PaginationRequest"} }},
		{"Get", "MAM", func(p *Payload) { p.Helpers = append(p.Helpers, "Query") }},
		{"Set", "Data", func(p *Payload) { p.Answer = "pub" }},
		{"Del", "SubscriptionList", func(p *Payload) { p.Answer = "status" }},
		{"Set", "ReferenceDesignation", func(p *Payload) { p.WriterID = "list" }},
		{"Call", "NewDataSetWriterId", func(p *Payload) { p.MessageType = "ua-data" }},
		{"Get", "Profile", func(p *Payload) { p.Reqs = []string{"REQ-XYZ-999"} }},
	} {
		s := load(t)
		found := false
		for i := range s.Matrix.Combinations {
			if c := &s.Matrix.Combinations[i]; c.Method == m.method && c.Resource == m.resource {
				m.mutate(c.Payload)
				found = true
			}
		}
		if !found {
			t.Fatalf("no row %s %s", m.method, m.resource)
		}
		opt := PayloadOptions{Reqs: map[string]bool{}}
		for _, c := range s.Matrix.Combinations {
			if c.Payload != nil {
				for _, r := range c.Payload.Reqs {
					opt.Reqs[r] = r != "REQ-XYZ-999"
				}
			}
		}
		if errs := s.CheckPayload(opt); len(errs) == 0 {
			t.Errorf("mutated payload columns of %s %s passed the check", m.method, m.resource)
		}
	}
	s := load(t)
	for i := range s.Matrix.Combinations {
		if c := &s.Matrix.Combinations[i]; c.Method == "Pub" && c.Resource == "Event" {
			c.Payload = nil
		}
	}
	if errs := s.CheckPayload(PayloadOptions{AreaStatus: map[string]string{"RES-EVENT": "drafted"}}); len(errs) == 0 {
		t.Error("missing payload columns of a drafted area passed the check")
	}
}
