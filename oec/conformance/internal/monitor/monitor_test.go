package monitor

import (
	"reflect"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

func run(t *testing.T, s *sc, opt Options) Result {
	t.Helper()
	v, err := validate.New()
	if err != nil {
		t.Fatal(err)
	}
	c := &capture.Capture{Header: s.header, Records: s.records, Trailer: &capture.Trailer{Ended: t0.Add(s.end)}}
	for i := range c.Records {
		c.Records[i].Line = i + 2
		c.Records[i].Size = len(c.Records[i].Payload)
	}
	res, err := Replay(v, c, opt)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func find(res Result, test, req string, kind verdict.Kind) []verdict.Observation {
	var out []verdict.Observation
	for _, o := range res.Observations {
		if o.Test == test && o.Req == req && o.Kind == kind {
			out = append(out, o)
		}
	}
	return out
}

func TestTS(t *testing.T) {
	for in, want := range map[string]int64{"1639693510094-OTConnector/a": 1639693510094, "x-1": -1, "": -1, "99999999999999999999-a": -1} {
		if got := ts(in); got != want {
			t.Errorf("ts(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestWillWithoutBirth(t *testing.T) {
	s := scenario(t, "x", "")
	s.will(5, compA, 1)
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if len(find(res, "T7-MON-03", "REQ-LIFE-001", verdict.Note)) != 1 {
		t.Errorf("want a note: %+v", res.Observations)
	}
	if len(find(res, "T7-MON-03", "REQ-LIFE-001", verdict.Skipped)) != 1 {
		t.Error("want an inconclusive skip without a connect")
	}
}

func TestSecondBirth(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	s.birth(5, compA)
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if len(find(res, "T7-MON-10", "REQ-IL-012", verdict.Note)) != 1 {
		t.Errorf("want a note: %+v", res.Observations)
	}
	if res.Participants[0].Births != 2 {
		t.Errorf("births %d", res.Participants[0].Births)
	}
}

func TestDebugAfterSet(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	s.req(2, "V-PAY-094", nil)
	s.fromVector(5, "V-PAY-064", "", func(m map[string]any) {
		d := first(m)
		d["WriterGroupName"] = "Syslog/Debug"
	})
	fixTopic(s, "Syslog/Notice", "Syslog/Debug")
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if len(find(res, "T7-MON-19", "REQ-TOP-024", verdict.Note)) != 1 || len(find(res, "T7-MON-19", "REQ-TOP-024", verdict.Violation)) != 0 {
		t.Errorf("want a note: %+v", find(res, "T7-MON-19", "REQ-TOP-024", verdict.Violation))
	}
}

func TestForeignAndInvalidTopics(t *testing.T) {
	s := scenario(t, "x", "")
	s.raw(1, "$SYS/broker/uptime", []byte("12"), 0, false)
	s.raw(2, "other/topic", []byte("{}"), 0, false)
	b, _ := marshal(healthMsg(compA, at(3), compA.il, compA.health, "NORMAL_0", 100, ""))
	s.raw(3, "Oi4v2/OTConnector/Pub/Health", b, 0, false)
	s.end = 5 * time.Second
	res := run(t, s, DefaultOptions())
	if res.Messages != 3 || res.Namespace != 1 {
		t.Errorf("messages %d, namespace %d", res.Messages, res.Namespace)
	}
	if len(find(res, "T7-MON-01", "REQ-TOP-003", verdict.Violation)) != 1 {
		t.Errorf("want REQ-TOP-003: %+v", res.Observations)
	}
}

func TestUnknownPacketSizeAndQoS0(t *testing.T) {
	s := scenario(t, "x", "")
	s.header.MaxPacketSize, s.header.SubscribeQoS = 0, 0
	s.start()
	s.end = 5 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-18", "REQ-PAG-005", verdict.Skipped); len(o) == 0 || !o[0].Inconclusive {
		t.Errorf("want an inconclusive skip: %+v", o)
	}
	if o := find(res, "T7-MON-12", "REQ-MQTT-017", verdict.Skipped); len(o) == 0 || o[0].Inconclusive {
		t.Errorf("want a skip: %+v", o)
	}
	opt := DefaultOptions()
	opt.MaxPacketSize = 100
	if len(find(run(t, s, opt), "T7-MON-18", "REQ-PAG-005", verdict.Violation)) == 0 {
		t.Error("the option overrides the header")
	}
}

func TestRequestIDAfterTimeout(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	s.call(2, 7, 1000)
	s.call(4, 7, 1000)
	s.end = 5 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-07", "REQ-ACT-011", verdict.Violation); len(o) != 0 {
		t.Errorf("a timed-out request is not pending: %+v", o)
	}
}

func TestDeterministic(t *testing.T) {
	fx := fixtures(t)
	a := run(t, fx["lifecycle-pass.jsonl"], DefaultOptions())
	b := run(t, fx["lifecycle-pass.jsonl"], DefaultOptions())
	if !reflect.DeepEqual(a, b) {
		t.Error("two replays differ")
	}
	if len(a.Participants) != 2 || a.Participants[0].Closes != 1 || len(a.Participants[1].Sources) != 1 {
		t.Errorf("participants %+v", a.Participants)
	}
}
