// Package errors provides an enhanced error handling system with stack traces,
// context, monitoring, and retry capabilities. It includes performance optimizations
// like optional stack traces, object pooling, and small context caching.
package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	ctxTimeout  = "[error] timeout"  // Context key for timeout flag
	ctxRetry    = "[error] retry"    // Context key for retryable flag
	ctxCategory = "[error] category" // Context key for error category
)

// ErrorOpts configures options for error creation.
type ErrorOpts struct {
	SkipStack  int  // Number of stack frames to skip (0 disables stack capture unless forced)
	UpdateLast bool // Whether to update the last error registry
}

// Config holds global configuration for the errors package.
type Config struct {
	StackDepth      int  // Maximum stack frames to capture
	ContextSize     int  // Size of small context cache
	DisableStack    bool // Disables stack traces by default
	DisableRegistry bool // Disables counting and tracking
	DisablePooling  bool // Disables object pooling
	FilterInternal  bool // Filters out internal package frames from stack traces
}

var (
	configMu sync.RWMutex
	config   = Config{
		StackDepth:      32,
		ContextSize:     2,
		DisableStack:    false,
		DisableRegistry: false,
		DisablePooling:  false,
		FilterInternal:  true, // Default to filtering internal frames
	}
	poolHits   uint64
	poolMisses uint64
)

// Configure sets global configuration options for the errors package.
// It must be called before using the package for settings to take effect.
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

// contextItem represents a key-value pair for small context caching.
type contextItem struct {
	key   string
	value interface{}
}

// Error represents a rich error object with message, name, context, cause, and stack trace.
type Error struct {
	msg          string                 // Formatted message
	name         string                 // Error identifier
	template     string                 // Message template
	context      map[string]interface{} // Additional context (used if >ContextSize items)
	cause        error                  // Wrapped error
	stack        []uintptr              // Stack trace (nil if not captured)
	smallContext []contextItem          // Cache for small context items
	smallCount   int                    // Number of items in smallContext
	callback     func()                 // Callback triggered when error is used
}

// errorPool manages reusable Error instances for performance.
var errorPool = sync.Pool{
	New: func() interface{} {
		configMu.RLock()
		defer configMu.RUnlock()
		return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
	},
}

// getPooledError retrieves an error from the pool or creates a new one.
func getPooledError() *Error {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisablePooling {
		return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
	}
	if err := errorPool.Get(); err != nil {
		atomic.AddUint64(&poolHits, 1)
		e := err.(*Error)
		e.Reset()
		return e
	}
	atomic.AddUint64(&poolMisses, 1)
	return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
}

// WarmPool pre-populates the error pool with a specified number of instances.
// Useful for reducing allocation overhead at startup.
func WarmPool(count int) {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisablePooling {
		return
	}
	for i := 0; i < count; i++ {
		errorPool.Put(&Error{smallContext: make([]contextItem, 0, config.ContextSize)})
	}
}

// New creates a basic error with the given message. Stack traces and registry updates
// are included unless disabled via Configure.
func New(text string) *Error {
	err := getPooledError()
	err.msg = text
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	if !config.DisableRegistry {
		updateLastError(err)
	}
	return err
}

// Make creates a function that returns a new *Error instance with the given message.
// This ensures fresh error instances while keeping the API clean and configurable.
func Make(msg string) func() *Error {
	return func() *Error {
		return New(msg)
	}
}

// Newf creates a formatted error with the given format string and arguments.
func Newf(format string, args ...interface{}) *Error {
	err := getPooledError()
	err.msg = fmt.Sprintf(format, args...)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	if !config.DisableRegistry {
		updateLastError(err)
	}
	return err
}

// Named creates an error with a specific identifier but no initial message.
func Named(name string) *Error {
	err := getPooledError()
	err.name = name
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	if !config.DisableRegistry {
		updateLastError(err)
	}
	return err
}

// Fast creates a lightweight error with no stack trace or registry updates.
// Ideal for performance-critical paths where minimal overhead is desired.
func Fast(text string) *Error {
	err := getPooledError()
	err.msg = text
	return err
}

// Wrapf creates a new error with a formatted message and wraps an existing error.
func Wrapf(err error, format string, args ...interface{}) *Error {
	return Newf(format, args...).Wrap(err)
}

// FromContext creates an error from a context and an existing error, marking it as a timeout
// if the context deadline was exceeded.
func FromContext(ctx context.Context, err error) *Error {
	if err == nil {
		return nil
	}
	e := New(err.Error())
	if ctx.Err() == context.DeadlineExceeded {
		e.WithTimeout()
	}
	return e
}

