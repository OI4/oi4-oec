package oec

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// newLevelApp builds an app of level on the container storage of b with a
// persistent store (Level 2 on offers the ReferenceDesignation,
// REQ-RES-REFDES-002).
func newLevelApp(t *testing.T, b *broker, level Level, st ServiceType) *App {
	t.Helper()
	app, err := New(Options{Level: level, ServiceType: st, Product: testProduct,
		ContainerRoot: b.container(t), StoreDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

// asJSON decodes a payload of the test broker.
func asJSON(t *testing.T, payload string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("%v: %s", err, payload)
	}
	return v
}

var requestSeq int

// request is a Set request answering the publication pub with payload:
// its first DataSetMessage (WriterGroupName, DataSetWriterId of the
// provider) with the new Payload; it returns the MessageId.
func request(t *testing.T, b *broker, conn int, topic string, pub map[string]any, payload any) string {
	t.Helper()
	requestSeq++
	id := fmt.Sprintf("%d-Aggregation/vendor.com/Client/C-1/1", 1700000000000+requestSeq)
	dsm := pub["Messages"].([]any)[0].(map[string]any)
	msg := map[string]any{}
	for k, v := range dsm {
		msg[k] = v
	}
	msg["Payload"] = payload
	req, _ := json.Marshal(map[string]any{
		"MessageId":      id,
		"MessageType":    "ua-data",
		"PublisherId":    "Aggregation/vendor.com/Client/C-1/1",
		"DataSetClassId": pub["DataSetClassId"],
		"Messages":       []any{msg},
	})
	b.send(t, conn, topic, string(req))
	return id
}

// answer waits for the publication on topic answering the request id.
func (b *broker) answer(t *testing.T, from int, topic, id string) (map[string]any, int) {
	t.Helper()
	s, i := b.expect(t, from, "answer to "+id+" on "+topic, func(s seen) bool {
		return s.Seen == "publish" && s.Topic == topic && strings.Contains(s.Payload, `"ReplyTo":"`+id+`"`)
	})
	return asJSON(t, s.Payload), i
}

// eventNumber is the Number of the Status event ev.
func eventNumber(t *testing.T, ev map[string]any) StatusCode {
	t.Helper()
	n, ok := ev["Messages"].([]any)[0].(map[string]any)["Payload"].(map[string]any)["Number"].(float64)
	if !ok {
		t.Fatalf("no Number in %v", ev)
	}
	return StatusCode(n)
}

// pump is the topic level of the Identification Link of the test asset
// (base64url, REQ-TOP-019).
var pump = base64.RawURLEncoding.EncodeToString([]byte("vendor.com/Pump/P-3/0815"))

// A represented asset with MAM, Health, a Generic event and a writable
// DataSet: Set Data decided by the handler, answered with the DataSet and
// the Status event Good or the handler's code; a panic answers
// BadInternalError (REQ-CMP-045, REQ-RES-DATA-011/012, REQ-RES-EVENT-005,
// REQ-RES-EVENT-009).
func TestProviderData(t *testing.T) {
	b := startBroker(t)
	app := newLevelApp(t, b, Level2, Utility)
	asset, err := app.AddAsset(AssetOptions{
		Product:      Product{ManufacturerURI: "vendor.com", Model: "Pump", ProductCode: "P-3", HardwareRevision: "2"},
		SerialNumber: "0815",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := asset.SetHealth(Normal, 90); err != nil {
		t.Fatal(err)
	}
	speed, err := asset.AddData(DataDef{
		Tag: "speed",
		Fields: []Field{
			{Name: "Setpoint", Type: TypeDouble, Unit: "rpm", Range: &Range{0, 3000}},
			{Name: "Running", Type: TypeBoolean},
		},
		Writable: WritableModeAndInterval,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := speed.Update(map[string]any{"Setpoint": "fast"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Update with a wrong type: %v", err)
	}
	if err := speed.Update(map[string]any{"Setpoint": 1000.0, "Running": true}); err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		calls []json.RawMessage
	)
	if err := speed.OnSet(func(v json.RawMessage) error {
		mu.Lock()
		calls = append(calls, v)
		mu.Unlock()
		var req struct{ Setpoint float64 }
		if err := json.Unmarshal(v, &req); err != nil {
			return err
		}
		if req.Setpoint < 0 || req.Setpoint > 3000 {
			return BadOutOfRange
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	connect, _ := b.expect(t, 0, "CONNECT", func(s seen) bool { return s.Seen == "connect" })
	_, i := b.publish(t, 0, "/Pub/MAM/"+pump)
	health, _ := b.publish(t, 0, "/Pub/Health/"+pump)
	if !strings.Contains(health.Payload, `"HealthScore":90`) {
		t.Fatalf("asset Health %s", health.Payload)
	}
	data, _ := b.publish(t, i, "/Pub/Data/"+pump+"/speed")
	pub := asJSON(t, data.Payload)
	if p := pub["Messages"].([]any)[0].(map[string]any)["Payload"]; fmt.Sprint(p) != "map[Running:true Setpoint:1000]" {
		t.Fatalf("Pub Data payload %v", p)
	}
	prefix := data.Topic[:strings.Index(data.Topic, "/Pub/")]
	setTopic := prefix + "/Set/Data/" + pump + "/speed"
	good := prefix + "/Pub/Event/" + pump + "/Status/Good"
	bad := prefix + "/Pub/Event/" + pump + "/Status/Bad"

	id := request(t, b, connect.Conn, setTopic, pub, map[string]any{"Setpoint": 1500.0, "Running": true})
	changed, j := b.answer(t, i, data.Topic, id)
	if p := changed["Messages"].([]any)[0].(map[string]any)["Payload"]; fmt.Sprint(p) != "map[Running:true Setpoint:1500]" {
		t.Fatalf("answer %v", p)
	}
	if ev, _ := b.answer(t, i, good, id); eventNumber(t, ev) != Good {
		t.Fatalf("Status event %v", ev)
	}
	mu.Lock()
	if len(calls) != 1 || !strings.Contains(string(calls[0]), `"Setpoint":1500`) {
		t.Fatalf("handler calls %q", calls)
	}
	mu.Unlock()

	id = request(t, b, connect.Conn, setTopic, pub, map[string]any{"Setpoint": 5000.0, "Running": true})
	if ev, _ := b.answer(t, j, bad, id); eventNumber(t, ev) != BadOutOfRange {
		t.Fatalf("Status event %v", ev)
	}

	// a further OnSet replaces the handler; a panic is answered
	// BadInternalError and the app runs on
	if err := speed.OnSet(func(json.RawMessage) error { panic("boom") }); err != nil {
		t.Fatal(err)
	}
	id = request(t, b, connect.Conn, setTopic, pub, map[string]any{"Setpoint": 10.0, "Running": false})
	if ev, _ := b.answer(t, j, bad, id); eventNumber(t, ev) != BadInternalError {
		t.Fatalf("Status event %v", ev)
	}
	if err := speed.OnSet(func(json.RawMessage) error { return errors.New("no") }); err != nil {
		t.Fatal(err)
	}
	id = request(t, b, connect.Conn, setTopic, pub, map[string]any{"Setpoint": 10.0, "Running": false})
	if ev, _ := b.answer(t, j, bad, id); eventNumber(t, ev) != Bad {
		t.Fatalf("Status event %v", ev)
	}

	if err := asset.EmitEvent(Event{Level: "Maintenance", Number: 7, Description: "bearing", Details: map[string]any{"hours": 1200}}); err != nil {
		t.Fatal(err)
	}
	ev, _ := b.publish(t, j, "/Pub/Event/"+pump+"/Generic/Maintenance")
	for _, want := range []string{`"Number":7`, `"bearing"`, `"hours":1200`} {
		if !strings.Contains(ev.Payload, want) {
			t.Fatalf("event %s: no %s", ev.Payload, want)
		}
	}
	if err := asset.EmitEvent(Event{Level: "x", Details: []int{1}}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("details no object: %v", err)
	}

	if err := speed.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := speed.Update(map[string]any{"Setpoint": 1.0}); err == nil {
		t.Fatal("Update after Remove succeeded")
	}
	if err := asset.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := asset.AddData(DataDef{Tag: "x", Fields: []Field{{Name: "A", Type: TypeDouble}}}); err == nil {
		t.Fatal("AddData after Remove succeeded")
	}
	if err := app.This().Remove(); err == nil {
		t.Fatal("This().Remove() succeeded")
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"AddAsset": func() error {
			_, err := app.AddAsset(AssetOptions{Product: testProduct, SerialNumber: "1"})
			return err
		}(),
		"OnSet": speed.OnSet(func(json.RawMessage) error { return nil }),
		"Event": app.This().EmitEvent(Event{Level: "x"}),
	} {
		if !errors.Is(err, ErrClosed) {
			t.Errorf("%s after Close: %v", name, err)
		}
	}
}

// Definitions the Go side refuses before the SDK.
func TestDataDefRejects(t *testing.T) {
	b := startBroker(t)
	app := newLevelApp(t, b, Level2, Utility)
	ok := []Field{{Name: "Pv", Type: TypeDouble}}
	for name, def := range map[string]DataDef{
		"type":        {Tag: "a", Fields: []Field{{Name: "Pv"}}},
		"mode":        {Tag: "a", Fields: ok, Mode: ModeOff | ModeFilter},
		"mode bits":   {Tag: "a", Fields: ok, Mode: 1 << 5},
		"interval":    {Tag: "a", Fields: ok, Interval: 1500 * time.Microsecond},
		"negative":    {Tag: "a", Fields: ok, Interval: -time.Second},
		"tag":         {Tag: "", Fields: ok},
		"no fields":   {Tag: "a"},
		"duplicate":   {Tag: "a", Fields: []Field{ok[0], ok[0]}},
		"Oi4Pv field": {Tag: OI4PV, Fields: []Field{{Name: "Value", Type: TypeDouble}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := app.This().AddData(def); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("got %v, want ErrInvalidArgument", err)
			}
		})
	}
	for _, m := range []Mode{0, ModeOff, ModeSource | ModeOnRequest} {
		if _, err := app.This().AddData(DataDef{Tag: fmt.Sprintf("m%d", m), Fields: ok, Mode: m, Interval: time.Second}); err != nil {
			t.Fatalf("Mode %#x: %v", uint(m), err)
		}
	}
	if s := BuiltinType(99).String(); s != "BuiltinType(99)" || TypeGUID.String() != "Guid" {
		t.Fatal(s)
	}
}

// statusCodeOf maps the results of a handler.
func TestStatusCodeOf(t *testing.T) {
	for _, c := range []struct {
		err  error
		want StatusCode
	}{
		{nil, Good},
		{BadOutOfRange, BadOutOfRange},
		{fmt.Errorf("wrapped: %w", BadNotWritable), BadNotWritable},
		{&Error{Status: StatusStatus, StatusCode: BadNotFound}, BadNotFound},
		{&Error{Status: StatusIO}, Bad},
		{Good, Bad},
		{errors.New("x"), Bad},
		{StatusCode(0x40000000), StatusCode(0x40000000)},
	} {
		if got := statusCodeOf(c.err); got != c.want {
			t.Errorf("statusCodeOf(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// me is the topic level of the test app's AppId.
var me = base64.RawURLEncoding.EncodeToString([]byte("example.com/Go,20Test/GO-TEST-1/app-1"))

// Config of the app: the sensitive key masked in every Pub, a Set Config
// with the mask keeps it and is answered with the Config and Good, the
// handler's rule refuses with its code; Get/Set from the application side
// (REQ-RES-CONFIG-010, REQ-RES-CONFIG-020 … 025, REQ-RES-EVENT-005).
func TestProviderConfig(t *testing.T) {
	b := startBroker(t)
	app := newLevelApp(t, b, Level2, Utility)
	interval := NumberObject("interval", 60)
	interval.Min, interval.Max, interval.Mandatory, interval.Unit = new(10.0), new(3600.0), true, "s"
	key := StringObject("api_key", "")
	key.Sensitive, key.Label = true, "API key"
	verbose := BooleanObject("verbose", false)
	cfg, err := app.This().AddConfig(ConfigDef{Context: "service", Label: "Service", Groups: []ConfigGroup{
		{Name: "common", Label: "Common settings", Objects: []ConfigObject{interval, key, verbose}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu       sync.Mutex
		proposed []ConfigValues
	)
	if err := cfg.OnSet(func(v ConfigValues) error {
		mu.Lock()
		proposed = append(proposed, v)
		mu.Unlock()
		if i, ok := v.Number("common", "interval"); !ok || int(i)%10 != 0 {
			return BadOutOfRange
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("common", "api_key", "s3cr3t"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("common", "interval", "5"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Set below Min: %v", err)
	}
	if v, ok, err := cfg.Get("common", "api_key"); err != nil || !ok || v != "s3cr3t" {
		t.Fatalf("Get api_key = %q %v %v", v, ok, err)
	}
	if _, ok, err := cfg.Get("common", "nope"); err != nil || ok {
		t.Fatalf("Get unknown: %v %v", ok, err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	connect, _ := b.expect(t, 0, "CONNECT", func(s seen) bool { return s.Seen == "connect" })
	topic := "Oi4v2/Utility/" + me + "/Pub/Config/" + me + "/service"
	pubCfg, i := b.publish(t, 0, topic)
	if strings.Contains(pubCfg.Payload, "s3cr3t") || !strings.Contains(pubCfg.Payload, `"Value":"********"`) {
		t.Fatalf("Pub Config not masked: %s", pubCfg.Payload)
	}
	pub := asJSON(t, pubCfg.Payload)
	set := "Oi4v2/Utility/" + me + "/Set/Config/" + me + "/service"

	id := request(t, b, connect.Conn, set, pub, map[string]any{"common": map[string]any{
		"interval": map[string]any{"Value": "30"},
		"api_key":  map[string]any{"Value": "********"},
	}})
	changed, j := b.answer(t, i, topic, id)
	common := changed["Messages"].([]any)[0].(map[string]any)["Payload"].(map[string]any)["common"].(map[string]any)
	if v := common["interval"].(map[string]any)["Value"]; v != "30" {
		t.Fatalf("interval %v", v)
	}
	if ev, _ := b.answer(t, i, "Oi4v2/Utility/"+me+"/Pub/Event/"+me+"/Status/Good", id); eventNumber(t, ev) != Good {
		t.Fatalf("Status %v", ev)
	}
	mu.Lock()
	if len(proposed) != 1 || proposed[0]["common"]["api_key"] != "s3cr3t" || proposed[0]["common"]["verbose"] != "false" {
		t.Fatalf("proposed %v", proposed)
	}
	mu.Unlock()
	if v, _, _ := cfg.Get("common", "api_key"); v != "s3cr3t" {
		t.Fatalf("api_key after the masked Set: %q", v)
	}

	id = request(t, b, connect.Conn, set, pub, map[string]any{"common": map[string]any{"interval": map[string]any{"Value": "25"}}})
	if ev, _ := b.answer(t, j, "Oi4v2/Utility/"+me+"/Pub/Event/"+me+"/Status/Bad", id); eventNumber(t, ev) != BadOutOfRange {
		t.Fatalf("Status %v", ev)
	}
	if v, _, _ := cfg.Get("common", "interval"); v != "30" {
		t.Fatalf("interval after the refused Set: %q", v)
	}
	if err := cfg.Set("common", "verbose", "true"); err != nil {
		t.Fatal(err)
	}
	b.expect(t, j, "Pub Config with verbose true", func(s seen) bool {
		return s.Seen == "publish" && s.Topic == topic && strings.Contains(s.Payload, `"verbose":{"Type":"Boolean","Value":"true"`)
	})

	v := ConfigValues{"g": {"n": "1.5", "b": "true", "e": ""}}
	if f, ok := v.Number("g", "n"); !ok || f != 1.5 {
		t.Fatal("Number")
	}
	if x, ok := v.Bool("g", "b"); !ok || !x {
		t.Fatal("Bool")
	}
	if _, ok := v.Value("g", "e"); ok {
		t.Fatal("unset Value")
	}
	if _, ok := v.Number("g", "b"); ok {
		t.Fatal("Number of a Boolean")
	}
}

// Config definitions refused by the Go side or the SDK.
func TestConfigDefRejects(t *testing.T) {
	b := startBroker(t)
	app := newLevelApp(t, b, Level2, Utility)
	bad := func(o ConfigObject) ConfigDef {
		return ConfigDef{Context: "c", Groups: []ConfigGroup{{Name: "common", Objects: []ConfigObject{o}}}}
	}
	pattern := StringObject("s", "x")
	pattern.Pattern = "("
	minmax := NumberObject("n", 5)
	minmax.Min, minmax.Max = new(10.0), new(1.0)
	defOut := NumberObject("n", 5)
	defOut.Min = new(10.0)
	for name, def := range map[string]ConfigDef{
		"type":       bad(ConfigObject{Name: "x", Default: "1"}),
		"name":       bad(StringObject("1bad", "")),
		"reserved":   bad(StringObject("Name", "")),
		"pattern":    bad(pattern),
		"min > max":  bad(minmax),
		"default":    bad(defOut),
		"number":     bad(ConfigObject{Name: "n", Type: ConfigNumber, Default: "abc"}),
		"context":    {Context: ""},
		"twice":      {Context: "c", Groups: []ConfigGroup{{Name: "g"}, {Name: "g"}}},
		"min string": {Context: "c", Groups: []ConfigGroup{{Name: "g", Objects: []ConfigObject{{Name: "s", Type: ConfigString, Min: new(1.0)}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := app.This().AddConfig(def); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("got %v, want ErrInvalidArgument", err)
			}
		})
	}
	if _, err := app.This().AddConfigJSON("j", []byte(`{"common":{"Name":{"Locale":"en-US","Text":"C"},"a":{"Type":"String","Value":"x","Name":{"Locale":"en-US","Text":"A"}}}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.This().AddConfigJSON("k", []byte(`[]`)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("JSON array: %v", err)
	}
}
