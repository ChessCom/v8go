package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestSetNamedPropertyHandler_Getter(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	tmpl := v8.NewObjectTemplate(iso)
	tmpl.SetNamedPropertyHandler(
		func(property string, info *v8.InterceptorCallbackInfo) *v8.Value {
			ctx := info.Context()
			if ctx == nil {
				t.Error("expected non-nil context from InterceptorCallbackInfo")
			}
			if property == "magic" {
				val, _ := v8.NewValue(iso, int32(42))
				return val
			}
			return nil
		},
		nil,
	)

	ctx := v8.NewContext(iso, tmpl)
	defer ctx.Close()

	val, err := ctx.RunScript("magic", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 42 {
		t.Fatalf("expected 42, got %d", val.Int32())
	}
}

func TestSetNamedPropertyHandler_FallThrough(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	tmpl := v8.NewObjectTemplate(iso)
	tmpl.SetNamedPropertyHandler(
		func(property string, info *v8.InterceptorCallbackInfo) *v8.Value {
			return nil // fall through for all properties
		},
		nil,
	)

	ctx := v8.NewContext(iso, tmpl)
	defer ctx.Close()

	_, err := ctx.RunScript("var x = 10;", "setup.js")
	if err != nil {
		t.Fatal(err)
	}
	val, err := ctx.RunScript("x", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 10 {
		t.Fatalf("expected 10, got %d", val.Int32())
	}
}

func TestSetNamedPropertyHandler_Setter(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var captured string
	tmpl := v8.NewObjectTemplate(iso)
	tmpl.SetNamedPropertyHandler(
		func(property string, info *v8.InterceptorCallbackInfo) *v8.Value {
			ctx := info.Context()
			if ctx == nil {
				t.Error("expected non-nil context in getter")
			}
			if property == "intercepted" {
				val, _ := v8.NewValue(iso, captured)
				return val
			}
			return nil
		},
		func(property string, value *v8.Value, info *v8.InterceptorCallbackInfo) bool {
			ctx := info.Context()
			if ctx == nil {
				t.Error("expected non-nil context in setter")
			}
			if property == "intercepted" {
				captured = value.String()
				return true
			}
			return false
		},
	)

	ctx := v8.NewContext(iso, tmpl)
	defer ctx.Close()

	_, err := ctx.RunScript("intercepted = 'hello';", "set.js")
	if err != nil {
		t.Fatal(err)
	}
	if captured != "hello" {
		t.Fatalf("expected setter to capture 'hello', got %q", captured)
	}

	val, err := ctx.RunScript("intercepted", "get.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", val.String())
	}
}
