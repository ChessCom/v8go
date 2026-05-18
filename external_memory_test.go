package v8go_test

import (
	"testing"

	v8 "github.com/ChessCom/v8go"
)

func TestAdjustExternalMemory_PositiveDelta(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	result := iso.AdjustExternalMemory(1024 * 1024) // +1 MiB
	if result < 1024*1024 {
		t.Fatalf("expected at least 1 MiB external memory, got %d", result)
	}
}

func TestAdjustExternalMemory_NegativeDelta(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.AdjustExternalMemory(2 * 1024 * 1024)        // +2 MiB
	result := iso.AdjustExternalMemory(-1024 * 1024) // -1 MiB

	if result < 1024*1024 {
		t.Fatalf("expected ~1 MiB remaining, got %d", result)
	}
}

func TestAdjustExternalMemory_Balanced(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	before := iso.AdjustExternalMemory(0)
	iso.AdjustExternalMemory(5 * 1024 * 1024)
	after := iso.AdjustExternalMemory(-5 * 1024 * 1024)

	if after != before {
		t.Fatalf("balanced add/remove should return to original; before=%d, after=%d", before, after)
	}
}

func TestAdjustExternalMemory_ReturnValue(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	r1 := iso.AdjustExternalMemory(10 * 1024 * 1024) // +10 MiB
	r2 := iso.AdjustExternalMemory(5 * 1024 * 1024)  // +5 MiB
	r3 := iso.AdjustExternalMemory(-3 * 1024 * 1024) // -3 MiB

	if r2 <= r1 {
		t.Fatalf("second adjustment should be larger; r1=%d, r2=%d", r1, r2)
	}
	if r3 >= r2 {
		t.Fatalf("third adjustment (negative) should decrease; r2=%d, r3=%d", r2, r3)
	}
}
