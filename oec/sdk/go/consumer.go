package oec

/*
#include <stdint.h>
#include <stdlib.h>
#include "oec.h"

oec_received_fn go_received_fn(void);
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

// Resource is the resource of a topic (REQ-TOP-012); the values are those
// of the C ABI (OEC_RES_*).
type Resource int32

// The resources.
const (
	ResourceMAM                  Resource = C.OEC_RES_MAM
	ResourceHealth               Resource = C.OEC_RES_HEALTH
	ResourceConfig               Resource = C.OEC_RES_CONFIG
	ResourceData                 Resource = C.OEC_RES_DATA
	ResourceMetadata             Resource = C.OEC_RES_METADATA
	ResourceEvent                Resource = C.OEC_RES_EVENT
	ResourceProfile              Resource = C.OEC_RES_PROFILE
	ResourcePublicationList      Resource = C.OEC_RES_PUBLICATION_LIST
	ResourceSubscriptionList     Resource = C.OEC_RES_SUBSCRIPTION_LIST
	ResourceReferenceDesignation Resource = C.OEC_RES_REFERENCE_DESIGNATION
)

var resourceNames = [...]string{"MAM", "Health", "Config", "Data", "Metadata", "Event",
	"Profile", "PublicationList", "SubscriptionList", "ReferenceDesignation"}

// String is the name of the resource on the topic.
func (r Resource) String() string {
	if r >= 0 && int(r) < len(resourceNames) {
		return resourceNames[r]
	}
	return fmt.Sprintf("Resource(%d)", int32(r))
}

// parseServiceType is the ServiceType of its name; AnyServiceType if
// unknown.
func parseServiceType(name string) ServiceType {
	for i, n := range serviceTypeNames[1:] {
		if n == name {
			return ServiceType(i + 1)
		}
	}
	return AnyServiceType
}

// Address is the addressed application and asset:
// Oi4v2/<ServiceType>/<App>/<Method>/<Resource>[/<Source>[/<Filter>]].
// App and Source are Identification Links, unencoded (REQ-IL-005); the
// encoding on the topic is the SDK's (REQ-IL-002).
//
// For Subscribe, a zero part (AnyServiceType, "") is the wildcard +
// (REQ-IL-021). A request (Get, Set, Del, NewDataSetWriterID) goes to one
// application and needs ServiceType and App; Source "" addresses the
// application itself.
type Address struct {
	ServiceType ServiceType
	App         string
	Source      string
	// Filter is the decoded value: a Tag (Data, Metadata), a
	// Context.Name (Config), <EventCategory>/<EventLevel> (Event) or
	// <ResourceType>[/<Tag>] (PublicationList, SubscriptionList).
	Filter string
}

// Received is one received DataSetMessage with the parts of its topic.
type Received struct {
	Resource    Resource
	ServiceType ServiceType
	// App is the AppId of the topic, unencoded.
	App string
	// Source and Filter of the topic; "" where the topic has none.
	Source          string
	Filter          string
	DataSetWriterID uint16
	// Payload of the DataSetMessage as JSON.
	Payload json.RawMessage
}

// recvTarget is the user data of an oec_received_fn: the callback of a
// subscription or the collector of a Get.
type recvTarget struct {
	what string // for the panic log
	mu   sync.RWMutex
	fn   func(Received)
}

func (t *recvTarget) handler() func(Received) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.fn
}

func (t *recvTarget) disable() {
	t.mu.Lock()
	t.fn = nil
	t.mu.Unlock()
}

//export goReceived
func goReceived(ud unsafe.Pointer, msg *C.oec_received) {
	t := cgo.Handle(uintptr(ud)).Value().(*recvTarget)
	fn := t.handler()
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("oec: panic in a %s: %v", t.what, r)
		}
	}()
	fn(received(msg))
}

// received copies msg (borrowed for the callback) to Go.
func received(msg *C.oec_received) Received {
	opt := func(p *C.char) string {
		if p == nil {
			return ""
		}
		return C.GoString(p)
	}
	return Received{
		Resource:        Resource(msg.resource),
		ServiceType:     parseServiceType(C.GoString(msg.service_type)),
		App:             C.GoString(msg.app),
		Source:          opt(msg.source),
		Filter:          opt(msg.filter),
		DataSetWriterID: uint16(msg.data_set_writer_id),
		Payload:         json.RawMessage(C.GoString(msg.payload_json)),
	}
}

// cAddress holds the C strings of an Address for the duration of a call.
type cAddress struct {
	st                  C.oec_service_type
	app, source, filter *C.char
}

func newCAddress(at Address) (*cAddress, error) {
	if err := noNUL("address", at.App, at.Source, at.Filter); err != nil {
		return nil, err
	}
	return &cAddress{st: at.ServiceType.c(), app: optString(at.App),
		source: optString(at.Source), filter: optString(at.Filter)}, nil
}

func (c *cAddress) free() {
	free(c.app)
	free(c.source)
	free(c.filter)
}

// Subscription is a subscription of an app; Close ends it.
type Subscription struct {
	once sync.Once
	c    *C.oec_subscription
	t    *recvTarget
}

// Subscribe subscribes to the publications of r at at; zero parts of at
// are wildcards. The subscription is listed in the SubscriptionList
// (REQ-RES-SUBLIST-001). fn runs on the thread of the SDK for every
// DataSetMessage, also for the answers to the Get of other applications;
// it must return soon, must not block, and must not call Get, Set, Del,
// NewDataSetWriterID or Close of the app (ErrInvalidState). It may call
// Close of its subscription. A panic in fn is logged and the message
// dropped.
func (a *App) Subscribe(r Resource, at Address, fn func(Received)) (*Subscription, error) {
	if fn == nil {
		return nil, &Error{Status: StatusInvalidArgument, Message: "Subscribe: fn is nil"}
	}
	ca, err := newCAddress(at)
	if err != nil {
		return nil, err
	}
	defer ca.free()
	t := &recvTarget{what: "subscription callback (" + r.String() + ")", fn: fn}
	h := cgo.NewHandle(t)
	s := &Subscription{t: t}
	err = a.do(func(c *C.oec_app) error {
		return lastError(C.oec_subscribe(c, C.oec_resource(r), ca.st, ca.app, ca.source,
			ca.filter, C.go_received_fn(), userData(h), &s.c))
	})
	if err != nil {
		h.Delete()
		return nil, err
	}
	// The callback may still run once after Close (ADR-0021 §6): the
	// handle lives until the app is released.
	a.hmu.Lock()
	a.handles = append(a.handles, h)
	a.hmu.Unlock()
	return s, nil
}

// Close ends the subscription and removes it from the SubscriptionList.
// Idempotent; also after the app's Close and inside its own callback.
// After Close the callback is not called again.
func (s *Subscription) Close() error {
	s.once.Do(func() {
		s.t.disable()
		C.oec_subscription_free(s.c)
		s.c = nil
	})
	return nil
}

// maxTimeoutMs is the longest timeout of a request in milliseconds.
const maxTimeoutMs = math.MaxUint32

// timeoutMs is the timeout of a request for ctx: 0 (the SDK default of
// 10 s) without a deadline, else the time left in whole milliseconds,
// at least 1; the error of a done ctx.
func timeoutMs(ctx context.Context) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, nil
	}
	left := time.Until(deadline)
	if left <= 0 {
		return 0, context.DeadlineExceeded
	}
	ms := (left + time.Millisecond - 1) / time.Millisecond
	if ms > maxTimeoutMs {
		return maxTimeoutMs, nil
	}
	return uint32(ms), nil
}

// request runs a blocking request with the timeout of ctx: a timeout of
// the SDK with the deadline of ctx is also context.DeadlineExceeded (by
// construction, even if the timer of the SDK ends a moment before the
// deadline of ctx).
func (a *App) request(ctx context.Context, at Address, f func(*C.oec_app, *cAddress, C.uint32_t) C.oec_status) error {
	ms, err := timeoutMs(ctx)
	if err != nil {
		return err
	}
	ca, err := newCAddress(at)
	if err != nil {
		return err
	}
	defer ca.free()
	err = a.do(func(c *C.oec_app) error { return lastError(f(c, ca, C.uint32_t(ms))) })
	if ms != 0 && ms != maxTimeoutMs {
		if e, ok := err.(*Error); ok && e.Status == StatusTimeout {
			return fmt.Errorf("%w: %w", err, context.DeadlineExceeded)
		}
	}
	return err
}

// payload is v as JSON for the C ABI; NULL for nil.
func payload(v any) (*C.char, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, &Error{Status: StatusInvalidArgument, Message: "payload: " + err.Error()}
	}
	return optBytes(b), nil
}

// Get sends a Get of r to the application of at and returns every
// answer, all pages in order (REQ-MSG-015, REQ-PAG-002); an empty list is
// a valid answer (REQ-CMP-041). The deadline of ctx is the timeout (no
// deadline: the SDK default of 10 s); a context cancelled without a
// deadline does not end a request in flight.
//
// Errors: ErrStatus with the StatusCode of a Status event Bad of the
// responder (errors.Is(err, BadNotFound) for an unknown asset,
// REQ-CMP-047), ErrTimeout, ErrNotConnected, ErrClosed, ErrInvalidArgument
// for an address without ServiceType or App or a request the guideline
// does not allow (REQ-TOP-013), ErrInvalidState inside a callback.
func (a *App) Get(ctx context.Context, r Resource, at Address) ([]Received, error) {
	got := []Received{}
	t := &recvTarget{what: "Get", fn: func(m Received) { got = append(got, m) }}
	h := cgo.NewHandle(t)
	defer h.Delete()
	err := a.request(ctx, at, func(c *C.oec_app, ca *cAddress, ms C.uint32_t) C.oec_status {
		return C.oec_get(c, C.oec_resource(r), ca.st, ca.app, ca.source, ca.filter, ms,
			C.go_received_fn(), userData(h))
	})
	if err != nil {
		return nil, err
	}
	return got, nil
}

// Set sends a Set of r with payload (JSON-encoded; json.RawMessage as is)
// to the application of at and waits for its Status event: nil for Good,
// an error with the StatusCode for Bad (errors.Is(err, BadOutOfRange),
// REQ-RES-EVENT-005). Timeout and errors as Get.
func (a *App) Set(ctx context.Context, r Resource, at Address, payload any) error {
	return a.write(ctx, r, at, payload, false)
}

// Del sends a Del of r (payload nil = none) like Set (REQ-TOP-013).
func (a *App) Del(ctx context.Context, r Resource, at Address, payload any) error {
	return a.write(ctx, r, at, payload, true)
}

func (a *App) write(ctx context.Context, r Resource, at Address, v any, del bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := payload(v)
	if err != nil {
		return err
	}
	defer free(p)
	return a.request(ctx, at, func(c *C.oec_app, ca *cAddress, ms C.uint32_t) C.oec_status {
		if del {
			return C.oec_del(c, C.oec_resource(r), ca.st, ca.app, ca.source, ca.filter, p, ms, nil)
		}
		return C.oec_set(c, C.oec_resource(r), ca.st, ca.app, ca.source, ca.filter, p, ms, nil)
	})
}

// NewDataSetWriterID calls the Action NewDataSetWriterId of the
// application of at for a new DataSet of r at at.Source ("" = the
// application itself) and returns the id with its validity (REQ-ACT-017).
// at.Filter must be "". Timeout and errors as Get; BadNotFound for an
// unknown Source.
func (a *App) NewDataSetWriterID(ctx context.Context, r Resource, at Address) (uint16, time.Duration, error) {
	if at.Filter != "" {
		return 0, 0, &Error{Status: StatusInvalidArgument, Message: "NewDataSetWriterID: the address has a Filter"}
	}
	var id C.uint16_t
	var ttl C.uint32_t
	err := a.request(ctx, at, func(c *C.oec_app, ca *cAddress, ms C.uint32_t) C.oec_status {
		return C.oec_new_data_set_writer_id(c, C.oec_resource(r), ca.st, ca.app, ca.source, ms, &id, &ttl)
	})
	if err != nil {
		return 0, 0, err
	}
	return uint16(id), time.Duration(ttl) * time.Second, nil
}
