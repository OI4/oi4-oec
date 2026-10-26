package capture

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, Header{Tool: "oec-check", Broker: "mqtts://localhost:8883", Subscription: "Oi4v2/#", SubscribeQoS: 1, Started: t0})
	if err != nil {
		t.Fatal(err)
	}
	recs := []Record{
		{Topic: "Oi4v2/a", Payload: []byte("{\n  \"a\": 1,\n  \"a\": 2\n}"), Received: t0.Add(time.Second), QoS: 1},
		{Topic: "Oi4v2/b", Payload: []byte{0xff, 0x00, '{'}, Received: t0.Add(2 * time.Second), Retain: true},
		{Topic: "Oi4v2/c", Payload: nil, Received: t0.Add(3 * time.Second)},
		{Topic: "Oi4v2/d", Payload: []byte(`{"x":"\u00e4"}`), Received: t0.Add(time.Second)}, // raised to the previous time
	}
	for _, r := range recs {
		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(t0.Add(10*time.Second), nil); err != nil {
		t.Fatal(err)
	}
	if !IsCapture(buf.Bytes()) {
		t.Fatal("IsCapture = false")
	}
	c, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	if len(c.Records) != 4 || c.Trailer == nil || c.Trailer.Messages != 4 || !c.End().Equal(t0.Add(10*time.Second)) {
		t.Fatalf("capture %+v", c)
	}
	r := c.Records[0]
	if string(r.Payload) != `{"a":1,"a":2}` || r.Size != 22 || r.QoS != 1 || r.Line != 2 {
		t.Errorf("record 0: %+v %q", r, r.Payload)
	}
	if r := c.Records[1]; !bytes.Equal(r.Payload, []byte{0xff, 0x00, '{'}) || !r.Retain || r.Size != 3 {
		t.Errorf("record 1: %+v", r)
	}
	if r := c.Records[2]; len(r.Payload) != 0 || r.Size != 0 {
		t.Errorf("record 2: %+v", r)
	}
	if r := c.Records[3]; !r.Received.Equal(t0.Add(3*time.Second)) || string(r.Payload) != `{"x":"\u00e4"}` {
		t.Errorf("record 3: %+v %q", r, r.Payload)
	}
	if !c.Header.Started.Equal(t0) || c.Header.Format != Format {
		t.Errorf("header %+v", c.Header)
	}
}

func TestReadErrors(t *testing.T) {
	h := `{"format":"oec-capture","version":1,"subscribe_qos":1,"started":"2026-10-26T12:00:00Z"}` + "\n"
	rec := func(topic, at string) string {
		return `{"topic":"` + topic + `","payload":{},"received":"` + at + `","qos":0,"retain":false}` + "\n"
	}
	for name, in := range map[string]string{
		"empty":         "",
		"no header":     rec("Oi4v2/a", "2026-10-26T12:00:01Z"),
		"version":       strings.Replace(h, `"version":1`, `"version":2`, 1),
		"no started":    `{"format":"oec-capture","version":1,"subscribe_qos":1}` + "\n",
		"unknown key":   h + `{"topic":"Oi4v2/a","received":"2026-10-26T12:00:01Z","qos":0,"retain":false,"x":1}` + "\n",
		"no topic":      h + `{"topic":"","received":"2026-10-26T12:00:01Z","qos":0,"retain":false}` + "\n",
		"no received":   h + `{"topic":"Oi4v2/a","qos":0,"retain":false}` + "\n",
		"qos":           h + `{"topic":"Oi4v2/a","received":"2026-10-26T12:00:01Z","qos":3,"retain":false}` + "\n",
		"both payloads": h + `{"topic":"Oi4v2/a","payload":{},"payload_base64":"","received":"2026-10-26T12:00:01Z","qos":0,"retain":false}` + "\n",
		"base64":        h + `{"topic":"Oi4v2/a","payload_base64":"%%","received":"2026-10-26T12:00:01Z","qos":0,"retain":false}` + "\n",
		"backwards":     h + rec("Oi4v2/a", "2026-10-26T12:00:02Z") + rec("Oi4v2/a", "2026-10-26T12:00:01Z"),
		"before start":  h + rec("Oi4v2/a", "2026-10-26T11:00:00Z"),
		"after trailer": h + `{"ended":"2026-10-26T12:00:05Z","messages":0}` + "\n" + rec("Oi4v2/a", "2026-10-26T12:00:06Z"),
		"count":         h + rec("Oi4v2/a", "2026-10-26T12:00:01Z") + `{"ended":"2026-10-26T12:00:05Z","messages":2}` + "\n",
		"no json":       h + "x\n",
	} {
		if _, err := Read(strings.NewReader(in)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if IsCapture([]byte(rec("Oi4v2/a", "2026-10-26T12:00:01Z"))) {
		t.Error("a message file is no capture")
	}
}
