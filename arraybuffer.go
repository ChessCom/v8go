package v8go

// #include "arraybuffer.h"
import "C"
import "unsafe"

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
// size. The backing store is allocated inside V8's sandbox address space.
// Use ArrayBufferGetBytes to get a writable slice into the backing store
// for populating data without an extra copy.
func NewArrayBufferAlloc(ctx *Context, byteLength int) (*Value, error) {
	ptr := C.NewArrayBufferAlloc(ctx.ptr, C.size_t(byteLength))
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
