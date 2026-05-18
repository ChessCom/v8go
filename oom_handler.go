package v8go

// #include "oom_handler.h"
import "C"

// OOMErrorCallback is called when V8 encounters an out-of-memory error.
// The location string identifies where in V8 the OOM occurred. isHeap is
// true when the OOM is a heap (JavaScript) allocation failure vs. a
// process-level allocation failure.
//
// The callback runs on the V8 thread mid-allocation — it must not allocate
// Go memory or call back into V8. Typical use: log the event and mark the
// isolate for disposal.
type OOMErrorCallback func(location string, isHeap bool)

// SetOOMErrorHandler installs a Go callback that V8 invokes on
// out-of-memory. Only one handler can be active at a time — calling
// this replaces the previous one. Pass nil to clear the handler and
// restore V8's default abort-on-OOM behavior.
func (i *Isolate) SetOOMErrorHandler(cb OOMErrorCallback) {
	i.oomErrorCB = cb
	if cb != nil {
		C.IsolateSetOOMErrorHandler(i.ptr)
	} else {
		C.IsolateClearOOMErrorHandler(i.ptr)
	}
}

//export goOOMErrorCallback
func goOOMErrorCallback(isoPtr C.uintptr_t, location *C.char, isHeap C.int) {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil || iso.oomErrorCB == nil {
		return
	}
	iso.oomErrorCB(C.GoString(location), isHeap != 0)
}
