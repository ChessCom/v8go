# API Reference

Comprehensive usage reference for the v8go public API, organized by
domain. All examples assume:

```go
import v8 "github.com/ChessCom/v8go"
```

## Core

### Version

```go
fmt.Println(v8.Version()) // e.g. "13.6.233.10-v8go"
```

### V8 flags

```go
v8.SetFlags("--max-old-space-size=512", "--harmony")
```

Flags affect all isolates, even ones already created. See
[V8 flag definitions](https://github.com/v8/v8/blob/master/src/flags/flag-definitions.h).

### Isolate

```go
iso := v8.NewIsolate()
defer iso.Dispose()
```

With resource constraints:

```go
iso := v8.NewIsolate(v8.WithResourceConstraints(
    8 * 1024 * 1024,   // initial heap
    64 * 1024 * 1024,  // max heap
))
```

With a snapshot blob:

```go
iso := v8.NewIsolate(v8.WithSnapshotBlob(blobBytes))
```

### Context

```go
ctx := v8.NewContext()       // creates a new isolate implicitly
ctx := v8.NewContext(iso)    // uses an existing isolate
ctx := v8.NewContext(iso, globalTemplate)  // with a global template
defer ctx.Close()
```

Access the parent isolate:

```go
iso := ctx.Isolate()
```

### RunScript

```go
val, err := ctx.RunScript("1 + 1", "math.js")
if err != nil {
    e := err.(*v8.JSError)
    fmt.Println(e.Message)
    fmt.Println(e.Location)
    fmt.Println(e.StackTrace)
}
fmt.Println(val.String()) // "2"
```

### Global object

```go
global := ctx.Global()
global.Set("version", "1.0.0")
val, _ := ctx.RunScript("version", "v.js")
```

### Microtask checkpoint

```go
ctx.PerformMicrotaskCheckpoint()
```

Drains the default microtask queue. Required to make progress on
Promises when V8's auto-run policy is not active.

### Terminate execution

```go
go func() {
    time.Sleep(100 * time.Millisecond)
    iso.TerminateExecution()
}()
val, err := ctx.RunScript(longRunningScript, "slow.js")
// err will be an ExecutionTerminated error
```

### Heap statistics

```go
stats := iso.GetHeapStatistics()
fmt.Printf("used: %d / %d\n", stats.UsedHeapSize, stats.HeapSizeLimit)
```

Fields: `TotalHeapSize`, `TotalHeapSizeExecutable`, `TotalPhysicalSize`,
`TotalAvailableSize`, `UsedHeapSize`, `HeapSizeLimit`, `MallocedMemory`,
`ExternalMemory`, `PeakMallocedMemory`, `NumberOfNativeContexts`,
`NumberOfDetachedContexts`.

## Values

### Creating values

```go
strVal, _ := v8.NewValue(iso, "hello")
intVal, _ := v8.NewValue(iso, int32(42))
uintVal, _ := v8.NewValue(iso, uint32(42))
i64Val, _ := v8.NewValue(iso, int64(42))
u64Val, _ := v8.NewValue(iso, uint64(42))
floatVal, _ := v8.NewValue(iso, 3.14)
boolVal, _ := v8.NewValue(iso, true)
bigVal, _ := v8.NewValue(iso, big.NewInt(9007199254740993))
```

Special values:

```go
null := v8.Null(iso)
undef := v8.Undefined(iso)
```

### Type checks

```go
val.IsString()
val.IsNumber()
val.IsInt32()
val.IsUint32()
val.IsBigInt()
val.IsBoolean()
val.IsObject()
val.IsArray()
val.IsFunction()
val.IsPromise()
val.IsNull()
val.IsUndefined()
val.IsNullOrUndefined()
val.IsDate()
val.IsRegExp()
val.IsMap()
val.IsSet()
val.IsSymbol()
val.IsProxy()
val.IsExternal()
val.IsArrayBuffer()
val.IsArrayBufferView()
val.IsTypedArray()
val.IsFloat32Array()
val.IsFloat64Array()
val.IsInt8Array()
val.IsInt16Array()
val.IsInt32Array()
val.IsUint8Array()
val.IsUint8ClampedArray()
val.IsUint16Array()
val.IsUint32Array()
val.IsBigInt64Array()
val.IsBigUint64Array()
val.IsDataView()
val.IsSharedArrayBuffer()
val.IsGeneratorFunction()
val.IsGeneratorObject()
val.IsAsyncFunction()
val.IsWasmModuleObject()
val.IsModuleNamespaceObject()
val.IsMapIterator()
val.IsSetIterator()
val.IsWeakMap()
val.IsWeakSet()
val.IsArgumentsObject()
val.IsNumberObject()
val.IsStringObject()
val.IsSymbolObject()
val.IsBooleanObject()
val.IsBigIntObject()
val.IsNativeError()
```

### Conversions

```go
val.String()       // string representation
val.DetailString() // detailed string (e.g. object contents)
val.Int32()        // int32
val.Uint32()       // uint32
val.Integer()      // int64
val.Number()       // float64
val.Boolean()      // bool
val.BigInt()       // *big.Int (returns nil if not a BigInt)
val.Object()       // *Object (returns nil if not an object)
```

### Comparison

```go
val1.SameValue(val2) // Object.is() semantics
```

### Release

```go
val.Release()  // release the C-side persistent handle
```

### Array index

```go
idx, ok := val.ArrayIndex() // uint32 index if the value is a valid array index
```

### Casting

```go
obj := val.Object()   // *Object or nil
fn, err := val.AsFunction()  // *Function or error
prom, err := val.AsPromise() // *Promise or error
```

## Objects

### Property access (string keys)

```go
obj.Set("key", "value")
obj.Set("count", int32(42))
obj.Set("pi", 3.14)
obj.Set("nested", anotherObject)

val, err := obj.Get("key")
exists := obj.Has("key")
deleted := obj.Delete("key")
```

### Property access (index)

```go
obj.SetIdx(0, "first")
val, err := obj.GetIdx(0)
exists := obj.HasIdx(0)
deleted := obj.DeleteIdx(0)
```

### Property access (symbol keys)

```go
sym := v8.SymbolIterator(iso)
obj.SetSymbol(sym, iteratorFunc)
val, err := obj.GetSymbol(sym)
exists := obj.HasSymbol(sym)
deleted := obj.DeleteSymbol(sym)
```

### Internal fields

```go
tmpl := v8.NewObjectTemplate(iso)
tmpl.SetInternalFieldCount(2)
obj, _ := tmpl.NewInstance(ctx)

obj.SetInternalField(0, "hidden-data")
val := obj.GetInternalField(0)
count := obj.InternalFieldCount()
```

### Method calls

```go
val, err := obj.MethodCall("toString")
val, err := obj.MethodCall("indexOf", searchVal)
```

## Functions

### Calling functions

```go
val, _ := ctx.Global().Get("myFunc")
fn, _ := val.AsFunction()
result, err := fn.Call(ctx.Global(), arg1, arg2)
```

### Creating functions from Go

```go
fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
    args := info.Args()
    this := info.This()
    ctx := info.Context()
    // ...
    return nil // returns undefined
})
```

With error handling:

```go
fn := v8.NewFunctionTemplateWithError(iso, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    if len(info.Args()) == 0 {
        return nil, fmt.Errorf("argument required")
    }
    return info.Args()[0], nil
})
```

### FunctionCallbackInfo

```go
info.Args()    // []*Value — argument slice
info.This()    // *Object — the receiver
info.Context() // *Context — current context
info.Release() // release all args and this
```

## Templates

### ObjectTemplate

```go
tmpl := v8.NewObjectTemplate(iso)
tmpl.Set("name", "default")
tmpl.Set("version", int32(1))
tmpl.SetInternalFieldCount(2)
tmpl.MarkAsUndetectable()
tmpl.SetCallAsFunctionHandler(func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    return v8.NewValue(info.Context().Isolate(), "called!")
})

obj, err := tmpl.NewInstance(ctx)
```

Use as a global template:

```go
ctx := v8.NewContext(iso, tmpl) // tmpl becomes the global object shape
```

### FunctionTemplate

```go
fn := v8.NewFunctionTemplate(iso, callback)

// Prototype methods (shared across instances)
fn.PrototypeTemplate().Set("greet", greetFn)

// Instance properties (own property on each new object)
fn.InstanceTemplate().Set("id", int32(0))
fn.InstanceTemplate().SetInternalFieldCount(1)

// Inheritance
child := v8.NewFunctionTemplate(iso, childCallback)
child.Inherit(fn)

// Get a callable function bound to a context
goFn := fn.GetFunction(ctx)
result, err := goFn.Call(ctx.Global())
```

### Accessor properties

```go
getter := v8.NewFunctionTemplateWithError(iso, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    return v8.NewValue(iso, "computed-value")
})
tmpl.SetAccessorProperty("computed", getter, nil, v8.None)
```

### Property attributes

```go
tmpl.Set("readonlyProp", "value", v8.ReadOnly)
tmpl.Set("hiddenProp", "value", v8.DontEnum)
tmpl.Set("permanentProp", "value", v8.DontDelete)
tmpl.Set("frozen", "value", v8.ReadOnly, v8.DontEnum, v8.DontDelete)
```

### Symbol-keyed template properties

```go
sym := v8.SymbolIterator(iso)
tmpl.SetSymbol(sym, iteratorFn)
```

### NewFastFunctionTemplate (V8 Fast API)

Registers a V8 Fast API callback alongside the normal slow-path Go
callback. When TurboFan optimizes a hot call site and can prove
argument types match the descriptor, it calls the C function directly
— bypassing CGo entirely.

```go
tmpl := v8.NewFastFunctionTemplate(iso, slowCallback, v8.FastCallDescriptor{
    FastFn:     unsafe.Pointer(C.MyFastAdd),
    ReturnType: v8.CTypeInt32,
    ArgTypes:   []v8.CType{v8.CTypeV8Value, v8.CTypeInt32, v8.CTypeInt32},
})
```

The first entry in `ArgTypes` must be `CTypeV8Value` (the receiver).

**CType enum:**

```go
v8.CTypeVoid
v8.CTypeBool
v8.CTypeUint8
v8.CTypeInt32
v8.CTypeUint32
v8.CTypeInt64
v8.CTypeUint64
v8.CTypeFloat32
v8.CTypeFloat64
v8.CTypePointer
v8.CTypeV8Value
v8.CTypeOneByteString
```

**Constraints:**
- The fast function must be C-linkage (not Go / CGo).
- It must not allocate on the JS heap or trigger JS execution.
- Register its address via `AddExternalReference` for snapshot compat.

## Promises

### Creating and resolving

```go
resolver, err := v8.NewPromiseResolver(ctx)
promise := resolver.GetPromise()

fmt.Println(promise.State()) // Pending

resolver.Resolve(someValue)
// or
resolver.Reject(errorValue)

ctx.PerformMicrotaskCheckpoint()
fmt.Println(promise.State())  // Fulfilled or Rejected
fmt.Println(promise.Result()) // the resolved/rejected value
```

### Chaining

With one callback (then):

```go
promise.Then(func(info *v8.FunctionCallbackInfo) *v8.Value {
    fmt.Println("resolved:", info.Args()[0])
    return nil
})
ctx.PerformMicrotaskCheckpoint()
```

With two callbacks (then + catch):

```go
promise.Then(
    func(info *v8.FunctionCallbackInfo) *v8.Value {
        return info.Args()[0] // pass through
    },
    func(info *v8.FunctionCallbackInfo) *v8.Value {
        fmt.Println("rejected:", info.Args()[0])
        return nil
    },
)
```

Catch only:

```go
promise.Catch(func(info *v8.FunctionCallbackInfo) *v8.Value {
    fmt.Println("error:", info.Args()[0])
    return nil
})
```

Error-returning variants:

```go
promise.ThenWithError(func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    return nil, fmt.Errorf("handler failed")
})
promise.CatchWithError(func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    return nil, fmt.Errorf("catch handler failed")
})
```

### Promise states

```go
v8.Pending   // 0
v8.Fulfilled // 1
v8.Rejected  // 2
```

## Symbols

### Well-known symbols

```go
v8.SymbolIterator(iso)
v8.SymbolAsyncIterator(iso)
v8.SymbolHasInstance(iso)
v8.SymbolIsConcatSpreadable(iso)
v8.SymbolMatch(iso)
v8.SymbolReplace(iso)
v8.SymbolSearch(iso)
v8.SymbolSplit(iso)
v8.SymbolToPrimitive(iso)
v8.SymbolToStringTag(iso)
v8.SymbolUnscopables(iso)
```

### Symbol properties

```go
sym := v8.SymbolIterator(iso)
fmt.Println(sym.Description()) // "Symbol.iterator"
fmt.Println(sym.String())      // "Symbol.iterator"
```

## Errors and exceptions

### JSError (Go-side JavaScript errors)

```go
val, err := ctx.RunScript("throw new Error('boom')", "err.js")
if err != nil {
    e := err.(*v8.JSError)
    fmt.Println(e.Message)    // "boom"
    fmt.Println(e.Location)   // "err.js:1:0"
    fmt.Println(e.StackTrace) // full stack trace

    // Verbose formatting includes stack trace
    fmt.Printf("%+v\n", e)
}
```

### Exception (throwable V8 error objects)

```go
e := v8.NewError(iso, "something went wrong")
e := v8.NewTypeError(iso, "expected a number")
e := v8.NewRangeError(iso, "index out of bounds")
e := v8.NewReferenceError(iso, "x is not defined")
e := v8.NewSyntaxError(iso, "unexpected token")
```

Wasm-specific errors:

```go
e := v8.NewWasmCompileError(iso, "compile failed")
e := v8.NewWasmLinkError(iso, "link failed")
e := v8.NewWasmRuntimeError(iso, "runtime error")
```

Throwing from a Go callback:

```go
fn := v8.NewFunctionTemplateWithError(iso, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
    return nil, v8.NewTypeError(info.Context().Isolate(), "wrong type")
})
```

`Exception` implements `error`, `errors.Is`, and `errors.As`:

```go
var exc *v8.Exception
if errors.As(err, &exc) {
    fmt.Println(exc.String())
}
```

### ThrowException

```go
iso.ThrowException(errorValue)
```

## JSON

### Parse

```go
val, err := v8.JSONParse(ctx, `{"key": "value"}`)
```

### Stringify

```go
str, err := v8.JSONStringify(ctx, val)
```

## Script compilation

### Unbound scripts (context-independent)

```go
script, err := iso.CompileUnboundScript(source, "math.js", v8.CompileOptions{})
val, err := script.Run(ctx)
```

### Code caching

```go
// Compile and create cache
script1, _ := iso.CompileUnboundScript(source, "app.js", v8.CompileOptions{})
cache := script1.CreateCodeCache()

// Use cache on a different isolate
script2, _ := iso2.CompileUnboundScript(source, "app.js", v8.CompileOptions{
    CachedData: cache,
})
if cache.Rejected {
    // cache was invalid, script was compiled from source
}
```

### Compile modes

```go
v8.CompileOptions{Mode: v8.CompileModeDefault}
v8.CompileOptions{Mode: v8.CompileModeEager} // force eager compilation
```

## Snapshots

### SnapshotCreator (low-level)

```go
sc := v8.NewSnapshotCreator()

ctx := sc.Context()
ctx.RunScript("globalThis.add = (a, b) => a + b", "bundle.js")

blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
sc.Dispose()

// Use the blob
iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
ctx = v8.NewContext(iso)
val, _ := ctx.RunScript("add(1, 2)", "test.js")
fmt.Println(val.Int32()) // 3
```

### Deterministic snapshots

```go
sc := v8.NewSnapshotCreator(
    v8.WithDeterministicTime(v8.SeedTimeMillis),
)
ctx := sc.Context()
ctx.RunScript(src, "bundle.js")
blob, _ := sc.CreateBlob(v8.FunctionCodeKeep)
sc.Dispose()
```

### Snapshot stacking

```go
// Layer 1: base runtime
sc1 := v8.NewSnapshotCreator()
sc1.Context().RunScript(polyfills, "polyfills.js")
baseBlob, _ := sc1.CreateBlob(v8.FunctionCodeKeep)
sc1.Dispose()

// Layer 2: app-specific (built on top of layer 1)
sc2 := v8.NewSnapshotCreator(v8.WithExistingSnapshotBlob(baseBlob))
sc2.Context().RunScript(appCode, "app.js")
appBlob, _ := sc2.CreateBlob(v8.FunctionCodeKeep)
sc2.Dispose()

// Consumer only needs the final blob
iso := v8.NewIsolate(v8.WithSnapshotBlob(appBlob))
```

### FunctionCodeHandling

```go
v8.FunctionCodeKeep  // preserve compiled bytecode (larger blob, no recompile)
v8.FunctionCodeClear // strip compiled code (smaller blob, recompile on first call)
```

### Resetting determinism after restore

```go
iso, _ := packed.RestoreIsolate(v8.RestoreOptions{})
ctx := v8.NewContext(iso)
v8.ResetNonDeterminism(ctx) // Date.now, Math.random, performance.now restored
```

## Pack/Restore (high-level snapshot API)

### PackBundle

```go
packed, err := v8.PackBundle(v8.PackOptions{
    Source:            string(bundleJS),
    Origin:            "bundle.js",           // default: "bundle.js"
    FCH:               v8.FunctionCodeKeep,
    DeterministicTime: true,
    SeedMillis:        v8.SeedTimeMillis,     // default if 0
    ExistingBlob:      baseBlob,              // optional: stack on base
    Extra: map[string]string{                 // arbitrary metadata
        "build_sha": "abc123",
        "route":     "/home",
    },
})
```

### Serialise / deserialise

```go
// To bytes (for disk or network)
data, err := packed.Marshal()

// From bytes
restored, err := v8.UnmarshalPackedSnapshot(data)
```

### PackedSnapshot fields

```go
packed.V8ABI        // V8 version string
packed.RefsDigest   // sha256 of external references
packed.BundleSHA256 // sha256 of the source
packed.CreatedUnix  // creation timestamp
packed.FCH          // FunctionCodeHandling used
packed.Extra        // arbitrary metadata map
packed.Blob         // raw V8 startup data (not in JSON header)
```

### RestoreIsolate

```go
iso, err := packed.RestoreIsolate(v8.RestoreOptions{})
if errors.Is(err, v8.ErrIncompatible) {
    // V8 ABI or refs digest mismatch — fall back to cold start
}
if errors.Is(err, v8.ErrCorruptSnapshot) {
    // bad magic, truncated, or invalid JSON
}
```

With options:

```go
iso, err := packed.RestoreIsolate(v8.RestoreOptions{
    AllowABIMismatch:        false,  // default
    AllowRefsDigestMismatch: false,  // default
    ResourceConstraints: v8.NewResourceConstraints(
        8 * 1024 * 1024,
        64 * 1024 * 1024,
    ),
})
```

## External references

### Registering references

```go
// Must be called BEFORE any snapshot is created or consumed.
v8.AddExternalReference("myCallback", unsafe.Pointer(C.myCallback))
```

The built-in `v8go.FunctionTemplateCallback` is registered
automatically.

### Querying the registry

```go
v8.IsExternalReferenceRegistryFrozen() // true after first snapshot op
v8.ExternalReferenceRegistryDigest()   // sha256 hex of sorted names
v8.ExternalReferenceRegistryNames()    // sorted name list
```

The registry freezes on first use (sorted by name, C array
materialised). After freeze, `AddExternalReference` panics.

## CPU Profiler

```go
profiler := v8.NewCPUProfiler(iso)
defer profiler.Dispose()

profiler.StartProfiling("my-profile")

ctx.RunScript(code, "script.js")
val, _ := ctx.Global().Get("start")
fn, _ := val.AsFunction()
fn.Call(ctx.Global())

profile := profiler.StopProfiling("my-profile")

fmt.Println(profile.GetTitle())
fmt.Println(profile.GetDuration())

root := profile.GetTopDownRoot()
printTree(root)
```

### CPUProfile

```go
profile.GetTitle()       // string
profile.GetDuration()    // time.Duration
profile.GetTopDownRoot() // *CPUProfileNode
```

### CPUProfileNode

```go
node.GetFunctionName()       // string
node.GetScriptResourceName() // string
node.GetLineNumber()         // int
node.GetColumnNumber()       // int
node.GetHitCount()           // int
node.GetBailoutReason()      // string
node.GetNodeId()             // int
node.GetScriptId()           // int
node.GetParent()             // *CPUProfileNode or nil
node.GetChildrenCount()      // int
node.GetChild(index)         // *CPUProfileNode
```

## Inspector

### Setup

```go
type myHandler struct{}

func (h *myHandler) ConsoleAPIMessage(msg v8.ConsoleAPIMessage) {
    fmt.Printf("[%s] %s\n", msg.ErrorLevel, msg.Message)
}

client := v8.NewInspectorClient(&myHandler{})
defer client.Dispose()

inspector := v8.NewInspector(iso, client)
defer inspector.Dispose()

inspector.ContextCreated(ctx)
defer inspector.ContextDestroyed(ctx)
```

### ConsoleAPIMessage fields

```go
msg.ErrorLevel   // MessageErrorLevel
msg.Message      // string
msg.Url          // string
msg.LineNumber   // uint
msg.ColumnNumber // uint
```

### MessageErrorLevel

```go
v8.ErrorLevelLog     // console.log
v8.ErrorLevelDebug   // console.debug
v8.ErrorLevelInfo    // console.info
v8.ErrorLevelError   // console.error
v8.ErrorLevelWarning // console.warn
v8.ErrorLevelAll     // bitmask of all levels
```

---

## GC and Memory Pressure

### LowMemoryNotification

Triggers a full garbage collection to free as much memory as possible.
Blocks until GC completes.

```go
iso.LowMemoryNotification()
```

### MemoryPressureNotification

Signals V8 about memory pressure so it adjusts its GC strategy.

```go
iso.MemoryPressureNotification(v8.MemoryPressureNone)     // default schedule
iso.MemoryPressureNotification(v8.MemoryPressureModerate)  // speed up incremental GC
iso.MemoryPressureNotification(v8.MemoryPressureCritical)  // aggressive GC
```

### CancelTerminateExecution

Cancels a pending `TerminateExecution` request. Useful when recycling
an isolate after a timeout.

```go
iso.TerminateExecution()       // schedule termination
iso.CancelTerminateExecution() // cancel before next script runs
```

### RequestGarbageCollectionForTesting

Forces an explicit GC cycle. Requires `--expose_gc` flag.

```go
v8.SetFlags("--expose_gc")
iso := v8.NewIsolate()
iso.RequestGarbageCollectionForTesting(v8.GCTypeFull)
iso.RequestGarbageCollectionForTesting(v8.GCTypeMinor)
```

### ContextDisposedNotification

Notifies V8 that a context was disposed to tune GC scheduling.

```go
ctx.Close()
iso.ContextDisposedNotification(false) // no dependant contexts
iso.ContextDisposedNotification(true)  // other contexts depend on this one
```

---

## Heap Limit Callbacks

### WithoutDefaultHeapLimitCallback

Disables the built-in callback that calls `TerminateExecution` on OOM.

```go
iso := v8.NewIsolate(
    v8.WithResourceConstraints(0, 50*1024*1024),
    v8.WithoutDefaultHeapLimitCallback(),
)
```

### AddNearHeapLimitCallback

Installs a custom callback when the heap approaches the configured limit.

```go
iso.AddNearHeapLimitCallback(func(current, initial uint64) uint64 {
    log.Printf("heap limit approaching: %d / %d", current, initial)
    iso.TerminateExecution()
    return current * 2 // must return > current to avoid crash
})
```

### RemoveNearHeapLimitCallback

Removes the custom callback and optionally restores a heap limit.

```go
iso.RemoveNearHeapLimitCallback(0) // keep current limit
```

---

## Object Enumeration and Prototype Access

### GetPropertyNames

Returns all property names including inherited ones from the prototype chain.

```go
names, err := obj.GetPropertyNames()
namesObj, _ := names.AsObject()
length, _ := namesObj.Get("length")
```

### GetOwnPropertyNames

Returns only the object's own property names (not inherited).

```go
names, err := obj.GetOwnPropertyNames()
```

### GetPrototype / SetPrototype

Access and modify the prototype chain.

```go
proto := obj.GetPrototype()
err := obj.SetPrototype(newProto)
```

---

## Promise Reject Callback

### SetPromiseRejectCallback

Installs a callback for unhandled promise rejections.

```go
iso.SetPromiseRejectCallback(func(msg v8.PromiseRejectMessage) {
    switch msg.Event {
    case v8.PromiseRejectWithNoHandler:
        log.Printf("unhandled rejection: %s", msg.Value.String())
    case v8.PromiseHandlerAddedAfterReject:
        log.Printf("handler added after reject")
    case v8.PromiseRejectAfterResolved:
        log.Printf("rejected after resolved")
    case v8.PromiseResolveAfterResolved:
        log.Printf("resolved after resolved")
    }
})
```

---

## Interrupt and Idle

### RequestInterrupt

Requests V8 to terminate execution at the next interrupt check point.
Safe to call from any goroutine.

```go
go func() {
    time.Sleep(5 * time.Second)
    iso.RequestInterrupt() // terminates long-running JS
}()
_, err := ctx.RunScript("while(true) {}", "loop.js")
```

### SetIdle

Tells V8 whether the embedder is idle so it can do speculative work.

```go
iso.SetIdle(true)  // hint that embedder is idle
iso.SetIdle(false) // back to active
```

### RunIdleTasks

Gives V8 a time budget to perform idle-time work such as incremental
GC sweeping, deoptimization cleanup, and code aging. Call this when
the embedder is idle (e.g. between SSR requests in a pool). Pair with
`SetIdle(true)` for best results.

```go
iso.SetIdle(true)
iso.RunIdleTasks(0.005) // 5 ms idle window
iso.SetIdle(false)
```

---

## GC Prologue and Epilogue Callbacks

### AddGCPrologueCallback / AddGCEpilogueCallback

Register callbacks that fire before and after each GC cycle.

```go
iso.AddGCPrologueCallback(func(gcType v8.GCType) {
    log.Printf("GC starting: type=%d", gcType)
})

iso.AddGCEpilogueCallback(func(gcType v8.GCType) {
    log.Printf("GC finished: type=%d", gcType)
})
```

### GCType constants

```go
v8.GCTypeScavenge             // young generation
v8.GCTypeMinorMarkSweep       // minor mark-sweep
v8.GCTypeMarkSweepCompact     // full mark-sweep-compact
v8.GCTypeIncrementalMarking   // incremental marking step
v8.GCTypeProcessWeakCallbacks // weak callback processing
v8.GCTypeAll                  // bitmask of all types
```

### RemoveGCPrologueCallbacks / RemoveGCEpilogueCallbacks

Remove all registered callbacks.

```go
iso.RemoveGCPrologueCallbacks()
iso.RemoveGCEpilogueCallbacks()
```

---

## External Memory Accounting

### AdjustExternalMemory

Reports Go-side allocations to V8's GC heuristic so V8 can factor
them into its collection decisions. Every positive adjustment should
eventually be balanced by a negative adjustment.

```go
newTotal := iso.AdjustExternalMemory(1024 * 1024)  // +1 MiB
iso.AdjustExternalMemory(-1024 * 1024)              // -1 MiB
```

---

## Microtask Control

### SetMicrotasksPolicy

Controls when V8 drains its microtask queue.

```go
iso.SetMicrotasksPolicy(v8.MicrotasksPolicyExplicit) // manual checkpoint required
iso.SetMicrotasksPolicy(v8.MicrotasksPolicyScoped)   // drain at scope boundaries
iso.SetMicrotasksPolicy(v8.MicrotasksPolicyAuto)     // drain at every script exit
```

### EnqueueMicrotask

Schedules a JavaScript function to run as a microtask, bypassing the
`Promise.resolve().then(fn)` pattern and saving one promise allocation.

```go
fnVal, _ := ctx.RunScript("(function() { console.log('microtask') })", "mt.js")
fn, _ := fnVal.AsFunction()
iso.EnqueueMicrotask(fn)
ctx.PerformMicrotaskCheckpoint() // or wait for auto-drain
```

---

## OOM Error Handler

### SetOOMErrorHandler

Installs a Go callback that V8 invokes on out-of-memory. The callback
runs on the V8 thread mid-allocation and must not allocate Go memory
or call back into V8. Pass nil to clear and restore default behavior.

```go
iso.SetOOMErrorHandler(func(location string, isHeap bool) {
    log.Printf("V8 OOM at %s (heap=%v)", location, isHeap)
})

iso.SetOOMErrorHandler(nil) // restore default abort-on-OOM
```

---

## ArrayBuffer

### NewArrayBuffer

Creates an ArrayBuffer by copying a Go byte slice into V8's heap.

```go
ab, _ := v8.NewArrayBuffer(ctx, []byte{1, 2, 3, 4})
```

### NewArrayBufferAlloc

Allocates a zero-initialized ArrayBuffer inside V8's sandbox address
space. Use `ArrayBufferGetBytes` to populate it without an extra copy.

```go
ab, _ := v8.NewArrayBufferAlloc(ctx, 1024)
backing := ab.ArrayBufferGetBytes()
copy(backing, myData)
```

### NewArrayBufferExternal

Creates an ArrayBuffer backed directly by a Go byte slice. When the V8
sandbox is disabled, this is true zero-copy — JS and Go share the same
memory. When `V8_ENABLE_SANDBOX` is active (current prebuilt deps), it
falls back to alloc + copy internally. The slice is pinned via
`runtime.Pinner` and released when V8 GCs the ArrayBuffer.

```go
data := make([]byte, 64*1024)
ab, _ := v8.NewArrayBufferExternal(ctx, data)
// With sandbox disabled: JS reads/writes `data` directly.
// With sandbox enabled: V8 copies data in; subsequent mutations
// to the Go slice are NOT visible in JS.
```

### SandboxEnabled

Reports whether the V8 binary was compiled with `V8_ENABLE_SANDBOX`.
Use this to decide between zero-copy and copy-in strategies at runtime.

```go
if v8.SandboxEnabled() {
    // External ArrayBuffer will copy; use NewArrayBufferAlloc + write instead
} else {
    // True zero-copy via NewArrayBufferExternal
}
```

### ArrayBufferGetBytes / ArrayBufferByteLength

Access the backing store and length of an ArrayBuffer.

```go
data := ab.ArrayBufferGetBytes() // []byte pointing into V8 backing store
length := ab.ArrayBufferByteLength()
```

---

## External Strings

### NewExternalOneByteString

Creates a V8 string that points directly at Go memory without copying.
The caller must keep the backing slice alive and immutable for the
string's lifetime. Only valid for Latin-1 / ASCII data.

```go
data := []byte("hello")
val, _ := v8.NewExternalOneByteString(ctx, data)
// data must remain live while val is reachable
```

---

## Named Property Interceptors

### SetNamedPropertyHandler

Installs interceptors on an ObjectTemplate for named property access.

```go
tmpl := v8.NewObjectTemplate(iso)
tmpl.SetNamedPropertyHandler(
    func(property string, info *v8.InterceptorCallbackInfo) *v8.Value {
        if property == "magic" {
            val, _ := v8.NewValue(iso, int32(42))
            return val
        }
        return nil // fall through to own properties
    },
    func(property string, value *v8.Value, info *v8.InterceptorCallbackInfo) bool {
        log.Printf("set %s = %s", property, value.String())
        return true // intercepted
    },
)
ctx := v8.NewContext(iso, tmpl) // tmpl as global
```

---

## Heap Profiler

### TakeHeapSnapshot

Takes a V8 heap snapshot and returns it as JSON. Compatible with
Chrome DevTools Memory tab.

```go
snapshot, _ := iso.TakeHeapSnapshot()
os.WriteFile("heap.json", snapshot, 0644)
```

---

## ES Modules (ESM)

### CompileModule

Compiles an ES module from source. The module starts in
`ModuleStatusUninstantiated`.

```go
mod, err := ctx.CompileModule(`export const x = 42;`, "my-module.mjs")
defer mod.Close()
```

### Module Status

```go
v8.ModuleStatusUninstantiated // 0 — compiled but not instantiated
v8.ModuleStatusInstantiating  // 1 — resolving imports
v8.ModuleStatusInstantiated   // 2 — ready to evaluate
v8.ModuleStatusEvaluating     // 3 — currently evaluating
v8.ModuleStatusEvaluated      // 4 — evaluation succeeded
v8.ModuleStatusErrored        // 5 — evaluation failed
```

### Instantiate and Evaluate

```go
err = mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
    // Look up and return the module for the given import specifier.
    return moduleMap[specifier]
})
val, err := mod.Evaluate()
```

### Module Namespace

After evaluation, access the module's exports via its namespace object.

```go
ns := mod.GetNamespace()
obj := ns.Object()
xVal, _ := obj.Get("x")
fmt.Println(xVal.Int32()) // 42
```

### Import Requests

Inspect a module's import statements before instantiation.

```go
n := mod.GetModuleRequestsLength()
for i := 0; i < n; i++ {
    fmt.Println(mod.GetModuleRequest(i)) // "./dep.mjs"
}
```

### ESM Snapshots (PackBundleESM)

Evaluate an ES module inside a SnapshotCreator and serialize the
resulting heap. The module namespace is bridged to a global property
so consumers can access exports without re-evaluating the module.

```go
packed, err := v8.PackBundleESM(v8.PackESMOptions{
    EntrySource: `
        import { helper } from './chunk.mjs';
        export function render(x) { return helper(x); }
    `,
    EntryOrigin: "entry.mjs",
    Chunks: map[string]string{
        "./chunk.mjs": `export function helper(x) { return "<div>" + x + "</div>"; }`,
    },
    FCH:       v8.FunctionCodeKeep,
    BridgeKey: "__app",              // default: "__esmExports"
    Extra:     map[string]string{"route": "/home"},
})
```

Restore and use:

```go
iso, err := packed.RestoreIsolate(v8.RestoreOptions{})
ctx := v8.NewContext(iso)
val, _ := ctx.RunScript(`__app.render("hello")`, "probe.js")
fmt.Println(val.String()) // "<div>hello</div>"
```

With snapshot stacking (base polyfills + ESM app):

```go
basePacked, _ := v8.PackBundle(v8.PackOptions{
    Source: polyfillsJS,
    FCH:   v8.FunctionCodeKeep,
})
appPacked, _ := v8.PackBundleESM(v8.PackESMOptions{
    EntrySource:  appBundleESM,
    ExistingBlob: basePacked.Blob,
    BridgeKey:    "__app",
    FCH:          v8.FunctionCodeKeep,
})
```

Low-level ESM snapshot (without PackBundleESM):

```go
sc := v8.NewSnapshotCreator()
ctx := sc.Context()

mod, _ := ctx.CompileModule(source, "app.mjs")
mod.Instantiate(resolver)
mod.Evaluate()

ns := mod.GetNamespace()
ctx.Global().Set("__app", ns)
mod.Close()

blob, _ := sc.CreateBlob(v8.FunctionCodeKeep)
sc.Dispose()
```

Module handles are tracked automatically when compiled on a
SnapshotCreator context. If `mod.Close()` is not called before
`CreateBlob`, the handles are auto-released to prevent V8 from
aborting on "global handle not serialized".
