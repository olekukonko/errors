package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// As wraps errors.As, using custom type assertion for *Error types.
// Falls back to standard errors.As for non-*Error types.
func As(err error, target interface{}) bool {
	if err == nil || target == nil {
		return false
	}

	// First try our custom *Error handling
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}

	// Fall back to standard errors.As
	return errors.As(err, target)
}

// Code returns the status code of an error, if it is an *Error.
// Returns 500 as a default for non-*Error types to indicate an internal error.
func Code(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code()
	}
	return 500
}

// Context extracts the context map from an error, if it is an *Error.
// Returns nil for non-*Error types or if no context is present.
func Context(err error) map[string]interface{} {
	if e, ok := err.(*Error); ok {
		return e.Context()
	}
	return nil
}

// Convert transforms any error into an *Error, preserving its message and wrapping it if needed.
// Returns nil if the input is nil; returns the original if already an *Error.
func Convert(err error) *Error {
	if err == nil {
		return nil
	}

	// First try direct type assertion (fast path)
	if e, ok := err.(*Error); ok {
		return e
	}

	// Try using errors.As (more flexible)
	var e *Error
	if errors.As(err, &e) {
		return e
	}

	// Manual unwrapping as fallback
	for unwrapped := err; unwrapped != nil; {
		if e, ok := unwrapped.(*Error); ok {
			return e
		}
		unwrapped = errors.Unwrap(unwrapped)
	}

	// Final fallback: create new error
	return New(err.Error()).Wrap(err)
}

// Count returns the occurrence count of an error, if it is an *Error.
// Returns 0 for non-*Error types.
func Count(err error) uint64 {
	if e, ok := err.(*Error); ok {
		return e.Count()
	}
	return 0
}

// Find searches the error chain for the first error matching pred.
// Returns nil if no match is found; traverses both Unwrap() and Cause() chains.
func Find(err error, pred func(error) bool) error {
	for current := err; current != nil; {
		if pred(current) {
			return current
		}

		// Attempt to unwrap using Unwrap() or Cause()
		switch v := current.(type) {
		case interface{ Unwrap() error }:
			current = v.Unwrap()
		case interface{ Cause() error }:
			current = v.Cause()
		default:
			return nil
		}
	}
	return nil
}

// From transforms any error into an *Error, preserving its message and wrapping it if needed.
// Returns nil if the input is nil; returns the original if already an *Error.
// alias of Convert
func From(err error) *Error {
	return Convert(err)
}

// FromContext creates an error from a context and an existing error.
// Adds context information including:
// - Timeout status and deadline (if applicable)
// - Cancellation status
// - Context values (optional)
// Returns nil if input error is nil.
// FromContext creates an error from a context and an existing error.
// Adds timeout info if applicable; returns nil if input error is nil.
func FromContext(ctx context.Context, err error) *Error {
	if err == nil {
		return nil
	}

	e := New(err.Error())

	// Handle context errors
	switch ctx.Err() {
	case context.DeadlineExceeded:
		e.WithTimeout()
		if deadline, ok := ctx.Deadline(); ok {
			e.With("deadline", deadline.Format(time.RFC3339))
		}
	case context.Canceled:
		e.With("cancelled", true)
	}

	return e
}

// Category returns the category of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func Category(err error) string {
	if e, ok := err.(*Error); ok {
		return e.category
	}
	return ""
}

// Has checks if an error contains meaningful content.
// Returns true for non-nil standard errors or *Error with content.
func Has(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Has()
	}
	return err != nil
}

// HasContextKey checks if the error's context contains the specified key.
// Returns false for non-*Error types or if the key is not present.
func HasContextKey(err error, key string) bool {
	if e, ok := err.(*Error); ok {
		ctx := e.Context()
		if ctx != nil {
			_, exists := ctx[key]
			return exists
		}
	}
	return false
}

// Is wraps errors.Is, using custom matching for *Error types.
// Falls back to standard errors.Is for non-*Error types.
func Is(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}

	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}

	// Use standard errors.Is for non-Error types
	return errors.Is(err, target)
}

// IsError checks if an error is an instance of *Error.
// Returns true only for this package's custom error type.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// IsEmpty checks if an error has no meaningful content
func IsEmpty(err error) bool {
	if err == nil {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.IsEmpty()
	}
	return strings.TrimSpace(err.Error()) == ""
}

// IsNull checks if an error is nil or represents a NULL value
func IsNull(err error) bool {
	if err == nil {
		fmt.Println("Package IsNull: nil error, returning true")
		return true
	}
	if e, ok := err.(*Error); ok {
		result := e.IsNull()
		fmt.Printf("Package IsNull: *Error result=%v\n", result)
		return result
	}
	result := sqlNull(err)
	fmt.Printf("Package IsNull: non-*Error, result=%v (err=%v)\n", result, err)
	return result
}

