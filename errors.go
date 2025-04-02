package errors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	ctxTimeout = "[error] timeout" // Context key for marking timeout errors
	ctxRetry   = "[error] retry"   // Context key for marking retryable errors

	contextSize = 4   // Default size of fixed-size context array
	bufferSize  = 256 // Initial buffer size for JSON marshaling
	warmUpSize  = 100 // Number of errors to pre-warm the pool
	stackDepth  = 32  // Default maximum stack trace depth
)

type ErrorCategory string

// ErrorOpts provides options for customizing error creation.
type ErrorOpts struct {
	SkipStack int // Number of stack frames to skip when capturing the stack trace
}

// Config defines the configuration for the errors package.
type Config struct {
	StackDepth     int  // Maximum depth of the stack trace; 0 uses default
	ContextSize    int  // Initial size of the context map; 0 uses default
	DisablePooling bool // Disables object pooling for errors if true
	FilterInternal bool // Filters internal package frames from stack traces if true
	AutoFree       bool // Automatically frees errors to pool if true
}

// cachedConfig holds the current configuration, updated only on Configure().
type cachedConfig struct {
	stackDepth     int
	contextSize    int
	disablePooling bool
	filterInternal bool
	autoFree       bool
}

var (
	currentConfig cachedConfig
	configMu      sync.RWMutex
	errorPool     = NewErrorPool() // Custom pool for Error instances
	stackPool     = sync.Pool{     // Pool for stack trace slices
		New: func() interface{} {
			return make([]uintptr, currentConfig.stackDepth)
		},
	}
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, bufferSize))
	},
}

type contextItem struct {
	key   string
	value interface{}
}

// Error represents a custom error with enhanced features like context, stack traces, and wrapping.
type Error struct {
	// Primary error information (most frequently accessed)
	msg   string    // Error message
	name  string    // Error name/type
	stack []uintptr // Stack trace

	// Secondary error metadata
	template   string // Message template used if msg is empty
	category   string // Error category (e.g., "network", "validation")
	count      uint64 // Occurrence count for tracking frequency
	code       int32  // HTTP-like error code
	smallCount int32  // Number of items in smallContext

	// Context and chaining
	context      map[string]interface{}   // Additional context as key-value pairs
	cause        error                    // Wrapped underlying error
	callback     func()                   // Optional callback executed on Error()
	smallContext [contextSize]contextItem // Fixed-size context storage for efficiency

	// Synchronization
	mu sync.RWMutex // Protects concurrent access to mutable fields
}

func init() {
	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: false,
		filterInternal: true,
		autoFree:       true,
	}
	WarmPool(warmUpSize) // Pre-warm pool with initial errors
}

// Configure updates the global configuration for the errors package.
// Thread-safe; should be called before heavy usage for optimal performance.
// Changes apply immediately to all subsequent error operations.
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()

	if cfg.StackDepth != 0 {
		currentConfig.stackDepth = cfg.StackDepth
	}
	if cfg.ContextSize != 0 {
		currentConfig.contextSize = cfg.ContextSize
	}
	currentConfig.disablePooling = cfg.DisablePooling
	currentConfig.filterInternal = cfg.FilterInternal
	currentConfig.autoFree = cfg.AutoFree
}

// newError creates a new Error instance, using the pool if enabled.
// Initializes smallContext and stack appropriately.
func newError() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
	}
	return errorPool.Get()
}

// Empty creates a new empty error with no stack trace.
// Useful as a base for building errors incrementally.
func Empty() *Error {
	return newError()
}

// Named creates a new error with a specific name and stack trace.
// The name is used as the error message if no other message is set.
func Named(name string) *Error {
	e := newError()
	e.name = name
	return e.WithStack()
}

// New creates a fast, lightweight error without stack tracing.
// Use instead of Trace() when stack traces aren't needed for better performance.
func New(text string) *Error {
	err := newError()
	err.msg = text
	return err
}

// Newf is an alias to Errorf for fmt.Errorf compatibility.
// Creates a formatted error without stack traces.
func Newf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// Trace creates an error with stack trace capture enabled.
// Use when call stacks are needed for debugging; has performance overhead.
func Trace(text string) *Error {
	e := New(text)
	return e.WithStack()
}

// Tracef creates a formatted error with stack trace.
// Combines Errorf and WithStack for convenience.
func Tracef(format string, args ...interface{}) *Error {
	e := Errorf(format, args...)
	return e.WithStack()
}

// As attempts to assign the error or its cause to the target interface.
// Implements the errors.As interface for type assertion compatibility.
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

// Callback sets a function to be called when Error() is invoked.
// Useful for logging or side effects; returns the error for chaining.
func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

// Category sets a category for the error and returns the error.
// Useful for classifying errors (e.g., "network", "validation").
func (e *Error) Category() string {
	return e.category
}

