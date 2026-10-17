package vectors

import (
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/topic"
)

const specDir = "../../.."

func TestSpecVectors(t *testing.T) {
	ts, err := topic.Load(specDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := Load(specDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no vector files")
	}
	if errs := Check(files, ts, Options{Baseline: files[0].Baseline}); len(errs) > 0 {
		t.Fatal(errs)
	}
	n := Count(files)
	for _, k := range []string{"topic", "identification-link", "dnp-encoding", "oi4identifier", "payload"} {
		if n[k] == 0 {
			t.Errorf("no vectors of kind %s", k)
		}
	}
}

func TestCheckDetectsMismatches(t *testing.T) {
	ts, err := topic.Load(specDir)
	if err != nil {
		t.Fatal(err)
	}
	const p = "cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU"
	il := "company.id/A87BAD34H"
	good := Vector{
		ID: "V-TOP-001", Title: "t", Expected: "valid", Reqs: []string{"REQ-TOP-001"},
		Origin: Origin{Kind: "guideline", Ref: "MG 7"}, Status: "draft",
		Topic: "Oi4v2/OTConnector/" + p + "/Pub/Health",
		Parse: &Parse{Rule: "pub-health", ServiceType: "OTConnector", AppID: p,
			AppIDIL: "provider.com/FieldDataService/FDS-001/1200-0345", Method: "Pub", Resource: "Health"},
	}
	sections := map[string]string{"MG 7": "Example: Oi4v2/OTConnector/" + p + "/Pub/Health"}
	opt := Options{Baseline: "abc1234", Reqs: map[string]bool{"REQ-TOP-001": true, "REQ-IL-018": true}, Sections: sections}
	run := func(kind string, v Vector) []error {
		return Check([]File{{Kind: kind, Baseline: "abc1234", Path: "x.yaml", Vectors: []Vector{v}}}, ts, opt)
	}
	if errs := run("topic", good); len(errs) > 0 {
		t.Fatalf("good vector rejected: %v", errs)
	}
	cases := []struct {
		name, kind, want string
		mutate           func(v *Vector)
	}{
		{"parse", "topic", "parse result differs", func(v *Vector) { p := *v.Parse; p.Rule = "get-health"; v.Parse = &p }},
		{"verdict", "topic", "vector expects invalid", func(v *Vector) {
			v.Expected, v.Parse, v.Reqs, v.Violates, v.Fails = "invalid", nil, nil, []string{"REQ-TOP-001"}, "grammar"
		}},
		{"warnings", "topic", "warnings", func(v *Vector) { v.Warnings = []string{"whitespace"} }},
		{"req", "topic", "does not exist", func(v *Vector) { v.Reqs = []string{"REQ-TOP-999"} }},
		{"origin", "topic", "not found verbatim", func(v *Vector) {
			v.Origin.Ref = "MG 7"
			v.Topic += "x"
			v.Parse = nil
			v.Expected = "invalid"
			v.Fails = "grammar"
		}},
		{"fails", "topic", "fails the grammar check", func(v *Vector) {
			v.Topic = "Oi4v2/OTConnector/" + p + "/Pub/Bogus"
			v.Origin = Origin{Kind: "constructed"}
			v.Expected, v.Parse, v.Fails = "invalid", nil, "identification-link"
		}},
		{"il", "identification-link", "violations", func(v *Vector) {
			*v = Vector{ID: "V-IL-001", Expected: "invalid", Violations: []string{"uppercase-authority"}, IL: &il, Origin: Origin{Kind: "constructed"}}
		}},
		{"encoding", "identification-link", "encoding of", func(v *Vector) {
			*v = Vector{ID: "V-IL-001", Expected: "valid", IL: &il, Encoded: "Y29tcGFueS5pZC9BODdCQUQzNEh", Origin: Origin{Kind: "constructed"}}
		}},
		{"dnp", "dnp-encoding", "dnp-encoding of", func(v *Vector) {
			*v = Vector{ID: "V-DNP-001", Expected: "valid", Input: "a b", Output: "a,20b", Origin: Origin{Kind: "constructed"}}
			v.Output = "a%20b"
		}},
		{"oid", "oi4identifier", "Oi4Identifier is", func(v *Vector) {
			*v = Vector{ID: "V-OID-001", Expected: "valid", ManufacturerURI: "Example.com", Model: "M", ProductCode: "P", SerialNumber: "S",
				Oi4Identifier: "Example.com/M/P/S", Origin: Origin{Kind: "constructed"}}
		}},
	}
	for _, c := range cases {
		v := good
		c.mutate(&v)
		errs := run(c.kind, v)
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected an error containing %q, got %v", c.name, c.want, errs)
		}
	}

	dup := []File{{Kind: "topic", Baseline: "other", Path: "x.yaml", Vectors: []Vector{good, good}}}
	errs := Check(dup, ts, opt)
	var sawDup, sawBase bool
	for _, e := range errs {
		sawDup = sawDup || strings.Contains(e.Error(), "duplicate id")
		sawBase = sawBase || strings.Contains(e.Error(), "baseline")
	}
	if !sawDup || !sawBase {
		t.Errorf("expected duplicate id and baseline errors, got %v", errs)
	}
}

