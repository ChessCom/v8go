package v8go

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SnapshotMagic is the on-disk magic prefix for packed v8go snapshots.
// The trailing byte is the wire-format version; bump it whenever the
// header layout changes in a backwards-incompatible way.
const SnapshotMagic = "BFV8\x01"

// snapshotMagicLen is the length of the magic prefix in bytes.
const snapshotMagicLen = 5

// snapshotHeaderLengthSize is the size in bytes of the header-length
// uint16 field that immediately follows the magic.
const snapshotHeaderLengthSize = 2

// PackedSnapshot is the durable, self-describing container around a raw
// v8::StartupData blob. It carries the metadata a consumer needs to
// decide whether the blob is safe to load: V8 ABI tag, external
// references digest, and a sha256 of the source bundle. Embedders are
// free to attach arbitrary string-keyed metadata in Extra.
//
// Layout written by Marshal (little-endian):
//
//	[0..5)        magic "BFV8\x01"
//	[5..7)        uint16 header length H
//	[7..H)        JSON header (Header below)
//	[H..end)      raw v8::StartupData bytes
//
// The blob field is intentionally placed outside the JSON header so
// large snapshots stay byte-addressable without a base64 round-trip.
type PackedSnapshot struct {
	V8ABI        string               `json:"v8_abi"`
	RefsDigest   string               `json:"refs_digest"`
	BundleSHA256 string               `json:"bundle_sha256"`
	CreatedUnix  int64                `json:"created_unix"`
	FCH          FunctionCodeHandling `json:"fch"`
	Extra        map[string]string    `json:"extra,omitempty"`

	// Blob is the raw v8::StartupData. Not embedded in the JSON header.
	Blob []byte `json:"-"`
}

// ErrIncompatible is returned when a PackedSnapshot cannot be safely
// consumed by the running v8go process — typically because the V8 ABI
// tag or external-references digest mismatch. RestoreIsolate wraps the
// specific reason via errors.Join / fmt.Errorf("%w: ...") so callers
// can use errors.Is(err, ErrIncompatible).
var ErrIncompatible = errors.New("v8go: incompatible packed snapshot")

// ErrCorruptSnapshot indicates the byte stream does not look like a
// valid PackedSnapshot at all (wrong magic, truncated header, invalid
// JSON, ...). Distinct from ErrIncompatible: corruption usually means
// data-handling bugs upstream, not ABI drift.
var ErrCorruptSnapshot = errors.New("v8go: corrupt packed snapshot")

// Marshal serialises the PackedSnapshot to a single contiguous byte
// stream. The returned slice is safe to write to disk or send over the
// wire. Marshal panics if Blob is empty; an empty blob always indicates
// a programming error (e.g. forgetting to assign CreateBlob's output).
func (p *PackedSnapshot) Marshal() ([]byte, error) {
	if len(p.Blob) == 0 {
		return nil, fmt.Errorf("v8go: PackedSnapshot.Marshal: blob is empty")
	}

	header := *p
	// Strip the blob from the header so it isn't double-encoded.
	header.Blob = nil
	headerJSON, err := json.Marshal(&header)
	if err != nil {
		return nil, fmt.Errorf("v8go: PackedSnapshot.Marshal: %w", err)
	}
	if len(headerJSON) > 0xFFFF {
		return nil, fmt.Errorf(
			"v8go: PackedSnapshot.Marshal: header is %d bytes, max is %d",
			len(headerJSON), 0xFFFF,
		)
	}

	out := make([]byte, 0, snapshotMagicLen+snapshotHeaderLengthSize+len(headerJSON)+len(p.Blob))
	out = append(out, SnapshotMagic...)
	hlen := make([]byte, snapshotHeaderLengthSize)
	binary.LittleEndian.PutUint16(hlen, uint16(len(headerJSON)))
	out = append(out, hlen...)
	out = append(out, headerJSON...)
	out = append(out, p.Blob...)
	return out, nil
}

