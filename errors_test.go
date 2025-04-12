// Package errors provides a robust error handling library with support for
// error wrapping, stack traces, context storage, and retry mechanisms.
// This test file verifies the correctness of the error type and its methods,
// ensuring proper behavior for creation, wrapping, inspection, and serialization.
// Tests cover edge cases, standard library compatibility, and thread-safety.
package errors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// customError is a test-specific error type for verifying error wrapping and traversal.
type customError struct {
	msg   string
	cause error
}

func (e *customError) Error() string {
	return e.msg
}

func (e *customError) Cause() error {
	return e.cause
}

// TestErrorNew verifies that New creates an error with the specified message
// and does not capture a stack trace, ensuring lightweight error creation.
func TestErrorNew(t *testing.T) {
	err := New("test error")
	defer err.Free()
	if err.Error() != "test error" {
		t.Errorf("New() error message = %v, want %v", err.Error(), "test error")
	}
	if len(err.Stack()) != 0 {
		t.Errorf("New() should not capture stack trace, got %d frames", len(err.Stack()))
	}
}

// TestErrorNewf checks that Newf formats the error message correctly using
// the provided format string and arguments, without capturing a stack trace.
func TestErrorNewf(t *testing.T) {
	err := Newf("test %s %d", "error", 42)
	defer err.Free()
	want := "test error 42"
	if err.Error() != want {
		t.Errorf("Newf() error message = %v, want %v", err.Error(), want)
	}
	if len(err.Stack()) != 0 {
		t.Errorf("Newf() should not capture stack trace, got %d frames", len(err.Stack()))
	}
}

// TestErrorNamed ensures that Named creates a named error with the given name
// and captures a stack trace for debugging purposes.
func TestErrorNamed(t *testing.T) {
	err := Named("test_name")
	defer err.Free()
	if err.Error() != "test_name" {
		t.Errorf("Named() error message = %v, want %v", err.Error(), "test_name")
	}
	if len(err.Stack()) == 0 {
		t.Errorf("Named() should capture stack trace")
	}
}

// TestErrorMethods tests the core methods of the Error type, including context
// addition, wrapping, message formatting, stack tracing, and metadata handling.
func TestErrorMethods(t *testing.T) {
	err := New("base error")
	defer err.Free()

	// Test With for adding key-value context.
	err = err.With("key", "value")
	if err.Context()["key"] != "value" {
		t.Errorf("With() failed, context[key] = %v, want %v", err.Context()["key"], "value")
	}

	// Test Wrap for setting a cause.
	cause := New("cause error")
	defer cause.Free()
	err = err.Wrap(cause)
	if err.Unwrap() != cause {
		t.Errorf("Wrap() failed, unwrapped = %v, want %v", err.Unwrap(), cause)
	}

	// Test Msgf for updating the error message.
	err = err.Msgf("new message %d", 123)
	if err.Error() != "new message 123: cause error" {
		t.Errorf("Msgf() failed, error = %v, want %v", err.Error(), "new message 123: cause error")
	}

	// Test stack absence initially.
	stackLen := len(err.Stack())
	if stackLen != 0 {
		t.Errorf("Initial stack length should be 0, got %d", stackLen)
	}

	// Test Trace for capturing a stack trace.
	err = err.Trace()
	if len(err.Stack()) == 0 {
		t.Errorf("Trace() should capture a stack trace, got no frames")
	}

	// Test WithCode for setting an HTTP status code.
	err = err.WithCode(400)
	if err.Code() != 400 {
		t.Errorf("WithCode() failed, code = %d, want 400", err.Code())
	}

	// Test WithCategory for setting a category.
	err = err.WithCategory("test_category")
	if Category(err) != "test_category" {
		t.Errorf("WithCategory() failed, category = %v, want %v", Category(err), "test_category")
	}

	// Test Increment for counting occurrences.
	err = err.Increment()
	if err.Count() != 1 {
		t.Errorf("Increment() failed, count = %d, want 1", err.Count())
	}
}

// TestErrorIs verifies that Is correctly identifies errors by name or through
// wrapping, including compatibility with standard library errors.
func TestErrorIs(t *testing.T) {
	err := Named("test_error")
	defer err.Free()
	err2 := Named("test_error")
	defer err2.Free()
	err3 := Named("other_error")
	defer err3.Free()

	// Test matching same-named errors.
	if !err.Is(err2) {
		t.Errorf("Is() failed, %v should match %v", err, err2)
	}
	// Test non-matching names.
	if err.Is(err3) {
		t.Errorf("Is() failed, %v should not match %v", err, err3)
	}

	// Test wrapped error matching.
	wrappedErr := Named("wrapper")
	defer wrappedErr.Free()
	cause := Named("cause_error")
	defer cause.Free()
	wrappedErr = wrappedErr.Wrap(cause)
	if !wrappedErr.Is(cause) {
		t.Errorf("Is() failed, wrapped error should match cause; wrappedErr = %+v, cause = %+v", wrappedErr, cause)
	}

	// Test wrapping standard library error.
	stdErr := errors.New("std error")
	wrappedErr = wrappedErr.Wrap(stdErr)
	if !wrappedErr.Is(stdErr) {
		t.Errorf("Is() failed, should match stdlib error")
	}
}

// TestErrorAs checks that As unwraps to the correct error type, supporting
// both custom *Error and standard library errors.
func TestErrorAs(t *testing.T) {
	err := New("base").Wrap(Named("target"))
	defer err.Free()
	var target *Error
	if !As(err, &target) {
		t.Errorf("As() failed, should unwrap to *Error")
	}
	if target.name != "target" {
		t.Errorf("As() unwrapped to wrong error, got %v, want %v", target.name, "target")
	}

	stdErr := errors.New("std error")
	err = New("wrapper").Wrap(stdErr)
	defer err.Free()
	var stdTarget error
	if !As(err, &stdTarget) {
		t.Errorf("As() failed, should unwrap to stdlib error")
	}
	if stdTarget != stdErr {
		t.Errorf("As() unwrapped to wrong error, got %v, want %v", stdTarget, stdErr)
	}
}

