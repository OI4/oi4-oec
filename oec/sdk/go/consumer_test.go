package oec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The provider of the consumer tests: the test product on its container
// storage (hostname app-1) with the represented pump.
const (
	providerIL = "example.com/Go,20Test/GO-TEST-1/app-1"
	pumpIL     = "vendor.com/Pump/P-3/0815"
)

var consumerProduct = Product{ManufacturerURI: "example.com", Model: "Go Consumer", ProductCode: "GO-CONS-1"}

// consumerPair starts a Level 3 provider (OTConnector with the pump and
// its writable DataSet "speed", Setpoint 0 … 3000 decided by the handler)
// and a Level 2 consumer (Aggregation) on b; both online, the provider
// answering.
func consumerPair(t *testing.T, b *broker) (provider, consumer *App, speed *DataSet) {
	t.Helper()
	provider = newLevelApp(t, b, Level3, OTConnector)
	pump, err := provider.AddAsset(AssetOptions{
		Product:      Product{ManufacturerURI: "vendor.com", Model: "Pump", ProductCode: "P-3"},
		SerialNumber: "0815",
	})
	if err != nil {
		t.Fatal(err)
	}
	speed, err = pump.AddData(DataDef{Tag: "speed", Fields: []Field{
		{Name: "Setpoint", Type: TypeDouble, Unit: "rpm", Range: &Range{0, 3000}},
		{Name: "Running", Type: TypeBoolean},
	}, Mode: ModeOnRequest | ModeSource})
	if err != nil {
		t.Fatal(err)
	}
	if err := speed.Update(map[string]any{"Setpoint": 1000.0, "Running": true}); err != nil {
		t.Fatal(err)
	}
	if err := speed.OnSet(func(v json.RawMessage) error {
		var req struct{ Setpoint float64 }
		if err := json.Unmarshal(v, &req); err != nil || req.Setpoint > 3000 {
			return BadOutOfRange
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	root := b.container(t)
	write(t, filepath.Join(root, "etc", "hostname"), "app-2\n") // own client id
	consumer, err = New(Options{Level: Level2, ServiceType: Aggregation, Product: consumerProduct,
		ContainerRoot: root, StoreDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = consumer.Close() })
	for _, a := range []*App{provider, consumer} {
		if err := a.Start(); err != nil {
			t.Fatal(err)
		}
	}
	// ready: the provider answers a Get MAM (its subscriptions are active)
	eventually(t, "the provider answers", func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		got, err := consumer.Get(ctx, ResourceMAM, Address{ServiceType: OTConnector, App: providerIL})
		return err == nil && len(got) == 2 // the MAM of the app and of the pump
	})
	return provider, consumer, speed
}

// Get, Set, Del and NewDataSetWriterId from Go against a Go provider on
// the test broker (REQ-MSG-015, REQ-CMP-047, REQ-RES-EVENT-005,
// REQ-ACT-017).
func TestConsumerRequests(t *testing.T) {
	b := startBroker(t)
	_, consumer, _ := consumerPair(t, b)
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	at := Address{ServiceType: OTConnector, App: providerIL, Source: pumpIL, Filter: "speed"}

	got, err := consumer.Get(ctx, ResourceData, at)
	if err != nil || len(got) != 1 {
		t.Fatalf("Get Data: %v %v", got, err)
	}
	r := got[0]
	if r.Resource != ResourceData || r.ServiceType != OTConnector || r.App != providerIL ||
		r.Source != pumpIL || r.Filter != "speed" || r.DataSetWriterID < 10 ||
		!bytes.Equal(r.Payload, []byte(`{"Running":true,"Setpoint":1000.0}`)) {
		t.Fatalf("Get Data: %+v (%s)", r, r.Payload)
	}
	mam, err := consumer.Get(ctx, ResourceMAM, Address{ServiceType: OTConnector, App: providerIL, Source: pumpIL})
	if err != nil || len(mam) != 1 || !strings.Contains(string(mam[0].Payload), `"ProductCode":"P-3"`) {
		t.Fatalf("Get MAM: %v %v", mam, err)
	}

	if err := consumer.Set(ctx, ResourceData, at, map[string]any{"Setpoint": 2000.0, "Running": true}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err = consumer.Get(ctx, ResourceData, at)
	if err != nil || len(got) != 1 || !strings.Contains(string(got[0].Payload), `"Setpoint":2000.0`) {
		t.Fatalf("Get after Set: %v %v", got, err)
	}
	err = consumer.Set(ctx, ResourceData, at, json.RawMessage(`{"Setpoint":5000.0,"Running":true}`))
	var e *Error
	if !errors.Is(err, ErrStatus) || !errors.Is(err, BadOutOfRange) || errors.Is(err, BadNotFound) ||
		!errors.As(err, &e) || e.StatusCode != BadOutOfRange {
		t.Fatalf("Set out of range: %v", err)
	}

	// Del/Data is no allowed combination (REQ-TOP-013): refused locally
	if err := consumer.Del(ctx, ResourceData, at, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Del Data: %v", err)
	}
	// the pump has no ReferenceDesignation: Status event Bad
	err = consumer.Del(ctx, ResourceReferenceDesignation, Address{ServiceType: OTConnector, App: providerIL, Source: pumpIL}, nil)
	if !errors.Is(err, BadNotFound) {
		t.Fatalf("Del ReferenceDesignation: %v", err)
	}
	// unknown asset (SF-099): Status event Bad BadNotFound
	_, err = consumer.Get(ctx, ResourceData, Address{ServiceType: OTConnector, App: providerIL, Source: "vendor.com/Pump/P-3/none", Filter: "speed"})
	if !errors.Is(err, BadNotFound) {
		t.Fatalf("Get of an unknown asset: %v", err)
	}

	id, ttl, err := consumer.NewDataSetWriterID(ctx, ResourceData, Address{ServiceType: OTConnector, App: providerIL})
	if err != nil || id < 10 || ttl != time.Minute {
		t.Fatalf("NewDataSetWriterID: %d %v %v", id, ttl, err)
	}
	if _, _, err := consumer.NewDataSetWriterID(ctx, ResourceData, Address{ServiceType: OTConnector, App: providerIL, Source: "vendor.com/Pump/P-3/none"}); !errors.Is(err, BadNotFound) {
		t.Fatalf("NewDataSetWriterID unknown: %v", err)
	}
	if _, _, err := consumer.NewDataSetWriterID(ctx, ResourceData, Address{ServiceType: OTConnector, App: providerIL, Filter: "x"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewDataSetWriterID with a Filter: %v", err)
	}

	// a request needs ServiceType and AppId; payloads must encode
	for name, err := range map[string]error{
		"Get without ServiceType": func() error { _, err := consumer.Get(ctx, ResourceMAM, Address{App: providerIL}); return err }(),
		"Get without AppId":       func() error { _, err := consumer.Get(ctx, ResourceMAM, Address{ServiceType: OTConnector}); return err }(),
		"Set with a channel":      consumer.Set(ctx, ResourceData, at, make(chan int)),
		"Set with NUL":            consumer.Set(ctx, ResourceData, Address{ServiceType: OTConnector, App: "a\x00b"}, map[string]int{}),
		"unknown resource":        func() error { _, err := consumer.Get(ctx, Resource(99), at); return err }(),
	} {
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The deadline of the context bounds a request; a done context is not
// sent at all.
func TestConsumerContext(t *testing.T) {
	b := startBroker(t)
	_, consumer, _ := consumerPair(t, b)
	nobody := Address{ServiceType: Utility, App: "example.com/Nobody/N-1/x"}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, err := consumer.Get(ctx, ResourceMAM, nobody)
	if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get without answer: %v", err)
	}
	if d := time.Since(began); d < 250*time.Millisecond || d > 3*time.Second {
		t.Fatalf("Get took %v", d)
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	for name, err := range map[string]error{
		"Get": func() error { _, err := consumer.Get(done, ResourceMAM, nobody); return err }(),
		"Set": consumer.Set(done, ResourceData, nobody, map[string]int{}),
		"Del": consumer.Del(done, ResourceReferenceDesignation, nobody, nil),
		"NewDataSetWriterID": func() error {
			_, _, err := consumer.NewDataSetWriterID(done, ResourceData, nobody)
			return err
		}(),
	} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a done context: %v", name, err)
		}
	}
	if ms, err := timeoutMs(context.Background()); err != nil || ms != 0 {
		t.Fatalf("no deadline: %d %v", ms, err)
	}
	far, cancel := context.WithDeadline(context.Background(), time.Now().Add(2000*time.Hour))
	defer cancel()
	if ms, err := timeoutMs(far); err != nil || ms != maxTimeoutMs {
		t.Fatalf("far deadline: %d %v", ms, err)
	}
}

// Subscriptions: delivery with the topic parts, a panic in the callback is
// recovered (the next message arrives), blocking calls inside are refused,
// Close inside and outside of the callback (REQ-RES-SUBLIST-001).
func TestConsumerSubscribe(t *testing.T) {
	b := startBroker(t)
	_, consumer, speed := consumerPair(t, b)
	logged := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(logged)
	defer log.SetOutput(prev)

	type event struct {
		r       Received
		blocked error
	}
	events := make(chan event, 16)
	first := true
	var mu sync.Mutex
	// ModeSource: Oi4v2/OTConnector/<App>/Pub/Data/<Source>, no Filter
	sub, err := consumer.Subscribe(ResourceData, Address{ServiceType: OTConnector, App: providerIL, Source: pumpIL}, func(r Received) {
		mu.Lock()
		panicNow := first
		first = false
		mu.Unlock()
		if panicNow {
			panic("boom")
		}
		_, blocked := consumer.Get(context.Background(), ResourceMAM, Address{ServiceType: OTConnector, App: providerIL})
		events <- event{r, blocked}
	})
	if err != nil {
		t.Fatal(err)
	}
	// published on change (ModeSource); the subscription becomes active
	// asynchronously: update until a message arrives (the first panics)
	v := 1.0
	eventually(t, "a delivered message", func() bool {
		if err := speed.Update(map[string]any{"Setpoint": v, "Running": true}); err != nil {
			t.Fatal(err)
		}
		v++
		return len(events) > 0
	})
	ev := <-events
	if ev.r.Resource != ResourceData || ev.r.Source != pumpIL || ev.r.Filter != "" || ev.r.ServiceType != OTConnector || ev.r.App != providerIL ||
		!strings.Contains(string(ev.r.Payload), `"Setpoint"`) {
		t.Fatalf("received %+v (%s)", ev.r, ev.r.Payload)
	}
	if !errors.Is(ev.blocked, ErrInvalidState) {
		t.Fatalf("Get inside the callback: %v", ev.blocked)
	}
	if !strings.Contains(logged.String(), "oec: panic in a subscription callback (Data): boom") {
		t.Fatalf("log %q", logged.String())
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	for len(events) > 0 {
		<-events
	}
	if err := speed.Update(map[string]any{"Setpoint": 2500.0, "Running": true}); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		t.Fatalf("after Close: %+v", ev.r)
	case <-time.After(300 * time.Millisecond):
	}

	// Close from inside its own callback
	ready := make(chan *Subscription, 1)
	closed := make(chan error, 1)
	var once sync.Once
	self, err := consumer.Subscribe(ResourceData, Address{}, func(Received) {
		once.Do(func() { closed <- (<-ready).Close() })
	})
	if err != nil {
		t.Fatal(err)
	}
	ready <- self
	eventually(t, "the second subscription delivers", func() bool {
		_ = speed.Update(map[string]any{"Setpoint": float64(time.Now().UnixNano()%3000) + 0.5, "Running": true})
		return len(closed) > 0
	})
	if err := <-closed; err != nil {
		t.Fatalf("Close inside the callback: %v", err)
	}

	if _, err := consumer.Subscribe(ResourceData, Address{}, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("nil callback: %v", err)
	}
	// a subscription outlives its app: Close after App.Close is fine
	late, err := consumer.Subscribe(ResourceHealth, Address{}, func(Received) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := late.Close(); err != nil {
		t.Fatalf("Close after App.Close: %v", err)
	}
	if _, err := consumer.Subscribe(ResourceData, Address{}, func(Received) {}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Subscribe after Close: %v", err)
	}
	if _, err := consumer.Get(context.Background(), ResourceMAM, Address{ServiceType: OTConnector, App: providerIL}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Get after Close: %v", err)
	}
}

// lockedBuffer is a log output written by the SDK thread.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestResourceNames(t *testing.T) {
	for r, want := range map[Resource]string{ResourceMAM: "MAM", ResourceData: "Data",
		ResourceReferenceDesignation: "ReferenceDesignation", Resource(42): "Resource(42)"} {
		if r.String() != want {
			t.Errorf("%d: %s", r, r)
		}
	}
	for _, st := range []ServiceType{Registry, OTConnector, ITConnector} {
		if parseServiceType(st.String()) != st {
			t.Errorf("parse %s", st)
		}
	}
	if parseServiceType("Other") != AnyServiceType {
		t.Error("unknown ServiceType")
	}
}
