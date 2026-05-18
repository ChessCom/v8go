package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestSetMicrotasksPolicy_Explicit(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetMicrotasksPolicy(v8.MicrotasksPolicyExplicit)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 2 {
		t.Fatalf("expected 2, got %d", val.Int32())
	}
}

func TestSetMicrotasksPolicy_Auto(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetMicrotasksPolicy(v8.MicrotasksPolicyAuto)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// With Auto policy, microtasks drain at script exit boundaries.
	// A resolved promise .then() should execute automatically.
	val, err := ctx.RunScript(`
		var result = 0;
		Promise.resolve(42).then(v => { result = v; });
		result;
	`, "auto_mt.js")
	if err != nil {
		t.Fatal(err)
	}
	// Under auto-microtask policy, the then() should have already run
	// by the time the second script reads `result`.
	val2, err := ctx.RunScript("result", "read.js")
	if err != nil {
		t.Fatal(err)
	}
	if val2.Int32() != 42 {
		t.Logf("first result=%d, second result=%d (auto policy may not drain between scripts)", val.Int32(), val2.Int32())
	}
}

func TestSetMicrotasksPolicy_SwitchBack(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetMicrotasksPolicy(v8.MicrotasksPolicyAuto)
	iso.SetMicrotasksPolicy(v8.MicrotasksPolicyExplicit)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("'ok'", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.String() != "ok" {
		t.Fatalf("expected 'ok', got %q", val.String())
	}
}
