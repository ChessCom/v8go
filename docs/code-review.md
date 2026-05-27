# Code Review

This document defines the adversarial code review procedure for
ChessCom/v8go, including checklists for memory safety, performance
regression gates, and architecture drift guards. It is the reference
procedure used by the automated PR review trigger and by human
reviewers.

## Adversarial review procedure

Every PR is reviewed with the assumption that it introduces a
correctness or safety bug until proven otherwise. The procedure was
established during PR #11 (SnapshotCreator.FreshContext) and is
codified here for repeatability.

### Step 1: Gather full context

```bash
gh pr diff <number> --repo ChessCom/v8go
gh pr view <number> --repo ChessCom/v8go --json title,body,commits,files,additions,deletions
```

Read every touched file **in full** — not just the diff hunk. Load at
least 200 lines of surrounding context for struct definitions, lifetime
ownership chains, and callers of any modified function.

### Step 2: C++ layer audit

Run every item in this checklist against the diff. Flag violations at
the specific line.

| ID | Check | Severity if violated |
|---|---|---|
| C1 | `ToLocalChecked()` / `.Check()` — must use `ToLocal()` + error propagation; these crash the host process on failure | CRITICAL |
| C2 | Null pointer guards on every `extern "C"` function parameter | HIGH |
| C3 | `HandleScope` / `Context::Scope` correctness — the entered context must match the context passed to `Get` / `Set` / `Call` | HIGH |
| C4 | `iso->SetData(N)` slot consistency — slot 0 = scaffolding `m_ctx*`, slot 1 = `StartupData*`, slot 2 = embedder `m_ctx*`; any new `m_ctx*` must be installed in the correct slot | HIGH |
| C5 | New `Global<>` handle must have a matching `Reset()` in `SnapshotCreatorReleaseEmbedderHandles` | HIGH |
| C6 | `Persistent::Reset()` before `delete` — redundant (destructor calls Reset); note but do not flag as blocking | LOW |
| C7 | No `new` without a corresponding `delete` on every exit path; no `delete` without clearing all external references to the freed memory | CRITICAL |
| C8 | Cross-context property access: `Object::Get(ctxA, key)` while `Context::Scope` is entered for ctxB — fragile under V8 upgrades if the property has accessors or interceptors | HIGH |
| C9 | V8 API version sensitivity: check whether the called V8 API has changed semantics between V8 13.x and 14.x (consult `deps/include/` headers) | MEDIUM |

### Step 3: Go layer audit

| ID | Check | Severity if violated |
|---|---|---|
| G1 | Use-after-free: when C++ frees `m_value*` / `m_ctx*`, Go `*Value` / `*Context` handles must be invalidated or documented as dangling | CRITICAL |
| G2 | `ctxRegistry` leaks — every `register()` needs a matching `deregister()` on all exit paths (including error returns and `Dispose`) | HIGH |
| G3 | `closeMu` lock discipline — held for the entire C call; no mutex nesting that could deadlock | HIGH |
| G4 | `runtime.KeepAlive` on every CGO argument that could be GC'd mid-call | HIGH |
| G5 | Determinism shim reinstallation — if a context is replaced (e.g. `FreshContext`), the shim must be reinstalled or the omission documented | MEDIUM |
| G6 | `runtime.SetFinalizer` on any new wrapper struct that holds a C pointer | MEDIUM |
| G7 | Error returns must not leave the object in a half-initialized state accessible to the caller | HIGH |

### Step 4: Test coverage audit

| ID | Check | Severity if violated |
|---|---|---|
| T1 | Happy path: at least one end-to-end round-trip (create → serialize → deserialize → use) | Required |
| T2 | Error paths: double call, post-consumed call, nil inputs | Required |
| T3 | Adversarial inputs: invalid UTF-8, `"__proto__"`, symbol keys, frozen globals, duplicates | Required for any API that accepts user-controlled strings |
| T4 | Use-after-free paths: old handles used after teardown | Required when C++ frees tracked objects |
| T5 | Concurrent access: if the new API is callable from multiple goroutines | Required for public API |
| T6 | Benchmark impact: if the PR touches hot paths (snapshot, isolate, context creation) | Required |

### Step 5: Post the review

Post via the GitHub API with inline comments on specific diff lines
plus a summary severity table:

```bash
gh api repos/ChessCom/v8go/pulls/<number>/reviews \
  --input review.json
```

Use event `"COMMENT"` (not `"REQUEST_CHANGES"` which GitHub blocks on
your own PRs). The JSON body contains:

- `body`: summary table mapping severity to issue
- `comments[]`: one entry per finding with `path`, `line`, `side: "RIGHT"`, `body`

## Performance regression gates

### Existing benchmarks

| Benchmark | File | Measures |
|---|---|---|
| `BenchmarkColdStart_FromSource` | `snapshot_bench_test.go` | NewIsolate + NewContext + RunScript(~750 KiB bundle) |
| `BenchmarkColdStart_FromSnapshot` | `snapshot_bench_test.go` | NewIsolate(WithSnapshotBlob) + NewContext + probe |
| `BenchmarkIsolateInitialization` | `isolate_test.go` | Raw NewIsolate + Dispose |
| `BenchmarkIsolateInitAndRun` | `isolate_test.go` | Isolate + context + simple script |
| `BenchmarkContext` | `context_test.go` | NewContext + Close on shared isolate |

### Speedup assertion tests

| Test | File | Threshold |
|---|---|---|
| `TestSnapshot_ColdStartSpeedup` | `snapshot_bench_test.go` | >= 3.5x |
| `TestSnapshotESM_ColdStartSpeedup` | `snapshot_esm_test.go` | >= 2.5x |

