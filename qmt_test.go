package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestQMTInFreshContext(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	// Check queueMicrotask in the original context
	v, _ := ctx.RunScript("typeof queueMicrotask", "q.js")
	t.Logf("qMT in original ctx: %s", v.String())

	sc.FreshContext(nil)
	// Now check in fresh context - but we can't run scripts
	// on the new context directly. Let's just check after restore.

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	sc.Dispose()
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	v2, err := c.RunScript("typeof queueMicrotask", "q.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	t.Logf("qMT in restored ctx: %s", v2.String())
}
