package v8go

// #include <stdlib.h>
// #include "snapshot.h"
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// FunctionCodeHandling controls how compiled functions are encoded in the
// resulting startup blob. Mirrors v8::SnapshotCreator::FunctionCodeHandling.
type FunctionCodeHandling int

const (
	// FunctionCodeKeep preserves compiled bytecode/baseline code. This
	// minimises cold-start CPU at the cost of a larger blob and produces
	// strictly more deterministic output across runs.
	FunctionCodeKeep FunctionCodeHandling = 0

	// FunctionCodeClear drops compiled code, keeping only source + parser
	// state. Functions are recompiled on first call after restore. Use this
	// when the blob has to survive V8 micro-version skew.
	FunctionCodeClear FunctionCodeHandling = 1
)

// SnapshotCreatorOption configures a SnapshotCreator at construction time.
type SnapshotCreatorOption func(*snapshotCreatorConfig)

type snapshotCreatorConfig struct {
	existingBlob      []byte
	deterministicTime bool
	seedMillis        int64
}

// WithExistingSnapshotBlob warm-starts the SnapshotCreator from an existing
// blob. The new blob will be a strict superset; this lets embedders stack
// snapshots (e.g. base runtime + app bundle).
func WithExistingSnapshotBlob(blob []byte) SnapshotCreatorOption {
	return func(cfg *snapshotCreatorConfig) {
		cfg.existingBlob = blob
	}
}

// SnapshotCreator wraps v8::SnapshotCreator. It owns a dedicated Isolate
// that is consumed by CreateBlob. The intended lifecycle is:
//
//  1. sc := NewSnapshotCreator()
//  2. ctx := sc.Context()
//  3. ctx.RunScript("/* bundle source */", "bundle.js")
//  4. blob, err := sc.CreateBlob(v8go.FunctionCodeKeep)
//  5. defer sc.Dispose()
//
// After CreateBlob the isolate is gone — using sc.Isolate(), sc.Context() or
// the values produced from them is undefined behaviour. The Go wrapper
// guards against this with explicit error returns.
type SnapshotCreator struct {
	ptr    C.SnapshotCreatorPtr
	iso    *Isolate
	ctx    *Context
	pinned []byte                 // existing blob bytes pinned for the creator's lifetime
	cfg    *snapshotCreatorConfig // immutable after NewSnapshotCreator

	closeMu sync.Mutex
	created bool
	closed  bool
}

// ErrSnapshotCreatorConsumed is returned by SnapshotCreator methods that
// require a live underlying isolate, after CreateBlob has been called.
var ErrSnapshotCreatorConsumed = errors.New(
	"v8go: snapshot creator already consumed by CreateBlob",
)

// snapshotCreatorMu serialises every SnapshotCreator lifecycle in the
// process. v8::SnapshotCreator touches V8 read-only-heap state that is
// process-wide and not concurrency-safe: two SnapshotCreators alive in
// parallel goroutines reliably trigger a `Check failed: IsFreeSpaceOrFiller(filler)`
// fatal assertion in v8::internal::ReadOnlyHeap::OnCreateHeapObjectsComplete
// on the v8 13.6 binaries we ship. Holding this mutex from NewSnapshotCreator
// until Dispose guarantees no two creators are ever live concurrently and
// lets the embedder use the wrapper freely from any goroutine.
var snapshotCreatorMu sync.Mutex

// snapshotDeserMu serialises Isolate::New and Isolate::Dispose against
// each other AND against CreateCodeCache. The V8 binaries we ship via
// tommie/v8go/deps assert and abort when two isolates are being
// constructed in parallel because the shared-heap initialiser (string
// table forwarding, read-only heap shrink, etc) is not thread-safe in
// the absence of v8::Locker.
//
// RWMutex semantics:
//   - NewIsolate / Dispose take Lock (exclusive writer) because they
//     mutate V8 process-global shared-heap state.
//   - CreateCodeCache takes RLock (shared reader) because it reads
//     shared-heap objects (ReadOnlySpace roots, StringTable) that
//     Dispose can tear down concurrently.
//
// Multiple CreateCodeCache calls on different isolates remain parallel;
// they only block while an isolate is being created or disposed.
var snapshotDeserMu sync.RWMutex

