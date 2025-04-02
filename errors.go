package errors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	ctxTimeout = "[error] timeout"
	ctxRetry   = "[error] retry"

	contextSize = 4
	bufferSize  = 256
	warmUpSize  = 100
	stackDepth  = 32
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
	DisablePooling bool // If true, disables object pooling for errors.
	FilterInternal bool // If true, filters internal package frames from stack traces.
	AutoFree       bool
}

// cachedConfig holds the current configuration, updated only on Configure().
type cachedConfig struct {
	stackDepth     int
	contextSize    int
	disablePooling bool
	filterInternal bool
	autoFree       bool // If true, filters internal package frames from stack traces.
}

var (
	currentConfig cachedConfig
	configMu      sync.RWMutex
	errorPool     = NewErrorPool() // Replace sync.Pool with ErrorPool
	stackPool     = sync.Pool{
		New: func() interface{} {
			return make([]uintptr, currentConfig.stackDepth)
		},
	}
)

func init() {
	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: false,
		filterInternal: true,
		autoFree:       true,
	}

	WarmPool(warmUpSize) // Pre-warm pool with 100 errors on initialization.
}

// Configure updates the global configuration for the errors package.
// It is thread-safe and should be called before heavy usage for optimal performance.
// The changes take effect immediately for all subsequent error operations.
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()

	// Only update fields that are explicitly set in cfg
	if cfg.StackDepth != 0 {
		currentConfig.stackDepth = cfg.StackDepth
	}
	if cfg.ContextSize != 0 {
		currentConfig.contextSize = cfg.ContextSize
	}

	if currentConfig.disablePooling != cfg.DisablePooling {
		currentConfig.disablePooling = cfg.DisablePooling
	}
	if currentConfig.filterInternal != cfg.FilterInternal {
		currentConfig.filterInternal = cfg.FilterInternal
	}
	if currentConfig.autoFree != cfg.AutoFree {
		currentConfig.autoFree = cfg.AutoFree
	}
}

type contextItem struct {
	key   string
	value interface{}
}

// Error represents a custom error with enhanced features like context, stack traces, and wrapping.
// Error is a custom error type with enhanced features like stack tracing, context, and error chaining.
// It is optimized for performance by separating fields into hot, warm, and cold paths based on usage frequency.
type Error struct {
	// Hot Path (performance critical fields that are frequently accessed)
	stack []uintptr // Stack trace (24 bytes), used for debugging and tracing.
	msg   string    // Error message (16 bytes), the primary description of the error.
	name  string    // Name of the error (16 bytes), useful for error classification and identification.

	// Warm Path (fields that are accessed less frequently but still important for error handling)
	template string // Formatted template for the error message (16 bytes).
	category string // Category for classifying the error (16 bytes), helps in categorizing errors.
	count    uint64 // Occurrence count (8 bytes), tracks how many times the error has occurred.
	code     int32  // Numeric error code (4 bytes), e.g., HTTP status code or custom error code.

	// ↓ Padding for memory alignment ↓
	_ [4]byte // Ensures that the next field (context) is 8-byte aligned (critical for performance).

	// Cold Path (less frequently accessed fields, often used for additional context or error chaining)
	context      map[string]interface{}   // Additional context data (8 bytes), typically a pointer to a map.
	cause        error                    // The underlying cause of the error (16 bytes), supports error chaining.
	callback     func()                   // Optional callback function (8 bytes), executed on error retrieval.
	smallContext [contextSize]contextItem // Fixed-size array (64 bytes), holds key/value pairs for smaller context.
	smallCount   int32                    // Number of items stored in `smallContext` (4 bytes).

	// ↓ Padding for mutex alignment ↓
	_ [3]byte // Ensures that the mutex (`sync.RWMutex`) is aligned on an 8-byte boundary.

	mu sync.RWMutex // Mutex (24 bytes), ensures thread-safe access to the error object in concurrent environments.
}

// WarmPool pre-populates the error pool with a specified number of instances.
// This reduces allocation overhead during initial usage. Has no effect if pooling is disabled.
func WarmPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		e := &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
		errorPool.Put(e)
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}

// Add this to errors.go
func newError() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
	}
	return errorPool.Get()
}

