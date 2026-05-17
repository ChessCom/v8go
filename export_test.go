package v8go

// RegisterCallback is exported for testing only.
func (i *Isolate) RegisterCallback(cb FunctionCallbackWithError) int {
	return i.registerCallback(cb)
}

// GetCallback is exported for testing only.
func (i *Isolate) GetCallback(ref int) FunctionCallbackWithError {
	return i.getCallback(ref)
}

// GetContext is exported for testing only.
var GetContext = getContext

// Ref is exported for testing only.
func (c *Context) Ref() int {
	return c.ref
}

// TimeUnixMicro is exported for testing only.
var TimeUnixMicro = timeUnixMicro

// SymbolValue is exported for testing the Valuer interface on Symbol.
func SymbolValue(sym *Symbol) *Value {
	return sym.value()
}
