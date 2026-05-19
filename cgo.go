package v8go

//go:generate clang-format -i --verbose -style=Chromium v8go.h v8go.cc

// #cgo CXXFLAGS: -fno-rtti -fPIC -std=c++20 -I${SRCDIR}/deps/include -Wall
// #cgo CXXFLAGS: -DV8_COMPRESS_POINTERS -DV8_31BIT_SMIS_ON_64BIT_ARCH
// #cgo CXXFLAGS: -DV8_DEPRECATION_WARNINGS -DV8_IMMINENT_DEPRECATION_WARNINGS
import "C"