// TestErrorCount verifies that Count tracks per-instance error occurrences.
func TestErrorCount(t *testing.T) {
	err := New("unnamed")
	defer err.Free()
	if err.Count() != 0 {
		t.Errorf("Count() on new error should be 0, got %d", err.Count())
	}

	err = Named("test_count").Increment()
	if err.Count() != 1 {
		t.Errorf("Count() after Increment() should be 1, got %d", err.Count())
	}
}

// TestErrorCode ensures that Code correctly sets and retrieves HTTP status codes.
func TestErrorCode(t *testing.T) {
	err := New("unnamed")
	defer err.Free()
	if err.Code() != 0 {
		t.Errorf("Code() on new error should be 0, got %d", err.Code())
	}

	err = Named("test_code").WithCode(400)
	if err.Code() != 400 {
		t.Errorf("Code() after WithCode(400) should be 400, got %d", err.Code())
	}
}

// TestErrorMarshalJSON verifies that JSON serialization includes all expected
// fields: message, context, cause, code, and stack (when present).
func TestErrorMarshalJSON(t *testing.T) {
	// Test basic error with context, code, and cause.
	err := New("test").
		With("key", "value").
		WithCode(400).
		Wrap(Named("cause"))
	defer err.Free()
	data, e := json.Marshal(err)
	if e != nil {
		t.Fatalf("MarshalJSON() failed: %v", e)
	}

	want := map[string]interface{}{
		"message": "test",
		"context": map[string]interface{}{"key": "value"},
		"cause":   map[string]interface{}{"name": "cause"},
		"code":    float64(400),
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got["message"] != want["message"] {
		t.Errorf("MarshalJSON() message = %v, want %v", got["message"], want["message"])
	}
	if !reflect.DeepEqual(got["context"], want["context"]) {
		t.Errorf("MarshalJSON() context = %v, want %v", got["context"], want["context"])
	}
	if cause, ok := got["cause"].(map[string]interface{}); !ok || cause["name"] != "cause" {
		t.Errorf("MarshalJSON() cause = %v, want %v", got["cause"], want["cause"])
	}
	if code, ok := got["code"].(float64); !ok || code != 400 {
		t.Errorf("MarshalJSON() code = %v, want %v", got["code"], 400)
	}

	// Test error with stack trace.
	t.Run("WithStack", func(t *testing.T) {
		err := New("test").WithStack().WithCode(500)
		defer err.Free()
		data, e := json.Marshal(err)
		if e != nil {
			t.Fatalf("MarshalJSON() failed: %v", e)
		}
		var got map[string]interface{}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if _, ok := got["stack"].([]interface{}); !ok || len(got["stack"].([]interface{})) == 0 {
			t.Error("MarshalJSON() should include non-empty stack")
		}
		if code, ok := got["code"].(float64); !ok || code != 500 {
			t.Errorf("MarshalJSON() code = %v, want 500", got["code"])
		}
	})
}

// TestErrorEdgeCases verifies behavior for unusual inputs, such as nil errors,
// empty names, and standard library error wrapping.
func TestErrorEdgeCases(t *testing.T) {
	// Test nil error handling.
	var nilErr *Error
	if nilErr.Is(nil) {
		t.Errorf("nil.Is(nil) should be false, got true")
	}
	if Is(nilErr, New("test")) {
		t.Errorf("Is(nil, non-nil) should be false")
	}

	// Test empty name mismatch.
	err := New("empty name")
	defer err.Free()
	if err.Is(Named("")) {
		t.Errorf("Error with empty name should not match unnamed error")
	}

	// Test wrapping standard library error.
	stdErr := errors.New("std error")
	customErr := New("custom").Wrap(stdErr)
	defer customErr.Free()
	if !Is(customErr, stdErr) {
		t.Errorf("Is() should match stdlib error through wrapping")
	}

	// Test As with nil error.
	var nilTarget *Error
	if As(nilErr, &nilTarget) {
		t.Errorf("As(nil, &nilTarget) should return false")
	}

	// Additional edge case: Wrapping nil error.
	t.Run("WrapNil", func(t *testing.T) {
		err := New("wrapper").Wrap(nil)
		defer err.Free()
		if err.Unwrap() != nil {
			t.Errorf("Wrap(nil) should set cause to nil, got %v", err.Unwrap())
		}
		if err.Error() != "wrapper" {
			t.Errorf("Wrap(nil) should preserve message, got %v, want %v", err.Error(), "wrapper")
		}
	})
}

// TestErrorRetryWithCallback verifies the retry mechanism, ensuring the callback
// is invoked correctly and retries exhaust as expected for retryable errors.
func TestErrorRetryWithCallback(t *testing.T) {
	// Test retry with multiple attempts.
	attempts := 0
	retry := NewRetry(
		WithMaxAttempts(3),
		WithDelay(1*time.Millisecond),
		WithOnRetry(func(attempt int, err error) {
			attempts++
		}),
	)

	err := retry.Execute(func() error {
		return New("retry me").WithRetryable()
	})

	if attempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", attempts)
	}
	if err == nil {
		t.Error("Expected retry to exhaust with error, got nil")
	}

	// Test zero max attempts, expecting one initial attempt (not a retry).
	t.Run("ZeroAttempts", func(t *testing.T) {
		attempts := 0
		retry := NewRetry(
			WithMaxAttempts(0),
			WithOnRetry(func(attempt int, err error) {
				attempts++
			}),
		)
		err := retry.Execute(func() error {
			return New("retry me").WithRetryable()
		})
		// Expect one attempt, as Execute runs the function once before checking retries.
		if attempts != 1 {
			t.Errorf("Expected 1 attempt (initial execution), got %d", attempts)
		}
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}

// TestErrorStackPresence confirms stack trace behavior for New and Trace methods.
func TestErrorStackPresence(t *testing.T) {
	// New should not capture stack.
	err := New("test")
	if len(err.Stack()) != 0 {
		t.Error("New() should not capture stack")
	}

	// Trace should capture stack.
	traced := Trace("test")
	if len(traced.Stack()) == 0 {
		t.Error("Trace() should capture stack")
	}
}

// TestErrorStackDepth ensures that stack traces respect the configured maximum depth.
func TestErrorStackDepth(t *testing.T) {
	err := Trace("test")
	frames := err.Stack()
	if len(frames) > currentConfig.stackDepth {
		t.Errorf("Stack depth %d exceeds configured max %d", len(frames), currentConfig.stackDepth)
	}
}

// TestErrorTransform verifies Transform behavior for nil, non-*Error, and *Error inputs.
func TestErrorTransform(t *testing.T) {
	// Test nil input.
	t.Run("NilError", func(t *testing.T) {
		result := Transform(nil, func(e *Error) {})
		if result != nil {
			t.Error("Should return nil for nil input")
		}
	})

	// Test standard library error.
	t.Run("NonErrorType", func(t *testing.T) {
		stdErr := errors.New("standard")
		transformed := Transform(stdErr, func(e *Error) {})
		if transformed == nil {
			t.Error("Should not return nil for non-nil input")
		}
		if transformed.Error() != "standard" {
			t.Errorf("Should preserve original message, got %q, want %q", transformed.Error(), "standard")
		}
		if transformed == stdErr {
			t.Error("Should return a new *Error, not the original")
		}
	})

	// Test transforming *Error.
	t.Run("TransformError", func(t *testing.T) {
		orig := New("original")
		defer orig.Free()
		transformed := Transform(orig, func(e *Error) {
			e.With("key", "value")
		})
		defer transformed.Free()

		if transformed == orig {
			t.Error("Should return a copy, not the original")
		}
		if transformed.Error() != "original" {
			t.Errorf("Should preserve original message, got %q, want %q", transformed.Error(), "original")
		}
		if transformed.Context()["key"] != "value" {
			t.Error("Should apply transformations, context missing 'key'='value'")
		}
	})
}

// TestErrorWalk ensures Walk traverses the error chain correctly, visiting all errors.
func TestErrorWalk(t *testing.T) {
	err1 := &customError{msg: "first error", cause: nil}
	err2 := &customError{msg: "second error", cause: err1}
	err3 := &customError{msg: "third error", cause: err2}

	var errorsWalked []string
	Walk(err3, func(e error) {
		errorsWalked = append(errorsWalked, e.Error())
	})

	expected := []string{"third error", "second error", "first error"}
	if !reflect.DeepEqual(errorsWalked, expected) {
		t.Errorf("Walk() = %v; want %v", errorsWalked, expected)
	}
}

// TestErrorFind verifies Find locates the first error matching the predicate.
func TestErrorFind(t *testing.T) {
	err1 := &customError{msg: "first error", cause: nil}
	err2 := &customError{msg: "second error", cause: err1}
	err3 := &customError{msg: "third error", cause: err2}

	// Find existing error.
	found := Find(err3, func(e error) bool {
		return e.Error() == "second error"
	})
	if found == nil || found.Error() != "second error" {
		t.Errorf("Find() = %v; want 'second error'", found)
	}

	// Find non-existent error.
	found = Find(err3, func(e error) bool {
		return e.Error() == "non-existent error"
	})
	if found != nil {
		t.Errorf("Find() = %v; want nil", found)
	}
}

// TestErrorTraceStackContent checks that Trace captures meaningful stack frames.
func TestErrorTraceStackContent(t *testing.T) {
	err := Trace("test")
	defer err.Free()
	frames := err.Stack()
	if len(frames) == 0 {
		t.Fatal("Trace() should capture stack frames")
	}
	found := false
	for _, frame := range frames {
		if strings.Contains(frame, "testing.tRunner") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Trace() stack does not contain testing.tRunner, got: %v", frames)
	}
}

// TestErrorWithStackContent ensures WithStack captures meaningful stack frames.
func TestErrorWithStackContent(t *testing.T) {
	err := New("test").WithStack()
	defer err.Free()
	frames := err.Stack()
	if len(frames) == 0 {
		t.Fatal("WithStack() should capture stack frames")
	}
	found := false
	for _, frame := range frames {
		if strings.Contains(frame, "testing.tRunner") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("WithStack() stack does not contain testing.tRunner, got: %v", frames)
	}
}

// TestErrorWrappingChain verifies a complex error chain with multiple layers,
// ensuring correct message propagation, context isolation, and stack behavior.
func TestErrorWrappingChain(t *testing.T) {
	databaseErr := New("connection timeout").
		With("timeout_sec", 5).
		With("server", "db01.prod")
	defer databaseErr.Free()

	businessErr := New("failed to process user 12345").
		With("user_id", "12345").
		With("stage", "processing").
		Wrap(databaseErr)
	defer businessErr.Free()

	apiErr := New("API request failed").
		WithCode(500).
		WithStack().
		Wrap(businessErr)
	defer apiErr.Free()

	// Verify full error message.
	expectedFullMessage := "API request failed: failed to process user 12345: connection timeout"
	if apiErr.Error() != expectedFullMessage {
		t.Errorf("Full error message mismatch\ngot: %q\nwant: %q", apiErr.Error(), expectedFullMessage)
	}

	// Verify error chain.
	chain := UnwrapAll(apiErr)
	if len(chain) != 3 {
		t.Fatalf("Expected chain length 3, got %d", len(chain))
	}

	tests := []struct {
		index    int
		expected string
	}{
		{0, "API request failed"},
		{1, "failed to process user 12345"},
		{2, "connection timeout"},
	}
	for _, tt := range tests {
		if chain[tt.index].Error() != tt.expected {
			t.Errorf("Chain position %d mismatch\ngot: %q\nwant: %q", tt.index, chain[tt.index].Error(), tt.expected)
		}
	}

	// Verify Is checks.
	if !errors.Is(apiErr, databaseErr) {
		t.Error("Is() should match the database error in the chain")
	}

	// Verify context isolation.
	if ctx := businessErr.Context(); ctx["timeout_sec"] != nil {
		t.Error("Business error should not have database context")
	}

	// Verify stack presence.
	if stack := apiErr.Stack(); len(stack) == 0 {
		t.Error("API error should have stack trace")
	}
	if stack := businessErr.Stack(); len(stack) != 0 {
		t.Error("Business error should not have stack trace")
	}

	// Verify code propagation.
	if apiErr.Code() != 500 {
		t.Error("API error should have code 500")
	}
	if businessErr.Code() != 0 {
		t.Error("Business error should have no code")
	}
}

// TestErrorExampleOutput verifies that formatted output includes all relevant
// details, such as message, context, code, and stack, for a realistic error chain.
func TestErrorExampleOutput(t *testing.T) {
	databaseErr := New("connection timeout").
		With("timeout_sec", 5).
		With("server", "db01.prod")
	businessErr := New("failed to process user 12345").
		With("user_id", "12345").
		With("stage", "processing").
		Wrap(databaseErr)
	apiErr := New("API request failed").
		WithCode(500).
		WithStack().
		Wrap(businessErr)

	chain := UnwrapAll(apiErr)
	for _, err := range chain {
		if e, ok := err.(*Error); ok {
			formatted := e.Format()
			if formatted == "" {
				t.Error("Format() returned empty string")
			}
			if !strings.Contains(formatted, "Error: "+e.Error()) {
				t.Errorf("Format() output missing error message: %q", formatted)
			}
			if e == apiErr {
				if !strings.Contains(formatted, "Code: 500") {
					t.Error("Format() missing code for API error")
				}
				if !strings.Contains(formatted, "Stack:") {
					t.Error("Format() missing stack for API error")
				}
			}
			if e == businessErr {
				if ctx := e.Context(); ctx != nil {
					if !strings.Contains(formatted, "Context:") {
						t.Error("Format() missing context for business error")
					}
					for k := range ctx {
						if !strings.Contains(formatted, k) {
							t.Errorf("Format() missing context key %q", k)
						}
					}
				}
			}
		}
	}

	if !errors.Is(apiErr, errors.New("connection timeout")) {
		t.Error("Is() failed to match connection timeout error")
	}
}

// TestErrorFullChain tests a complex chain with mixed error types (custom and standard),
// verifying wrapping, unwrapping, and compatibility with standard library functions.
func TestErrorFullChain(t *testing.T) {
	stdErr := errors.New("file not found")
	authErr := Named("AuthError").WithCode(401)
	storageErr := Wrapf(stdErr, "storage failed")
	authErrWrapped := Wrap(storageErr, authErr)
	wrapped := Wrapf(authErrWrapped, "request failed")

	var targetAuth *Error
	expectedTopLevelMsg := "request failed: AuthError: storage failed: file not found"
	if !errors.As(wrapped, &targetAuth) || targetAuth.Error() != expectedTopLevelMsg {
		t.Errorf("stderrors.As(wrapped, &targetAuth) failed, got %v, want %q", targetAuth.Error(), expectedTopLevelMsg)
	}

	var targetAuthPtr *Error
	if !As(wrapped, &targetAuthPtr) || targetAuthPtr.Name() != "AuthError" || targetAuthPtr.Code() != 401 {
		t.Errorf("As(wrapped, &targetAuthPtr) failed, got name=%s, code=%d; want AuthError, 401", targetAuthPtr.Name(), targetAuthPtr.Code())
	}

	if !Is(wrapped, authErr) {
		t.Errorf("Is(wrapped, authErr) failed, expected true")
	}
	if !errors.Is(wrapped, authErr) {
		t.Errorf("stderrors.Is(wrapped, authErr) failed, expected true")
	}
	if !Is(wrapped, stdErr) {
		t.Errorf("Is(wrapped, stdErr) failed, expected true")
	}
	if !errors.Is(wrapped, stdErr) {
		t.Errorf("stderrors.Is(wrapped, stdErr) failed, expected true")
	}

	chain := UnwrapAll(wrapped)
	if len(chain) != 4 {
		t.Errorf("UnwrapAll(wrapped) length = %d, want 4", len(chain))
	}
	expected := []string{
		"request failed",
		"AuthError",
		"storage failed",
		"file not found",
	}
	for i, err := range chain {
		if err.Error() != expected[i] {
			t.Errorf("UnwrapAll[%d] = %v, want %v", i, err.Error(), expected[i])
		}
	}
}

// TestErrorUnwrapAllMessageIsolation ensures UnwrapAll preserves individual error messages.
func TestErrorUnwrapAllMessageIsolation(t *testing.T) {
	inner := New("inner")
	middle := New("middle").Wrap(inner)
	outer := New("outer").Wrap(middle)

	chain := UnwrapAll(outer)
	if chain[0].Error() != "outer" {
		t.Errorf("Expected 'outer', got %q", chain[0].Error())
	}
	if chain[1].Error() != "middle" {
		t.Errorf("Expected 'middle', got %q", chain[1].Error())
	}
	if chain[2].Error() != "inner" {
		t.Errorf("Expected 'inner', got %q", chain[2].Error())
	}
}

// TestErrorIsEmpty verifies IsEmpty behavior for various error states, including
// nil, empty messages, and errors with causes or templates.
func TestErrorIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected bool
	}{
		{"nil error", nil, true},
		{"empty error", New(""), true},
		{"named empty", Named(""), true},
		{"with empty template", New("").WithTemplate(""), true},
		{"with message", New("test"), false},
		{"with name", Named("test"), false},
		{"with template", New("").WithTemplate("template"), false},
		{"with cause", New("").Wrap(New("cause")), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err != nil {
				defer tt.err.Free()
			}
			if got := tt.err.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestErrorIsNull verifies IsNull behavior for null and non-null errors, including
// SQL null values in context or causes.
func TestErrorIsNull(t *testing.T) {
	nullString := sql.NullString{Valid: false}
	validString := sql.NullString{String: "test", Valid: true}

	tests := []struct {
		name     string
		err      *Error
		expected bool
	}{
		{"nil error", nil, true},
		{"empty error", New(""), false},
		{"with NULL context", New("").With("data", nullString), true},
		{"with valid context", New("").With("data", validString), false},
		{"with NULL cause", New("").Wrap(New("NULL value").With("data", nullString)), true},
		{"with valid cause", New("").Wrap(New("valid value").With("data", validString)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err != nil {
				defer tt.err.Free()
			}
			if got := tt.err.IsNull(); got != tt.expected {
				t.Errorf("IsNull() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestErrorFromContext ensures FromContext enhances errors with context information,
// such as deadlines and cancellations.
func TestErrorFromContext(t *testing.T) {
	// Test nil error.
	t.Run("nil error returns nil", func(t *testing.T) {
		ctx := context.Background()
		if FromContext(ctx, nil) != nil {
			t.Error("Expected nil for nil input error")
		}
	})

	// Test deadline exceeded.
	t.Run("deadline exceeded", func(t *testing.T) {
		deadline := time.Now().Add(-1 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		err := errors.New("operation failed")
		cerr := FromContext(ctx, err)

		if !IsTimeout(cerr) {
			t.Error("Expected timeout error")
		}
		if !HasContextKey(cerr, "deadline") {
			t.Error("Expected deadline in context")
		}
	})

	// Test cancelled context.
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := errors.New("operation failed")
		cerr := FromContext(ctx, err)

		if !HasContextKey(cerr, "cancelled") {
			t.Error("Expected cancelled flag")
		}
	})
}

// TestContextStorage verifies the smallContext optimization and its expansion
// to a full map, including thread-safety under concurrent access.
func TestContextStorage(t *testing.T) {
	// Test smallContext for first 4 items.
	t.Run("stores first 4 items in smallContext", func(t *testing.T) {
		err := New("test")

		err.With("a", 1)
		err.With("b", 2)
		err.With("c", 3)
		err.With("d", 4)

		if err.smallCount != 4 {
			t.Errorf("expected smallCount=4, got %d", err.smallCount)
		}
		if err.context != nil {
			t.Error("expected context map to be nil")
		}
	})

	// Test expansion to map on 5th item.
	t.Run("switches to map on 5th item", func(t *testing.T) {
		err := New("test")

		err.With("a", 1)
		err.With("b", 2)
		err.With("c", 3)
		err.With("d", 4)
		err.With("e", 5)

		if err.context == nil {
			t.Error("expected context map to be initialized")
		}
		if len(err.context) != 5 {
			t.Errorf("expected 5 items in map, got %d", len(err.context))
		}
	})

	// Test preservation of all context items.
	t.Run("preserves all context items", func(t *testing.T) {
		err := New("test")
		items := []struct {
			k string
			v interface{}
		}{
			{"a", 1}, {"b", 2}, {"c", 3},
			{"d", 4}, {"e", 5}, {"f", 6},
		}

		for _, item := range items {
			err.With(item.k, item.v)
		}

		ctx := err.Context()
		if len(ctx) != len(items) {
			t.Errorf("expected %d items, got %d", len(items), len(ctx))
		}
		for _, item := range items {
			if val, ok := ctx[item.k]; !ok || val != item.v {
				t.Errorf("missing item %s in context", item.k)
			}
		}
	})

	// Test concurrent access safety.
	t.Run("concurrent access", func(t *testing.T) {
		err := New("test")
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			err.With("a", 1)
			err.With("b", 2)
			err.With("c", 3)
		}()

		go func() {
			defer wg.Done()
			err.With("d", 4)
			err.With("e", 5)
			err.With("f", 6)
		}()

		wg.Wait()
		ctx := err.Context()
		if len(ctx) != 6 {
			t.Errorf("expected 6 items, got %d", len(ctx))
		}
	})
}

// TestNewf verifies Newf behavior, including %w wrapping, formatting, and error cases.
// errors_test.go

// TestNewf verifies Newf behavior, including %w wrapping, formatting, and error cases.
// It now expects the string output for %w cases to match fmt.Errorf.
func TestNewf(t *testing.T) {
	// Reusable error instances for testing %w
	stdErrorInstance := errors.New("std error")
	customErrorInstance := New("custom error") // Assuming this exists in your tests
	firstErrorInstance := New("first")
	secondErrorInstance := New("second")

	tests := []struct {
		name            string
		format          string
		args            []interface{}
		wantFinalMsg    string // EXPECTATION UPDATED TO MATCH fmt.Errorf
		wantInternalMsg string // This field might be less relevant now, maybe remove? Kept for reference.
		wantCause       error
		wantErrFormat   bool // Indicates if Newf itself should return a format error message
	}{
		// Basic formatting (no change needed)
		{
			name:            "simple string",
			format:          "simple %s",
			args:            []interface{}{"test"},
			wantFinalMsg:    "simple test",
			wantInternalMsg: "simple test", // Stays same as FinalMsg when no %w
		},
		{
			name:            "complex format without %w",
			format:          "code=%d msg=%s",
			args:            []interface{}{123, "hello"},
			wantFinalMsg:    "code=123 msg=hello",
			wantInternalMsg: "code=123 msg=hello",
		},
		{
			name:            "empty format no args",
			format:          "",
			args:            []interface{}{},
			wantFinalMsg:    "",
			wantInternalMsg: "",
		},

		// --- %w wrapping cases (EXPECTATIONS UPDATED) ---
		{
			name:            "wrap standard error",
			format:          "prefix %w",
			args:            []interface{}{stdErrorInstance},
			wantFinalMsg:    "prefix std error", // Matches fmt.Errorf output
			wantInternalMsg: "prefix std error", // Now wantInternalMsg matches FinalMsg for %w
			wantCause:       stdErrorInstance,
		},
		{
			name:            "wrap custom error",
			format:          "prefix %w",
			args:            []interface{}{customErrorInstance},
			wantFinalMsg:    "prefix custom error", // Matches fmt.Errorf output
			wantInternalMsg: "prefix custom error",
			wantCause:       customErrorInstance,
		},
		{
			name:            "%w at start",
			format:          "%w suffix",
			args:            []interface{}{stdErrorInstance},
			wantFinalMsg:    "std error suffix", // Matches fmt.Errorf output
			wantInternalMsg: "std error suffix",
			wantCause:       stdErrorInstance,
		},
		{
			name:            "%w with flags (flags ignored by %w)",
			format:          "prefix %+w suffix", // fmt.Errorf ignores flags like '+' for %w
			args:            []interface{}{stdErrorInstance},
			wantFinalMsg:    "prefix std error suffix", // Matches fmt.Errorf output
			wantInternalMsg: "prefix std error suffix",
			wantCause:       stdErrorInstance,
		},
		{
			name:            "no space around %w",
			format:          "prefix%wsuffix",
			args:            []interface{}{stdErrorInstance},
			wantFinalMsg:    "prefixstd errorsuffix", // Matches fmt.Errorf output
			wantInternalMsg: "prefixstd errorsuffix",
			wantCause:       stdErrorInstance,
		},
		{
			name:            "format becomes empty after removing %w",
			format:          "%w",
			args:            []interface{}{stdErrorInstance},
			wantFinalMsg:    "std error", // Matches fmt.Errorf output
			wantInternalMsg: "std error",
			wantCause:       stdErrorInstance,
		},

		// Error cases (no change needed in expectations, as these test Newf's error messages)
		{
			name:            "multiple %w",
			format:          "%w %w",
			args:            []interface{}{firstErrorInstance, secondErrorInstance},
			wantFinalMsg:    `errors.Newf: format "%w %w" has multiple %w verbs`,
			wantInternalMsg: `errors.Newf: format "%w %w" has multiple %w verbs`,
			wantErrFormat:   true,
		},
		{
			name:            "no args for %w",
			format:          "prefix %w",
			args:            []interface{}{},
			wantFinalMsg:    `errors.Newf: format "prefix %w" has %w but not enough arguments`,
			wantInternalMsg: `errors.Newf: format "prefix %w" has %w but not enough arguments`,
			wantErrFormat:   true,
		},
		{
			name:            "non-error for %w",
			format:          "prefix %w",
			args:            []interface{}{"not an error"},
			wantFinalMsg:    `errors.Newf: argument 0 for %w is not a non-nil error (string)`,
			wantInternalMsg: `errors.Newf: argument 0 for %w is not a non-nil error (string)`,
			wantErrFormat:   true,
		},
		{
			name:            "nil error for %w",
			format:          "prefix %w",
			args:            []interface{}{error(nil)},
			wantFinalMsg:    `errors.Newf: argument 0 for %w is not a non-nil error (<nil>)`,
			wantInternalMsg: `errors.Newf: argument 0 for %w is not a non-nil error (<nil>)`,
			wantErrFormat:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Need to ensure pooled errors are freed if they are used in args
			// Safest is often to recreate them inside the test run if pooling is enabled
			// For simplicity here, assuming they are managed correctly or pooling is off
			// If customErrorInstance is pooled, it needs defer Free() or similar management.

			got := Newf(tt.format, tt.args...)
			if got == nil {
				t.Fatalf("Newf() returned nil, expected *Error")
			}
			// Consider defer got.Free() if AutoFree is false in config

			if gotMsg := got.Error(); gotMsg != tt.wantFinalMsg {
				t.Errorf("Newf().Error() = %q, want %q", gotMsg, tt.wantFinalMsg)
			}

			// --- Cause verification remains crucial ---
			gotCause := errors.Unwrap(got)
			if tt.wantCause != nil {
				// Use errors.Is for robust checking, especially if causes might be wrapped themselves
				if gotCause == nil {
					t.Errorf("Newf() cause = nil, want %v (%T)", tt.wantCause, tt.wantCause)
				} else if !errors.Is(got, tt.wantCause) { // Check the chain
					t.Errorf("Newf() cause mismatch (using Is): got chain does not contain %v (%T)", tt.wantCause, tt.wantCause)
				} else if gotCause != tt.wantCause {
					// Optional: Also check direct cause equality if important
					// t.Logf("Note: Unwrap() direct cause = %v (%T), expected %v (%T)", gotCause, gotCause, tt.wantCause, tt.wantCause)
				}
			} else { // Expected no cause
				if gotCause != nil {
					t.Errorf("Newf() cause = %v (%T), want nil", gotCause, gotCause)
				}
			}

			// If we expected a format error, the cause should definitely be nil
			if tt.wantErrFormat && gotCause != nil {
				t.Errorf("Newf() returned format error %q but unexpectedly set cause to %v", got.Error(), gotCause)
			}

			// Check internal message field if still relevant (might remove this check)
			// if !tt.wantErrFormat && got.msg != tt.wantInternalMsg {
			//  t.Errorf("Newf().msg internal field = %q, want %q", got.msg, tt.wantInternalMsg)
			// }
		})
	}
}

// TestNewfCompatibilityWithFmtErrorf compares the functional behavior of this library's
// Newf function (when using the %w verb) with the standard library's fmt.Errorf.
//
// Rationale for using compareWrappedErrorStrings helper:
//
//  1. Goal: Ensure essential compatibility - correct error wrapping (for Unwrap/Is/As)
//     and preservation of the message content surrounding the wrapped error.
//  2. Formatting Difference: This library consistently formats wrapped errors in its
//     Error() method as "MESSAGE: CAUSE_ERROR" (or just "CAUSE_ERROR" if MESSAGE is empty).
//     The standard fmt.Errorf has more complex and variable spacing rules depending on
//     characters around %w (e.g., sometimes omitting the colon, adding spaces differently).
//  3. Semantic Comparison: Attempting to replicate fmt.Errorf's exact spacing makes the
//     library code brittle and overly complex. Therefore, this test focuses on *semantic*
//     equivalence rather than exact string matching.
//  4. Helper Logic: compareWrappedErrorStrings verifies compatibility by:
//     a) Checking that errors.Unwrap returns the same underlying cause instance.
//     b) Extracting the textual prefix from this library's error string (before ": CAUSE").
//     c) Extracting the textual remainder from fmt.Errorf's string by removing the cause string.
//     d) Normalizing both extracted parts (trimming space, collapsing internal whitespace).
//     e) Comparing the normalized parts to ensure the core message content matches.
//
// This approach ensures functional compatibility without being overly sensitive to minor
// formatting variations between the libraries.
func TestNewfCompatibilityWithFmtErrorf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		argsFn func() []interface{} // Fresh args for each run
	}{
		{"simple %w", "simple %w", func() []interface{} { return []interface{}{errors.New("error")} }},
		{"complex %s %d %w", "complex %s %d %w", func() []interface{} { return []interface{}{"test", 42, errors.New("error")} }},
		{"no space %w next", "no space %w next", func() []interface{} { return []interface{}{errors.New("error")} }},
		{"%w starts", "%w starts", func() []interface{} { return []interface{}{errors.New("error")} }},
		{"format is only %w", "%w", func() []interface{} { return []interface{}{errors.New("error")} }},
		{"%w with flags", "%+w suffix", func() []interface{} { return []interface{}{errors.New("error")} }}, // fmt.Errorf ignores flags
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.argsFn()
			var causeErrArg error // Find the error argument used for %w
			for _, arg := range args {
				if e, ok := arg.(error); ok {
					causeErrArg = e
					break // Assume the first error found is the one for %w
				}
			}
			if causeErrArg == nil {
				t.Fatalf("Test setup error: Could not find error argument for %%w in args: %v", args)
			}

			// Generate errors using both libraries
			stdErr := fmt.Errorf(tt.format, args...)
			customErrImpl := Newf(tt.format, args...)
			if customErrImpl == nil {
				t.Fatalf("Newf returned nil unexpectedly")
			}
			// Consider defer customErrImpl.Free() if needed

			// --- Verify Cause ---
			stdUnwrapped := errors.Unwrap(stdErr)
			customUnwrapped := errors.Unwrap(customErrImpl)

			if stdUnwrapped == nil || customUnwrapped == nil {
				t.Errorf("Expected both errors to be unwrappable, stdUnwrap=%v, customUnwrap=%v", stdUnwrapped, customUnwrapped)
			} else {
				// Check if the unwrapped errors are the *same instance* we passed in
				if customUnwrapped != causeErrArg {
					t.Errorf("Custom error did not unwrap to the original cause instance.\n got: %p (%T)\n want: %p (%T)", customUnwrapped, customUnwrapped, causeErrArg, causeErrArg)
				}
				if stdUnwrapped != causeErrArg {
					// This check is more about validating the test itself
					t.Logf("Standard error did not unwrap to the original cause instance (test validation).\n got: %p (%T)\n want: %p (%T)", stdUnwrapped, stdUnwrapped, causeErrArg, causeErrArg)
				}
				// Verify errors.Is works correctly on the custom error
				if !errors.Is(customErrImpl, causeErrArg) {
					t.Errorf("errors.Is(customErrImpl, causeErrArg) failed")
				}
			}

			// --- Verify String Output (Exact Match) ---
			gotStr := customErrImpl.Error()
			wantStr := stdErr.Error()
			if gotStr != wantStr {
				t.Errorf("String output mismatch:\n got: %q\nwant: %q", gotStr, wantStr)
			}
		})
	}
}

var errForEdgeCases = errors.New("error")

// TestNewfEdgeCases covers additional Newf scenarios, such as nil interfaces,
// escaped percent signs, and malformed formats.
// Expectations for %w cases are updated for fmt.Errorf compatibility.
func TestNewfEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		args      []interface{}
		wantMsg   string // EXPECTATION UPDATED
		wantCause error
	}{
		// Cases without %w (no change)
		{
			name:    "nil interface arg for %v",
			format:  "test %v",
			args:    []interface{}{interface{}(nil)},
			wantMsg: "test <nil>",
		},
		{
			name:      "malformed format ends with %",
			format:    "test %w %", // This case causes a parse error, not a %w formatting issue
			args:      []interface{}{errForEdgeCases},
			wantMsg:   `errors.Newf: format "test %w %" ends with %`, // Newf's specific error message
			wantCause: nil,
		},

		// Cases with %w (EXPECTATIONS UPDATED)
		{
			name:      "escaped %% with %w",
			format:    "%%prefix %% %w %%suffix",
			args:      []interface{}{errForEdgeCases},
			wantMsg:   "%prefix % error %suffix", // Matches fmt.Errorf output
			wantCause: errForEdgeCases,
		},
		{
			name:      "multiple verbs before %w",
			format:    "%s %d %w",
			args:      []interface{}{"foo", 42, errForEdgeCases},
			wantMsg:   "foo 42 error", // Matches fmt.Errorf output
			wantCause: errForEdgeCases,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.format, tt.args...)
			if err == nil {
				t.Fatalf("Newf returned nil")
			}

			if gotMsg := err.Error(); gotMsg != tt.wantMsg {
				t.Errorf("Newf().Error() = %q, want %q", gotMsg, tt.wantMsg)
			}

			// Cause verification
			gotCause := errors.Unwrap(err)
			if tt.wantCause != nil {
				if !errors.Is(err, tt.wantCause) {
					t.Errorf("errors.Is(err, wantCause) failed.\n  err: [%T: %q]\n  wantCause: [%T: %q]\n  gotCause (Unwrap): [%T: %v]",
						err, err, tt.wantCause, tt.wantCause, gotCause, gotCause)
				}
			} else {
				if gotCause != nil {
					t.Errorf("Newf() cause = [%T: %v], want nil", gotCause, gotCause)
				}
			}
		})
	}
}

