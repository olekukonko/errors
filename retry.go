// Package errors provides utilities for error handling, including a flexible retry mechanism.
package errors

import (
	"context"
	"math/rand"
	"time"
)

// BackoffStrategy defines the interface for calculating retry delays.
type BackoffStrategy interface {
	// Backoff returns the delay for a given attempt based on the base delay.
	Backoff(attempt int, baseDelay time.Duration) time.Duration
}

// ConstantBackoff provides a fixed delay for each retry attempt.
type ConstantBackoff struct{}

// Backoff returns the base delay regardless of the attempt number.
func (c ConstantBackoff) Backoff(_ int, baseDelay time.Duration) time.Duration {
	return baseDelay
}

// ExponentialBackoff provides an exponentially increasing delay for retry attempts.
type ExponentialBackoff struct{}

// Backoff returns a delay that doubles with each attempt, starting from the base delay.
// Uses bit shifting for efficient exponential growth.
func (e ExponentialBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 1 {
		return baseDelay
	}
	return baseDelay * time.Duration(1<<uint(attempt-1))
}

// LinearBackoff provides a linearly increasing delay for retry attempts.
type LinearBackoff struct{}

// Backoff returns a delay that increases linearly with each attempt.
func (l LinearBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	return baseDelay * time.Duration(attempt)
}

// RetryOption configures a Retry instance.
type RetryOption func(*Retry)

// Retry represents a retryable operation with configurable backoff and retry logic.
type Retry struct {
	maxAttempts int
	delay       time.Duration    // Base delay for backoff
	maxDelay    time.Duration    // Maximum delay cap
	retryIf     func(error) bool // Condition to determine if retry should occur
	onRetry     func(int, error) // Callback after each failed attempt
	backoff     BackoffStrategy  // Strategy for calculating retry delays
	jitter      bool             // Whether to add jitter to delays
	ctx         context.Context  // Context for cancellation and deadlines
}

// NewRetry creates a new Retry instance with the given options.
// Defaults to 3 attempts, 100ms base delay, 10s max delay, exponential backoff with jitter,
// and retrying on IsRetryable errors from helpers.go.
func NewRetry(options ...RetryOption) *Retry {
	r := &Retry{
		maxAttempts: 3,
		delay:       100 * time.Millisecond,
		maxDelay:    10 * time.Second,
		retryIf:     func(err error) bool { return IsRetryable(err) }, // Ensure this is always set
		onRetry:     nil,
		backoff:     ExponentialBackoff{},
		jitter:      true,
		ctx:         context.Background(),
	}
	for _, opt := range options {
		opt(r)
	}
	// Ensure retryIf is never nil
	if r.retryIf == nil {
		r.retryIf = func(err error) bool { return IsRetryable(err) }
	}
	return r
}

// addJitter adds ±25% jitter to avoid thundering herd problems.
// Returns a duration adjusted by a random value between -25% and +25% of the input.
func addJitter(d time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(d/2))) - (d / 4)
	return d + jitter
}

// Attempts returns the configured maximum number of retry attempts.
func (r *Retry) Attempts() int {
	return r.maxAttempts
}

// Execute runs the provided function with the configured retry logic.
// Returns nil if the function succeeds, or the last error if all attempts fail.
// Respects context cancellation and deadlines.
func (r *Retry) Execute(fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if retry is applicable
		if r.retryIf != nil && !r.retryIf(err) {
			return err
		}

		lastErr = err
		if r.onRetry != nil {
			r.onRetry(attempt, err)
		}

		// Exit if this was the last attempt
		if attempt == r.maxAttempts {
			break
		}

		// Calculate delay with backoff and cap at maxDelay
		currentDelay := r.backoff.Backoff(attempt, r.delay)
		if currentDelay > r.maxDelay {
			currentDelay = r.maxDelay
		}
		if r.jitter {
			currentDelay = addJitter(currentDelay)
		}

		// Respect context cancellation or wait for delay
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-time.After(currentDelay):
		}
	}
	return lastErr
}

