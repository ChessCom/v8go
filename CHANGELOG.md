# Changelog

All notable changes to this fork are documented in this file. Tags
follow `vMAJOR.MINOR.PATCH-chess.N`, where `MAJOR.MINOR.PATCH` mirrors
the upstream `tommie/v8go` version this fork tracks and `chess.N`
increments per ChessCom-side change set.

## v0.34.0-chess.5 — 2026-05

Warm-path performance: zero-copy ArrayBuffer (with sandbox fallback),
V8 Fast API callbacks, idle-task GC scheduling, and CI consolidation.

### Added

* **Zero-copy ArrayBuffer** (`arraybuffer.{h,cc,go}`):
  `NewArrayBufferExternal(ctx, []byte)` creates an ArrayBuffer backed
  directly by Go memory via external BackingStore + `runtime.Pinner`.
  When the V8 sandbox is disabled, JS and Go share the same bytes with
  no copy. When `V8_ENABLE_SANDBOX` is active (current prebuilt deps),
  the function falls back to alloc + `memcpy` since the sandbox
  requires backing stores to reside in its address space.

* **`SandboxEnabled() bool`** (`arraybuffer.go`):
  Runtime query for whether the V8 binary was compiled with
  `V8_ENABLE_SANDBOX`. Callers can use this to choose between
  zero-copy and copy-in strategies.

* **V8 Fast API callbacks** (`fast_api.{h,cc}`, `function_template.{h,cc,go}`):
  `NewFastFunctionTemplate(iso, slowCB, FastCallDescriptor{...})` wires
  a C-linkage fast path directly into TurboFan-compiled code. Bypasses
  CGo, argument marshaling, and `m_value` allocation on hot paths.

* **Idle-task GC scheduling** (`isolate.{h,cc,go}`):
  `RunIdleTasks(deadlineSeconds)` drives V8's incremental GC sweeper,
  deopt cleanup, and code aging within a caller-controlled time budget.
  Platform now initialized with `IdleTaskSupport::kEnabled`.

### Changed

* **Sandbox status** (`cgo.go`, `deps/build.py`):
  `V8_ENABLE_SANDBOX` remains active in `cgo.go` to match the prebuilt
  static libraries. `deps/build.py` sets `v8_enable_sandbox=false` for
  local rebuilds — once deps are rebuilt without the sandbox, the
  `NewArrayBufferExternal` zero-copy path activates automatically.
  `SandboxEnabled()` reports the compile-time state at runtime.

* **CI consolidated** (`.github/workflows/ci.yml`):
  Collapsed from 10 jobs to 2 (one per OS). Each job runs lint, build,
  test + coverage, ESM flake detection, and downstream compat checks
  sequentially. Deleted `auto-bump-downstreams.yml`.

---

## v0.34.0-chess.4 — 2026-05

ESM snapshot support: evaluate ES modules inside SnapshotCreator and
serialize the resulting heap for instant restore.

### Added

- `PackBundleESM()` high-level API for snapshotting ES module bundles
  with multi-chunk resolver support
- Module handle auto-tracking in SnapshotCreator contexts to prevent
  V8 "global handle not serialized" aborts
- 21 regression tests covering named/default exports, closures,
  multi-chunk, diamond deps, stacking, complex object graphs,
  auto-release safety, render parity, and cold-start speedup
- Dedicated `esm-snapshot` CI job with `-count=3` flake detection

### Performance

- ESM snapshot cold-start achieves 3.5–4.6x speedup over fresh module
  evaluation on M-class hardware (~750 KiB bundle)

---

## v0.34.0-chess.3 — 2026-05

Fork-frontier Waves 1–3: cooperative GC, microtask control, external
memory accounting, OOM observability, ArrayBuffer, external strings,
named property interceptors, heap profiling, and ES module support.

### Added (Wave 3)

* **ES Module support** (`module.{h,cc,go}`):
  `Context.CompileModule(source, origin)` compiles an ES module.
  `Module.Instantiate(resolver)` resolves imports via a Go callback.
  `Module.Evaluate()` executes the module and returns the completion
  value. `Module.GetNamespace()` returns the namespace object
  containing all exports. Full `ModuleStatus` enum and import request
  introspection (`GetModuleRequestsLength`, `GetModuleRequest`).
  The resolve callback uses the same trampoline pattern as
  `FunctionTemplateCallback`.

