package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestGetPropertyNames(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript(`
		const parent = { inherited: true };
		const child = Object.create(parent);
		child.own1 = 1;
		child.own2 = 2;
		child;
	`, "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	names, err := obj.GetPropertyNames()
	if err != nil {
		t.Fatal(err)
	}
	namesObj, err := names.AsObject()
	if err != nil {
		t.Fatal(err)
	}
	lengthVal, err := namesObj.Get("length")
	if err != nil {
		t.Fatal(err)
	}
	length := lengthVal.Int32()
	if length < 3 {
		t.Fatalf("expected at least 3 property names (own1, own2, inherited), got %d", length)
	}
}

func TestGetOwnPropertyNames(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript(`
		const parent = { inherited: true };
		const child = Object.create(parent);
		child.own1 = 1;
		child.own2 = 2;
		child;
	`, "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	names, err := obj.GetOwnPropertyNames()
	if err != nil {
		t.Fatal(err)
	}
	namesObj, err := names.AsObject()
	if err != nil {
		t.Fatal(err)
	}
	lengthVal, err := namesObj.Get("length")
	if err != nil {
		t.Fatal(err)
	}
	length := lengthVal.Int32()
	if length != 2 {
		t.Fatalf("expected 2 own property names, got %d", length)
	}
}

func TestGetPropertyNames_Empty(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript("Object.create(null)", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	names, err := obj.GetPropertyNames()
	if err != nil {
		t.Fatal(err)
	}
	namesObj, err := names.AsObject()
	if err != nil {
		t.Fatal(err)
	}
	lengthVal, err := namesObj.Get("length")
	if err != nil {
		t.Fatal(err)
	}
	if lengthVal.Int32() != 0 {
		t.Fatalf("expected 0 property names for null-prototype object, got %d", lengthVal.Int32())
	}
}

func TestGetPrototype(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript(`
		const proto = { greeting: "hello" };
		const obj = Object.create(proto);
		obj;
	`, "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	proto := obj.GetPrototype()
	if proto == nil {
		t.Fatal("expected non-nil prototype")
	}
	if !proto.IsObject() {
		t.Fatal("expected prototype to be an object")
	}
	protoObj, _ := proto.AsObject()
	greetVal, err := protoObj.Get("greeting")
	if err != nil {
		t.Fatal(err)
	}
	if greetVal.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", greetVal.String())
	}
}

func TestSetPrototype(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript("({})", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	protoVal, err := ctx.RunScript(`({ myMethod: function() { return 42; } })`, "proto.js")
	if err != nil {
		t.Fatal(err)
	}

	if err := obj.SetPrototype(protoVal); err != nil {
		t.Fatalf("SetPrototype: %v", err)
	}

	result, err := ctx.RunScript(`
		const target = globalThis.__test_obj__;
	`, "verify.js")
	_ = result

	// Verify by setting the object on global and calling the method
	ctx.Global().Set("testObj", obj)
	methodResult, err := ctx.RunScript("testObj.myMethod()", "call.js")
	if err != nil {
		t.Fatalf("method call: %v", err)
	}
	if methodResult.Int32() != 42 {
		t.Fatalf("expected 42 from prototype method, got %d", methodResult.Int32())
	}
}

func TestSetPrototype_Null(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript("({})", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := val.AsObject()
	if err != nil {
		t.Fatal(err)
	}

	nullVal, err := ctx.RunScript("null", "null.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.SetPrototype(nullVal); err != nil {
		t.Fatalf("SetPrototype(null): %v", err)
	}

	proto := obj.GetPrototype()
	if proto == nil {
		t.Fatal("GetPrototype returned nil Go pointer")
	}
	if !proto.IsNull() {
		t.Fatal("expected null prototype after SetPrototype(null)")
	}
}

func TestGetPrototype_GlobalObject(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	global := ctx.Global()
	proto := global.GetPrototype()
	if proto == nil {
		t.Fatal("global prototype should not be nil")
	}
}