func TestCheckPayload(t *testing.T) {
	ts, err := topic.Load(specDir)
	if err != nil {
		t.Fatal(err)
	}
	const (
		appID = "cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU"
		pub   = "OTConnector/provider.com/FieldDataService/FDS-001/1200-0345"
	)
	tp := "Oi4v2/OTConnector/" + appID + "/Pub/Health"
	msg := `{"MessageId": "1-` + pub + `", "MessageType": "ua-data", "PublisherId": "` + pub + `",
  "DataSetClassId": "d8e7b6df-42ba-448a-975a-199f59e8ffeb", "ReplyTo": "",
  "Messages": [{"DataSetWriterId": 10, "WriterGroupName": "", "DataSetWriterName": "x.com/a", "Payload": {}}]}`
	orig := strings.Replace(msg, `"Payload": {}}]`, `"Payload": {}}, {
    // more
  }]`, 1)
	good := Vector{
		ID: "V-PAY-001", Title: "t", Expected: "valid", Reqs: []string{"REQ-MSG-002"},
		Origin: Origin{Kind: "guideline", Ref: "MG B1"}, Status: "draft", Topic: tp, Payload: msg,
	}
	sections := map[string]string{"MG B1": "Topic " + tp + " Example " + guidelineText(msg) + " Other " + guidelineText(orig)}
	opt := Options{Baseline: "abc1234", Reqs: map[string]bool{"REQ-MSG-002": true, "REQ-MSG-020": true}, Sections: sections}
	run := func(v Vector) []error {
		return Check([]File{{Kind: "payload", Baseline: "abc1234", Path: "x.yaml", Vectors: []Vector{v}}}, ts, opt)
	}
	if errs := run(good); len(errs) > 0 {
		t.Fatalf("good vector rejected: %v", errs)
	}
	edited := good
	edited.Payload, edited.Original = msg, orig
	edited.Edits = []Edit{{Target: "/Messages/1", Kind: "removed", Reason: "comment only"}}
	if errs := run(edited); len(errs) > 0 {
		t.Fatalf("edited vector rejected: %v", errs)
	}
	cases := []struct {
		name, want string
		mutate     func(v *Vector)
	}{
		{"payload not verbatim", "payload not found verbatim", func(v *Vector) { v.Payload = strings.Replace(v.Payload, `"ReplyTo": ""`, `"ReplyTo": "1-x"`, 1) }},
		{"topic not verbatim", "topic \"Oi4v2/OTConnector/", func(v *Vector) { v.Topic = "Oi4v2/OTConnector/" + appID + "/Get/Health" }},
		{"invalid topic", "payload vectors need a valid topic", func(v *Vector) { v.Topic = "Oi4v2/Bogus" }},
		{"no json", "no strict JSON", func(v *Vector) { v.Payload = `{"a": 1,}` }},
		{"problems", "payload violates [REQ-MSG-020]", func(v *Vector) {
			v.Payload = strings.Replace(v.Payload, `"DataSetWriterId": 10`, `"DataSetWriterId": 5`, 1)
		}},
		{"violates", "vector says [REQ-MSG-002]", func(v *Vector) {
			v.Expected, v.Reqs, v.Violates, v.Reason = "invalid", nil, []string{"REQ-MSG-002"}, "wrong"
			v.Origin = Origin{Kind: "constructed"}
			v.Payload = strings.Replace(v.Payload, `"DataSetWriterId": 10`, `"DataSetWriterId": 5`, 1)
		}},
		{"original without pointer edit", "original is required exactly", func(v *Vector) {
			v.Original = orig
			v.Edits = []Edit{{Target: "topic", Kind: "completed", Reason: "no topic"}}
		}},
		{"pointer edit without original", "original is required exactly", func(v *Vector) {
			v.Edits = []Edit{{Target: "/Messages/0", Kind: "corrected", Reason: "x"}}
		}},
		{"removed target", "does not exist in the original", func(v *Vector) {
			v.Original = orig
			v.Edits = []Edit{{Target: "/Messages/2", Kind: "removed", Reason: "x"}}
		}},
		{"corrected target", "does not exist in the payload", func(v *Vector) {
			v.Original = orig
			v.Edits = []Edit{{Target: "/Messages/1", Kind: "corrected", Reason: "x"}}
		}},
		{"original equals payload", "payload equals the original", func(v *Vector) {
			v.Original = msg
			v.Edits = []Edit{{Target: "/Messages/0", Kind: "corrected", Reason: "x"}}
		}},
	}
	for _, c := range cases {
		v := good
		c.mutate(&v)
		found := false
		errs := run(v)
		for _, e := range errs {
			found = found || strings.Contains(e.Error(), c.want)
		}
		if !found {
			t.Errorf("%s: expected an error containing %q, got %v", c.name, c.want, errs)
		}
	}
}

func guidelineText(s string) string { return guideline.NormalizeForCompare(s) }
