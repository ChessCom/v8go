# Performance

This document covers cold-start benchmarks, memory management,
deterministic snapshots, and snapshot sizing trade-offs.

## Cold start: source vs snapshot

The primary performance feature of this fork is snapshot-based cold
start. Instead of parsing and compiling JavaScript on every isolate
boot, a snapshot blob is deserialized directly into V8's heap.

### Benchmark setup

`snapshot_bench_test.go` defines two benchmarks and an assertion test:

| Benchmark | What it measures |
|---|---|
| `BenchmarkColdStart_FromSource` | `NewIsolate` + `NewContext` + `RunScript(bundle)` |
| `BenchmarkColdStart_FromSnapshot` | `NewIsolate(WithSnapshotBlob)` + `NewContext` + probe |
| `TestSnapshot_ColdStartSpeedup` | Asserts >= 3.5x speedup factor |

The synthetic bundle is ~750 KiB of JavaScript (15,000 named arrow
functions plus a coordinator). At this size, V8's source parsing and
IC warmup dominate the cold-start wall clock.

### Typical results (M-class Apple Silicon)

| Path | Avg cold start | Notes |
|---|---|---|
| From source | ~15 ms | Parse + compile + execute |
| From snapshot | ~4 ms | Deserialize heap |
| **Speedup** | **~3.5–6x** | Bounded by per-isolate setup floor |

The speedup factor is bounded below by V8's per-isolate setup cost
(~1.5 ms on local hardware, ~3 ms on cloud ARM), which is the same on
both paths. Even very large bundles plateau around 4–6x because the
setup floor dominates.

### Running benchmarks

```bash
go test -bench=BenchmarkColdStart -benchmem -count=5
```

To run the speedup assertion (skipped in `-short` mode):

```bash
go test -run TestSnapshot_ColdStartSpeedup -v -count=1
```

### Other benchmarks

| Benchmark | File | What it measures |
|---|---|---|
| `BenchmarkIsolateInitialization` | `isolate_test.go` | Raw `NewIsolate` + `Dispose` cost |
| `BenchmarkIsolateInitAndRun` | `isolate_test.go` | Isolate + context + simple script |
| `BenchmarkContext` | `context_test.go` | `NewContext` + `Close` on a shared isolate |

## Memory management

### Resource constraints

`WithResourceConstraints` sets V8 heap limits on an isolate:

```go
iso := v8.NewIsolate(v8.WithResourceConstraints(
    8 * 1024 * 1024,   // initial heap: 8 MiB
    16 * 1024 * 1024,  // max heap: 16 MiB
))
```

When the hard limit is reached, V8 calls `TerminateExecution`
internally, causing `RunScript` to return an
`ExecutionTerminated` error. The isolate remains usable after
termination.

### Heap statistics

`GetHeapStatistics()` returns a snapshot of the isolate's memory
usage:

```go
stats := iso.GetHeapStatistics()
fmt.Printf("used: %d, limit: %d\n",
    stats.UsedHeapSize, stats.HeapSizeLimit)
```

Key fields: `TotalHeapSize`, `UsedHeapSize`, `HeapSizeLimit`,
`MallocedMemory`, `ExternalMemory`.

### Per-isolate overhead

Each isolate carries a baseline memory cost:

| Component | Approximate size |
|---|---|
| V8 heap (empty) | ~2 MiB |
| Embedded builtins (shared) | ~700 KiB (shared across isolates) |
| Go-side state | ~1 KiB (maps, finalizers) |

Snapshot-based isolates add the deserialized heap objects on top of
the baseline. A ~750 KiB source bundle typically produces a ~200 KiB
snapshot blob that inflates to ~1 MiB of live heap objects.

## Deterministic snapshots

Non-deterministic APIs (`Date.now()`, `Math.random()`,
`performance.now()`) bake the host's wall-clock and PRNG state into
the snapshot heap if called during bundle evaluation. This makes
snapshots non-reproducible and can cause user-facing bugs (e.g.
stale timestamps rendered on first load).

### The determinism shim

`WithDeterministicTime` installs a JavaScript prelude that replaces:

