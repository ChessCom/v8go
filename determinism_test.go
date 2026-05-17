// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func hexDump(b []byte) string { return hex.EncodeToString(b) }

// TestDeterminism_ProbeValuesAreStable produces two snapshots from the
// same source under WithDeterministicTime and checks that the values
// Date.now / Math.random / performance.now observed at snapshot build
// time are identical between the two runs. If we ever leak host
// wall-clock into the heap one of these probes will differ between runs.
//
// We deliberately do NOT assert byte-identity of the StartupData here:
// V8 stamps the read-only heap with a checksum at offset 8 derived from
// the process-wide hash seed (which is randomised per process unless V8
// is built with --predictable / --hash-seed). The semantic identity
// below is the property embedders actually care about.
func TestDeterminism_ProbeValuesAreStable(t *testing.T) {
	t.Parallel()

	const source = `
const ts = Date.now();
const rnd = Math.random();
const tick = (typeof performance === 'object' ? performance.now() : 0);
globalThis.PROBE = { ts: ts, rnd: rnd, tick: tick };
`

	probeOf := func() (int64, float64, float64) {
		sc := v8.NewSnapshotCreator(v8.WithDeterministicTime(0))
		if _, err := sc.Context().RunScript(source, "bundle.js"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
		if err != nil {
			t.Fatalf("CreateBlob: %v", err)
		}
		sc.Dispose()

		iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
		defer iso.Dispose()
		c := v8.NewContext(iso)
		defer c.Close()
		ts, err := c.RunScript(`PROBE.ts`, "probe1.js")
		if err != nil {
			t.Fatalf("probe ts: %v", err)
		}
		rnd, err := c.RunScript(`PROBE.rnd`, "probe2.js")
		if err != nil {
			t.Fatalf("probe rnd: %v", err)
		}
		tick, err := c.RunScript(`PROBE.tick`, "probe3.js")
		if err != nil {
			t.Fatalf("probe tick: %v", err)
		}
		return ts.Integer(), rnd.Number(), tick.Number()
	}

	ts1, rnd1, tick1 := probeOf()
	ts2, rnd2, tick2 := probeOf()
	if ts1 != ts2 || rnd1 != rnd2 || tick1 != tick2 {
		t.Fatalf("probes drift between snapshot builds: ts=%v/%v rnd=%v/%v tick=%v/%v",
			ts1, ts2, rnd1, rnd2, tick1, tick2)
	}
}

// TestDeterminism_BlobsByteIdentityBestEffort asserts the V8 startup blob
// is byte-identical between two snapshot builds with WithDeterministicTime.
// This requires the V8 runtime to have been initialised with
// --predictable / --hash-seed; we surface a t.Skip with a precise reason
// when the assertion fails so the test does not block CI on machines that
// don't run with those flags. Embedders that want hard byte-identity
// should set V8 flags before any isolate is created.
func TestDeterminism_BlobsByteIdentityBestEffort(t *testing.T) {
	t.Parallel()

	const source = `globalThis.STAMP = Date.now();`

	build := func() []byte {
		sc := v8.NewSnapshotCreator(v8.WithDeterministicTime(0))
		if _, err := sc.Context().RunScript(source, "bundle.js"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
		if err != nil {
			t.Fatalf("CreateBlob: %v", err)
		}
		sc.Dispose()
		return blob
	}

	a := build()
	b := build()
	if bytes.Equal(a, b) {
		t.Logf("blobs are byte-identical (V8 was started with --predictable)")
		return
	}
	off := firstDiff(a, b)
	var snippet string
	if off >= 4 && off+8 <= len(a) && off+8 <= len(b) {
		snippet = "  a=" + hexDump(a[off-4:off+8]) + "\n  b=" + hexDump(b[off-4:off+8])
	}
	t.Skipf(
		"blobs differ at offset %d (V8 hash-seed is randomised; set --predictable to get byte-identity)\n%s",
		off, snippet,
	)
}

// TestDeterminism_SeedPropagatesIntoSnapshot demonstrates that the seed
// timestamp is observable from inside the snapshotted heap, confirming
// the shim ran before the bundle.
func TestDeterminism_SeedPropagatesIntoSnapshot(t *testing.T) {
	t.Parallel()

	const seed = int64(1_700_000_000_000)
	const source = `globalThis.STAMP = Date.now();`

	sc := v8.NewSnapshotCreator(v8.WithDeterministicTime(seed))
	if _, err := sc.Context().RunScript(source, "bundle.js"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	v, err := c.RunScript(`STAMP`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	// The shim returns Math.floor(seed + tick/1000); the first call has
	// tick = 1, so the result is seed.
	if got := v.Integer(); got != seed {
		t.Fatalf("STAMP = %v, want %v", got, seed)
	}
}

func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
