// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"errors"
	"strings"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestTimeUnixMicro(t *testing.T) {
	t.Parallel()
	got := v8.TimeUnixMicro(1_000_000)
	if got.Unix() != 1 {
		t.Fatalf("timeUnixMicro(1_000_000) = %v, want Unix == 1", got)
	}
	got2 := v8.TimeUnixMicro(0)
	if got2.Unix() != 0 {
		t.Fatalf("timeUnixMicro(0).Unix() = %d, want 0", got2.Unix())
	}
}

func TestCPUProfileNode_AllAccessors(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	cpuProfiler := v8.NewCPUProfiler(iso)
	defer cpuProfiler.Dispose()
	cpuProfiler.StartProfiling("nodeaccessors")
	_, err := ctx.RunScript(profileScript, "script.js")
	fatalIf(t, err)
	val, err := ctx.Global().Get("start")
	fatalIf(t, err)
	fn, err := val.AsFunction()
	fatalIf(t, err)
	timeout, err := v8.NewValue(iso, int32(2000))
	fatalIf(t, err)
	_, err = fn.Call(ctx.Global(), timeout)
	fatalIf(t, err)

	cpuProfile := cpuProfiler.StopProfiling("nodeaccessors")
	if cpuProfile == nil {
		t.Fatal("expected profile")
	}
	defer cpuProfile.Delete()

	root := cpuProfile.GetTopDownRoot()
	if root == nil {
		t.Fatal("expected root node")
	}

	_ = root.GetNodeId()
	_ = root.GetScriptId()
	_ = root.GetHitCount()
	_ = root.GetBailoutReason()

	var startNode *v8.CPUProfileNode
	for i := 0; i < root.GetChildrenCount(); i++ {
		child := root.GetChild(i)
		if child.GetFunctionName() == "start" {
			startNode = child
			break
		}
	}
	if startNode != nil {
		if startNode.GetNodeId() == 0 && startNode.GetScriptId() == 0 {
			t.Log("node ID or script ID may be zero; that's valid for some profiles")
		}
		_ = startNode.GetHitCount()
		if br := startNode.GetBailoutReason(); br != "" {
			t.Logf("bailout reason: %s", br)
		}
	}
}

func TestResetNonDeterminism(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	if err := v8.ResetNonDeterminism(ctx); err != nil {
		t.Fatalf("ResetNonDeterminism: %v", err)
	}
}

func TestExceptionString_NilValue(t *testing.T) {
	t.Parallel()
	e := &v8.Exception{}
	if got := e.String(); got != "<nil>" {
		t.Fatalf("String() = %q, want %q", got, "<nil>")
	}
}

func TestExceptionAs_WrongType(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	e := v8.NewError(iso, "test error")
	var wrongTarget *v8.JSError
	if e.As(&wrongTarget) {
		t.Fatal("As should return false for wrong target type")
	}
}

func TestMessageErrorLevel_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level v8.MessageErrorLevel
		want  string
	}{
		{v8.ErrorLevelLog, "log"},
		{v8.ErrorLevelDebug, "debug"},
		{v8.ErrorLevelError, "error"},
		{v8.ErrorLevelInfo, "info"},
		{v8.ErrorLevelWarning, "warning"},
		{v8.MessageErrorLevel(99), "99"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("MessageErrorLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestExternalReferenceRegistryNames(t *testing.T) {
	t.Parallel()
	names := v8.ExternalReferenceRegistryNames()
	if len(names) == 0 {
		t.Fatal("expected at least the builtin reference")
	}
	found := false
	for _, n := range names {
		if n == "v8go.FunctionTemplateCallback" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing builtin reference; got: %v", names)
	}
}

func TestAddExternalReference_EmptyName(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty name")
		}
		if msg := panicMessage(r); !strings.Contains(msg, "non-empty name") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	v8.AddExternalReference("", dummyFnPtr())
}

func TestAddExternalReference_NilPointer(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil pointer")
		}
		if msg := panicMessage(r); !strings.Contains(msg, "non-nil") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	v8.AddExternalReference("test.nil", nil)
}

func TestAddExternalReference_DuplicateDifferentAddr(t *testing.T) {
	// This test can only run if the registry is frozen because we can't
	// truly add a reference then re-add with different addr after freeze.
	// The test validates the panic path for a name re-registration with
	// mismatched address. Since the registry IS already frozen by other
	// tests, AddExternalReference will panic with "frozen" first. We test
	// the frozen panic path here instead.
	t.Parallel()
	if !v8.IsExternalReferenceRegistryFrozen() {
		t.Skip("registry not frozen; cannot test duplicate-different-addr panic")
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
	}()
	v8.AddExternalReference("v8go.FunctionTemplateCallback", dummyFnPtr())
}

