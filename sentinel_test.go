package errors

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestConstCreatesUniquePointers(t *testing.T) {
	a := Const("not_found", "resource not found")
	b := Const("not_found", "resource not found")
	if a == b {
		t.Error("Const() should return distinct pointers on each call")
	}
}

func TestConstIsComparable(t *testing.T) {
	ErrNotFound := Const("not_found", "resource not found")
	if !Is(ErrNotFound, ErrNotFound) {
		t.Error("Is(sentinel, sentinel) should be true")
	}
	wrapped := New("request failed").Wrap(ErrNotFound)
	if !Is(wrapped, ErrNotFound) {
		t.Error("Is should find sentinel through a wrapped *Error chain")
	}
}

func TestConstDoesNotMatchDifferentSentinel(t *testing.T) {
	ErrA := Const("a", "error a")
	ErrB := Const("b", "error b")
	if Is(ErrA, ErrB) {
		t.Error("two distinct sentinels should not match each other")
	}
}

func TestConstError(t *testing.T) {
	s := Const("validation_failed", "input is invalid")
	if s.Error() != "input is invalid" {
		t.Errorf("Error() = %q, want %q", s.Error(), "input is invalid")
	}
}

func TestConstName(t *testing.T) {
	s := Const("my_error", "something happened")
	if s.Name() != "my_error" {
		t.Errorf("Name() = %q, want %q", s.Name(), "my_error")
	}
}

func TestConstDoesNotMatchPlainError(t *testing.T) {
	s := Const("sentinel", "sentinel error")
	other := New("different message")
	if Is(s, other) {
		t.Error("sentinel should not match a *Error with a different message")
	}
	// Note: Is(sentinel, target) uses pointer equality so is always false
	// for non-identical sentinels regardless of message content.
}

func TestConstImplementsError(t *testing.T) {
	var _ error = Const("x", "y")
}

// Unwrap

func TestSentinelUnwrap(t *testing.T) {
	s := Const("root", "root cause")
	if s.Unwrap() != nil {
		t.Error("Sentinel.Unwrap() should return nil — sentinels are root errors")
	}
}

// As

func TestSentinelAs(t *testing.T) {
	ErrNotFound := Const("not_found", "resource not found")
	wrapped := New("handler failed").Wrap(ErrNotFound)

	var target *Sentinel
	if !As(wrapped, &target) {
		t.Fatal("As() should find the Sentinel in the cause chain")
	}
	if target != ErrNotFound {
		t.Error("As() should set target to the exact sentinel pointer")
	}
}

func TestSentinelAsWrongType(t *testing.T) {
	s := Const("x", "x")
	var target *Error
	if As(s, &target) {
		t.Error("As() should return false when target type does not match")
	}
}

// String

func TestSentinelString(t *testing.T) {
	s := Const("not_found", "resource not found")
	got := s.String()
	if !strings.Contains(got, "not_found") || !strings.Contains(got, "resource not found") {
		t.Errorf("String() = %q — expected name and message", got)
	}
}

// LogValue

func TestSentinelLogValue(t *testing.T) {
	s := Const("auth_error", "authentication failed")
	val := s.LogValue()
	if val.Kind() != slog.KindGroup {
		t.Errorf("LogValue() kind = %v, want Group", val.Kind())
	}
	attrs := val.Group()
	keys := make(map[string]string, len(attrs))
	for _, a := range attrs {
		keys[a.Key] = a.Value.String()
	}
	if keys["error"] != "authentication failed" {
		t.Errorf("LogValue error attr = %q, want %q", keys["error"], "authentication failed")
	}
	if keys["code"] != "auth_error" {
		t.Errorf("LogValue code attr = %q, want %q", keys["code"], "auth_error")
	}
}

// MarshalJSON

func TestSentinelMarshalJSON(t *testing.T) {
	s := Const("not_found", "resource not found")
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	var out struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if out.Error != "resource not found" {
		t.Errorf("JSON error = %q, want %q", out.Error, "resource not found")
	}
	if out.Code != "not_found" {
		t.Errorf("JSON code = %q, want %q", out.Code, "not_found")
	}
}

// With

func TestSentinelWith(t *testing.T) {
	ErrNotFound := Const("not_found", "resource not found")
	err := ErrNotFound.With("user 42 not found")

	// The returned *Error should carry the call-site message.
	if err.Error() != "user 42 not found: resource not found" &&
		!strings.Contains(err.Error(), "user 42 not found") {
		t.Errorf("With() message = %q, want it to contain call-site context", err.Error())
	}
	// The original sentinel must still be findable via Is.
	if !Is(err, ErrNotFound) {
		t.Error("Is(With(...), sentinel) should be true — sentinel is the cause")
	}
}

func TestSentinelWithPreservesChain(t *testing.T) {
	ErrForbidden := Const("forbidden", "access denied")
	err := ErrForbidden.With("route /admin requires admin role")

	var s *Sentinel
	if !As(err, &s) {
		t.Fatal("As() should find Sentinel through With() chain")
	}
	if s != ErrForbidden {
		t.Error("As() should return the original sentinel pointer")
	}
}
