/*
Package v8go provides an API to execute JavaScript.
*/
package v8go

// #include "v8go.h"
// #include <stdlib.h>
import "C"
import (
	"strings"
	"sync"
	"unsafe"
)

// Version returns the version of the V8 Engine with the -v8go suffix
func Version() string {
	return C.GoString(C.Version())
}

// SetFlags sets flags for V8. For possible flags: https://github.com/v8/v8/blob/master/src/flags/flag-definitions.h
// Flags are expected to be prefixed with `--`, for example: `--harmony`.
// Flags can be reverted using the `--no` prefix equivalent, for example: `--use_strict` vs `--nouse_strict`.
// Flags will affect all Isolates created, even after creation.
func SetFlags(flags ...string) {
	cflags := C.CString(strings.Join(flags, " "))
	C.SetFlags(cflags)
	C.free(unsafe.Pointer(cflags))
}

func initializeIfNecessary() {
	v8once.Do(func() {
		cflags := C.CString("--no-freeze_flags_after_init")
		defer C.free(unsafe.Pointer(cflags))
		C.SetFlags(cflags)
		C.Init()
	})
}

var v8once sync.Once

// SetForceRosettaFallback forces (or unforces) the Rosetta-safe compile path
// inside IsolateCompileUnboundScript. When enabled, CompileUnboundScript uses
// ScriptCompiler::Compile + GetUnboundScript instead of the direct
// ScriptCompiler::CompileUnboundScript call that crashes under Rosetta 2.
//
// This is primarily useful for testing the fallback path on native hardware.
func SetForceRosettaFallback(enabled bool) {
	v := C.int(0)
	if enabled {
		v = 1
	}
	C.SetForceRosettaFallback(v)
}
