package errors

import (
	"strings"
	"sync"
)

// MultiError aggregates multiple errors into a single error.
// MultiError aggregates multiple errors into a single error.
type MultiError struct {
	errors []error
	mu     sync.RWMutex
}

// NewMultiError creates a new MultiError instance.
func NewMultiError() *MultiError {
	return &MultiError{
		errors: make([]error, 0),
	}
}

// Error returns a concatenated string of all errors in the MultiError.
func (m *MultiError) Error() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.errors) == 0 {
		return ""
	}
	if len(m.errors) == 1 {
		return m.errors[0].Error()
	}

	var sb strings.Builder
	sb.WriteString("multiple errors: ")
	for i, err := range m.errors {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// Add appends an error to the MultiError.
func (m *MultiError) Add(err error) {
	if err == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, err)
}

// Errors returns the slice of contained errors.
func (m *MultiError) Errors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	errs := make([]error, len(m.errors))
	copy(errs, m.errors)
	return errs
}

// Unwrap returns the contained errors to support errors.Is and errors.As.
func (m *MultiError) Unwrap() []error {
	return m.Errors()
}

// HasError reports whether the MultiError contains any errors.
func (m *MultiError) Has() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors) > 0
}

// Single returns nil if no errors are present, the single error if there's
// exactly one, or the MultiError itself if there are multiple.
func (m *MultiError) Single() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch len(m.errors) {
	case 0:
		return nil
	case 1:
		return m.errors[0]
	default:
		return m
	}
}
