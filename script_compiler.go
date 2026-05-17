package v8go

// #include "v8go.h"
import "C"

type CompileMode C.int

var (
	CompileModeDefault = CompileMode(C.ScriptCompilerNoCompileOptions)
	CompileModeEager   = CompileMode(C.ScriptCompilerEagerCompile)
)

type CompilerCachedData struct {
	Bytes    []byte
	Rejected bool
}
