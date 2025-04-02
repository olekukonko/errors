package errors

import (
	"encoding/json"
	"errors"
	"reflect"
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
	if err.Error() != "new message 123" {
		t.Errorf("Msgf() failed, error = %v, want %v", err.Error(), "new message 123")
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
	if GetCategory(err) != "test_category" {
		t.Errorf("WithCategory() failed, category = %v, want %v", GetCategory(err), "test_category")
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

	// Test wrapping
	cause := Named("cause_error")
	defer cause.Free()
	err = err.Wrap(cause)
	if !Is(err, cause) {
		t.Errorf("Is() failed, wrapped error should match cause")
	}

	// Test with stdlib error
	stdErr := errors.New("std error")
	err = err.Wrap(stdErr)
	if !Is(err, stdErr) {
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
