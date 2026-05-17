package v8go_test

import (
	"sync/atomic"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestAddGCPrologueCallback(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var called atomic.Int32
	iso.AddGCPrologueCallback(func(gcType v8.GCType) {
		called.Add(1)
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if called.Load() == 0 {
		t.Fatal("expected GC prologue callback to be called at least once")
	}
	t.Logf("prologue called %d times", called.Load())
}

func TestAddGCEpilogueCallback(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var called atomic.Int32
	iso.AddGCEpilogueCallback(func(gcType v8.GCType) {
		called.Add(1)
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if called.Load() == 0 {
		t.Fatal("expected GC epilogue callback to be called at least once")
	}
	t.Logf("epilogue called %d times", called.Load())
}

func TestGCCallbacks_PrologueAndEpilogue(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var order []string
	iso.AddGCPrologueCallback(func(gcType v8.GCType) {
		order = append(order, "prologue")
	})
	iso.AddGCEpilogueCallback(func(gcType v8.GCType) {
		order = append(order, "epilogue")
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if len(order) == 0 {
		t.Fatal("expected some GC callbacks")
	}
	t.Logf("callback order: %v (total %d events)", order[:min(10, len(order))], len(order))
}

func TestRemoveGCPrologueCallbacks(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var called atomic.Int32
	iso.AddGCPrologueCallback(func(gcType v8.GCType) {
		called.Add(1)
	})
	iso.RemoveGCPrologueCallbacks()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 50; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if called.Load() != 0 {
		t.Fatalf("expected no prologue callbacks after removal, got %d", called.Load())
	}
}

func TestRemoveGCEpilogueCallbacks(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var called atomic.Int32
	iso.AddGCEpilogueCallback(func(gcType v8.GCType) {
		called.Add(1)
	})
	iso.RemoveGCEpilogueCallbacks()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 50; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if called.Load() != 0 {
		t.Fatalf("expected no epilogue callbacks after removal, got %d", called.Load())
	}
}

func TestGCCallbackType(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var receivedType atomic.Int32
	iso.AddGCPrologueCallback(func(gcType v8.GCType) {
		receivedType.Store(int32(gcType))
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		ctx.RunScript("new Array(1000).fill('x')", "alloc.js")
	}
	iso.LowMemoryNotification()

	if receivedType.Load() == 0 {
		t.Fatal("expected to receive a non-zero GC type")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
