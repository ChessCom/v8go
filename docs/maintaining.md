# Maintaining the ChessCom/v8go fork

This document describes the ongoing maintenance burden of keeping this
fork alive and in sync with its upstream, [tommie/v8go](https://github.com/tommie/v8go).

## Upstream synchronisation

A weekly GitHub Actions workflow
([upstream-sync.yml](../.github/workflows/upstream-sync.yml)) fetches
`tommie/v8go` master every Monday at 06:00 UTC and opens a merge PR
when new commits are found. The PR title includes the commit count so
reviewers know whether it is a trivial bump or a V8-version upgrade.

### What to expect

| Upstream change type | Typical effort |
|---|---|
| Test-only or doc-only | Auto-merge after CI passes |
| Go API additions | Review for compatibility with our downstream consumers |
| V8 minor version bump (13.6 → 13.7) | Rebuild deps modules, re-run snapshot tests, validate ABI stability |
| V8 major version bump (13.x → 14.x) | Likely merge conflicts in fork-specific C++; full regression pass on blindfox and er |
| C++ header restructuring | Manual resolution of `snapshot.{h,cc}` and `isolate.{h,cc}` conflicts |

### Resolving merge conflicts

The fork-specific C++ files are the most common source of conflicts:

| Fork file | Upstream counterpart | Why they conflict |
|---|---|---|
| `snapshot.h` / `snapshot.cc` | _does not exist upstream_ | Upstream may rename or restructure headers that `snapshot.cc` includes |
| `isolate.h` / `isolate.cc` | same | Fork adds GC/memory pressure exports, `NewIsolateNoDefaultHeapCB`, `SnapshotCreatorPtr` typedefs |
| `v8go.h` / `v8go.cc` | same | Fork registers additional C exports for the snapshot bridge |
| `object.h` / `object.cc` | same | Fork adds `ObjectGetPropertyNames`, `ObjectGetOwnPropertyNames`, `ObjectGetPrototype`, `ObjectSetPrototype` |
| `heap_limit.h` / `heap_limit.cc` | _does not exist upstream_ | Custom near-heap-limit callback trampoline |
| `promise_reject.h` / `promise_reject.cc` | _does not exist upstream_ | Promise reject callback trampoline |
| `interrupt.h` / `interrupt.cc` | _does not exist upstream_ | Interrupt termination + SetIdle wrappers |
| `gc_callback.h` / `gc_callback.cc` | _does not exist upstream_ | GC prologue/epilogue callback trampolines |
| `arraybuffer.h` / `arraybuffer.cc` | _does not exist upstream_ | ArrayBuffer creation (copy, alloc, external) with sandbox fallback |
| `fast_api.h` / `fast_api.cc` | _does not exist upstream_ | V8 Fast API CFunctionInfo builder |
| `fast_api_test_helpers.cc` | _does not exist upstream_ | Test-only C++ fast function for Fast API tests |
| `fast_api_test_export.go` | _does not exist upstream_ | CGo bridge to test fast function address |
| `isolate_registry.go` | _does not exist upstream_ | Global Go-side isolate registry for CGO callback dispatch |
| `deps/*/cgo.go` | _does not exist upstream_ | Platform-specific CGo linker directives for local V8 archives |
| `deps/*/go.mod` | _does not exist upstream_ | Go module definition for each platform dep |
| `deps/*/vendor.go` | _does not exist upstream_ | Empty Go file to make platform dep a valid package |
| `.gitattributes` | _does not exist upstream_ | Marks `*.a` as binary to prevent git line-ending corruption |
| `Makefile` | _does not exist upstream_ | Convenience targets for local V8 builds |
| `deps/Dockerfile.builder` | _does not exist upstream_ | Docker image for local Linux V8 builds |
| `deps/build-all-local.sh` | _does not exist upstream_ | Orchestration script for local V8 builds |

When conflicts appear, resolve them locally, ensure the CI matrix
passes on both ubuntu-latest and macos-latest, and push to the sync
branch. If conflicts are non-trivial, tag a second reviewer.

**Important**: fork-only `.h`/`.cc` files (heap_limit, promise_reject,
interrupt, gc_callback) will never conflict with upstream directly, but
they may break if upstream renames V8 headers they include or changes
the V8 API signatures they call. When upstream bumps V8, compile all
fork-only C++ files first to catch breakage early.

### Skipping a sync

Occasionally an upstream merge introduces a V8 binary change that is
not yet mirrored in the deps modules. In that case, close the
automated PR with a comment explaining the skip and reopen manually
once the deps are updated.

## V8 rebuild via GitHub Actions

The [`build-v8-deps.yml`](../.github/workflows/build-v8-deps.yml)
workflow rebuilds V8 monolith from source for all 4 platforms. It is
triggered manually via `workflow_dispatch` and optionally opens a PR
with the rebuilt archives.

### Matrix

| Platform | Runner | Compiler | Cross-compile? | Special flags |
|----------|--------|----------|----------------|---------------|
| linux/amd64 | `ubuntu-latest` | gcc (`is_clang=false`) | No | -- |
| linux/arm64 | `ubuntu-latest` | V8 clang | Yes (x86\_64 host) | `use_sysroot=true`, `use_custom_libcxx=true` |
| darwin/arm64 | `macos-latest` | V8 clang | No | -- |
| darwin/amd64 | `macos-latest` | V8 clang | Yes (arm64 host) | `use_custom_libcxx=true` |

Build time is ~60-90 minutes per platform.

### Key GN args

All platforms share these V8 build flags:

- `v8_enable_sandbox=false` -- enables true zero-copy ArrayBuffer
- `v8_monolithic=true` -- single static library
- `use_thin_lto=false` -- avoids LLVM bitcode in archives
- `v8_embedder_string="-v8go"` -- identifies fork builds

### Why linux/amd64 uses gcc

V8's bundled clang produces ELF objects with features (section types,
relocation formats) that GNU `ld` on Ubuntu cannot parse, resulting in
`unknown architecture of input file` errors. Building with the system
gcc produces standard GNU-compatible ELF objects.

