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
	ctxTimeout = "[error] timeout"
	ctxRetry   = "[error] retry"
)

type ErrorCategory string

// ErrorOpts provides options for customizing error creation.
type ErrorOpts struct {
	SkipStack int // Number of stack frames to skip when capturing the stack trace.
}

// Config defines the configuration for the errors package.
type Config struct {
	StackDepth     int  // Maximum depth of the stack trace.
	ContextSize    int  // Initial size of the context map (not used with fixed-size array).
	DisableStack   bool // If true, disables stack trace capture.
	DisablePooling bool // If true, disables object pooling for errors.
	FilterInternal bool // If true, filters internal package frames from stack traces.
	AutoFree       bool
}

// cachedConfig holds the current configuration, updated only on Configure().
type cachedConfig struct {
	stackDepth     int
	contextSize    int
	disableStack   bool
	disablePooling bool
	filterInternal bool
	autofree       bool // If true, filters internal package frames from stack traces.
}

var (
	currentConfig cachedConfig
	configMu      sync.RWMutex
	poolHits      uint64 // Tracks pool hits for debugging; can be removed in production.
	poolMisses    uint64 // Tracks pool misses for debugging; can be removed in production.
)

func init() {
	currentConfig = cachedConfig{
		stackDepth:     32,
		contextSize:    2,
		disableStack:   false,
		disablePooling: false,
		filterInternal: true,
		autofree:       true,
	}
	WarmPool(100) // Pre-warm pool with 100 errors on initialization.
}

// Configure updates the global configuration for the errors package.
// It is thread-safe and should be called before heavy usage for optimal performance.
// The changes take effect immediately for all subsequent error operations.
func Configure(cfg Config) {
	configMu.Lock()
	currentConfig = cachedConfig{
		stackDepth:     cfg.StackDepth,
		contextSize:    cfg.ContextSize,
		disableStack:   cfg.DisableStack,
		disablePooling: cfg.DisablePooling,
		filterInternal: cfg.FilterInternal,
		autofree:       cfg.AutoFree,
	}
	configMu.Unlock()
}

type contextItem struct {
	key   string
	value interface{}
}

// Error represents a custom error with enhanced features like context, stack traces, and wrapping.
// Error is a custom error type with enhanced features like stack tracing, context, and error chaining.
// It is optimized for performance by separating fields into hot, warm, and cold paths based on usage frequency.
type Error struct {
	// Hot Path (frequently accessed in Error() and Stack() methods)
	stack []uintptr // Captured stack trace (approx. 24 bytes) used for debugging and tracing.
	msg   string    // The error message (approx. 16 bytes), primary field for error description.
	name  string    // Name of the error (approx. 16 bytes), can be used for error classification.

	// Warm Path (frequently accessed, but not as critical as hot fields)
	template string // Formatted template for the error message (approx. 16 bytes).
	category string // Category string for classifying the error (approx. 16 bytes).
	code     int    // Numeric code (e.g., HTTP status) for the error (approx. 8 bytes).
	count    uint64 // A counter for tracking occurrences (approx. 8 bytes).

	// Cold Path (rarely accessed fields, used for additional context or chaining)
	context      map[string]interface{} // Additional context data (map pointer, approx. 8 bytes).
	cause        error                  // Wrapped underlying error (approx. 16 bytes).
	callback     func()                 // Callback function to execute on error retrieval (approx. 8 bytes).
	smallContext [2]contextItem         // Fixed-size array for up to 2 key/value context pairs (size depends on contextItem).
	smallCount   int                    // Number of items stored in smallContext (approx. 8 bytes).
	pooled       bool                   // Flag indicating whether the error is pooled for reuse (1 byte).
	_            [7]byte                // Padding to align the struct correctly.
}

// WarmPool pre-populates the error pool with a specified number of instances.
// This reduces allocation overhead during initial usage. Has no effect if pooling is disabled.
func WarmPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		errorPool.Put(&Error{})
	}
}

// Empty creates a new empty error with an optional stack trace.
// It is useful as a base for building errors incrementally.
func Empty() *Error {
	err := getPooledError()
	if !currentConfig.disableStack {
		err.stack = captureStack(1)
	}
	return err
}

// New creates a new error with the specified message and an optional stack trace.
// The message is stored directly without formatting.
func New(text string) *Error {
	err := getPooledError()
	err.msg = text
	if !currentConfig.disableStack {
		err.stack = captureStack(1)
	}
	return err
}

