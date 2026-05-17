package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestAddNearHeapLimitCallback(t *testing.T) {
	iso := v8.NewIsolate(v8.WithoutDefaultHeapLimitCallback())
	defer iso.Dispose()

	var callbackArgs [2]uint64
	iso.AddNearHeapLimitCallback(func(current, initial uint64) uint64 {
		callbackArgs[0] = current
		callbackArgs[1] = initial
		return current * 2
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Just run normal JS — the callback won't fire unless heap pressure
	// is high enough. We verify the API doesn't panic.
	val, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 2 {
		t.Fatalf("expected 2, got %d", val.Int32())
	}
}

func TestRemoveNearHeapLimitCallback(t *testing.T) {
	iso := v8.NewIsolate(v8.WithoutDefaultHeapLimitCallback())
	defer iso.Dispose()

	var called bool
	iso.AddNearHeapLimitCallback(func(current, initial uint64) uint64 {
		called = true
		return current * 2
	})
	iso.RemoveNearHeapLimitCallback(0)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("callback should not have been called after removal")
	}
}

func TestWithoutDefaultHeapLimitCallback(t *testing.T) {
	iso := v8.NewIsolate(v8.WithoutDefaultHeapLimitCallback())
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("42", "test.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if val.Int32() != 42 {
		t.Fatalf("expected 42, got %d", val.Int32())
	}
}

func TestWithoutDefaultHeapLimitCallback_WithConstraints(t *testing.T) {
	iso := v8.NewIsolate(
		v8.WithResourceConstraints(0, 50*1024*1024),
		v8.WithoutDefaultHeapLimitCallback(),
	)
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("'hello'", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", val.String())
	}
}
