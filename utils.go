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

// stackPool manages reusable stack slices for performance.
var stackPool = sync.Pool{
	New: func() interface{} {
		configMu.RLock()
		defer configMu.RUnlock()
		return make([]uintptr, 0, config.StackDepth)
	},
}

// WarmStackPool pre-populates the stack pool with a specified number of slices.
// Useful for reducing allocation overhead at startup.
func WarmStackPool(count int) {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisablePooling {
		return
	}
	for i := 0; i < count; i++ {
		stackPool.Put(make([]uintptr, 0, config.StackDepth))
	}
}

// captureStack captures the call stack with configurable depth and filtering.
// The skip parameter indicates how many frames to skip (e.g., 1 skips the caller of this function).
func captureStack(skip int) []uintptr {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableStack {
		return nil
	}
	stack := stackPool.Get().([]uintptr)
	var pcs [128]uintptr // Larger buffer to avoid truncation, trimmed later
	n := runtime.Callers(skip+2, pcs[:])
	if n > config.StackDepth {
		n = config.StackDepth
	}
	stack = append(stack[:0], pcs[:n]...)
	return stack
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
	if config.DisableStack {
		return "", 0, "stack tracing disabled"
	}
	var pcs [1]uintptr
	n := runtime.Callers(skip+2, pcs[:])
	if n == 0 {
		return "", 0, "unknown"
	}
	frame, _ := runtime.CallersFrames(pcs[:n]).Next()
	return frame.File, frame.Line, frame.Function
}