// Code returns the error's status code, if set.
// Returns 0 if no code is defined.
func (e *Error) Code() int {
	return int(e.code)
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

// Copy creates a deep copy of the error, preserving all fields except stack.
// The new error does not capture a new stack trace unless explicitly added.
func (e *Error) Copy() *Error {
	newErr := newError()

	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count

	if e.smallCount > 0 {
		newErr.smallCount = e.smallCount
		for i := int32(0); i < e.smallCount; i++ {
			newErr.smallContext[i] = e.smallContext[i]
		}
	} else if e.context != nil {
		newErr.context = make(map[string]interface{}, len(e.context))
		for k, v := range e.context {
			newErr.context[k] = v
		}
	}

	if e.stack != nil && len(e.stack) > 0 {
		if newErr.stack == nil {
			newErr.stack = stackPool.Get().([]uintptr)
		}
		newErr.stack = append(newErr.stack[:0], e.stack...)
	}

	return newErr
}

// Count returns the number of times the error has been incremented.
// Useful for tracking occurrence frequency.
func (e *Error) Count() uint64 {
	return e.count
}

// Err returns the error as an error interface.
// Provided for compatibility; simply returns the error itself.
func (e *Error) Err() error {
	return e
}

// Error returns the string representation of the error.
// Prioritizes msg, then template, then name, falling back to "unknown error".
// Executes callback if set before returning the message.
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

// Errorf creates a formatted error without stack traces.
// Compatible with fmt.Errorf; does not capture stack trace for performance.
func Errorf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// FastStack returns a lightweight stack trace without function names.
// Filters internal frames if FilterInternal is enabled; returns nil if no stack.
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
			continue
		}
		frames = append(frames, fmt.Sprintf("%s:%d", file, line))
	}
	return frames
}

// Find searches the error chain for the first error matching pred.
// Starts with the current error and follows Unwrap() and Cause() chains.
func (e *Error) Find(pred func(error) bool) error {
	if e == nil || pred == nil {
		return nil
	}
	return Find(e, pred)
}

// Format returns a formatted string representation of the error.
func (e *Error) Format() string {
	if e == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString(e.Error())
	if ctx := e.Context(); ctx != nil {
		sb.WriteString("\ncontext: ")
		data, _ := json.MarshalIndent(ctx, "", "  ")
		sb.Write(data)
	}
	if stack := e.Stack(); len(stack) > 0 {
		sb.WriteString("\nstack:\n")
		for _, frame := range stack {
			sb.WriteString(frame)
			sb.WriteRune('\n')
		}
	}
	if e.cause != nil {
		sb.WriteString("\ncause: ")
		if ce, ok := e.cause.(*Error); ok {
			sb.WriteString(ce.Format())
		} else {
			sb.WriteString(e.cause.Error())
		}
	}
	return sb.String()
}

// Free resets the error and returns it to the pool if pooling is enabled.
// Does nothing beyond reset if pooling is disabled.
func (e *Error) Free() {
	if currentConfig.disablePooling {
		return
	}

	e.Reset()

	if e.stack != nil {
		stackPool.Put(e.stack[:cap(e.stack)])
		e.stack = nil
	}

	errorPool.Put(e)
}

// Has checks if the error contains meaningful content.
// Returns true if msg, template, name, or cause is non-empty/nil.
func (e *Error) Has() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

// HasContextKey checks if the specified key exists in the error's context.
func (e *Error) HasContextKey(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 {
		for i := int32(0); i < e.smallCount; i++ {
			if e.smallContext[i].key == key {
				return true
			}
		}
	}
	if e.context != nil {
		_, exists := e.context[key]
		return exists
	}
	return false
}

// Increment increases the error's count by 1 and returns the error.
// Uses atomic operation for thread safety.
func (e *Error) Increment() *Error {
	atomic.AddUint64(&e.count, 1)
	return e
}

// Is checks if the error matches a target error by name or wrapped cause.
// Implements the errors.Is interface for standard library compatibility.
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

// MarshalJSON serializes the error to JSON, including name, message, context, cause, and stack.
// Handles nested *Error causes and custom marshalers efficiently.
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
		for i := int32(0); i < e.smallCount && i < contextSize; i++ {
			ctx[e.smallContext[i].key] = e.smallContext[i].value
		}
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

// Merge combines this error with another, aggregating context and stack.
// Returns a new *Error; original error remains unchanged.
func (e *Error) Merge(other error) *Error {
	if other == nil {
		return e.Copy()
	}
	result := e.Copy()
	if o, ok := other.(*Error); ok {
		result.msg = e.Error() + "; " + o.Error()
		if o.stack != nil && result.stack == nil {
			result.WithStack()
		}
		if ctx := o.Context(); ctx != nil {
			for k, v := range ctx {
				result.With(k, v)
			}
		}
		if o.cause != nil {
			result.Wrap(o.cause)
		}
	} else {
		result.msg = e.Error() + "; " + other.Error()
		result.Wrap(other)
	}
	return result
}

