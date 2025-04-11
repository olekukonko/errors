package errors

import (
	"context"
	stderrs "errors" // Alias for standard errors package to avoid conflicts
	"fmt"
	"log/slog" // Structured logging package for testing log output
	"strings"
	"testing" // Standard Go testing package
	"time"
)

// memoryLogHandler is a custom slog handler that captures log output in memory.
// It’s used to verify logging behavior in tests without writing to external systems.
type memoryLogHandler struct {
	attrs []slog.Attr     // Stores attributes for WithAttrs
	mu    strings.Builder // Accumulates log output as a string
}

// NewMemoryLogHandler creates a new memoryLogHandler.
// It initializes an empty handler for capturing logs.
func NewMemoryLogHandler() *memoryLogHandler {
	return &memoryLogHandler{}
}

// Enabled indicates whether the handler processes logs for a given level.
// Always returns true to capture all logs for testing.
func (h *memoryLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

// Handle processes a log record and formats it into the handler’s buffer.
// It includes the level, message, and all attributes (including groups).
func (h *memoryLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Write the log level and message
	h.mu.WriteString(fmt.Sprintf("level=%s msg=%q", r.Level, r.Message))
	prefix := " "
	// processAttr recursively handles attributes, including nested groups
	var processAttr func(a slog.Attr)
	processAttr = func(a slog.Attr) {
		if a.Value.Kind() == slog.KindGroup {
			// Handle group attributes
			groupAttrs := a.Value.Group()
			if len(groupAttrs) > 0 {
				h.mu.WriteString(fmt.Sprintf("%s%s={", prefix, a.Key))
				groupPrefix := ""
				for _, ga := range groupAttrs {
					h.mu.WriteString(groupPrefix)
					processAttr(ga)
					groupPrefix = " "
				}
				h.mu.WriteString("}")
			}
		} else {
			// Handle simple key-value attributes
			h.mu.WriteString(fmt.Sprintf("%s%s=%v", prefix, a.Key, a.Value.Any()))
		}
		prefix = " "
	}
	// Process handler-level attributes
	for _, a := range h.attrs {
		processAttr(a)
	}
	// Process record-level attributes
	r.Attrs(func(a slog.Attr) bool {
		processAttr(a)
		return true
	})
	// Append a newline to separate log entries
	h.mu.WriteByte('\n')
	return nil
}

// WithAttrs creates a new handler with additional attributes.
// It preserves existing attributes and appends new ones.
func (h *memoryLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := NewMemoryLogHandler()
	// Copy existing attributes to avoid modifying the original
	newHandler.attrs = append(make([]slog.Attr, 0, len(h.attrs)+len(attrs)), h.attrs...)
	newHandler.attrs = append(newHandler.attrs, attrs...)
	return newHandler
}

// WithGroup creates a new handler with a group attribute.
// It adds a group to the attribute list for nested logging.
func (h *memoryLogHandler) WithGroup(name string) slog.Handler {
	newHandler := NewMemoryLogHandler()
	// Copy existing attributes and add a new group
	newHandler.attrs = append(make([]slog.Attr, 0, len(h.attrs)+1), h.attrs...)
	newHandler.attrs = append(newHandler.attrs, slog.Group(name))
	return newHandler
}

// GetOutput returns the accumulated log output as a string.
func (h *memoryLogHandler) GetOutput() string {
	return h.mu.String()
}

// Reset clears the handler’s buffer and attributes.
// It prepares the handler for a new test.
func (h *memoryLogHandler) Reset() {
	h.mu.Reset()
	h.attrs = nil
}

// Define test errors for consistent use across tests.
// These simulate various error scenarios.
var (
	errTest      = stderrs.New("test error")      // Generic test error
	errTemporary = stderrs.New("temporary error") // Error for retry scenarios
	errPermanent = stderrs.New("permanent error") // Non-retryable error
	errOptional  = stderrs.New("optional error")  // Error for optional steps
	errPayment   = stderrs.New("payment failed")  // Error for payment scenarios
	errStep1     = stderrs.New("error1")          // First step error
	errStep2     = stderrs.New("error2")          // Second step error
	errStep3     = stderrs.New("error3")          // Third step error
)

// TestChainExampleFromDocs tests the example usage from documentation.
// It verifies retry behavior and logging for a payment processing function.
func TestChainExampleFromDocs(t *testing.T) {
	// Initialize a memory log handler to capture logs
	logHandler := NewMemoryLogHandler()
	attempts := 0 // Track number of function executions

	// Define a payment processing function that fails twice before succeeding
	processPayment := func() error {
		attempts++
		if attempts < 3 {
			// Return a retryable error for the first two attempts
			return New("payment failed").WithRetryable()
		}
		return nil // Succeed on the third attempt
	}

	// Create a chain with the log handler and a single step with retries
	c := NewChain(ChainWithLogHandler(logHandler)).
		Step(processPayment).
		Retry(3, 5*time.Millisecond) // Allow 3 total attempts with 5ms delay

	// Run the chain
	err := c.Run()

	// Verify no error was returned (should succeed after retries)
	if err != nil {
		t.Errorf("Expected nil error after retries, got %v", err)
	}

	// Check that exactly 3 attempts were made (initial + 2 retries)
	if attempts != 3 {
		t.Errorf("Expected 3 total attempts (initial + 2 retries), got %d", attempts)
	}

	// Get the captured log output
	logOutput := logHandler.GetOutput()

	// Log the output for debugging purposes
	t.Logf("Captured Log Output:\n%s", logOutput)

	// Verify retry log messages
	// Check for the first retry attempt log
	if !strings.Contains(logOutput, "Retrying step (attempt 1/3)") {
		t.Error("Missing first retry log message (expected '...attempt 1/3...')")
	}
	// Check for the second retry attempt log
	if !strings.Contains(logOutput, "Retrying step (attempt 2/3)") {
		t.Error("Missing second retry log message (expected '...attempt 2/3...')")
	}

	// Verify retry attributes in the logs
	if !strings.Contains(logOutput, "attempt=1 max_attempts=3") {
		t.Errorf("Log for first retry missing correct attributes (expected 'attempt=1 max_attempts=3')")
	}
	if !strings.Contains(logOutput, "attempt=2 max_attempts=3") {
		t.Errorf("Log for second retry missing correct attributes (expected 'attempt=2 max_attempts=3')")
	}
}

// TestChainBasicOperations tests basic chain functionality.
// It covers empty chains, successful steps, failing steps, and optional steps.
func TestChainBasicOperations(t *testing.T) {
	// Subtest: EmptyChain
	// Verifies that an empty chain runs without errors and has no steps or errors.
	t.Run("EmptyChain", func(t *testing.T) {
		c := NewChain()
		if err := c.Run(); err != nil {
			t.Errorf("Empty chain should not return error, got %v", err)
		}
		if c.Len() != 0 {
			t.Errorf("Empty chain should have length 0, got %d", c.Len())
		}
		if c.HasErrors() {
			t.Error("Empty chain should not have errors")
		}
	})

	// Subtest: SingleSuccessfulStep
	// Verifies that a single successful step executes and returns no error.
	t.Run("SingleSuccessfulStep", func(t *testing.T) {
		var executed bool
		c := NewChain().Step(func() error { executed = true; return nil })
		if err := c.Run(); err != nil {
			t.Errorf("Single successful step should not return error, got %v", err)
		}
		if !executed {
			t.Error("Successful step was not executed")
		}
	})

	// Subtest: SingleFailingStep
	// Verifies that a single failing step returns an enhanced error and records it.
	t.Run("SingleFailingStep", func(t *testing.T) {
		var executed bool
		c := NewChain().Step(func() error { executed = true; return errTest })
		err := c.Run()

		if !executed {
			t.Error("Failing step was not executed")
		}
		// Check that the error is of the enhanced *Error type
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected error to be *errors.Error, got %T", err)
		}
		// Verify the error wraps the original errTest
		if !stderrs.Is(enhancedErr, errTest) {
			t.Errorf("Expected wrapped error to contain '%v', got '%v'", errTest, enhancedErr)
		}
		// Ensure the chain recorded the error
		if !c.HasErrors() {
			t.Error("Chain should have errors after failure")
		}
	})

	// Subtest: MultipleStepsWithFailure
	// Verifies that execution stops after a non-optional failure and only prior steps run.
	t.Run("MultipleStepsWithFailure", func(t *testing.T) {
		var step1, step3 bool
		c := NewChain().
			Step(func() error { step1 = true; return nil }).
			Step(func() error { return errTest }).
			Step(func() error { step3 = true; return nil })

		err := c.Run()

		if !step1 {
			t.Error("Step 1 should have executed")
		}
		if step3 {
			t.Error("Step 3 should not have executed after failure")
		}
		// Verify the error is enhanced
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected error to be *errors.Error, got %T", err)
		}
		// Verify the error wraps errTest
		if !stderrs.Is(enhancedErr, errTest) {
			t.Errorf("Expected wrapped error '%v', got '%v'", errTest, enhancedErr)
		}
	})

	// Subtest: OptionalStepFailure
	// Verifies that an optional failing step doesn’t stop execution and Run() returns nil.
	t.Run("OptionalStepFailure", func(t *testing.T) {
		var step1, step3 bool
		c := NewChain().
			Step(func() error { step1 = true; return nil }).
			Step(func() error { return errOptional }).Optional().
			Step(func() error { step3 = true; return nil })

		err := c.Run()

		if !step1 {
			t.Error("Step 1 should have executed")
		}
		if !step3 {
			t.Error("Step 3 should have executed after optional failure")
		}
		if err != nil {
			t.Errorf("Run() should return nil when only optional steps fail, got %v", err)
		}
		if !c.HasErrors() {
			t.Error("Chain should have errors even if only optional failed")
		}
	})

	// Subtest: OptionalStepSuccess
	// Verifies that all steps, including optional successful ones, execute correctly.
	t.Run("OptionalStepSuccess", func(t *testing.T) {
		var step1, step2, step3 bool
		c := NewChain().
			Step(func() error { step1 = true; return nil }).
			Step(func() error { step2 = true; return nil }).Optional().
			Step(func() error { step3 = true; return nil })

		if err := c.Run(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !step1 || !step2 || !step3 {
			t.Error("All steps should have executed")
		}
	})
}

