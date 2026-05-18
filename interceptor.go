package v8go

// #include "interceptor.h"
// #include "value.h"
import "C"
import "unsafe"

// NamedPropertyGetterCallback is invoked when a named property is accessed
// on an object with an interceptor. Return a non-nil *Value to intercept
// the access, or nil to fall through to the object's own properties.
type NamedPropertyGetterCallback func(property string, info *InterceptorCallbackInfo) *Value

// NamedPropertySetterCallback is invoked when a named property is set.
// Return true to indicate the property set was intercepted.
type NamedPropertySetterCallback func(property string, value *Value, info *InterceptorCallbackInfo) bool

// InterceptorCallbackInfo holds context for interceptor callbacks.
type InterceptorCallbackInfo struct {
	ctx *Context
}

// Context returns the context for this interceptor callback.
func (i *InterceptorCallbackInfo) Context() *Context {
	return i.ctx
}

type interceptorCallbacks struct {
	getter NamedPropertyGetterCallback
	setter NamedPropertySetterCallback
}

// SetNamedPropertyHandler installs named property interceptors on the
// ObjectTemplate. When a named property is accessed on objects created
// from this template, the provided callbacks are invoked instead of
// accessing the property directly.
//
// The getter callback receives the property name and should return a
// *Value to intercept, or nil to fall through. The setter callback
// returns true if the set was intercepted.
func (o *ObjectTemplate) SetNamedPropertyHandler(
	getter NamedPropertyGetterCallback,
	setter NamedPropertySetterCallback,
) {
	icb := &interceptorCallbacks{getter: getter, setter: setter}
	cbref := o.iso.registerInterceptor(icb)

	hasGetter := C.int(0)
	hasSetter := C.int(0)
	if getter != nil {
		hasGetter = 1
	}
	if setter != nil {
		hasSetter = 1
	}

	C.ObjectTemplateSetNamedPropertyHandler(
		o.ptr,
		C.int(cbref),
		hasGetter, hasSetter,
		0, 0, 0,
	)
}

//export goNamedPropertyGetterCallback
func goNamedPropertyGetterCallback(isoPtr C.uintptr_t, ctxRef C.int, cbRef C.int, prop *C.char) C.ValuePtr {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil {
		return nil
	}
	icb := iso.getInterceptor(int(cbRef))
	if icb == nil || icb.getter == nil {
		return nil
	}
	ctx := getContext(int(ctxRef))
	if ctx == nil {
		return nil
	}
	info := &InterceptorCallbackInfo{ctx: ctx}
	result := icb.getter(C.GoString(prop), info)
	if result == nil {
		return nil
	}
	return result.ptr
}

//export goNamedPropertySetterCallback
func goNamedPropertySetterCallback(isoPtr C.uintptr_t, ctxRef C.int, cbRef C.int, prop *C.char, valPtr C.ValuePtr) C.int {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil {
		return 0
	}
	icb := iso.getInterceptor(int(cbRef))
	if icb == nil || icb.setter == nil {
		return 0
	}
	ctx := getContext(int(ctxRef))
	if ctx == nil {
		return 0
	}
	val := &Value{ptr: valPtr, ctx: ctx}
	info := &InterceptorCallbackInfo{ctx: ctx}
	if icb.setter(C.GoString(prop), val, info) {
		return 1
	}
	return 0
}

var _ = unsafe.Pointer(nil)
