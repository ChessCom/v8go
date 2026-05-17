package v8go

// #include "promise_reject.h"
import "C"

// PromiseRejectEvent mirrors V8's PromiseRejectEvent enum.
type PromiseRejectEvent int

const (
	PromiseRejectWithNoHandler       PromiseRejectEvent = 0
	PromiseHandlerAddedAfterReject   PromiseRejectEvent = 1
	PromiseRejectAfterResolved       PromiseRejectEvent = 2
	PromiseResolveAfterResolved      PromiseRejectEvent = 3
)

// PromiseRejectMessage is passed to the PromiseRejectCallback with details
// about the rejection event.
type PromiseRejectMessage struct {
	Event   PromiseRejectEvent
	Promise *Value
	Value   *Value
}

// PromiseRejectCallback is called by V8 whenever a promise is rejected.
type PromiseRejectCallback func(msg PromiseRejectMessage)

// SetPromiseRejectCallback installs a callback that V8 invokes whenever a
// promise rejection occurs. Only one callback can be active at a time.
func (i *Isolate) SetPromiseRejectCallback(cb PromiseRejectCallback) {
	i.promiseRejectCB = cb
	C.IsolateSetPromiseRejectCallback(i.ptr)
}

//export goPromiseRejectCallback
func goPromiseRejectCallback(isoPtr C.uintptr_t, event C.int, promise C.ValuePtr, value C.ValuePtr) {
	iso := lookupIsolate(uintptr(isoPtr))
	if iso == nil || iso.promiseRejectCB == nil {
		return
	}
	msg := PromiseRejectMessage{
		Event:   PromiseRejectEvent(event),
		Promise: &Value{promise, nil},
	}
	if value != nil {
		msg.Value = &Value{value, nil}
	}
	iso.promiseRejectCB(msg)
}
