package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const xHealthTopic = "Oi4v2/OTConnector/cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU/Pub/Health/cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU"

const xHealthPub = `{"DataSetClassId":"d8e7b6df-42ba-448a-975a-199f59e8ffeb",
"MessageId":"1639693510094-OTConnector/provider.com/FieldDataService/FDS-001/1200-0345",
"MessageType":"ua-data","PublisherId":"OTConnector/provider.com/FieldDataService/FDS-001/1200-0345","ReplyTo":"",
"Messages":[{"DataSetWriterId":3456,"WriterGroupName":"","DataSetWriterName":"provider.com/FieldDataService/FDS-001/1200-0345",
"SequenceNumber":57,"Timestamp":"2021-12-16T22:25:10.094Z","Payload":{"Health":"NORMAL_0","HealthScore":100}}]}`

func writeCases(t *testing.T, cases []xcase) string {
	t.Helper()
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "cases.json")
	if err := os.WriteFile(f, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCrosscheck(t *testing.T) {
	const entry = "https://schemas.openindustry4.com/oec/v2/topics/pub-health.schema.json"
	const generic = "https://schemas.openindustry4.com/oec/v2/message.schema.json"
	badReply := strings.Replace(xHealthPub, `"ReplyTo":""`, `"ReplyTo":"1639693510094-abc"`, 1)
	tests := []struct {
		name  string
		cases []xcase
		fail  string
	}{
		{"agree", []xcase{
			{ID: "ok", Schema: entry, Instance: json.RawMessage(xHealthPub), Valid: true, Topic: xHealthTopic},
			{ID: "bad", Schema: entry, Instance: json.RawMessage(badReply), Topic: xHealthTopic, Reqs: []string{"REQ-MSG-015"}},
			{ID: "generic", Schema: generic, Instance: json.RawMessage(badReply), Topic: xHealthTopic, Reqs: []string{"REQ-MSG-001"}},
		}, ""},
		{"schema REQ missing in the oracle", []xcase{
			{ID: "x", Schema: entry, Instance: json.RawMessage(badReply), Topic: xHealthTopic, Reqs: []string{"REQ-MSG-015", "REQ-MSG-007"}},
		}, "1 disagreement"},
		{"generic schema rejects, oracle accepts", []xcase{
			{ID: "x", Schema: generic, Instance: json.RawMessage(xHealthPub), Topic: xHealthTopic, Reqs: []string{"REQ-MSG-001"}},
		}, "1 disagreement"},
		{"schema accepts, oracle reports a schema REQ", []xcase{
			{ID: "x", Schema: entry, Instance: json.RawMessage(badReply), Valid: true, Topic: xHealthTopic},
		}, "1 disagreement"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdCrosscheck([]string{"-spec", "../../..", "-cases", writeCases(t, tc.cases)})
			switch {
			case tc.fail == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.fail != "" && (err == nil || !strings.Contains(err.Error(), tc.fail)):
				t.Fatalf("got %v, want an error containing %q", err, tc.fail)
			}
		})
	}
}
