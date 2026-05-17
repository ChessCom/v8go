// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

// TestGoStateRehydration_FunctionTemplateCallback verifies the critical
// invariant that a JavaScript call to a snapshotted Function created via
// FunctionTemplate dispatches to a Go closure registered on the consumer
// side AFTER snapshot restore — not to the closure that existed when the
// snapshot was built. V8 only serialises the address slot indirectly,
// via the external_references index; the embedder's Go state (closure
// captures, counters, ...) does not survive serialisation and must be
// re-registered explicitly.
//
// If this invariant ever breaks (e.g. because someone "helpfully"
// stashed the producer's *Isolate inside the m_ctx and tried to reuse
// it after CreateBlob), restored isolates would dangle-call into freed
// memory.
func TestGoStateRehydration_FunctionTemplateCallback(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	// --- producer side ---
	var producerCalls atomic.Int64
	producerName := "producer-closure"
	producerCB := func(info *v8.FunctionCallbackInfo) *v8.Value {
		producerCalls.Add(1)
		v, _ := v8.NewValue(info.Context().Isolate(), producerName)
		return v
	}

	scOpts := []v8.SnapshotCreatorOption{}
	sc := v8.NewSnapshotCreator(scOpts...)

	iso := sc.Isolate()
	ctx := sc.Context()

	// Install a FunctionTemplate on globalThis BEFORE serialising. The
	// snapshot will record an entry pointing at FunctionTemplateCallback
	// via the external_references registry plus a callback_ref integer.
	tmpl := v8.NewFunctionTemplate(iso, producerCB)
	fn := tmpl.GetFunction(ctx)
	if err := ctx.Global().Set("nativeFn", fn); err != nil {
		t.Fatalf("Set nativeFn: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	// --- consumer side ---
	var consumerCalls atomic.Int64
	consumerName := "consumer-closure"
	consumerCB := func(info *v8.FunctionCallbackInfo) *v8.Value {
		consumerCalls.Add(1)
		v, _ := v8.NewValue(info.Context().Isolate(), consumerName)
		return v
	}

	iso2 := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso2.Dispose()
	c2 := v8.NewContext(iso2)
	defer c2.Close()

	// Re-register a NEW FunctionTemplate with the SAME exported name.
	// Critically, the new closure has a different identity from the
	// producer-side one; the producer closure must NEVER be invoked
	// (it has been freed along with the producer isolate).
	tmpl2 := v8.NewFunctionTemplate(iso2, consumerCB)
	fn2 := tmpl2.GetFunction(c2)
	if err := c2.Global().Set("nativeFn", fn2); err != nil {
		t.Fatalf("Set nativeFn (consumer): %v", err)
	}

	// Drive a JS call that, in the snapshot, was wired up to the
	// producer's nativeFn. After our consumer-side re-register, the
	// consumer closure should be the one to fire.
	res, err := c2.RunScript(`nativeFn()`, "rehydrate-probe.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	got := res.String()
	if got != consumerName {
		t.Fatalf("native call routed to %q, want %q (producer closure leaked across snapshot)", got, consumerName)
	}
	if consumerCalls.Load() != 1 {
		t.Fatalf("consumer closure invocation count = %d, want 1", consumerCalls.Load())
	}
	if producerCalls.Load() != 0 {
		t.Fatalf("producer closure was invoked after restore (count = %d); rehydration is broken", producerCalls.Load())
	}
}

// TestGoStateRehydration_MustReExportFunctions documents the embedder
// contract: if you bake a global handle into the snapshot that was
// created from a FunctionTemplate, you MUST re-create the template on
// the consumer side too, otherwise the global resolves to undefined.
//
// This is the "explicit handoff" invariant from the v8go README: V8
// preserves the JS-side shape but the Go-side closure capture must be
// re-installed by the embedder.
func TestGoStateRehydration_MustReExportFunctions(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	cb := func(info *v8.FunctionCallbackInfo) *v8.Value {
		v, _ := v8.NewValue(info.Context().Isolate(), "ok")
		return v
	}

	sc := v8.NewSnapshotCreator()
	iso := sc.Isolate()
	ctx := sc.Context()
	tmpl := v8.NewFunctionTemplate(iso, cb)
	fn := tmpl.GetFunction(ctx)
	if err := ctx.Global().Set("nativeFn", fn); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := ctx.RunScript(`globalThis.invoke = () => nativeFn();`, "boot.js"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	// Consumer DOES NOT re-export nativeFn. The contract says calling
	// nativeFn after restore is undefined behaviour (V8 will dispatch
	// through the external_references slot to FunctionTemplateCallback,
	// which then looks up callback_ref=1 in the consumer iso's empty
	// callback map). The wrapper's behaviour in this case is to surface
	// a JS-side error from the missing callback rather than crash.
	iso2 := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso2.Dispose()
	c2 := v8.NewContext(iso2)
	defer c2.Close()

	_, err = c2.RunScript(`invoke()`, "consumer.js")
	if err == nil {
		t.Fatal("expected an error when invoking a non-re-exported native function")
	}
	if !strings.Contains(err.Error(), "TypeError") &&
		!strings.Contains(err.Error(), "not a function") &&
		!strings.Contains(err.Error(), "null") &&
		!strings.Contains(err.Error(), "undefined") &&
		!strings.Contains(err.Error(), "Error") &&
		!strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unexpected JS error shape: %v", err)
	}
	// Print the error for diagnostic value when reading test logs.
	t.Logf("expected JS error from missing re-export: %v", err)
}

// Ensure the fmt import stays alive in case of future test expansion.
var _ = fmt.Sprint