### Why cross-compiles need `use_custom_libcxx`

When cross-compiling, the host system's C++ standard library headers
may lack C++20/23 features that V8 requires (e.g. `std::bit_cast`,
`std::make_unique_for_overwrite`). Using V8's bundled libc++ avoids
these compatibility issues.

### Archive splitting

GitHub limits individual files to ~100 MB. The `split_ar` step in
`deps/build.py` splits `libv8_monolith.a` (~100-140 MB) into chunks
under 40 MB each (`libv8-0.a`, `libv8-1.a`, etc.). A `libmanifest`
file lists the split archives for `update_cgo.py`.

On Linux, the split step forces use of the system `ar` (GNU ar)
instead of `llvm-ar` because GNU `ld` cannot reliably link archives
with LLVM's SysV64 symbol table format.

### Assemble job

After all 4 platform builds succeed, the `assemble` job:

1. Downloads all platform artifacts
2. Runs `deps/update_cgo.py` to regenerate CGo files
3. Updates `go.mod` with local `replace` directives
4. Strips `-DV8_ENABLE_SANDBOX` from `cgo.go`
5. Opens a PR on the `rebuild-v8-deps-no-sandbox` branch

### Workflow inputs

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `create_pr` | boolean | `true` | Open a PR with rebuilt deps |

## V8 rebuild locally (escape hatch)

When CI is unavailable, too slow, or you need to iterate on V8 build
flags locally, use the local build system. It builds V8 monolith for
all 4 platforms from a single macOS (Apple Silicon) machine.

### Prerequisites

- macOS with Xcode Command Line Tools (`xcode-select --install`)
- Docker Desktop with "Use Rosetta for x86\_64/amd64 emulation" enabled
- ~20 GB free disk (V8 source + build artifacts)
- ~16 GB RAM recommended

### Quick start

```bash
# Build all 4 platforms (~45 min with parallelism)
make v8-deps-all

# Or build a single platform
make v8-deps-darwin-arm64
make v8-deps-linux-amd64
```

