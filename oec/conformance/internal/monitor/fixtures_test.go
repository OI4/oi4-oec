package monitor

// The capture fixtures in oec/conformance/testdata/captures are generated
// from the scenarios below (go test ./internal/monitor -run TestFixtureFiles
// -update); their payloads are derived from reviewed payload vectors. The
// expected verdicts live in testdata/captures/index.yaml (test data with
// verdict, reviewed by the PO, G-3.9).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/testvec"
	"github.com/OI4/oec/conformance/internal/validate"
)

var update = flag.Bool("update", false, "rewrite the capture fixtures in testdata/captures")

const fixtureDir = "../../testdata/captures"

var t0 = time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)

// comp is a component of a scenario.
type comp struct {
	st, il string
	mam    map[string]any // payload of its MAM about itself
	mamID  int64
	health int64
}

func (c comp) pub() string  { return c.st + "/" + c.il }
func enc(il string) string  { return base64.RawURLEncoding.EncodeToString([]byte(il)) }
func (c comp) base() string { return "Oi4v2/" + c.st + "/" + enc(c.il) }
func (c comp) self() string { return enc(c.il) }
func msgID(c comp, at time.Time) string {
	return fmt.Sprintf("%d-%s", at.UnixMilli(), c.pub())
}

var (
	compA = comp{st: "OTConnector", il: "provider.com/FieldDataService/FDS-001/1200-0345", mamID: 55, health: 3456}
	compB = comp{st: "ITConnector", il: "consumer.com/MEService/MES-7/7322", mamID: 21, health: 22,
		mam: map[string]any{
			"Manufacturer": map[string]any{"Locale": "en-US", "Text": "Consumer Inc."}, "ManufacturerUri": "consumer.com",
			"Model": map[string]any{"Locale": "en-US", "Text": "MEService"}, "ProductCode": "MES-7",
			"HardwareRevision": "", "SoftwareRevision": "2.3.0", "DeviceRevision": "", "DeviceManual": "",
			"DeviceClass": "Oi4.ITConnector", "SerialNumber": "7322", "ProductInstanceUri": "consumer.com/MEService/MES-7/7322",
			"RevisionCounter": json.Number("1"), "Description": map[string]any{"Locale": "en-US", "Text": "Consumer of field data"}}}
	groe     = "groe.mca/DeviceModelB/AC33.447/20443"
	sensorIL = "https://id.groe.mca/sensors/S1"
)

var vectors = map[string]testvec.Vector{}

