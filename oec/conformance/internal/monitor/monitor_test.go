package monitor

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
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

// TestLateBirth: after a Will the component reconnects, publishes a Health
// and then its Birth. REQ-LIFE-001 is fulfilled (the order is REQ-LIFE-015,
// T7-LIFE-02); the late Birth is no second Birth (C-066).
func TestLateBirth(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	s.will(5, compA, 0)
	s.health(6, compA, "NORMAL_0", 100)
	s.birth(7, compA)
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if v := find(res, "T7-MON-03", "REQ-LIFE-001", verdict.Violation); len(v) != 0 {
		t.Errorf("unexpected violation: %+v", v)
	}
	if v := find(res, "T7-MON-10", "REQ-IL-012", verdict.Note); len(v) != 0 {
		t.Errorf("unexpected second Birth note: %+v", v)
	}
	if res.Participants[0].Births != 2 {
		t.Errorf("births %d", res.Participants[0].Births)
	}
}

// TestRecoveryBirth: a Health FAILURE_1/0 while connected (not a Will) and
// the Birth again on recovery (REQ-LIFE-017) is no second Birth.
func TestRecoveryBirth(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	s.health(3, compA, "FAILURE_1", 0)
	s.birth(5, compA)
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if v := find(res, "T7-MON-10", "REQ-IL-012", verdict.Note); len(v) != 0 {
		t.Errorf("unexpected second Birth note: %+v", v)
	}
}

// TestMAMOnChange: a MAM about itself published on change with an
// incremented RevisionCounter (REQ-RES-MAM-018, published without request,
// REQ-RES-PUBLIST) is no second Birth; later MAMs are compared with it.
func TestMAMOnChange(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	rev, err := strconv.ParseInt(revision(compA.mam), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	changed := map[string]any{}
	for k, v := range compA.mam {
		changed[k] = v
	}
	changed["RevisionCounter"] = json.Number(strconv.FormatInt(rev+1, 10))
	topic := compA.base() + "/Pub/MAM/" + compA.self()
	s.add(5, topic, network(compA, at(5), classMAM, "", dsMsg(compA.mamID, compA.il, "", changed)))
	// an answer with the new RevisionCounter that differs from the changed MAM
	other := map[string]any{}
	for k, v := range changed {
		other[k] = v
	}
	other["HardwareRevision"] = "other"
	s.add(7, topic, network(compA, at(7), classMAM, "1639693510094-Aggregation/x/y/z/1", dsMsg(compA.mamID, compA.il, "", other)))
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if n := len(find(res, "T7-MON-10", "REQ-IL-012", verdict.Note)); n != 0 {
		t.Errorf("a MAM on change is no second Birth: %+v", res.Observations)
	}
	if res.Participants[0].Births != 1 {
		t.Errorf("births %d", res.Participants[0].Births)
	}
	if n := len(find(res, "T7-MON-03", "REQ-LIFE-003", verdict.Violation)); n != 1 {
		t.Errorf("want 1 violation against the changed MAM, got %d: %+v", n, res.Observations)
	}
}

// TestLocalizedMAM: the answer to a Get with a Locale carries the
// LocalizedText in the requested language (REQ-LOC-002); it is not the
// regular Pub MAM that REQ-LIFE-003 compares with the Birth. The same
// answer to a Get without Locale is.
func TestLocalizedMAM(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	id := s.req(2, "V-PAY-007", nil)
	s.fromVector(3, "V-PAY-047", id, nil)
	plain := s.req(4, "V-PAY-007", func(m map[string]any) { m["Messages"] = []any{} })
	s.fromVector(5, "V-PAY-047", plain, nil)
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	if n := len(find(res, "T7-MON-03", "REQ-LIFE-003", verdict.Violation)); n != 1 {
		t.Errorf("want 1 violation (the German MAM without Locale), got %d: %+v", n, find(res, "T7-MON-03", "REQ-LIFE-003", verdict.Violation))
	}
}

// TestRequestedPage: the answer to a Get with a PaginationRequest for one
// page (Page > 0) is that page alone (REQ-PAG-002); HasNext true without a
// further page is no incomplete series. The same answer to a Get without
// PaginationRequest is (REQ-PAG-005).
func TestRequestedPage(t *testing.T) {
	s := scenario(t, "x", "")
	s.birth(0, compA)
	answer := func(sec float64, replyTo string) {
		id := msgID(compA, at(sec))
		pag := dsMsg(2, compA.il, "Pagination", map[string]any{"TotalCount": 2, "PerPage": 1, "Page": 1, "HasNext": true, "PaginationId": id})
		s.add(sec, compA.base()+"/Pub/MAM", network(compA, at(sec), classMAM, replyTo, dsMsg(compA.mamID, compA.il, "", compA.mam), pag))
	}
	page := s.req(2, "V-PAY-007", func(m map[string]any) {
		m["Messages"] = []any{dsMsg(1, compB.il, "PaginationRequest", map[string]any{"PerPage": 1, "Page": 1})}
	})
	answer(3, page)
	plain := s.req(4, "V-PAY-007", func(m map[string]any) { m["Messages"] = []any{} })
	answer(5, plain)
	s.end = 30 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-18", "REQ-PAG-005", verdict.Violation); len(o) != 1 || o[0].Line != 6 {
		t.Errorf("want 1 violation (line 6, the answer to the Get without PaginationRequest): %+v", o)
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

// A Pub Metadata carries its content in MetaData, not in Messages: an
// unsolicited one is content (REQ-CMP-042, found against the SDK in C-054).
func TestUnsolicitedMetadataHasContent(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	s.fromVector(2, "V-PAY-021", "", nil)
	s.end = 5 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-11", "REQ-CMP-042", verdict.Violation); len(o) != 0 {
		t.Errorf("an unsolicited Pub Metadata has content: %+v", o)
	}
	for _, o := range res.Observations {
		if o.Test == "T7-MON-01" && o.Kind == verdict.Violation {
			t.Errorf("the scenario is valid: %+v", o)
		}
	}
}

// TestRevisionCounter: the RevisionCounter of an asset neither decreases
// nor changes between -1 and a counter (REQ-RES-MAM-018).
func TestRevisionCounter(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	s.mamRevision(2, compA, "3", "")
	s.mamRevision(3, compA, "2", "")
	s.mamRevision(4, compA, "-1", "")
	s.end = 10 * time.Second
	res := run(t, s, DefaultOptions())
	o := find(res, "T7-MON-16", "REQ-RES-MAM-018", verdict.Violation)
	if len(o) != 2 || !strings.Contains(o[0].Evidence.Detail, "decreases") || !strings.Contains(o[1].Evidence.Detail, "-1") {
		t.Errorf("want a decrease and a change to -1: %+v", o)
	}
}

// TestDelSubscriptionList: a successful Del SubscriptionList need not be
// followed by a publication (REQ-RES-SUBLIST-015); the rule skips without a
// gap of the run.
func TestDelSubscriptionList(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	del := s.req(2, "V-PAY-074", nil)
	s.fromVector(2.1, "V-PAY-073", del, nil)
	s.end = 20 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-15", "REQ-RES-SUBLIST-005", verdict.Violation); len(o) != 0 {
		t.Errorf("no violation after a Del: %+v", o)
	}
	o := find(res, "T7-MON-15", "REQ-RES-SUBLIST-005", verdict.Skipped)
	if len(o) != 1 || o[0].Inconclusive {
		t.Errorf("want one conclusive skip: %+v", o)
	}
}

