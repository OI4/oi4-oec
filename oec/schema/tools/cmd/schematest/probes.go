package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	schema "github.com/OI4/oec/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// probe is a synthetic mutation of a payload vector. Probes are no catalog
// vectors (no reviewed expectation, no oracle run); they only check that the
// PoC schemas reject the mutation with the REQ of the "$comment" that the
// schema author intended, for paths no reviewed vector exercises yet. Gaps
// found this way become reviewed vectors in the P2 increments.
type probe struct {
	id     string
	base   string // payload vector the mutation starts from
	rule   string // topic form (entry schema)
	mutate func(m map[string]any)
	want   []string // exact REQ set reported by the entry schema
}

func msgs(m map[string]any) []any { return m["Messages"].([]any) }

func validPagination() map[string]any {
	return map[string]any{
		"DataSetWriterId":   json.Number("2"),
		"WriterGroupName":   "Pagination",
		"DataSetWriterName": "provider.com/FieldDataService/FDS-001/1200-0345",
		"Payload": map[string]any{
			"TotalCount": json.Number("1"), "PerPage": json.Number("25"), "Page": json.Number("1"),
			"HasNext": false, "PaginationId": "1639693510094-OTConnector/provider.com/FieldDataService/FDS-001/1200-0345",
		},
	}
}

var probes = []probe{
	{"P-01 Pagination fixed", "V-PAY-124", "pub-health", func(m map[string]any) {
		msgs(m)[1].(map[string]any)["Payload"].(map[string]any)["HasNext"] = false
	}, nil},
	{"P-02 two Paginations", "V-PAY-124", "pub-health", func(m map[string]any) {
		m["Messages"] = []any{msgs(m)[0], validPagination(), validPagination()}
	}, []string{"REQ-PAG-008"}},
	{"P-03 PaginationRequest in Pub", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["Messages"] = append(msgs(m), map[string]any{"DataSetWriterId": json.Number("1"), "WriterGroupName": "PaginationRequest",
			"DataSetWriterName": "x", "Payload": map[string]any{"PerPage": json.Number("5"), "Page": json.Number("1")}})
	}, []string{"REQ-PAG-008"}},
	{"P-04 Query in Get", "V-PAY-001", "get-health", func(m map[string]any) {
		m["Messages"] = []any{map[string]any{"DataSetWriterId": json.Number("9"), "WriterGroupName": "Query", "DataSetWriterName": "x", "Payload": map[string]any{}}}
	}, []string{"REQ-QRY-001"}},
	{"P-05 reserved DataSetWriterId 5", "V-PAY-003", "pub-health", func(m map[string]any) {
		msgs(m)[0].(map[string]any)["DataSetWriterId"] = json.Number("5")
	}, []string{"REQ-MSG-020"}},
	{"P-06 MessageType ua-metadata with Messages", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["MessageType"] = "ua-metadata"
	}, []string{"REQ-MSG-002"}},
	{"P-07 unknown key in Health", "V-PAY-003", "pub-health", func(m map[string]any) {
		msgs(m)[0].(map[string]any)["Payload"].(map[string]any)["Vendor"] = "x"
	}, []string{"REQ-MSG-001"}},
	{"P-08 two PaginationRequests", "V-PAY-001", "get-health", func(m map[string]any) {
		pr := map[string]any{"DataSetWriterId": json.Number("1"), "WriterGroupName": "PaginationRequest",
			"DataSetWriterName": "ITConnector/https://id.consumer.com/MEService/Inst07322", "Payload": map[string]any{"PerPage": json.Number("5"), "Page": json.Number("1")}}
		m["Messages"] = []any{pr, pr}
	}, []string{"REQ-PAG-008"}},
	{"P-09 PerPage 0", "V-PAY-001", "get-health", func(m map[string]any) {
		m["Messages"] = []any{map[string]any{"DataSetWriterId": json.Number("1"), "WriterGroupName": "PaginationRequest",
			"DataSetWriterName": "ITConnector/https://id.consumer.com/MEService/Inst07322", "Payload": map[string]any{"PerPage": json.Number("0"), "Page": json.Number("1")}}}
	}, []string{"REQ-PAG-001"}},
	{"P-10 helper with wrong WriterGroupName", "V-PAY-124", "pub-health", func(m map[string]any) {
		p := validPagination()
		p["WriterGroupName"] = "Paging"
		m["Messages"] = []any{msgs(m)[0], p}
	}, []string{"REQ-MSG-019"}},
	{"P-11 empty Pub Health", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["Messages"] = []any{}
	}, []string{"REQ-RES-HEALTH-007"}},
	{"P-12 page beyond the last page (Pagination only)", "V-PAY-124", "pub-health", func(m map[string]any) {
		p := validPagination()
		p["Payload"].(map[string]any)["Page"] = json.Number("2")
		m["Messages"] = []any{p}
	}, nil},
	{"P-13 scalar DataSetMessage", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["Messages"] = append(msgs(m), json.Number("2"))
	}, []string{"REQ-MSG-016"}},
	{"P-14 Details is no array", "V-PAY-003", "pub-health", func(m map[string]any) {
		msgs(m)[0].(map[string]any)["Payload"].(map[string]any)["Details"] = "FAILURE_1"
	}, []string{"REQ-RES-HEALTH-001"}},
	{"P-15 Timestamp with month 13", "V-PAY-003", "pub-health", func(m map[string]any) {
		msgs(m)[0].(map[string]any)["Timestamp"] = "2026-13-01T00:00:00Z"
	}, []string{"REQ-MSG-016"}},
	{"P-16 ReplyTo without PublisherId", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["ReplyTo"] = "1639693510094-abc"
	}, []string{"REQ-MSG-015"}},
	{"P-17 Locale in Pub", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["Messages"] = append(msgs(m), map[string]any{"DataSetWriterId": json.Number("3"), "WriterGroupName": "Locale",
			"DataSetWriterName": msgs(m)[0].(map[string]any)["DataSetWriterName"], "Payload": map[string]any{"Locale": "de-DE"}})
	}, []string{"REQ-LOC-003"}},
	{"P-18 two Locales in Get", "V-PAY-001", "get-health", func(m map[string]any) {
		loc := map[string]any{"DataSetWriterId": json.Number("3"), "WriterGroupName": "Locale",
			"DataSetWriterName": "https://id.consumer.com/MEService/Inst07322", "Payload": map[string]any{"Locale": "de-DE"}}
		m["Messages"] = []any{loc, loc}
	}, []string{"REQ-LOC-003"}},
}

