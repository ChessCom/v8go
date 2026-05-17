package v8go

import (
	"sync"
	"unsafe"
)

var (
	isolateRegistry   = make(map[uintptr]*Isolate)
	isolateRegistryMu sync.RWMutex
)

func registerIsolate(iso *Isolate) {
	isolateRegistryMu.Lock()
	isolateRegistry[uintptr(unsafe.Pointer(iso.ptr))] = iso
	isolateRegistryMu.Unlock()
}

func unregisterIsolate(iso *Isolate) {
	isolateRegistryMu.Lock()
	delete(isolateRegistry, uintptr(unsafe.Pointer(iso.ptr)))
	isolateRegistryMu.Unlock()
}

func lookupIsolate(ptr uintptr) *Isolate {
	isolateRegistryMu.RLock()
	iso := isolateRegistry[ptr]
	isolateRegistryMu.RUnlock()
	return iso
}