// TestChainErrorEnhancement tests error wrapping and metadata enhancement.
// It verifies auto-wrapping, disabling wrapping, and adding metadata.
func TestChainErrorEnhancement(t *testing.T) {
	// Subtest: AutoWrapStandardErrors
	// Verifies that standard errors are automatically wrapped with stack traces.
	t.Run("AutoWrapStandardErrors", func(t *testing.T) {
		stdErr := fmt.Errorf("standard error %d", 123)
		c := NewChain().Step(func() error { return stdErr })
		err := c.Run()

		// Verify the error is enhanced
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected error to be *errors.Error, got %T", err)
		}
		// Check that it wraps the original error
		if !stderrs.Is(enhancedErr, stdErr) {
			t.Errorf("Wrapped error should contain '%v', got '%v'", stdErr, enhancedErr)
		}
		// Ensure a stack trace was added
		if len(enhancedErr.Stack()) == 0 {
			t.Error("Enhanced error should have a stack trace")
		}
	})

	// Subtest: DisableAutoWrap
	// Verifies that disabling auto-wrapping returns the raw error.
	t.Run("DisableAutoWrap", func(t *testing.T) {
		stdErr := fmt.Errorf("standard error %d", 456)
		c := NewChain(ChainWithAutoWrap(false)).Step(func() error { return stdErr })
		err := c.Run()

		// Ensure the error is not wrapped
		if _, ok := err.(*Error); ok {
			t.Fatalf("Error should not be wrapped when ChainWithAutoWrap(false), got *errors.Error")
		}
		// Verify it’s the original error
		if !stderrs.Is(err, stdErr) {
			t.Errorf("Expected raw error '%v', got '%v'", stdErr, err)
		}
	})

	// Subtest: ErrorMetadataViaEnhancement
	// Verifies that metadata (context, category, code, log attributes) is added to errors.
	t.Run("ErrorMetadataViaEnhancement", func(t *testing.T) {
		// Define metadata
		category := ErrorCategory("database")
		code := 503
		key := "query_id"
		value := "xyz789"
		logKey := "trace_id"
		logValue := "trace-abc"

		// Create a chain with a failing step and metadata
		c := NewChain().
			Step(func() error { return errTest }).
			With(key, value).Tag(category).Code(code).
			WithLog(slog.String(logKey, logValue))

		err := c.Run()
		if err == nil {
			t.Fatal("Expected an error")
		}

		// Verify the error is enhanced
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected error to be *errors.Error, got %T", err)
		}

		// Check context metadata
		contextMap := enhancedErr.Context()
		if val, ok := contextMap[key]; !ok || val != value {
			t.Errorf("Expected context['%s'] == %v, got %v", key, value, val)
		}
		if val, ok := contextMap[logKey]; !ok || val != logValue {
			t.Errorf("Expected context['%s'] == %v, got %v", logKey, logValue, val)
		}
		// Check category
		if enhancedErr.Category() != string(category) {
			t.Errorf("Expected category %q, got %q", category, enhancedErr.Category())
		}
		// Check error code
		if enhancedErr.Code() != code {
			t.Errorf("Expected code %d, got %d", code, enhancedErr.Code())
		}
	})
}

