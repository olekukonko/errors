package errors

import (
	"reflect"
	"runtime"
	"strings"
)

// captureStack captures call stack
func captureStack(skip int) []uintptr {
	var pcs [32]uintptr
	n := runtime.Callers(skip+2, pcs[:])
	return pcs[:n]
}

// Helper functions
func getFuncName(fn interface{}) string {
	if fn == nil {
		return "unknown"
	}
	return strings.TrimPrefix(runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(), ".")
}
