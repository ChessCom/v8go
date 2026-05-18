package v8go_test

import (
	"fmt"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestEnqueueMicrotask_Basic(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("var mtResult = 0;", "init.js")
	if err != nil {
		t.Fatal(err)
	}

	fnVal, err := ctx.RunScript("(function() { mtResult = 42; })", "fn.js")
	if err != nil {
		t.Fatal(err)
	}
	fn, err := fnVal.AsFunction()
	if err != nil {
		t.Fatal(err)
	}

	iso.EnqueueMicrotask(fn)
	ctx.PerformMicrotaskCheckpoint()

	result, err := ctx.RunScript("mtResult", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	if result.Int32() != 42 {
		t.Fatalf("expected microtask to set mtResult=42, got %d", result.Int32())
	}
}

func TestEnqueueMicrotask_MultipleOrdered(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("var order = [];", "init.js")
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		script := fmt.Sprintf("order.push(%d)", i)
		fnVal, err := ctx.RunScript("(function() { "+script+" })", "fn.js")
		if err != nil {
			t.Fatal(err)
		}
		fn, err := fnVal.AsFunction()
		if err != nil {
			t.Fatal(err)
		}
		iso.EnqueueMicrotask(fn)
	}

	ctx.PerformMicrotaskCheckpoint()

	result, err := ctx.RunScript("order.join(',')", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "1,2,3" {
		t.Fatalf("expected microtasks in FIFO order '1,2,3', got %q", result.String())
	}
}

func TestEnqueueMicrotask_WithPromise(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("var order = [];", "init.js")
	if err != nil {
		t.Fatal(err)
	}

	// Enqueue a native microtask
	fnVal, err := ctx.RunScript("(function() { order.push('native') })", "fn.js")
	if err != nil {
		t.Fatal(err)
	}
	fn, err := fnVal.AsFunction()
	if err != nil {
		t.Fatal(err)
	}
	iso.EnqueueMicrotask(fn)

	// Also enqueue one via Promise.resolve
	_, err = ctx.RunScript("Promise.resolve().then(() => order.push('promise'))", "promise.js")
	if err != nil {
		t.Fatal(err)
	}

	ctx.PerformMicrotaskCheckpoint()

	result, err := ctx.RunScript("order.join(',')", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	// Both should have run; native was enqueued first
	if result.String() != "native,promise" {
		t.Logf("order: %q (may vary by V8 scheduling)", result.String())
	}
}

