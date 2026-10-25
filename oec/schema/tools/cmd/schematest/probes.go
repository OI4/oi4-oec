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
// schemas reject the mutation with the REQ of the "$comment" that the
// schema author intended, for paths no reviewed vector exercises yet. Gaps
// found this way become reviewed vectors in the P2 increments.
type probe struct {
	id     string
	base   string // payload vector the mutation starts from
	rule   string // topic form (entry schema); bareMetaData for a bare MetaData, anyMessage for message.schema.json
	mutate func(m map[string]any)
	want   []string // exact REQ set reported by the entry schema
}

// bareMetaData is the probe "rule" of a bare DataSetMetaDataType (MG B2).
const bareMetaData = "(bare) ResourceMetaData"

// anyMessage is the probe "rule" of the topic-agnostic message schema
// (ADR-0019 option A), which dispatches by MessageType, DataSetClassId and
// the DataSetWriterId of a common service.
const anyMessage = "(message) topic-agnostic"

func item(m map[string]any) map[string]any { return msgs(m)[0].(map[string]any) }

func payload(m map[string]any) map[string]any { return item(m)["Payload"].(map[string]any) }

func msgs(m map[string]any) []any { return m["Messages"].([]any) }

func meta(m map[string]any) map[string]any { return m["MetaData"].(map[string]any) }

