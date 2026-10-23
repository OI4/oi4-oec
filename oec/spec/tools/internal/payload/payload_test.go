package payload

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	app       = "provider.com/FieldDataService/FDS-001/1200-0345"
	requestor = "ITConnector/https://id.consumer.com/MEService/Inst07322"
)

var healthPub = `{
  "MessageId": "1639693510094-OTConnector/` + app + `",
  "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "d8e7b6df-42ba-448a-975a-199f59e8ffeb",
  "ReplyTo": "",
  "Messages": [{
    "DataSetWriterId": 3456, "WriterGroupName": "", "DataSetWriterName": "` + app + `",
    "SequenceNumber": 58, "Timestamp": "2021-12-16T22:25:10.094Z",
    "Status": {"Code": 1083310080, "Symbol": "UncertainLastUsableValue"},
    "Payload": {"Health": "NORMAL_0"}
  }]
}`

var dataPub = `{
  "MessageId": "1639693510094-OTConnector/` + app + `",
  "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `",
  "ReplyTo": "",
  "Messages": [{
    "DataSetWriterId": 20, "WriterGroupName": "oee", "DataSetWriterName": "` + app + `",
    "MetaDataVersion": {"MajorVersion": 1, "MinorVersion": 2}, "MessageType": "ua-keyframe",
    "Payload": {"Oee": 0.83}
  }]
}`

var pubList = `{
  "MessageId": "1-OTConnector/` + app + `",
  "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "217434d6-6e1e-4230-b907-f52bc9ffe152",
  "ReplyTo": "",
  "Messages": [
    {"DataSetWriterId": 11, "WriterGroupName": "Data/oee", "DataSetWriterName": "` + app + `",
     "Payload": {"Resource": "Data", "Source": "` + app + `", "Filter": "oee", "DataSetWriterId": 93,
       "Mode": "APPLICATION_SOURCE_FILTER_8", "Interval": 0, "Precisions": {"Oee": 0.5}, "Config": "MODE_AND_INTERVAL_3"}},
    {"DataSetWriterId": 11, "WriterGroupName": "MAM", "DataSetWriterName": "` + app + `",
     "Payload": {"Resource": "MAM", "Source": "` + app + `", "DataSetWriterId": 55, "Mode": "SOURCE_3", "Config": "NONE_0"}},
    {"DataSetWriterId": 11, "WriterGroupName": "Event/Syslog/Notice", "DataSetWriterName": "` + app + `",
     "Payload": {"Resource": "Event", "Source": "` + app + `", "Filter": "Syslog/Notice", "DataSetWriterId": 625, "Mode": "FILTER_4"}},
    {"DataSetWriterId": 2, "WriterGroupName": "Pagination", "DataSetWriterName": "` + app + `",
     "Payload": {"TotalCount": 3, "PerPage": 3, "Page": 1, "HasNext": false, "PaginationId": "1-OTConnector/` + app + `"}}
  ]
}`

var metadataPub = `{
  "MessageId": "1639693510094-OTConnector/` + app + `",
  "MessageType": "ua-metadata",
  "PublisherId": "OTConnector/` + app + `",
  "DataSetWriterId": 20, "WriterGroupName": "oee", "DataSetWriterName": "` + app + `",
  "Timestamp": "2021-12-16T22:25:10.094Z", "ReplyTo": "",
  "MetaData": {
    "Name": "oee", "Namespaces": [], "StructureDataTypes": [], "EnumDataTypes": [], "SimpleDataTypes": [],
    "Description": {"Locale": "en-US", "Text": "OEE"},
    "ConfigurationVersion": {"MajorVersion": 1, "MinorVersion": 2},
    "Fields": [{
      "Name": "Oee", "Description": {"Locale": "en-US", "Text": "OEE"}, "FieldFlags": 0, "BuiltInType": 11,
      "DataType": "i=11", "ValueRank": -1, "ArrayDimensions": [], "MaxStringLength": 0,
      "DataSetFieldId": "0b8e2a3c-55c1-4d5e-9f00-3d6c1d7a2b11",
      "Properties": [{"Key": "Unit", "Value": {"UaType": 12, "Value": "%"}},
        {"Key": "EngineeringUnits", "Value": {"UaType": 22, "Value": {"UaTypeId": "i=887",
          "NamespaceUri": "http://www.opcfoundation.org/UA/units/un/cefact", "UnitId": 5066068,
          "DisplayName": {"Text": "%"}, "Description": {"Text": "percent"}}}}]
    }]
  }
}`

var call = `{
  "MessageId": "1639693510000-` + requestor + `",
  "MessageType": "ua-action-request",
  "PublisherId": "OTConnector/` + app + `",
  "Timestamp": "2021-12-16T22:25:10.000Z",
  "ResponseAddress": "Oi4v2/OTConnector/cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU/Reply/NewDataSetWriterId",
  "RequestorId": "` + requestor + `",
  "TimeoutHint": 10000,
  "CorrelationData": "AQID",
  "Messages": [{"DataSetWriterId": 8, "ActionTargetId": 1, "RequestId": 1, "ActionState": 1, "Payload": {"Resource": "ReferenceDesignation"}}]
}`

var reply = `{
  "MessageId": "1639693510094-OTConnector/` + app + `",
  "MessageType": "ua-action-response",
  "PublisherId": "OTConnector/` + app + `",
  "Timestamp": "2021-12-16T22:25:10.094Z",
  "RequestorId": "` + requestor + `",
  "Messages": [{"DataSetWriterId": 8, "ActionTargetId": 1, "Status": {}, "RequestId": 1, "ActionState": 2, "Payload": {"DataSetWriterId": 57211, "Ttl": 10}}]
}`

var (
	tHealth   = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "Health", SourceIL: app}
	tData     = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "Data", SourceIL: app, Filter: "oee"}
	tPubList  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "PublicationList"}
	tMetadata = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "Metadata", SourceIL: app, Filter: "oee"}
	tCall     = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Call", Resource: "NewDataSetWriterId"}
	tReply    = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Reply", Resource: "NewDataSetWriterId"}
)

func reqs(t *testing.T, top Topic, msg string) []string {
	t.Helper()
	v, err := Parse([]byte(msg))
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, msg)
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range Check(top, v) {
		if !seen[p.Req] {
			seen[p.Req] = true
			out = append(out, p.Req)
		}
	}
	sort.Strings(out)
	return out
}

func TestValidMessages(t *testing.T) {
	for name, c := range map[string]struct {
		top Topic
		msg string
	}{
		"health": {tHealth, healthPub}, "data": {tData, dataPub}, "publist": {tPubList, pubList},
		"metadata": {tMetadata, metadataPub}, "call": {tCall, call}, "reply": {tReply, reply},
	} {
		if got := reqs(t, c.top, c.msg); len(got) > 0 {
			v, _ := Parse([]byte(c.msg))
			t.Errorf("%s: unexpected problems %v", name, Check(c.top, v))
		}
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name     string
		top      Topic
		msg      string
		old, new string
		want     []string
	}{
		{"unknown key", tHealth, healthPub, `"ReplyTo": ""`, `"ReplyTo": "", "Foo": 1`, []string{"REQ-MSG-001"}},
		{"message type", tHealth, healthPub, `"ua-data"`, `"ua-metadata"`, []string{"REQ-MSG-002"}},
		{"writer group name in network message", tHealth, healthPub, `"ReplyTo": ""`, `"ReplyTo": "", "WriterGroupName": ""`, []string{"REQ-MSG-003"}},
		{"missing ReplyTo", tHealth, healthPub, `"ReplyTo": "",`, ``, []string{"REQ-MSG-003"}},
		{"messages object", tHealth, healthPub, `"ReplyTo": "",
  "Messages": [{`, `"ReplyTo": "", "Messages": {}, "x": [{`, []string{"REQ-MSG-001", "REQ-MSG-005"}},
		{"publisher differs from topic", tHealth, healthPub, `"PublisherId": "OTConnector/`, `"PublisherId": "Utility/`, []string{"REQ-MSG-006", "REQ-MSG-007"}},
		{"message id", tHealth, healthPub, `"1639693510094-OTConnector`, `"1639693510094-Utility`, []string{"REQ-MSG-007"}},
		{"class id wrong", tHealth, healthPub, `d8e7b6df-42ba-448a-975a-199f59e8ffeb`, `360ca8f3-5e66-42a2-8f10-9cdf45f4bf58`, []string{"REQ-MSG-010"}},
		{"class id upper case", tHealth, healthPub, `d8e7b6df-42ba`, `D8E7B6DF-42BA`, []string{"REQ-MSG-003", "REQ-MSG-010"}},
		{"class id missing", tHealth, healthPub, `"DataSetClassId": "d8e7b6df-42ba-448a-975a-199f59e8ffeb",`, ``, []string{"REQ-MSG-011"}},
		{"class id reserved", tData, dataPub, `"ReplyTo": ""`, `"ReplyTo": "", "DataSetClassId": "2ae0505e-2830-4980-b65e-0bbdf08e2d45"`, []string{"REQ-MSG-014"}},
		{"reply to", tHealth, healthPub, `"ReplyTo": ""`, `"ReplyTo": "x"`, []string{"REQ-MSG-015"}},
		{"dsm publisher id", tHealth, healthPub, `"SequenceNumber": 58`, `"SequenceNumber": 58, "PublisherId": "x"`, []string{"REQ-MSG-016"}},
		{"dsm missing payload", tHealth, healthPub, `,
    "Payload": {"Health": "NORMAL_0"}`, ``, []string{"REQ-MSG-016"}},
		{"timestamp", tHealth, healthPub, `2021-12-16T22:25:10.094Z`, `16.12.2021`, []string{"REQ-MSG-016"}},
		{"delta frame", tData, dataPub, `"MessageType": "ua-keyframe"`, `"MessageType": "ua-deltaframe"`, []string{"REQ-MSG-029", "REQ-MSG-030"}},
		{"keep alive", tData, dataPub, `"MessageType": "ua-keyframe"`, `"MessageType": "ua-keepalive"`, []string{"REQ-MSG-029", "REQ-MSG-030"}},
		{"metadata and minor version", tData, dataPub, `"MessageType": "ua-keyframe"`, `"MinorVersion": 2`, []string{"REQ-MSG-018"}},
		{"helper writer group name", tHealth, healthPub, `"DataSetWriterId": 3456, "WriterGroupName": ""`, `"DataSetWriterId": 3, "WriterGroupName": "Pagination"`, []string{"REQ-LOC-001", "REQ-LOC-003", "REQ-MSG-001", "REQ-MSG-019"}},
		{"reserved writer id", tHealth, healthPub, `"DataSetWriterId": 3456`, `"DataSetWriterId": 8`, []string{"REQ-MSG-020"}},
		{"writer id zero", tHealth, healthPub, `"DataSetWriterId": 3456`, `"DataSetWriterId": 0`, []string{"REQ-MSG-020"}},
		{"writer name differs from source", tHealth, healthPub, `"DataSetWriterName": "provider.com`, `"DataSetWriterName": "x.com`, []string{"REQ-MSG-023"}},
		{"health writer group name", tHealth, healthPub, `"WriterGroupName": ""`, `"WriterGroupName": "x"`, []string{"REQ-MSG-024"}},
		{"data writer group name", tData, dataPub, `"WriterGroupName": "oee"`, `"WriterGroupName": "x"`, []string{"REQ-MSG-024"}},
		{"list writer group name", tPubList, pubList, `"WriterGroupName": "Data/oee"`, `"WriterGroupName": "oee"`, []string{"REQ-MSG-025", "REQ-RES-PUBLIST-002"}},
		{"good status", tHealth, healthPub, `"Code": 1083310080`, `"Code": 0`, []string{"REQ-MSG-026"}},
		{"status without code", tHealth, healthPub, `"Code": 1083310080, `, ``, []string{"REQ-MSG-027"}},
		{"dsm message type", tData, dataPub, `"ua-keyframe"`, `"ua-event"`, []string{"REQ-MSG-029"}},
		{"metadata missing namespaces", tMetadata, metadataPub, `"Namespaces": [], `, ``, []string{"REQ-MSG-035"}},
		{"metadata wrong writer group", tMetadata, metadataPub, `"WriterGroupName": "oee"`, `"WriterGroupName": ""`, []string{"REQ-MSG-024"}},
		{"metadata missing timestamp", tMetadata, metadataPub, `"Timestamp": "2021-12-16T22:25:10.094Z", `, ``, []string{"REQ-MSG-032"}},
		{"configuration version", tMetadata, metadataPub, `"ConfigurationVersion": {"MajorVersion": 1, "MinorVersion": 2}`, `"ConfigurationVersion": {"MajorVersion": -1, "MinorVersion": 2}`, []string{"REQ-MSG-036"}},
		{"field missing properties", tMetadata, metadataPub, `,
      "Properties": [{"Key": "Unit", "Value": {"UaType": 12, "Value": "%"}},`, `, "x": [`, []string{"REQ-MSG-001", "REQ-MSG-037"}},
		{"node id with index", tMetadata, metadataPub, `"DataType": "i=11"`, `"DataType": "ns=1;i=11"`, []string{"REQ-MSG-039"}},
		{"localized text locale", tMetadata, metadataPub, `"Description": {"Locale": "en-US", "Text": "OEE"},
    "Configuration`, `"Description": {"Locale": "english", "Text": "OEE"},
    "Configuration`, []string{"REQ-MSG-040"}},
		{"key value pair", tMetadata, metadataPub, `{"UaType": 12, "Value": "%"}`, `"%"`, []string{"REQ-MSG-041"}},
		{"qualified name index form", tMetadata, metadataPub, `"Key": "Unit"`, `"Key": "1:Unit"`, []string{"REQ-MSG-041"}},
		{"qualified name object form", tMetadata, metadataPub, `"Key": "Unit"`, `"Key": {"Name": "Unit"}`, []string{"REQ-MSG-041"}},
		{"extension object without type id", tMetadata, metadataPub, `"UaTypeId": "i=887",`, ``, []string{"REQ-MSG-041"}},
		{"description without locale", tMetadata, metadataPub, `"Description": {"Locale": "en-US", "Text": "OEE"}, "FieldFlags"`, `"Description": {"Text": "OEE"}, "FieldFlags"`, []string{"REQ-MSG-040"}},
		{"null description", tMetadata, metadataPub, `"Description": {"Locale": "en-US", "Text": "OEE"}, "FieldFlags"`, `"Description": null, "FieldFlags"`, []string{"REQ-MSG-040"}},
		{"action message type", tCall, call, `"ua-action-request"`, `"ua-data"`, []string{"REQ-ACT-001"}},
		{"call without timeout", tCall, call, `"TimeoutHint": 10000,`, ``, []string{"REQ-ACT-002"}},
		{"reply with response address", tReply, reply, `"RequestorId"`, `"TimeoutHint": 5, "RequestorId"`, []string{"REQ-ACT-002"}},
		{"action reply to", tCall, call, `"TimeoutHint": 10000,`, `"TimeoutHint": 10000, "ReplyTo": "",`, []string{"REQ-ACT-003"}},
		{"action request with writer name", tCall, call, `"ActionState": 1,`, `"ActionState": 1, "DataSetWriterName": "x",`, []string{"REQ-ACT-003"}},
		{"call message id of responder", tCall, call, `"1639693510000-ITConnector/https://id.consumer.com/MEService/Inst07322"`, `"1639693510000-OTConnector/` + app + `"`, []string{"REQ-ACT-004"}},
		{"publisher is requestor", tCall, call, `"PublisherId": "OTConnector/` + app + `"`, `"PublisherId": "` + requestor + `"`, []string{"REQ-ACT-005"}},
		{"response address", tCall, call, `/Reply/NewDataSetWriterId"`, `/Call/NewDataSetWriterId"`, []string{"REQ-ACT-006"}},
		{"request state", tCall, call, `"ActionState": 1`, `"ActionState": 2`, []string{"REQ-ACT-010"}},
		{"duplicate request id", tCall, call, `"Payload": {"Resource": "ReferenceDesignation"}}]`, `"Payload": {"Resource": "ReferenceDesignation"}}, {"DataSetWriterId": 8, "ActionTargetId": 1, "RequestId": 1, "ActionState": 1, "Payload": {"Resource": "MAM"}}]`, []string{"REQ-ACT-011"}},
		{"response without status", tReply, reply, `"Status": {}, `, ``, []string{"REQ-ACT-012"}},
		{"bad response with payload", tReply, reply, `"Status": {}`, `"Status": {"Code": 2158690304}`, []string{"REQ-ACT-012"}},
		{"status symbol", tReply, reply, `"Status": {}`, `"Status": {"Symbol": "Good"}`, []string{"REQ-ACT-014"}},
		{"common service writer id", tCall, call, `"DataSetWriterId": 8, "ActionTargetId": 1`, `"DataSetWriterId": 9, "ActionTargetId": 2`, []string{"REQ-ACT-019"}},
		{"request resource", tCall, call, `"Resource": "ReferenceDesignation"`, `"Resource": "Foo"`, []string{"REQ-ACT-021"}},
		{"response reserved id", tReply, reply, `"DataSetWriterId": 57211`, `"DataSetWriterId": 3`, []string{"REQ-ACT-022"}},
	}
	for _, c := range cases {
		if !strings.Contains(c.msg, c.old) {
			t.Fatalf("%s: %q not in message", c.name, c.old)
		}
		msg := strings.Replace(c.msg, c.old, c.new, 1)
		if got := reqs(t, c.top, msg); !reflect.DeepEqual(got, c.want) {
			v, _ := Parse([]byte(msg))
			t.Errorf("%s: got %v, want %v: %v", c.name, got, c.want, Check(c.top, v))
		}
	}
}

func TestGetPublisherIsSender(t *testing.T) {
	get := `{"MessageId": "1-` + requestor + `", "MessageType": "ua-data", "PublisherId": "` + requestor + `",
	  "DataSetClassId": "d8e7b6df-42ba-448a-975a-199f59e8ffeb", "ReplyTo": "", "Messages": []}`
	top := tHealth
	top.Method = "Get"
	if got := reqs(t, top, get); len(got) > 0 {
		t.Errorf("Get of another application rejected: %v", got)
	}
}

func TestParseStrict(t *testing.T) {
	for _, bad := range []string{`{"a": 1, "a": 2}`, `{"a": {"b": 1, "b": 1}}`, `{"a": 1} {}`, `{"a": 1,}`} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if _, err := Parse([]byte(`{"a": [{"b": 1}, {"b": 2}], "b": {"a": 1}}`)); err != nil {
		t.Errorf("valid JSON rejected: %v", err)
	}
}

func TestResolve(t *testing.T) {
	v, _ := Parse([]byte(`{"a/b": [1, {"~c": true}]}`))
	for p, ok := range map[string]bool{"": true, "/a~1b": true, "/a~1b/1/~0c": true, "/a~1b/2": false, "/a~1b/01": false, "/x": false, "a": false} {
		if _, got := Resolve(v, p); got != ok {
			t.Errorf("Resolve(%q) = %v, want %v", p, got, ok)
		}
	}
}

func TestModes(t *testing.T) {
	for _, c := range []struct {
		name     string
		top      Topic
		msg      string
		old, new string
		strict   []string
	}{
		{"network message", tHealth, healthPub, `"ReplyTo": ""`, `"ReplyTo": "", "Foo": 1`, []string{"REQ-MSG-001"}},
		{"dataset message", tHealth, healthPub, `"SequenceNumber": 58`, `"SequenceNumber": 58, "Foo": 1`, []string{"REQ-MSG-001"}},
		{"field metadata", tMetadata, metadataPub, `"FieldFlags": 0`, `"FieldFlags": 0, "Foo": 1`, []string{"REQ-MSG-001"}},
		{"action", tCall, call, `"TimeoutHint": 10000`, `"TimeoutHint": 10000, "Foo": 1`, []string{"REQ-ACT-003"}},
	} {
		msg := strings.Replace(c.msg, c.old, c.new, 1)
		v, err := Parse([]byte(msg))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := reqs(t, c.top, msg); !reflect.DeepEqual(got, c.strict) {
			t.Errorf("%s strict: got %v, want %v", c.name, got, c.strict)
		}
		if ps := CheckMode(c.top, v, Tolerant); len(ps) > 0 {
			t.Errorf("%s tolerant: unexpected problems %v", c.name, ps)
		}
	}
	// Tolerant mode still reports missing and malformed defined keys.
	msg := strings.Replace(healthPub, `"ReplyTo": "",`, `"Foo": 1,`, 1)
	v, _ := Parse([]byte(msg))
	if ps := CheckMode(tHealth, v, Tolerant); len(ps) != 1 || ps[0].Req != "REQ-MSG-003" {
		t.Errorf("tolerant missing ReplyTo: got %v", ps)
	}
	for in, want := range map[string]Mode{"": Strict, "strict": Strict, "tolerant": Tolerant} {
		if got, err := ParseMode(in); err != nil || got != want {
			t.Errorf("ParseMode(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseMode("lenient"); err == nil {
		t.Error("ParseMode(lenient) must fail")
	}
}

func TestCheckMetaDataType(t *testing.T) {
	v, _ := Parse([]byte(metadataPub))
	md, _ := Resolve(v, "/MetaData")
	if ps := CheckMetaDataType(md, Strict); len(ps) > 0 {
		t.Errorf("unexpected problems %v", ps)
	}
	md.(map[string]any)["Foo"] = 1
	delete(md.(map[string]any), "Namespaces")
	got := map[string]bool{}
	for _, p := range CheckMetaDataType(md, Strict) {
		got[p.Pointer+" "+p.Req] = true
	}
	if !got["/Foo REQ-MSG-001"] || !got[" REQ-MSG-035"] || len(got) != 2 {
		t.Errorf("got %v", got)
	}
	if ps := CheckMetaDataType(md, Tolerant); len(ps) != 1 || ps[0].Req != "REQ-MSG-035" {
		t.Errorf("tolerant: got %v", ps)
	}
}

func TestGetMetadata(t *testing.T) {
	top := tMetadata
	top.Method = "Get"
	get := `{"MessageId": "1639693510000-ITConnector/https://id.consumer.com/MEService/Inst07322", "MessageType": "ua-metadata",
"PublisherId": "ITConnector/https://id.consumer.com/MEService/Inst07322", "DataSetWriterId": 93, "WriterGroupName": "oee",
"DataSetWriterName": "` + app + `", "Timestamp": "2021-12-16T22:25:10.000Z", "ReplyTo": "", "MetaData": {}}`
	if got := reqs(t, top, get); len(got) > 0 {
		t.Errorf("Get with empty MetaData: unexpected %v", got)
	}
	if got := reqs(t, top, strings.Replace(get, `"MetaData": {}`, `"MetaData": []`, 1)); !reflect.DeepEqual(got, []string{"REQ-RES-META-001"}) {
		t.Errorf("Get with MetaData array: got %v", got)
	}
	if got := reqs(t, top, strings.Replace(get, `"MetaData": {}`, `"MetaData": {"Name": "oee"}`, 1)); !reflect.DeepEqual(got, []string{"REQ-RES-META-001"}) {
		t.Errorf("Get with filled MetaData: got %v", got)
	}
}

var oi4PvPub = strings.Replace(strings.Replace(dataPub, `"WriterGroupName": "oee"`, `"WriterGroupName": "Oi4Pv"`, 1),
	`"Payload": {"Oee": 0.83}`, `"Payload": {"Pv": 21.5, "Sv1": [1, 2], "Sv255": {"a": 1}}`, 1)

func statusEvent(group, category string, number uint64, details string) string {
	return `{"MessageId": "5-OTConnector/` + app + `", "MessageType": "ua-data", "PublisherId": "OTConnector/` + app + `",
"DataSetClassId": "543ae05e-b6d9-4161-a0a3-350a0fac5976", "ReplyTo": "",
"Messages": [{"DataSetWriterId": 623, "WriterGroupName": "` + group + `", "DataSetWriterName": "` + app + `",
  "MessageType": "ua-event", "Payload": {"Number": ` + strconv.FormatUint(number, 10) + `, "Category": "` + category + `"` + details + `}}]}`
}

var subList = `{
  "MessageId": "3-OTConnector/` + app + `", "MessageType": "ua-data", "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "e5d68c47-c276-4929-8ab9-4c1090cac785", "ReplyTo": "",
  "Messages": [{"DataSetWriterId": 227, "WriterGroupName": "Data/oee", "DataSetWriterName": "` + app + `",
    "Payload": {"TopicPath": "Oi4v2/OTConnector/x/Pub/Data/y/oee", "Interval": 0, "Config": "CONF_1"}}]
}`

func TestContentC028(t *testing.T) {
	tOi4Pv := tData
	tOi4Pv.Filter = "Oi4Pv"
	tDataGet, tDataSet := tData, tData
	tDataGet.Method, tDataSet.Method = "Get", "Set"
	ev := func(cat, level string) Topic {
		return Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "Event", SourceIL: app, Filter: cat + "/" + level}
	}
	sub := func(method, filter string) Topic {
		return Topic{ServiceType: "OTConnector", AppIL: app, Method: method, Resource: "SubscriptionList", SourceIL: app, Filter: filter}
	}
	sub2 := strings.Replace(subList, `}}]`, `}}, {"DataSetWriterId": 227, "WriterGroupName": "Data/oee", "DataSetWriterName": "`+app+`",
    "Payload": {"TopicPath": "Oi4v2/OTConnector/x/Pub/Data/y/oee"}}]`, 1)
	cases := []struct {
		name string
		top  Topic
		msg  string
		want []string
	}{
		{"oi4pv", tOi4Pv, oi4PvPub, nil},
		{"oi4pv without pv", tOi4Pv, strings.Replace(oi4PvPub, `"Pv": 21.5, `, ``, 1), []string{"REQ-RES-DATA-004"}},
		{"oi4pv sv0", tOi4Pv, strings.Replace(oi4PvPub, `"Sv1"`, `"Sv0"`, 1), []string{"REQ-RES-DATA-004"}},
		{"oi4pv sv256", tOi4Pv, strings.Replace(oi4PvPub, `"Sv255"`, `"Sv256"`, 1), []string{"REQ-RES-DATA-004"}},
		{"application data", tData, dataPub, nil},
		{"get data with data", tDataGet, dataPub, []string{"REQ-RES-DATA-006"}},
		{"set data two", tDataSet, strings.Replace(dataPub, `}
  }]`, `}
  }, {"DataSetWriterId": 21, "WriterGroupName": "oee", "DataSetWriterName": "`+app+`", "Payload": {}}]`, 1), []string{"REQ-RES-DATA-009"}},
		{"status good", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 0, `, "Description": "ok", "Details": {"SymbolicId": "Good"}`), nil},
		{"status bad", ev("Status", "Bad"), statusEvent("Status/Bad", "CAT_STATUS_1", 2147876864, `, "Details": {"SymbolicId": "BadEncodingError"}`), nil},
		{"status uncertain", ev("Status", "Uncertain"), statusEvent("Status/Uncertain", "CAT_STATUS_1", 1083310080, ``), nil},
		{"status level", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 2147876864, ``), []string{"REQ-RES-EVENT-004"}},
		{"status symbol", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 0, `, "Details": {"SymbolicId": "BadFoo"}`), []string{"REQ-RES-EVENT-004"}},
		{"status reserved class", ev("Status", "Bad"), statusEvent("Status/Bad", "CAT_STATUS_1", 3221225472, ``), nil},
		{"status reserved class level", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 3221225472, ``), []string{"REQ-RES-EVENT-004"}},
		{"status details key", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 0, `, "Details": {"MSG": "x"}`), []string{"REQ-RES-EVENT-003"}},
		{"category mismatch", ev("Status", "Good"), statusEvent("Status/Good", "CAT_GENERIC_99", 0, ``), []string{"REQ-RES-EVENT-002"}},
		{"category unknown", ev("Status", "Good"), statusEvent("Status/Good", "STATUS", 0, ``), []string{"REQ-RES-EVENT-001"}},
		{"event unknown field", ev("Status", "Good"), statusEvent("Status/Good", "CAT_STATUS_1", 0, `, "Foo": 1`), []string{"REQ-MSG-001"}},
		{"event number", ev("Generic", "x"), statusEvent("Generic/x", "CAT_GENERIC_99", 4294967296, ``), []string{"REQ-RES-EVENT-001"}},
		{"syslog", ev("Syslog", "Warning"), statusEvent("Syslog/Warning", "CAT_SYSLOG_0", 12, `, "Details": {"MSG": "m", "HEADER": "h"}`), nil},
		{"syslog level", ev("Syslog", "Error"), statusEvent("Syslog/Error", "CAT_SYSLOG_0", 12, ``), []string{"REQ-RES-EVENT-008"}},
		{"syslog pri", ev("Syslog", "Debug"), statusEvent("Syslog/Debug", "CAT_SYSLOG_0", 192, ``), []string{"REQ-RES-EVENT-007"}},
		{"syslog description", ev("Syslog", "Warning"), statusEvent("Syslog/Warning", "CAT_SYSLOG_0", 12, `, "Description": "x"`), []string{"REQ-RES-EVENT-007"}},
		{"syslog details", ev("Syslog", "Warning"), statusEvent("Syslog/Warning", "CAT_SYSLOG_0", 12, `, "Details": {"Foo": "x"}`), []string{"REQ-RES-EVENT-007"}},
		{"generic", ev("Generic", "Hint"), statusEvent("Generic/Hint", "CAT_GENERIC_99", 7, `, "Description": "x", "Details": {"Foo": [1]}`), nil},
		{"generic details type", ev("Generic", "Hint"), statusEvent("Generic/Hint", "CAT_GENERIC_99", 7, `, "Details": []`), []string{"REQ-RES-EVENT-001"}},
		{"sublist", sub("Pub", ""), subList, nil},
		{"sublist set", sub("Set", "Data/oee"), subList, nil},
		{"sublist del", sub("Del", "Data/oee"), subList, nil},
		{"sublist del none", sub("Del", "Data/oee"), strings.Replace(subList[:strings.Index(subList, `[{`)], "", "", 0) + "[]}", []string{"REQ-RES-SUBLIST-011"}},
		{"sublist del two", sub("Del", "Data/oee"), sub2, []string{"REQ-RES-SUBLIST-011"}},
		{"sublist set two", sub("Set", "Data/oee"), sub2, []string{"REQ-RES-SUBLIST-010"}},
		{"sublist get with entry", sub("Get", ""), subList, []string{"REQ-RES-SUBLIST-007"}},
		{"sublist missing topic path", sub("Pub", ""), strings.Replace(subList, `"TopicPath": "Oi4v2/OTConnector/x/Pub/Data/y/oee", `, ``, 1), []string{"REQ-RES-SUBLIST-001"}},
		{"sublist config", sub("Pub", ""), strings.Replace(subList, `"CONF_1"`, `"MODE_1"`, 1), []string{"REQ-RES-SUBLIST-001"}},
		{"sublist interval", sub("Pub", ""), strings.Replace(subList, `"Interval": 0`, `"Interval": -1`, 1), []string{"REQ-RES-SUBLIST-001"}},
		{"sublist group", sub("Pub", ""), strings.Replace(subList, `/Pub/Data/y/oee"`, `/Pub/Data/y/other"`, 1), []string{"REQ-RES-SUBLIST-002"}},
		{"sublist wildcard filter", sub("Pub", ""), strings.Replace(subList, `/Pub/Data/y/oee"`, `/Pub/Data/+/#"`, 1), nil},
		{"sublist wildcard resource", sub("Pub", ""), strings.Replace(subList, `/Pub/Data/y/oee"`, `/Pub/Event/+/#"`, 1), []string{"REQ-RES-SUBLIST-002"}},
		{"sublist resource plus", sub("Pub", ""), strings.Replace(subList, `/Pub/Data/y/oee"`, `/Pub/+/y/oee"`, 1), []string{"REQ-RES-SUBLIST-002"}},
		{"sublist resource hash", sub("Pub", ""), strings.Replace(subList, `/x/Pub/Data/y/oee"`, `/#"`, 1), []string{"REQ-RES-SUBLIST-002"}},
		{"sublist foreign topic", sub("Pub", ""), strings.Replace(subList, `"Oi4v2/OTConnector/x/Pub/Data/y/oee"`, `"site/line1/#"`, 1), nil},
		{"set data sequence number", tDataSet, strings.Replace(dataPub, `"DataSetWriterName": "`+app+`",`, `"DataSetWriterName": "`+app+`", "SequenceNumber": 1,`, 1), []string{"REQ-BUS-010"}},
		{"sublist no filter", sub("Pub", ""), strings.Replace(subList, `/Pub/Data/y/oee"`, `/Pub/Data/y"`, 1), []string{"REQ-RES-SUBLIST-002"}},
	}
	for _, c := range cases {
		got := reqs(t, c.top, c.msg)
		if !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
			v, _ := Parse([]byte(c.msg))
			t.Errorf("%s: got %v, want %v: %v", c.name, got, c.want, Check(c.top, v))
		}
	}
}