// compareWrappedErrorStrings verifies semantic equivalence between custom and
// standard library error messages, normalizing spacing differences.
func compareWrappedErrorStrings(t *testing.T, customStr, stdStr, causeStr string) {
	t.Helper()

	var customPrefix string
	if strings.HasSuffix(customStr, ": "+causeStr) {
		customPrefix = strings.TrimSuffix(customStr, ": "+causeStr)
	} else if customStr == causeStr {
		customPrefix = ""
	} else {
		t.Logf("Unexpected custom error string structure: %q for cause %q", customStr, causeStr)
		customPrefix = customStr
	}

	stdRemainder := strings.Replace(stdStr, causeStr, "", 1)
	normCustomPrefix := strings.TrimSpace(spaceRe.ReplaceAllString(customPrefix, " "))
	normStdRemainder := strings.TrimSpace(spaceRe.ReplaceAllString(stdRemainder, " "))

	if normCustomPrefix != normStdRemainder {
		t.Errorf("Semantic content mismatch (excluding cause):\n custom prefix: %q (from %q)\n std remainder: %q (from %q)",
			normCustomPrefix, customStr,
			normStdRemainder, stdStr)
	}
}

func TestWithVariadic(t *testing.T) {
	t.Run("single key-value", func(t *testing.T) {
		err := New("test").With("key1", "value1")
		if val, ok := err.Context()["key1"]; !ok || val != "value1" {
			t.Errorf("Expected key1=value1, got %v", val)
		}
	})

	t.Run("multiple key-values", func(t *testing.T) {
		err := New("test").With("key1", 1, "key2", 2, "key3", 3)
		ctx := err.Context()
		if ctx["key1"] != 1 || ctx["key2"] != 2 || ctx["key3"] != 3 {
			t.Errorf("Expected all keys to be set, got %v", ctx)
		}
	})

	t.Run("odd number of args", func(t *testing.T) {
		err := New("test").With("key1", 1, "key2")
		ctx := err.Context()
		if ctx["key1"] != 1 || ctx["key2"] != "(MISSING)" {
			t.Errorf("Expected key1=1 and key2=(MISSING), got %v", ctx)
		}
	})

	t.Run("non-string keys", func(t *testing.T) {
		err := New("test").With(123, "value1", true, "value2")
		ctx := err.Context()
		if ctx["123"] != "value1" || ctx["true"] != "value2" {
			t.Errorf("Expected converted keys, got %v", ctx)
		}
	})

	t.Run("transition to map context", func(t *testing.T) {
		// Assuming contextSize is 4
		err := New("test").
			With("k1", 1, "k2", 2, "k3", 3, "k4", 4). // fills smallContext
			With("k5", 5)                             // should trigger map transition

		if err.smallCount != 0 {
			t.Error("Expected smallCount to be 0 after transition")
		}
		if len(err.context) != 5 {
			t.Error("Expected all 5 items in map context")
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		err := New("test")
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			err.With("key1", 1, "key2", 2)
		}()

		go func() {
			defer wg.Done()
			err.With("key3", 3, "key4", 4)
		}()

		wg.Wait()
		ctx := err.Context()
		if len(ctx) != 4 {
			t.Errorf("Expected 4 items in context, got %d", len(ctx))
		}
	})

	t.Run("mixed existing context", func(t *testing.T) {
		err := New("test").
			With("k1", 1).                           // smallContext
			With("k2", 2, "k3", 3, "k4", 4, "k5", 5) // some in small, some in map

		if len(err.context) != 5 {
			t.Errorf("Expected 5 items total, got %d", len(err.context))
		}
	})

	t.Run("large number of pairs", func(t *testing.T) {
		err := New("test")
		args := make([]interface{}, 20)
		for i := 0; i < 10; i++ {
			args[i*2] = i
			args[i*2+1] = i * 10
		}
		err = err.With(args...)

		ctx := err.Context()
		if len(ctx) != 10 {
			t.Errorf("Expected 10 items, got %d", len(ctx))
		}
		if ctx["5"] != 50 {
			t.Errorf("Expected ctx[5]=50, got %v", ctx["5"])
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("basic wrapf", func(t *testing.T) {
		cause := New("cause")
		err := New("wrapper").Wrapf(cause, "formatted %s", "message")

		if err.Unwrap() != cause {
			t.Error("Unwrap() should return the cause")
		}
		if err.Error() != "formatted message: cause" {
			t.Errorf("Expected 'formatted message: cause', got '%s'", err.Error())
		}
	})

	t.Run("nil cause", func(t *testing.T) {
		err := New("wrapper").Wrapf(nil, "format %s", "test")
		if err.Unwrap() != nil {
			t.Error("Unwrap() should return nil for nil cause")
		}
		if err.Error() != "format test" {
			t.Errorf("Expected 'format test', got '%s'", err.Error())
		}
	})

	t.Run("complex formatting", func(t *testing.T) {
		cause := New("cause")
		err := New("wrapper").Wrapf(cause, "value: %d, str: %s", 42, "hello")

		if err.Error() != "value: 42, str: hello: cause" {
			t.Errorf("Expected complex formatting, got '%s'", err.Error())
		}
	})

	t.Run("wrapf with std error", func(t *testing.T) {
		stdErr := errors.New("io error")
		err := New("wrapper").Wrapf(stdErr, "operation failed after %d attempts", 3)

		if err.Unwrap() != stdErr {
			t.Error("Should be able to wrap standard errors with Wrapf")
		}
		if err.Error() != "operation failed after 3 attempts: io error" {
			t.Errorf("Expected formatted message with cause, got '%s'", err.Error())
		}
	})

	t.Run("preserves other fields", func(t *testing.T) {
		cause := New("cause").WithCode(404)
		err := New("wrapper").
			With("key", "value").
			WithCode(500).
			Wrapf(cause, "formatted")

		if err.Code() != 500 {
			t.Error("Wrapf should preserve error code")
		}
		if val, ok := err.Context()["key"]; !ok || val != "value" {
			t.Error("Wrapf should preserve context")
		}
		if err.Unwrap().(*Error).Code() != 404 {
			t.Error("Should preserve cause's code")
		}
	})
}
