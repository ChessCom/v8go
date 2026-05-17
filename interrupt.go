package v8go

// #include "interrupt.h"
import "C"

// RequestInterrupt asks V8 to terminate execution of JavaScript code as soon
// as possible. Unlike TerminateExecution, this uses the V8 interrupt mechanism
// which checks at safe points during JS execution and terminates cleanly.
// The termination happens at the next interrupt check point within JS code.
// If no JS is running, it fires on the next RunScript call.
func (i *Isolate) RequestInterrupt() {
	C.IsolateRequestInterruptTerminate(i.ptr)
}

// SetIdle tells V8 whether the embedder is idle. When idle, V8 may perform
// speculative optimisation or incremental GC work.
func (i *Isolate) SetIdle(isIdle bool) {
	v := C.int(0)
	if isIdle {
		v = 1
	}
	C.IsolateSetIdle(i.ptr, v)
}
