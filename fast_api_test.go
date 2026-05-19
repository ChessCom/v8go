package v8go_test

import (
	"testing"
	"unsafe"

	v8 "github.com/ChessCom/v8go"
)

func TestFastFunctionTemplate_Basic(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	slowAdd := func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		args := info.Args()
		a := args[0].Int32()
		b := args[1].Int32()
		return v8.NewValue(iso, a+b)
	}

	tmpl := v8.NewFastFunctionTemplate(iso, slowAdd, v8.FastCallDescriptor{
		FastFn:     unsafe.Pointer(v8.TestFastAddInt32Addr()),
		ReturnType: v8.CTypeInt32,
		ArgTypes:   []v8.CType{v8.CTypeV8Value, v8.CTypeInt32, v8.CTypeInt32},
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	fn := tmpl.GetFunction(ctx)
	ctx.Global().Set("fastAdd", fn)

	val, err := ctx.RunScript("fastAdd(3, 4)", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 7 {
		t.Fatalf("expected 7, got %d", val.Int32())
	}
}

func TestFastFunctionTemplate_HotLoop(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	slowAdd := func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		args := info.Args()
		return v8.NewValue(iso, args[0].Int32()+args[1].Int32())
	}

	tmpl := v8.NewFastFunctionTemplate(iso, slowAdd, v8.FastCallDescriptor{
		FastFn:     unsafe.Pointer(v8.TestFastAddInt32Addr()),
		ReturnType: v8.CTypeInt32,
		ArgTypes:   []v8.CType{v8.CTypeV8Value, v8.CTypeInt32, v8.CTypeInt32},
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	fn := tmpl.GetFunction(ctx)
	ctx.Global().Set("fastAdd", fn)

	// Run enough iterations to trigger TurboFan optimization.
	val, err := ctx.RunScript(`
		let sum = 0;
		for (let i = 0; i < 100000; i++) {
			sum += fastAdd(i, 1);
		}
		sum;
	`, "hot.js")
	if err != nil {
		t.Fatal(err)
	}

	// sum = sum(i+1 for i in 0..99999) = sum(0..99999) + 100000
	// = 99999*100000/2 + 100000 = 5000050000
	expected := int64(5000050000)
	if val.Integer() != expected {
		t.Fatalf("expected %d, got %d", expected, val.Integer())
	}
}