* **ESM snapshot support** (`pack.go`, `snapshot.cc`, `context.h`):
  `PackBundleESM` evaluates ES modules inside `SnapshotCreator`,
  bridges the module namespace to a configurable global key, and
  serialises the heap into a `PackedSnapshot`. Supports multi-chunk
  resolution (entry + dependency modules), snapshot stacking on
  existing blobs, and `FunctionCodeKeep`/`FunctionCodeClear` modes.
  Module `Persistent<Module>` handles are auto-tracked in `m_ctx` and
  released before `CreateBlob` to prevent V8 serialisation aborts.
  Achieves 3.5-6x cold-start speedup over fresh ESM evaluation.
  Covered by 18 regression tests in `snapshot_esm_test.go` and a
  dedicated `esm-snapshot` CI job with flake detection (`-count=3`).

### Deferred

* **Fast API Callbacks** (F4.1): requires C++ template machinery
  (`CFunction`/`CFunctionInfo`) that is impractical to expose through
  CGO. Deferred to a future release.
* **Streaming Bundle Compile** (F1.3): requires background thread
  management for `ScriptStreamingTask`. Deferred to a future release.

### Added (Wave 2)

* **ArrayBuffer API** (`arraybuffer.{h,cc,go}`):
  `NewArrayBuffer(ctx, []byte)` copies Go data into a V8 ArrayBuffer.
  `NewArrayBufferAlloc(ctx, size)` allocates a zero-initialized
  ArrayBuffer inside V8's sandbox; `ArrayBufferGetBytes()` returns a
  writable slice into the backing store for single-copy data transfer.
  `ArrayBufferByteLength()` returns the buffer size.
* **External strings** (`external_string.{h,cc,go}`):
  `NewExternalOneByteString(ctx, []byte)` creates a V8 string pointing
  directly at Go-owned memory (Latin-1/ASCII only). Zero-copy for
  immutable string data like DOM attribute values and CSS properties.
* **Named property interceptors** (`interceptor.{h,cc,go}`):
  `ObjectTemplate.SetNamedPropertyHandler(getter, setter)` installs
  interceptor callbacks for lazy property resolution. The getter returns
  a `*Value` to intercept or nil to fall through. The setter returns
  true if the set was intercepted. Routes through the isolate callback
  registry using the `Integer` data pattern.
* **Heap profiler** (`heap_profiler.{h,cc,go}`):
  `Isolate.TakeHeapSnapshot()` captures a V8 heap snapshot as JSON,
  compatible with Chrome DevTools Memory tab. Serialises via a
  `StringOutputStream` adapter and copies to Go memory.

### Added (Wave 1)

* **External memory accounting** (`isolate.go`):
  `Isolate.AdjustExternalMemory(int64) int64` — reports Go-side
  allocations to V8's GC heuristic so collection frequency adjusts
  to the true memory footprint. Wraps the deprecated
  `AdjustAmountOfExternalAllocatedMemory` (still functional; a future
  release will migrate to `ExternalMemoryAccounter`).
* **Microtask policy control** (`isolate.go`):
  `Isolate.SetMicrotasksPolicy(MicrotasksPolicy)` — switch between
  Explicit, Scoped, and Auto microtask drain modes.
  `MicrotasksPolicyExplicit` / `MicrotasksPolicyScoped` /
  `MicrotasksPolicyAuto` constants.
* **Microtask enqueue** (`isolate.go`):
  `Isolate.EnqueueMicrotask(*Function)` — schedules a JS function as
  a microtask, bypassing `Promise.resolve().then()` and saving one
  promise allocation per enqueue.
* **OOM error handler** (`oom_handler.{h,cc,go}`):
  `Isolate.SetOOMErrorHandler(OOMErrorCallback)` — installs a Go
  callback invoked by V8 on out-of-memory. Uses
  `Isolate::TryGetCurrent()` to recover the active isolate since V8's
  OOM callback signature does not include it. Pass nil to clear.

---

## v0.34.0-chess.1 — 2026-05

API surface extensions for improved resilience and performance.

### Added

* **GC and memory pressure APIs** (`isolate.go`):
  `LowMemoryNotification`, `MemoryPressureNotification` (None/Moderate/Critical),
  `CancelTerminateExecution`, `RequestGarbageCollectionForTesting`,
  `ContextDisposedNotification`.
* **Configurable heap limit policy** (`heap_limit.go`):
  `WithoutDefaultHeapLimitCallback` option, `AddNearHeapLimitCallback`,
  `RemoveNearHeapLimitCallback` — allows custom Go callbacks when the
  heap approaches the configured maximum.
* **Object enumeration and prototype access** (`object.go`):
  `Object.GetPropertyNames`, `Object.GetOwnPropertyNames`,
  `Object.GetPrototype`, `Object.SetPrototype`.
