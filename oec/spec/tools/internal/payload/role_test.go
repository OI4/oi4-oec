package payload

import (
	"strings"
	"testing"
)

func roleMsg(t *testing.T, payload string, n int) any {
	t.Helper()
	dsm := `{"DataSetWriterId": 3456, "WriterGroupName": "", "DataSetWriterName": "` + app + `", "Payload": ` + payload + `}`
	msg, err := Parse([]byte(`{"MessageId": "1639693570000-OTConnector/` + app + `", "MessageType": "ua-data",
  "PublisherId": "OTConnector/` + app + `", "ReplyTo": "",
  "Messages": [` + strings.Repeat(dsm+",", n-1) + dsm + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestCheckRole(t *testing.T) {
	self := func(res string) Topic {
		return Topic{ServiceType: "OTConnector", AppIL: app, Method: "Pub", Resource: res, SourceIL: app}
	}
	mam := `{"ProductInstanceUri": "` + app + `"}`
	for _, tc := range []struct {
		name, role, payload string
		n                   int
		t                   Topic
		want                string // "" = no problem, else the REQ and a substring
	}{
		{"birth", "birth", mam, 1, self("MAM"), ""},
		{"birth of another asset", "birth", `{"ProductInstanceUri": "vendor.com/Other/X/1"}`, 1, self("MAM"), "REQ-LIFE-003 not of the component itself"},
		{"birth on Health", "birth", mam, 1, self("Health"), "REQ-LIFE-003 Pub MAM"},
		{"birth with two messages", "birth", mam, 2, self("MAM"), "REQ-LIFE-003 exactly one"},
		{"close", "close", `{"Health": "NORMAL_0", "HealthScore": 0}`, 1, self("Health"), ""},
		{"close without score", "close", `{"Health": "NORMAL_0"}`, 1, self("Health"), "REQ-LIFE-007 missing"},
		{"close with score", "close", `{"Health": "NORMAL_0", "HealthScore": 100}`, 1, self("Health"), "REQ-LIFE-007 HealthScore is 100"},
		{"close failure", "close", `{"Health": "FAILURE_1", "HealthScore": 0}`, 1, self("Health"), "REQ-LIFE-007 requires NORMAL_0"},
		{"close on Get", "close", `{"Health": "NORMAL_0", "HealthScore": 0}`, 1, Topic{AppIL: app, Method: "Get", Resource: "Health"}, "REQ-LIFE-007 not a Get Health"},
		{"will", "will", `{"Health": "FAILURE_1", "HealthScore": 0}`, 1, self("Health"), ""},
		{"will normal", "will", `{"Health": "NORMAL_0", "HealthScore": 0}`, 1, self("Health"), "REQ-LIFE-009 requires FAILURE_1"},
		{"will with score", "will", `{"Health": "FAILURE_1", "HealthScore": 20}`, 1, self("Health"), "REQ-LIFE-009 HealthScore is 20"},
		{"will on MAM", "will", mam, 1, self("MAM"), "REQ-LIFE-009 Pub Health"},
		{"malformed payload left to CheckMode", "close", `"x"`, 1, self("Health"), ""},
	} {
		got, err := CheckRole(tc.t, roleMsg(t, tc.payload, tc.n), tc.role)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: unexpected %v", tc.name, got)
			}
			continue
		}
		req, sub, _ := strings.Cut(tc.want, " ")
		if len(got) == 0 || got[0].Req != req || !strings.Contains(got[0].Msg, sub) {
			t.Errorf("%s: got %v, want %s", tc.name, got, tc.want)
		}
	}
	if _, err := CheckRole(self("MAM"), nil, "rebirth"); err == nil {
		t.Error("unknown role accepted")
	}
}
