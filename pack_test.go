// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"errors"
	"strings"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestPackBundle_HappyPath(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	const src = `
globalThis.add = (a, b) => a + b;
globalThis.greet = (n) => "hi " + n;
globalThis.K = 17;
`
	p, err := v8.PackBundle(v8.PackOptions{
		Source: src,
		Origin: "happy.js",
		FCH:    v8.FunctionCodeKeep,
		Extra:  map[string]string{"build": "test-1"},
	})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	if p.V8ABI == "" {
		t.Fatal("V8ABI is empty")
	}
	if p.RefsDigest == "" {
		t.Fatal("RefsDigest is empty")
	}
	if len(p.BundleSHA256) != 64 {
		t.Fatalf("BundleSHA256 length = %d, want 64 hex chars", len(p.BundleSHA256))
	}
	if p.CreatedUnix == 0 {
		t.Fatal("CreatedUnix not populated")
	}
	if p.Extra["build"] != "test-1" {
		t.Fatalf("Extra round-trip lost; got %v", p.Extra)
	}
	if len(p.Blob) == 0 {
		t.Fatal("empty Blob")
	}

	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	res, err := c.RunScript(`add(2, 3) + K`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := res.Integer(); got != 22 {
		t.Fatalf("add(2,3)+K = %d, want 22", got)
	}
}

func TestPackedSnapshot_MarshalUnmarshalRoundTrip(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	const src = `globalThis.X = 1;`
	p, err := v8.PackBundle(v8.PackOptions{
		Source: src,
		FCH:    v8.FunctionCodeKeep,
		Extra:  map[string]string{"k": "v"},
	})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	wire, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.HasPrefix(string(wire), v8.SnapshotMagic) {
		t.Fatalf("wire stream does not start with magic")
	}

	got, err := v8.UnmarshalPackedSnapshot(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.V8ABI != p.V8ABI ||
		got.RefsDigest != p.RefsDigest ||
		got.BundleSHA256 != p.BundleSHA256 ||
		got.CreatedUnix != p.CreatedUnix ||
		got.FCH != p.FCH ||
		got.Extra["k"] != "v" ||
		len(got.Blob) != len(p.Blob) {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", got, p)
	}

	// RestoreIsolate on the unmarshalled value works.
	iso, err := got.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate on unmarshalled: %v", err)
	}
	iso.Dispose()
}

func TestUnmarshal_NegativePaths(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	tt := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, v8.ErrCorruptSnapshot},
		{"too_short", []byte("BFV"), v8.ErrCorruptSnapshot},
		{"bad_magic", []byte("XXXX\x01\x00\x00xxx"), v8.ErrCorruptSnapshot},
		{"header_overruns", append([]byte(v8.SnapshotMagic), 0xFF, 0xFF), v8.ErrCorruptSnapshot},
		{"bad_json", append(
			[]byte(v8.SnapshotMagic),
			0x05, 0x00, // header len = 5
			'n', 'o', 't', '-', 'j',
		), v8.ErrCorruptSnapshot},
		{"no_blob", append(
			[]byte(v8.SnapshotMagic),
			0x02, 0x00, // header len = 2
			'{', '}',
		), v8.ErrCorruptSnapshot},
	}
	for _, c := range tt {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := v8.UnmarshalPackedSnapshot(c.in)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want errors.Is(%v)", err, c.want)
			}
		})
	}
}

func TestRestoreIsolate_RejectsABIMismatch(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	p, err := v8.PackBundle(v8.PackOptions{Source: `globalThis.X = 1;`})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	original := p.V8ABI
	p.V8ABI = "1.2.3-fake"

	_, err = p.RestoreIsolate(v8.RestoreOptions{})
	if !errors.Is(err, v8.ErrIncompatible) {
		t.Fatalf("err = %v, want errors.Is(ErrIncompatible)", err)
	}

	p.V8ABI = original
	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("restore after restoring V8ABI: %v", err)
	}
	iso.Dispose()
}

func TestRestoreIsolate_RejectsRefsDigestMismatch(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	p, err := v8.PackBundle(v8.PackOptions{Source: `globalThis.X = 1;`})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	p.RefsDigest = "deadbeef"

	_, err = p.RestoreIsolate(v8.RestoreOptions{})
	if !errors.Is(err, v8.ErrIncompatible) {
		t.Fatalf("err = %v, want errors.Is(ErrIncompatible)", err)
	}

	// AllowRefsDigestMismatch override.
	iso, err := p.RestoreIsolate(v8.RestoreOptions{AllowRefsDigestMismatch: true})
	if err != nil {
		t.Fatalf("AllowRefsDigestMismatch should bypass refs check: %v", err)
	}
	iso.Dispose()
}

func TestRestoreIsolate_AllowABIMismatch(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	p, err := v8.PackBundle(v8.PackOptions{Source: `globalThis.X = 1;`})
	if err != nil {
		t.Fatalf("PackBundle: %v", err)
	}
	p.V8ABI = "1.2.3-fake"

	iso, err := p.RestoreIsolate(v8.RestoreOptions{AllowABIMismatch: true})
	if err != nil {
		t.Fatalf("AllowABIMismatch should bypass ABI check: %v", err)
	}
	iso.Dispose()
}

func TestRestoreIsolate_RejectsTrucatedBlob(t *testing.T) {
	// Intentionally NOT t.Parallel(): SnapshotCreator and snapshot-restore paths touch V8 process-global state that races against parallel upstream tests on the v8 14.x binaries we ship.

	// V8 fatal-aborts on truncated snapshots; the wrapper must catch
	// them in the envelope-validation phase BEFORE V8 sees the bytes.
	p := &v8.PackedSnapshot{
		V8ABI:        v8.Version(),
		RefsDigest:   v8.ExternalReferenceRegistryDigest(),
		BundleSHA256: "00",
		Blob:         []byte{0x01, 0x02, 0x03}, // way too small
	}
	_, err := p.RestoreIsolate(v8.RestoreOptions{})
	if !errors.Is(err, v8.ErrIncompatible) {
		t.Fatalf("err = %v, want errors.Is(ErrIncompatible)", err)
	}
}