These are skipped under `-short` and under coverage profiling.

### Review protocol for performance-sensitive PRs

A PR is performance-sensitive if it touches any of: `snapshot.cc`,
`snapshot.go`, `pack.go`, `isolate.cc`, `isolate.go`, `context.cc`,
`context.go`, `arraybuffer.cc`, `arraybuffer.go`, `fast_api.cc`,
`fast_api.go`.

| Step | Command | Gate |
|---|---|---|
| Baseline benchmarks (before) | `go test -bench=BenchmarkColdStart -benchmem -count=5` | Record ns/op, B/op, allocs/op |
| Apply PR changes | — | — |
| Post-change benchmarks | `go test -bench=BenchmarkColdStart -benchmem -count=5` | Compare against baseline |
| Blob size check | Read `t.Logf("blob size: %d bytes", len(blob))` from test output | Must not regress >5% without justification |
| Speedup assertions | `go test -run TestSnapshot_ColdStartSpeedup -v -count=1` | Must pass (>= 3.5x) |
| ESM speedup assertions | `go test -run TestSnapshotESM_ColdStartSpeedup -v -count=1` | Must pass (>= 2.5x) |
| Allocation check | Compare `-benchmem` allocs/op | No new allocations in hot path without justification |

### CI coverage

CI enforces >= 93% total coverage on ubuntu-latest. ESM snapshot tests
run 3x (`-count=3`) for flake detection. There is no automated
benchmark comparison in CI yet; reviewers must run benchmarks locally.

## Architecture drift guards

### Canonical layers

```
Go public API  →  C shim (*.h / *.cc)  →  V8 C++ API  →  prebuilt libv8-*.a
```

Every C++ function is `extern "C"` in a `.h` header and implemented in
the matching `.cc` file. Go calls them via CGO. The C shim translates
between opaque C pointers (`IsolatePtr`, `ContextPtr`, `ValuePtr`) and
V8 C++ objects.

### Invariants

These are load-bearing properties of the system. A PR that violates
any of them must be flagged as architecture drift.

**INV-1: One SnapshotCreator at a time.** `snapshotCreatorMu` is held
from `NewSnapshotCreator` until `Dispose`. New APIs must not bypass
this lock or create additional SnapshotCreator instances.

**INV-2: External references frozen before first snapshot.**
`extref.go` freezes the registry on first use (sorted by name, sha256
digest). The digest must match at create and restore. A PR that adds
a new external reference must do so before any snapshot operation.

**INV-3: Isolate data slot allocation.** V8 isolate data slots are a
shared resource:

| Slot | Contents | Set by |
|---|---|---|
| 0 | `m_ctx*` scaffolding (template tracking, isolate-level context) | `NewSnapshotCreator`, `NewIsolate` |
| 1 | `StartupData*` (existing snapshot blob pointer for cleanup) | `NewSnapshotCreator`, `NewIsolate` |
| 2 | `m_ctx*` embedder context (registered via `AddContext`) | `SnapshotCreatorAddContext` |

A PR that uses a new slot (3+) or reassigns an existing slot must
document the change and verify no other code path reads the old value.

**INV-4: Context::FromSnapshot(iso, 0) for embedder context.**
`SnapshotCreatorAddContext` must return index 0. If it returns
non-zero, `CreateBlob` fails. A PR that adds a second context must
use index 1+ and update the consumer-side `NewContext` to pass the
correct index.

**INV-5: Thread pinning for SnapshotCreator.**
`runtime.LockOSThread()` is called in `NewSnapshotCreator` and
`runtime.UnlockOSThread()` in `Dispose`. V8 requires all API calls
against the creator's isolate to happen on the thread that called
`Isolate::Enter`. A PR that adds async work inside the creator
lifecycle must not release the OS thread.

**INV-6: Concurrency serialisation.** Locking order:

```
snapshotCreatorMu  >  snapshotDeserMu  >  extrefMu  >  ctxMutex  >  iso.cbMutex
```

A PR that adds a new mutex must document where it fits in this order.

### Drift signals

Flag these patterns during review — they indicate potential drift:

- New `iso->SetData(N, ...)` call with N not in {0, 1, 2}
- New `extern "C"` function without null guards on every pointer parameter
- New `Global<>` handle without a matching `Reset()` path in `SnapshotCreatorReleaseEmbedderHandles`
- New mutex without documented locking order relative to the six above
- New build tag without CI matrix coverage (currently: `leakcheck`)
- New platform-specific code without matching `cgo_*.go` file
- New V8 API call not present in `deps/include/` headers (version mismatch)
- `delete` of a C++ struct without clearing the corresponding Go pointer to nil
- `runtime.SetFinalizer` on a struct that can outlive its C++ backing

### Downstream compatibility

Two internal repositories consume this fork:

| Downstream | CI check |
|---|---|
| ChessCom/blindfox | `go mod edit -replace` + `go build` + `go test -short` |
| ChessCom/er | `go mod edit -replace` + `go build` + `go test -short` |

These run in CI when `CROSS_REPO_READ_TOKEN` is configured. A PR that
changes the public API surface must not break downstream compilation.

## Severity scale

| Severity | Definition | Action |
|---|---|---|
| CRITICAL | Memory safety violation, host process crash, data corruption | Must fix before merge |
| HIGH | Correctness bug, fragile pattern, missing safety guard | Must fix or provide written justification |
| MEDIUM | Missing test coverage, undocumented invariant, style inconsistency | Should fix; merge at maintainer discretion |
| LOW | Redundant code, misleading comment, minor inefficiency | Note for author; does not block merge |