### How it works

The orchestration script (`deps/build-all-local.sh`) handles:
1. Fetching `depot_tools` and V8 source (first run only, cached after)
2. Building darwin targets natively (cross-compile for amd64)
3. Building linux targets in Docker containers
4. Regenerating `cgo_*.go` files via `update_cgo.py`

| Target | Method |
|--------|--------|
| darwin/arm64 | Native build via V8's bundled clang |
| darwin/amd64 | Native cross-compile (`target_cpu="x64"`) |
| linux/arm64 | Docker `--platform linux/arm64` (native on Apple Silicon) |
| linux/amd64 | Docker `--platform linux/amd64` (Rosetta emulation, ~2x slower) |

The V8 source (~10 GB) is fetched once to `deps/v8/` and shared
across all builds (bind-mounted into Docker containers).

### After building

```bash
# Remove sandbox define (if rebuilding to disable sandbox)
sed -i '' 's/ -DV8_ENABLE_SANDBOX//' cgo.go

# Update go.mod to use local deps
for p in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  go mod edit -droprequire="github.com/tommie/v8go/deps/$p" || true
  go mod edit -require="github.com/ChessCom/v8go/deps/${p}@v0.0.0"
  go mod edit -replace="github.com/ChessCom/v8go/deps/${p}=./deps/${p}"
done
go mod tidy

# Verify
go build ./...
go test -count=1 -timeout 5m ./...
```

### Files involved

| File | Purpose |
|------|---------|
| `deps/build-all-local.sh` | Orchestration script |
| `deps/build.py` | Per-platform V8 build (gclient, gn, ninja) |
| `deps/Dockerfile.builder` | Docker image for linux builds (Ubuntu 24.04) |
| `deps/update_cgo.py` | Regenerates cgo Go files from libmanifest |
| `Makefile` | Convenience targets (`v8-deps-*`) |

## Platform-specific linker notes

### Linux

The fork's V8 archives link with Go's CGo toolchain, which invokes
the system `gcc`/`g++` and ultimately GNU `ld`. Several compatibility
constraints apply:

- **gcc for linux/amd64:** V8's clang produces ELF objects with
  features GNU `ld` cannot parse. The linux/amd64 build uses gcc
  (`is_clang=false`) to produce standard GNU ELF objects.
- **`--start-group` / `--end-group`:** gcc-built V8 archives have
  cross-archive symbol dependencies that require group linking. The
  `cgo.go` for linux platforms wraps archives with
  `-Wl,--start-group ... -Wl,--end-group`.
- **`use_thin_lto=false`:** ThinLTO produces LLVM bitcode objects
  that GNU `ld` cannot link. Disabled for all platforms.
- **System `ar` for archive splitting:** On Linux, the `split_ar`
  step uses GNU `ar` instead of `llvm-ar` to produce archives with
  standard symbol tables that GNU `ld` can read.
- **System libraries:** `-ldl -lm -lstdc++`

### Darwin

- No linker group needed (ld64 handles cross-archive references).
- System libraries: `-lc++ -framework CoreFoundation`
- V8's bundled clang is used for all darwin builds.

## V8 binary upgrades

The fork maintains its own V8 static libraries under `deps/{os}_{arch}/`
with `v8_enable_sandbox=false`. When upgrading the V8 version:

1. Update `deps/v8_hash` with the new V8 commit hash.
2. Run the `build-v8-deps` workflow (or `make v8-deps-all` locally)
   to rebuild all 4 platforms.
3. Every CGO call site must be re-validated: the V8 C++ API is not
   ABI-stable across versions, and header renames or signature changes
   can break compilation.
4. Snapshot blobs are version-locked — a blob produced by V8 13.6
   cannot be loaded by V8 13.7. The `PackedSnapshot.V8ABI` field
   catches this at runtime, but any cached blobs in production must be
   regenerated after an upgrade.

### Verification checklist

