// Copyright 2025 ChessCom and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"fmt"
	"unsafe"
)

// dummyFnPtr returns a non-nil unsafe.Pointer suitable for passing to
// AddExternalReference in negative-path tests where the reference is
// expected to be rejected before it is ever dereferenced.
func dummyFnPtr() unsafe.Pointer {
	var sentinel byte
	return unsafe.Pointer(&sentinel)
}

func panicMessage(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