// IsRetryable checks if an error is retryable.
// For *Error, checks the context; otherwise, infers from timeout or "retry" in the message.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxRetry].(bool); ok {
			return val
		}
	}
	lowerMsg := strings.ToLower(err.Error())
	return IsTimeout(err) || strings.Contains(lowerMsg, "retry")
}

// IsTimeout checks if an error indicates a timeout.
// For *Error, checks the context; otherwise, inspects the error string for "timeout".
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxTimeout].(bool); ok {
			return val
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// Merge combines multiple errors into a single *Error.
// Aggregates messages, contexts, and stacks; returns nil if no errors provided.
func Merge(errs ...error) *Error {
	if len(errs) == 0 {
		return nil
	}
	var messages []string
	combined := New("")
	for _, err := range errs {
		if err == nil {
			continue
		}
		messages = append(messages, err.Error())
		if e, ok := err.(*Error); ok {
			if e.stack != nil && combined.stack == nil {
				combined.WithStack() // Capture stack from first *Error with stack
			}
			if ctx := e.Context(); ctx != nil {
				for k, v := range ctx {
					combined.With(k, v)
				}
			}
			if e.cause != nil {
				combined.Wrap(e.cause)
			}
		} else {
			combined.Wrap(err)
		}
	}
	if len(messages) > 0 {
		combined.msg = strings.Join(messages, "; ")
	}
	return combined
}

// Name returns the name of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

// UnwrapAll returns a slice of all errors in the chain, including the root error.
// Traverses both Unwrap() and Cause() chains; returns nil if err is nil.
func UnwrapAll(err error) []error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e.UnwrapAll()
	}
	var result []error
	Walk(err, func(e error) {
		result = append(result, e)
	})
	return result
}

// Stack extracts the stack trace from an error, if it is an *Error.
// Returns nil for non-*Error types or if no stack is present.
func Stack(err error) []string {
	if e, ok := err.(*Error); ok {
		return e.Stack()
	}
	return nil
}

// Transform applies transformations to an error if it's an *Error.
// Returns a new transformed error or the original if no changes are needed.
func Transform(err error, fn func(*Error)) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		newErr := e.Copy()
		fn(newErr)
		return newErr
	}
	// If not an *Error, create a new one and transform it
	newErr := New(err.Error())
	fn(newErr)
	return newErr
}

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
func Unwrap(err error) error {
	for current := err; current != nil; {
		if e, ok := current.(*Error); ok {
			if e.cause == nil {
				return current
			}
			current = e.cause
		} else {
			return current
		}
	}
	return nil
}

// Walk traverses the error chain, applying fn to each error.
// Works with both *Error and standard error chains via Unwrap() or Cause().
func Walk(err error, fn func(error)) {
	for current := err; current != nil; {
		fn(current)

		// Attempt to unwrap using Unwrap() or Cause()
		switch v := current.(type) {
		case interface{ Unwrap() error }:
			current = v.Unwrap()
		case interface{ Cause() error }:
			current = v.Cause()
		default:
			return
		}
	}
}

// With adds a key-value pair to an error's context, if it is an *Error.
// Returns the original error unchanged if not an *Error.
func With(err error, key string, value interface{}) error {
	if e, ok := err.(*Error); ok {
		return e.With(key, value)
	}
	return err
}

// WithStack converts any error to *Error and captures stack trace
// If input is nil, returns nil. If already *Error, adds stack trace.
func WithStack(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e.WithStack()
	}
	return New(err.Error()).WithStack().Wrap(err)
}

// Wrap creates a new error that wraps another error with additional context.
// It accepts either a *Error, an error, or a string as its wrapper.
func Wrap(err error, wrapper *Error) *Error {
	if err == nil {
		return nil
	}
	if wrapper == nil {
		wrapper = newError()
	}
	newErr := wrapper.Copy()
	newErr.cause = err
	fmt.Printf("Wrap: created newErr %p, msg=%q, name=%q, code=%d, cause=%p\n", newErr, newErr.msg, newErr.name, newErr.code, newErr.cause)
	return newErr
}

// Wrapf creates a new formatted error that wraps another error.
// It ensures the cause is properly chained in the error hierarchy.
func Wrapf(err error, format string, args ...interface{}) *Error {
	if err == nil {
		return nil
	}
	e := newError()
	e.msg = fmt.Sprintf(format, args...)
	e.cause = err
	return e
}