// TestSetConfigUnchanged: a Set Config that repeats the published values
// and masks the sensitive one changes nothing; no RevisionCounter is
// expected (REQ-RES-MAM-018) and the mandatory objects are present
// (REQ-RES-CONFIG-011).
func TestSetConfigUnchanged(t *testing.T) {
	s := scenario(t, "x", "")
	s.start()
	get := s.req(2, "V-PAY-098", nil)
	s.fromVector(2.1, "V-PAY-099", get, nil)
	pub := first(vec(t, "V-PAY-099"))["Payload"].(map[string]any)
	set := s.req(3, "V-PAY-100", func(m map[string]any) {
		payload := map[string]any{}
		for g, gv := range pub {
			group, ok := gv.(map[string]any)
			if !ok {
				continue
			}
			objs := map[string]any{}
			for o, ov := range group {
				if obj, ok := ov.(map[string]any); ok {
					if v, ok := obj["Value"]; ok {
						objs[o] = map[string]any{"Value": v}
					}
				}
			}
			if len(objs) > 0 {
				payload[g] = objs
			}
		}
		first(m)["Payload"] = payload
	})
	s.fromVector(3.1, "V-PAY-076", set, nil)
	s.end = 20 * time.Second
	res := run(t, s, DefaultOptions())
	if o := find(res, "T7-MON-16", "REQ-RES-MAM-018", verdict.Skipped); len(o) != 0 {
		t.Errorf("an unchanged Set Config expects no MAM: %+v", o)
	}
	if o := find(res, "T7-MON-17", "REQ-RES-CONFIG-011", verdict.Violation); len(o) != 0 {
		t.Errorf("mandatory objects present: %+v", o)
	}
	if o := find(res, "T7-MON-17", "REQ-RES-CONFIG-011", verdict.Checked); len(o) == 0 {
		t.Error("want a check")
	}
}