// Msgf sets the error message using a formatted string.
// Overwrites any existing message; returns the error for chaining.
func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Name returns the error's name, if set.
// Returns an empty string if no name is defined.
func (e *Error) Name() string {
	return e.name
}

// Null checks if an error is nil or empty across various error types.
// Considers basic fields, context, and wrapped causes.
func (e *Error) Null() bool {
	if e == nil {
		return true
	}

	if e.Has() {
		return false
	}

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

// Reset clears all fields of the error, preparing it for reuse.
// Frees the stack to the pool if present but keeps it allocated.
func (e *Error) Reset() {
	e.msg = ""
	e.name = ""
	e.template = ""
	e.category = ""
	e.code = 0
	e.count = 0
	e.cause = nil
	e.callback = nil

	if e.context != nil {
		for k := range e.context {
			delete(e.context, k)
		}
	}
	e.smallCount = 0

	if e.stack != nil {
		e.stack = e.stack[:0]
	}
}

// Stack returns a detailed stack trace as a slice of strings.
// Filters internal frames if FilterInternal is enabled; returns nil if no stack.
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
			continue
		}
		trace = append(trace, fmt.Sprintf("%s\n\t%s:%d",
			frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace
}

// Trace ensures the error has a stack trace, capturing it if missing.
// Does nothing if stack tracing is disabled; returns the error for chaining.
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// Transform applies transformations to the error if it's a *Error.
// Returns a new transformed error or the original if no changes needed.
func (e *Error) Transform(fn func(*Error)) *Error {
	if e == nil || fn == nil {
		return e
	}
	newErr := e.Copy()
	fn(newErr)
	return newErr
}

// Unwrap returns the underlying cause of the error, if any.
// Implements the errors.Unwrap interface for unwrapping chains.
func (e *Error) Unwrap() error {
	return e.cause
}

// UnwrapAll returns a slice of all errors in the chain, starting with this error.
// Traverses both Unwrap() and Cause() chains.
func (e *Error) UnwrapAll() []error {
	if e == nil {
		return nil
	}
	var result []error
	e.Walk(func(err error) {
		result = append(result, err)
	})
	return result
}

// Walk traverses the error chain, applying fn to each error.
// Starts with the current error and follows Unwrap() and Cause() chains.
func (e *Error) Walk(fn func(error)) {
	if e == nil || fn == nil {
		return
	}
	Walk(e, fn)
}

// With adds a key-value pair to the error's context.
// Uses smallContext for efficiency until full, then switches to map; returns the error.
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

// WithCategory sets a category for the error and returns the error.
// Useful for classifying errors (e.g., "network", "validation").
func (e *Error) WithCategory(category ErrorCategory) *Error {
	e.category = string(category)
	return e
}

// WithCode sets an HTTP-like status code for the error and returns the error.
// Overwrites any existing code.
func (e *Error) WithCode(code int) *Error {
	e.code = int32(code)
	return e
}

// WithName sets the error's name and returns the error.
// Overwrites any existing name.
func (e *Error) WithName(name string) *Error {
	e.name = name
	return e
}

// WithRetryable marks the error as retryable in its context.
// Adds a "retry" key with value true; returns the error.
func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

// WithStack captures the stack trace at call time and returns the error.
// Skips capturing if stack already exists or depth is 0.
func (e *Error) WithStack() *Error {
	if e.stack == nil {
		if currentConfig.stackDepth > 0 {
			e.stack = stackPool.Get().([]uintptr)
			e.stack = e.stack[:0]
			n := runtime.Callers(2, e.stack[:cap(e.stack)])
			e.stack = e.stack[:n]
		}
	}
	return e
}

// WithTemplate sets a template string for the error and returns the error.
// Used as the error message if no explicit message is set.
func (e *Error) WithTemplate(template string) *Error {
	e.template = template
	return e
}

// WithTimeout marks the error as a timeout error in its context.
// Adds a "timeout" key with value true; returns the error.
func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

// Wrap associates a cause error with this error, creating an error chain.
// Returns the error for method chaining.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// Wrapf creates a new error with a formatted message and wraps an existing error.
// Combines Newf and Wrap for convenience.
func Wrapf(err error, format string, args ...interface{}) *Error {
	return Newf(format, args...).Wrap(err)
}

// WrapNotNil wraps a cause error only if it is non-nil.
// Returns the error for method chaining.
func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

// WarmPool pre-populates the error pool with a specified number of instances.
// Reduces allocation overhead during initial usage; no effect if pooling is disabled.
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
