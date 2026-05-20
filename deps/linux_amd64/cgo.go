package linux_amd64

// #cgo LDFLAGS: -pthread -fuse-ld=lld
// #cgo LDFLAGS: ${SRCDIR}/libv8-0.a ${SRCDIR}/libv8-1.a ${SRCDIR}/libv8-2.a
// #cgo LDFLAGS: -ldl -lm -lstdc++
import "C"
