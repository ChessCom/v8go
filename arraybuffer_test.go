package v8go_test

import (
	"bytes"
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestNewArrayBuffer_Basic(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte{1, 2, 3, 4, 5}
	ab, err := v8.NewArrayBuffer(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	if !ab.IsArrayBuffer() {
		t.Fatal("expected ArrayBuffer")
	}
	if ab.ArrayBufferByteLength() != 5 {
		t.Fatalf("expected byte length 5, got %d", ab.ArrayBufferByteLength())
	}
}

func TestNewArrayBuffer_Contents(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte("hello world")
	ab, err := v8.NewArrayBuffer(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	got := ab.ArrayBufferGetBytes()
	if !bytes.Equal(got, data) {
		t.Fatalf("expected %q, got %q", data, got)
	}
}

func TestNewArrayBuffer_CopiesByDefault(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte("original")
	ab, err := v8.NewArrayBuffer(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	// Modify Go-side data — it should NOT affect the ArrayBuffer.
	data[0] = 'X'
	got := ab.ArrayBufferGetBytes()
	if got[0] == 'X' {
		t.Fatal("expected copy semantics: modifying Go slice should not affect ArrayBuffer")
	}
}

func TestNewArrayBufferAlloc_Basic(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	ab, err := v8.NewArrayBufferAlloc(ctx, 16)
	if err != nil {
		t.Fatal(err)
	}
	if !ab.IsArrayBuffer() {
		t.Fatal("expected ArrayBuffer")
	}
	if ab.ArrayBufferByteLength() != 16 {
		t.Fatalf("expected byte length 16, got %d", ab.ArrayBufferByteLength())
	}
}

func TestNewArrayBufferAlloc_WriteAndRead(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	ab, err := v8.NewArrayBufferAlloc(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}

	// Write directly into the backing store.
	backing := ab.ArrayBufferGetBytes()
	backing[0] = 10
	backing[1] = 20
	backing[2] = 30
	backing[3] = 40

	ctx.Global().Set("buf", ab)
	val, err := ctx.RunScript("new Uint8Array(buf)[2]", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Uint32() != 30 {
		t.Fatalf("expected 30, got %d", val.Uint32())
	}
}

func TestNewArrayBuffer_Empty(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	ab, err := v8.NewArrayBuffer(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ab.ArrayBufferByteLength() != 0 {
		t.Fatalf("expected byte length 0, got %d", ab.ArrayBufferByteLength())
	}
	got := ab.ArrayBufferGetBytes()
	if got != nil {
		t.Fatalf("expected nil bytes for empty ArrayBuffer, got %v", got)
	}
}

func TestNewArrayBuffer_JSInterop(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte{0, 0, 0, 42}
	ab, err := v8.NewArrayBuffer(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	ctx.Global().Set("buf", ab)
	val, err := ctx.RunScript("new Uint8Array(buf)[3]", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Uint32() != 42 {
		t.Fatalf("expected 42, got %d", val.Uint32())
	}
}