| API | Replacement |
|---|---|
| `Date.now()` | Fixed timestamp from `SeedTimeMillis` (default: 2024-01-01T00:00:00Z) with monotonic tick |
| `Math.random()` | Mulberry32 PRNG seeded from the timestamp |
| `performance.now()` | Monotonic counter starting at 0 |

The tick counter increments on each call so back-to-back reads are
strictly monotonic rather than frozen.

### Usage

At snapshot creation:

```go
packed, _ := v8.PackBundle(v8.PackOptions{
    Source:            src,
    DeterministicTime: true,
    SeedMillis:        v8.SeedTimeMillis,
})
```

At restoration, strip the shim to restore live behaviour:

```go
iso, _ := packed.RestoreIsolate(v8.RestoreOptions{})
ctx := v8.NewContext(iso)
v8.ResetNonDeterminism(ctx)
```

After `ResetNonDeterminism`, `Date.now()` returns the real wall clock,
`Math.random()` returns V8's intrinsic random, and `performance.now()`
returns the intrinsic high-resolution timer.

### Reproducibility guarantee

Two `PackBundle` calls with the same `Source`, `SeedMillis`, and
`FCH` produce byte-identical snapshot blobs, regardless of the host's
wall clock or entropy source. This enables:

- Build cache keying by `BundleSHA256`
- Reproducible CI artifacts
- Snapshot diffing for debugging

## Snapshot sizing

### FunctionCodeHandling

The `FCH` parameter controls a fundamental trade-off:

| Mode | Blob size | Cold-start CPU | Determinism |
|---|---|---|---|
| `FunctionCodeKeep` | Larger | Minimal (bytecode ready) | More deterministic |
| `FunctionCodeClear` | Smaller | Higher (recompile on first call) | Less deterministic |

**`FunctionCodeKeep`** preserves compiled bytecode and baseline code
in the blob. Functions are immediately callable after restoration
without any compilation step. Use this for latency-sensitive paths.

**`FunctionCodeClear`** strips compiled code, keeping only source
text and parser state. Functions are recompiled by V8 on first
invocation. Use this when blob size matters (e.g. transmitting
snapshots over the network) or when the snapshot must survive minor
V8 version skew (compiled code is more version-sensitive than AST
state).

### Typical blob sizes

| Bundle size (source) | FunctionCodeKeep | FunctionCodeClear |
|---|---|---|
| 50 KiB | ~150 KiB | ~80 KiB |
| 750 KiB | ~2 MiB | ~800 KiB |
| 2 MiB | ~6 MiB | ~2.5 MiB |

These are approximate. Actual sizes depend on the ratio of function
bodies to data in the bundle.

## Isolate lifecycle cost

### Serial construction

All `NewIsolate` calls are serialised by `snapshotDeserMu`. This is
required because V8 13.6's shared-heap initialiser
(StringForwardingTable, ReadOnlyHeap) is not thread-safe when two
isolates are being constructed concurrently.

The practical throughput ceiling for isolate construction is:

```
max isolates/sec ≈ 1000 / (avg_construction_ms)
                 ≈ 1000 / 4
                 ≈ 250 isolates/sec  (from snapshot, M-class)
```

For services that need higher throughput, use an isolate pool (see
[zero-cold-start.md](zero-cold-start.md)) to decouple request
handling from isolate construction.

### Disposal

`Isolate.Dispose()` is also serialised by `snapshotDeserMu` because
V8's shared-heap teardown races with construction. Disposal is fast
(sub-millisecond) but blocks any concurrent construction.

### Recommendation

For request-per-isolate architectures:

1. Use `PackBundle` + `RestoreIsolate` for the snapshot path.
2. Size the isolate pool to match expected concurrency.
3. Set `WithResourceConstraints` to bound heap growth.
4. Monitor `GetHeapStatistics` to detect memory leaks in long-lived
   isolates.

For long-lived isolates (one isolate, many requests):

1. Use `TerminateExecution` with timeouts to prevent runaway scripts.
2. Use `PerformMicrotaskCheckpoint` to drain the microtask queue
   between requests.
3. Monitor `NumberOfDetachedContexts` for context leaks.
