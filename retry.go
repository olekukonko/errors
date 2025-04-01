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
func (e ExponentialBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 1 {
		return baseDelay
	}
	return baseDelay * time.Duration(1<<uint(attempt-1))
}

// LinearBackoff backoff strategy
type LinearBackoff struct{}

func (l LinearBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	return baseDelay * time.Duration(attempt)
}

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
// It defaults to exponential backoff with jitter if no backoff strategy is specified.
func NewRetry(options ...RetryOption) *Retry {
	r := &Retry{
		maxAttempts: 3,
		delay:       100 * time.Millisecond,
		maxDelay:    10 * time.Second,
		retryIf:     func(err error) bool { return IsRetryable(err) }, // Default from errors.go
		backoff:     ExponentialBackoff{},                             // Default strategy
		jitter:      true,
		ctx:         context.Background(),
	}
	for _, opt := range options {
		opt(r)
	}
	return r
}

// RetryOption configures a Retry instance.
type RetryOption func(*Retry)

// WithMaxAttempts sets the maximum number of retry attempts.
func WithMaxAttempts(maxAttempts int) RetryOption {
	return func(r *Retry) { r.maxAttempts = maxAttempts }
}

// WithDelay sets the initial delay between retries.
func WithDelay(delay time.Duration) RetryOption {
	return func(r *Retry) { r.delay = delay }
}

// WithMaxDelay sets the maximum delay between retries.
func WithMaxDelay(maxDelay time.Duration) RetryOption {
	return func(r *Retry) { r.maxDelay = maxDelay }
}

// WithRetryIf sets the condition under which to retry.
func WithRetryIf(retryIf func(error) bool) RetryOption {
	return func(r *Retry) { r.retryIf = retryIf }
}

// WithOnRetry sets a callback to execute on each retry attempt.
func WithOnRetry(onRetry func(attempt int, err error)) RetryOption {
	return func(r *Retry) { r.onRetry = onRetry }
}

// WithBackoff sets the backoff strategy using the BackoffStrategy interface.
func WithBackoff(strategy BackoffStrategy) RetryOption {
	return func(r *Retry) { r.backoff = strategy }
}

// WithJitter enables or disables jitter in the backoff delay.
func WithJitter(jitter bool) RetryOption {
	return func(r *Retry) { r.jitter = jitter }
}

// WithContext sets the context for cancellation and deadlines.
func WithContext(ctx context.Context) RetryOption {
	return func(r *Retry) { r.ctx = ctx }
}

// Execute runs the provided function with the configured retry logic.
// It returns nil if the function succeeds, or the последний error if all attempts fail.
func (r *Retry) Execute(fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		if r.retryIf != nil && !r.retryIf(err) {
			return err
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
			return r.ctx.Err()
		case <-time.After(currentDelay):
		}
	}
	return lastErr
}

// addJitter adds ±25% jitter to avoid thundering herd problems.
func addJitter(d time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(d/2))) - (d / 4)
	return d + jitter
}
