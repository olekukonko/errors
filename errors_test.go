package errors

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	err := New("test error")
	if err.Error() != "test error" {
		t.Errorf("New() error message = %v, want %v", err.Error(), "test error")
	}
	if len(err.Stack()) == 0 {
		t.Errorf("New() should capture stack trace")
	}
}

func TestNewf(t *testing.T) {
	err := Newf("test %s %d", "error", 42)
	want := "test error 42"
	if err.Error() != want {
		t.Errorf("Newf() error message = %v, want %v", err.Error(), want)
	}
	if len(err.Stack()) == 0 {
		t.Errorf("Newf() should capture stack trace")
	}
}

func TestNamed(t *testing.T) {
	err := Named("test_name")
	if err.Error() != "test_name" {
		t.Errorf("Named() error message = %v, want %v", err.Error(), "test_name")
	}
	if len(err.Stack()) == 0 {
		t.Errorf("Named() should capture stack trace")
	}
}

func TestErrorMethods(t *testing.T) {
	err := New("base error")

	// Test With
	err = err.With("key", "value")
	if err.Context()["key"] != "value" {
		t.Errorf("With() failed, context[key] = %v, want %v", err.Context()["key"], "value")
	}

	// Test Wrap
	cause := New("cause error")
	err = err.Wrap(cause)
	if err.Unwrap() != cause {
		t.Errorf("Wrap() failed, unwrapped = %v, want %v", err.Unwrap(), cause)
	}

	// Test Msg
	err = err.Msg("new message %d", 123)
	if err.Error() != "new message 123" {
		t.Errorf("Msg() failed, error = %v, want %v", err.Error(), "new message 123")
	}

	// Test Trace (already captured, should not overwrite)
	stackLen := len(err.Stack())
	err = err.Trace()
	if len(err.Stack()) != stackLen {
		t.Errorf("Trace() should not overwrite existing stack, got %d, want %d", len(err.Stack()), stackLen)
	}

	// Test WithCode (no name, so no effect)
	err = err.WithCode(400)
	if err.Code() != 500 {
		t.Errorf("WithCode() on unnamed error should return 500, got %d", err.Code())
	}
}

func TestIs(t *testing.T) {
	err := Named("test_error")
	err2 := Named("test_error")
	err3 := Named("other_error")

	if !err.Is(err2) {
		t.Errorf("Is() failed, %v should match %v", err, err2)
	}
	if err.Is(err3) {
		t.Errorf("Is() failed, %v should not match %v", err, err3)
	}

	// Test wrapping
	cause := Named("cause_error")
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

func TestAs(t *testing.T) {
	err := New("base").Wrap(Named("target"))
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
	var stdTarget error
	if !As(err, &stdTarget) {
		t.Errorf("As() failed, should unwrap to stdlib error")
	}
	if stdTarget != stdErr {
		t.Errorf("As() unwrapped to wrong error, got %v, want %v", stdTarget, stdErr)
	}
}

func TestCount(t *testing.T) {
	err := New("unnamed")
	if err.Count() != 0 {
		t.Errorf("Count() on unnamed error should be 0, got %d", err.Count())
	}

	err = Named("test_count")
	if err.Count() != 0 {
		t.Errorf("Count() on new named error should be 0, got %d", err.Count())
	}
}

func TestCode(t *testing.T) {
	err := New("unnamed")
	if err.Code() != 500 {
		t.Errorf("Code() on unnamed error should be 500, got %d", err.Code())
	}

	err = Named("test_code").WithCode(400)
	if err.Code() != 400 {
		t.Errorf("Code() after WithCode(400) should be 400, got %d", err.Code())
	}
}

func TestMarshalJSON(t *testing.T) {
	err := New("test").With("key", "value").Wrap(Named("cause"))
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
	if err.Is(Named("")) {
		t.Errorf("Error with empty name should not match unnamed error")
	}

	// Test stdlib error wrapping
	stdErr := errors.New("std error")
	customErr := New("custom").Wrap(stdErr)
	if !errors.Is(customErr, stdErr) {
		t.Errorf("errors.Is() should match stdlib error through wrapping")
	}

	// Test As with nil target
	var nilTarget *Error
	if As(nilErr, &nilTarget) {
		t.Errorf("As(nil, &nilTarget) should return false")
	}
}
