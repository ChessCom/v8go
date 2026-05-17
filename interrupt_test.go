package v8go_test

import (
	"testing"
	"time"

	v8 "github.com/ChessCom/v8go"
)

func TestRequestInterrupt_FromGoroutine(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Fire interrupt from a background goroutine after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		iso.RequestInterrupt()
	}()

	_, err := ctx.RunScript("var x = 0; while(true) { x++; }", "loop.js")
	if err == nil {
		t.Fatal("expected execution to be terminated by interrupt")
	}
}

func TestSetIdle(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.SetIdle(true)
	iso.SetIdle(false)

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("1 + 1", "test.js")
	if err != nil {
		t.Fatal(err)
	}
	if val.Int32() != 2 {
		t.Fatalf("expected 2, got %d", val.Int32())
	}
}

func TestSetIdle_Toggle(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for j := 0; j < 3; j++ {
		iso.SetIdle(true)
		iso.SetIdle(false)

		val, err := ctx.RunScript("42", "test.js")
		if err != nil {
			t.Fatal(err)
		}
		if val.Int32() != 42 {
			t.Fatalf("expected 42, got %d", val.Int32())
		}
	}
}