- [ ] `go build ./...` passes on linux/amd64 and darwin/arm64
- [ ] `go test -count=1 -timeout 5m ./...` passes on both platforms
- [ ] `snapshot_bench_test.go` benchmarks complete without assertion failures
- [ ] `compat-blindfox` and `compat-er` CI jobs pass

## CGO surface drift

The fork extends the upstream C bridge across several areas.

### Snapshot system (`snapshot.h` / `snapshot.cc`)

- `NewSnapshotCreator`
- `SnapshotCreatorGetIsolate`
- `SnapshotCreatorAddContext`
- `SnapshotCreatorCreateBlob`
- `SnapshotCreatorDispose`
- `SnapshotCreatorFreeBlob`

### Isolate extensions (`isolate.h` / `isolate.cc`)

- `NewIsolateNoDefaultHeapCB` — creates isolate without the default near-heap-limit callback
- `IsolateLowMemoryNotification` — triggers full GC
- `IsolateMemoryPressureNotification` — signals memory pressure level
- `IsolateCancelTerminateExecution` — cancels pending termination
- `IsolateRequestGarbageCollectionForTesting` — forces GC (testing only)
- `IsolateContextDisposedNotification` — context disposal hint
- `IsolateRunIdleTasks` — drives incremental GC and idle work within a deadline

### Object extensions (`object.h` / `object.cc`)

- `ObjectGetPropertyNames` — enumerate all property names including prototype chain
- `ObjectGetOwnPropertyNames` — enumerate own property names only
- `ObjectGetPrototype` — get object prototype
- `ObjectSetPrototype` — set object prototype

### Heap limit callbacks (`heap_limit.h` / `heap_limit.cc`)

- `IsolateAddCustomNearHeapLimitCallback` — installs Go-side heap limit callback
- `IsolateRemoveCustomNearHeapLimitCallback` — removes custom callback

### Promise reject callbacks (`promise_reject.h` / `promise_reject.cc`)

- `IsolateSetPromiseRejectCallback` — installs promise rejection handler

### Interrupt and idle (`interrupt.h` / `interrupt.cc`)

- `IsolateRequestInterruptTerminate` — schedules termination via interrupt
- `IsolateSetIdle` — hints idle state to V8

### External memory and microtask control (`isolate.h` / `isolate.cc`)

- `IsolateAdjustExternalMemory` — reports Go-side allocations to V8's GC heuristic
- `IsolateSetMicrotasksPolicy` — controls microtask queue drain policy (Explicit/Scoped/Auto)
- `IsolateEnqueueMicrotask` — schedules a JS function as a microtask

### OOM error handler (`oom_handler.h` / `oom_handler.cc`)

- `IsolateSetOOMErrorHandler` — installs a Go callback for V8 out-of-memory events
- `IsolateClearOOMErrorHandler` — restores default abort-on-OOM behavior

### ArrayBuffer (`arraybuffer.h` / `arraybuffer.cc`)

- `NewArrayBufferFromBytes` — creates ArrayBuffer with copied data
- `NewArrayBufferAlloc` — allocates empty ArrayBuffer
- `NewArrayBufferExternal` — zero-copy external backing store (sandbox fallback to copy)
- `V8SandboxIsEnabled` — compile-time sandbox detection
- `ArrayBufferGetData` / `ArrayBufferGetByteLength` / `ArrayBufferGetBackingStore` — accessors

### Fast API (`fast_api.h` / `fast_api.cc`, `function_template.{h,cc}`)

- `BuildCFunctionInfo` — constructs `v8::CFunctionInfo` from C-level type array
- `FreeCFunctionInfo` — releases the info struct
- `NewFastFunctionTemplate` — creates FunctionTemplate with CFunction fast path

### External strings (`external_string.h` / `external_string.cc`)

- `NewExternalOneByteString` — creates string backed by external Go memory

### Named property interceptors (`interceptor.h` / `interceptor.cc`)

- `ObjectTemplateSetNamedPropertyHandler` — installs getter/setter interceptors
- Uses callback registry pattern via `Integer` data (same as FunctionTemplate)

### Heap profiler (`heap_profiler.h` / `heap_profiler.cc`)