// TestChainRetryLogic tests retry behavior for different scenarios.
// It verifies successful retries, failed retries, and context timeout interactions.
func TestChainRetryLogic(t *testing.T) {
	// Define errors for the test
	errTemporary := New("temporary error").WithRetryable() // Retryable error
	errPermanent := stderrs.New("permanent error")         // Non-retryable error

	// Subtest: RetrySuccessful
	// Verifies that a retryable error eventually succeeds after retries.
	t.Run("RetrySuccessful", func(t *testing.T) {
		attempts := 0
		logHandler := NewMemoryLogHandler()
		c := NewChain(ChainWithLogHandler(logHandler)).
			Step(func() error {
				attempts++
				t.Logf("RetrySuccessful: Attempt %d", attempts)
				if attempts < 3 {
					return errTemporary // Fails for first two attempts
				}
				return nil // Succeeds on third attempt
			}).
			Retry(3, 1*time.Millisecond) // Allow 3 attempts

		err := c.Run()
		if err != nil {
			t.Errorf("Expected success after retries, got %v", err)
		}
		// Verify exactly 3 attempts were made
		if attempts != 3 {
			t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempts)
		}
	})

	// Subtest: RetryFailure
	// Verifies that a non-retryable error fails after forced retries.
	t.Run("RetryFailure", func(t *testing.T) {
		attempts := 0
		logHandler := NewMemoryLogHandler()
		c := NewChain(ChainWithLogHandler(logHandler)).
			Step(func() error {
				attempts++
				t.Logf("RetryFailure: Attempt %d", attempts)
				return errPermanent // Always fails
			}).
			// Force retries even for non-retryable errors
			Retry(3, 1*time.Millisecond, WithRetryIf(func(error) bool { return true }))

		err := c.Run()
		if err == nil {
			t.Error("Expected failure after retries")
		}
		// Verify all attempts were made
		if attempts != 3 {
			t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempts)
		}
		// Verify the error is enhanced
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected enhanced *Error, got %T", err)
		}
		// Check that it wraps the original error
		if !stderrs.Is(enhancedErr, errPermanent) {
			t.Errorf("Expected enhanced error wrapping '%v', got '%v'", errPermanent, enhancedErr)
		}
	})

	// Subtest: RetryRespectsContext
	// Verifies that retries respect the chain’s timeout.
	t.Run("RetryRespectsContext", func(t *testing.T) {
		attempts := 0
		logHandler := NewMemoryLogHandler()
		c := NewChain(ChainWithLogHandler(logHandler), ChainWithTimeout(10*time.Millisecond)).
			Step(func() error {
				attempts++
				t.Logf("RetryRespectsContext: Attempt %d starting sleep", attempts)
				// Sleep longer than the timeout to trigger context cancellation
				time.Sleep(25 * time.Millisecond)
				t.Logf("RetryRespectsContext: Attempt %d finished sleep (should not happen)", attempts)
				return errPermanent
			}).
			// Force retries to ensure timeout is the limiting factor
			Retry(2, 5*time.Millisecond, WithRetryIf(func(error) bool { return true }))

		err := c.Run()
		if err == nil {
			t.Fatal("Expected an error due to timeout")
		}
		// Verify the error is due to timeout
		if !stderrs.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected error wrapping context.DeadlineExceeded, got %v (type %T)", err, err)
		}
		// Expect only one attempt due to timeout
		if attempts != 1 {
			t.Errorf("Expected exactly 1 attempt before timeout, got %d", attempts)
		}
	})
}

