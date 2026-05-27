package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestQMTInRestoredIsolate(t *testing.T) {
	// Create a minimal snapshot
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()
	ctx.RunScript("var x = 1;", "s.js")
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	// Create isolate from snapshot
	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()

	// Context::FromSnapshot path
	c1 := v8.NewContext(iso)
	defer c1.Close()
	v1, _ := c1.RunScript("typeof queueMicrotask", "q.js")
	t.Logf("FromSnapshot context: qMT = %s", v1.String())

	// Regular Context::New path (by creating a second context with
	// a global template — bypasses Context::FromSnapshot)
	// Actually v8go doesn't expose this. Let me just check the value.
}
