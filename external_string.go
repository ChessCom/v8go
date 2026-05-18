package v8go

// #include "external_string.h"
import "C"
import "unsafe"

// NewExternalOneByteString creates a V8 string that points directly at the
// provided Go byte slice without copying. V8 will read from this memory for
// the lifetime of the string.
//
// IMPORTANT: The caller MUST keep the backing slice alive and immutable for
// the entire lifetime of the returned Value. If the slice is garbage-collected
// or modified, V8 will read corrupted data or crash.
//
// This is the zero-copy fast path for string data that is known to be
// Latin-1 / ASCII. For UTF-8 data that may contain multi-byte sequences,
// use the standard NewValue(iso, str) path instead.
func NewExternalOneByteString(ctx *Context, data []byte) (*Value, error) {
	if len(data) == 0 {
		return NewValue(ctx.Isolate(), "")
	}
	ptr := C.NewExternalOneByteString(
		ctx.ptr,
		(*C.char)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
	)
	if ptr == nil {
		return NewValue(ctx.Isolate(), string(data))
	}
	return &Value{ptr: ptr, ctx: ctx}, nil
}