// TestChainContext tests context-related behavior, specifically timeouts.
// It verifies that timeouts stop execution as expected.
func TestChainContext(t *testing.T) {
	// Subtest: TimeoutStopsExecution
	// Verifies that a chain timeout prevents subsequent steps from running.
	t.Run("TimeoutStopsExecution", func(t *testing.T) {
		var step1Started, step2Executed bool
		c := NewChain(ChainWithTimeout(20 * time.Millisecond)).
			Step(func() error {
				step1Started = true
				// Sleep longer than the timeout
				time.Sleep(50 * time.Millisecond)
				return nil
			}).
			Step(func() error {
				step2Executed = true
				return nil
			})

		err := c.Run()

		if err == nil {
			t.Fatal("Expected an error")
		}
		// Verify the error is due to timeout
		if !stderrs.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded wrapped, got %v", err)
		}
		if !step1Started {
			t.Error("Step 1 should have started execution")
		}
		if step2Executed {
			t.Error("Step 2 should not have executed after timeout")
		}
	})
}

// TestChainLogging tests logging behavior for failing steps.
// It verifies log messages and attributes for optional and non-optional steps.
func TestChainLogging(t *testing.T) {
	logHandler := NewMemoryLogHandler()

	// Subtest: LogOnFail_NonOptional
	// Verifies that a non-optional failing step logs with all metadata.
	t.Run("LogOnFail_NonOptional", func(t *testing.T) {
		logHandler.Reset()
		category := ErrorCategory("test_cat")
		c := NewChain(ChainWithLogHandler(logHandler)).
			Step(func() error { return errTest }).LogOnFail().
			With("key", "value").Tag(category).Code(500)

		err := c.Run()
		if err == nil {
			t.Fatal("Expected error")
		}

		logOutput := logHandler.GetOutput()
		// Verify error message in logs
		if !strings.Contains(logOutput, "test error") {
			t.Errorf("Log missing 'error=test error' attribute. Got: %s", logOutput)
		}
		// Verify category
		if !strings.Contains(logOutput, "category=test_cat") {
			t.Errorf("Log missing 'category=test_cat' attribute. Got: %s", logOutput)
		}
		// Verify error code
		if !strings.Contains(logOutput, "code=500") {
			t.Errorf("Log missing 'code=500' attribute. Got: %s", logOutput)
		}
		// Verify context metadata
		if !strings.Contains(logOutput, "key=value") {
			t.Errorf("Log missing 'key=value' attribute. Got: %s", logOutput)
		}
		// Verify log message
		if !strings.Contains(logOutput, "Chain stopped due to error in step") {
			t.Errorf("Log missing correct message. Got: %s", logOutput)
		}
	})

	// Subtest: LogOnFail_Optional
	// Verifies that an optional failing step logs correctly when configured.
	t.Run("LogOnFail_Optional", func(t *testing.T) {
		logHandler.Reset()
		category := ErrorCategory("opt_cat")
		c := NewChain(ChainWithLogHandler(logHandler)).
			Step(func() error { return errOptional }).Optional().LogOnFail().
			With("optKey", "optValue").Tag(category)

		err := c.Run()
		if err != nil {
			t.Fatalf("Run should succeed when only optional fails, got: %v", err)
		}

		logOutput := logHandler.GetOutput()
		// Verify log message for optional failure
		if !strings.Contains(logOutput, "Optional step failed") {
			t.Errorf("Log should contain 'Optional step failed' message: %s", logOutput)
		}
		// Verify error message
		if !strings.Contains(logOutput, "error=optional error") {
			t.Errorf("Log should contain 'error=optional error': %s", logOutput)
		}
		// Verify category
		if !strings.Contains(logOutput, "category=opt_cat") {
			t.Errorf("Log missing 'category=opt_cat': %s", logOutput)
		}
		// Verify context metadata
		if !strings.Contains(logOutput, "optKey=optValue") {
			t.Errorf("Log missing 'optKey=optValue': %s", logOutput)
		}
	})

	// Subtest: NoLogOnFail_Optional
	// Verifies that an optional failing step doesn’t log without LogOnFail.
	t.Run("NoLogOnFail_Optional", func(t *testing.T) {
		logHandler.Reset()
		c := NewChain(ChainWithLogHandler(logHandler)).
			Step(func() error { return errOptional }).Optional()

		err := c.Run()
		if err != nil {
			t.Fatalf("Run should succeed when only optional fails, got: %v", err)
		}

		logOutput := logHandler.GetOutput()
		if logOutput != "" {
			t.Errorf("Expected no log output without LogOnFail, got: %s", logOutput)
		}
	})
}

