package oec

/*
#include <stdlib.h>
#include "oec.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/cgo"
	"sync"
)

// ABIVersion is the version of the C ABI this package is written for.
const ABIVersion = 1

var (
	abiOnce sync.Once
	abiErr  error
)

// checkABI compares the ABI version of the linked library with ABIVersion
// (ADR-0027 §3).
func checkABI() error {
	abiOnce.Do(func() {
		if v := uint32(C.oec_abi_version()); v != ABIVersion {
			abiErr = &Error{Status: StatusInternal, Message: fmt.Sprintf(
				"liboec_ffi has ABI version %d, this package needs %d; rebuild lib/ with cmd/oec-buildlib",
				v, ABIVersion)}
		}
	})
	return abiErr
}

// Version is the version of the linked SDK library (the Rust workspace).
func Version() string {
	return C.GoString(C.oec_version())
}

// Product describes the application or an asset; it is its MAM
// (REQ-RES-MAM-001). ManufacturerURI, Model and ProductCode are the
// type-level elements of the Oi4Identifier; an empty other field keeps
// the default of the SDK.
type Product struct {
	ManufacturerURI string
	Model           string
	ProductCode     string

	Manufacturer     string // en-US
	Description      string // en-US
	HardwareRevision string
	SoftwareRevision string
	DeviceRevision   string
	DeviceManual     string
	DeviceClass      string
	GlobalAssetID    string
}

// newProduct builds the C product; the caller holds its OS thread and
// releases the product with oec_product_free.
func (p *Product) newProduct() (*C.oec_product, error) {
	if err := noNUL("product", p.ManufacturerURI, p.Model, p.ProductCode, p.Manufacturer,
		p.Description, p.HardwareRevision, p.SoftwareRevision, p.DeviceRevision, p.DeviceManual,
		p.DeviceClass, p.GlobalAssetID); err != nil {
		return nil, err
	}
	uri, model, code := cString(p.ManufacturerURI), cString(p.Model), cString(p.ProductCode)
	defer free(uri)
	defer free(model)
	defer free(code)
	var out *C.oec_product
	if err := lastError(C.oec_product_new(uri, model, code, &out)); err != nil {
		return nil, err
	}
	for _, f := range [...]struct{ name, value string }{
		{"Manufacturer", p.Manufacturer},
		{"Description", p.Description},
		{"HardwareRevision", p.HardwareRevision},
		{"SoftwareRevision", p.SoftwareRevision},
		{"DeviceRevision", p.DeviceRevision},
		{"DeviceManual", p.DeviceManual},
		{"DeviceClass", p.DeviceClass},
		{"GlobalAssetId", p.GlobalAssetID},
	} {
		if f.value == "" {
			continue
		}
		name, value := cString(f.name), cString(f.value)
		st := C.oec_product_set(out, name, value)
		free(name)
		free(value)
		if err := lastError(st); err != nil {
			C.oec_product_free(out)
			return nil, err
		}
	}
	return out, nil
}

// Broker is an explicit broker instead of the container storage. TLS is
// always on (REQ-MQTT-005).
type Broker struct {
	Host   string
	Port   uint16 // 0 = 8883
	CAFile string // PEM file of the CA

	// Username and Password, or ClientCertFile and ClientKeyFile (PEM);
	// the client certificate wins (REQ-MQTT-010).
	Username       string
	Password       string
	ClientCertFile string
	ClientKeyFile  string
}

// Options of an application for New.
type Options struct {
	Level       Level
	ServiceType ServiceType // not AnyServiceType
	Product     Product

	// ContainerRoot is the root of the container storage contract
	// (REQ-CNT-004, REQ-CNT-009, REQ-CNT-021): broker.json, the CA, the
	// credentials, the hostname. "" = "/" (production). Ignored if Broker
	// is set.
	ContainerRoot string
	// Broker replaces the container storage.
	Broker *Broker

	// IdentificationLink is the AppId instead of the derived Oi4Identifier
	// (checked, REQ-IL-008).
	IdentificationLink string
	// SerialNumber of the application (default: from the container
	// storage).
	SerialNumber string
	// StoreDir is the directory of what the SDK persists
	// (RevisionCounter, DataSetWriterIds, Config values).
	StoreDir string
	// SyslogLevel is the lowest Syslog level that is published; zero =
	// SyslogNotice (REQ-TOP-024).
	SyslogLevel SyslogLevel
}

// App is an OEC application. Create it with New, run it with Run (or
// Start), release it with Close. Its methods may be called from any
// goroutine.
type App struct {
	// life is held shared by every call and exclusively by the release,
	// so no call uses a released handle.
	life sync.RWMutex
	c    *C.oec_app
	this *Asset

	state   sync.Mutex
	started bool
	closing bool

	hmu     sync.Mutex
	handles []cgo.Handle // user data of callbacks, deleted after the release
}

var errClosed = &Error{Status: StatusClosed, Message: "the app is closed"}

// New builds an application: checks the level, the identity and the
// bootstrap (container storage or Broker). Nothing connects before Run or
// Start.
func New(o Options) (*App, error) {
	if err := checkABI(); err != nil {
		return nil, err
	}
	if err := noNUL("options", o.ContainerRoot, o.IdentificationLink, o.SerialNumber,
		o.StoreDir); err != nil {
		return nil, err
	}
	if b := o.Broker; b != nil {
		if err := noNUL("broker", b.Host, b.CAFile, b.Username, b.Password, b.ClientCertFile,
			b.ClientKeyFile); err != nil {
			return nil, err
		}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	product, err := o.Product.newProduct()
	if err != nil {
		return nil, err
	}
	var b *C.oec_builder
	st := C.oec_builder_new(C.oec_level(o.Level), o.ServiceType.c(), product, &b)
	C.oec_product_free(product)
	if err := lastError(st); err != nil {
		return nil, err
	}
	if err := configure(b, &o); err != nil {
		C.oec_builder_free(b)
		return nil, err
	}
	var app *C.oec_app
	if err := lastError(C.oec_builder_build(b, &app)); err != nil {
		return nil, err
	}
	a := &App{c: app}
	a.this = &Asset{app: a, c: C.oec_app_this(app)}
	return a, nil
}

// configure applies the options to the builder; on the caller's thread.
func configure(b *C.oec_builder, o *Options) error {
	str := func(f func(*C.char) C.oec_status, s string) error {
		if s == "" {
			return nil
		}
		cs := cString(s)
		defer free(cs)
		return lastError(f(cs))
	}
	if br := o.Broker; br != nil {
		host, ca := cString(br.Host), optString(br.CAFile)
		st := C.oec_builder_broker(b, host, C.uint16_t(br.Port), ca)
		free(host)
		free(ca)
		if err := lastError(st); err != nil {
			return err
		}
		if br.Username != "" || br.Password != "" {
			u, p := cString(br.Username), cString(br.Password)
			st := C.oec_builder_credentials(b, u, p)
			free(u)
			free(p)
			if err := lastError(st); err != nil {
				return err
			}
		}
		if br.ClientCertFile != "" || br.ClientKeyFile != "" {
			c, k := cString(br.ClientCertFile), cString(br.ClientKeyFile)
			st := C.oec_builder_client_cert(b, c, k)
			free(c)
			free(k)
			if err := lastError(st); err != nil {
				return err
			}
		}
	} else if err := str(func(s *C.char) C.oec_status {
		return C.oec_builder_container_storage(b, s)
	}, o.ContainerRoot); err != nil {
		return err
	}
	if err := str(func(s *C.char) C.oec_status {
		return C.oec_builder_identification_link(b, s)
	}, o.IdentificationLink); err != nil {
		return err
	}
	if err := str(func(s *C.char) C.oec_status {
		return C.oec_builder_serial_number(b, s)
	}, o.SerialNumber); err != nil {
		return err
	}
	if err := str(func(s *C.char) C.oec_status {
		return C.oec_builder_store_dir(b, s)
	}, o.StoreDir); err != nil {
		return err
	}
	if o.SyslogLevel != 0 {
		lvl := C.oec_syslog_level(o.SyslogLevel.Severity())
		if err := lastError(C.oec_builder_syslog_level(b, lvl)); err != nil {
			return err
		}
	}
	return nil
}

// with runs f on the live handle (shared lock); errClosed after Close.
func (a *App) with(f func(*C.oec_app) C.oec_status) error {
	a.life.RLock()
	defer a.life.RUnlock()
	if a.c == nil {
		return errClosed
	}
	return call(func() C.oec_status { return f(a.c) })
}

// do runs f on the live handle (shared lock) with the OS thread locked,
// so f can read the errors of its C calls with lastError; errClosed after
// Close.
func (a *App) do(f func(*C.oec_app) error) error {
	a.life.RLock()
	defer a.life.RUnlock()
	if a.c == nil {
		return errClosed
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return f(a.c)
}

// Start connects in the background on a thread of the SDK: Birth, Will,
// Health heartbeat (REQ-LIFE-001, REQ-LIFE-008, REQ-TOP-017); a lost
// connection is re-established. Once per app. Errors of the first
// connection (TLS material, host) end the run; Run and Close report them.
// The SDK installs no signal handlers: stop the app with Shutdown, Close
// or the context of Run (e.g. signal.NotifyContext).
func (a *App) Start() error {
	a.state.Lock()
	defer a.state.Unlock()
	if a.closing {
		return errClosed
	}
	err := a.with(func(c *C.oec_app) C.oec_status { return C.oec_app_start(c) })
	if err == nil {
		a.started = true
	}
	return err
}

// Run starts the app (see Start) and blocks until ctx is done or the run
// ends. When ctx is done it sends the Close message and disconnects
// (REQ-LIFE-005). It returns nil after a shutdown (ctx, Shutdown or Close)
// and the start error otherwise.
func (a *App) Run(ctx context.Context) error {
	if err := a.Start(); err != nil {
		return err
	}
	ended := make(chan error, 1)
	go func() { ended <- a.wait(C.OEC_WAIT_FOREVER) }()
	select {
	case err := <-ended:
		return err
	case <-ctx.Done():
		a.Shutdown()
		return <-ended
	}
}

// wait waits for the end of the run of Start.
func (a *App) wait(timeoutMs C.uint32_t) error {
	return a.with(func(c *C.oec_app) C.oec_status { return C.oec_app_wait(c, timeoutMs) })
}

// Shutdown sends the Close message and stops the app (REQ-LIFE-005);
// it does not wait. Idempotent; before the run it makes Run return at
// once.
func (a *App) Shutdown() {
	a.life.RLock()
	defer a.life.RUnlock()
	if a.c != nil {
		C.oec_app_shutdown(a.c)
	}
}

// Close shuts the app down, waits for the end of its run and releases it
// with its assets. It returns the start error of the run, if any.
// Idempotent. Not inside a callback of the app (ErrInvalidState): the
// callback runs on the thread Close would wait for.
func (a *App) Close() error {
	a.state.Lock()
	if a.closing {
		a.state.Unlock()
		return nil
	}
	a.closing = true
	started := a.started
	a.state.Unlock()

	a.Shutdown()
	var runErr error
	if started {
		runErr = a.wait(C.OEC_WAIT_FOREVER)
		var e *Error
		if errors.As(runErr, &e) && e.Status == StatusInvalidState {
			a.state.Lock()
			a.closing = false
			a.state.Unlock()
			return runErr
		}
	}
	a.life.Lock()
	defer a.life.Unlock()
	C.oec_app_free(a.c)
	a.c = nil
	a.hmu.Lock()
	for _, h := range a.handles {
		h.Delete()
	}
	a.handles = nil
	a.hmu.Unlock()
	return runErr
}

// State is the connection state; Closed after Close.
func (a *App) State() ConnectionState {
	var s C.oec_connection_state = C.OEC_STATE_CLOSED
	if a.with(func(c *C.oec_app) C.oec_status { return C.oec_app_state(c, &s) }) != nil {
		return Closed
	}
	return ConnectionState(s)
}

// This is the application itself as asset (its Health, Data, Config).
func (a *App) This() *Asset { return a.this }

// Syslog publishes a Syslog event of the application (REQ-TOP-023) if
// level is switched on (default SyslogNotice and more severe,
// REQ-TOP-024); offline it is dropped.
func (a *App) Syslog(level SyslogLevel, message string) error {
	if err := noNUL("message", message); err != nil {
		return err
	}
	if level < SyslogEmergency || level > SyslogDebug {
		return &Error{Status: StatusInvalidArgument, Message: level.String() + " is no Syslog level"}
	}
	m := cString(message)
	defer free(m)
	return a.with(func(c *C.oec_app) C.oec_status {
		return C.oec_app_syslog(c, C.oec_syslog_level(level.Severity()), m)
	})
}

// Asset is the application itself or an asset it represents; owned by its
// App.
type Asset struct {
	app *App
	c   *C.oec_asset
}

// with runs f on the live asset handle.
func (s *Asset) with(f func(*C.oec_asset) C.oec_status) error {
	return s.app.with(func(*C.oec_app) C.oec_status { return f(s.c) })
}

// SetHealth sets the Health and the HealthScore 0 … 100 (NoScore: none);
// a change is published at once (REQ-TOP-017).
func (s *Asset) SetHealth(h Health, score int) error {
	if score < NoScore || score > 100 {
		return &Error{Status: StatusInvalidArgument, Message: fmt.Sprintf("HealthScore %d is outside 0 … 100", score)}
	}
	return s.with(func(c *C.oec_asset) C.oec_status {
		return C.oec_asset_set_health(c, C.oec_health(h), C.int(score))
	})
}
