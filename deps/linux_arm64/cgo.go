package linux_arm64

// #cgo LDFLAGS: -pthread
// #cgo LDFLAGS: ${SRCDIR}/libv8-0.a ${SRCDIR}/libv8-1.a ${SRCDIR}/libv8-2.a
// #cgo LDFLAGS: -ldl -lm -lstdc++
import "C"
