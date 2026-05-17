package v8go

// #include "heap_limit.h"
import "C"
import "unsafe"

// NearHeapLimitCallback is called when the V8 heap approaches the configured
// maximum size. The callback receives the current and initial heap limits, and
// must return a new limit. If the returned value equals the current limit, V8
// will crash with an out-of-memory error. Return a larger value to grant the
// isolate more headroom (e.g., current_limit * 2).
type NearHeapLimitCallback func(currentHeapLimit, initialHeapLimit uint64) uint64

// WithoutDefaultHeapLimitCallback disables the built-in NearHeapLimitCallback
// that calls TerminateExecution. Use this when you want to install a custom
// callback with AddNearHeapLimitCallback instead.
func WithoutDefaultHeapLimitCallback() IsolateOption {
	return func(config *isolateConfig) {
		config.disableDefaultHeapLimitCB = true
	}
}

// AddNearHeapLimitCallback installs a Go callback that V8 invokes when the
// heap approaches the configured maximum. Only one callback can be active at
// a time — calling this again replaces the previous callback. This replaces
// the default built-in callback that calls TerminateExecution.
func (i *Isolate) AddNearHeapLimitCallback(cb NearHeapLimitCallback) {
	i.nearHeapLimitCB = cb
	C.IsolateAddCustomNearHeapLimitCallback(i.ptr)
}

// RemoveNearHeapLimitCallback removes a previously installed callback and
// restores the heap limit to the given value. Pass 0 to keep the current limit.
func (i *Isolate) RemoveNearHeapLimitCallback(heapLimit uint64) {
	C.IsolateRemoveCustomNearHeapLimitCallback(i.ptr, C.size_t(heapLimit))
	i.nearHeapLimitCB = nil
}

//export goNearHeapLimitCallback
func goNearHeapLimitCallback(isoPtr C.uintptr_t, currentLimit, initialLimit C.size_t) C.size_t {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil || iso.nearHeapLimitCB == nil {
		return currentLimit * 2
	}
	return C.size_t(iso.nearHeapLimitCB(uint64(currentLimit), uint64(initialLimit)))
}

// Ensure unsafe is used for correct cgo parameter passing.
var _ = unsafe.Pointer(nil)