func metaField(m map[string]any, i int) map[string]any {
	return meta(m)["Fields"].([]any)[i].(map[string]any)
}

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
	{"P-11 empty Pub Health (answer to a Get without content, SF-099)", "V-PAY-003", "pub-health", func(m map[string]any) {
		m["Messages"] = []any{}
	}, nil},
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
	{"P-19 NodeId i= at the UInt32 bounds", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		metaField(m, 0)["DataType"] = "i=4294967295"
		metaField(m, 1)["DataType"] = "nsu=http://vendor.com/UA/;i=0003001"
		metaField(m, 2)["DataType"] = "nsu=http://vendor.com/UA/;s=Mode;1"
		metaField(m, 3)["DataType"] = "g=5b0e8a52-3c1f-4a8e-9d27-6f41c2b7e904"
	}, nil},
	{"P-20 NodeId i= beyond UInt32", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		metaField(m, 0)["DataType"] = "i=4294967296"
	}, []string{"REQ-MSG-039"}},
	{"P-21 NodeId as JSON object", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		metaField(m, 0)["DataType"] = map[string]any{"Id": json.Number("11")}
	}, []string{"REQ-MSG-037"}},
	{"P-22 QualifiedName in the index form", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		meta(m)["SimpleDataTypes"].([]any)[0].(map[string]any)["Name"] = "2:Percent"
	}, []string{"REQ-MSG-045"}},
	{"P-23 reserved DataSetClassId in the MetaData", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		meta(m)["DataSetClassId"] = "2ae0505e-2830-4980-b65e-0bbdf08e2d45"
	}, []string{"REQ-MSG-014"}},
	{"P-24 MetaData DataSetWriterId 9", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		m["DataSetWriterId"] = json.Number("9")
	}, []string{"REQ-MSG-020"}},
	{"P-25 ExtensionObject Variant without UaTypeId", "V-PAY-121", "pub-metadata", func(m map[string]any) {
		v := metaField(m, 0)["Properties"].([]any)[0].(map[string]any)["Value"].(map[string]any)["Value"].(map[string]any)
		delete(v, "UaTypeId")
	}, []string{"REQ-MSG-041"}},
	{"P-26 EnumField Value with a leading zero", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		meta(m)["EnumDataTypes"].([]any)[0].(map[string]any)["EnumDefinition"].(map[string]any)["Fields"].([]any)[1].(map[string]any)["Value"] = "01"
	}, []string{"REQ-MSG-044"}},
	{"P-27 EnumDescription BuiltInType Int16", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		meta(m)["EnumDataTypes"].([]any)[0].(map[string]any)["BuiltInType"] = json.Number("4")
	}, []string{"REQ-MSG-044"}},
	{"P-28 unknown key in a StructureField", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		f := meta(m)["StructureDataTypes"].([]any)[0].(map[string]any)["StructureDefinition"].(map[string]any)["Fields"].([]any)[0].(map[string]any)
		f["Unit"] = "mm"
	}, []string{"REQ-MSG-001"}},
	{"P-29 Pub Metadata MetaData is no object", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		m["MetaData"] = "axis"
	}, []string{"REQ-MSG-035"}},
	{"P-30 MessageType ua-data in a ua-metadata", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		m["MessageType"] = "ua-data"
	}, []string{"REQ-MSG-002"}},
	{"P-31 resource MetaData with the DataSetClassId of another resource", "V-PAY-011", bareMetaData, func(m map[string]any) {
		m["DataSetClassId"] = "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58"
	}, []string{"REQ-MSG-010"}},
	{"P-32 publisher-defined bare MetaData without DataSetClassId", "V-PAY-011", bareMetaData, func(m map[string]any) {
		m["Name"] = "oee"
		delete(m, "DataSetClassId")
	}, nil},
	{"P-33 FieldMetaData ValueRank beyond Int32", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		metaField(m, 0)["ValueRank"] = json.Number("2147483648")
	}, []string{"REQ-MSG-037"}},
	{"P-34 LocalizedText with a lower-case region", "V-PAY-152", "pub-metadata", func(m map[string]any) {
		meta(m)["Description"].(map[string]any)["Locale"] = "en-us"
	}, []string{"REQ-MSG-040"}},
	{"P-35 empty Pub MAM (SF-099)", "V-PAY-028", "pub-mam", func(m map[string]any) {
		m["Messages"] = []any{}
	}, nil},
	{"P-36 empty Pub Profile (SF-099)", "V-PAY-032", "pub-profile", func(m map[string]any) {
		m["Messages"] = []any{}
	}, nil},
	{"P-37 Reply with an Uncertain Status keeps its response fields", "V-PAY-009", "reply-newdatasetwriterid", func(m map[string]any) {
		item(m)["Status"] = map[string]any{"Code": json.Number("1073741824")}
	}, nil},
	{"P-38 Reply with the lowest Bad Status and response fields", "V-PAY-009", "reply-newdatasetwriterid", func(m map[string]any) {
		item(m)["Status"] = map[string]any{"Code": json.Number("2147483648")}
	}, []string{"REQ-ACT-012"}},
	{"P-39 message: Call NewDataSetWriterId for an unknown Resource", "V-PAY-008", anyMessage, func(m map[string]any) {
		payload(m)["Resource"] = "Status"
	}, []string{"REQ-ACT-021"}},
	{"P-40 message: Reply NewDataSetWriterId with the reserved DataSetWriterId 8", "V-PAY-009", anyMessage, func(m map[string]any) {
		payload(m)["DataSetWriterId"] = json.Number("8")
	}, []string{"REQ-ACT-022"}},
	{"P-41 message: Reply with a Bad Status and response fields", "V-PAY-009", anyMessage, func(m map[string]any) {
		item(m)["Status"] = map[string]any{"Code": json.Number("2158690304")}
	}, []string{"REQ-ACT-012"}},
	{"P-42 message: Call of a publisher-defined Action", "V-PAY-008", anyMessage, func(m map[string]any) {
		item(m)["DataSetWriterId"] = json.Number("100")
		item(m)["Payload"] = map[string]any{"Speed": json.Number("3")}
	}, nil},
	{"P-43 message: Pub MAM without SerialNumber", "V-PAY-028", anyMessage, func(m map[string]any) {
		delete(payload(m), "SerialNumber")
	}, []string{"REQ-RES-MAM-001"}},
	{"P-44 message: Pub Profile with an unknown resource", "V-PAY-032", anyMessage, func(m map[string]any) {
		payload(m)["Resources"] = []any{"MAM", "Health", "Status"}
	}, []string{"REQ-RES-PROFILE-001"}},
	{"P-45 empty Pub PublicationList (SF-099)", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		m["Messages"] = []any{}
	}, nil},
	{"P-46 empty Pub ReferenceDesignation (SF-099)", "V-PAY-043", "pub-referencedesignation", func(m map[string]any) {
		m["Messages"] = []any{}
	}, nil},
	{"P-47 entry Filter with an unencoded +", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Filter"] = "o+e"
	}, []string{"REQ-RES-PUBLIST-001"}},
	{"P-48 Data entry Filter with two levels", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Filter"] = "oee/line"
	}, []string{"REQ-RES-PUBLIST-002"}},
	{"P-49 Event entry Filter without EventLevel", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Resource"] = "Event"
		payload(m)["Filter"] = "Status/Fine"
		payload(m)["Mode"] = "FILTER_4"
	}, []string{"REQ-RES-PUBLIST-002"}},
	{"P-50 MAM entry with a Filter", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Resource"] = "MAM"
	}, []string{"REQ-RES-PUBLIST-002"}},
	{"P-51 Event entry with the Mode ON_REQUEST_1", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Resource"] = "Event"
		payload(m)["Filter"] = "Status/Good"
		payload(m)["Mode"] = "ON_REQUEST_1"
	}, []string{"REQ-RES-PUBLIST-012"}},
	{"P-52 Metadata entry without Mode (default ON_REQUEST_1)", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Resource"] = "Metadata"
		delete(payload(m), "Mode")
	}, []string{"REQ-RES-PUBLIST-012"}},
	{"P-53 entry with a numeric Mode", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["Mode"] = json.Number("8")
	}, []string{"REQ-MSG-031", "REQ-RES-PUBLIST-001"}},
	{"P-54 entry describing the reserved DataSetWriterId 5", "V-PAY-037", "pub-publicationlist", func(m map[string]any) {
		payload(m)["DataSetWriterId"] = json.Number("5")
	}, []string{"REQ-MSG-020"}},
	{"P-55 Query in Get PublicationList", "V-PAY-034", "get-publicationlist", func(m map[string]any) {
		m["Messages"] = []any{map[string]any{"DataSetWriterId": json.Number("9"), "WriterGroupName": "Query", "DataSetWriterName": "x", "Payload": map[string]any{}}}
	}, []string{"REQ-QRY-001"}},
	{"P-56 Set PublicationList with a SequenceNumber", "V-PAY-040", "set-publicationlist", func(m map[string]any) {
		item(m)["SequenceNumber"] = json.Number("4")
	}, []string{"REQ-BUS-010"}},
	{"P-57 Set PublicationList with a PaginationRequest", "V-PAY-040", "set-publicationlist", func(m map[string]any) {
		m["Messages"] = append(msgs(m), map[string]any{"DataSetWriterId": json.Number("1"), "WriterGroupName": "PaginationRequest",
			"DataSetWriterName": "x", "Payload": map[string]any{"PerPage": json.Number("5"), "Page": json.Number("1")}})
	}, []string{"REQ-PAG-008", "REQ-PAG-009"}},
	{"P-58 two Queries in Get ReferenceDesignation", "V-PAY-053", "get-referencedesignation", func(m map[string]any) {
		m["Messages"] = []any{msgs(m)[0], msgs(m)[0]}
	}, []string{"REQ-QRY-001"}},
	{"P-59 Query with an empty Parent", "V-PAY-053", "get-referencedesignation", func(m map[string]any) {
		payload(m)["Parent"] = ""
	}, []string{"REQ-RES-REFDES-012"}},
	{"P-60 Locale in Del ReferenceDesignation", "V-PAY-046", "del-referencedesignation", func(m map[string]any) {
		m["Messages"] = []any{map[string]any{"DataSetWriterId": json.Number("3"), "WriterGroupName": "Locale", "DataSetWriterName": "x", "Payload": map[string]any{"Locale": "de-DE"}}}
	}, []string{"REQ-LOC-003"}},
	{"P-61 Pub ReferenceDesignation with a WriterGroupName", "V-PAY-043", "pub-referencedesignation", func(m map[string]any) {
		item(m)["WriterGroupName"] = "Function"
	}, []string{"REQ-MSG-024"}},
	{"P-62 message: Pub PublicationList with an unknown Resource", "V-PAY-037", anyMessage, func(m map[string]any) {
		payload(m)["Resource"] = "Status"
	}, []string{"REQ-RES-PUBLIST-001"}},
	{"P-63 message: Pub ReferenceDesignation without Value", "V-PAY-043", anyMessage, func(m map[string]any) {
		delete(payload(m)["Function"].(map[string]any), "Value")
	}, []string{"REQ-RES-REFDES-001"}},
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
		switch p.rule {
		case bareMetaData:
			entry = schema.BaseURI + "metadata.schema.json#/$defs/ResourceMetaData"
		case anyMessage:
			entry = schema.BaseURI + "message.schema.json"
		}
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
