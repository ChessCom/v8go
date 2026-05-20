# Toward true zero cold start

This document explores techniques that could eliminate the remaining
cold-start latency in v8go-based services, bringing isolate startup
from the current ~4 ms down to sub-1 ms — comparable to what
Cloudflare Workers achieves at production scale.

## Where we are today

The Pack/Restore API (`PackBundle` / `PackBundleESM` + `RestoreIsolate`)
gives a 3.5–6x speedup over parsing raw source at boot. On M-class
hardware a ~750 KiB synthetic bundle goes from ~15 ms (source eval) to
~4 ms (snapshot restore). Both IIFE/script bundles and ESM bundles are
supported — `PackBundleESM` evaluates modules inside the SnapshotCreator,
bridges the module namespace to a global, and serialises the heap.

The fork's V8 deps are now compiled with `v8_enable_sandbox=false`,
which removes the sandbox memory cage and enables true zero-copy
`NewArrayBufferExternal`. This eliminates one barrier from the
zero-copy data transfer path described below.

The breakdown of that 4 ms is roughly:

| Phase | Cost |
|---|---|
| `NewIsolate` (V8 heap allocation + shared-heap init) | ~1.5 ms |
| Snapshot deserialization (context + heap objects) | ~2.0 ms |
| Go-side bookkeeping (finalizers, mutex, map init) | ~0.5 ms |

Each phase is serialised: `snapshotDeserMu` ensures only one isolate
is constructed at a time, and `snapshotCreatorMu` prevents concurrent
snapshot creation. These mutexes protect against V8 14.x crashes in
the shared-heap initialiser but impose a throughput ceiling.

## Technique 1: isolate pool with pre-warmed snapshots

Maintain a pool of N pre-constructed isolates from the latest snapshot
blob. Incoming requests grab a warm isolate instead of constructing one.
The pool refills in background goroutines.

```
                          ┌─────────────┐
    request ──────────────► pooled iso  │─► execute ──► return to pool
                          └─────────────┘        or discard
                               ▲
                               │ background
                          ┌────┴────┐
                          │ refill  │ PackedSnapshot.RestoreIsolate()
                          └─────────┘
```

**Pros:** No per-request construction cost. Amortises the serial
construction bottleneck over idle time. Straightforward to implement
in Go with a `sync.Pool` or buffered channel.

**Cons:** Memory overhead — each idle isolate holds a V8 heap. Pool
sizing requires tuning: too small and requests stall waiting for a warm
isolate; too large and RSS grows. Snapshot updates require draining
and refilling the pool.

**Implementation sketch:**

```go
type IsolatePool struct {
    ready chan *v8go.Isolate
    snap  atomic.Pointer[v8go.PackedSnapshot]
}

func (p *IsolatePool) Get(ctx context.Context) (*v8go.Isolate, error) {
    select {
    case iso := <-p.ready:
        return iso, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (p *IsolatePool) refillLoop() {
    for {
        snap := p.snap.Load()
        iso, err := snap.RestoreIsolate(v8go.RestoreOptions{})
        if err != nil { continue }
        p.ready <- iso
    }
}
```

## Technique 2: TLS handshake prewarming

This is Cloudflare's published approach. When the first TLS
`ClientHello` packet arrives during the HTTPS handshake, the runtime
begins constructing the isolate before the HTTP request is decoded.
By the time the request handler runs, the isolate is ready.

```
    ClientHello ──► start isolate construction
                     │
    TLS handshake    │   (1.5–3 ms)
                     │
    HTTP request ──► isolate ready ──► execute
```

The TLS handshake itself takes 1–3 ms (round-trip for key exchange),
which overlaps almost perfectly with isolate construction time.

**Pros:** True zero perceived latency from the application's
perspective. No pool memory overhead.

**Cons:** Requires integration at the transport layer (TLS
termination). Only works for HTTPS; plain HTTP has no handshake to
exploit. The runtime must know which Worker (snapshot blob) to load
from the SNI hostname before the full request arrives. Not applicable
to all deployment topologies (e.g. if TLS is terminated at an upstream
load balancer).

**Applicability to v8go:** This technique is most valuable when v8go
is embedded in a Go HTTP server that terminates TLS itself. The
`crypto/tls` package exposes `GetConfigForClient` which fires on
`ClientHello` — a natural trigger point:

```go
tlsConfig := &tls.Config{
    GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
        go preWarmIsolate(hello.ServerName)
        return nil, nil
    },
}
```

## Technique 3: mmap-backed snapshot blobs

Currently `RestoreIsolate` passes a `[]byte` slice to V8's
`CreateParams.snapshot_blob`. V8 reads the snapshot data through a
`v8::StartupData` struct (`{const char* data; int raw_size}`), but
the blob is already copied into Go heap memory by `UnmarshalPackedSnapshot`.

Memory-mapping the snapshot file instead of reading it into a byte
slice has two benefits:

1. **No copy on load** — the OS maps pages lazily; only pages V8
   actually touches are faulted in.
2. **Shared pages** — multiple isolates restored from the same
   mmap'd file share physical pages through the OS page cache,
   reducing per-isolate RSS overhead.

**Implementation notes:**

```go
f, _ := os.Open("snapshot.bin")
data, _ := syscall.Mmap(int(f.Fd()), 0, size,
    syscall.PROT_READ, syscall.MAP_PRIVATE)

// data can be sliced and passed directly to WithSnapshotBlob
// since V8 only reads from the pointer during Isolate::New.
iso := v8go.NewIsolate(v8go.WithSnapshotBlob(data))
```

V8 reads the snapshot during `Isolate::New` and does not retain a
reference to the pointer after construction completes, so the mmap
can be safely unmapped after the isolate is live. However, pinning the
mmap for the process lifetime is simpler and enables the shared-pages
benefit.

**Estimated savings:** Eliminates the ~0.5 ms spent copying large
blobs (500 KiB – 2 MiB) from disk to Go heap. For very large
snapshots (5+ MiB), the savings are more significant.

## Technique 4: lazy deserialization

V8 supports lazy builtin deserialization (shipped in Chrome 64) where
built-in functions are deserialized on first call rather than at
isolate startup. This reduces initial deserialization time and
per-isolate memory footprint.

The relevant V8 flags:

- `--lazy-deserialization` (enabled by default since Chrome 64)
- `--lazy-handler-deserialization` (handler bytecodes)

**Verification:** Run with `--print-flag-values` on a debug build to
confirm these flags are active in the shipped deps binaries. If
disabled, they can be set via `v8go.SetFlags("--lazy-deserialization")`
before any isolate is created.

**Estimated savings:** 10–30% reduction in deserialization time for
typical bundles, depending on how many builtins the bundle actually
uses at startup.

## Technique 5: context snapshot stacking

v8go already supports `WithExistingSnapshotBlob` which layers a new
snapshot on top of a base blob. This enables a two-tier architecture:

```
    ┌─────────────────────────────────────┐
    │ Layer 2: app-specific state         │  (small, per-route)
    │   - route handler, config, preload  │
    ├─────────────────────────────────────┤
    │ Layer 1: base runtime               │  (large, shared)
    │   - polyfills, framework, stdlib    │
    └─────────────────────────────────────┘
```

The base runtime layer (Vue, React, polyfills) is large but identical
across all routes. It can be pre-snapshotted once and shared. The
per-route layer captures only the route-specific handler and
configuration, producing a small delta blob.

**Pros:** Isolates for different routes share the base runtime in their
snapshot lineage. V8 can skip deserializing objects already present in
the base layer.

**Cons:** V8's snapshot stacking does not currently support independent
base-layer sharing at the memory level — each isolate still gets its
own heap copy. The benefit is primarily in snapshot creation time
(build system can cache the base layer) and blob size (delta blobs are
smaller to transmit and store).

**Combined with mmap:** If the base-layer blob is mmap'd and shared
across isolates, the OS page cache effectively provides the memory
sharing that V8 itself does not.

## Technique 6: fork/COW isolate cloning

At the OS level, `fork()` duplicates a process with copy-on-write
pages. A process holding a warm V8 isolate could fork, and the child
would inherit the isolate's heap pages at near-zero cost — only pages
the child mutates would be physically copied.

```
    parent process                child process
    ┌──────────┐    fork()       ┌──────────┐
    │ warm iso │ ───────────────► │ warm iso │  (COW pages)
    │ V8 heap  │                 │ V8 heap  │
    └──────────┘                 └──────────┘
                                      │
                                 execute request
                                 (dirty pages copied)
```

**Constraints:**

1. **V8 thread-local state** — V8 uses thread-local storage (TLS keys)
   for per-isolate data. After `fork()`, the child inherits the
   parent's TLS but V8's internal thread registry is stale. V8 APIs
   called from the child may crash.
