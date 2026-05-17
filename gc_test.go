package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestLowMemoryNotification(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Allocate some objects to give GC something to collect.
	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}

	before := iso.GetHeapStatistics()

	iso.LowMemoryNotification()

	after := iso.GetHeapStatistics()
	t.Logf("heap before LMN: %d, after: %d", before.UsedHeapSize, after.UsedHeapSize)
	if after.UsedHeapSize > before.UsedHeapSize {
		t.Logf("warning: heap did not shrink (may be normal for small heaps)")
	}
}

func TestLowMemoryNotification_NoContext(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	// Should not panic even without any context.
	iso.LowMemoryNotification()
}

func TestMemoryPressureNotification_None(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	iso.MemoryPressureNotification(v8.MemoryPressureNone)
}

func TestMemoryPressureNotification_Moderate(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	iso.MemoryPressureNotification(v8.MemoryPressureModerate)
}

func TestMemoryPressureNotification_Critical(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 50; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.MemoryPressureNotification(v8.MemoryPressureCritical)
}

func TestCancelTerminateExecution(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Terminate and immediately cancel before running any script.
	iso.TerminateExecution()
	iso.CancelTerminateExecution()

	// Script should run fine after cancellation.
	val, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatalf("RunScript after cancel: %v", err)
	}
	if val.Int32() != 2 {
		t.Fatalf("expected 2, got %d", val.Int32())
	}
}

func TestCancelTerminateExecution_Noop(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	// Cancelling when no termination is pending should be safe.
	iso.CancelTerminateExecution()

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

func TestRequestGarbageCollectionForTesting(t *testing.T) {
	v8.SetFlags("--expose_gc")

	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}

	// Should not panic with --expose_gc.
	iso.RequestGarbageCollectionForTesting(v8.GCTypeFull)
	iso.RequestGarbageCollectionForTesting(v8.GCTypeMinor)
}

func TestContextDisposedNotification(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	ctx.Close()

	// Notify V8 that the context was disposed.
	iso.ContextDisposedNotification(false)
}

func TestContextDisposedNotification_Dependant(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	ctx.Close()

	iso.ContextDisposedNotification(true)
}

func TestContextDisposedNotification_Multiple(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	for i := 0; i < 5; i++ {
		ctx := v8.NewContext(iso)
		ctx.Close()
		iso.ContextDisposedNotification(false)
	}
}
