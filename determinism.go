// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

import (
	"fmt"
)

// nonDeterminismShim is the JavaScript prelude installed on a
// SnapshotCreator context when WithDeterministicTime is requested. It pins
// every well-known non-deterministic globals to constant values so a
// bundle evaluated during snapshot creation cannot bake stale wall-clock
// time, random numbers, or performance counters into the heap.
//
// The variable __V8GO_DET_TIME is the seed timestamp in milliseconds since
// the Unix epoch and __V8GO_DET_TICK is a monotonically increasing nano
// counter used by performance.now() / Date.now() on subsequent calls. We
// increment the tick on each call so back-to-back reads never compare
// equal — code that relies on time deltas at least sees a strictly
// monotonic sequence rather than a frozen value.
const nonDeterminismShim = `
(function(){
  const seed = __V8GO_DET_TIME;
  let tick = 0;
  function next() {
    tick = tick + 1;
    return seed + (tick / 1000);
  }
  function dateNow() { return Math.floor(next()); }

  Date.now = dateNow;
  if (typeof performance === 'object' && performance) {
    performance.now = function(){ return next() - seed; };
  } else {
    globalThis.performance = { now: function(){ return next() - seed; } };
  }

  // Math.random must remain a function; a deterministic mulberry32 PRNG
  // seeded by the wall-clock millis keeps it stable across snapshot
  // rebuilds for the same seed timestamp.
  let _r = (seed >>> 0);
  Math.random = function(){
    _r = (_r + 0x6D2B79F5) >>> 0;
    let t = _r;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return (((t ^ (t >>> 14)) >>> 0) / 4294967296);
  };
})();
`

// SeedTimeMillis is the default seed timestamp used by
// WithDeterministicTime when no explicit value is provided. It is chosen
// far from "round" wall-clock values so accidental matches in tests are
// obvious. Equivalent to 2024-01-01T00:00:00Z in milliseconds.
const SeedTimeMillis int64 = 1_704_067_200_000

// WithDeterministicTime installs the determinism shim before the embedder
// runs any bundle script. Use this whenever the bundle (or one of its
// transitive imports) calls Date.now(), Math.random(), performance.now(),
// or new Date() without arguments — without this option the resulting
// snapshot bakes the host's wall-clock at snapshot-build time into the
// heap, leading to misleading user-facing values at restore time.
//
// seedMillis selects the pinned wall-clock value (milliseconds since the
// Unix epoch). Pass 0 to use SeedTimeMillis.
func WithDeterministicTime(seedMillis int64) SnapshotCreatorOption {
	return func(cfg *snapshotCreatorConfig) {
		cfg.deterministicTime = true
		if seedMillis == 0 {
			cfg.seedMillis = SeedTimeMillis
		} else {
			cfg.seedMillis = seedMillis
		}
	}
}

// installDeterminismShim is called automatically by SnapshotCreator.Context
// when WithDeterministicTime was set. It runs the shim before the embedder
// gets a chance to run their bundle.
func (sc *SnapshotCreator) installDeterminismShim(ctx *Context, seed int64) error {
	prelude := fmt.Sprintf("const __V8GO_DET_TIME = %d;\n%s",
		seed, nonDeterminismShim)
	_, err := ctx.RunScript(prelude, "v8go-determinism.js")
	return err
}

// ResetNonDeterminism strips the determinism shim from an isolate's
// global object. Embedders restore a snapshotted isolate by calling
// NewIsolate(WithSnapshotBlob(...)), then NewContext, then optionally
// ResetNonDeterminism so live wall-clock / random behaviour returns. It
// is safe to call on isolates that were never snapshotted under the shim.
//
// Note: this strictly restores Date.now / Math.random / performance.now
// to V8's intrinsic implementations. It does NOT replace globalThis.Date
// itself, because the snapshot did not wrap it.
func ResetNonDeterminism(ctx *Context) error {
	const reset = `
(function(){
  // Removing an own property exposes the prototype's intrinsic. On V8,
  // Math.random / Date.now / performance.now are configurable own
  // properties of their respective host objects, so 'delete' restores
  // the intrinsic in one step.
  try { delete Math.random; } catch (e) {}
  try { delete Date.now; } catch (e) {}
  if (typeof performance === 'object' && performance) {
    try { delete performance.now; } catch (e) {}
  }
})();
`
	_, err := ctx.RunScript(reset, "v8go-determinism-reset.js")
	return err
}
