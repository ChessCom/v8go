package v8go

// #include "gc_callback.h"
import "C"

// GCType identifies the kind of garbage collection that occurred.
type GCType int

const (
	GCTypeScavenge             GCType = 1 << 0
	GCTypeMinorMarkSweep       GCType = 1 << 1
	GCTypeMarkSweepCompact     GCType = 1 << 2
	GCTypeIncrementalMarking   GCType = 1 << 3
	GCTypeProcessWeakCallbacks GCType = 1 << 4
	GCTypeAll                  GCType = GCTypeScavenge | GCTypeMinorMarkSweep |
		GCTypeMarkSweepCompact | GCTypeIncrementalMarking | GCTypeProcessWeakCallbacks
)

// GCCallback is called before (prologue) or after (epilogue) a garbage
// collection cycle. The GCType identifies which type of GC triggered the call.
type GCCallback func(gcType GCType)

// AddGCPrologueCallback registers a callback invoked before each GC cycle.
// Multiple callbacks can be registered; they are called in order.
func (i *Isolate) AddGCPrologueCallback(cb GCCallback) {
	first := len(i.gcPrologueCBs) == 0
	i.gcPrologueCBs = append(i.gcPrologueCBs, cb)
	if first {
		C.IsolateAddGCPrologueCallback(i.ptr)
	}
}

// RemoveGCPrologueCallbacks removes all prologue callbacks.
func (i *Isolate) RemoveGCPrologueCallbacks() {
	C.IsolateRemoveGCPrologueCallback(i.ptr)
	i.gcPrologueCBs = nil
}

// AddGCEpilogueCallback registers a callback invoked after each GC cycle.
// Multiple callbacks can be registered; they are called in order.
func (i *Isolate) AddGCEpilogueCallback(cb GCCallback) {
	first := len(i.gcEpilogueCBs) == 0
	i.gcEpilogueCBs = append(i.gcEpilogueCBs, cb)
	if first {
		C.IsolateAddGCEpilogueCallback(i.ptr)
	}
}

// RemoveGCEpilogueCallbacks removes all epilogue callbacks.
func (i *Isolate) RemoveGCEpilogueCallbacks() {
	C.IsolateRemoveGCEpilogueCallback(i.ptr)
	i.gcEpilogueCBs = nil
}

//export goGCPrologueCallback
func goGCPrologueCallback(isoPtr C.uintptr_t, gcType C.int) {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil {
		return
	}
	t := GCType(gcType)
	for _, cb := range iso.gcPrologueCBs {
		cb(t)
	}
}

//export goGCEpilogueCallback
func goGCEpilogueCallback(isoPtr C.uintptr_t, gcType C.int) {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil {
		return
	}
	t := GCType(gcType)
	for _, cb := range iso.gcEpilogueCBs {
		cb(t)
	}
}
