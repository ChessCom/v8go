// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "isolate.h"
// #include "snapshot.h"
import "C"

import (
	"sync"
	"unsafe"
)

// Isolate is a JavaScript VM instance with its own heap and
// garbage collector. Most applications will create one isolate
// with many V8 contexts for execution.
type Isolate struct {
	ptr C.IsolatePtr

	cbMutex sync.RWMutex
	cbSeq   int
	cbs     map[int]FunctionCallbackWithError

	null      *Value
	undefined *Value

	snapshotData unsafe.Pointer // pinned C copy, freed in Dispose

	nearHeapLimitCB NearHeapLimitCallback
	promiseRejectCB PromiseRejectCallback
	gcPrologueCBs   []GCCallback
	gcEpilogueCBs   []GCCallback
}

// HeapStatistics represents V8 isolate heap statistics
type HeapStatistics struct {
	TotalHeapSize            uint64
	TotalHeapSizeExecutable  uint64
	TotalPhysicalSize        uint64
	TotalAvailableSize       uint64
	UsedHeapSize             uint64
	HeapSizeLimit            uint64
	MallocedMemory           uint64
	ExternalMemory           uint64
	PeakMallocedMemory       uint64
	NumberOfNativeContexts   uint64
	NumberOfDetachedContexts uint64
}

type resourceConstraints struct {
	InitialHeapSizeInBytes uint64
	MaxHeapSizeInBytes     uint64
}

// IsolateOption configures an Isolate on creation.
type IsolateOption func(*isolateConfig)

// isolateConfig holds the configuration for creating an isolate.
type isolateConfig struct {
	resourceConstraints       *resourceConstraints
	snapshotBlob              []byte
	disableDefaultHeapLimitCB bool
}

// WithResourceConstraints sets memory constraints for the isolate.
// If constraints are set, v8go will try to call `TerminateExecution` when the hard limit is hit.
func WithResourceConstraints(initialHeapSizeInBytes, maxHeapSizeInBytes uint64) IsolateOption {
	return func(config *isolateConfig) {
		config.resourceConstraints = &resourceConstraints{
			InitialHeapSizeInBytes: initialHeapSizeInBytes,
			MaxHeapSizeInBytes:     maxHeapSizeInBytes,
		}
	}
}

// WithSnapshotBlob creates the isolate with a V8 startup snapshot.
// The snapshot must have been produced by v8::SnapshotCreator using the
// same V8 version. Contexts created on a snapshot isolate inherit the
// heap state that was serialised into the blob (all pre-executed JS).
// Passing nil is a no-op (equivalent to not setting the option).
func WithSnapshotBlob(data []byte) IsolateOption {
	return func(config *isolateConfig) {
		config.snapshotBlob = data
	}
}

// NewIsolate creates a new V8 isolate with the provided options.
// Only one thread may access a given isolate at a time, but different
// threads may access different isolates simultaneously.
// When an isolate is no longer used its resources should be freed
// by calling iso.Dispose().
// An *Isolate can be used as a v8go.ContextOption to create a new
// Context, rather than creating a new default Isolate.
func NewIsolate(opts ...IsolateOption) *Isolate {
	initializeIfNecessary()

	config := &isolateConfig{}
	for _, opt := range opts {
		opt(config)
	}

	var cConstraints C.IsolateConstraintsPtr
	if config.resourceConstraints != nil {
		cConstraints = &C.IsolateConstraints{
			initial_heap_size_in_bytes: C.size_t(config.resourceConstraints.InitialHeapSizeInBytes),
			maximum_heap_size_in_bytes: C.size_t(config.resourceConstraints.MaxHeapSizeInBytes),
		}
	}

	iso := &Isolate{
		cbs: make(map[int]FunctionCallbackWithError),
	}

	// V8 isolate construction is not concurrency-safe on the binaries we
	// ship: parallel goroutines calling Isolate::New corrupt the
	// process-wide shared-heap state (StringForwardingTable, ReadOnly
	// heap, etc) and trigger fatal asserts. Serialising every
	// construction-plus-sentinel-bootstrap keeps the wrapper safe to
	// call from any goroutine.
	snapshotDeserMu.Lock()
	defer snapshotDeserMu.Unlock()
	if len(config.snapshotBlob) > 0 {
		// Route through the snapshot-aware constructor so the
		// process-wide external_references array is wired in. The
		// registry is frozen on first use.
		cData := C.CBytes(config.snapshotBlob)
		iso.snapshotData = cData
		iso.ptr = C.v8go_NewIsolateWithSnapshotAndRefs(
			cConstraints,
			(*C.char)(cData),
			C.int(len(config.snapshotBlob)),
			frozenExtRefArray(),
		)
	} else if config.disableDefaultHeapLimitCB {
		iso.ptr = C.NewIsolateNoDefaultHeapCB(cConstraints)
	} else {
		iso.ptr = C.NewIsolate(cConstraints)
	}

	iso.null = newValueNull(iso)
	iso.undefined = newValueUndefined(iso)
	registerIsolate(iso)
	return iso
}