// NewSnapshotCreator returns a new SnapshotCreator wired to the frozen
// process-wide external_references registry. The very first call to this
// function (or to any of the other registry-touching helpers) freezes the
// registry: subsequent AddExternalReference calls panic.
func NewSnapshotCreator(opts ...SnapshotCreatorOption) *SnapshotCreator {
	initializeIfNecessary()

	// v8::SnapshotCreator's constructor calls Isolate::Enter on the
	// calling OS thread. V8 requires every subsequent API call against
	// that isolate to happen on the same OS thread (else its internal
	// per-thread state is corrupted). Pin the calling goroutine to its
	// OS thread for the entire SnapshotCreator lifecycle.
	runtime.LockOSThread()

	// Acquire the process-wide snapshot lock first. We hold it until
	// Dispose so that no two creators are alive concurrently. Going past
	// initializeIfNecessary first means a hung Init never deadlocks the
	// mutex.
	snapshotCreatorMu.Lock()

	cfg := &snapshotCreatorConfig{}
	for _, o := range opts {
		o(cfg)
	}

	refs := frozenExtRefArray()

	var (
		dataPtr *C.char
		dataLen C.int
		pinned  []byte
	)
	if len(cfg.existingBlob) > 0 {
		pinned = append([]byte(nil), cfg.existingBlob...)
		dataPtr = (*C.char)(unsafe.Pointer(&pinned[0]))
		dataLen = C.int(len(pinned))
	}

	creator := C.NewSnapshotCreator(refs, dataPtr, dataLen)
	if creator == nil {
		panic("v8go: v8::SnapshotCreator construction failed")
	}

	isoPtr := C.SnapshotCreatorGetIsolate(creator)
	iso := &Isolate{
		ptr: isoPtr,
		cbs: make(map[int]FunctionCallbackWithError),
	}
	iso.null = newValueNull(iso)
	iso.undefined = newValueUndefined(iso)

	sc := &SnapshotCreator{
		ptr:    creator,
		iso:    iso,
		pinned: pinned,
		cfg:    cfg,
	}
	runtime.SetFinalizer(sc, (*SnapshotCreator).finalizer)
	return sc
}

// Isolate returns the SnapshotCreator-owned isolate. The caller MUST NOT
// call iso.Dispose() — that is the SnapshotCreator's responsibility.
// Returns nil after CreateBlob.
func (sc *SnapshotCreator) Isolate() *Isolate {
	sc.closeMu.Lock()
	defer sc.closeMu.Unlock()
	if sc.created {
		return nil
	}
	return sc.iso
}

// Context returns an embedder Context bound to the SnapshotCreator's
// isolate. The context is created lazily on first call. Subsequent calls
// return the same context. The returned context is also the context that
// Context::FromSnapshot(iso, 0) will recover on the consumer side.
func (sc *SnapshotCreator) Context() *Context {
	sc.closeMu.Lock()
	defer sc.closeMu.Unlock()
	if sc.created {
		return nil
	}
	if sc.ctx == nil {
		sc.ctx = NewContext(sc.iso)
		if sc.cfg != nil && sc.cfg.deterministicTime {
			if err := sc.installDeterminismShim(sc.ctx, sc.cfg.seedMillis); err != nil {
				panic("v8go: failed to install determinism shim: " + err.Error())
			}
		}
	}
	return sc.ctx
}

