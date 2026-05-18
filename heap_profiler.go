package v8go

// #include <stdlib.h>
// #include "heap_profiler.h"
import "C"
import "unsafe"

// TakeHeapSnapshot takes a V8 heap snapshot and returns it as a JSON
// byte slice. The snapshot is a complete graph of all reachable heap
// objects and can be loaded in Chrome DevTools (Memory tab).
//
// This is a blocking operation that pauses JS execution while the
// snapshot is being captured. The snapshot is serialized to JSON and
// copied to Go memory before returning — the V8-side snapshot object
// is deleted immediately.
func (i *Isolate) TakeHeapSnapshot() ([]byte, error) {
	data := C.IsolateTakeHeapSnapshot(i.ptr)
	if data.data == nil {
		return nil, nil
	}
	defer C.HeapSnapshotDataFree(data)
	result := C.GoBytes(unsafe.Pointer(data.data), C.int(data.length))
	return result, nil
}
