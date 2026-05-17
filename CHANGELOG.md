# Changelog

All notable changes to this fork are documented in this file. Tags
follow `vMAJOR.MINOR.PATCH-chess.N`, where `MAJOR.MINOR.PATCH` mirrors
the upstream `tommie/v8go` version this fork tracks and `chess.N`
increments per ChessCom-side change set.

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
