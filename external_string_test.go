package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestNewExternalOneByteString_Basic(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte("hello external")
	val, err := v8.NewExternalOneByteString(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	if !val.IsString() {
		t.Fatal("expected string value")
	}
	if val.String() != "hello external" {
		t.Fatalf("expected 'hello external', got %q", val.String())
	}
}

func TestNewExternalOneByteString_Empty(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := v8.NewExternalOneByteString(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if val.String() != "" {
		t.Fatalf("expected empty string, got %q", val.String())
	}
}

func TestNewExternalOneByteString_JSInterop(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := []byte("external-test")
	val, err := v8.NewExternalOneByteString(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	ctx.Global().Set("ext", val)
	result, err := ctx.RunScript("ext.length", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if result.Int32() != 13 {
		t.Fatalf("expected length 13, got %d", result.Int32())
	}
}

func TestNewExternalOneByteString_Large(t *testing.T) {
	ctx := v8.NewContext()
	defer ctx.Close()

	data := make([]byte, 1024*1024) // 1 MiB
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}

	val, err := v8.NewExternalOneByteString(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	ctx.Global().Set("bigstr", val)
	result, err := ctx.RunScript("bigstr.length", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if result.Int32() != 1024*1024 {
		t.Fatalf("expected length %d, got %d", 1024*1024, result.Int32())
	}
}