// runProbes validates every probe against its entry schema and appends the
// probes to the cases of the second validator.
func runProbes(c *jsonschema.Compiler, docs map[string]any, vectors []vector) ([]testCase, []string, error) {
	byID := map[string]vector{}
	for _, v := range vectors {
		byID[v.ID] = v
	}
	var cases []testCase
	var failures []string
	fmt.Println("\n| Probe | Topic form | Expected | Entry schema (B) |")
	fmt.Println("|-------|------------|----------|------------------|")
	for _, p := range probes {
		v, ok := byID[p.base]
		if !ok {
			return nil, nil, fmt.Errorf("probe %s: unknown base vector %s", p.id, p.base)
		}
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(v.Payload))
		if err != nil {
			return nil, nil, err
		}
		p.mutate(inst.(map[string]any))
		entry := schema.BaseURI + "topics/" + p.rule + ".schema.json"
		r, err := validate(c, docs, entry, inst)
		if err != nil {
			return nil, nil, err
		}
		want := "valid"
		if p.want != nil {
			want = "invalid " + fmt.Sprint(p.want)
		}
		fmt.Printf("| %s | %s | %s | %s |\n", p.id, p.rule, want, verdict(r))
		raw, err := json.Marshal(inst)
		if err != nil {
			return nil, nil, err
		}
		cases = append(cases, testCase{ID: p.id, Schema: entry, Instance: raw, Valid: r.valid})
		switch {
		case p.want == nil && !r.valid:
			failures = append(failures, fmt.Sprintf("%s: rejected: %v", p.id, r.msgs))
		case p.want != nil && (r.valid || !slices.Equal(r.reqs, p.want)):
			failures = append(failures, fmt.Sprintf("%s: want %v, got %s: %v", p.id, p.want, verdict(r), r.msgs))
		}
	}
	return cases, failures, nil
}