// UnmarshalPackedSnapshot decodes a byte stream produced by Marshal back
// into a PackedSnapshot. It validates the magic prefix and parses the
// JSON header, but does NOT enforce ABI / refs-digest compatibility:
// that is RestoreIsolate's job, so callers can still inspect metadata
// (V8ABI, BundleSHA256, ...) on incompatible blobs for diagnostics or
// fallback routing.
func UnmarshalPackedSnapshot(b []byte) (*PackedSnapshot, error) {
	if len(b) < snapshotMagicLen+snapshotHeaderLengthSize {
		return nil, fmt.Errorf(
			"%w: byte stream is %d bytes, need at least %d",
			ErrCorruptSnapshot, len(b),
			snapshotMagicLen+snapshotHeaderLengthSize,
		)
	}
	if string(b[:snapshotMagicLen]) != SnapshotMagic {
		return nil, fmt.Errorf(
			"%w: magic prefix %q does not match expected %q",
			ErrCorruptSnapshot,
			b[:snapshotMagicLen], SnapshotMagic,
		)
	}
	headerLen := binary.LittleEndian.Uint16(
		b[snapshotMagicLen : snapshotMagicLen+snapshotHeaderLengthSize],
	)
	headerStart := snapshotMagicLen + snapshotHeaderLengthSize
	headerEnd := headerStart + int(headerLen)
	if headerEnd > len(b) {
		return nil, fmt.Errorf(
			"%w: header declares %d bytes but stream is %d bytes",
			ErrCorruptSnapshot, headerLen, len(b),
		)
	}

	out := &PackedSnapshot{}
	if err := json.Unmarshal(b[headerStart:headerEnd], out); err != nil {
		return nil, fmt.Errorf("%w: header JSON: %v", ErrCorruptSnapshot, err)
	}
	out.Blob = append([]byte(nil), b[headerEnd:]...)
	if len(out.Blob) == 0 {
		return nil, fmt.Errorf("%w: no v8::StartupData bytes after header", ErrCorruptSnapshot)
	}
	return out, nil
}

// PackOptions configure PackBundle.
type PackOptions struct {
	// Source is the JavaScript bundle to evaluate before serialisation.
	// Must be non-empty.
	Source string

	// Origin is the script origin recorded in stack traces. Defaults to
	// "bundle.js" if blank.
	Origin string

	// FCH controls how compiled functions are encoded. FunctionCodeKeep
	// produces a fully warm blob, FunctionCodeClear is smaller but
	// requires recompilation on first call.
	FCH FunctionCodeHandling

	// DeterministicTime, when true, installs the determinism shim so the
	// bundle cannot bake host wall-clock / random into the heap.
	// SeedMillis selects the pinned timestamp; 0 means SeedTimeMillis.
	DeterministicTime bool
	SeedMillis        int64

	// ExistingBlob warm-starts the SnapshotCreator from a prior pack so
	// snapshots can be stacked (e.g. base runtime + app overlay).
	ExistingBlob []byte

	// Extra is copied verbatim into PackedSnapshot.Extra. Use it to
	// attach app-specific metadata (build SHA, route, etc.).
	Extra map[string]string
}

// PackBundle evaluates the given JavaScript source on a fresh
// SnapshotCreator, serialises the resulting heap, and wraps the bytes
// in a PackedSnapshot annotated with the running V8 ABI tag and the
// frozen external_references digest. The returned PackedSnapshot is
// ready to be Marshalled to disk or transmitted to consumers.
func PackBundle(opts PackOptions) (*PackedSnapshot, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("v8go: PackBundle: Source is required")
	}
	origin := opts.Origin
	if origin == "" {
		origin = "bundle.js"
	}

	scOpts := []SnapshotCreatorOption{}
	if len(opts.ExistingBlob) > 0 {
		scOpts = append(scOpts, WithExistingSnapshotBlob(opts.ExistingBlob))
	}
	if opts.DeterministicTime {
		scOpts = append(scOpts, WithDeterministicTime(opts.SeedMillis))
	}

	sc := NewSnapshotCreator(scOpts...)
	defer sc.Dispose()

	ctx := sc.Context()
	if _, err := ctx.RunScript(opts.Source, origin); err != nil {
		return nil, fmt.Errorf("v8go: PackBundle: bundle eval failed: %w", err)
	}
	blob, err := sc.CreateBlob(opts.FCH)
	if err != nil {
		return nil, fmt.Errorf("v8go: PackBundle: CreateBlob failed: %w", err)
	}

	bundleSum := sha256.Sum256([]byte(opts.Source))

	p := &PackedSnapshot{
		V8ABI:        Version(),
		RefsDigest:   ExternalReferenceRegistryDigest(),
		BundleSHA256: hex.EncodeToString(bundleSum[:]),
		CreatedUnix:  time.Now().Unix(),
		FCH:          opts.FCH,
		Blob:         blob,
	}
	if len(opts.Extra) > 0 {
		p.Extra = make(map[string]string, len(opts.Extra))
		for k, v := range opts.Extra {
			p.Extra[k] = v
		}
	}
	return p, nil
}

