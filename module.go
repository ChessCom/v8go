package v8go

// #include <stdlib.h>
// #include "module.h"
import "C"
import (
	"sync"
	"unsafe"
)

// ModuleStatus represents the state of a compiled ES module.
type ModuleStatus int

const (
	ModuleStatusUninstantiated ModuleStatus = 0
	ModuleStatusInstantiating  ModuleStatus = 1
	ModuleStatusInstantiated   ModuleStatus = 2
	ModuleStatusEvaluating     ModuleStatus = 3
	ModuleStatusEvaluated      ModuleStatus = 4
	ModuleStatusErrored        ModuleStatus = 5
)

// Module represents a compiled ES module (ESM).
type Module struct {
	ptr C.ModulePtr
	iso *Isolate
	ctx *Context
}

// ModuleResolveCallback is called when an import statement needs to be
// resolved. The callback receives the specifier (the import path) and
// the referrer module's identity hash. It must return the resolved
// Module or nil if resolution fails.
type ModuleResolveCallback func(specifier string, referrerHash int) *Module

var (
	moduleResolverMu sync.RWMutex
	moduleResolvers  = make(map[int]ModuleResolveCallback)
)

func setModuleResolver(ctxRef int, cb ModuleResolveCallback) {
	moduleResolverMu.Lock()
	moduleResolvers[ctxRef] = cb
	moduleResolverMu.Unlock()
}

func clearModuleResolver(ctxRef int) {
	moduleResolverMu.Lock()
	delete(moduleResolvers, ctxRef)
	moduleResolverMu.Unlock()
}

// CompileModule compiles an ES module from source. The module is in
// Uninstantiated state after compilation. Call Instantiate() and
// Evaluate() to run it.
func (c *Context) CompileModule(source, origin string) (*Module, error) {
	cSource := C.CString(source)
	cOrigin := C.CString(origin)
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cOrigin))

	rtn := C.CompileESModule(c.ptr, cSource, cOrigin)
	if rtn.ptr == nil {
		return nil, newJSError(rtn.error)
	}
	return &Module{ptr: rtn.ptr, iso: c.Isolate(), ctx: c}, nil
}

// Status returns the current status of the module.
func (m *Module) Status() ModuleStatus {
	return ModuleStatus(C.ModuleGetStatus(m.iso.ptr, m.ptr))
}

// GetModuleRequestsLength returns the number of import requests.
func (m *Module) GetModuleRequestsLength() int {
	return int(C.ModuleGetRequestsLength(m.iso.ptr, m.ptr))
}

// GetModuleRequest returns the specifier of the i-th import request.
func (m *Module) GetModuleRequest(index int) string {
	cStr := C.ModuleGetRequest(m.ctx.ptr, m.ptr, C.int(index))
	defer C.free(unsafe.Pointer(cStr))
	return C.GoString(cStr)
}

// IdentityHash returns a hash code for the module, suitable for use
// as a map key. Two modules with the same identity hash are the same
// module.
func (m *Module) IdentityHash() int {
	return int(C.ModuleGetIdentityHash(m.iso.ptr, m.ptr))
}

// Instantiate instantiates the module, resolving all import requests
// using the provided resolver callback. The resolver is called for
// each `import` statement in the module and must return the compiled
// Module for the given specifier.
func (m *Module) Instantiate(resolver ModuleResolveCallback) error {
	ctxRef := m.ctx.ref
	setModuleResolver(ctxRef, resolver)
	defer clearModuleResolver(ctxRef)

	ok := C.ModuleInstantiate(m.ctx.ptr, m.ptr)
	if ok == 0 {
		return errModuleInstantiation
	}
	return nil
}

// Evaluate evaluates the module after instantiation. Returns the
// completion value (typically a Promise for async modules).
func (m *Module) Evaluate() (*Value, error) {
	rtn := C.ModuleEvaluate(m.ctx.ptr, m.ptr)
	return valueResult(m.ctx, rtn)
}

// GetNamespace returns the module's namespace object (the object that
// contains the module's exports).
func (m *Module) GetNamespace() *Value {
	ptr := C.ModuleGetNamespace(m.ctx.ptr, m.ptr)
	if ptr == nil {
		return nil
	}
	return &Value{ptr: ptr, ctx: m.ctx}
}

// Close releases the module's V8 resources.
func (m *Module) Close() {
	if m.ptr != nil {
		C.ModuleFree(m.ptr)
		m.ptr = nil
	}
}

var errModuleInstantiation = &JSError{Message: "module instantiation failed"}

//export goResolveModuleCallback
func goResolveModuleCallback(ctxRef C.int, specifier *C.char, referrerHash C.int) C.ModulePtr {
	moduleResolverMu.RLock()
	resolver := moduleResolvers[int(ctxRef)]
	moduleResolverMu.RUnlock()
	if resolver == nil {
		return nil
	}
	mod := resolver(C.GoString(specifier), int(referrerHash))
	if mod == nil {
		return nil
	}
	return mod.ptr
}
