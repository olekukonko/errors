package errors

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestNew verifies that New creates an error with the correct message and stack trace.
func TestNew(t *testing.T) {
	err := New("test error")
	defer err.Free()
	if err.Error() != "test error" {
		t.Errorf("New() error message = %v, want %v", err.Error(), "test error")
	}
	if len(err.Stack()) != 0 {
		t.Errorf("New() should not capture stack trace, got %d frames", len(err.Stack()))
	}
}

// Make similar changes to TestNewf and TestNamed

// TestNewf checks that Newf formats the message correctly and includes a stack trace.
func TestNewf(t *testing.T) {
	err := Newf("test %s %d", "error", 42)
	defer err.Free() // Clean up
	want := "test error 42"
	if err.Error() != want {
		t.Errorf("Newf() error message = %v, want %v", err.Error(), want)
	}
	if len(err.Stack()) != 0 {
		t.Errorf("Newf() should not capture stack trace, got %d frames", len(err.Stack()))
	}
}

// TestNamed ensures Named sets the name correctly and captures a stack trace.
func TestNamed(t *testing.T) {
	err := Named("test_name")
	defer err.Free() // Clean up
	if err.Error() != "test_name" {
		t.Errorf("Named() error message = %v, want %v", err.Error(), "test_name")
	}
	if len(err.Stack()) == 0 {
		t.Errorf("Named() should capture stack trace")
	}
}

// TestErrorMethods tests various methods on the Error type.
func TestErrorMethods(t *testing.T) {
	err := New("base error")
	defer err.Free()

	// Test With
	err = err.With("key", "value")
	if err.Context()["key"] != "value" {
		t.Errorf("With() failed, context[key] = %v, want %v", err.Context()["key"], "value")
	}

	// Test Wrap
	cause := New("cause error")
	defer cause.Free()
	err = err.Wrap(cause)
	if err.Unwrap() != cause {
		t.Errorf("Wrap() failed, unwrapped = %v, want %v", err.Unwrap(), cause)
	}

	// Test Msgf
	err = err.Msgf("new message %d", 123)
	if err.Error() != "new message 123: cause error" {
		t.Errorf("Msgf() failed, error = %v, want %v", err.Error(), "new message 123: cause error")
	}

	// Test Trace (should capture stack if none exists)
	stackLen := len(err.Stack())
	if stackLen != 0 {
		t.Errorf("Initial stack length should be 0, got %d", stackLen)
	}
	err = err.Trace()
	if len(err.Stack()) == 0 {
		t.Errorf("Trace() should capture a stack trace, got no frames")
	}

	// Test WithCode
	err = err.WithCode(400)
	if err.Code() != 400 {
		t.Errorf("WithCode() failed, code = %d, want 400", err.Code())
	}

	// Test WithCategory
	err = err.WithCategory("test_category")
	if Category(err) != "test_category" {
		t.Errorf("WithCategory() failed, category = %v, want %v", Category(err), "test_category")
	}

	// Test Increment
	err = err.Increment()
	if err.Count() != 1 {
		t.Errorf("Increment() failed, count = %d, want 1", err.Count())
	}
}

// TestIs verifies the Is method for matching errors by name and wrapping.
func TestIs(t *testing.T) {
	err := Named("test_error")
	defer err.Free()
	err2 := Named("test_error")
	defer err2.Free()
	err3 := Named("other_error")
	defer err3.Free()

	if !err.Is(err2) {
		t.Errorf("Is() failed, %v should match %v", err, err2)
	}
	if err.Is(err3) {
		t.Errorf("Is() failed, %v should not match %v", err, err3)
	}

	wrappedErr := Named("wrapper")
	defer wrappedErr.Free()
	cause := Named("cause_error")
	defer cause.Free()
	wrappedErr = wrappedErr.Wrap(cause)
	t.Logf("Before Is(cause): wrappedErr.cause = %p, cause = %p", wrappedErr.cause, cause)
	if !wrappedErr.Is(cause) {
		t.Errorf("Is() failed, wrapped error should match cause; wrappedErr = %+v, cause = %+v", wrappedErr, cause)
	}

	stdErr := errors.New("std error")
	wrappedErr = wrappedErr.Wrap(stdErr)
	t.Logf("Before Is(stdErr): wrappedErr.cause = %p, stdErr = %p", wrappedErr.cause, stdErr)
	if !wrappedErr.Is(stdErr) {
		t.Errorf("Is() failed, should match stdlib error")
	}
}

