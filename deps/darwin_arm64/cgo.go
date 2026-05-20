package darwin_arm64

// #cgo LDFLAGS: -pthread
// #cgo LDFLAGS: ${SRCDIR}/libv8-0.a ${SRCDIR}/libv8-1.a
// #cgo LDFLAGS: -lc++ -framework CoreFoundation
import "C"