// TestChainRunAll tests the RunAll method.
// It verifies error collection and max error limits.
func TestChainRunAll(t *testing.T) {
	// Subtest: CollectAllErrors
	// Verifies that RunAll collects all errors and executes all steps.
	t.Run("CollectAllErrors", func(t *testing.T) {
		var step2Executed bool
		c := NewChain().
			Step(func() error { return errStep1 }).
			Step(func() error { step2Executed = true; return nil }).Optional().
			Step(func() error { return errStep2 })

		err := c.RunAll()

		if !step2Executed {
			t.Error("Optional successful step should have executed in RunAll")
		}
		// Verify the error is a MultiError
		multiErr, ok := err.(*MultiError)
		if !ok {
			t.Fatalf("Expected *MultiError, got %T", err)
		}
		// Check that exactly two errors were collected
		if len(multiErr.Errors()) != 2 {
			t.Errorf("Expected 2 errors collected in RunAll, got %d", len(multiErr.Errors()))
		}
	})

	// Subtest: RunAllWithMaxErrors
	// Verifies that RunAll stops after reaching the max error limit.
	t.Run("RunAllWithMaxErrors", func(t *testing.T) {
		var step3Executed bool
		c := NewChain(ChainWithMaxErrors(2)).
			Step(func() error { return errStep1 }).
			Step(func() error { return errStep2 }).
			Step(func() error { step3Executed = true; return errStep3 })

		err := c.RunAll()

		if step3Executed {
			t.Error("Step 3 should not have executed after MaxErrors limit")
		}
		// Verify the error is a MultiError
		multiErr, ok := err.(*MultiError)
		if !ok {
			t.Fatalf("Expected MultiError, got %T", err)
		}
		// Check that only two errors were collected due to the limit
		if len(multiErr.Errors()) != 2 {
			t.Errorf("Expected exactly 2 errors due to max limit, got %d", len(multiErr.Errors()))
		}
	})
}