func vec(t *testing.T, id string) map[string]any {
	t.Helper()
	if len(vectors) == 0 {
		vs, err := testvec.Load("payload")
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range vs {
			vectors[v.ID] = v
		}
	}
	v, ok := vectors[id]
	if !ok {
		t.Fatalf("vector %s missing", id)
	}
	return decode(t, v.Payload)
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

// sc builds one capture.
type sc struct {
	t       *testing.T
	name    string
	header  capture.Header
	records []capture.Record
	end     time.Duration
}

func scenario(t *testing.T, name, note string) *sc {
	if compA.mam == nil {
		b := vec(t, "V-PAY-023")
		compA.mam = b["Messages"].([]any)[0].(map[string]any)["Payload"].(map[string]any)
	}
	return &sc{t: t, name: name, header: capture.Header{Tool: "oec-check", ToolVersion: "fixture", Subscription: "Oi4v2/#",
		SubscribeQoS: 1, MaxPacketSize: 262144, Started: t0, Note: note}}
}

func at(s float64) time.Time { return t0.Add(time.Duration(s * float64(time.Second))) }

// raw appends a record with a payload as is.
func (s *sc) raw(sec float64, topic string, payload []byte, qos byte, retain bool) {
	s.records = append(s.records, capture.Record{Topic: topic, Payload: payload, Received: at(sec), QoS: qos, Retain: retain})
}

func (s *sc) add(sec float64, topic string, m map[string]any) {
	b, err := json.Marshal(m)
	if err != nil {
		s.t.Fatal(err)
	}
	s.raw(sec, topic, b, 0, false)
}

func network(c comp, sent time.Time, class, replyTo string, msgs ...map[string]any) map[string]any {
	if msgs == nil {
		msgs = []map[string]any{}
	}
	arr := make([]any, len(msgs))
	for i, m := range msgs {
		arr[i] = m
	}
	return map[string]any{"MessageId": msgID(c, sent), "MessageType": "ua-data", "PublisherId": c.pub(),
		"DataSetClassId": class, "ReplyTo": replyTo, "Messages": arr}
}

func dsMsg(id int64, name, group string, payload any) map[string]any {
	return map[string]any{"DataSetWriterId": id, "WriterGroupName": group, "DataSetWriterName": name, "Payload": payload}
}

const (
	classMAM    = "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58"
	classHealth = "d8e7b6df-42ba-448a-975a-199f59e8ffeb"
)

// birth publishes the Birth of c (sent = received).
func (s *sc) birth(sec float64, c comp) {
	s.add(sec, c.base()+"/Pub/MAM/"+c.self(), network(c, at(sec), classMAM, "", dsMsg(c.mamID, c.il, "", c.mam)))
}

func healthMsg(c comp, sent time.Time, source string, id int64, state string, score int, replyTo string) map[string]any {
	return network(c, sent, classHealth, replyTo, dsMsg(id, source, "", map[string]any{"Health": state, "HealthScore": score}))
}

// health publishes the Health of c about itself.
func (s *sc) health(sec float64, c comp, state string, score int) {
	s.add(sec, c.base()+"/Pub/Health/"+c.self(), healthMsg(c, at(sec), c.il, c.health, state, score, ""))
}

// will is the Will message of c created at the connect at sent and
// published by the broker at sec.
func (s *sc) will(sec float64, c comp, sent float64) {
	s.add(sec, c.base()+"/Pub/Health/"+c.self(), healthMsg(c, at(sent), c.il, c.health, "FAILURE_1", 0, ""))
}

func (s *sc) close(sec float64, c comp) { s.health(sec, c, "NORMAL_0", 0) }

// heartbeats publishes the Health of c about itself every 30 s in (from, to].
func (s *sc) heartbeats(c comp, from, to, offset float64) {
	for x := from + 30; x <= to; x += 30 {
		s.health(x+offset, c, "NORMAL_0", 100)
	}
}

// req sends a request vector (a Get of the consumer of the vectors) as
// compB: the consumer's Identification Link becomes compB's.
func (s *sc) req(sec float64, id string, edit func(m map[string]any)) string {
	vec(s.t, id)
	v := vectors[id]
	m := decode(s.t, strings.NewReplacer("ITConnector/https://id.consumer.com/MEService/Inst07322", compB.pub(),
		"https://id.consumer.com/MEService/Inst07322", compB.il).Replace(v.Payload))
	m["MessageId"] = msgID(compB, at(sec))
	if _, ok := m["Timestamp"]; ok {
		m["Timestamp"] = at(sec).Format("2006-01-02T15:04:05.000Z")
	}
	if edit != nil {
		edit(m)
	}
	s.add(sec, v.Topic, m)
	return m["MessageId"].(string)
}

// fromVector adapts a payload vector of compA: MessageId, ReplyTo and the
// Timestamps go to the publish time.
func (s *sc) fromVector(sec float64, id, replyTo string, edit func(m map[string]any)) {
	v := vectors[id]
	if v.ID == "" {
		vec(s.t, id)
		v = vectors[id]
	}
	m := decode(s.t, v.Payload)
	orig := m["MessageId"]
	m["MessageId"] = msgID(compA, at(sec))
	if _, ok := m["ReplyTo"]; ok {
		m["ReplyTo"] = replyTo
	}
	ts := at(sec).Format("2006-01-02T15:04:05.000Z")
	if _, ok := m["Timestamp"]; ok {
		m["Timestamp"] = ts
	}
	if arr, ok := m["Messages"].([]any); ok {
		for _, x := range arr {
			if d, ok := x.(map[string]any); ok {
				if _, ok := d["Timestamp"]; ok {
					d["Timestamp"] = ts
				}
				delete(d, "SequenceNumber")
				if pg, ok := d["Payload"].(map[string]any); ok && pg["PaginationId"] == orig {
					pg["PaginationId"] = m["MessageId"]
				}
			}
		}
	}
	if edit != nil {
		edit(m)
	}
	s.add(sec, v.Topic, m)
}

func messages(m map[string]any) []any { return m["Messages"].([]any) }
func first(m map[string]any) map[string]any {
	return messages(m)[0].(map[string]any)
}

// call sends a Call NewDataSetWriterId from compB to compA.
func (s *sc) call(sec float64, requestID int64, timeout int64) string {
	m := map[string]any{"MessageId": msgID(compB, at(sec)), "MessageType": "ua-action-request", "PublisherId": compA.pub(),
		"Timestamp": at(sec).Format("2006-01-02T15:04:05.000Z"), "ResponseAddress": compA.base() + "/Reply/NewDataSetWriterId",
		"RequestorId": compB.pub(), "TimeoutHint": timeout,
		"Messages": []any{map[string]any{"DataSetWriterId": 8, "ActionTargetId": 1, "RequestId": requestID, "ActionState": 1,
			"Payload": map[string]any{"Resource": "ReferenceDesignation"}}}}
	s.add(sec, compA.base()+"/Call/NewDataSetWriterId", m)
	return m["MessageId"].(string)
}

func (s *sc) reply(sec float64, requestID, writerID int64) {
	m := map[string]any{"MessageId": msgID(compA, at(sec)), "MessageType": "ua-action-response", "PublisherId": compA.pub(),
		"Timestamp": at(sec).Format("2006-01-02T15:04:05.000Z"), "RequestorId": compB.pub(),
		"Messages": []any{map[string]any{"DataSetWriterId": 8, "ActionTargetId": 1, "Status": map[string]any{}, "RequestId": requestID,
			"ActionState": 2, "Payload": map[string]any{"DataSetWriterId": writerID, "Ttl": 10}}}}
	s.add(sec, compA.base()+"/Reply/NewDataSetWriterId", m)
}

// start is the common beginning: Birth and Health of A and B.
func (s *sc) start() {
	s.birth(0, compA)
	s.health(0.05, compA, "NORMAL_0", 100)
	s.birth(0.1, compB)
	s.health(0.15, compB, "NORMAL_0", 100)
}

func (s *sc) bytes() []byte {
	var buf bytes.Buffer
	w, err := capture.NewWriter(&buf, s.header)
	if err != nil {
		s.t.Fatal(err)
	}
	for _, r := range s.records {
		if err := w.Write(r); err != nil {
			s.t.Fatal(err)
		}
	}
	if err := w.Close(t0.Add(s.end), nil); err != nil {
		s.t.Fatal(err)
	}
	return buf.Bytes()
}

// fixtures returns the scenarios by file name.
func fixtures(t *testing.T) map[string]*sc {
	out := map[string]*sc{}
	add := func(s *sc, end float64) {
		s.end = time.Duration(end * float64(time.Second))
		out[s.name+".jsonl"] = s
	}

	// --- positive -------------------------------------------------------
	s := scenario(t, "lifecycle-pass", "two components from Birth to Close over 130 s: Health heartbeat every 30 s, Get MAM with pagination, Call/Reply, Data with Metadata and PublicationList, Syslog Notice")
	s.start()
	getMAM := s.req(1, "V-PAY-005", nil)
	s.fromVector(1.1, "V-PAY-006", getMAM, nil)
	s.add(1.2, compA.base()+"/Pub/Health/"+enc(groe), healthMsg(compA, at(1.2), groe, 3457, "NORMAL_0", 100, ""))
	s.call(2, 1, 10000)
	s.reply(2.1, 1, 57211)
	s.call(3, 1, 10000)
	s.reply(3.1, 1, 57212)
	s.fromVector(5, "V-PAY-058", "", nil)
	getMeta := s.req(5.1, "V-PAY-020", nil)
	s.fromVector(5.2, "V-PAY-021", getMeta, nil)
	getPL := s.req(6, "V-PAY-034", nil)
	s.fromVector(6.1, "V-PAY-037", getPL, nil)
	s.fromVector(10, "V-PAY-064", "", nil)
	s.heartbeats(compA, 0, 120, 0.05)
	s.heartbeats(compB, 0, 120, 0.15)
	sortRecords(s)
	s.close(126, compB)
	s.close(127, compA)
	add(s, 128)

	s = scenario(t, "reconnect-pass", "a component loses its connection (the broker publishes its Will, created at the connect) and reconnects with a new Birth")
	s.birth(0, compA)
	s.health(0.05, compA, "NORMAL_0", 100)
	s.will(20, compA, -0.01)
	s.birth(25, compA)
	s.health(25.05, compA, "NORMAL_0", 100)
	add(s, 30)

	// --- negative -------------------------------------------------------
	s = scenario(t, "invalid-message", "a Health with HealthScore 101 (T7-MON-01)")
	s.start()
	s.health(5, compA, "NORMAL_0", 101)
	add(s, 10)

	s = scenario(t, "publisher-mismatch", "a Pub Health on the topic of A whose PublisherId names B (T7-MON-02)")
	s.start()
	m := healthMsg(compA, at(5), compA.il, compA.health, "NORMAL_0", 100, "")
	m["PublisherId"] = compB.pub()
	s.add(5, compA.base()+"/Pub/Health/"+compA.self(), m)
	add(s, 10)

	s = scenario(t, "will-then-publish", "after its Will the component publishes a Health without a new Birth (T7-MON-03)")
	s.birth(0, compA)
	s.health(0.05, compA, "NORMAL_0", 100)
	s.will(20, compA, -0.01)
	s.health(25, compA, "NORMAL_0", 100)
	add(s, 30)

	s = scenario(t, "birth-mam-differs", "a regular Pub MAM about itself with another DataSetWriterId and another payload at the same RevisionCounter as the Birth (T7-MON-03)")
	s.start()
	getMAM = s.req(1, "V-PAY-026", nil)
	s.fromVector(1.1, "V-PAY-028", getMAM, func(m map[string]any) {
		d := first(m)
		d["DataSetWriterId"] = json.Number("56")
		d["Payload"].(map[string]any)["SoftwareRevision"] = "1.0.2"
	})
	add(s, 5)

	s = scenario(t, "no-health", "a component publishes Data for 70 s after its Birth but no Health about itself (T7-MON-04, T7-MON-05)")
	s.birth(0, compA)
	for x := 1.0; x <= 61; x += 20 {
		s.fromVector(x, "V-PAY-058", "", nil)
	}
	add(s, 70)

	s = scenario(t, "heartbeat-gap", "the Health about itself pauses for 70 s (T7-MON-05)")
	s.birth(0, compA)
	s.health(0.05, compA, "NORMAL_0", 100)
	s.health(30, compA, "NORMAL_0", 100)
	s.health(100, compA, "NORMAL_0", 100)
	s.health(130, compA, "NORMAL_0", 100)
	add(s, 140)

	s = scenario(t, "close-then-publish", "after its Close the component publishes a Health without a new Birth (T7-MON-05)")
	s.birth(0, compA)
	s.health(0.05, compA, "NORMAL_0", 100)
	s.close(10, compA)
	s.health(12, compA, "NORMAL_0", 100)
	add(s, 15)

	s = scenario(t, "message-id-duplicate", "two Pub Data with the same MessageId (T7-MON-06)")
	s.start()
	s.fromVector(5, "V-PAY-058", "", nil)
	s.fromVector(6, "V-PAY-058", "", func(m map[string]any) { m["MessageId"] = msgID(compA, at(5)) })
	add(s, 10)

	s = scenario(t, "request-id-pending", "a second Call with the RequestId of a pending Call (T7-MON-07)")
	s.start()
	s.call(2, 7, 10000)
	s.call(3, 7, 10000)
	s.reply(3.5, 7, 57211)
	add(s, 10)

	s = scenario(t, "writer-id-conflict", "DataSetWriterId 93 for Data oee and for an Event; Metadata and PublicationList name other ids (T7-MON-08)")
	s.start()
	s.fromVector(5, "V-PAY-058", "", nil)
	s.fromVector(5.5, "V-PAY-062", "", func(m map[string]any) { first(m)["DataSetWriterId"] = json.Number("93") })
	getMeta = s.req(6, "V-PAY-020", nil)
	s.fromVector(6.1, "V-PAY-021", getMeta, func(m map[string]any) { m["DataSetWriterId"] = json.Number("94") })
	getPL = s.req(7, "V-PAY-034", nil)
	s.fromVector(7.1, "V-PAY-037", getPL, func(m map[string]any) { first(m)["Payload"].(map[string]any)["DataSetWriterId"] = json.Number("95") })
	add(s, 10)

	s = scenario(t, "identity-conflict", "A and B publish the MAM of an asset with the same Identification Link but another serial number, and the AppId of A also publishes as Aggregation (T7-MON-10)")
	s.start()
	getMAM = s.req(1, "V-PAY-005", nil)
	s.fromVector(1.1, "V-PAY-006", getMAM, nil)
	sensor := func(c comp, sec float64, id int64, serial string) {
		mam := map[string]any{}
		for k, v := range compB.mam {
			mam[k] = v
		}
		mam["Manufacturer"] = map[string]any{"Locale": "en-US", "Text": "GROE"}
		mam["ManufacturerUri"], mam["Model"], mam["ProductCode"] = "groe.mca", map[string]any{"Locale": "en-US", "Text": "Sensor S"}, "S-1"
		mam["DeviceClass"], mam["SerialNumber"], mam["ProductInstanceUri"] = "", serial, sensorIL
		mam["Description"] = map[string]any{"Locale": "en-US", "Text": "Sensor"}
		s.add(sec, c.base()+"/Pub/MAM/"+enc(sensorIL), network(c, at(sec), classMAM, "", dsMsg(id, sensorIL, "", mam)))
	}
	sensor(compA, 2, 4799, "20443")
	sensor(compB, 2.5, 31, "20444")
	agg := compA
	agg.st = "Aggregation"
	s.add(3, agg.base()+"/Pub/Health/"+agg.self(), healthMsg(agg, at(3), agg.il, agg.health, "NORMAL_0", 100, ""))
	add(s, 5)

	s = scenario(t, "empty-pub", "an unsolicited Pub Health without DataSetMessage (T7-MON-11)")
	s.start()
	s.add(5, compA.base()+"/Pub/Health", network(compA, at(5), classHealth, ""))
	add(s, 10)

	s = scenario(t, "retain-qos", "a Health delivered with the retain flag and QoS 1 (T7-MON-12)")
	s.start()
	b, _ := json.Marshal(healthMsg(compA, at(5), compA.il, compA.health, "NORMAL_0", 100, ""))
	s.raw(5, compA.base()+"/Pub/Health/"+compA.self(), b, 1, true)
	add(s, 10)

	s = scenario(t, "packet-size", "a Pub MAM of 1.8 kB on a Message Bus with MaxPacketSize 1024 (T7-MON-18; below the minimum 262144 of broker.json to keep the fixture small)")
	s.header.MaxPacketSize = 1024
	s.start()
	getMAM = s.req(1, "V-PAY-005", nil)
	s.fromVector(1.1, "V-PAY-006", getMAM, nil)
	add(s, 5)

	s = scenario(t, "pagination-broken", "a paged MAM response skips page 2; a second one ends with HasNext true and no further page (T7-MON-18)")
	s.start()
	getMAM = s.req(1, "V-PAY-005", func(m map[string]any) { first(m)["Payload"].(map[string]any)["PerPage"] = json.Number("1") })
	page := func(sec float64, first float64, n int64, next bool) {
		s.fromVector(sec, "V-PAY-006", getMAM, func(m map[string]any) {
			arr := messages(m)
			pg := arr[2].(map[string]any)["Payload"].(map[string]any)
			pg["TotalCount"], pg["PerPage"], pg["Page"], pg["HasNext"] = json.Number("3"), json.Number("1"), json.Number(fmt.Sprint(n)), next
			pg["PaginationId"] = msgID(compA, at(first))
			m["Messages"] = []any{arr[n%2], arr[2]}
		})
	}
	page(1.1, 1.1, 1, true)
	page(1.2, 1.1, 3, false)
	getMAM = s.req(2, "V-PAY-005", func(m map[string]any) { first(m)["Payload"].(map[string]any)["PerPage"] = json.Number("1") })
	page(2.1, 2.1, 1, true)
	add(s, 10)

	s = scenario(t, "debug-event", "a Syslog Debug event after the Birth without a Set of the log level (T7-MON-19)")
	s.start()
	s.fromVector(5, "V-PAY-064", "", func(m map[string]any) {
		d := first(m)
		d["WriterGroupName"] = "Syslog/Debug"
		d["Payload"].(map[string]any)["Number"] = json.Number("15")
	})
	fixTopic(s, "Syslog/Notice", "Syslog/Debug")
	add(s, 10)
	return out
}

func fixTopic(s *sc, from, to string) {
	r := &s.records[len(s.records)-1]
	r.Topic = strings.TrimSuffix(r.Topic, from) + to
}

func sortRecords(s *sc) {
	rs := s.records
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Received.Before(rs[j-1].Received); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// TestFixtureFiles keeps the capture files equal to the scenarios.
func TestFixtureFiles(t *testing.T) {
	fx := fixtures(t)
	if *update {
		if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
			t.Fatal(err)
		}
		old, _ := filepath.Glob(filepath.Join(fixtureDir, "*.jsonl"))
		for _, f := range old {
			os.Remove(f)
		}
	}
	for name, s := range fx {
		path := filepath.Join(fixtureDir, name)
		b := s.bytes()
		if *update {
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		have, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run with -update)", name, err)
		}
		if !bytes.Equal(have, b) {
			t.Errorf("%s differs from its scenario (run go test ./internal/monitor -run TestFixtureFiles -update)", name)
		}
	}
	files, _ := filepath.Glob(filepath.Join(fixtureDir, "*.jsonl"))
	for _, f := range files {
		if fx[filepath.Base(f)] == nil {
			t.Errorf("%s has no scenario", f)
		}
	}
}

// TestFixturePayloads: every message of a fixture is valid except where the
// scenario is about an invalid message.
func TestFixturePayloads(t *testing.T) {
	v, err := validate.New()
	if err != nil {
		t.Fatal(err)
	}
	invalid := map[string]bool{"invalid-message.jsonl": true, "publisher-mismatch.jsonl": true}
	for name, s := range fixtures(t) {
		bad := 0
		for i, r := range s.records {
			res, err := v.Validate(validate.Message{Topic: r.Topic, Payload: json.RawMessage(r.Payload)})
			if err != nil {
				t.Fatal(err)
			}
			if !res.Valid {
				bad++
				if !invalid[name] {
					t.Errorf("%s record %d (%s): %v", name, i+1, r.Topic, res.Findings)
				}
			}
		}
		if invalid[name] && bad == 0 {
			t.Errorf("%s: all records valid", name)
		}
	}
}

func marshal(m map[string]any) ([]byte, error) { return json.Marshal(m) }
