//go:build leakcheck
// +build leakcheck

package v8go_test

import (
	"os"
	"testing"

	"github.com/ChessCom/v8go"
)

func TestMain(m *testing.M) {
	exitCode := m.Run()
	v8go.DoLeakSanitizerCheck()
	os.Exit(exitCode)
}