- `IsolateTakeHeapSnapshot` — captures heap snapshot as JSON
- `HeapSnapshotDataFree` — frees the C-allocated snapshot buffer

### ES Modules (`module.h` / `module.cc`)

- `CompileESModule` — compiles ES module source
- `ModuleInstantiate` — instantiates with resolve callback trampoline
- `ModuleEvaluate` — evaluates the module
- `ModuleGetStatus` / `ModuleGetRequestsLength` / `ModuleGetRequest` — introspection
- `ModuleGetNamespace` — returns module namespace object
- `ModuleGetIdentityHash` — identity hash for module map keys
- `ModuleFree` — releases the Persistent<Module> handle

### GC callbacks (`gc_callback.h` / `gc_callback.cc`)

- `IsolateAddGCPrologueCallback` / `IsolateRemoveGCPrologueCallback`
- `IsolateAddGCEpilogueCallback` / `IsolateRemoveGCEpilogueCallback`

### Conflict resolution for API surface changes

When upstream modifies the signatures of existing C exports (e.g.
`NewIsolate`, `IsolateDispose`), the fork-specific additions must be
re-aligned. Follow this process:

1. **Identify affected files**: Grep for `// ChessCom:` comments in C++ files to locate fork-specific modifications.
2. **Check all fork-only headers**: Compile `heap_limit.cc`, `promise_reject.cc`, `interrupt.cc`, `gc_callback.cc` first — they include V8 headers and will break immediately if signatures changed.
3. **Verify callback trampolines**: If upstream changes `v8::Isolate` internals, verify that `_cgo_export.h` still generates correct signatures for `goNearHeapLimitCallback`, `goPromiseRejectCallback`, `goGCPrologueCallback`, `goGCEpilogueCallback`.
4. **Check data slot usage**: Slots 0 (m_ctx), 1 (snapshot blob), and 2 (embedder ctx in snapshot) are in use. If upstream claims new slots, renumber.
5. **Run full test suite**: `go test -count=1 -timeout 5m ./...` must pass. Pay special attention to `gc_test.go`, `heap_limit_test.go`, `promise_reject_test.go`, `interrupt_test.go`, `gc_callback_test.go`, `object_enum_test.go`.

## Downstream compatibility

Two internal repositories depend on this fork:

| Downstream | Import path | How it consumes v8go |
|---|---|---|
| **blindfox** | `github.com/tommie/v8go` (via `replace`) | V8 isolate management, snapshot creation, DOM JS execution |
| **er** | `github.com/tommie/v8go` (via `replace`) | Lightweight JS evaluation |

### CI guardrails

The `ci.yml` workflow runs two jobs — `ci (ubuntu-latest)` and
`ci (macos-latest)`. Each job runs lint, build, test + coverage, ESM
flake detection, and downstream compat checks sequentially. The compat
steps check out blindfox and er, swap in the PR's v8go via
`go mod edit -replace`, and run `go build` + `go test -short`.

Compat steps require the `CROSS_REPO_READ_TOKEN` secret. Without it
they skip gracefully — useful for external PRs from forks.

## Release process

See [docs/release.md](release.md) for the full release guide, including
versioning scheme, step-by-step flow, consumer integration, and
automation details.

## Repository secrets

| Secret | Used by | Scope |
|---|---|---|
| `CROSS_REPO_READ_TOKEN` | compat steps in `ci.yml` | `repo:read` on ChessCom/blindfox and ChessCom/er |

Should be a fine-scoped GitHub App token (preferred) or PAT with the
minimum scope above. Compat steps auto-skip when the secret is missing,
so PRs from forks degrade gracefully.

To rotate: create a new token with the same scope, paste it into
**Settings > Secrets and variables > Actions** under the matching name,
and delete the old one.

## Branch protection

Configure under **Settings > Branches > main** with:

- Require a pull request before merging (1 approval)
- Dismiss stale approvals on new commits
- Required status checks: `ci (ubuntu-latest)`, `ci (macos-latest)`
- Require branches to be up to date before merging
- Restrict pushes to maintainers only

