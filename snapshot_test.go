// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"strings"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestSnapshotRoundTrip_PureJS(t *testing.T) {
	t.Parallel()

	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()
	if _, err := ctx.RunScript(`globalThis.warm = 42; globalThis.greet = (n) => "hi " + n;`, "boot.js"); err != nil {
		t.Fatalf("seed script: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()
	if len(blob) == 0 {
		t.Fatal("expected non-empty blob")
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript(`warm`, "probe.js")
	if err != nil {
		t.Fatalf("probe warm: %v", err)
	}
	if got := v.Integer(); got != 42 {
		t.Fatalf("warm = %d, want 42", got)
	}

	v2, err := c.RunScript(`greet("world")`, "probe2.js")
	if err != nil {
		t.Fatalf("probe greet: %v", err)
	}
	if got := v2.String(); got != "hi world" {
		t.Fatalf("greet result = %q, want %q", got, "hi world")
	}
}

func TestSnapshotCreator_ConsumedReturnsNil(t *testing.T) {
	t.Parallel()

	sc := v8.NewSnapshotCreator()
	_ = sc.Context()
	if _, err := sc.CreateBlob(v8.FunctionCodeKeep); err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	if iso := sc.Isolate(); iso != nil {
		t.Fatalf("Isolate after CreateBlob should be nil, got %p", iso)
	}
	if ctx := sc.Context(); ctx != nil {
		t.Fatalf("Context after CreateBlob should be nil, got %p", ctx)
	}
	if _, err := sc.CreateBlob(v8.FunctionCodeKeep); err != v8.ErrSnapshotCreatorConsumed {
		t.Fatalf("second CreateBlob err = %v, want ErrSnapshotCreatorConsumed", err)
	}
	sc.Dispose()
	sc.Dispose() // idempotent
}

func TestExternalReferenceRegistry_FrozenAfterUse(t *testing.T) {
	// Force the registry to freeze by reading the digest.
	dig := v8.ExternalReferenceRegistryDigest()
	if dig == "" {
		t.Fatal("digest is empty after freeze")
	}
	if !v8.IsExternalReferenceRegistryFrozen() {
		t.Fatal("registry should be frozen after digest call")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("AddExternalReference should panic when registry is frozen")
		}
		msg, ok := r.(string)
		if !ok {
			// AddExternalReference panics with fmt-formatted string.
			msg = panicMessage(r)
		}
		if !strings.Contains(msg, "frozen") {
			t.Fatalf("panic message %q does not mention freeze", msg)
		}
	}()

	// Try to register a fake reference after freeze; must panic.
	dummy := dummyFnPtr()
	v8.AddExternalReference("v8go.test.LateRegistration", dummy)
}

func TestSnapshotRoundTrip_FunctionCodeClear(t *testing.T) {
	t.Parallel()

	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()
	if _, err := ctx.RunScript(`globalThis.compute = (a,b) => a*1000+b;`, "boot.js"); err != nil {
		t.Fatalf("seed script: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeClear)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	res, err := c.RunScript(`compute(3, 4)`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := res.Integer(); got != 3004 {
		t.Fatalf("compute(3,4) = %d, want 3004", got)
	}
}
