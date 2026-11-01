package oec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var testProduct = Product{
	ManufacturerURI:  "example.com",
	Model:            "Go Test",
	ProductCode:      "GO-TEST-1",
	Manufacturer:     "Example Inc.",
	SoftwareRevision: "1.0.0",
}

// newApp builds a Level 1 Utility on the container storage of b.
func newApp(t *testing.T, b *broker) *App {
	t.Helper()
	app, err := New(Options{
		Level:         Level1,
		ServiceType:   Utility,
		Product:       testProduct,
		ContainerRoot: b.container(t),
		StoreDir:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func TestVersion(t *testing.T) {
	if err := checkABI(); err != nil {
		t.Fatal(err)
	}
	if v := Version(); v == "" || strings.ContainsAny(v, " \n") {
		t.Fatalf("Version() = %q", v)
	}
}

func TestErrors(t *testing.T) {
	err := error(&Error{Status: StatusBootstrap, Message: "no CA", StatusCode: BadNotFound})
	if !errors.Is(err, ErrBootstrap) || errors.Is(err, ErrIO) {
		t.Fatalf("errors.Is: %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "no CA") {
		t.Fatalf("Error() = %q", got)
	}
	var e *Error
	if !errors.As(err, &e) || e.StatusCode != BadNotFound || !e.StatusCode.IsBad() {
		t.Fatalf("errors.As: %#v", e)
	}
	if Good.IsBad() || Good.String() != "Good" || StatusCode(0x80FF0000).String() != "0x80FF0000" {
		t.Fatal("StatusCode")
	}
}

func TestNewRejects(t *testing.T) {
	root := t.TempDir()
	for name, o := range map[string]Options{
		"level 0":      {ServiceType: Utility, Product: testProduct, ContainerRoot: root},
		"service type": {Level: Level1, Product: testProduct, ContainerRoot: root},
		"NUL":          {Level: Level1, ServiceType: Utility, Product: Product{ManufacturerURI: "example.com", Model: "a\x00b", ProductCode: "P"}},
		"syslog":       {Level: Level1, ServiceType: Utility, Product: testProduct, ContainerRoot: root, SyslogLevel: 9},
		"IL":           {Level: Level1, ServiceType: Utility, Product: testProduct, ContainerRoot: root, IdentificationLink: "not an IL"},
	} {
		t.Run(name, func(t *testing.T) {
			app, err := New(o)
			if err == nil {
				app.Close()
				t.Fatal("New succeeded")
			}
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("got %v, want ErrInvalidArgument", err)
			}
		})
	}
	// An empty container storage: no broker.json.
	_, err := New(Options{Level: Level1, ServiceType: Utility, Product: testProduct, ContainerRoot: root})
	if !errors.Is(err, ErrBootstrap) {
		t.Fatalf("empty container storage: got %v, want ErrBootstrap", err)
	}
}

// A start error is reported by Run and by Close (ADR-0021 amendment C-076).
func TestRunStartError(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	write(t, ca, "not a certificate\n")
	app, err := New(Options{
		Level: Level1, ServiceType: Utility, Product: testProduct, StoreDir: dir, SerialNumber: "go-1",
		Broker: &Broker{Host: "localhost", Port: 1, CAFile: ca, Username: "u", Password: "p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background())
	if !errors.Is(err, ErrBootstrap) || !strings.Contains(err.Error(), "ca.pem") {
		t.Fatalf("Run: got %v, want ErrBootstrap naming ca.pem", err)
	}
	if s := app.State(); s != Closed {
		t.Fatalf("State() = %v", s)
	}
	if err := app.Close(); !errors.Is(err, ErrBootstrap) {
		t.Fatalf("Close: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// Birth, Health, Syslog, and on Close the Close message and DISCONNECT
// (REQ-LIFE-001, REQ-LIFE-005, REQ-TOP-017, REQ-TOP-023).
func TestLifecycle(t *testing.T) {
	b := startBroker(t)
	app := newApp(t, b)
	if s := app.State(); s != Closed {
		t.Fatalf("State() before Start = %v", s)
	}
	if err := app.This().SetHealth(Normal, 100); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second Start: %v", err)
	}
	connect, i := b.expect(t, 0, "CONNECT", func(s seen) bool { return s.Seen == "connect" })
	if connect.ClientID != "app-1" || connect.Username == nil || *connect.Username != "user" || connect.Will == nil {
		t.Fatalf("CONNECT %+v", connect)
	}
	b.publish(t, i, "/Pub/MAM/")
	health, i := b.publish(t, i, *connect.Will)
	if !strings.Contains(health.Payload, `"Health":"NORMAL_0"`) {
		t.Fatalf("Health %s", health.Payload)
	}
	eventually(t, "Online", func() bool { return app.State() == Online })

	if err := app.This().SetHealth(MaintenanceRequired, 70); err != nil {
		t.Fatal(err)
	}
	b.expect(t, i, "Health MAINTENANCE_REQUIRED_4", func(s seen) bool {
		return s.Seen == "publish" && s.Topic == *connect.Will &&
			strings.Contains(s.Payload, `"Health":"MAINTENANCE_REQUIRED_4"`) && strings.Contains(s.Payload, `"HealthScore":70`)
	})
	if err := app.Syslog(SyslogError, "go says hi"); err != nil {
		t.Fatal(err)
	}
	b.expect(t, i, "Syslog event", func(s seen) bool {
		return s.Seen == "publish" && strings.Contains(s.Topic, "/Pub/Event/") && strings.Contains(s.Payload, "go says hi")
	})
	if err := app.Syslog(0, "x"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Syslog level 0: %v", err)
	}
	if err := app.This().SetHealth(Normal, 101); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("score 101: %v", err)
	}

	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	closeMsg, j := b.expect(t, i, "Close message", func(s seen) bool {
		return s.Seen == "publish" && s.Topic == *connect.Will && strings.Contains(s.Payload, `"HealthScore":0`)
	})
	var payload struct {
		Messages []struct{ Payload struct{ HealthScore int } }
	}
	if err := json.Unmarshal([]byte(closeMsg.Payload), &payload); err != nil || len(payload.Messages) != 1 {
		t.Fatalf("Close message %s: %v", closeMsg.Payload, err)
	}
	b.expect(t, j, "DISCONNECT", func(s seen) bool { return s.Seen == "disconnect" })

	if err := app.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if s := app.State(); s != Closed {
		t.Fatalf("State() after Close = %v", s)
	}
	for name, err := range map[string]error{
		"SetHealth": app.This().SetHealth(Normal, 100),
		"Syslog":    app.Syslog(SyslogError, "x"),
		"Start":     app.Start(),
		"Run":       app.Run(context.Background()),
	} {
		if !errors.Is(err, ErrClosed) {
			t.Errorf("%s after Close: %v", name, err)
		}
	}
	app.Shutdown() // no effect
}

// Run ends when its context is done: Close message, DISCONNECT, nil.
func TestRunContext(t *testing.T) {
	b := startBroker(t)
	app := newApp(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	ran := make(chan error, 1)
	go func() { ran <- app.Run(ctx) }()
	eventually(t, "Online", func() bool { return app.State() == Online })
	cancel()
	select {
	case err := <-ran:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(wait):
		t.Fatal("Run did not return")
	}
	b.expect(t, 0, "DISCONNECT", func(s seen) bool { return s.Seen == "disconnect" })
	if s := app.State(); s != Closed {
		t.Fatalf("State() = %v", s)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
}

// Shutdown before the run makes Run return at once.
func TestShutdownBeforeRun(t *testing.T) {
	b := startBroker(t)
	app := newApp(t, b)
	app.Shutdown()
	done := make(chan error, 1)
	go func() { done <- app.Run(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(wait):
		t.Fatal("Run did not return")
	}
}

// Close from another goroutine ends Run; calls racing with Close see nil
// or ErrClosed, never a released handle (run with -race).
func TestCloseRaces(t *testing.T) {
	b := startBroker(t)
	app := newApp(t, b)
	ran := make(chan error, 1)
	go func() { ran <- app.Run(context.Background()) }()
	eventually(t, "Online", func() bool { return app.State() == Online })

	var wg sync.WaitGroup
	stop := make(chan struct{})
	errs := make(chan error, 64)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				var err error
				switch n % 3 {
				case 0:
					err = app.This().SetHealth(Normal, n%101)
				case 1:
					err = app.Syslog(SyslogDebug, "race")
				default:
					_ = app.State()
				}
				if err != nil && !errors.Is(err, ErrClosed) && !errors.Is(err, ErrNotConnected) {
					errs <- err
					return
				}
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-ran; err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// An explicit broker with credentials and serial number instead of the
// container storage (REQ-MQTT-009).
func TestExplicitBroker(t *testing.T) {
	b := startBroker(t)
	app, err := New(Options{
		Level: Level1, ServiceType: Utility, Product: testProduct,
		Broker:       &Broker{Host: "localhost", Port: uint16(b.Port), CAFile: b.CA, Username: "go", Password: "secret"},
		SerialNumber: "go-1",
		StoreDir:     t.TempDir(),
		SyslogLevel:  SyslogDebug,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	connect, i := b.expect(t, 0, "CONNECT", func(s seen) bool { return s.Seen == "connect" })
	if connect.Username == nil || *connect.Username != "go" {
		t.Fatalf("CONNECT %+v", connect)
	}
	mam, _ := b.publish(t, i, "/Pub/MAM/")
	if !strings.Contains(mam.Payload, `"PublisherId":"Utility/example.com/Go,20Test/GO-TEST-1/go-1"`) {
		t.Fatalf("MAM %s", mam.Payload)
	}
	eventually(t, "Online", func() bool { return app.State() == Online })
	if err := app.Syslog(SyslogDebug, "debug on"); err != nil {
		t.Fatal(err)
	}
	b.expect(t, i, "Syslog debug", func(s seen) bool {
		return s.Seen == "publish" && strings.Contains(s.Payload, "debug on")
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
