package errors

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MultiError represents a thread-safe collection of errors with enhanced features.
type MultiError struct {
	errors []error
	mu     sync.RWMutex

	// Configuration fields
	limit      int            // Maximum number of errors to store (0 = unlimited)
	formatter  ErrorFormatter // Custom formatting function for error string
	sampling   bool           // Whether sampling is enabled to limit error collection
	sampleRate uint32         // Sampling percentage (1-100) when sampling is enabled
}

// ErrorFormatter defines a function for custom error message formatting.
type ErrorFormatter func([]error) string

// MultiErrorOption configures MultiError behavior during creation.
type MultiErrorOption func(*MultiError)

// Add appends an error to the collection with optional sampling and limit checks.
// Ignores nil errors; thread-safe.
func (m *MultiError) Add(err error) {
	if err == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply sampling if enabled and not the first error
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

// Clear removes all errors from the collection.
// Thread-safe; resets the slice while preserving capacity.
func (m *MultiError) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = m.errors[:0]
}

// Count returns the number of errors in the collection.
// Thread-safe.
func (m *MultiError) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors)
}

// Error returns a formatted string representation of the errors.
// Returns an empty string if no errors, the single error's message if one exists,
// or a formatted list using the custom formatter or default format if multiple.
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

// Errors returns a copy of the contained Historic.
// Thread-safe; returns nil if no errors exist.
func (m *MultiError) Errors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.errors) == 0 {
		return nil
	}
	errs := make([]error, len(m.errors))
	copy(errs, m.errors)
	return errs
}

// Filter returns a new MultiError containing only errors that match the predicate.
// Thread-safe; preserves original configuration, including sampling behavior.
func (m *MultiError) Filter(fn func(error) bool) *MultiError {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var opts []MultiErrorOption
	opts = append(opts, WithLimit(m.limit))
	if m.formatter != nil {
		opts = append(opts, WithFormatter(m.formatter))
	}
	if m.sampling {
		opts = append(opts, WithSampling(m.sampleRate))
	}

	filtered := NewMultiError(opts...)
	for _, err := range m.errors {
		if fn(err) {
			filtered.Add(err)
		}
	}
	return filtered
}

// First returns the first error in the collection, if any.
// Thread-safe; returns nil if the collection is empty.
func (m *MultiError) First() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[0]
	}
	return nil
}

// Has reports whether the collection contains any errors.
// Thread-safe.
func (m *MultiError) Has() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors) > 0
}

// Last returns the most recently added error in the collection, if any.
// Thread-safe; returns nil if the collection is empty.
func (m *MultiError) Last() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[len(m.errors)-1]
	}
	return nil
}

// Merge combines another MultiError's errors into this one.
// Thread-safe; respects limit and sampling settings.
func (m *MultiError) Merge(other *MultiError) {
	if other == nil || !other.Has() {
		return
	}

	other.mu.RLock()
	defer other.mu.RUnlock()

	for _, err := range other.errors {
		m.Add(err)
	}
}

// NewMultiError creates a new MultiError instance with optional configuration.
// Initial capacity is set to 4; applies options in the order provided.
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

// Single returns nil if the collection is empty, the single error if only one exists,
// or the MultiError itself if multiple errors are present.
// Thread-safe.
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

// String implements the Stringer interface for a concise string representation.
// Thread-safe; uses the same logic as Error().
func (m *MultiError) String() string {
	return m.Error()
}

// Unwrap returns a copy of the contained errors for multi-error unwrapping.
// Implements the errors.Unwrap interface for compatibility; thread-safe.
func (m *MultiError) Unwrap() []error {
	return m.Errors()
}

// WithFormatter sets a custom error formatting function.
// Returns a MultiErrorOption for use with NewMultiError.
func WithFormatter(f ErrorFormatter) MultiErrorOption {
	return func(m *MultiError) {
		m.formatter = f
	}
}

// WithLimit sets the maximum number of errors to store.
// Returns a MultiErrorOption for use with NewMultiError; 0 means unlimited.
func WithLimit(n int) MultiErrorOption {
	return func(m *MultiError) {
		if n < 0 {
			n = 0 // Ensure non-negative limit
		}
		m.limit = n
	}
}

// WithSampling enables error sampling with a specified rate (1-100).
// Returns a MultiErrorOption for use with NewMultiError; caps rate at 100.
func WithSampling(rate uint32) MultiErrorOption {
	return func(m *MultiError) {
		if rate > 100 {
			rate = 100
		}
		m.sampling = true
		m.sampleRate = rate
	}
}

// defaultFormat provides the default formatting for multiple errors.
// Returns a semicolon-separated list prefixed with the error count.
func defaultFormat(errs []error) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("errors(%d): ", len(errs)))
	for i, err := range errs {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// fastRand generates a quick pseudo-random number for sampling.
// Uses a simple xorshift algorithm based on the current time.
func fastRand() uint32 {
	r := uint32(time.Now().UnixNano())
	r ^= r << 13
	r ^= r >> 17
	r ^= r << 5
	return r
}
