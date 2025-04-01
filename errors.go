package errors

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync/atomic"
)

// Error represents a rich error object
type Error struct {
	msg      string                 // Formatted message
	name     string                 // Error identifier
	template string                 // Message template
	context  map[string]interface{} // Additional context
	cause    error                  // Wrapped error
	stack    []uintptr              // Stack trace
}

// New creates a basic error
func New(text string) *Error {
	err := &Error{
		msg:   text,
		stack: captureStack(1),
	}
	updateLastError(err)
	return err
}

// Newf creates a formatted error
func Newf(format string, args ...interface{}) *Error {
	err := &Error{
		msg:   fmt.Sprintf(format, args...),
		stack: captureStack(1),
	}
	updateLastError(err)
	return err
}

// Named creates an identifiable error
func Named(name string) *Error {
	err := &Error{
		name:  name,
		stack: captureStack(1),
	}
	updateLastError(err)
	return err
}

// Error implements error interface
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

// Is implements errors.Is with custom name comparison
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == target
	}
	if te, ok := target.(*Error); ok {
		return e.name == te.name && e.name != ""
	}
	if e.cause != nil {
		return Is(e.cause, target)
	}
	return false
}

// As implements errors.As
func (e *Error) As(target interface{}) bool {
	if e.cause != nil {
		return As(e.cause, target)
	}
	return false
}

// Unwrap implements error unwrapping
func (e *Error) Unwrap() error {
	return e.cause
}

// Stack returns formatted stack trace
func (e *Error) Stack() []string {
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

// With adds context to error
func (e *Error) With(key string, value interface{}) *Error {
	if e.context == nil {
		e.context = make(map[string]interface{})
	}
	e.context[key] = value
	return e
}

// Wrap adds causal error
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// Msg sets error message
func (e *Error) Msg(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Context returns the error's context map
func (e *Error) Context() map[string]interface{} {
	return e.context
}

// Trace captures stack trace if not present
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// WithCode associates HTTP code
func (e *Error) WithCode(code int) *Error {
	if e.name != "" {
		registry.mu.Lock()
		registry.codes[e.name] = code
		registry.mu.Unlock()
	}
	return e
}

// Count returns the occurrence count of this error type
func (e *Error) Count() uint64 {
	if e.name == "" {
		return 0
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if counter, ok := registry.counts[e.name]; ok {
		return atomic.LoadUint64(counter)
	}
	return 0
}

// Code returns the error code
func (e *Error) Code() int {
	if e.name == "" {
		return 500 // Default
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if code, ok := registry.codes[e.name]; ok {
		return code
	}
	return 500
}

// MarshalJSON implements json.Marshaler
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

// Is provides errors.Is compatibility with custom logic
func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return false
}

// As provides errors.As compatibility
func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
	return false
}
