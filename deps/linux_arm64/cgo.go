package linux_arm64

// #cgo LDFLAGS: -pthread -L${SRCDIR}
// #cgo LDFLAGS: -lv8-0 -lv8-1 -lv8-2
// #cgo linux LDFLAGS: -ldl
import "C"
