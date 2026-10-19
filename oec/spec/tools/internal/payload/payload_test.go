package payload

import (
	"reflect"
	"sort"
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
    {"DataSetWriterId": 11, "WriterGroupName": "Data/oee", "DataSetWriterName": "` + app + `", "Payload": {}},
    {"DataSetWriterId": 12, "WriterGroupName": "MAM", "DataSetWriterName": "` + app + `", "Payload": {}}
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
		{"metadata and minor version", tData, dataPub, `"MessageType": "ua-keyframe"`, `"MinorVersion": 2`, []string{"REQ-MSG-018"}},
		{"helper writer group name", tHealth, healthPub, `"DataSetWriterId": 3456`, `"DataSetWriterId": 2`, []string{"REQ-MSG-019"}},
		{"reserved writer id", tHealth, healthPub, `"DataSetWriterId": 3456`, `"DataSetWriterId": 8`, []string{"REQ-MSG-020"}},
		{"writer id zero", tHealth, healthPub, `"DataSetWriterId": 3456`, `"DataSetWriterId": 0`, []string{"REQ-MSG-020"}},
		{"writer name differs from source", tHealth, healthPub, `"DataSetWriterName": "provider.com`, `"DataSetWriterName": "x.com`, []string{"REQ-MSG-023"}},
		{"health writer group name", tHealth, healthPub, `"WriterGroupName": ""`, `"WriterGroupName": "x"`, []string{"REQ-MSG-024"}},
		{"data writer group name", tData, dataPub, `"WriterGroupName": "oee"`, `"WriterGroupName": "x"`, []string{"REQ-MSG-024"}},
		{"list writer group name", tPubList, pubList, `"WriterGroupName": "Data/oee"`, `"WriterGroupName": "oee"`, []string{"REQ-MSG-025"}},
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
	if got := reqs(t, top, strings.Replace(get, `"MetaData": {}`, `"MetaData": []`, 1)); !reflect.DeepEqual(got, []string{"REQ-MSG-032"}) {
		t.Errorf("Get with MetaData array: got %v", got)
	}
}