// Newf creates a new error with a formatted message and an optional stack trace.
// It uses fmt.Sprintf to format the message with the provided arguments.
func Newf(format string, args ...interface{}) *Error {
	err := getPooledError()
	err.msg = fmt.Sprintf(format, args...)
	if !currentConfig.disableStack {
		err.stack = captureStack(1)
	}
	return err
}

// Named creates a new error with a specific name and an optional stack trace.
// The name is used as the error message if no other message is set.
func Named(name string) *Error {
	err := getPooledError()
	err.name = name
	if !currentConfig.disableStack {
		err.stack = captureStack(1)
	}
	return err
}

// Fast creates a lightweight error with a message, skipping stack trace capture.
// It is optimized for performance in scenarios where stack traces are unnecessary.
func Fast(text string) *Error {
	err := getPooledError()
	err.msg = text
	return err
}

// Wrapf creates a new error with a formatted message and wraps an existing error.
// It combines Newf and Wrap for convenience.
func Wrapf(err error, format string, args ...interface{}) *Error {
	return Newf(format, args...).Wrap(err)
}

// FromContext creates an error from a context and an existing error, adding timeout info if applicable.
// Returns nil if the input error is nil.
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

// Error returns the string representation of the error.
// It prioritizes msg, then template, then name, falling back to "unknown error".
// If a callback is set, it is executed before returning the message.
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

// Name returns the error's name, if set.
// Returns an empty string if no name is defined.
func (e *Error) Name() string {
	return e.name
}

// HasError checks if the error contains meaningful content.
// Returns true if msg, template, name, or cause is non-empty/nil.
func (e *Error) HasError() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

// Is checks if the error matches a target error by name or wrapped cause.
// Implements the errors.Is interface for compatibility with standard library unwrapping.
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

// As attempts to assign the error or its cause to the target interface.
// Implements the errors.As interface for type assertion.
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
// Implements the errors.Unwrap interface for unwrapping chains.
func (e *Error) Unwrap() error {
	return e.cause
}

// Count returns the number of times the error has been incremented.
// Useful for tracking occurrence frequency.
func (e *Error) Count() uint64 {
	return e.count
}

// Increment increases the error's count by 1 and returns the error.
// The count is updated atomically for thread safety.
func (e *Error) Increment() *Error {
	atomic.AddUint64(&e.count, 1)
	return e
}

