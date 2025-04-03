package main

import (
	"context"
	"fmt"
	"github.com/olekukonko/errors"
	"math/rand"
	"time"
)

// DatabaseClient simulates a flaky database connection
type DatabaseClient struct {
	healthyAfterAttempt int
}

func (db *DatabaseClient) Query() error {
	if db.healthyAfterAttempt > 0 {
		db.healthyAfterAttempt--
		return errors.New("database connection failed").
			With("attempt_remaining", db.healthyAfterAttempt).
			WithRetryable() // Mark as retryable
	}
	return nil
}

// ExternalService simulates an unreliable external API
func ExternalService() error {
	if rand.Intn(100) < 30 { // 30% failure rate
		return errors.New("service unavailable").
			WithCode(503).
			WithRetryable()
	}
	return nil
}

func main() {
	// Configure retry with exponential backoff and jitter
	retry := errors.NewRetry(
		errors.WithMaxAttempts(5),
		errors.WithDelay(200*time.Millisecond),
		errors.WithMaxDelay(2*time.Second),
		errors.WithJitter(true),
		errors.WithBackoff(errors.ExponentialBackoff{}),
		errors.WithOnRetry(func(attempt int, err error) {
			// Calculate delay using the same logic as in Execute
			baseDelay := 200 * time.Millisecond
			maxDelay := 2 * time.Second
			delay := errors.ExponentialBackoff{}.Backoff(attempt, baseDelay)
			if delay > maxDelay {
				delay = maxDelay
			}
			fmt.Printf("Attempt %d failed: %v (retrying in %v)\n",
				attempt,
				err.Error(),
				delay)
		}),
	)

	// Scenario 1: Database connection with known recovery point
	db := &DatabaseClient{healthyAfterAttempt: 3}
	fmt.Println("Starting database operation...")
	err := retry.Execute(func() error {
		return db.Query()
	})
	if err != nil {
		fmt.Printf("Database operation failed after %d attempts: %v\n", retry.Attempts(), err)
	} else {
		fmt.Println("Database operation succeeded!")
	}

	// Scenario 2: External service with random failures
	fmt.Println("\nStarting external service call...")
	var lastAttempts int
	start := time.Now()

	// Using ExecuteReply to demonstrate return values
	result, err := errors.ExecuteReply[string](retry, func() (string, error) {
		lastAttempts++
		if err := ExternalService(); err != nil {
			return "", err
		}
		return "service response data", nil
	})

	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Service call failed after %d attempts (%.2f sec): %v\n",
			lastAttempts,
			duration.Seconds(),
			err)
	} else {
		fmt.Printf("Service call succeeded after %d attempts (%.2f sec): %s\n",
			lastAttempts,
			duration.Seconds(),
			result)
	}

	// Scenario 3: Context cancellation with more visibility
	fmt.Println("\nStarting operation with timeout...")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	timeoutRetry := retry.Transform(
		errors.WithContext(ctx),
		errors.WithMaxAttempts(10),
		errors.WithOnRetry(func(attempt int, err error) {
			fmt.Printf("Timeout scenario attempt %d: %v\n", attempt, err)
		}),
	)

	startTimeout := time.Now()
	err = timeoutRetry.Execute(func() error {
		time.Sleep(300 * time.Millisecond) // Simulate long operation
		return errors.New("operation timed out")
	})

	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Printf("Operation cancelled by timeout after %.2f sec: %v\n",
			time.Since(startTimeout).Seconds(),
			err)
	} else if err != nil {
		fmt.Printf("Operation failed: %v\n", err)
	} else {
		fmt.Println("Operation succeeded (unexpected)")
	}
}
