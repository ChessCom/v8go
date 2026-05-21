package v8go

// #include <stdlib.h>
// #include "unbound_script.h"
import "C"
import "unsafe"

type UnboundScript struct {
	ptr C.UnboundScriptPtr
	iso *Isolate
}

// Run will bind the unbound script to the provided context and run it.
// If the context provided does not belong to the same isolate that the script
// was compiled in, Run will panic.
// If an error occurs, it will be of type `JSError`.
func (u *UnboundScript) Run(ctx *Context) (*Value, error) {
	if ctx.Isolate() != u.iso {
		panic("attempted to run unbound script in a context that belongs to a different isolate")
	}
	rtn := C.UnboundScriptRun(ctx.ptr, u.ptr)
	return valueResult(ctx, rtn)
}

// CreateCodeCache serialises the compiled bytecode so it can be fed to
// a future CompileUnboundScript call (on any isolate) to skip parsing.
//
// An RLock on snapshotDeserMu is held for the duration of the CGo call
// so that Isolate.Dispose (which takes a write Lock) cannot tear down
// V8 shared-heap state while CreateCodeCache is reading it. Multiple
// concurrent CreateCodeCache calls on different isolates remain parallel.
//
// Returns nil without calling into V8 if the isolate has already been
// disposed (ptr set to nil inside Dispose's write lock).
func (u *UnboundScript) CreateCodeCache() *CompilerCachedData {
	if u == nil || u.iso == nil {
		return nil
	}

	snapshotDeserMu.RLock()
	defer snapshotDeserMu.RUnlock()

	if u.iso.ptr == nil {
		return nil
	}

	rtn := C.UnboundScriptCreateCodeCache(u.iso.ptr, u.ptr)
	if rtn == nil {
		return nil
	}

	cachedData := &CompilerCachedData{
		Bytes:    []byte(C.GoBytes(unsafe.Pointer(rtn.data), rtn.length)),
		Rejected: int(rtn.rejected) == 1,
	}
	C.ScriptCompilerCachedDataDelete(rtn)
	return cachedData
}
