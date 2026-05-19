package v8go

// #include "arraybuffer.h"
import "C"
import (
	"runtime"
	"sync"
	"unsafe"
)

// SandboxEnabled reports whether V8 was compiled with V8_ENABLE_SANDBOX.
// When the sandbox is active, NewArrayBufferExternal falls back to a copy
// because backing stores must live inside the sandbox address space.
func SandboxEnabled() bool {
	return C.V8SandboxIsEnabled() != 0
}

// NewArrayBuffer creates a new ArrayBuffer in the given context, copying the
// provided Go byte slice into V8's heap. The Go slice can be freed or
// modified after this call without affecting the ArrayBuffer contents.
func NewArrayBuffer(ctx *Context, data []byte) (*Value, error) {
	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	ptr := C.NewArrayBufferFromBytes(ctx.ptr, dataPtr, C.size_t(len(data)))
	return &Value{ptr: ptr, ctx: ctx}, nil
}

// NewArrayBufferAlloc creates a new zero-initialized ArrayBuffer of the given
// size. Use ArrayBufferGetBytes to get a writable slice into the backing store
// for populating data without an extra copy.
func NewArrayBufferAlloc(ctx *Context, byteLength int) (*Value, error) {
	ptr := C.NewArrayBufferAlloc(ctx.ptr, C.size_t(byteLength))
	return &Value{ptr: ptr, ctx: ctx}, nil
}

// NewArrayBufferExternal creates a zero-copy ArrayBuffer backed directly by
// the provided Go byte slice. V8 will read/write this memory without copying.
// The slice MUST remain valid until V8 releases the backing store (which
// happens when the ArrayBuffer is GC'd or the isolate is disposed). The
// implementation pins the slice header via runtime.Pinner to prevent the Go
// GC from moving or collecting the underlying array.
func NewArrayBufferExternal(ctx *Context, data []byte) (*Value, error) {
	if len(data) == 0 {
		return NewArrayBufferAlloc(ctx, 0)
	}

	ref := externalABRegistry.pin(data)
	ptr := C.NewArrayBufferExternal(
		ctx.ptr,
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
		C.int(ref),
	)
	return &Value{ptr: ptr, ctx: ctx}, nil
}

// ArrayBufferGetBytes returns the raw bytes of an ArrayBuffer value.
// The returned slice points directly into V8's backing store — do not
// use it after the ArrayBuffer is garbage-collected.
func (v *Value) ArrayBufferGetBytes() []byte {
	data := C.ArrayBufferGetData(v.ptr)
	length := C.ArrayBufferGetByteLength(v.ptr)
	if data == nil || length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(data), int(length))
}

// ArrayBufferByteLength returns the byte length of an ArrayBuffer value.
func (v *Value) ArrayBufferByteLength() int {
	return int(C.ArrayBufferGetByteLength(v.ptr))
}

// externalArrayBufferRegistry tracks pinned Go slices that back external
// ArrayBuffers. When V8's BackingStore deleter fires, we unpin the slice.
type externalArrayBufferRegistry struct {
	mu      sync.Mutex
	seq     int
	entries map[int]*externalABEntry
}

type externalABEntry struct {
	data   []byte
	pinner runtime.Pinner
}

var externalABRegistry = &externalArrayBufferRegistry{
	entries: make(map[int]*externalABEntry),
}

func (r *externalArrayBufferRegistry) pin(data []byte) int {
	entry := &externalABEntry{data: data}
	entry.pinner.Pin(&data[0])

	r.mu.Lock()
	r.seq++
	ref := r.seq
	r.entries[ref] = entry
	r.mu.Unlock()
	return ref
}

func (r *externalArrayBufferRegistry) release(ref int) {
	r.mu.Lock()
	entry, ok := r.entries[ref]
	if ok {
		delete(r.entries, ref)
	}
	r.mu.Unlock()
	if ok {
		entry.pinner.Unpin()
	}
}

//export goReleaseExternalArrayBuffer
func goReleaseExternalArrayBuffer(ref C.int) {
	externalABRegistry.release(int(ref))
}
