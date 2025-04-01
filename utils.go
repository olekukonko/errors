// Package errors provides utility functions for error handling, including stack
// trace capture and function name extraction.
package errors

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// Stack pool and capture functions for managing stack traces.
var stackPool = sync.Pool{
	New: func() interface{} {
		return make([]uintptr, 0, currentConfig.stackDepth)
	},
}

// WarmStackPool pre-populates the stack pool with a specified number of slices.
// Reduces allocation overhead for stack traces; no effect if pooling is disabled.
func WarmStackPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}

// captureStack captures a stack trace with the configured depth.
// skip=0 means capture current call site
func captureStack(skip int) []uintptr {
	// Get buffer with enough capacity
	buf := stackPool.Get().([]uintptr)
	buf = buf[:cap(buf)]

	// +1 skips runtime.Callers itself
	n := runtime.Callers(skip+2, buf)
	if n == 0 {
		stackPool.Put(buf)
		return nil
	}

	// Return exact-sized slice
	stack := make([]uintptr, n)
	copy(stack, buf[:n])
	stackPool.Put(buf)

	return stack
}

// min returns the smaller of two integers.
// Helper function for limiting stack trace size.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getFuncName extracts the function name from an interface, typically a function or method.
// Returns "unknown" if the input is nil or invalid.
func getFuncName(fn interface{}) string {
	if fn == nil {
		return "unknown"
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return strings.TrimPrefix(fullName, ".")
}

// FormatError returns a formatted string representation of an error, including its message,
// stack trace, and context if it’s an enhanced *Error.
// Useful for logging or debugging.
func FormatError(err error) string {
	if err == nil {
		return "<nil>"
	}
	var sb strings.Builder
	if e, ok := err.(*Error); ok {
		sb.WriteString(fmt.Sprintf("Error: %s\n", e.Error()))
		if e.name != "" {
			sb.WriteString(fmt.Sprintf("Name: %s\n", e.name))
		}
		if ctx := e.Context(); len(ctx) > 0 {
			sb.WriteString("Context:\n")
			for k, v := range ctx {
				sb.WriteString(fmt.Sprintf("\t%s: %v\n", k, v))
			}
		}
		if stack := e.Stack(); len(stack) > 0 {
			sb.WriteString("Stack Trace:\n")
			for _, frame := range stack {
				sb.WriteString(fmt.Sprintf("\t%s\n", frame))
			}
		}
		if e.cause != nil {
			sb.WriteString(fmt.Sprintf("Caused by: %s\n", FormatError(e.cause)))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
	}
	return sb.String()
}

// Caller returns the file, line, and function name of the caller at the specified skip level.
// Skip 0 returns the caller of this function, 1 returns its caller, etc.
func Caller(skip int) (file string, line int, function string) {
	configMu.RLock()
	defer configMu.RUnlock()
	var pcs [1]uintptr
	n := runtime.Callers(skip+2, pcs[:])
	if n == 0 {
		return "", 0, "unknown"
	}
	frame, _ := runtime.CallersFrames(pcs[:n]).Next()
	return frame.File, frame.Line, frame.Function
}
