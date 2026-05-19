package v8go

// #include <stdlib.h>
// #include "function_template.h"
// #include "fast_api.h"
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// CType represents V8 Fast API argument/return types.
type CType int

const (
	CTypeVoid            CType = C.kCTypeVoid
	CTypeBool            CType = C.kCTypeBool
	CTypeUint8           CType = C.kCTypeUint8
	CTypeInt32           CType = C.kCTypeInt32
	CTypeUint32          CType = C.kCTypeUint32
	CTypeInt64           CType = C.kCTypeInt64
	CTypeUint64          CType = C.kCTypeUint64
	CTypeFloat32         CType = C.kCTypeFloat32
	CTypeFloat64         CType = C.kCTypeFloat64
	CTypePointer         CType = C.kCTypePointer
	CTypeV8Value         CType = C.kCTypeV8Value
	CTypeOneByteString   CType = C.kCTypeSeqOneByteString
)

// FastCallDescriptor describes the signature of a V8 Fast API callback.
// FastFn is a C-linkage function pointer (unsafe.Pointer to a C function).
// ReturnType describes the return type. ArgTypes describes the parameter
// types (the first entry must be CTypeV8Value for the receiver).
type FastCallDescriptor struct {
	FastFn     unsafe.Pointer
	ReturnType CType
	ArgTypes   []CType
}

// FunctionCallback is a callback that is executed in Go when a function is executed in JS.
type FunctionCallback func(info *FunctionCallbackInfo) *Value

// FunctionCallbackWithError is a callback that is executed in Go when
// a function is executed in JS. If a ValueError is returned, its
// value will be thrown as an exception in V8, otherwise Error() is
// invoked, and the string is thrown.
type FunctionCallbackWithError func(info *FunctionCallbackInfo) (*Value, error)

// FunctionCallbackInfo is the argument that is passed to a FunctionCallback.
type FunctionCallbackInfo struct {
	ctx  *Context
	args []*Value
	this *Object
}

// A ValueError can be returned from a FunctionCallbackWithError, and
// its value will be thrown as an exception in V8.
type ValueError interface {
	error
	Valuer
}

// Context is the current context that the callback is being executed in.
func (i *FunctionCallbackInfo) Context() *Context {
	return i.ctx
}

// This returns the receiver object "this".
func (i *FunctionCallbackInfo) This() *Object {
	return i.this
}

// Args returns a slice of the value arguments that are passed to the JS function.
func (i *FunctionCallbackInfo) Args() []*Value {
	return i.args
}

func (i *FunctionCallbackInfo) Release() {
	for _, arg := range i.args {
		arg.Release()
	}
	i.this.Release()
}

// FunctionTemplate is used to create functions at runtime.
// There can only be one function created from a FunctionTemplate in a context.
// The lifetime of the created function is equal to the lifetime of the context.
//
// A FunctionTemplate can be used to create "constructors", and add methods to
// the "class". [FunctionTemplate.PrototypeTemplate] can be used to add normal
// methods on the class, and [FunctionTemplate.InstanceTemplate] can be used to
// add fields automatically to new instances of a class.
//
// V8 API Docs: https://v8.github.io/api/head/classv8_1_1FunctionTemplate.html
type FunctionTemplate struct {
	*template
}

// NewFunctionTemplate creates a FunctionTemplate for a given
// callback. Prefer using NewFunctionTemplateWithError.
func NewFunctionTemplate(iso *Isolate, callback FunctionCallback) *FunctionTemplate {
	if callback == nil {
		panic("nil FunctionCallback argument not supported")
	}
	return NewFunctionTemplateWithError(iso, func(info *FunctionCallbackInfo) (*Value, error) {
		return callback(info), nil
	})
}

// NewFunctionTemplateWithError creates a FunctionTemplate for a given
// callback. If the callback returns an error, it will be thrown as a
// JS error.
func NewFunctionTemplateWithError(
	iso *Isolate,
	callback FunctionCallbackWithError,
) *FunctionTemplate {
	if iso == nil {
		panic("nil Isolate argument not supported")
	}
	if callback == nil {
		panic("nil FunctionCallback argument not supported")
	}

	cbref := iso.registerCallback(callback)

	tmpl := &template{
		ptr: C.NewFunctionTemplate(iso.ptr, C.int(cbref)),
		iso: iso,
	}
	runtime.SetFinalizer(tmpl, (*template).finalizer)
	return &FunctionTemplate{tmpl}
}

// NewFastFunctionTemplate creates a FunctionTemplate with a V8 Fast API path.
// When TurboFan can prove the argument types match the descriptor at compile
// time, it calls the C function directly — bypassing the generic slow callback,
// CGo overhead, and all argument marshaling. The slow callback is still used
// for unoptimized calls and when types don't match.
//
// The fast function must:
//   - Be a C-linkage function (not a Go function)
//   - Not allocate on the JS heap
//   - Not trigger JS execution
//   - Be registered in the external_references array for snapshot compatibility
//
// The first argument of the fast function is always the receiver
// (v8::Local<v8::Object>).
func NewFastFunctionTemplate(
	iso *Isolate,
	slowCallback FunctionCallbackWithError,
	fast FastCallDescriptor,
) *FunctionTemplate {
	if iso == nil {
		panic("nil Isolate argument not supported")
	}
	if slowCallback == nil {
		panic("nil FunctionCallback argument not supported")
	}
	if fast.FastFn == nil {
		panic("nil FastFn in descriptor not supported")
	}

	cbref := iso.registerCallback(slowCallback)

	argTypes := make([]C.CTypeInfoType, len(fast.ArgTypes))
	for i, t := range fast.ArgTypes {
		argTypes[i] = C.CTypeInfoType(t)
	}
	var argPtr *C.CTypeInfoType
	if len(argTypes) > 0 {
		argPtr = &argTypes[0]
	}
	fnInfo := C.BuildCFunctionInfo(
		C.CTypeInfoType(fast.ReturnType),
		argPtr,
		C.int(len(argTypes)),
	)

	tmpl := &template{
		ptr: C.NewFastFunctionTemplate(iso.ptr, C.int(cbref), fast.FastFn, fnInfo),
		iso: iso,
	}
	runtime.SetFinalizer(tmpl, (*template).finalizer)
	return &FunctionTemplate{tmpl}
}

