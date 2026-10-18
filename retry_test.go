package errors

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano()) // Ensure jitter randomness
}

// TestExecuteReply_Success tests successful execution after retries with a string result.
func TestExecuteReply_Success(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(3),
		WithDelay(50*time.Millisecond),
		WithBackoff(LinearBackoff{}),
		WithJitter(false),
	)
	calls := 0

	start := time.Now()
	result, err := ExecuteReply[string](retry, func() (string, error) {
		calls++
		if calls < 2 {
			return "", New("temporary error").WithRetryable()
		}
		return "success", nil
	})
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %q", result)
	}
	if calls != 2 {
		t.Errorf("Expected 2 calls, got %d", calls)
	}
	if duration < 45*time.Millisecond { // Slightly less than 50ms for execution overhead
		t.Errorf("Expected at least 50ms delay, got %v", duration)
	}
}

func TestExecuteReply_Failure(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(2),
		WithDelay(10*time.Millisecond),
	)
	calls := 0

	result, err := ExecuteReply[int](retry, func() (int, error) {
		calls++
		return 0, New("persistent error").WithRetryable()
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if result != 0 {
		t.Errorf("Expected zero value (0), got %d", result)
	}
	if calls != 2 {
		t.Errorf("Expected 2 calls, got %d", calls)
	}
}

func TestExecuteReply_NonRetryable(t *testing.T) {
	retry := NewRetry(WithMaxAttempts(3))
	calls := 0

	result, err := ExecuteReply[float64](retry, func() (float64, error) {
		calls++
		return 0.0, New("fatal error") // Not retryable
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if result != 0.0 {
		t.Errorf("Expected zero value (0.0), got %f", result)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestExecuteReply_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	retry := NewRetry(
		WithMaxAttempts(5),
		WithContext(ctx),
		WithDelay(50*time.Millisecond),
	)
	calls := 0

	go func() {
		time.Sleep(125 * time.Millisecond) // Allow 2 calls (100ms total) before cancel
		cancel()
	}()

	result, err := ExecuteReply[string](retry, func() (string, error) {
		calls++
		time.Sleep(25 * time.Millisecond) // Simulate work
		return "", New("retryable error").WithRetryable()
	})

	if !Is(err, context.Canceled) {
		t.Errorf("Expected context canceled error, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected zero value (\"\"), got %q", result)
	}
	if calls < 2 {
		t.Errorf("Expected at least 2 calls before cancellation, got %d", calls)
	}
}

func TestExecuteReply_DifferentTypes(t *testing.T) {
	type Result struct {
		Value int
	}
	retry := NewRetry(WithMaxAttempts(3))
	calls := 0

	result, err := ExecuteReply[Result](retry, func() (Result, error) {
		calls++
		if calls < 2 {
			return Result{}, New("temporary error").WithRetryable()
		}
		return Result{Value: 42}, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result.Value != 42 {
		t.Errorf("Expected Value 42, got %d", result.Value)
	}
	if calls != 2 {
		t.Errorf("Expected 2 calls, got %d", calls)
	}
}

func TestRetryJitterWithTinyDelays(t *testing.T) {
	for _, test := range []struct {
		name  string
		delay time.Duration
		cap   time.Duration
	}{
		{name: "zero", delay: 0},
		{name: "one nanosecond", delay: time.Nanosecond},
		{name: "negative delay clamped to zero", delay: -time.Second},
		{name: "capped at one nanosecond", delay: time.Second, cap: time.Nanosecond},
	} {
		for _, withContext := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/context=%t", test.name, withContext), func(t *testing.T) {
				attempts := 0
				r := NewRetry(WithDelay(test.delay), WithMaxDelay(test.cap), WithMaxAttempts(2), WithRetryIf(func(error) bool { return true }))
				fn := func() error {
					attempts++
					if attempts == 1 {
						return fmt.Errorf("temporary failure")
					}
					return nil
				}
				var err error
				if withContext {
					err = r.ExecuteContext(context.Background(), fn)
				} else {
					err = r.Execute(fn)
				}
				if err != nil {
					t.Fatalf("retry returned %v", err)
				}
				if attempts != 2 {
					t.Fatalf("got %d attempts, want 2", attempts)
				}
			})
		}
	}
}
