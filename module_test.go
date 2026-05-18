package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestCompileModule_Basic(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	mod, err := ctx.CompileModule("export const x = 42;", "test.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close()

	if mod.Status() != v8.ModuleStatusUninstantiated {
		t.Fatalf("expected Uninstantiated, got %d", mod.Status())
	}
}

func TestCompileModule_SyntaxError(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.CompileModule("export {", "bad.mjs")
	if err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestModule_ImportRequests(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	mod, err := ctx.CompileModule(`
		import { foo } from './foo.mjs';
		import { bar } from './bar.mjs';
		export default foo + bar;
	`, "main.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close()

	n := mod.GetModuleRequestsLength()
	if n != 2 {
		t.Fatalf("expected 2 import requests, got %d", n)
	}

	req0 := mod.GetModuleRequest(0)
	req1 := mod.GetModuleRequest(1)
	if req0 != "./foo.mjs" {
		t.Fatalf("expected './foo.mjs', got %q", req0)
	}
	if req1 != "./bar.mjs" {
		t.Fatalf("expected './bar.mjs', got %q", req1)
	}
}

func TestModule_InstantiateAndEvaluate(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	mod, err := ctx.CompileModule("export const answer = 42;", "simple.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close()

	err = mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if mod.Status() != v8.ModuleStatusInstantiated {
		t.Fatalf("expected Instantiated, got %d", mod.Status())
	}

	val, err := mod.Evaluate()
	if err != nil {
		t.Fatal(err)
	}
	_ = val

	if mod.Status() != v8.ModuleStatusEvaluated {
		t.Fatalf("expected Evaluated, got %d", mod.Status())
	}
}

func TestModule_Namespace(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	mod, err := ctx.CompileModule("export const x = 10; export const y = 20;", "ns.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close()

	err = mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = mod.Evaluate()
	if err != nil {
		t.Fatal(err)
	}

	ns := mod.GetNamespace()
	if ns == nil {
		t.Fatal("expected namespace object")
	}

	obj := ns.Object()
	if obj == nil {
		t.Fatal("expected object")
	}

	xVal, err := obj.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if xVal.Int32() != 10 {
		t.Fatalf("expected x=10, got %d", xVal.Int32())
	}

	yVal, err := obj.Get("y")
	if err != nil {
		t.Fatal(err)
	}
	if yVal.Int32() != 20 {
		t.Fatalf("expected y=20, got %d", yVal.Int32())
	}
}

func TestModule_WithDependency(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	depMod, err := ctx.CompileModule("export const dep = 'hello';", "dep.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer depMod.Close()

	mainMod, err := ctx.CompileModule(`
		import { dep } from './dep.mjs';
		export const result = dep + ' world';
	`, "main.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer mainMod.Close()

	resolver := func(specifier string, referrerHash int) *v8.Module {
		if specifier == "./dep.mjs" {
			return depMod
		}
		return nil
	}

	err = depMod.Instantiate(resolver)
	if err != nil {
		t.Fatal(err)
	}

	err = mainMod.Instantiate(resolver)
	if err != nil {
		t.Fatal(err)
	}

	_, err = depMod.Evaluate()
	if err != nil {
		t.Fatal(err)
	}

	_, err = mainMod.Evaluate()
	if err != nil {
		t.Fatal(err)
	}

	ns := mainMod.GetNamespace()
	obj := ns.Object()
	resultVal, err := obj.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if resultVal.String() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", resultVal.String())
	}
}