// CreateBlob serialises the heap into a startup blob and disposes the
// underlying isolate. Returns the raw v8::StartupData bytes (caller-owned;
// safe to copy/store/transmit). After this call sc.Isolate() and
// sc.Context() return nil and produce ErrSnapshotCreatorConsumed on use.
func (sc *SnapshotCreator) CreateBlob(fch FunctionCodeHandling) ([]byte, error) {
	sc.closeMu.Lock()
	defer sc.closeMu.Unlock()
	if sc.created {
		return nil, ErrSnapshotCreatorConsumed
	}
	if sc.ctx == nil {
		// CreateBlob requires at least one embedder context so the
		// consumer's NewContext path can call Context::FromSnapshot(iso, 0).
		sc.ctx = NewContext(sc.iso)
		if sc.cfg != nil && sc.cfg.deterministicTime {
			if err := sc.installDeterminismShim(sc.ctx, sc.cfg.seedMillis); err != nil {
				return nil, fmt.Errorf("v8go: failed to install determinism shim: %w", err)
			}
		}
	}

	// Register the embedder context with the SnapshotCreator. AddContext
	// must be invoked before CreateBlob.
	idx := C.SnapshotCreatorAddContext(sc.ptr, sc.ctx.ptr)
	if int(idx) != 0 {
		return nil, errors.New("v8go: SnapshotCreator.AddContext returned non-zero index")
	}

	blob := C.SnapshotCreatorCreateBlob(sc.ptr, C.int(fch))
	if blob.data == nil || blob.raw_size <= 0 {
		return nil, errors.New("v8go: SnapshotCreator.CreateBlob returned empty blob")
	}
	defer C.SnapshotCreatorFreeBlob(blob)
	sc.created = true

	out := C.GoBytes(unsafe.Pointer(blob.data), blob.raw_size)

	// The isolate is gone; clear our references so accidental use surfaces
	// as a nil-deref rather than a use-after-free.
	sc.iso.ptr = nil
	sc.iso = nil
	sc.ctx = nil
	return out, nil
}

// FreshContext replaces the SnapshotCreator's embedder context with a
// freshly created one that has a clean global object Map. Only the
// specified global property names are copied from the old context to
// the new one. All other state (including values tracked by the old
// context) is released.
//
// This resolves V8's Genesis::InitializeGlobal Map collision: when a
// snapshot context accumulates hundreds of global property additions
// (e.g. polyfill constructors), the Map transitions persist even after
// the properties are deleted. Deserializing such a context via
// Context::FromSnapshot triggers a fatal V8 assertion. FreshContext
// creates a new context whose Map has no extra transitions, then
// copies the specified properties (typically just "_bf") by reference
// so the full object graph survives serialization.
//
// Must be called after all scripts have executed and before CreateBlob.
func (sc *SnapshotCreator) FreshContext(keep []string) error {
	sc.closeMu.Lock()
	defer sc.closeMu.Unlock()
	if sc.created {
		return ErrSnapshotCreatorConsumed
	}
	if sc.ctx == nil {
		return errors.New("v8go: FreshContext called before Context()")
	}

	cNames := make([]*C.char, len(keep))
	for i, name := range keep {
		cNames[i] = C.CString(name)
	}
	defer func() {
		for _, p := range cNames {
			C.free(unsafe.Pointer(p))
		}
	}()

	var namesPtr **C.char
	if len(cNames) > 0 {
		namesPtr = &cNames[0]
	}

	newCtx := C.SnapshotCreatorFreshContext(
		sc.ptr, sc.ctx.ptr, namesPtr, C.int(len(keep)),
	)
	if newCtx == nil {
		return errors.New("v8go: SnapshotCreatorFreshContext failed")
	}

	// The old context's m_ctx has been freed by the C++ side.
	// Deregister the Go-side context reference.
	sc.ctx.deregister()
	sc.ctx.ptr = nil

	// Wire up the new context.
	ctxMutex.Lock()
	ctxSeq++
	ref := ctxSeq
	ctxMutex.Unlock()

	sc.ctx = &Context{
		ref: ref,
		ptr: newCtx,
		iso: sc.iso,
	}
	sc.ctx.register()
	return nil
}

// Dispose releases the SnapshotCreator. Safe to call multiple times. If
// CreateBlob was not invoked, the underlying isolate is disposed by the
// SnapshotCreator destructor.
func (sc *SnapshotCreator) Dispose() {
	sc.closeMu.Lock()
	if sc.closed {
		sc.closeMu.Unlock()
		return
	}
	sc.closed = true
	if sc.ptr != nil {
		C.SnapshotCreatorDispose(sc.ptr)
		sc.ptr = nil
	}
	sc.iso = nil
	sc.ctx = nil
	sc.pinned = nil
	runtime.SetFinalizer(sc, nil)
	sc.closeMu.Unlock()
	// Release the process-wide snapshot lock last so any goroutine
	// blocked in NewSnapshotCreator sees a fully torn-down predecessor.
	snapshotCreatorMu.Unlock()
	runtime.UnlockOSThread()
}

func (sc *SnapshotCreator) finalizer() {
	sc.Dispose()
}