var mamPub = `{
  "MessageId": "7-OTConnector/` + app + `",
  "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58",
  "ReplyTo": "",
  "Messages": [{
    "DataSetWriterId": 55, "WriterGroupName": "", "DataSetWriterName": "` + app + `",
    "Payload": {
      "Manufacturer": {"Locale": "en-US", "Text": "Provider"}, "ManufacturerUri": "provider.com",
      "Model": {"Locale": "en-US", "Text": "FieldDataService"}, "ProductCode": "FDS-001",
      "HardwareRevision": "", "SoftwareRevision": "1.0.0.0", "DeviceRevision": "",
      "DeviceManual": "https://provider.com/fds.pdf", "DeviceClass": "Oi4.OTConnector",
      "SerialNumber": "1200-0345", "ProductInstanceUri": "` + app + `", "RevisionCounter": 1,
      "Description": {"Locale": "en-US", "Text": "Field data"}
    }
  }, {
    "DataSetWriterId": 2, "WriterGroupName": "Pagination", "DataSetWriterName": "` + app + `",
    "Payload": {"TotalCount": 3, "PerPage": 1, "Page": 1, "HasNext": true, "PaginationId": "7-OTConnector/` + app + `"}
  }]
}`

var mamGet = `{
  "MessageId": "1-` + requestor + `",
  "MessageType": "ua-data",
  "PublisherId": "` + requestor + `",
  "DataSetClassId": "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58",
  "ReplyTo": "",
  "Messages": [{
    "DataSetWriterId": 1, "WriterGroupName": "PaginationRequest", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322",
    "Payload": {"PerPage": 25, "Page": 0}
  }, {
    "DataSetWriterId": 3, "WriterGroupName": "Locale", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322", "Payload": {"Locale": "de-DE"}
  }]
}`

