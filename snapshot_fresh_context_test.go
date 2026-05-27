package v8go_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

// TestFreshContext_CleanMapSerializes verifies that FreshContext produces
// a snapshot blob that deserializes without a V8 Genesis collision, even
// when the original context had 400+ global properties.
func TestFreshContext_CleanMapSerializes(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	// Install a container namespace.
	if _, err := ctx.RunScript(`
		var _bf = Object.create(null);
		_bf._deferredGlobals = [];
	`, "bootstrap.js"); err != nil {
		sc.Dispose()
		t.Fatalf("bootstrap: %v", err)
	}

	// Add 400+ properties to globalThis (simulates browserHelpers polyfills).
	var b strings.Builder
	for i := 0; i < 420; i++ {
		fmt.Fprintf(&b, "globalThis.Polyfill%d = function Polyfill%d() {};\n", i, i)
	}
	if _, err := ctx.RunScript(b.String(), "polyfills.js"); err != nil {
		sc.Dispose()
		t.Fatalf("polyfills: %v", err)
	}

	// Stash non-builtins into _bf._deferredGlobals and delete from global.
	if _, err := ctx.RunScript(`(function() {
		var builtins = {
			Object:1, Function:1, Array:1, Number:1, String:1, Boolean:1,
			Symbol:1, BigInt:1, Map:1, Set:1, WeakMap:1, WeakSet:1,
			Promise:1, Proxy:1, Reflect:1, RegExp:1, Date:1,
			Error:1, EvalError:1, RangeError:1, ReferenceError:1,
			SyntaxError:1, TypeError:1, URIError:1, AggregateError:1,
			JSON:1, Math:1, Atomics:1,
			ArrayBuffer:1, SharedArrayBuffer:1, DataView:1,
			Int8Array:1, Uint8Array:1, Uint8ClampedArray:1,
			Int16Array:1, Uint16Array:1, Int32Array:1, Uint32Array:1,
			Float32Array:1, Float64Array:1, BigInt64Array:1, BigUint64Array:1,
			WeakRef:1, FinalizationRegistry:1, Iterator:1, AsyncIterator:1,
			eval:1, parseInt:1, parseFloat:1, isNaN:1, isFinite:1,
			decodeURI:1, decodeURIComponent:1, encodeURI:1, encodeURIComponent:1,
			escape:1, unescape:1,
			undefined:1, NaN:1, Infinity:1, globalThis:1,
			console:1, Intl:1, WebAssembly:1,
			queueMicrotask:1, structuredClone:1,
			_bf:1,
		};
		var names = Object.getOwnPropertyNames(globalThis);
		for (var i = 0; i < names.length; i++) {
			var name = names[i];
			if (builtins[name]) continue;
			var desc;
			try { desc = Object.getOwnPropertyDescriptor(globalThis, name); } catch(e) { continue; }
			if (!desc) continue;
			_bf._deferredGlobals.push({n: name, v: desc.value});
			try { delete globalThis[name]; } catch(e) {}
		}
	})()`, "cleanup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("cleanup: %v", err)
	}

	// Swap to a fresh context with a clean Map.
	if err := sc.FreshContext([]string{"_bf"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	t.Logf("blob size: %d bytes", len(blob))

	// Restore on a new isolate.
	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	// Verify _bf namespace survived.
	v, err := c.RunScript("typeof _bf", "probe.js")
	if err != nil {
		t.Fatalf("probe typeof _bf: %v", err)
	}
	if v.String() != "object" {
		t.Fatalf("typeof _bf = %s, want object", v.String())
	}

	// Verify deferred globals are present.
	v2, err := c.RunScript("_bf._deferredGlobals.length", "probe2.js")
	if err != nil {
		t.Fatalf("probe deferred length: %v", err)
	}
	count := v2.Integer()
	if count < 400 {
		t.Fatalf("deferred globals = %d, want >= 400", count)
	}
	t.Logf("deferred globals: %d", count)

	// Replay deferred globals and verify one exists.
	if _, err := c.RunScript(`(function() {
		var defs = _bf._deferredGlobals;
		for (var i = 0; i < defs.length; i++) {
			globalThis[defs[i].n] = defs[i].v;
		}
	})()`, "install.js"); err != nil {
		t.Fatalf("install: %v", err)
	}

	v3, err := c.RunScript("typeof Polyfill0", "probe3.js")
	if err != nil {
		t.Fatalf("probe Polyfill0: %v", err)
	}
	if v3.String() != "function" {
		t.Fatalf("typeof Polyfill0 = %s, want function", v3.String())
	}
}

// TestFreshContext_MinimalPreservation verifies a trivial case: one
// property preserved through FreshContext.
func TestFreshContext_MinimalPreservation(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.keep = 42;", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"keep"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("keep", "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if v.Integer() != 42 {
		t.Fatalf("keep = %d, want 42", v.Integer())
	}
}

// TestFreshContext_EmptyKeepList verifies FreshContext with no properties
// to keep still produces a valid snapshot.
func TestFreshContext_EmptyKeepList(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.temp = 'gone';", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext(nil); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("typeof temp", "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if v.String() != "undefined" {
		t.Fatalf("typeof temp = %s, want undefined", v.String())
	}
}

// ---------------------------------------------------------------------------
// Adversarial / error-path regression tests
// ---------------------------------------------------------------------------

// TestFreshContext_CalledTwice verifies that calling FreshContext twice in
// sequence produces a valid snapshot (the second call replaces the context
// created by the first call).
func TestFreshContext_CalledTwice(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.a = 1; globalThis.b = 2;", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"a", "b"}); err != nil {
		sc.Dispose()
		t.Fatalf("first FreshContext: %v", err)
	}

	// Run a script on the first fresh context to set up state for the second.
	freshCtx := sc.Context()
	if _, err := freshCtx.RunScript("globalThis.c = 3;", "mid.js"); err != nil {
		sc.Dispose()
		t.Fatalf("mid script: %v", err)
	}

	if err := sc.FreshContext([]string{"a", "c"}); err != nil {
		sc.Dispose()
		t.Fatalf("second FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	// "a" was kept in both rounds.
	v, err := c.RunScript("a", "probe_a.js")
	if err != nil {
		t.Fatalf("probe a: %v", err)
	}
	if v.Integer() != 1 {
		t.Fatalf("a = %d, want 1", v.Integer())
	}

	// "c" was kept in the second round.
	v2, err := c.RunScript("c", "probe_c.js")
	if err != nil {
		t.Fatalf("probe c: %v", err)
	}
	if v2.Integer() != 3 {
		t.Fatalf("c = %d, want 3", v2.Integer())
	}

	// "b" was NOT kept in the second round — should be undefined.
	v3, err := c.RunScript("typeof b", "probe_b.js")
	if err != nil {
		t.Fatalf("probe b: %v", err)
	}
	if v3.String() != "undefined" {
		t.Fatalf("typeof b = %s, want undefined", v3.String())
	}
}

// TestFreshContext_AfterCreateBlob verifies that FreshContext returns
// ErrSnapshotCreatorConsumed when called after CreateBlob.
func TestFreshContext_AfterCreateBlob(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	_ = sc.Context()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		sc.Dispose()
		t.Fatalf("CreateBlob: %v", err)
	}
	_ = blob

	err = sc.FreshContext([]string{"x"})
	if !errors.Is(err, v8.ErrSnapshotCreatorConsumed) {
		t.Fatalf("FreshContext after CreateBlob: err = %v, want ErrSnapshotCreatorConsumed", err)
	}
	sc.Dispose()
}

// TestFreshContext_NonexistentProperty verifies that requesting a property
// name that doesn't exist on the old global does not crash and produces a
// valid snapshot where the property is undefined.
func TestFreshContext_NonexistentProperty(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.real = 99;", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"real", "nonexistent"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("real", "probe_real.js")
	if err != nil {
		t.Fatalf("probe real: %v", err)
	}
	if v.Integer() != 99 {
		t.Fatalf("real = %d, want 99", v.Integer())
	}

	v2, err := c.RunScript("typeof nonexistent", "probe_missing.js")
	if err != nil {
		t.Fatalf("probe nonexistent: %v", err)
	}
	if v2.String() != "undefined" {
		t.Fatalf("typeof nonexistent = %s, want undefined", v2.String())
	}
}

// TestFreshContext_DuplicateKeepNames verifies that duplicate names in the
// keep list do not crash and the property is preserved correctly.
func TestFreshContext_DuplicateKeepNames(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.dup = 'hello';", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"dup", "dup", "dup"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("dup", "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if v.String() != "hello" {
		t.Fatalf("dup = %s, want 'hello'", v.String())
	}
}

// TestFreshContext_RunScriptAfterFresh verifies that RunScript works on the
// new context returned by FreshContext and that values are tracked correctly
// for CreateBlob.
func TestFreshContext_RunScriptAfterFresh(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.ns = {v: 1};", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"ns"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	freshCtx := sc.Context()
	if _, err := freshCtx.RunScript("globalThis.added = ns.v + 100;", "post.js"); err != nil {
		sc.Dispose()
		t.Fatalf("post-FreshContext RunScript: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("added", "probe.js")
	if err != nil {
		t.Fatalf("probe added: %v", err)
	}
	if v.Integer() != 101 {
		t.Fatalf("added = %d, want 101", v.Integer())
	}
}

// TestFreshContext_DeterminismShimPreserved verifies that the determinism
// shim is automatically reinstalled on the fresh context when
// WithDeterministicTime was set.
func TestFreshContext_DeterminismShimPreserved(t *testing.T) {
	seed := int64(1_700_000_000_000)
	sc := v8.NewSnapshotCreator(v8.WithDeterministicTime(seed))
	ctx := sc.Context()

	if _, err := ctx.RunScript("globalThis.ns = {stamp: Date.now()};", "setup.js"); err != nil {
		sc.Dispose()
		t.Fatalf("setup: %v", err)
	}

	if err := sc.FreshContext([]string{"ns"}); err != nil {
		sc.Dispose()
		t.Fatalf("FreshContext: %v", err)
	}

	freshCtx := sc.Context()
	// Date.now() should return the deterministic seed, not wall-clock time.
	if _, err := freshCtx.RunScript("globalThis.postFreshStamp = Date.now();", "post.js"); err != nil {
		sc.Dispose()
		t.Fatalf("post-FreshContext RunScript: %v", err)
	}

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript("postFreshStamp", "probe.js")
	if err != nil {
		t.Fatalf("probe postFreshStamp: %v", err)
	}
	stamp := v.Integer()
	// The determinism shim returns values close to the seed (seed + tick/1000).
	// With real wall-clock time the value would be ~1.7+ trillion (current time).
	// We check that the stamp is within a reasonable range of the seed.
	if stamp < seed || stamp > seed+1000 {
		t.Fatalf("postFreshStamp = %d, want ~%d (determinism shim not active)", stamp, seed)
	}
}

// TestFreshContext_BeforeContextCreated verifies that FreshContext returns
// an error when called before Context() has been invoked.
func TestFreshContext_BeforeContextCreated(t *testing.T) {
	sc := v8.NewSnapshotCreator()

	err := sc.FreshContext([]string{"x"})
	if err == nil {
		sc.Dispose()
		t.Fatal("FreshContext before Context() should return an error")
	}
	if !strings.Contains(err.Error(), "before Context()") {
		sc.Dispose()
		t.Fatalf("unexpected error: %v", err)
	}
	sc.Dispose()
}