// TestChainReset tests the Reset method.
// It verifies that the chain is fully cleared.
func TestChainReset(t *testing.T) {
	// Create a chain with a step, timeout, and metadata
	c := NewChain(ChainWithTimeout(1*time.Second)).
		Step(func() error { return errTest }).With("key", "value")

	_ = c.Run()

	// Reset the chain
	c.Reset()

	// Verify the chain is empty
	if c.Len() != 0 {
		t.Errorf("Reset chain should have 0 steps, got %d", c.Len())
	}
	if c.HasErrors() {
		t.Errorf("Reset chain should have 0 errors, got %v", c.Errors())
	}
	if c.lastStep != nil {
		t.Error("Reset chain should have nil lastStep")
	}
}

// TestChainReflectionCall tests the Call method with reflection.
// It verifies that functions with arguments are handled correctly.
func TestChainReflectionCall(t *testing.T) {
	// Subtest: CallWithArgsFailure
	// Verifies that a function with arguments returns an enhanced error.
	t.Run("CallWithArgsFailure", func(t *testing.T) {
		internalErr := fmt.Errorf("failure with %d", 10)
		fn := func(a int) error { return internalErr }

		c := NewChain().Call(fn, 10)
		err := c.Run()

		if err == nil {
			t.Fatal("Expected error from Call")
		}
		// Verify the error is enhanced
		enhancedErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("Expected wrapped *errors.Error, got %T", err)
		}
		// Check that it wraps the original error
		if !stderrs.Is(enhancedErr, internalErr) {
			t.Errorf("Expected enhanced error to wrap '%v', got '%v'", internalErr, enhancedErr)
		}
	})
}

// TestChainErrorInspection tests error inspection methods.
// It verifies LastError and Errors after execution.
func TestChainErrorInspection(t *testing.T) {
	// Create a chain with two failing steps
	c := NewChain().
		Step(func() error { return errStep1 }).
		Step(func() error { return errStep2 })

	_ = c.RunAll()

	// Verify the last error
	lastErr := c.LastError()
	if lastErr == nil {
		t.Fatal("LastError should not be nil after RunAll")
	}
	if !stderrs.Is(lastErr, errStep2) {
		t.Errorf("LastError should wrap %v, got %v", errStep2, lastErr)
	}
	// Verify the number of collected errors
	if len(c.Errors()) != 2 {
		t.Errorf("Expected 2 errors collected, got %d", len(c.Errors()))
	}
}