// RestoreOptions configure PackedSnapshot.RestoreIsolate.
type RestoreOptions struct {
	// AllowABIMismatch lets the consumer load a blob whose V8 ABI tag
	// differs from the running V8. This is almost always wrong — V8
	// snapshots are version-locked — but is exposed for advanced
	// recovery scenarios. The default (false) returns ErrIncompatible.
	AllowABIMismatch bool

	// AllowRefsDigestMismatch lets the consumer load a blob whose
	// external_references digest differs from the running registry.
	// Allowing this when the snapshot baked in Go-backed FunctionTemplate
	// references will crash the isolate the first time one is invoked,
	// so the default (false) returns ErrIncompatible.
	AllowRefsDigestMismatch bool

	// ResourceConstraints are forwarded to NewIsolate.
	ResourceConstraints *isolateResourceConstraints
}

// isolateResourceConstraints mirrors the unexported resourceConstraints
// type from isolate.go so RestoreOptions can be declared in the public
// API without leaking the internal name. Use NewResourceConstraints to
// construct one.
type isolateResourceConstraints struct {
	InitialHeapSizeInBytes uint64
	MaxHeapSizeInBytes     uint64
}

// NewResourceConstraints is a constructor for the RestoreOptions
// ResourceConstraints field.
func NewResourceConstraints(initialHeapSizeInBytes, maxHeapSizeInBytes uint64) *isolateResourceConstraints {
	return &isolateResourceConstraints{
		InitialHeapSizeInBytes: initialHeapSizeInBytes,
		MaxHeapSizeInBytes:     maxHeapSizeInBytes,
	}
}

// minPlausibleV8Blob is the smallest size a real v8::StartupData blob
// could occupy. V8's serialized snapshot has a fixed-size header
// (number of contexts, rehashability, checksum, version string, and
// per-context offsets) plus a non-empty heap. We use a conservative
// 1 KiB floor to reject obviously truncated or garbage payloads before
// V8 can fatal-abort on them. Real snapshots are tens to hundreds of
// kilobytes.
const minPlausibleV8Blob = 1024

// RestoreIsolate validates compatibility and constructs an isolate
// initialised from this snapshot. On compatibility failure it returns
// errors that wrap ErrIncompatible so callers can use errors.Is and
// fall back to a cold-start path.
//
// RestoreIsolate also enforces a minimum plausible blob size
// (minPlausibleV8Blob) before handing bytes to V8: V8 fatal-aborts
// rather than returning an error on truncated snapshots, and a Go-side
// abort is the worst possible failure mode for a server process.
func (p *PackedSnapshot) RestoreIsolate(opts RestoreOptions) (*Isolate, error) {
	if p == nil {
		return nil, fmt.Errorf("v8go: PackedSnapshot.RestoreIsolate: nil receiver")
	}
	if len(p.Blob) == 0 {
		return nil, fmt.Errorf("v8go: PackedSnapshot.RestoreIsolate: empty blob")
	}
	if len(p.Blob) < minPlausibleV8Blob {
		return nil, fmt.Errorf(
			"%w: blob is %d bytes, must be at least %d",
			ErrIncompatible, len(p.Blob), minPlausibleV8Blob,
		)
	}

	runtimeABI := Version()
	if p.V8ABI != runtimeABI && !opts.AllowABIMismatch {
		return nil, fmt.Errorf(
			"%w: snapshot V8 ABI %q != runtime %q (pass AllowABIMismatch to override at your own risk)",
			ErrIncompatible, p.V8ABI, runtimeABI,
		)
	}

	runtimeDigest := ExternalReferenceRegistryDigest()
	if p.RefsDigest != runtimeDigest && !opts.AllowRefsDigestMismatch {
		return nil, fmt.Errorf(
			"%w: snapshot refs digest %q != runtime %q",
			ErrIncompatible, p.RefsDigest, runtimeDigest,
		)
	}

	isoOpts := []IsolateOption{WithSnapshotBlob(p.Blob)}
	if opts.ResourceConstraints != nil {
		isoOpts = append(isoOpts, WithResourceConstraints(
			opts.ResourceConstraints.InitialHeapSizeInBytes,
			opts.ResourceConstraints.MaxHeapSizeInBytes,
		))
	}
	return NewIsolate(isoOpts...), nil
}
