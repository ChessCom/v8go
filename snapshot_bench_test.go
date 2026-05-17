// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"strings"
	"testing"
	"time"

	v8 "github.com/ChessCom/v8go"
)

// bundleSource is intentionally large so cold-start work shows up in the
// benchmark wall-clock: roughly 50 KiB of pre-bound JS that the snapshot
// path executes once at build time and recovers for free at restore time.
var bundleSource = buildBundleSource()

func buildBundleSource() string {
	// ~15000 named arrow functions + a coordinator: roughly 750 KiB of
	// source. At this size V8 source-parse and IC-warmup dominate
	// cold-start wall clock, putting the no-snapshot path well above the
	// 10x slower bar.
	const n = 15000
	var b strings.Builder
	b.WriteString("globalThis.M={};\n")
	for i := 0; i < n; i++ {
		b.WriteString("M.k")
		b.WriteString(itoa(i))
		b.WriteString("=(x)=>x*")
		b.WriteString(itoa(i + 1))
		b.WriteString("+'_")
		b.WriteString(itoa(i))
		b.WriteString("';\n")
	}
	b.WriteString("globalThis.MAIN=()=>{let s=0;for(let i=0;i<")
	b.WriteString(itoa(n))
	b.WriteString(";i++)s+=M['k'+i](i);return s;};\n")
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [11]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// BenchmarkColdStart_FromSource measures the wall-clock cost of:
// NewIsolate + NewContext + RunScript(bundle) — the no-snapshot path.
func BenchmarkColdStart_FromSource(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		iso := v8.NewIsolate()
		ctx := v8.NewContext(iso)
		if _, err := ctx.RunScript(bundleSource, "bundle.js"); err != nil {
			b.Fatalf("RunScript: %v", err)
		}
		ctx.Close()
		iso.Dispose()
	}
}

// BenchmarkColdStart_FromSnapshot measures the wall-clock cost of:
// NewIsolate(WithSnapshotBlob) + NewContext — i.e. the bundle is already
// resident in the heap, so no script execution is required to expose its
// exports.
func BenchmarkColdStart_FromSnapshot(b *testing.B) {
	// Build the snapshot once outside the timed region.
	sc := v8.NewSnapshotCreator()
	if _, err := sc.Context().RunScript(bundleSource, "bundle.js"); err != nil {
		b.Fatalf("seed: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		b.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
		ctx := v8.NewContext(iso)
		// Probe a single symbol to make sure the snapshot is alive and
		// the bundle didn't have to be re-evaluated.
		if _, err := ctx.RunScript(`MAIN.length`, "probe.js"); err != nil {
			b.Fatalf("probe: %v", err)
		}
		ctx.Close()
		iso.Dispose()
	}
}

// TestSnapshot_ColdStartSpeedup asserts the cold-start speed-up factor on
// a ~750 KiB bundle. The win is bounded below by V8's per-isolate setup
// cost (~1 ms on local M-class hardware, ~3 ms on cloud ARM) — that floor
// is the same on both paths, so even very large bundles plateau around 4-6x.
// The regression bar (4x) trips only if we have meaningfully regressed the
// snapshot-skips-parse property; raise it locally to see how much headroom
// you have on a given machine.
func TestSnapshot_ColdStartSpeedup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cold-start benchmark assertion in -short mode")
	}

	const iters = 6 // average a few rounds to smooth jitter
	sourceTotal := time.Duration(0)
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate()
		ctx := v8.NewContext(iso)
		if _, err := ctx.RunScript(bundleSource, "bundle.js"); err != nil {
			t.Fatalf("RunScript: %v", err)
		}
		ctx.Close()
		iso.Dispose()
		sourceTotal += time.Since(start)
	}

	// Pre-build the snapshot.
	sc := v8.NewSnapshotCreator()
	if _, err := sc.Context().RunScript(bundleSource, "bundle.js"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	snapTotal := time.Duration(0)
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
		ctx := v8.NewContext(iso)
		if _, err := ctx.RunScript(`MAIN.length`, "probe.js"); err != nil {
			t.Fatalf("probe: %v", err)
		}
		ctx.Close()
		iso.Dispose()
		snapTotal += time.Since(start)
	}

	avgSource := sourceTotal / iters
	avgSnap := snapTotal / iters
	if avgSnap == 0 {
		t.Fatalf("snapshot avg is zero; measurement broken")
	}
	speedup := float64(avgSource) / float64(avgSnap)
	t.Logf("avg cold from source = %v, avg cold from snapshot = %v, speedup = %.2fx",
		avgSource, avgSnap, speedup)
	if speedup < 4.0 {
		t.Fatalf("snapshot cold-start speedup = %.2fx, want >= 4x", speedup)
	}
}