var healthDetails = strings.Replace(healthPub, `"Payload": {"Health": "NORMAL_0"}`, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1", "Location": "fan", "Description": "stopped"}]}`, 1)

var profilePub = `{
  "MessageId": "1-OTConnector/` + app + `", "MessageType": "ua-data", "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "48017c6a-05c8-48d7-9d85-4b08bbb707f3", "ReplyTo": "",
  "Messages": [{"DataSetWriterId": 74, "WriterGroupName": "", "DataSetWriterName": "` + app + `",
    "Payload": {"Resources": ["MAM", "Health", "Profile", "PublicationList", "NewDataSetWriterId"]}}]
}`

var refDesSet = `{
  "MessageId": "1-` + requestor + `", "MessageType": "ua-data", "PublisherId": "` + requestor + `",
  "DataSetClassId": "27a75019-164a-496d-a38b-90e8a55c2cfa", "ReplyTo": "",
  "Messages": [{"DataSetWriterId": 94, "WriterGroupName": "", "DataSetWriterName": "` + app + `",
    "Payload": {"Function": {"Value": "=P1.L2.FDS1", "Local": "=FDS1", "Parent": {"Value": "=P1.L2", "IdentificationLink": "consumer.com/LC/4711"}},
      "Location": {"Value": "+P1.H3.R2"}}}]
}`

var refDesChildren = `{
  "MessageId": "2-` + requestor + `", "MessageType": "ua-data", "PublisherId": "` + requestor + `",
  "DataSetClassId": "27a75019-164a-496d-a38b-90e8a55c2cfa", "ReplyTo": "",
  "Messages": [{"DataSetWriterId": 9, "WriterGroupName": "Query", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322",
    "Payload": {"Parent": "consumer.com/LC/4711", "Aspect": "Function"}}]
}`

var (
	tMAM     = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "MAM"}
	tMAMGet  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Get", Resource: "MAM"}
	tProfile = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "Profile", SourceIL: app}
	tRefSet  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Set", Resource: "ReferenceDesignation", SourceIL: app}
	tRefDel  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Del", Resource: "ReferenceDesignation", SourceIL: app}
	tRefGet  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Get", Resource: "ReferenceDesignation"}
	tRefGetS = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Get", Resource: "ReferenceDesignation", SourceIL: app}
	tRefPub  = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: "ReferenceDesignation"}
	tListSet = Topic{ServiceType: "OTConnector", AppIL: app, Method: "Set", Resource: "PublicationList", SourceIL: app, Filter: "Data/oee"}
)