// Empty creates a new empty error with an optional stack trace.
// It is useful as a base for building errors incrementally.
func Empty() *Error {
	return newError()
}

// New creates a fast, lightweight error without stack tracing.
// For better performance, use this instead of Trace() when stack traces aren't needed.
func New(text string) *Error {
	err := newError()
	err.msg = text
	return err
}

// Newf is an alias to Errorf for fmt.Errorf compatibility
func Newf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// Errorf creates a formatted error without stack traces
func Errorf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// Trace creates an error with stack trace capture enabled.
// Use when call stacks are needed for debugging, but note this has ~20x performance overhead.
func Trace(text string) *Error {
	e := New(text)
	return e.WithStack()
}

// Tracef creates a formatted error with stack trace
func Tracef(format string, args ...interface{}) *Error {
	e := Errorf(format, args...)
	return e.WithStack()
}

// Named creates a new error with a specific name and an optional stack trace.
// The name is used as the error message if no other message is set.
func Named(name string) *Error {
	e := newError()
	e.name = name
	return e.WithStack()
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
func (e *Error) Has() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

// Null checks if an error is nil/empty across various error types
func (e *Error) Null() bool {
	if e == nil {
		return true
	}

	// Check basic error fields first (fast path)
	if e.Has() {
		return false
	}

	// Check for sql.Null types in context (if any)
	if e.smallCount > 0 {
		for i := 0; i < int(e.smallCount); i++ {
			if sqlNull(e.smallContext[i].value) {
				return false
			}
		}
	}
	if e.context != nil {
		for _, v := range e.context {
			if sqlNull(v) {
				return false
			}
		}
	}

	// Check if this error wraps a sql.Null type
	if e.cause != nil {
		if sqlNull(e.cause) {
			return false
		}
		if _, ok := e.cause.(interface{ Valid() bool }); ok {
			return false
		}
	}

	return true
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
// Filters internal frames if FilterInternal is enabled in the configuration.
func (e *Error) Stack() []string {
	if e.stack == nil {
		return nil
	}
	configMu.RLock()
	filter := currentConfig.filterInternal
	configMu.RUnlock()

	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for {
		frame, more := frames.Next()
		if filter && isInternalFrame(frame) {
			continue // Skip internal frames
		}
		trace = append(trace, fmt.Sprintf("%s\n\t%s:%d",
			frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace
}

// FastStack returns a lightweight stack trace without filtering or function names.
// Filters internal frames if FilterInternal is enabled in the configuration.
func (e *Error) FastStack() []string {
	if e.stack == nil {
		return nil
	}
	configMu.RLock()
	filter := currentConfig.filterInternal
	configMu.RUnlock()

	pcs := e.stack
	frames := make([]string, 0, len(pcs))
	for _, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			frames = append(frames, "unknown")
			continue
		}
		file, line := fn.FileLine(pc)
		if filter && isInternalFrame(runtime.Frame{File: file, Function: fn.Name()}) {
			continue // Skip internal frames
		}
		frames = append(frames, fmt.Sprintf("%s:%d", file, line))
	}
	return frames
}

// Wrap associates a cause error with this error, creating an error chain.
// Returns the error for method chaining.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// With adds a key-value pair to the error's context.
func (e *Error) With(key string, value interface{}) *Error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.smallCount < contextSize {
		e.smallContext[e.smallCount] = contextItem{key, value}
		e.smallCount++
		return e
	}
	if e.smallCount == contextSize && e.context == nil {
		e.context = make(map[string]interface{}, currentConfig.contextSize)
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
		e.smallCount = 3
	}
	if e.context == nil {
		e.context = make(map[string]interface{}, currentConfig.contextSize)
	}
	e.context[key] = value
	return e
}

// WithStack captures the stack trace at call time and returns the error
//
//	func (e *Error) WithStack() *Error {
//		if e.stack == nil {
//			e.stack = captureStack(1)
//		}
//		return e
//	}
func (e *Error) WithStack() *Error {
	if e.stack == nil {
		if currentConfig.stackDepth > 0 {
			e.stack = stackPool.Get().([]uintptr)
			e.stack = e.stack[:0]
			// +1 skips runtime.Callers itself
			n := runtime.Callers(2, e.stack[:cap(e.stack)])
			e.stack = e.stack[:n]
		}
	}
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
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

// Trace ensures the error has a stack trace, capturing it if missing.
// Does nothing if stack tracing is disabled; returns the error for chaining.
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// Copy creates a deep copy of the error, preserving all fields except stack.
// The new error does not capture a new stack trace unless explicitly added.
func (e *Error) Copy() *Error {
	newErr := newError() // Use our improved newError() instead of getPooledError()

	// Copy all fields
	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count

	// Handle context copy efficiently
	if e.smallCount > 0 {
		// Fast path: copy smallContext directly
		newErr.smallCount = e.smallCount
		for i := int32(0); i < e.smallCount; i++ {
			newErr.smallContext[i] = e.smallContext[i]
		}
	} else if e.context != nil {
		// Slow path: copy map if exists
		newErr.context = make(map[string]interface{}, len(e.context))
		for k, v := range e.context {
			newErr.context[k] = v
		}
	}

	// Handle stack trace copy
	if e.stack != nil && len(e.stack) > 0 {
		if newErr.stack == nil {
			newErr.stack = stackPool.Get().([]uintptr)
		}
		newErr.stack = append(newErr.stack[:0], e.stack...)
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
	e.code = int32(code)
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
	return int(e.code)
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, bufferSize))
	},
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

	var ctx map[string]interface{}
	if e.smallCount > 0 || e.context != nil {
		ctx = make(map[string]interface{}, e.smallCount)
		// Copy key/value pairs from smallContext (the fast path)
		for i := int32(0); i < e.smallCount && i < contextSize; i++ {
			ctx[e.smallContext[i].key] = e.smallContext[i].value
		}
		// If a map has been allocated, copy its items as well.
		if e.context != nil {
			for k, v := range e.context {
				ctx[k] = v
			}
		}
	}

	je := jsonError{
		Name:    e.name,
		Message: e.msg,
		Context: ctx,
	}

	if e.stack != nil {
		je.Stack = e.Stack()
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

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(je); err != nil {
		return nil, err
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// Reset clears all fields of the error, preparing it for reuse.
// Frees the stack to the pool if present.
func (e *Error) Reset() {
	// Clear all fields
	e.msg = ""
	e.name = ""
	e.template = ""
	e.category = ""
	e.code = 0
	e.count = 0
	e.cause = nil
	e.callback = nil

	// Clear context
	if e.context != nil {
		for k := range e.context {
			delete(e.context, k)
		}
	}
	e.smallCount = 0

	// Keep stack slice allocated but empty
	if e.stack != nil {
		e.stack = e.stack[:0]
	}
}

// Free resets the error and returns it to the pool, if pooling is enabled.
// Does nothing beyond reset if pooling is disabled.
func (e *Error) Free() {
	if currentConfig.disablePooling {
		return
	}

	e.Reset()

	// Return stack to stackPool if it exists
	if e.stack != nil {
		stackPool.Put(e.stack[:cap(e.stack)]) // Return full capacity slice
		e.stack = nil
	}

	// Return error to ErrorPool
	errorPool.Put(e)
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

func Has(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Has()
	}
	return err != nil
}

// Null checks if an error is completely null/empty across all error types.
// Handles: nil errors, *Error types, sql.Null* types, and zero-valued errors.
func Null(err error) bool {
	if err == nil {
		return true
	}

	// Handle *Error types using their Null() method
	if e, ok := err.(*Error); ok {
		return e.Null()
	}

	// Handle sql.Null types
	if sqlNull(err) {
		return true
	}

	// Handle empty error strings
	if err.Error() == "" {
		return true
	}

	// Handle nil concrete error types via reflection
	val := reflect.ValueOf(err)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return true
	}
	return false
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

// Transform applies transformations to an error if it's a *Error.
// Returns the original error if it's not a *Error or if fn is nil.
func Transform(err error, fn func(*Error)) error {
	if err == nil || fn == nil {
		return err
	}

	if e, ok := err.(*Error); ok {
		// Create a copy to avoid modifying the original
		newErr := e.Copy()
		fn(newErr)
		return newErr
	}

	// For non-*Error types, return as-is
	return err
}
