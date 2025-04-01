package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDefine(t *testing.T) {
	tmpl := Define("test_tmpl", "test error: %s")
	err := tmpl("detail")
	if err.Error() != "test error: detail" {
		t.Errorf("Define() error = %v, want %v", err.Error(), "test error: detail")
	}
	if err.name != "test_tmpl" {
		t.Errorf("Define() name = %v, want %v", err.name, "test_tmpl")
	}
	if err.Count() != 1 {
		t.Errorf("Define() count = %d, want 1", err.Count())
	}
}

func TestCallable(t *testing.T) {
	fn := Callable("test_call", func(args ...interface{}) *Error {
		return Named("test_call").Msg("called with %v", args[0])
	})
	err := fn("arg1")
	if err.Error() != "called with arg1" {
		t.Errorf("Callable() error = %v, want %v", err.Error(), "called with arg1")
	}
	if err.Count() != 1 {
		t.Errorf("Callable() count = %d propriétéwant 1", err.Count())
	}
}

func TestCoded(t *testing.T) {
	tmpl := Coded("test_coded", 400, "coded error: %s")
	err := tmpl("reason")
	if err.Error() != "coded error: reason" {
		t.Errorf("Coded() error = %v, want %v", err.Error(), "coded error: reason")
	}
	if err.Code() != 400 {
		t.Errorf("Coded() code = %d, want 400", err.Code())
	}
	if err.Count() != 1 {
		t.Errorf("Coded() count = %d, want 1", err.Count())
	}
}

func TestFunc(t *testing.T) {
	err := Func(testFunc, "func error")
	wantName := "github.com/olekukonko/errors.testFunc"
	if err.name != wantName {
		t.Errorf("Func() name = %v, want %v", err.name, wantName)
	}
	if err.Error() != fmt.Sprintf("%s: func error", wantName) {
		t.Errorf("Func() error = %v, want %v", err.Error(), fmt.Sprintf("%s: func error", wantName))
	}
	if err.Count() != 1 {
		t.Errorf("Func() count = %d, want 1", err.Count())
	}
}

func TestThresholdAndMonitor(t *testing.T) {
	SetThreshold("test_monitor", 2)
	ch := Monitor("test_monitor")
	done := make(chan bool)

	go func() {
		count := 0
		for err := range ch {
			count++
			if err.Count() < 2 {
				t.Errorf("Monitor() received error with count %d, want >= 2", err.Count())
			}
			if count == 2 {
				done <- true
				return
			}
		}
	}()

	tmpl := Define("test_monitor", "monitor test: %d")
	for i := 0; i < 3; i++ {
		_ = tmpl(i)
		time.Sleep(10 * time.Millisecond) // Allow goroutine to process
	}

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Errorf("Monitor() failed to trigger alerts within timeout")
	}
}

func TestLastError(t *testing.T) {
	tmpl := Define("test_last", "last error: %s")
	_ = tmpl("first")
	err2 := tmpl("second")

	last := GetLastError("test_last")
	if last != err2 {
		t.Errorf("GetLastError() = %v, want %v", last, err2)
	}
	if last.Error() != "last error: second" {
		t.Errorf("GetLastError() error = %v, want %v", last.Error(), "last error: second")
	}
}

func TestIncrementCount(t *testing.T) {
	tmpl := Define("test_count", "count test")
	for i := 0; i < 5; i++ {
		tmpl("test")
	}
	err := tmpl("final")
	if err.Count() != 6 {
		t.Errorf("IncrementCount() failed, count = %d, want 6", err.Count())
	}
}

func TestRegistryEdgeCases(t *testing.T) {
	// Test Define with empty name
	tmpl := Define("", "unnamed: %s")
	err := tmpl("test")
	if err.name != "" {
		t.Errorf("Define() with empty name should have empty name, got %v", err.name)
	}
	if err.Count() != 0 {
		t.Errorf("Count() for empty name should be 0, got %d", err.Count())
	}

	// Test Callable with stdlib error
	stdErr := errors.New("std error")
	fn := Callable("std_call", func(args ...interface{}) *Error {
		return New("wrapper").Wrap(stdErr)
	})
	err = fn()
	if !Is(err, stdErr) {
		t.Errorf("Callable() with stdlib error should be compatible with Is")
	}

	// Test nil function in Func
	err = Func(nil, "nil func")
	if err.name != "unknown" {
		t.Errorf("Func(nil) should set name to 'unknown', got %v", err.name)
	}

	// Test Monitor with no threshold
	ch := Monitor("no_threshold")
	select {
	case <-ch:
		t.Errorf("Monitor() with no threshold should not send alerts")
	case <-time.After(10 * time.Millisecond):
		// Expected: no alert
	}
}

func testFunc() {}
