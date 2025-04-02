package errors

import (
	"errors"
	"reflect"
	"strings"
)

// As wraps errors.As, using custom type assertion for *Error types.
// Falls back to standard errors.As for non-*Error types.
func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
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

// GetCategory returns the category of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func GetCategory(err error) string {
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

// Is wraps errors.Is, using custom matching for *Error types.
// Falls back to standard errors.Is for non-*Error types.
func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return errors.Is(err, target)
}

// IsError checks if an error is an instance of *Error.
// Returns true only for this package's custom error type.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
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

// Name returns the name of an error, if it is an *Error.
// Returns an empty string for non-*Error types.
func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

// Null checks if an error is completely null/empty across all error types.
// Handles nil errors, *Error types, sql.Null* types, and zero-valued errors.
func Null(err error) bool {
	if err == nil {
		return true
	}

	// Use *Error's Null() method if applicable
	if e, ok := err.(*Error); ok {
		return e.Null()
	}

	// Check for sql.Null types (placeholder logic)
	if sqlNull(err) {
		return true
	}

	// Check for empty error messages
	if err.Error() == "" {
		return true
	}

	// Use reflection to detect nil concrete error types
	val := reflect.ValueOf(err)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return true
	}
	return false
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
func Transform(err error, fn func(*Error)) error {
	if err == nil || fn == nil {
		return err
	}
	if e, ok := err.(*Error); ok {
		return e.Transform(fn)
	}
	return err
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

// Wrap associates a cause with a wrapper error, if the wrapper is an *Error.
// Returns the wrapper unchanged if not an *Error.
func Wrap(wrapper, cause error) error {
	if e, ok := wrapper.(*Error); ok {
		return e.Wrap(cause)
	}
	return wrapper
}
