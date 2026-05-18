package v8go_test

import (
	"encoding/json"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestTakeHeapSnapshot_Basic(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript("var obj = {key: 'value', arr: [1,2,3]};", "setup.js")
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := iso.TakeHeapSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) == 0 {
		t.Fatal("expected non-empty heap snapshot")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(snapshot, &parsed); err != nil {
		t.Fatalf("snapshot should be valid JSON: %v", err)
	}

	if _, ok := parsed["nodes"]; !ok {
		t.Fatal("expected 'nodes' key in heap snapshot")
	}
	if _, ok := parsed["edges"]; !ok {
		t.Fatal("expected 'edges' key in heap snapshot")
	}
	t.Logf("snapshot size: %d bytes", len(snapshot))
}

func TestTakeHeapSnapshot_MultipleSnapshots(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	snap1, err := iso.TakeHeapSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = ctx.RunScript("var bigArray = new Array(1000).fill({x: 1});", "alloc.js")

	snap2, err := iso.TakeHeapSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	if len(snap2) <= len(snap1) {
		t.Logf("snap1=%d, snap2=%d (second may not always be larger)", len(snap1), len(snap2))
	}
}
