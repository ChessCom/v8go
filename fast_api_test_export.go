package v8go

// #include <stdint.h>
// extern void* v8go_test_FastAddInt32Addr();
import "C"
import "unsafe"

// TestFastAddInt32Addr returns the address of a test C++ fast function
// that adds two int32 values.
func TestFastAddInt32Addr() unsafe.Pointer {
	return C.v8go_test_FastAddInt32Addr()
}