// TerminateExecution terminates forcefully the current thread
// of JavaScript execution in the given isolate.
func (i *Isolate) TerminateExecution() {
	C.IsolateTerminateExecution(i.ptr)
}

// IsExecutionTerminating returns whether V8 is currently terminating
// Javascript execution. If true, there are still JavaScript frames
// on the stack and the termination exception is still active.
func (i *Isolate) IsExecutionTerminating() bool {
	return C.IsolateIsExecutionTerminating(i.ptr) == 1
}

// MemoryPressureLevel represents the severity of memory pressure.
type MemoryPressureLevel int

const (
	MemoryPressureNone     MemoryPressureLevel = 0
	MemoryPressureModerate MemoryPressureLevel = 1
	MemoryPressureCritical MemoryPressureLevel = 2
)

// LowMemoryNotification signals V8 to perform a full garbage collection
// to free as much memory as possible. This is a synchronous call that
// blocks until GC completes. Use MemoryPressureNotification for a more
// nuanced signal.
func (i *Isolate) LowMemoryNotification() {
	C.IsolateLowMemoryNotification(i.ptr)
}

// MemoryPressureNotification signals V8 about the current memory pressure
// level so it can adjust its GC strategy accordingly.
// MemoryPressureNone allows V8 to use its default GC schedule.
// MemoryPressureModerate speeds up incremental GC at the cost of latency.
// MemoryPressureCritical triggers aggressive GC to free memory immediately.
func (i *Isolate) MemoryPressureNotification(level MemoryPressureLevel) {
	C.IsolateMemoryPressureNotification(i.ptr, C.int(level))
}

// CancelTerminateExecution cancels a pending TerminateExecution request.
// This is useful when recycling an isolate after a timeout: call
// CancelTerminateExecution before running new scripts so the termination
// flag from the previous execution does not kill the next one.
func (i *Isolate) CancelTerminateExecution() {
	C.IsolateCancelTerminateExecution(i.ptr)
}

// GarbageCollectionType selects which GC generation to collect.
type GarbageCollectionType int

const (
	GCTypeFull  GarbageCollectionType = 0
	GCTypeMinor GarbageCollectionType = 1
)

// RequestGarbageCollectionForTesting triggers an explicit garbage collection.
// This only works when V8 is started with the --expose_gc flag
// (via SetFlags("--expose_gc") before creating any isolate).
// Intended for testing only — do not use in production.
func (i *Isolate) RequestGarbageCollectionForTesting(typ GarbageCollectionType) {
	C.IsolateRequestGarbageCollectionForTesting(i.ptr, C.int(typ))
}

// ContextDisposedNotification notifies V8 that a context has been disposed.
// V8 uses this to tune garbage collection and finalization scheduling.
// Call this after Context.Close() when the context will not be reused.
// Set dependantContext to true if other contexts depend on this one.
func (i *Isolate) ContextDisposedNotification(dependantContext bool) {
	dep := 0
	if dependantContext {
		dep = 1
	}
	C.IsolateContextDisposedNotification(i.ptr, C.int(dep))
}

type CompileOptions struct {
	CachedData *CompilerCachedData

	Mode CompileMode
}

