package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestSetOOMErrorHandler_Install(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var called bool
	iso.SetOOMErrorHandler(func(location string, isHeap bool) {
		called = true
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Normal execution should not trigger OOM.
	_, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("OOM handler should not be called during normal execution")
	}
}

func TestSetOOMErrorHandler_Clear(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetOOMErrorHandler(func(location string, isHeap bool) {
		t.Fatal("should not be called after clearing")
	})
	iso.SetOOMErrorHandler(nil)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("42", "test.js")
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetOOMErrorHandler_Replace(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetOOMErrorHandler(func(location string, isHeap bool) {
		t.Fatal("first handler should have been replaced")
	})

	var secondCalled bool
	iso.SetOOMErrorHandler(func(location string, isHeap bool) {
		secondCalled = true
	})
	_ = secondCalled

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("'hello'", "test.js")
	if err != nil {
		t.Fatal(err)
	}
}
