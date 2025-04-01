// Package errors provides an enhanced error handling system with stack traces,
// context, and monitoring capabilities. It supports performance optimizations like
// optional stack traces, lazy capture, object pooling, and configurable modes.
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// ErrorOpts configures options for error creation.
type ErrorOpts struct {
	SkipStack  int  // Number of stack frames to skip (0 disables stack capture)
	UpdateLast bool // Whether to update the last error registry
}

// Error represents a rich error object with message, name, context, cause, and stack trace.
type Error struct {
	msg      string                 // Formatted message
	name     string                 // Error identifier
	template string                 // Message template
	context  map[string]interface{} // Additional context
	cause    error                  // Wrapped error
	stack    []uintptr              // Stack trace (nil if not captured)
}

// errorPool manages reusable Error instances for performance.
var errorPool = sync.Pool{
	New: func() interface{} {
		return &Error{}
	},
}

// Performance mode flags
var (
	DisableStack    = false // When true, disables stack traces by default
	DisableRegistry = false // When true, disables counting and tracking
)

// NewOpts creates an error with custom options.
func NewOpts(text string, opts ErrorOpts) *Error {
	err := errorPool.Get().(*Error)
	err.Reset()
	err.msg = text
	if !DisableStack && opts.SkipStack > 0 {
		err.stack = captureStack(opts.SkipStack)
	}
	if !DisableRegistry && opts.UpdateLast {
		updateLastError(err)
	}
	return err
}

// New creates a basic error with default options (stack trace enabled, updates last error).
//
// Example:
//
//	err := errors.New("operation failed")
//	fmt.Println(err) // "operation failed"
func New(text string) *Error {
	return NewOpts(text, ErrorOpts{
		SkipStack:  1, // Skip caller
		UpdateLast: true,
	})
}

// Newf creates a formatted error with default options.
//
// Example:
//
//	err := errors.Newf("failed %s %d", "test", 42)
//	fmt.Println(err) // "failed test 42"
func Newf(format string, args ...interface{}) *Error {
	return NewOpts(fmt.Sprintf(format, args...), ErrorOpts{
		SkipStack:  1,
		UpdateLast: true,
	})
}

// Named creates an error with a specific identifier and default options.
//
// Example:
//
//	err := errors.Named("db_error")
//	fmt.Println(err) // "db_error"
func Named(name string) *Error {
	err := NewOpts("", ErrorOpts{
		SkipStack:  1,
		UpdateLast: true,
	})
	err.name = name
	return err
}

// Error returns the error message as a string, implementing the error interface.
func (e *Error) Error() string {
	if e.msg != "" {
		return e.msg
	}
	if e.template != "" {
		return e.template
	}
	if e.name != "" {
		return e.name
	}
	return "unknown error"
}

// Is reports whether any error in e's chain matches target, using name comparison for *Error types.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == target
	}
	if te, ok := target.(*Error); ok {
		if e.name != "" && e.name == te.name {
			return true
		}
	} else if e.cause != nil {
		return errors.Is(e.cause, target)
	}
	if e.cause != nil {
		return Is(e.cause, target)
	}
	return false
}

// As finds the first error in e's chain that matches target, setting *target to that error value.
func (e *Error) As(target interface{}) bool {
	if e == nil {
		return false
	}
	if targetPtr, ok := target.(**Error); ok {
		if e.name != "" {
			*targetPtr = e
			return true
		}
		if e.cause != nil {
			if ce, ok := e.cause.(*Error); ok {
				*targetPtr = ce
				return true
			}
		}
	}
	if e.cause != nil {
		return As(e.cause, target)
	}
	return false
}

// Unwrap returns the underlying cause of the error, if any.
func (e *Error) Unwrap() error {
	return e.cause
}

// Stack returns a formatted stack trace, capturing it lazily if not already present.
func (e *Error) Stack() []string {
	if e.stack == nil && !DisableStack {
		e.stack = captureStack(1)
	}
	if e.stack == nil {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for {
		frame, more := frames.Next()
		trace = append(trace, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace
}

// With adds a key-value pair to the error's context.
func (e *Error) With(key string, value interface{}) *Error {
	if e.context == nil {
		e.context = make(map[string]interface{})
	}
	e.context[key] = value
	return e
}

// Wrap sets the cause of the error, creating a chain of errors.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// Msg updates the error message with a formatted string.
func (e *Error) Msg(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Context returns the error's context map.
func (e *Error) Context() map[string]interface{} {
	return e.context
}

// Trace captures a stack trace if not already present, overriding DisableStack if called explicitly.
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// WithCode associates an HTTP status code with the error, if it has a name.
func (e *Error) WithCode(code int) *Error {
	if e.name != "" {
		registry.codes.Store(e.name, code) // Use sync.Map Store instead of assignment
	}
	return e
}

// Count returns the number of occurrences of this error type, based on its name.
func (e *Error) Count() uint64 {
	if e.name == "" || DisableRegistry {
		return 0
	}
	if counter, ok := registry.counts.Load(e.name); ok {
		return atomic.LoadUint64(counter.(*uint64))
	}
	return 0
}

// Code returns the HTTP status code associated with the error, defaulting to 500 if unnamed.
func (e *Error) Code() int {
	if e.name == "" {
		return 500
	}
	if code, ok := registry.codes.Load(e.name); ok {
		return code.(int)
	}
	return 500
}

// MarshalJSON serializes the error to JSON, including name, message, context, cause, and stack.
func (e *Error) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Name    string                 `json:"name,omitempty"`
		Message string                 `json:"message,omitempty"`
		Context map[string]interface{} `json:"context,omitempty"`
		Cause   interface{}            `json:"cause,omitempty"`
		Stack   []string               `json:"stack,omitempty"`
	}
	je := jsonError{
		Name:    e.name,
		Message: e.msg,
		Context: e.context,
		Stack:   e.Stack(),
	}
	if e.cause != nil {
		if ce, ok := e.cause.(*Error); ok {
			je.Cause = ce
		} else {
			je.Cause = e.cause.Error()
		}
	}
	return json.Marshal(je)
}

// Reset clears all fields of the error, preparing it for reuse.
func (e *Error) Reset() {
	e.msg = ""
	e.name = ""
	e.template = ""
	e.context = nil
	e.cause = nil
	e.stack = nil
}

// Free returns the error to the pool after resetting it.
func (e *Error) Free() {
	e.Reset()
	errorPool.Put(e)
}

// Is provides compatibility with errors.Is, delegating to custom logic or stdlib as needed.
func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return errors.Is(err, target)
}

// As provides compatibility with errors.As, delegating to custom logic or stdlib as needed.
func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
	return errors.As(err, target)
}
