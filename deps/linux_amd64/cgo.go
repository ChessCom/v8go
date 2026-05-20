package linux_amd64

// #cgo LDFLAGS: -pthread
// #cgo LDFLAGS: -Wl,--start-group ${SRCDIR}/libv8-0.a ${SRCDIR}/libv8-1.a ${SRCDIR}/libv8-2.a -Wl,--end-group
// #cgo LDFLAGS: -ldl -lm -lstdc++
import "C"
