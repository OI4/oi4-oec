// C trampolines of the Go callbacks (ADR-0027 §5): the exported Go
// functions are declared by _cgo_export.h without const, so they are cast
// to the callback types of oec.h here, outside of any cgo preamble.
#include <stdint.h>
#include "oec.h"
#include "_cgo_export.h"

oec_set_fn go_set_fn(void) { return (oec_set_fn)goSet; }

// go_handle turns a runtime/cgo.Handle into the user data of a callback
// (go vet flags unsafe.Pointer(uintptr(h)) in Go).
void *go_handle(uintptr_t h) { return (void *)h; }
