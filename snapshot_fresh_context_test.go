package v8go_test

import (
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