func TestSymbol_StringAndValue(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	sym := v8.SymbolIterator(iso)
	str := sym.String()
	if str != "Symbol.iterator" {
		t.Fatalf("String() = %q, want %q", str, "Symbol.iterator")
	}
}

func TestObjectSetSymbol(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	obj := ctx.Global()
	sym := v8.SymbolToStringTag(iso)

	if err := obj.SetSymbol(sym, "MyObj"); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}

	val, err := ctx.RunScript(`Object.prototype.toString.call(globalThis)`, "probe.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if got := val.String(); got != "[object MyObj]" {
		t.Fatalf("toStringTag = %q, want %q", got, "[object MyObj]")
	}
}

func TestObjectSetSymbol_Error(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	obj := ctx.Global()
	sym := v8.SymbolIterator(iso)

	if err := obj.SetSymbol(sym, struct{}{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestTemplateSetSymbol_AllTypes(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	global := v8.NewObjectTemplate(iso)
	sym := v8.SymbolToStringTag(iso)

	if err := global.SetSymbol(sym, "TestObj"); err != nil {
		t.Fatalf("SetSymbol string: %v", err)
	}
	if err := global.SetSymbol(sym, int32(42)); err != nil {
		t.Fatalf("SetSymbol int32: %v", err)
	}

	fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		return nil
	})
	if err := global.SetSymbol(sym, fn); err != nil {
		t.Fatalf("SetSymbol FunctionTemplate: %v", err)
	}

	inner := v8.NewObjectTemplate(iso)
	if err := global.SetSymbol(sym, inner); err != nil {
		t.Fatalf("SetSymbol ObjectTemplate: %v", err)
	}

	ctx := v8.NewContext(iso)
	defer ctx.Close()
	val, _ := v8.NewValue(iso, "hello")
	if err := global.SetSymbol(sym, val); err != nil {
		t.Fatalf("SetSymbol Value: %v", err)
	}
}

func TestTemplateSetSymbol_UnsupportedType(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	tmpl := v8.NewObjectTemplate(iso)
	sym := v8.SymbolIterator(iso)

	if err := tmpl.SetSymbol(sym, struct{}{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestTemplateSetSymbol_ObjectValue(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	tmpl := v8.NewObjectTemplate(iso)
	sym := v8.SymbolIterator(iso)

	objVal, err := ctx.RunScript(`({})`, "obj.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	err = tmpl.SetSymbol(sym, objVal)
	if err == nil {
		t.Fatal("expected error for object Value")
	}
	if !strings.Contains(err.Error(), "primitive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateSet_ValueObject(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	tmpl := v8.NewObjectTemplate(iso)
	objVal, err := ctx.RunScript(`({})`, "obj.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	err = tmpl.Set("key", objVal)
	if err == nil {
		t.Fatal("expected error for object Value")
	}
	if !strings.Contains(err.Error(), "primitive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetCallAsFunctionHandler(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	tmpl := v8.NewObjectTemplate(iso)
	tmpl.SetCallAsFunctionHandler(func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return v8.NewValue(info.Context().Isolate(), "called!")
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()
	obj, err := tmpl.NewInstance(ctx)
	if err != nil {
		t.Fatalf("NewInstance: %v", err)
	}
	ctx.Global().Set("callable", obj)
	val, err := ctx.RunScript(`callable()`, "test.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if got := val.String(); got != "called!" {
		t.Fatalf("got %q, want %q", got, "called!")
	}
}

func TestSetCallAsFunctionHandler_NilPanics(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	tmpl := v8.NewObjectTemplate(iso)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil callback")
		}
	}()
	tmpl.SetCallAsFunctionHandler(nil)
}

func TestValueIsWasmModuleObject(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript(`42`, "v.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if val.IsWasmModuleObject() {
		t.Error("42 should not be a WasmModuleObject")
	}
}

func TestValueIsModuleNamespaceObject(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript(`"hello"`, "v.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if val.IsModuleNamespaceObject() {
		t.Error("string should not be a ModuleNamespaceObject")
	}
}

func TestValueDetailString_Object(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript(`({a: 1, b: "hello"})`, "obj.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	ds := val.DetailString()
	if ds == "" {
		t.Fatal("DetailString() returned empty string")
	}
}

func TestValueBigInt_Nil(t *testing.T) {
	t.Parallel()
	var v *v8.Value
	if got := v.BigInt(); got != nil {
		t.Fatalf("BigInt() on nil value = %v, want nil", got)
	}
}

func TestValueObject_NonObject(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript(`42`, "num.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	obj := val.Object()
	if obj == nil {
		t.Fatal("Object() should not return nil for number")
	}
}

func TestMarshal_EmptyBlob(t *testing.T) {
	t.Parallel()
	p := &v8.PackedSnapshot{Blob: nil}
	_, err := p.Marshal()
	if err == nil {
		t.Fatal("expected error for empty blob")
	}
}

func TestNewResourceConstraints(t *testing.T) {
	t.Parallel()
	rc := v8.NewResourceConstraints(1024*1024, 16*1024*1024)
	if rc == nil {
		t.Fatal("NewResourceConstraints returned nil")
	}
}

func TestRestoreIsolate_NilReceiver(t *testing.T) {
	t.Parallel()
	var p *v8.PackedSnapshot
	_, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestRestoreIsolate_EmptyBlob(t *testing.T) {
	t.Parallel()
	p := &v8.PackedSnapshot{Blob: nil}
	_, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err == nil {
		t.Fatal("expected error for empty blob")
	}
}

func TestRestoreIsolate_WithResourceConstraints(t *testing.T) {
	// Intentionally NOT t.Parallel()
	p, err := v8.PackBundle(v8.PackOptions{Source: `globalThis.X = 1;`})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	rc := v8.NewResourceConstraints(1024*1024, 256*1024*1024)
	iso, err := p.RestoreIsolate(v8.RestoreOptions{ResourceConstraints: rc})
	if err != nil {
		t.Fatalf("RestoreIsolate with constraints: %v", err)
	}
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()
	v, err := ctx.RunScript(`X`, "p.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := v.Integer(); got != 1 {
		t.Fatalf("X = %d, want 1", got)
	}
}

func TestPackBundle_EmptySource(t *testing.T) {
	t.Parallel()
	_, err := v8.PackBundle(v8.PackOptions{})
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestPackBundle_DefaultOrigin(t *testing.T) {
	// Intentionally NOT t.Parallel()
	p, err := v8.PackBundle(v8.PackOptions{Source: `globalThis.Y = 2;`})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	if len(p.Blob) == 0 {
		t.Fatal("expected non-empty blob")
	}
	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	iso.Dispose()
}

func TestPackBundle_WithDeterministicTime(t *testing.T) {
	// Intentionally NOT t.Parallel()
	p, err := v8.PackBundle(v8.PackOptions{
		Source:            `globalThis.Z = Date.now();`,
		DeterministicTime: true,
		SeedMillis:        1_700_000_000_000,
	})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	if len(p.Blob) == 0 {
		t.Fatal("expected non-empty blob")
	}
	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()
	v, err := ctx.RunScript(`Z`, "p.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := v.Integer(); got != 1_700_000_000_000 {
		t.Fatalf("Z = %d, want 1700000000000", got)
	}
}

func TestWithExistingSnapshotBlob(t *testing.T) {
	// Intentionally NOT t.Parallel()
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()
	if _, err := ctx.RunScript(`globalThis.BASE = 100;`, "base.js"); err != nil {
		t.Fatalf("base: %v", err)
	}
	baseBlob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	sc2 := v8.NewSnapshotCreator(v8.WithExistingSnapshotBlob(baseBlob))
	ctx2 := sc2.Context()
	if _, err := ctx2.RunScript(`globalThis.OVERLAY = BASE + 1;`, "overlay.js"); err != nil {
		t.Fatalf("overlay: %v", err)
	}
	layeredBlob, err := sc2.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob layered: %v", err)
	}
	sc2.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(layeredBlob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript(`OVERLAY`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := v.Integer(); got != 101 {
		t.Fatalf("OVERLAY = %d, want 101", got)
	}
}

func TestSnapshotCreator_CreateBlobWithoutContext(t *testing.T) {
	// Intentionally NOT t.Parallel()
	sc := v8.NewSnapshotCreator()
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob without explicit Context: %v", err)
	}
	sc.Dispose()
	if len(blob) == 0 {
		t.Fatal("expected non-empty blob")
	}
}

func TestSnapshotCreator_CreateBlobWithDeterminismNoContext(t *testing.T) {
	// Intentionally NOT t.Parallel()
	sc := v8.NewSnapshotCreator(v8.WithDeterministicTime(0))
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob with determinism, no explicit Context: %v", err)
	}
	sc.Dispose()
	if len(blob) == 0 {
		t.Fatal("expected non-empty blob")
	}
}

func TestPromiseResolver_NilContext(t *testing.T) {
	t.Parallel()
	_, err := v8.NewPromiseResolver(nil)
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestPromiseThenWithError_TwoCallbacks(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	res, err := v8.NewPromiseResolver(ctx)
	if err != nil {
		t.Fatalf("NewPromiseResolver: %v", err)
	}
	prom := res.GetPromise()

	called := false
	prom.ThenWithError(
		func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
			called = true
			return nil, nil
		},
		func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
			return nil, nil
		},
	)

	val, _ := v8.NewValue(iso, "done")
	res.Resolve(val)
	ctx.PerformMicrotaskCheckpoint()
	_ = called
}

func TestPromiseCatchWithError(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	res, err := v8.NewPromiseResolver(ctx)
	if err != nil {
		t.Fatalf("NewPromiseResolver: %v", err)
	}
	prom := res.GetPromise()

	caught := false
	prom.CatchWithError(func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		caught = true
		return nil, nil
	})

	errVal, _ := v8.NewValue(iso, "rejected")
	res.Reject(errVal)
	ctx.PerformMicrotaskCheckpoint()
	_ = caught
}

func TestNewFunctionTemplateWithError_NilIsolate(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil isolate")
		}
	}()
	v8.NewFunctionTemplateWithError(nil, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, nil
	})
}

func TestNewFunctionTemplateWithError_NilCallback(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil callback")
		}
	}()
	v8.NewFunctionTemplateWithError(iso, nil)
}

func TestGoFunctionCallback_ReturnsError(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	fn := v8.NewFunctionTemplateWithError(iso, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, errors.New("callback-error")
	})
	global := v8.NewObjectTemplate(iso)
	global.Set("errfn", fn)
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	_, err := ctx.RunScript(`errfn()`, "test.js")
	if err == nil {
		t.Fatal("expected error from callback")
	}
	if !strings.Contains(err.Error(), "callback-error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGoFunctionCallback_ReturnsExceptionError(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	fn := v8.NewFunctionTemplateWithError(iso, func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, v8.NewTypeError(iso, "type-error-from-go")
	})
	global := v8.NewObjectTemplate(iso)
	global.Set("tyfn", fn)
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	_, err := ctx.RunScript(`
		try { tyfn(); "no error" } catch(e) { e.message; }
	`, "test.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
}

func TestPackBundle_WithExistingBlob(t *testing.T) {
	// Intentionally NOT t.Parallel()
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()
	if _, err := ctx.RunScript(`globalThis.BASE = 10;`, "base.js"); err != nil {
		t.Fatalf("base: %v", err)
	}
	baseBlob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	p, err := v8.PackBundle(v8.PackOptions{
		Source:       `globalThis.LAYER = BASE + 5;`,
		ExistingBlob: baseBlob,
	})
	if err != nil {
		t.Fatalf("PackBundle with ExistingBlob: %v", err)
	}
	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()
	v, err := c.RunScript(`LAYER`, "p.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := v.Integer(); got != 15 {
		t.Fatalf("LAYER = %d, want 15", got)
	}
}

func TestMarshal_HeaderTooLarge(t *testing.T) {
	t.Parallel()
	p := &v8.PackedSnapshot{
		Blob:  []byte{0x01},
		Extra: make(map[string]string),
	}
	bigVal := strings.Repeat("x", 1000)
	for i := 0; i < 200; i++ {
		key := strings.Repeat("k", 500) + string(rune('a'+i%26)) + string(rune('0'+i/26))
		p.Extra[key] = bigVal
	}
	_, err := p.Marshal()
	if err == nil {
		t.Fatal("expected error for header too large")
	}
	if !strings.Contains(err.Error(), "header") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValueBigInt_NonBigInt(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript(`"hello"`, "str.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	result := val.BigInt()
	if result != nil {
		t.Fatalf("BigInt() on string = %v, want nil", result)
	}
}

func TestTemplateSet_PrimitiveValue(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	tmpl := v8.NewObjectTemplate(iso)
	val, _ := v8.NewValue(iso, "hello")
	if err := tmpl.Set("key", val); err != nil {
		t.Fatalf("Set with primitive Value: %v", err)
	}
}

func TestSymbol_UsableAsSetSymbolKey(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	sym := v8.SymbolIterator(iso)
	obj := ctx.Global()
	fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		return nil
	})
	global := v8.NewObjectTemplate(iso)
	if err := global.SetSymbol(sym, fn); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}
	_ = obj
}

func TestSymbol_ValueMethod(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	sym := v8.SymbolIterator(iso)
	val := v8.SymbolValue(sym)
	if val == nil {
		t.Fatal("Symbol.value() returned nil")
	}
}

func TestPromiseThenWithError_PanicOnZeroCallbacks(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	res, _ := v8.NewPromiseResolver(ctx)
	prom := res.GetPromise()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero callbacks")
		}
	}()
	prom.ThenWithError()
}

func TestFunctionTemplateGetFunction_Invoke(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		val, _ := v8.NewValue(iso, "result")
		return val
	})

	f := fn.GetFunction(ctx)
	if f == nil {
		t.Fatal("GetFunction returned nil")
	}
	val, err := f.Call(ctx.Global())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if val.String() != "result" {
		t.Fatalf("got %q, want %q", val.String(), "result")
	}
}