// Transform creates a new Retry instance with modified configuration.
// It copies all settings from the original Retry and applies the given options.
func (r *Retry) Transform(opts ...RetryOption) *Retry {
	newRetry := &Retry{
		maxAttempts: r.maxAttempts,
		delay:       r.delay,
		maxDelay:    r.maxDelay,
		retryIf:     r.retryIf,
		onRetry:     r.onRetry,
		backoff:     r.backoff,
		jitter:      r.jitter,
		ctx:         r.ctx,
	}
	for _, opt := range opts {
		opt(newRetry)
	}
	return newRetry
}

// WithBackoff sets the backoff strategy using the BackoffStrategy interface.
// Returns a RetryOption for use with NewRetry.
func WithBackoff(strategy BackoffStrategy) RetryOption {
	return func(r *Retry) {
		if strategy != nil {
			r.backoff = strategy
		}
	}
}

// WithContext sets the context for cancellation and deadlines.
// Returns a RetryOption for use with NewRetry; defaults to context.Background if nil.
func WithContext(ctx context.Context) RetryOption {
	return func(r *Retry) {
		if ctx != nil {
			r.ctx = ctx
		}
	}
}

// WithDelay sets the initial delay between retries.
// Returns a RetryOption for use with NewRetry; ensures non-negative delay.
func WithDelay(delay time.Duration) RetryOption {
	return func(r *Retry) {
		if delay < 0 {
			delay = 0
		}
		r.delay = delay
	}
}

// WithJitter enables or disables jitter in the backoff delay.
// Returns a RetryOption for use with NewRetry.
func WithJitter(jitter bool) RetryOption {
	return func(r *Retry) {
		r.jitter = jitter
	}
}

// WithMaxAttempts sets the maximum number of retry attempts.
// Returns a RetryOption for use with NewRetry; ensures at least 1 attempt.
func WithMaxAttempts(maxAttempts int) RetryOption {
	return func(r *Retry) {
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		r.maxAttempts = maxAttempts
	}
}

// WithMaxDelay sets the maximum delay between retries.
// Returns a RetryOption for use with NewRetry; ensures non-negative delay.
func WithMaxDelay(maxDelay time.Duration) RetryOption {
	return func(r *Retry) {
		if maxDelay < 0 {
			maxDelay = 0
		}
		r.maxDelay = maxDelay
	}
}

// WithOnRetry sets a callback to execute after each failed attempt.
// Returns a RetryOption for use with NewRetry.
func WithOnRetry(onRetry func(attempt int, err error)) RetryOption {
	return func(r *Retry) {
		r.onRetry = onRetry
	}
}

// WithRetryIf sets the condition under which to retry.
// Returns a RetryOption for use with NewRetry; defaults to IsRetryable if nil.
func WithRetryIf(retryIf func(error) bool) RetryOption {
	return func(r *Retry) {
		if retryIf != nil {
			r.retryIf = retryIf
		}
	}
}

// ExecuteReply runs the provided function with retry logic and returns its result.
// Returns the function's result and nil error on success, or the last error on failure.
// Type parameter T allows for generic return values.
func ExecuteReply[T any](r *Retry, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		if r.retryIf != nil && !r.retryIf(err) {
			return zero, err
		}

		lastErr = err
		if r.onRetry != nil {
			r.onRetry(attempt, err)
		}

		if attempt == r.maxAttempts {
			break
		}

		currentDelay := r.backoff.Backoff(attempt, r.delay)
		if currentDelay > r.maxDelay {
			currentDelay = r.maxDelay
		}
		if r.jitter {
			currentDelay = addJitter(currentDelay)
		}

		select {
		case <-r.ctx.Done():
			return zero, r.ctx.Err()
		case <-time.After(currentDelay):
		}
	}
	return zero, lastErr
}