// Stack returns a detailed stack trace as a slice of strings.
// Captures the stack lazily if not already present and stack tracing is enabled.
func (e *Error) Stack() []string {
	if e.stack == nil && !currentConfig.disableStack {
		e.stack = captureStack(1)
	}
	if e.stack == nil {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for i := 0; i < currentConfig.stackDepth; i++ {
		frame, more := frames.Next()
		if currentConfig.filterInternal && strings.Contains(frame.Function, "github.com/olekukonko/errors") {
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

// FastStack returns a lightweight stack trace without filtering or function names.
// It is faster than Stack() but provides less detail, only file:line pairs.
func (e *Error) FastStack() []string {
	if e.stack == nil {
		return nil
	}
	pcs := e.stack
	frames := make([]string, len(pcs))
	for i, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			frames[i] = "unknown"
			continue
		}
		file, line := fn.FileLine(pc)
		frames[i] = fmt.Sprintf("%s:%d", file, line)
	}
	return frames
}

// With adds a key-value pair to the error's context.
// Stores up to 2 items in a fixed array; additional items use a map.
func (e *Error) With(key string, value interface{}) *Error {
	if e.smallCount < 2 { // Matches fixed size of smallContext
		e.smallContext[e.smallCount] = contextItem{key, value}
		e.smallCount++
		return e
	}
	if e.context == nil {
		e.context = make(map[string]interface{}, currentConfig.contextSize+2)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	e.context[key] = value
	return e
}

// Wrap associates a cause error with this error, creating an error chain.
// Returns the error for method chaining.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// WrapNotNil wraps a cause error only if it is non-nil.
// Returns the error for method chaining.
func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

// Msgf sets the error message using a formatted string.
// Overwrites any existing message; returns the error for chaining.
func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Context returns the error's context as a map.
// Converts smallContext to a map if needed; returns nil if empty.
func (e *Error) Context() map[string]interface{} {
	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

// Trace ensures the error has a stack trace, capturing it if missing.
// Does nothing if stack tracing is disabled; returns the error for chaining.
func (e *Error) Trace() *Error {
	if e.stack == nil && !currentConfig.disableStack {
		e.stack = captureStack(1)
	}
	return e
}

// Copy creates a deep copy of the error, preserving all fields except stack.
// The new error does not capture a new stack trace unless explicitly added.
func (e *Error) Copy() *Error {
	newErr := getPooledError()
	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count
	if e.smallCount > 0 || e.context != nil {
		for k, v := range e.Context() {
			newErr.With(k, v)
		}
	}
	return newErr
}

// WithName sets the error's name and returns the error.
// Overwrites any existing name.
func (e *Error) WithName(name string) *Error {
	e.name = name
	return e
}

// WithTemplate sets a template string for the error and returns the error.
// Used as the error message if no explicit message is set.
func (e *Error) WithTemplate(template string) *Error {
	e.template = template
	return e
}

// WithCode sets an HTTP-like status code for the error and returns the error.
func (e *Error) WithCode(code int) *Error {
	e.code = code
	return e
}

// WithTimeout marks the error as a timeout error in its context.
// Adds a "timeout" key with value true; returns the error.
func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

// WithRetryable marks the error as retryable in its context.
// Adds a "retry" key with value true; returns the error.
func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

// WithCategory sets a category for the error and returns the error.
// Useful for classifying errors (e.g., "network", "validation").
func (e *Error) WithCategory(category ErrorCategory) *Error {
	e.category = string(category)
	return e
}

// Callback sets a function to be called when Error() is invoked.
// Useful for logging or side effects; returns the error.
func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

// Code returns the error's status code, if set.
// Returns 0 if no code is defined.
func (e *Error) Code() int {
	return e.code
}

// MarshalJSON serializes the error to JSON, including name, message, context, cause, and stack.
// Handles nested *Error causes and custom marshalers.
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
// Frees the stack to the pool if present.
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
	e.callback = nil
	e.code = 0
	e.category = ""
	e.count = 0
}

// Free resets the error and returns it to the pool, if pooling is enabled.
// Does nothing beyond reset if pooling is disabled.
func (e *Error) Free() {
	if e.pooled && !currentConfig.disablePooling {
		e.Reset()
		errorPool.Put(e)
		// For Go 1.24+, the cleanup handler will handle it automatically
		// For older versions, explicit Free() is needed
	}
}

// IsError checks if an error is an instance of *Error.
// Returns true only for this package's custom error type.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// Stack extracts the stack trace from an error, if it is an *Error.
// Returns nil for non-*Error types or if no stack is present.
func Stack(err error) []string {
	if e, ok := err.(*Error); ok {
		return e.Stack()
	}
	return nil
}

// Context extracts the context map from an error, if it is an *Error.
// Returns nil for non-*Error types or if no context is present.
func Context(err error) map[string]interface{} {
	if e, ok := err.(*Error); ok {
		return e.Context()
	}
	return nil
}

// Count returns the occurrence count of an error, if it is an *Error.
// Returns 0 for non-*Error types.
func Count(err error) uint64 {
	if e, ok := err.(*Error); ok {
		return e.Count()
	}
	return 0
}

// Code returns the status code of an error, if it is an *Error.
// Returns 500 for non-*Error types as a default.
func Code(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code()
	}
	return 500
}

// Name returns the name of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

// With adds a key-value pair to an error's context, if it is an *Error.
// Returns the original error unchanged if not an *Error.
func With(err error, key string, value interface{}) error {
	if e, ok := err.(*Error); ok {
		return e.With(key, value)
	}
	return err
}

// Wrap associates a cause with a wrapper error, if the wrapper is an *Error.
// Returns the wrapper unchanged if not an *Error.
func Wrap(wrapper, cause error) error {
	if e, ok := wrapper.(*Error); ok {
		return e.Wrap(cause)
	}
	return wrapper
}

// IsTimeout checks if an error indicates a timeout.
// For *Error, checks the context; otherwise, inspects the error string.
func IsTimeout(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxTimeout].(bool); ok {
			return val
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// IsRetryable checks if an error is retryable.
// For *Error, checks the context; otherwise, infers from timeout or "retry" in the message.
func IsRetryable(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxRetry].(bool); ok {
			return val
		}
	}
	return IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "retry")
}

// GetCategory returns the category of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func GetCategory(err error) string {
	if e, ok := err.(*Error); ok {
		return e.category
	}
	return ""
}

// Is wraps errors.Is, using custom matching for *Error types.
// Falls back to standard errors.Is for non-*Error types.
func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return errors.Is(err, target)
}

// As wraps errors.As, using custom type assertion for *Error types.
// Falls back to standard errors.As for non-*Error types.
func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
	return errors.As(err, target)
}