2. **Go runtime** — Go's runtime is hostile to `fork()` without
   `exec()`. Goroutine stacks, the scheduler, and the garbage
   collector are not fork-safe.
3. **File descriptors** — the child inherits all parent FDs (network
   sockets, log files), requiring careful `CLOEXEC` management.

**Verdict:** Not viable in a standard Go process. Could work in a
specialised C/C++ harness that forks before `cgo_init` and
communicates via shared memory or Unix sockets, but the complexity
is extreme. Listed for completeness.

## Technique 7: embedded builtins

V8's "embedded builtins" feature (shipped in Chrome 69) maps built-in
code into a shared, read-only memory segment. Instead of each isolate
carrying its own copy of ~700 KiB of builtin code, all isolates in the
process share a single mapped copy.

This is an _implicit_ savings — it does not speed up isolate
construction but reduces per-isolate memory overhead, enabling more
concurrent isolates within a given RSS budget.

**Verification:** Embedded builtins are active when the V8 binary
includes the `embedded.S` blob. Check with:

```bash
nm deps/linux_x86_64/libv8.a | grep v8_Default_embedded_blob_
```

If the symbols are present, embedded builtins are compiled in and
active by default.

**Estimated savings:** ~700 KiB per isolate on a 64-bit platform.
For a pool of 50 isolates, this is ~35 MiB of RSS reduction.

## Eliminating the serialisation mutex

The current `snapshotDeserMu` mutex serialises all `NewIsolate` calls
process-wide. This was added because V8 13.6's shared-heap
initialiser (string table forwarding, read-only heap shrink) crashes
when two isolates are constructed concurrently in the same process.

Potential paths to removing this bottleneck:

1. **Upgrade to V8 14.x+** — newer V8 builds may have fixed the
   shared-heap init race. Test by removing `snapshotDeserMu` on a
   debug build with ThreadSanitizer and running concurrent isolate
   construction under stress.

2. **Use `v8::Locker`** — V8's `Locker` API is designed for
   multi-threaded isolate access. If the shared-heap init code checks
   for a locker, using `v8::Locker(isolate)` during construction
   might eliminate the race without a Go-side mutex.

3. **Separate process per isolate** — move isolate construction into a
   subprocess (or pre-forked worker pool) so each process has a single
   isolate and the shared-heap is never contended. Communication via
   Unix sockets or shared memory.

**Impact of removal:** With the mutex gone, N isolates can be
constructed in parallel across N OS threads. Combined with an isolate
pool (Technique 1), this would eliminate the serial refill bottleneck
and achieve near-constant-time scaling.

## What "zero" actually means

"Zero cold start" is a marketing term. The practical target is:

> **Sub-1 ms p99** from request arrival to first JS instruction.

Cloudflare achieves this by combining TLS prewarming (Technique 2)
with a warm isolate pool and embedded builtins. Their published
numbers show Workers starting in "less than 5 milliseconds" with most
requests hitting pre-warmed isolates at sub-millisecond latency.

For a v8go-based Go service, the most practical path is:

1. **Isolate pool** (Technique 1) — eliminates per-request
   construction cost.
2. **mmap blobs** (Technique 3) — reduces pool refill cost and RSS.
3. **Lazy deserialization** (Technique 4) — speeds up per-isolate
   init during pool refill.
4. **TLS prewarming** (Technique 2) — masks any remaining pool-miss
   latency.

This combination should bring effective cold start below 1 ms for
the common case and below 4 ms for pool-miss worst case, while keeping
RSS proportional to actual concurrency rather than peak capacity.

## Summary

| Technique | Savings | Complexity | Status |
|---|---|---|---|
| Isolate pool | Eliminates per-request construction | Low | Ready to implement |
| TLS prewarming | Masks remaining construction time | Medium | Requires TLS integration |
| mmap blobs | Reduces copy + RSS overhead | Low | Ready to implement |
| Lazy deserialization | 10–30% faster init | Verify only | Check V8 flags |
| Context stacking | Smaller per-route blobs | Low | Already supported |
| fork/COW cloning | Near-zero cloning | Very high | Not viable in Go |
| Embedded builtins | ~700 KiB/isolate RSS | Verify only | Likely already active |
| Remove deser mutex | Parallel construction | Medium-High | Needs V8 14.x testing |
