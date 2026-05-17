package v8go

// #include <stdint.h>
// #include <stdlib.h>
// #include "snapshot.h"
import "C"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"unsafe"
)

// externalRefEntry is one row of the process-wide external_references
// registry. V8 stores indexes into this array inside snapshot blobs and
// looks the indices back up at deserialization time, so the order MUST
// be identical between the producer and the consumer.
type externalRefEntry struct {
	name string
	addr uintptr
}

var (
	extrefMu       sync.Mutex
	extrefEntries  []externalRefEntry
	extrefFrozen   bool
	extrefIndex    = map[string]int{} // name -> position in extrefEntries
	extrefCArray   unsafe.Pointer     // (*C.intptr_t) array, 0-terminated
	extrefCArrLen  int                // number of entries, excluding terminator
	extrefDigest   string             // sha256 hex over the frozen name list
	extrefBuiltins = []string{        // baked-in entries seeded on first use
		"v8go.FunctionTemplateCallback",
	}
)

func initBuiltinExternalReferences() {
	if len(extrefEntries) > 0 {
		return
	}
	addr := uintptr(C.v8go_FunctionTemplateCallback_addr())
	extrefEntries = append(extrefEntries, externalRefEntry{
		name: extrefBuiltins[0],
		addr: addr,
	})
	extrefIndex[extrefBuiltins[0]] = 0
}

// AddExternalReference registers a C-callable symbol (passed as
// unsafe.Pointer to a function) with the snapshot external_references
// registry under a stable name. Embedders that expose their own native
// callbacks to V8 (e.g. accessors registered via the V8 API) MUST register
// them here BEFORE the first snapshot is created or consumed. The order in
// which references are added does not matter — entries are sorted by name
// on freeze — but the SET of registered names MUST match across all
// processes that interact with a given snapshot blob.
//
// AddExternalReference panics if called after the registry has been
// frozen. Re-registering a name with the same address is a no-op.
func AddExternalReference(name string, fn unsafe.Pointer) {
	if name == "" {
		panic("v8go: AddExternalReference requires a non-empty name")
	}
	if fn == nil {
		panic("v8go: AddExternalReference requires a non-nil function pointer")
	}
	extrefMu.Lock()
	defer extrefMu.Unlock()
	if extrefFrozen {
		panic(fmt.Sprintf("v8go: external reference registry is frozen; cannot add %q", name))
	}
	initBuiltinExternalReferences()
	addr := uintptr(fn)
	if idx, ok := extrefIndex[name]; ok {
		if extrefEntries[idx].addr != addr {
			panic(fmt.Sprintf(
				"v8go: external reference %q already registered with different address",
				name,
			))
		}
		return
	}
	extrefIndex[name] = len(extrefEntries)
	extrefEntries = append(extrefEntries, externalRefEntry{name: name, addr: addr})
}

// IsExternalReferenceRegistryFrozen reports whether the registry has
// been frozen.
func IsExternalReferenceRegistryFrozen() bool {
	extrefMu.Lock()
	defer extrefMu.Unlock()
	return extrefFrozen
}

// ExternalReferenceRegistryDigest returns a stable hex sha256 of the
// frozen name list (one name per line, sorted ascending). Compatibility
// checks for packed snapshots compare this digest across producer and
// consumer processes.
func ExternalReferenceRegistryDigest() string {
	extrefMu.Lock()
	defer extrefMu.Unlock()
	freezeRegistryLocked()
	return extrefDigest
}

// ExternalReferenceRegistryNames returns a copy of the frozen, sorted
// name list. Intended for diagnostics and tests.
func ExternalReferenceRegistryNames() []string {
	extrefMu.Lock()
	defer extrefMu.Unlock()
	freezeRegistryLocked()
	names := make([]string, 0, extrefCArrLen)
	for _, e := range extrefEntries {
		names = append(names, e.name)
	}
	return names
}

// freezeRegistryLocked finalises the registry: sorts entries by name,
// materialises a 0-terminated C array, and computes the digest. Idempotent.
// Caller must hold extrefMu.
func freezeRegistryLocked() {
	if extrefFrozen {
		return
	}
	initBuiltinExternalReferences()
	sort.SliceStable(extrefEntries, func(i, j int) bool {
		return extrefEntries[i].name < extrefEntries[j].name
	})
	for i, e := range extrefEntries {
		extrefIndex[e.name] = i
	}

	n := len(extrefEntries)
	size := C.size_t((n + 1)) * C.size_t(C.sizeof_intptr_t)
	arr := C.malloc(size)
	if arr == nil {
		panic("v8go: failed to allocate external_references array")
	}
	slot := (*[1 << 28]C.intptr_t)(arr)[: n+1 : n+1]
	for i, e := range extrefEntries {
		slot[i] = C.intptr_t(e.addr)
	}
	slot[n] = 0
	extrefCArray = arr
	extrefCArrLen = n

	h := sha256.New()
	for _, e := range extrefEntries {
		h.Write([]byte(e.name))
		h.Write([]byte{'\n'})
	}
	extrefDigest = hex.EncodeToString(h.Sum(nil))

	extrefFrozen = true
}

// frozenExtRefArray returns the C pointer to the 0-terminated
// external_references array. Triggers the freeze if not already done.
func frozenExtRefArray() *C.intptr_t {
	extrefMu.Lock()
	defer extrefMu.Unlock()
	freezeRegistryLocked()
	if extrefCArray == nil {
		return nil
	}
	return (*C.intptr_t)(extrefCArray)
}
