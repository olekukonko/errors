package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	Configure(Config{
		StackDepth:      32,
		ContextSize:     2,
		DisableStack:    false,
		DisableRegistry: false,
		DisablePooling:  false,
		FilterInternal:  true,
	})
	WarmPool(10)
	WarmStackPool(10)
	m.Run()
}

func TestDefine(t *testing.T) {
	CountReset("test_tmpl") // Reset before test
	tmpl := Define("test_tmpl", "test error: %s")
	err := tmpl("detail")
	defer err.Free()
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
	CountReset("test_call")
	fn := Callable("test_call", func(args ...interface{}) *Error {
		return Named("test_call").Msgf("called with %v", args[0])
	})
	err := fn("arg1")
	defer err.Free()
	if err.Error() != "called with arg1" {
		t.Errorf("Callable() error = %v, want %v", err.Error(), "called with arg1")
	}
	if err.Count() != 1 {
		t.Errorf("Callable() count = %d, want 1", err.Count())
	}
}

func TestCoded(t *testing.T) {
	CountReset("test_coded")
	tmpl := Coded("test_coded", 400, "coded error: %s")
	err := tmpl("reason")
	defer err.Free()
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
	name := "github.com/olekukonko/errors.testFunc"
	CountReset(name)
	err := Func(testFunc, "func error")
	defer err.Free()
	wantName := name
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
	name := "test_monitor"
	CountReset(name)
	SetThreshold(name, 2)
	m := NewMonitor(name)
	defer m.Close()
	done := make(chan bool)

	go func() {
		count := 0
		for err := range m.Chan() {
			defer err.Free()
			count++
			ctxCount, ok := err.Context()["count"].(uint64)
			if !ok || ctxCount < 2 {
				t.Errorf("Monitor() received error with count %v, want >= 2", ctxCount)
			}
			if count == 1 {
				done <- true
				return
			}
		}
	}()

	tmpl := Define(name, "monitor test: %d")
	for i := 0; i < 3; i++ {
		err := tmpl(i)
		err.Free()
		time.Sleep(50 * time.Millisecond)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Errorf("Monitor() failed to trigger alert within timeout")
	}
}

func TestLastError(t *testing.T) {
	name := "test_last"
	CountReset(name)
	tmpl := Define(name, "last error: %s")
	err1 := tmpl("first")
	err1.Free()
	err2 := tmpl("second")
	defer err2.Free()

	last := GetLastError(name)
	if last != err2 {
		t.Errorf("GetLastError() = %v, want %v", last, err2)
	}
	if last.Error() != "last error: second" {
		t.Errorf("GetLastError() error = %v, want %v", last.Error(), "last error: second")
	}
}

func TestIncrementCount(t *testing.T) {
	name := "test_count"
	CountReset(name)
	tmpl := Define(name, "count test")
	for i := 0; i < 5; i++ {
		err := tmpl("test")
		err.Free()
	}
	err := tmpl("final")
	defer err.Free()
	if err.Count() != 6 {
		t.Errorf("IncrementCount() failed, count = %d, want 6", err.Count())
	}
}

func TestMetrics(t *testing.T) {
	ResetRegistry() // Clear all previous counts
	CountReset("metric1")
	CountReset("metric2")
	tmpl1 := Define("metric1", "metric one: %s")
	tmpl2 := Define("metric2", "metric two: %s")

	for i := 0; i < 3; i++ {
		err := tmpl1(fmt.Sprintf("test%d", i))
		err.Free()
	}
	for i := 0; i < 2; i++ {
		err := tmpl2(fmt.Sprintf("test%d", i))
		err.Free()
	}

	metrics := Metrics()
	if len(metrics) != 2 {
		t.Errorf("Metrics() len = %d, want 2", len(metrics))
	}
	if metrics["metric1"] != 3 {
		t.Errorf("Metrics()[metric1] = %d, want 3", metrics["metric1"])
	}
	if metrics["metric2"] != 2 {
		t.Errorf("Metrics()[metric2] = %d, want 2", metrics["metric2"])
	}
}

func TestCountReset(t *testing.T) {
	name := "test_reset"
	CountReset(name)
	tmpl := Define(name, "reset test")

	for i := 0; i < 5; i++ {
		err := tmpl("test")
		err.Free()
	}

	err := tmpl("before reset")
	defer err.Free()
	if err.Count() != 6 {
		t.Errorf("Count before reset = %d, want 6", err.Count())
	}

	CountReset(name)
	err2 := tmpl("after reset")
	defer err2.Free()
	if err2.Count() != 1 {
		t.Errorf("Count after reset = %d, want 1", err2.Count())
	}
	if GetLastError(name) != err2 {
		t.Errorf("GetLastError() after reset = %v, want %v", GetLastError(name), err2)
	}
}

func TestRegistryEdgeCases(t *testing.T) {
	// Test Define with empty name
	CountReset("")
	tmpl := Define("", "unnamed: %s")
	err := tmpl("test")
	defer err.Free()
	if err.name != "" {
		t.Errorf("Define() with empty name should have empty name, got %v", err.name)
	}
	if err.Count() != 1 {
		t.Errorf("Count() for empty name should be 1, got %d", err.Count())
	}

	// Test Callable with stdlib error
	CountReset("std_call")
	stdErr := errors.New("std error")
	fn := Callable("std_call", func(args ...interface{}) *Error {
		return New("wrapper").Wrap(stdErr)
	})
	err = fn()
	defer err.Free()
	if !Is(err, stdErr) {
		t.Errorf("Callable() with stdlib error should be compatible with Is")
	}

	// Test nil function in Func
	CountReset("unknown")
	err = Func(nil, "nil func")
	defer err.Free()
	if err.name != "unknown" {
		t.Errorf("Func(nil) should set name to 'unknown', got %v", err.name)
	}

	// Test Monitor with no threshold
	name := "no_threshold"
	CountReset(name)
	m := NewMonitor(name)
	defer m.Close()
	select {
	case <-m.Chan():
		t.Errorf("Monitor() with no threshold should not send alerts")
	case <-time.After(10 * time.Millisecond):
		// Expected: no alert
	}

	// Test Metrics with disabled registry
	Configure(Config{DisableRegistry: true})
	defer Configure(Config{DisableRegistry: false}) // Restore default
	CountReset("disabled_test")
	tmpl = Define("disabled_test", "disabled: %s")
	err = tmpl("test")
	defer err.Free()
	if err.Count() != 0 {
		t.Errorf("Count() with disabled registry should be 0, got %d", err.Count())
	}
	if metrics := Metrics(); metrics != nil {
		t.Errorf("Metrics() with disabled registry should be nil, got %v", metrics)
	}
}

func testFunc() {}
