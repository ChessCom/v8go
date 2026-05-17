package v8go_test

import (
	"sync/atomic"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestSetPromiseRejectCallback(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var rejectEvent atomic.Int32
	iso.SetPromiseRejectCallback(func(msg v8.PromiseRejectMessage) {
		rejectEvent.Store(int32(msg.Event))
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("Promise.reject('fail')", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	ctx.PerformMicrotaskCheckpoint()

	if rejectEvent.Load() != int32(v8.PromiseRejectWithNoHandler) {
		t.Fatalf("expected PromiseRejectWithNoHandler (0), got %d", rejectEvent.Load())
	}
}

func TestSetPromiseRejectCallback_HandlerAdded(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var events []v8.PromiseRejectEvent
	iso.SetPromiseRejectCallback(func(msg v8.PromiseRejectMessage) {
		events = append(events, msg.Event)
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript(`
		const p = Promise.reject('fail');
		p.catch(() => {});
	`, "test.js")
	if err != nil {
		t.Fatal(err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0] != v8.PromiseRejectWithNoHandler {
		t.Fatalf("expected first event to be PromiseRejectWithNoHandler, got %d", events[0])
	}
	if events[1] != v8.PromiseHandlerAddedAfterReject {
		t.Fatalf("expected second event to be PromiseHandlerAddedAfterReject, got %d", events[1])
	}
}

func TestSetPromiseRejectCallback_WithValue(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var rejectValue string
	iso.SetPromiseRejectCallback(func(msg v8.PromiseRejectMessage) {
		if msg.Event == v8.PromiseRejectWithNoHandler && msg.Value != nil {
			rejectValue = msg.Value.String()
		}
	})

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("Promise.reject('error_message')", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	ctx.PerformMicrotaskCheckpoint()

	if rejectValue != "error_message" {
		t.Fatalf("expected 'error_message', got %q", rejectValue)
	}
}