## License headers

This fork does **not** carry license headers or a `LICENSE` file.
Upstream `tommie/v8go` ships BSD-style headers on every source file;
those are stripped during upstream syncs to keep the codebase clean.

When merging an upstream sync PR:

1. Run `grep -rl 'Use of this source code is governed by' --include='*.go' --include='*.h' --include='*.cc' . | grep -v '^./deps/'` to find any re-introduced headers.
2. Strip them: remove the 3-line `// Copyright … // found in the LICENSE file.` block and the trailing blank line.
3. If upstream re-adds a `LICENSE` file at the root, delete it.
4. Do **not** touch files under `deps/` — those are upstream V8 headers and must stay intact.

## Maintenance ceremonies

These ceremonies are automated via GNS triggers (see
[docs/gns.md](gns.md)) and can also be run manually. Each ceremony
has a matching GNS trigger that executes on Sunday mornings.

### Ceremony A: Weekly upstream sync review

**Trigger:** `user.yuri.triggers.v8go-upstream-check` (cron, Sunday 09:00 UTC)

The `upstream-sync.yml` workflow runs Monday 06:00 UTC and opens a
merge PR when new upstream commits exist. This ceremony triages it.

1. Check whether an open upstream sync PR exists (`gh pr list --label upstream-sync`).
2. Verify CI passed on both ubuntu-latest and macos-latest.
3. Inspect the diff for V8 header renames or API signature changes.
4. If V8 headers changed, compile fork-only C++ files first:
   ```bash
   go build ./...
   ```
5. Strip re-introduced license headers (see "License headers" above).
6. Merge if clean, or close with a comment explaining the skip reason.
7. Log the outcome to `user.yuri.public.v8go.maintenance-log` via GNS.

### Ceremony B: Weekly benchmark baseline

**Trigger:** `user.yuri.triggers.v8go-benchmark` (cron, Sunday 08:00 UTC)

1. Run cold-start benchmarks:
   ```bash
   go test -bench=BenchmarkColdStart -benchmem -count=5 -timeout 5m
   ```
2. Run speedup assertion tests:
   ```bash
   go test -run TestSnapshot_ColdStartSpeedup -v -count=1
   go test -run TestSnapshotESM_ColdStartSpeedup -v -count=1
   ```
3. Read previous baseline from GNS:
   ```bash
   gns get user.yuri.public.v8go.benchmarks.latest
   ```
4. Compare ns/op, B/op, allocs/op against previous. Flag regressions
   exceeding 10%.
5. Store new baseline in GNS:
   ```bash
   gns set user.yuri.public.v8go.benchmarks.latest --stdin < results.json
   ```
6. Notify via Slack DM if any regression exceeds the threshold.

### Ceremony C: Weekly V8 build sync check

**Trigger:** runs as part of the upstream check (Ceremony A)

1. Check `deps/v8_hash` against upstream for V8 version changes.
2. If V8 version changed and `build-v8-deps.yml` has not been run:
   - Flag for manual `build-v8-deps.yml` dispatch.
   - Verify all 4 platform archives after build completes.
3. Run full test suite + downstream compat checks.
4. Cut a release if the V8 build produced new archives.

### Ceremony D: Weekly documentation refresh

**Trigger:** runs as part of the full maintenance trigger
(`user.yuri.triggers.v8go-maintenance`, on-demand)

1. Verify docs match current code:
   - `docs/maintaining.md` — upstream sync, V8 rebuild, CGO surface
   - `docs/release.md` — versioning scheme, workflow jobs
   - `docs/architecture.md` — layer diagram, data slots, concurrency
   - `docs/performance.md` — benchmark numbers, speedup thresholds
2. Update GNS bootstrap if API surface changed:
   ```bash
   gns set user.yuri.public.v8go.bootstrap --stdin < bootstrap.md
   ```
3. Update GNS code-review if new audit checks are needed:
   ```bash
   gns set user.yuri.public.v8go.code-review --stdin < code-review.md
   ```
4. Verify `docs/performance.md` benchmarks match latest GNS baseline.
