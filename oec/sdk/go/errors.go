package oec

/*
#include <stdlib.h>
#include "oec.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// Status is the result class of an SDK call (OEC_ERR_* of the C ABI).
type Status int32

// The statuses of the C ABI (ADR-0021 §4).
const (
	StatusOK              Status = C.OEC_OK
	StatusInvalidArgument Status = C.OEC_ERR_INVALID_ARGUMENT
	StatusLevelNotMet     Status = C.OEC_ERR_LEVEL_NOT_MET
	StatusBootstrap       Status = C.OEC_ERR_BOOTSTRAP
	StatusIO              Status = C.OEC_ERR_IO
	StatusTimeout         Status = C.OEC_ERR_TIMEOUT
	StatusInvalidState    Status = C.OEC_ERR_INVALID_STATE
	StatusClosed          Status = C.OEC_ERR_CLOSED
	StatusNotConnected    Status = C.OEC_ERR_NOT_CONNECTED
	StatusStatus          Status = C.OEC_ERR_STATUS
	StatusPanic           Status = C.OEC_ERR_PANIC
	StatusInternal        Status = C.OEC_ERR_INTERNAL
)

var statusNames = map[Status]string{
	StatusOK:              "ok",
	StatusInvalidArgument: "invalid argument",
	StatusLevelNotMet:     "level not met",
	StatusBootstrap:       "bootstrap",
	StatusIO:              "I/O",
	StatusTimeout:         "timeout",
	StatusInvalidState:    "invalid state",
	StatusClosed:          "closed",
	StatusNotConnected:    "not connected",
	StatusStatus:          "status Bad",
	StatusPanic:           "panic in the SDK",
	StatusInternal:        "internal",
}

func (s Status) String() string {
	if n, ok := statusNames[s]; ok {
		return n
	}
	return fmt.Sprintf("status %d", int32(s))
}

// Error is a failed SDK call: its status, the message of the SDK and, for
// StatusStatus, the Bad StatusCode of the responder (REQ-CMP-047).
//
// Compare with errors.Is against the sentinels (ErrNotConnected, …).
type Error struct {
	Status     Status
	Message    string
	StatusCode StatusCode
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "oec: " + e.Status.String()
	}
	if e.Status == StatusStatus {
		return fmt.Sprintf("oec: %s (%s): %s", e.Status, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("oec: %s: %s", e.Status, e.Message)
}

// Is reports whether target is the sentinel of e's status.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Message == "" && t.Status == e.Status
}

// Sentinels for errors.Is, one per status.
var (
	ErrInvalidArgument = &Error{Status: StatusInvalidArgument}
	ErrLevelNotMet     = &Error{Status: StatusLevelNotMet}
	ErrBootstrap       = &Error{Status: StatusBootstrap}
	ErrIO              = &Error{Status: StatusIO}
	ErrTimeout         = &Error{Status: StatusTimeout}
	ErrInvalidState    = &Error{Status: StatusInvalidState}
	ErrClosed          = &Error{Status: StatusClosed}
	ErrNotConnected    = &Error{Status: StatusNotConnected}
	ErrStatus          = &Error{Status: StatusStatus}
	ErrPanic           = &Error{Status: StatusPanic}
	ErrInternal        = &Error{Status: StatusInternal}
)

// call runs one C ABI call and reads its error on the same OS thread:
// oec_last_error and oec_last_status_code are thread-local, and a
// goroutine may change its thread between two cgo calls.
func call(f func() C.oec_status) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return lastError(f())
}

// lastError is the error of status st; on the thread of the failed call.
func lastError(st C.oec_status) error {
	if st == C.OEC_OK {
		return nil
	}
	e := &Error{Status: Status(st), Message: C.GoString(C.oec_last_error())}
	if e.Message == "" {
		e.Message = e.Status.String()
	}
	if e.Status == StatusStatus {
		e.StatusCode = StatusCode(C.oec_last_status_code())
	}
	return e
}

// noNUL refuses a string with U+0000: C would see it cut off there.
func noNUL(what string, values ...string) error {
	for _, v := range values {
		if strings.IndexByte(v, 0) >= 0 {
			return &Error{Status: StatusInvalidArgument, Message: what + " contains U+0000"}
		}
	}
	return nil
}

// cString is a C copy of s for the duration of a call; release it with
// free. Check the string with noNUL first.
func cString(s string) *C.char {
	return C.CString(s)
}

// optString is NULL for "", else as cString.
func optString(s string) *C.char {
	if s == "" {
		return nil
	}
	return C.CString(s)
}

func free(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}