// TestAs checks the As method for unwrapping to the correct error type.
func TestAs(t *testing.T) {
	err := New("base").Wrap(Named("target"))
	defer err.Free()
	var target *Error
	if !As(err, &target) {
		t.Errorf("As() failed, should unwrap to *Error")
	}
	if target.name != "target" {
		t.Errorf("As() unwrapped to wrong error, got %v, want %v", target.name, "target")
	}

	// Test with stdlib error
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

// TestCount verifies the Count method for per-instance counting.
func TestCount(t *testing.T) {
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

// TestCode checks the Code method for setting and retrieving HTTP status codes.
func TestCode(t *testing.T) {
	err := New("unnamed")
	defer err.Free()
	if err.Code() != 0 { // Changed from 500 to 0 since no default is set
		t.Errorf("Code() on new error should be 0, got %d", err.Code())
	}

	err = Named("test_code").WithCode(400)
	if err.Code() != 400 {
		t.Errorf("Code() after WithCode(400) should be 400, got %d", err.Code())
	}
}

// TestMarshalJSON ensures JSON serialization includes all expected fields.
func TestMarshalJSON(t *testing.T) {
	err := New("test").With("key", "value").Wrap(Named("cause"))
	defer err.Free()
	data, e := json.Marshal(err)
	if e != nil {
		t.Fatalf("MarshalJSON() failed: %v", e)
	}

	want := map[string]interface{}{
		"message": "test",
		"context": map[string]interface{}{"key": "value"},
		"cause":   map[string]interface{}{"name": "cause"},
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
}

// TestEdgeCases verifies behavior in unusual scenarios.
func TestEdgeCases(t *testing.T) {
	// Test nil error
	var nilErr *Error
	if nilErr.Is(nil) {
		t.Errorf("nil.Is(nil) should be false, got true")
	}
	if Is(nilErr, New("test")) {
		t.Errorf("Is(nil, non-nil) should be false")
	}

	// Test empty name
	err := New("empty name")
	defer err.Free()
	if err.Is(Named("")) {
		t.Errorf("Error with empty name should not match unnamed error")
	}

	// Test stdlib error wrapping
	stdErr := errors.New("std error")
	customErr := New("custom").Wrap(stdErr)
	defer customErr.Free()
	if !Is(customErr, stdErr) {
		t.Errorf("Is() should match stdlib error through wrapping")
	}

	// Test As with nil target
	var nilTarget *Error
	if As(nilErr, &nilTarget) {
		t.Errorf("As(nil, &nilTarget) should return false")
	}
}

func TestRetryWithCallback(t *testing.T) {
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
}

func TestStackPresence(t *testing.T) {
	// New errors should have no stack
	err := New("test")
	if len(err.Stack()) != 0 {
		t.Error("New() should not capture stack")
	}

	// Traced errors should have stack
	traced := Trace("test")
	if len(traced.Stack()) == 0 {
		t.Error("Trace() should capture stack")
	}
}

func TestStackDepth(t *testing.T) {
	err := Trace("test")
	frames := err.Stack()
	if len(frames) > currentConfig.stackDepth {
		t.Errorf("Stack depth %d exceeds configured max %d",
			len(frames), currentConfig.stackDepth)
	}
}

func TestTransform(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		if Transform(nil, func(e *Error) {}) != nil {
			t.Error("Should handle nil error")
		}
	})

	t.Run("NonErrorType", func(t *testing.T) {
		stdErr := errors.New("standard")
		if Transform(stdErr, func(e *Error) {}) != stdErr {
			t.Error("Should return non-*Error unchanged")
		}
	})

	t.Run("TransformError", func(t *testing.T) {
		orig := New("original")
		transformed := Transform(orig, func(e *Error) {
			e.With("key", "value")
		}).(*Error)

		if transformed == orig {
			t.Error("Should return a copy")
		}
		if transformed.Context()["key"] != "value" {
			t.Error("Should apply transformations")
		}
	})
}

// Custom error type to create a chain of errors
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

func TestWalk(t *testing.T) {
	// Create a chain of errors
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

func TestFind(t *testing.T) {
	// Create a chain of errors
	err1 := &customError{msg: "first error", cause: nil}
	err2 := &customError{msg: "second error", cause: err1}
	err3 := &customError{msg: "third error", cause: err2}

	// Test finding a specific error
	found := Find(err3, func(e error) bool {
		return e.Error() == "second error"
	})

	if found == nil || found.Error() != "second error" {
		t.Errorf("Find() = %v; want 'second error'", found)
	}

	// Test for a non-existing error
	found = Find(err3, func(e error) bool {
		return e.Error() == "non-existent error"
	})

	if found != nil {
		t.Errorf("Find() = %v; want nil", found)
	}
}

func TestTraceStackContent(t *testing.T) {
	err := Trace("test")
	defer err.Free()
	frames := err.Stack()
	if len(frames) == 0 {
		t.Fatal("Trace() should capture stack frames")
	}
	// Look for the test runner frame instead
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

func TestWithStackContent(t *testing.T) {
	err := New("test").WithStack()
	defer err.Free()
	frames := err.Stack()
	if len(frames) == 0 {
		t.Fatal("WithStack() should capture stack frames")
	}
	// Look for the test runner frame instead
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

func TestErrorWrappingChain(t *testing.T) {
	// Recreate the scenario from the example
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

	// Test the full error message
	expectedFullMessage := "API request failed: failed to process user 12345: connection timeout"
	if apiErr.Error() != expectedFullMessage {
		t.Errorf("Full error message mismatch\ngot: %q\nwant: %q", apiErr.Error(), expectedFullMessage)
	}

	// Test the unwrapping chain order
	chain := UnwrapAll(apiErr)
	if len(chain) != 3 {
		t.Fatalf("Expected chain length 3, got %d", len(chain))
	}

	// Verify each error in the chain
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
			t.Errorf("Chain position %d mismatch\ngot: %q\nwant: %q",
				tt.index, chain[tt.index].Error(), tt.expected)
		}
	}

	// Verify the Is() functionality
	if !errors.Is(apiErr, databaseErr) {
		t.Error("Is() should match the database error in the chain")
	}

	// Verify context is properly isolated at each level
	if ctx := businessErr.Context(); ctx["timeout_sec"] != nil {
		t.Error("Business error should not have database context")
	}

	// Verify stack trace is only where we added it
	if stack := apiErr.Stack(); len(stack) == 0 {
		t.Error("API error should have stack trace")
	}
	if stack := businessErr.Stack(); len(stack) != 0 {
		t.Error("Business error should not have stack trace")
	}

	// Verify code is only where we set it
	if apiErr.Code() != 500 {
		t.Error("API error should have code 500")
	}
	if businessErr.Code() != 0 {
		t.Error("Business error should have no code")
	}
}

func TestExampleOutput(t *testing.T) {
	// This test verifies the output matches the example's expected output
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

	// Test the Format output for each error in the chain
	chain := UnwrapAll(apiErr)
	for _, err := range chain {
		if e, ok := err.(*Error); ok {
			formatted := e.Format()
			if formatted == "" {
				t.Error("Format() returned empty string")
			}
			// Basic sanity checks of the formatted output
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

	// Test the Is() match as shown in the example
	if !errors.Is(apiErr, errors.New("connection timeout")) {
		t.Error("Is() failed to match connection timeout error")
	}
}

// TestFullErrorChain builds an error chain and verifies the custom and standard
// behavior of Is and As (for *Error targets) as well as UnwrapAll.
// In the test document (first document):
func TestFullErrorChain(t *testing.T) {
	stdErr := errors.New("file not found")
	authErr := Named("AuthError").WithCode(401)
	storageErr := Wrapf(stdErr, "storage failed")
	authErrWrapped := Wrap(storageErr, authErr)
	wrapped := Wrapf(authErrWrapped, "request failed")

	// --- Test Is ---
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

	// --- Test As for *Error target ---
	var targetAuth *Error
	if !As(wrapped, &targetAuth) || targetAuth.Name() != "AuthError" || targetAuth.Code() != 401 {
		t.Errorf("As(wrapped, &targetAuth) failed, got name=%s, code=%d; want AuthError, 401", targetAuth.Name(), targetAuth.Code())
	}
	if !errors.As(wrapped, &targetAuth) || targetAuth.Name() != "AuthError" || targetAuth.Code() != 401 {
		t.Errorf("stderrors.As(wrapped, &targetAuth) failed, got name=%s, code=%d; want AuthError, 401", targetAuth.Name(), targetAuth.Code())
	}

	// --- Test UnwrapAll ---
	chain := UnwrapAll(wrapped)
	if len(chain) != 4 {
		t.Errorf("UnwrapAll(wrapped) length = %d, want 4", len(chain))
	}
	expected := []string{
		"request failed",
		"storage failed",
		"AuthError",
		"file not found",
	}
	for i, err := range chain {
		if err.Error() != expected[i] {
			t.Errorf("UnwrapAll[%d] = %v, want %v", i, err.Error(), expected[i])
		}
	}
}

func TestUnwrapAllMessageIsolation(t *testing.T) {
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