// GetFunction returns an instance of this function template bound to the given context.
func (tmpl *FunctionTemplate) GetFunction(ctx *Context) *Function {
	rtn := C.FunctionTemplateGetFunction(tmpl.ptr, ctx.ptr)
	runtime.KeepAlive(tmpl)
	val, err := valueResult(ctx, rtn)
	if err != nil {
		panic(err) // TODO: Consider returning the error
	}
	return &Function{val}
}

// InstanceTemplate gets the [ObjectTemplate] that is used for new object
// instances created when this function is used as a constructor.
//
// You can add functions and values to new instance using [ObjectTemplate.Set]
// and [ObjectTemplate.SetSymbol]. Those values will become [own properties] on
// the instance, not the prototype.
//
// Adding a function to an instance template corresponds to the following
// JavaScript:
//
//	class Example() {
//		constructor() {
//			this.foo = function() { /* creates a function on the instance */ }
//		}
//	}
//
// [own properties]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Enumerability_and_ownership_of_properties
func (tmpl *FunctionTemplate) InstanceTemplate() *ObjectTemplate {
	result := &template{
		ptr: C.FunctionTemplateInstanceTemplate(tmpl.ptr),
		iso: tmpl.iso,
	}
	runtime.SetFinalizer(result, (*template).finalizer)
	return &ObjectTemplate{result}
}

// PrototypeTemplate gets the [ObjectTemplate] that is used to create the
// prototype object associated with the function.
//
// You can call [ObjectTemplate.Set] or [ObjectTemplate.SetSymbol], passing a
// [FunctionTemplate] to add a "method" to the class.
//
// Adding a function to a prototype template corresponds normal method on a
// JavaScript "class":
//
//	class Example {
//		foo() { /* this is a method on the prototype */ }
//	}
//
// Or the old-school way
//
//	function Example() {}
//	Example.prototype.foo = function() { }
//
// The function becomes an [own property] on the prototype, not the instance.
//
// [own property]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Enumerability_and_ownership_of_properties
func (tmpl *FunctionTemplate) PrototypeTemplate() *ObjectTemplate {
	result := &template{
		ptr: C.FunctionTemplatePrototypeTemplate(tmpl.ptr),
		iso: tmpl.iso,
	}
	runtime.SetFinalizer(result, (*template).finalizer)
	return &ObjectTemplate{result}
}

func (tmpl *FunctionTemplate) Inherit(base *FunctionTemplate) {
	C.FunctionTemplateInherit(tmpl.ptr, base.ptr)
}

// Note that ideally `thisAndArgs` would be split into two separate arguments, but they were combined
// to workaround an ERROR_COMMITMENT_LIMIT error on windows that was detected in CI.
//
//export goFunctionCallback
func goFunctionCallback(
	ctxref int,
	cbref int,
	thisAndArgs *C.ValuePtr,
	argsCount int,
) (rval C.ValuePtr, rerr C.ValuePtr) {
	ctx := getContext(ctxref)

	this := *thisAndArgs
	info := &FunctionCallbackInfo{
		ctx:  ctx,
		this: &Object{&Value{ptr: this, ctx: ctx}},
		args: make([]*Value, argsCount),
	}

	argv := (*[1 << 30]C.ValuePtr)(unsafe.Pointer(thisAndArgs))[1 : argsCount+1 : argsCount+1]
	for i, v := range argv {
		val := &Value{ptr: v, ctx: ctx}
		info.args[i] = val
	}

	callbackFunc := ctx.iso.getCallback(cbref)
	if callbackFunc == nil {
		// The isolate has no Go closure registered for this callback
		// reference. This commonly happens after restoring a snapshot
		// that was produced with FunctionTemplates: the snapshot
		// preserves the trampoline pointer (via external_references)
		// and the integer ref, but Go-side closures must be
		// re-registered on the consumer side. Return a JS error
		// instead of nil-derefing.
		errv, err := NewValue(ctx.iso, fmt.Sprintf(
			"v8go: callback %d is not registered on this isolate; "+
				"re-export functions after restoring a snapshot", cbref))
		if err != nil {
			return nil, nil
		}
		return nil, errv.ptr
	}
	val, err := callbackFunc(info)
	if err != nil {
		if verr, ok := err.(ValueError); ok {
			return nil, verr.value().ptr
		}
		errv, err := NewValue(ctx.iso, err.Error())
		if err != nil {
			panic(err)
		}
		return nil, errv.ptr
	}
	if val == nil {
		return nil, nil
	}
	return val.ptr, nil
}