* **Promise reject callback** (`promise_reject.go`):
  `Isolate.SetPromiseRejectCallback` — notifies Go of unhandled
  promise rejections with event type, promise, and rejection value.
* **Interrupt and idle** (`interrupt.go`):
  `Isolate.RequestInterrupt` (terminates via V8 interrupt mechanism),
  `Isolate.SetIdle` (hints idle state for speculative GC).
* **GC prologue/epilogue callbacks** (`gc_callback.go`):
  `AddGCPrologueCallback`, `RemoveGCPrologueCallbacks`,
  `AddGCEpilogueCallback`, `RemoveGCEpilogueCallbacks` — observe GC
  lifecycle with typed GC events.
* **Isolate registry** (`isolate_registry.go`): process-global map
  from `IsolatePtr` to `*Isolate` for CGO callback dispatch.

### Changed

* `ConfigureIsolate` in `isolate.cc` now accepts a `bool add_heap_limit_cb`
  parameter; `NewIsolateNoDefaultHeapCB` bypasses the default callback.
* Dead `ObjectGetPrototype`/`ObjectSetPrototype` declarations removed
  from `value.h`; they now live in `object.h` with corrected signatures.

### Documentation

* Renamed `docs/MAINTAINING.md` → `docs/maintaining.md`.
* Updated CGO surface drift table and conflict resolution guide for all
  new fork-specific C++ files.
* Added complete API reference sections for all new APIs.
* Added callback architecture section to `docs/architecture.md`.

---

## v0.34.0-chess.0 — 2026-05

First release of the ChessCom fork.

### Added

* **`v8::SnapshotCreator` bindings** (`snapshot.{h,cc,go}`): construct
  startup blobs from Go, including support for `external_references`
  so Go-backed `FunctionTemplate` callbacks survive serialisation.
  `SnapshotCreator` pins its goroutine to the calling OS thread via
  `runtime.LockOSThread` and is serialised process-wide so the V8
  read-only-heap initialiser is never reentered.
* **Process-wide external references registry** (`extref.go`): ordered,
  frozen-on-first-use registry of named function pointers with a
  SHA-256 digest that downstream consumers can compare against when
  loading a snapshot. The `v8go.FunctionTemplateCallback` trampoline
  is baked in.
* **High-level Pack/Restore API** (`pack.go`):
  * `PackBundle(PackOptions) (*PackedSnapshot, error)` — one-shot
    snapshot from JS source, with optional determinism and existing
    blob inputs.
  * `PackedSnapshot.Marshal` / `UnmarshalPackedSnapshot` — versioned
    envelope (`BFV8\x01` magic + JSON metadata + raw V8 blob).
  * `PackedSnapshot.RestoreIsolate(RestoreOptions)` — validates V8
    ABI and external-references digest before V8 sees the bytes;
    truncated and obviously-malformed blobs return `ErrIncompatible`
    instead of crashing the host process.
* **Deterministic snapshot mode** (`determinism.go`): a
  `WithDeterministicTime(seedMillis)` option pins `Date.now`,
  `Math.random`, and `performance.now` so snapshot inputs are
  reproducible; `ResetNonDeterminism` restores intrinsics on the
  consumer side.
* **CI scaffolding** (`.github/workflows/`):
  * `ci.yml` — unit (linux+macos), vet (gofmt + go vet +
    clang-format dry-run), and downstream compat jobs that build
    `ChessCom/blindfox` and `ChessCom/er` against the PR HEAD.
  * `auto-bump-downstreams.yml` — manual-dispatch workflow that
    opens dep-bump PRs in blindfox and er (becomes automatic once
    those repos migrate off `github.com/tommie/v8go`).
* `MAINTAINING.md` documenting required secrets and branch protection.

### Fixed

The Phase B integration surfaced three V8 14.x process-state bugs in
the binaries shipped via `tommie/v8go/deps/*`; all are now mitigated
on the wrapper side:

* Concurrent isolate construction is serialised process-wide.
* `SnapshotCreator` requires single-OS-thread affinity.
* Embedder `Global<Template>` handles are drained before `CreateBlob`,
  but only on isolates that opt in via `m_ctx.track_templates` so
  regular isolates do not race on the tracking vector.

`goFunctionCallback` also no longer nil-derefs when a snapshotted
callback ref is invoked without being re-registered on the consumer
side; it surfaces a JS-side error instead.

### Module path

The module path is `github.com/ChessCom/v8go`. The platform-specific
binary subpackages still live under `github.com/tommie/v8go/deps/*`
so existing `libv8.a` artefacts are reused without rebuild.
