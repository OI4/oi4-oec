package oec

/*
#include <stdint.h>
#include "oec.h"

oec_set_fn go_set_fn(void);
void *go_handle(uintptr_t h);
*/
import "C"

import (
	"errors"
	"log"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// setTarget is the user data of an oec_set_fn: the current handler of a
// DataSet or Config. A further OnSet swaps the handler and keeps the
// handle, so the C side is told only once.
type setTarget struct {
	what string // for the panic log
	mu   sync.RWMutex
	fn   func(json string) error
}

func (t *setTarget) handler() func(string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.fn
}

// onSet hands t as handler to the C ABI through set (on the caller's
// locked thread). The handle is kept until the app is released: the C ABI
// may call the handler until oec_app_free (ADR-0021 §6).
func (a *App) onSet(t *setTarget, set func(C.oec_set_fn, unsafe.Pointer) C.oec_status) error {
	h := cgo.NewHandle(t)
	if err := lastError(set(C.go_set_fn(), userData(h))); err != nil {
		h.Delete()
		return err
	}
	a.hmu.Lock()
	a.handles = append(a.handles, h)
	a.hmu.Unlock()
	return nil
}

// replace swaps the handler of an installed target.
func (t *setTarget) replace(fn func(string) error) {
	t.mu.Lock()
	t.fn = fn
	t.mu.Unlock()
}

// userData is h as user data of a callback.
func userData(h cgo.Handle) unsafe.Pointer { return C.go_handle(C.uintptr_t(h)) }

//export goSet
func goSet(ud unsafe.Pointer, json *C.char) (code C.oec_status_code) {
	t := cgo.Handle(uintptr(ud)).Value().(*setTarget)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("oec: panic in the OnSet handler of %s: %v", t.what, r)
			code = C.oec_status_code(BadInternalError)
		}
	}()
	return C.oec_status_code(statusCodeOf(t.handler()(C.GoString(json))))
}

// statusCodeOf is the answer to a Set for the result of a handler: nil
// accepts (Good); a StatusCode or an *Error with a StatusCode that is not
// Good rejects with that code; any other error rejects with Bad.
func statusCodeOf(err error) StatusCode {
	if err == nil {
		return Good
	}
	var sc StatusCode
	if errors.As(err, &sc) && sc>>30 != 0 {
		return sc
	}
	var e *Error
	if errors.As(err, &e) && e.StatusCode>>30 != 0 {
		return e.StatusCode
	}
	return Bad
}
