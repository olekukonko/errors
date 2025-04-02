package errors

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MultiError represents a collection of errors with enhanced features
type MultiError struct {
	errors []error
	mu     sync.RWMutex

	// Configuration fields
	limit      int            // Max errors to store (0=unlimited)
	formatter  ErrorFormatter // Custom formatting
	sampling   bool           // Whether sampling is enabled
	sampleRate uint32         // Sampling percentage (1-100)
}

// ErrorFormatter defines custom error formatting
type ErrorFormatter func([]error) string

// NewMultiError creates a new MultiError with options
func NewMultiError(opts ...MultiErrorOption) *MultiError {
	m := &MultiError{
		errors: make([]error, 0, 4),
		limit:  0, // Unlimited by default
	}

	for _, opt := range opts {
		opt(m)
	}
	return m
}

// MultiErrorOption configures MultiError behavior
type MultiErrorOption func(*MultiError)

// WithLimit sets maximum number of errors to store
func WithLimit(n int) MultiErrorOption {
	return func(m *MultiError) {
		m.limit = n
	}
}

// WithFormatter sets custom error formatting
func WithFormatter(f ErrorFormatter) MultiErrorOption {
	return func(m *MultiError) {
		m.formatter = f
	}
}

// WithSampling enables error sampling (rate 1-100)
func WithSampling(rate uint32) MultiErrorOption {
	return func(m *MultiError) {
		if rate > 100 {
			rate = 100
		}
		m.sampling = true
		m.sampleRate = rate
	}
}

// Error returns formatted error string
func (m *MultiError) Error() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch len(m.errors) {
	case 0:
		return ""
	case 1:
		return m.errors[0].Error()
	default:
		if m.formatter != nil {
			return m.formatter(m.errors)
		}
		return defaultFormat(m.errors)
	}
}

// Add appends an error with optional sampling
func (m *MultiError) Add(err error) {
	if err == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply sampling if enabled
	if m.sampling && len(m.errors) > 0 {
		if fastRand()%100 >= m.sampleRate {
			return
		}
	}

	// Apply limit if set
	if m.limit > 0 && len(m.errors) >= m.limit {
		return
	}

	m.errors = append(m.errors, err)
}

// Errors returns a copy of contained errors
func (m *MultiError) Errors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errs := make([]error, len(m.errors))
	copy(errs, m.errors)
	return errs
}

// Unwrap returns contained errors
func (m *MultiError) Unwrap() []error {
	return m.Errors()
}

// Has reports if any errors exist
func (m *MultiError) Has() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors) > 0
}

// Count returns number of errors
func (m *MultiError) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors)
}

// First returns the first error if any
func (m *MultiError) First() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[0]
	}
	return nil
}

// Last returns the most recent error
func (m *MultiError) Last() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[len(m.errors)-1]
	}
	return nil
}

// Single returns nil if empty, single error if one exists, or self if multiple
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

// Filter returns new MultiError with filtered errors
func (m *MultiError) Filter(fn func(error) bool) *MultiError {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := NewMultiError()
	for _, err := range m.errors {
		if fn(err) {
			filtered.Add(err)
		}
	}
	return filtered
}

// fastRand is a quick pseudo-random number generator
func fastRand() uint32 {
	// Simple xorshift implementation
	r := uint32(time.Now().UnixNano())
	r ^= r << 13
	r ^= r >> 17
	r ^= r << 5
	return r
}

func defaultFormat(errs []error) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(" errors(%d): ", len(errs)))
	for i, err := range errs {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}