// CompileUnboundScript will create an UnboundScript (i.e. context-indepdent)
// using the provided source JavaScript, origin (a.k.a. filename), and options.
// If options contain a non-null CachedData, compilation of the script will use
// that code cache.
// error will be of type `JSError` if not nil.
func (i *Isolate) CompileUnboundScript(
	source, origin string,
	opts CompileOptions,
) (*UnboundScript, error) {
	cSource := C.CString(source)
	cOrigin := C.CString(origin)
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cOrigin))

	var cOptions C.CompileOptions
	if opts.CachedData != nil {
		if opts.Mode != 0 {
			panic("On CompileOptions, Mode and CachedData can't both be set")
		}
		cOptions.compileOption = C.ScriptCompilerConsumeCodeCache
		cOptions.cachedData = C.ScriptCompilerCachedData{
			data:   (*C.uchar)(unsafe.Pointer(&opts.CachedData.Bytes[0])),
			length: C.int(len(opts.CachedData.Bytes)),
		}
	} else {
		cOptions.compileOption = C.int(opts.Mode)
	}

	rtn := C.IsolateCompileUnboundScript(i.ptr, cSource, cOrigin, cOptions)
	if rtn.ptr == nil {
		return nil, newJSError(rtn.error)
	}
	if opts.CachedData != nil {
		opts.CachedData.Rejected = int(rtn.cachedDataRejected) == 1
	}
	return &UnboundScript{
		ptr: rtn.ptr,
		iso: i,
	}, nil
}

// GetHeapStatistics returns heap statistics for an isolate.
func (i *Isolate) GetHeapStatistics() HeapStatistics {
	hs := C.IsolationGetHeapStatistics(i.ptr)

	return HeapStatistics{
		TotalHeapSize:            uint64(hs.total_heap_size),
		TotalHeapSizeExecutable:  uint64(hs.total_heap_size_executable),
		TotalPhysicalSize:        uint64(hs.total_physical_size),
		TotalAvailableSize:       uint64(hs.total_available_size),
		UsedHeapSize:             uint64(hs.used_heap_size),
		HeapSizeLimit:            uint64(hs.heap_size_limit),
		MallocedMemory:           uint64(hs.malloced_memory),
		ExternalMemory:           uint64(hs.external_memory),
		PeakMallocedMemory:       uint64(hs.peak_malloced_memory),
		NumberOfNativeContexts:   uint64(hs.number_of_native_contexts),
		NumberOfDetachedContexts: uint64(hs.number_of_detached_contexts),
	}
}

// Dispose will dispose the Isolate VM; subsequent calls will panic.
func (i *Isolate) Dispose() {
	if i.ptr == nil {
		return
	}
	unregisterIsolate(i)
	// Serialise dispose against snapshotDeserMu: V8 14.x corrupts its
	// shared-heap teardown state if a parallel goroutine is constructing
	// a new isolate at the same moment we are tearing one down. The
	// critical section is short (microseconds) and idiomatic v8::Locker
	// rules continue to apply for everyday API calls.
	snapshotDeserMu.Lock()
	C.IsolateDisposeSnapshot(i.ptr)
	C.IsolateDispose(i.ptr)
	snapshotDeserMu.Unlock()
	i.ptr = nil
	if i.snapshotData != nil {
		C.free(i.snapshotData)
		i.snapshotData = nil
	}
}

// ThrowException schedules an exception to be thrown when returning to
// JavaScript. When an exception has been scheduled it is illegal to invoke
// any JavaScript operation; the caller must return immediately and only after
// the exception has been handled does it become legal to invoke JavaScript operations.
func (i *Isolate) ThrowException(value *Value) *Value {
	if i.ptr == nil {
		panic("Isolate has been disposed")
	}
	return &Value{
		ptr: C.IsolateThrowException(i.ptr, value.ptr),
	}
}

// Deprecated: use `iso.Dispose()`.
func (i *Isolate) Close() {
	i.Dispose()
}

func (i *Isolate) apply(opts *contextOptions) {
	opts.iso = i
}

func (i *Isolate) registerCallback(cb FunctionCallbackWithError) int {
	i.cbMutex.Lock()
	i.cbSeq++
	ref := i.cbSeq
	i.cbs[ref] = cb
	i.cbMutex.Unlock()
	return ref
}

func (i *Isolate) getCallback(ref int) FunctionCallbackWithError {
	i.cbMutex.RLock()
	defer i.cbMutex.RUnlock()
	return i.cbs[ref]
}