// Error returns the error message, triggering the callback if set.
func (e *Error) Error() string {
	if e.callback != nil {
		e.callback()
	}
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

// HasError reports whether this error represents a meaningful error condition.
func (e *Error) HasError() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
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

// As finds the first error in e's chain that matches target, setting *target to that value.
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

// Stack returns a formatted stack trace, capturing it lazily if not present.
func (e *Error) Stack() []string {
	configMu.RLock()
	defer configMu.RUnlock()
	if e.stack == nil && !config.DisableStack {
		e.stack = captureStack(1)
	}
	if e.stack == nil {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for i := 0; i < config.StackDepth; i++ {
		frame, more := frames.Next()
		if config.FilterInternal && strings.Contains(frame.Function, "github.com/olekukonko/errors") {
			if !more {
				break
			}
			continue
		}
		trace = append(trace, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace
}

// With adds a key-value pair to the error's context, using a small cache up to ContextSize.
func (e *Error) With(key string, value interface{}) *Error {
	configMu.RLock()
	defer configMu.RUnlock()
	if e.smallCount < cap(e.smallContext) {
		e.smallContext = append(e.smallContext[:e.smallCount], contextItem{key, value})
		e.smallCount++
		return e
	}
	if e.context == nil {
		e.context = make(map[string]interface{}, config.ContextSize+2)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	e.context[key] = value
	return e
}

// Wrap sets the cause of the error, creating a chain of errors.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// WrapNotNil sets the cause only if non-nil, avoiding unnecessary assignments.
func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

// Msgf updates the error message with a formatted string.
func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Context returns the error's context map, merging smallContext if needed.
func (e *Error) Context() map[string]interface{} {
	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

// Trace captures a stack trace if not already present, overriding DisableStack.
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// Copy creates a new *Error instance that duplicates the current error’s state.
// It copies the message, name, template, context, and cause, but resets stack and callback.
// The new instance can be modified independently without affecting the original.
func (e *Error) Copy() *Error {
	newErr := getPooledError()
	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	// Copy context
	if e.smallCount > 0 || e.context != nil {
		for k, v := range e.Context() {
			newErr.With(k, v)
		}
	}
	// Stack and callback are not copied, left fresh for the new instance
	return newErr
}

// WithCode associates an HTTP status code with the error, if it has a name.
func (e *Error) WithCode(code int) *Error {
	if e.name != "" {
		codes.mu.Lock()
		codes.m[e.name] = code
		codes.mu.Unlock()
	}
	return e
}

// WithTimeout marks the error as a timeout error.
func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

// WithRetryable marks the error as retryable.
func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

// WithCategory adds a category to the error.
func (e *Error) WithCategory(category ErrorCategory) *Error {
	return e.With(ctxCategory, string(category))
}

// WithLocale adds locale-specific messaging.
func (e *Error) WithLocale(locale string) *Error {
	if e.name == "" {
		return e
	}
	if localizedMsg, ok := locales[locale+"."+e.name]; ok {
		e.msg = localizedMsg
	}
	return e
}

// Callback sets a function to be called when the error is used.
func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

// Count returns the number of occurrences of this error type, based on its name.
func (e *Error) Count() uint64 {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableRegistry {
		return 0
	}
	return registry.counts.Value(e.name)
}

// Code returns the HTTP status code associated with the error, defaulting to 500 if unnamed.
func (e *Error) Code() int {
	if e.name == "" {
		return 500
	}
	codes.mu.RLock()
	defer codes.mu.RUnlock()
	if code, ok := codes.m[e.name]; ok {
		return code
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
		Context: e.Context(),
		Stack:   e.Stack(),
	}
	if e.cause != nil {
		switch c := e.cause.(type) {
		case *Error:
			je.Cause = c
		case json.Marshaler:
			je.Cause = c
		default:
			je.Cause = c.Error()
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
	if e.stack != nil {
		stackPool.Put(e.stack)
	}
	e.stack = nil
	e.smallCount = 0
	e.smallContext = e.smallContext[:0]
	e.callback = nil
}

// Free returns the error to the pool after resetting it.
func (e *Error) Free() {
	configMu.RLock()
	defer configMu.RUnlock()
	e.Reset()
	if !config.DisablePooling {
		errorPool.Put(e)
	}
}

// IsError reports whether the error is of type *Error.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// Stack returns the stack trace if err is enhanced, nil otherwise.
func Stack(err error) []string {
	if e, ok := err.(*Error); ok {
		return e.Stack()
	}
	return nil
}

// Context returns the context map if err is enhanced, nil otherwise.
func Context(err error) map[string]interface{} {
	if e, ok := err.(*Error); ok {
		return e.Context()
	}
	return nil
}

// Code returns the HTTP status code if err is enhanced, defaulting to 500.
func Code(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code()
	}
	return 500
}

// Name returns the error name if err is enhanced, empty string otherwise.
func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

// With adds context to an error if it's enhanced, returns unchanged otherwise.
func With(err error, key string, value interface{}) error {
	if e, ok := err.(*Error); ok {
		return e.With(key, value)
	}
	return err
}

// Wrap wraps an error if the wrapper is enhanced.
func Wrap(wrapper, cause error) error {
	if e, ok := wrapper.(*Error); ok {
		return e.Wrap(cause)
	}
	return wrapper
}

// IsTimeout checks if the error is a timeout.
func IsTimeout(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxTimeout].(bool); ok {
			return val
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// IsRetryable checks if the error is retryable based on context or common patterns.
func IsRetryable(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxRetry].(bool); ok {
			return val
		}
	}
	return IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "retry")
}

// GetCategory returns the error category if set.
func GetCategory(err error) string {
	if e, ok := err.(*Error); ok {
		if cat, ok := e.Context()[ctxCategory].(string); ok {
			return cat
		}
	}
	return ""
}

// Is provides compatibility with errors.Is.
func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return errors.Is(err, target)
}

// As provides compatibility with errors.As.
func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
	return errors.As(err, target)
}

//
//// RetryOptions configures retry behavior.
//type RetryOptions struct {
//	MaxAttempts  int
//	InitialDelay time.Duration
//	MaxDelay     time.Duration
//	RetryIf      func(error) bool
//	OnRetry      func(attempt int, err error)
//	Exponential  bool
//	Jitter       bool
//	Context      context.Context // Optional context for cancellation
//}
//
//// DefaultRetryOptions provides sensible defaults for retry behavior.
//var DefaultRetryOptions = RetryOptions{
//	MaxAttempts:  3,
//	InitialDelay: 100 * time.Millisecond,
//	MaxDelay:     10 * time.Second,
//	Exponential:  true,
//	Jitter:       true,
//	RetryIf: func(err error) bool {
//		return IsRetryable(err) || Name(err) == "ErrDBTimeout" || Name(err) == "ErrNetworkTimeout"
//	},
//}