func TestContent(t *testing.T) {
	listSet := strings.Replace(pubList, `"MessageId": "1-OTConnector/`+app+`"`, `"MessageId": "1-`+requestor+`"`, 1)
	listSet = strings.Replace(listSet, `"PublisherId": "OTConnector/`+app+`"`, `"PublisherId": "`+requestor+`"`, 1)
	listSet = listSet[:strings.Index(listSet, `,
    {"DataSetWriterId": 11, "WriterGroupName": "MAM"`)] + "]}"
	for name, c := range map[string]struct {
		top Topic
		msg string
	}{
		"mam": {tMAM, mamPub}, "mam get": {tMAMGet, mamGet}, "health details": {tHealth, healthDetails},
		"profile": {tProfile, profilePub}, "refdes set": {tRefSet, refDesSet}, "publist set": {tListSet, listSet},
		"refdes children": {tRefGet, refDesChildren},
	} {
		if got := reqs(t, c.top, c.msg); len(got) > 0 {
			v, _ := Parse([]byte(c.msg))
			t.Errorf("%s: unexpected problems %v", name, Check(c.top, v))
		}
	}
	refDel := strings.Replace(refDesSet, `"Messages": [{`, `"Messages": [], "x": [{`, 1)
	if got := reqs(t, tRefDel, refDel); !reflect.DeepEqual(got, []string{"REQ-MSG-001"}) {
		t.Errorf("refdes del: got %v", got)
	}
	cases := []struct {
		name     string
		top      Topic
		msg      string
		old, new string
		want     []string
	}{
		{"pagination request in pub", tMAM, mamPub, `"DataSetWriterId": 2, "WriterGroupName": "Pagination"`, `"DataSetWriterId": 1, "WriterGroupName": "PaginationRequest"`, []string{"REQ-MSG-001", "REQ-PAG-008"}},
		{"pagination in get", tMAMGet, mamGet, `"DataSetWriterId": 1, "WriterGroupName": "PaginationRequest"`, `"DataSetWriterId": 2, "WriterGroupName": "Pagination"`, []string{"REQ-PAG-008"}},
		{"per page zero", tMAMGet, mamGet, `"PerPage": 25`, `"PerPage": 0`, []string{"REQ-PAG-001"}},
		{"pagination missing field", tMAM, mamPub, `"HasNext": true, `, ``, []string{"REQ-PAG-003"}},
		{"pagination id of other sender", tMAM, mamPub, `"PaginationId": "7-OTConnector/`, `"PaginationId": "7-Utility/`, []string{"REQ-PAG-003", "REQ-PAG-004"}},
		{"pagination id of page 1", tMAM, mamPub, `"PaginationId": "7-OTConnector/`, `"PaginationId": "6-OTConnector/`, []string{"REQ-PAG-004"}},
		{"pagination has next", tMAM, mamPub, `"HasNext": true`, `"HasNext": false`, []string{"REQ-PAG-004"}},
		{"pagination count", tMAM, mamPub, `"TotalCount": 3, "PerPage": 1`, `"TotalCount": 3, "PerPage": 2`, []string{"REQ-PAG-004"}},
		{"pagination page zero", tMAM, mamPub, `"Page": 1`, `"Page": 0`, []string{"REQ-PAG-004"}},
		{"locale", tMAMGet, mamGet, `"Locale": "de-DE"`, `"Locale": "german"`, []string{"REQ-LOC-001"}},
		{"locale in pub", tMAM, mamPub, `"DataSetWriterId": 2, "WriterGroupName": "Pagination"`, `"DataSetWriterId": 3, "WriterGroupName": "Locale", "DataSetWriterName": "` + app + `", "Payload": {"Locale": "de-DE"}}, {"DataSetWriterId": 2, "WriterGroupName": "Pagination"`, []string{"REQ-LOC-003"}},
		{"second locale", tMAMGet, mamGet, `"Payload": {"Locale": "de-DE"}`, `"Payload": {"Locale": "de-DE"}}, {"DataSetWriterId": 3, "WriterGroupName": "Locale", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322", "Payload": {"Locale": "fr-FR"}`, []string{"REQ-LOC-003"}},
		{"get with mam", tMAMGet, mamGet, `"Messages": [{`, `"Messages": [{"DataSetWriterId": 55, "WriterGroupName": "", "DataSetWriterName": "x.com/a", "Payload": {}}, {`, []string{"REQ-RES-MAM-020"}},
		{"mam missing field", tMAM, mamPub, `"ProductCode": "FDS-001",`, ``, []string{"REQ-RES-MAM-001"}},
		{"mam unknown field", tMAM, mamPub, `"ProductCode": "FDS-001",`, `"ProductCode": "FDS-001", "Foo": 1,`, []string{"REQ-MSG-001"}},
		{"mam localized text", tMAM, mamPub, `"Text": "Provider"}`, `"Text": "Provider", "x": 1}`, []string{"REQ-MSG-001"}},
		{"mam revision counter", tMAM, mamPub, `"RevisionCounter": 1`, `"RevisionCounter": 2147483648`, []string{"REQ-RES-MAM-001"}},
		{"manufacturer uri scheme", tMAM, mamPub, `"ManufacturerUri": "provider.com"`, `"ManufacturerUri": "https://provider.com"`, []string{"REQ-RES-MAM-004"}},
		{"manufacturer uri case", tMAM, mamPub, `"ManufacturerUri": "provider.com"`, `"ManufacturerUri": "Provider.com"`, []string{"REQ-RES-MAM-004"}},
		{"device manual", tMAM, mamPub, `"DeviceManual": "https://provider.com/fds.pdf"`, `"DeviceManual": "fds.pdf"`, []string{"REQ-RES-MAM-012"}},
		{"device manual empty", tMAM, mamPub, `"DeviceManual": "https://provider.com/fds.pdf"`, `"DeviceManual": ""`, nil},
		{"product instance uri", tMAM, mamPub, `"ProductInstanceUri": "provider.com`, `"ProductInstanceUri": "x.com`, []string{"REQ-RES-MAM-016"}},
		{"product instance uri length", tMAM, mamPub, `"ProductInstanceUri": "` + app + `"`, `"ProductInstanceUri": "` + strings.Repeat("a", 256) + `"`, []string{"REQ-RES-MAM-016", "REQ-RES-MAM-017"}},
		{"health enum", tHealth, healthPub, `"Health": "NORMAL_0"`, `"Health": "NORMAL"`, []string{"REQ-RES-HEALTH-001"}},
		{"health enum number", tHealth, healthPub, `"Health": "NORMAL_0"`, `"Health": 0`, []string{"REQ-MSG-031", "REQ-RES-HEALTH-001"}},
		{"aspect number", tRefGet, refDesChildren, `"Aspect": "Function"`, `"Aspect": 1`, []string{"REQ-RES-REFDES-012"}},
		{"health score", tHealth, healthDetails, `"HealthScore": 40`, `"HealthScore": 101`, []string{"REQ-RES-HEALTH-001"}},
		{"health detail field", tHealth, healthDetails, `"DiagnosticCode": "M1"`, `"DiagnosticCode": 1`, []string{"REQ-RES-HEALTH-001"}},
		{"health not highest priority", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1"`, `"Payload": {"Health": "OFF_SPEC_3"`, []string{"REQ-RES-HEALTH-002"}},
		{"health order", tHealth, healthDetails, `{"Health": "FAILURE_1", "Location"`, `{"Health": "CHECK_FUNCTION_2", "Location"`, []string{"REQ-RES-HEALTH-002"}},
		{"health check function over maintenance", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "MAINTENANCE_REQUIRED_4", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "CHECK_FUNCTION_2"`, []string{"REQ-RES-HEALTH-002"}},
		{"health low priority check", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "CHECK_FUNCTION_2", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "CHECK_FUNCTION_2"`, nil},
		{"health low priority normal", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "NORMAL_0", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "CHECK_FUNCTION_2"`, []string{"REQ-RES-HEALTH-002"}},
		{"health check function over off spec", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "OFF_SPEC_3", "HealthScore": 40,
      "Details": [{"Health": "OFF_SPEC_3", "DiagnosticCode": "M1"}, {"Health": "CHECK_FUNCTION_2"`, []string{"REQ-RES-HEALTH-002"}},
		{"health check function wins", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "CHECK_FUNCTION_2", "HealthScore": 40,
      "Details": [{"Health": "OFF_SPEC_3", "DiagnosticCode": "M1"}, {"Health": "CHECK_FUNCTION_2"`, nil},
		{"health off spec over maintenance", tHealth, healthDetails, `"Payload": {"Health": "FAILURE_1", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "FAILURE_1"`, `"Payload": {"Health": "OFF_SPEC_3", "HealthScore": 40,
      "Details": [{"Health": "MAINTENANCE_REQUIRED_4", "DiagnosticCode": "M1"}, {"Health": "OFF_SPEC_3"`, nil},
		{"profile unknown resource", tProfile, profilePub, `"NewDataSetWriterId"]`, `"Foo"]`, []string{"REQ-RES-PROFILE-001"}},
		{"profile duplicate", tProfile, profilePub, `"NewDataSetWriterId"]`, `"MAM"]`, []string{"REQ-RES-PROFILE-001"}},
		{"profile without health", tProfile, profilePub, `"Health", `, ``, []string{"REQ-RES-PROFILE-002"}},
		{"entry missing source", tPubList, pubList, `"Resource": "MAM", "Source": "` + app + `", `, `"Resource": "MAM", `, []string{"REQ-RES-PUBLIST-001"}},
		{"entry resource", tPubList, pubList, `"Resource": "MAM"`, `"Resource": "Foo"`, []string{"REQ-RES-PUBLIST-001"}},
		{"entry mode", tPubList, pubList, `"Mode": "SOURCE_3"`, `"Mode": "SOURCE"`, []string{"REQ-RES-PUBLIST-001"}},
		{"entry config", tPubList, pubList, `"Config": "NONE_0"`, `"Config": "NONE"`, []string{"REQ-RES-PUBLIST-001"}},
		{"entry precision", tPubList, pubList, `"Oee": 0.5`, `"Oee": -1`, []string{"REQ-RES-PUBLIST-001"}},
		{"entry reserved id", tPubList, pubList, `"DataSetWriterId": 55`, `"DataSetWriterId": 3`, []string{"REQ-MSG-020"}},
		{"entry filter of mam", tPubList, pubList, `"Resource": "MAM",`, `"Resource": "MAM", "Filter": "x",`, []string{"REQ-RES-PUBLIST-002"}},
		{"entry without filter", tPubList, pubList, `"Filter": "oee", `, ``, []string{"REQ-RES-PUBLIST-002"}},
		{"entry writer name", tPubList, pubList, `"Resource": "MAM", "Source": "provider.com`, `"Resource": "MAM", "Source": "x.com`, []string{"REQ-RES-PUBLIST-002"}},
		{"entry duplicate", tPubList, pubList, `"Resource": "MAM", "Source": "` + app + `", "DataSetWriterId": 55`, `"Resource": "Data", "Source": "` + app + `", "Filter": "oee", "DataSetWriterId": 55`, []string{"REQ-RES-PUBLIST-002", "REQ-RES-PUBLIST-003"}},
		{"event mode", tPubList, pubList, `"Mode": "FILTER_4"`, `"Mode": "APPLICATION_2"`, []string{"REQ-RES-PUBLIST-012"}},
		{"mam mode", tPubList, pubList, `"Mode": "SOURCE_3"`, `"Mode": "ON_REQUEST_1"`, []string{"REQ-RES-PUBLIST-012"}},
		{"mam default mode", tPubList, pubList, `"Mode": "SOURCE_3", `, ``, []string{"REQ-RES-PUBLIST-012"}},
		{"set two entries", tListSet, pubList, `"MessageId": "1-OTConnector/` + app + `"`, `"MessageId": "1-OTConnector/` + app + `"`, []string{"REQ-MSG-025", "REQ-PAG-008", "REQ-PAG-009", "REQ-RES-PUBLIST-022"}},
		{"refdes missing value", tRefSet, refDesSet, `"Location": {"Value": "+P1.H3.R2"}`, `"Location": {"Local": "+R2"}`, []string{"REQ-RES-REFDES-001"}},
		{"refdes parent", tRefSet, refDesSet, `"IdentificationLink": "consumer.com/LC/4711"`, `"IdentificationLink": 1`, []string{"REQ-RES-REFDES-001"}},
		{"refdes set none", tRefSet, refDesSet, `"Messages": [{`, `"Messages": [], "x": [{`, []string{"REQ-MSG-001", "REQ-RES-REFDES-006"}},
		{"refdes del with object", tRefDel, refDesSet, `"ReplyTo": ""`, `"ReplyTo": ""`, []string{"REQ-RES-REFDES-007"}},
		{"children without aspect", tRefGet, refDesChildren, `, "Aspect": "Function"`, ``, nil},
		{"children aspect", tRefGet, refDesChildren, `"Aspect": "Function"`, `"Aspect": "function"`, []string{"REQ-RES-REFDES-012"}},
		{"children missing parent", tRefGet, refDesChildren, `"Parent": "consumer.com/LC/4711", `, ``, []string{"REQ-RES-REFDES-012"}},
		{"children empty parent", tRefGet, refDesChildren, `"Parent": "consumer.com/LC/4711"`, `"Parent": ""`, []string{"REQ-RES-REFDES-012"}},
		{"children unknown field", tRefGet, refDesChildren, `"Aspect": "Function"`, `"Aspect": "Function", "Depth": 2`, []string{"REQ-MSG-001"}},
		{"children group name", tRefGet, refDesChildren, `"WriterGroupName": "Query"`, `"WriterGroupName": "ChildrenRequest"`, []string{"REQ-MSG-019"}},
		{"children writer name", tRefGet, refDesChildren, `"DataSetWriterName": "https://id.consumer.com/MEService/Inst07322"`, `"DataSetWriterName": "consumer.com/LC/4711"`, []string{"REQ-MSG-019"}},
		{"children twice", tRefGet, refDesChildren, `}}]`, `}}, {"DataSetWriterId": 9, "WriterGroupName": "Query", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322", "Payload": {"Parent": "x.com/a"}}]`, []string{"REQ-QRY-001"}},
		{"children with source", tRefGetS, refDesChildren, `"ReplyTo": ""`, `"ReplyTo": ""`, []string{"REQ-RES-REFDES-013"}},
		{"children in mam get", tMAMGet, mamGet, `"Payload": {"PerPage": 25, "Page": 0}`, `"Payload": {"PerPage": 25, "Page": 0}}, {"DataSetWriterId": 9, "WriterGroupName": "Query", "DataSetWriterName": "https://id.consumer.com/MEService/Inst07322", "Payload": {"Parent": "x.com/a"}`, []string{"REQ-QRY-001"}},
		{"query in pub", tRefPub, refDesChildren, `"ReplyTo": ""`, `"ReplyTo": ""`, []string{"REQ-MSG-006", "REQ-QRY-001"}},
	}
	for _, c := range cases {
		if !strings.Contains(c.msg, c.old) {
			t.Fatalf("%s: %q not in message", c.name, c.old)
		}
		msg := strings.Replace(c.msg, c.old, c.new, 1)
		if got := reqs(t, c.top, msg); !reflect.DeepEqual(got, c.want) {
			v, _ := Parse([]byte(msg))
			t.Errorf("%s: got %v, want %v: %v", c.name, got, c.want, Check(c.top, v))
		}
	}
	// Tolerant mode ignores undefined query parameters (REQ-QRY-002).
	msg := strings.Replace(refDesChildren, `"Aspect": "Function"`, `"Aspect": "Function", "Depth": 2`, 1)
	v, _ := Parse([]byte(msg))
	if ps := CheckMode(tRefGet, v, Tolerant); len(ps) > 0 {
		t.Errorf("tolerant Query: unexpected problems %v", ps)
	}
	// Tolerant mode ignores undefined keys of the DataSets, too.
	msg = strings.Replace(mamPub, `"ProductCode": "FDS-001",`, `"ProductCode": "FDS-001", "Foo": 1,`, 1)
	v, _ = Parse([]byte(msg))
	if ps := CheckMode(tMAM, v, Tolerant); len(ps) > 0 {
		t.Errorf("tolerant MAM: unexpected problems %v", ps)
	}
}

var configPub = `{
  "MessageId": "8-OTConnector/` + app + `",
  "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `",
  "DataSetClassId": "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2",
  "ReplyTo": "",
  "Messages": [{
    "DataSetWriterId": 42, "WriterGroupName": "Network", "DataSetWriterName": "` + app + `",
    "Payload": {
      "eth0": {
        "Name": {"Locale": "en-US", "Text": "ETH0"},
        "ip_address": {"Type": "String", "Value": "192.168.1.10", "Name": {"Locale": "en-US", "Text": "IP"},
          "Mandatory": true, "Validation": {"Length": 15, "Pattern": "^[0-9.]+$"}},
        "port": {"Type": "Number", "Value": "8080", "DefaultValue": "80", "Unit": "", "Name": {"Locale": "en-US", "Text": "Port"},
          "Validation": {"Min": 1, "Max": 65535}},
        "dhcp": {"Type": "Boolean", "Value": "false", "Name": {"Locale": "en-US", "Text": "DHCP"}, "Sensitive": false},
        "mode": {"Type": "String", "Value": "auto", "Name": {"Locale": "en-US", "Text": "Mode"}, "Validation": {"Values": ["auto", "manual"]}},
        "since": {"Type": "DateTime", "Value": "2026-10-21T10:00:00Z", "Name": {"Locale": "en-US", "Text": "Since"}},
        "key": {"Type": "ByteString", "Value": "AQID", "Name": {"Locale": "en-US", "Text": "Key"}},
        "note": {"Type": "String", "Value": "", "Name": {"Locale": "en-US", "Text": "Note"}, "Validation": {"Length": 1, "Values": ["x"]}}
      },
      "Context": {"Name": {"Locale": "en-US", "Text": "Network"}}
    }
  }]
}`

func TestContentC029(t *testing.T) {
	cfg := func(method, source, filter string) Topic {
		return Topic{ServiceType: "OTConnector", AppIL: app, Method: method, Resource: "Config", SourceIL: source, Filter: filter}
	}
	pub, get, set := cfg("Pub", app, "Network"), cfg("Get", "", ""), cfg("Set", app, "Network")
	r := func(old, new string) string { return strings.Replace(configPub, old, new, 1) }
	setMin := strings.Replace(configPub, configPub[strings.Index(configPub, `"Payload": {`):strings.Index(configPub, `
  }]`)], `"Payload": {"eth0": {"port": {"Value": "1"}, "note": {"Value": ""}}}`, 1)
	sens := func(values string) string {
		return r(`"Value": "8080", "DefaultValue": "80", "Unit": ""`, values+`, "Unit": "", "Sensitive": true`)
	}
	cases := []struct {
		name string
		top  Topic
		msg  string
		want []string
	}{
		{"pub", pub, configPub, nil},
		{"set full", set, configPub, nil},
		{"set minimal", set, setMin, nil},
		{"get with config", get, configPub, []string{"REQ-RES-CONFIG-016"}},
		{"set two", set, r(`}
  }]`, `}
  }, {"DataSetWriterId": 43, "WriterGroupName": "Network", "DataSetWriterName": "`+app+`", "Payload": {"g": {"c": {"Value": ""}}}}]`), []string{"REQ-RES-CONFIG-019"}},
		{"group name digit", pub, r(`"eth0": {`, `"0eth": {`), []string{"REQ-RES-CONFIG-004"}},
		{"config name minus", pub, r(`"port": {`, `"-port": {`), []string{"REQ-RES-CONFIG-004"}},
		{"config name dot", pub, r(`"port": {`, `"p.ort": {`), []string{"REQ-RES-CONFIG-004"}},
		{"no group", pub, strings.Replace(configPub, configPub[strings.Index(configPub, `"Payload": {`):strings.Index(configPub, `
  }]`)], `"Payload": {"Context": {"Name": {"Locale": "en-US", "Text": "x"}}}`, 1), []string{"REQ-RES-CONFIG-002"}},
		{"set empty group", set, strings.Replace(setMin, `"eth0": {"port": {"Value": "1"}, "note": {"Value": ""}}`, `"eth0": {"Name": {"Locale": "en-US", "Text": "x"}}`, 1), []string{"REQ-RES-CONFIG-003"}},
		{"group not object", pub, r(`"Context": {"Name": {"Locale": "en-US", "Text": "Network"}}`, `"Context": {"Name": {"Locale": "en-US", "Text": "Network"}}, "g": 1`), []string{"REQ-RES-CONFIG-002"}},
		{"group without name", pub, r(`"Name": {"Locale": "en-US", "Text": "ETH0"},`, ``), []string{"REQ-RES-CONFIG-002"}},
		{"set group without name", set, r(`"Name": {"Locale": "en-US", "Text": "ETH0"},`, ``), nil},
		{"pub config without type", pub, r(`"dhcp": {"Type": "Boolean", `, `"dhcp": {`), []string{"REQ-RES-CONFIG-002"}},
		{"set config without value", set, strings.Replace(setMin, `{"Value": "1"}`, `{"Type": "Number"}`, 1), []string{"REQ-RES-CONFIG-003"}},
		{"type integer", pub, r(`"Type": "Number"`, `"Type": "Integer"`), []string{"REQ-RES-CONFIG-006"}},
		{"value number", pub, r(`"Value": "8080"`, `"Value": 8080`), []string{"REQ-RES-CONFIG-007"}},
		{"boolean yes", pub, r(`"Value": "false"`, `"Value": "yes"`), []string{"REQ-RES-CONFIG-007"}},
		{"number text", pub, r(`"DefaultValue": "80"`, `"DefaultValue": "eighty"`), []string{"REQ-RES-CONFIG-007"}},
		{"date time", pub, r(`"2026-10-21T10:00:00Z"`, `"21.10.2026"`), []string{"REQ-RES-CONFIG-007"}},
		{"byte string", pub, r(`"AQID"`, `"A*"`), []string{"REQ-RES-CONFIG-007"}},
		{"mandatory string", pub, r(`"Mandatory": true`, `"Mandatory": "true"`), []string{"REQ-RES-CONFIG-002"}},
		{"name text", pub, r(`"Text": "Port"}`, `"Text": "Port", "Foo": 1}`), []string{"REQ-MSG-001"}},
		{"name plain", pub, r(`{"Locale": "en-US", "Text": "Port"}`, `"Port"`), []string{"REQ-MSG-040"}},
		{"context without name", pub, r(`"Context": {"Name": {"Locale": "en-US", "Text": "Network"}}`, `"Context": {}`), []string{"REQ-RES-CONFIG-002"}},
		{"unknown config key", pub, r(`"Sensitive": false`, `"Sensitive": false, "Access": "rw"`), []string{"REQ-MSG-001"}},
		{"above max", pub, r(`"Value": "8080"`, `"Value": "70000"`), []string{"REQ-RES-CONFIG-008"}},
		{"below min", pub, r(`"Value": "8080"`, `"Value": "0"`), []string{"REQ-RES-CONFIG-008"}},
		{"too long", pub, r(`"192.168.1.10"`, `"192.168.100.100x"`), []string{"REQ-RES-CONFIG-008"}},
		{"not in values", pub, r(`"Value": "auto"`, `"Value": "off"`), []string{"REQ-RES-CONFIG-008"}},
		{"values not strings", pub, r(`["auto", "manual"]`, `["auto", 1]`), []string{"REQ-RES-CONFIG-002"}},
		{"length negative", pub, r(`"Length": 15`, `"Length": -1`), []string{"REQ-RES-CONFIG-002"}},
		{"min text", pub, r(`"Min": 1`, `"Min": "1"`), []string{"REQ-RES-CONFIG-002"}},
		{"pattern re2 unsupported", pub, r(`"^[0-9.]+$"`, `"^(?=x)"`), nil},
		// GL-ADR-021: masking of sensitive values.
		{"sensitive masked", pub, sens(`"Value": "********", "DefaultValue": "********"`), nil},
		{"sensitive empty", pub, sens(`"Value": "", "DefaultValue": ""`), nil},
		{"sensitive value clear", pub, sens(`"Value": "8080", "DefaultValue": "********"`), []string{"REQ-RES-CONFIG-024"}},
		{"sensitive default clear", pub, sens(`"Value": "********", "DefaultValue": "80"`), []string{"REQ-RES-CONFIG-024"}},
		{"mask not sensitive", pub, r(`"Value": "8080"`, `"Value": "********"`), []string{"REQ-RES-CONFIG-007"}},
		{"set mask sensitive", set, sens(`"Value": "********", "DefaultValue": "********"`), nil},
		{"set clear sensitive", set, sens(`"Value": "8080", "DefaultValue": "80"`), nil},
	}
	for _, c := range cases {
		got := reqs(t, c.top, c.msg)
		if !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
			v, _ := Parse([]byte(c.msg))
			t.Errorf("%s: got %v, want %v: %v", c.name, got, c.want, Check(c.top, v))
		}
	}
}
