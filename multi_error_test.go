package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// TestMultiError_Basic verifies basic MultiError functionality.
// Ensures empty creation, nil error handling, and single error addition work as expected.
func TestMultiError_Basic(t *testing.T) {
	m := NewMultiError()
	if m.Has() {
		t.Error("New MultiError should be empty")
	}

	m.Add(nil) // Single nil error
	if m.Has() {
		t.Error("Adding nil should not create error")
	}

	err1 := errors.New("error 1")
	m.Add(err1) // Single error
	if !m.Has() {
		t.Error("Should detect errors after adding one")
	}
	if m.Count() != 1 {
		t.Errorf("Count should be 1, got %d", m.Count())
	}
	if m.First() != err1 || m.Last() != err1 {
		t.Errorf("First() and Last() should both be %v, got First=%v, Last=%v", err1, m.First(), m.Last())
	}

	// Test variadic Add with nil and duplicate
	m.Add(nil, err1, errors.New("error 1")) // Nil, duplicate, and same message
	if m.Count() != 1 {
		t.Errorf("Count should remain 1 after adding nil and duplicate, got %d", m.Count())
	}
}

// TestMultiError_Sampling tests the sampling behavior of MultiError.
// Adds many unique errors with a 50% sampling rate and checks the resulting ratio is within 45-55%.
func TestMultiError_Sampling(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // Fixed seed for reproducible results
	m := NewMultiError(WithSampling(50), WithRand(r))
	total := 1000

	// Add errors in batches to test variadic Add
	batchSize := 100
	for i := 0; i < total; i += batchSize {
		batch := make([]error, batchSize)
		for j := 0; j < batchSize; j++ {
			batch[j] = errors.New(fmt.Sprintf("test%d", i+j)) // Unique errors
		}
		m.Add(batch...)
	}

	count := m.Count()
	ratio := float64(count) / float64(total)
	// Expect roughly 50% (±5%) due to sampling; adjust range if sampling logic changes
	if ratio < 0.45 || ratio > 0.55 {
		t.Errorf("Sampling ratio %v not within expected range (45-55%%), count=%d, total=%d", ratio, count, total)
	}
}

// TestMultiError_Limit tests the error limit enforcement of MultiError.
// Adds twice the limit of unique errors and verifies the count caps at the limit.
func TestMultiError_Limit(t *testing.T) {
	limit := 10
	m := NewMultiError(WithLimit(limit))

	// Add errors in a single variadic call
	errors := make([]error, limit*2)
	for i := 0; i < limit*2; i++ {
		errors[i] = New(fmt.Sprintf("test%d", i)) // Unique errors
	}
	m.Add(errors...)

	if m.Count() != limit {
		t.Errorf("Should cap at %d errors, got %d", limit, m.Count())
	}
}

// TestMultiError_Formatting verifies custom formatting in MultiError.
// Adds two errors and checks the custom formatter outputs the expected string.
func TestMultiError_Formatting(t *testing.T) {
	customFormat := func(errs []error) string {
		return fmt.Sprintf("custom: %d", len(errs))
	}

	m := NewMultiError(WithFormatter(customFormat))
	m.Add(errors.New("test1"), errors.New("test2")) // Add two errors at once

	expected := "custom: 2"
	if m.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, m.Error())
	}
}

// TestMultiError_Filter tests the filtering functionality of MultiError.
// Adds three errors, filters out one, and verifies the resulting count is correct.
func TestMultiError_Filter(t *testing.T) {
	m := NewMultiError()
	m.Add(errors.New("error1"), errors.New("skip"), errors.New("error2")) // Variadic add

	filtered := m.Filter(func(err error) bool {
		return err.Error() != "skip"
	})

	if filtered.Count() != 2 {
		t.Errorf("Should filter out one error, leaving 2, got %d", filtered.Count())
	}
}

// TestMultiError_AsSingle tests the Single() method across different scenarios.
// Verifies behavior for empty, single-error, and multi-error cases.
func TestMultiError_AsSingle(t *testing.T) {
	// Subtest: Empty MultiError should return nil
	t.Run("Empty", func(t *testing.T) {
		m := NewMultiError()
		if m.Single() != nil {
			t.Errorf("Empty should return nil, got %v", m.Single())
		}
	})

	// Subtest: Single error should return that error
	t.Run("Single", func(t *testing.T) {
		m := NewMultiError()
		err := errors.New("test")
		m.Add(err)
		if m.Single() != err {
			t.Errorf("Should return single error %v, got %v", err, m.Single())
		}
	})

	// Subtest: Multiple errors should return the MultiError itself
	t.Run("Multiple", func(t *testing.T) {
		m := NewMultiError()
		m.Add(errors.New("test1"), errors.New("test2")) // Variadic add
		if m.Single() != m {
			t.Errorf("Should return self for multiple errors, got %v", m.Single())
		}
	})
}

// TestMultiError_MarshalJSON tests the JSON serialization of MultiError.
// Verifies correct output for empty, single-error, multiple-error, and mixed-error cases.
func TestMultiError_MarshalJSON(t *testing.T) {
	// Subtest: Empty
	t.Run("Empty", func(t *testing.T) {
		m := NewMultiError()
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		expected := `{"count":0,"errors":[]}`
		if string(data) != expected {
			t.Errorf("Expected %q, got %q", expected, string(data))
		}
	})

	// Subtest: Single standard error
	t.Run("SingleStandardError", func(t *testing.T) {
		m := NewMultiError()
		err := errors.New("timeout")
		m.Add(err)

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		expected := `{"count":1,"errors":[{"error":"timeout"}]}`
		var expectedJSON, actualJSON interface{}
		if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
			t.Fatalf("Failed to parse expected JSON: %v", err)
		}
		if err := json.Unmarshal(data, &actualJSON); err != nil {
			t.Fatalf("Failed to parse actual JSON: %v", err)
		}

		if !reflect.DeepEqual(expectedJSON, actualJSON) {
			t.Errorf("JSON output mismatch.\nGot: %s\nWant: %s", string(data), expected)
		}
	})

	// Subtest: Multiple errors including *Error
	t.Run("MultipleMixedErrors", func(t *testing.T) {
		m := NewMultiError(WithLimit(5)) // No sampling to ensure all errors are added
		m.Add(
			New("db error").WithCode(500).With("user_id", 123), // *Error
			errors.New("timeout"),                              // Standard error
			nil,                                                // Nil error (skipped by Add)
		)

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		expected := `{
			"count":2,
			"limit":5,
			"errors":[
				{"error":{"message":"db error","context":{"user_id":123},"code":500}},
				{"error":"timeout"}
			]
		}`
		var expectedJSON, actualJSON interface{}
		if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
			t.Fatalf("Failed to parse expected JSON: %v", err)
		}
		if err := json.Unmarshal(data, &actualJSON); err != nil {
			t.Fatalf("Failed to parse actual JSON: %v", err)
		}

		if !reflect.DeepEqual(expectedJSON, actualJSON) {
			t.Errorf("JSON output mismatch.\nGot: %s\nWant: %s", string(data), expected)
		}
	})

	// Subtest: Concurrent access to ensure thread safety
	t.Run("Concurrent", func(t *testing.T) {
		m := NewMultiError()
		err1 := New("error1").WithCode(400)
		err2 := errors.New("error2")
		m.Add(err1, err2) // Variadic add

		// Run multiple goroutines to marshal concurrently
		const numGoroutines = 10
		results := make(chan []byte, numGoroutines)
		errorsChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				data, err := json.Marshal(m)
				if err != nil {
					errorsChan <- err
					return
				}
				results <- data
			}()
		}

		// Collect results
		expected := `{
			"count":2,
			"errors":[
				{"error":{"message":"error1","code":400}},
				{"error":"error2"}
			]
		}`
		var expectedJSON interface{}
		if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
			t.Fatalf("Failed to parse expected JSON: %v", err)
		}

		for i := 0; i < numGoroutines; i++ {
			select {
			case err := <-errorsChan:
				t.Errorf("Concurrent MarshalJSON failed: %v", err)
			case data := <-results:
				var actualJSON interface{}
				if err := json.Unmarshal(data, &actualJSON); err != nil {
					t.Errorf("Failed to parse actual JSON: %v", err)
				}
				if !reflect.DeepEqual(expectedJSON, actualJSON) {
					t.Errorf("Concurrent JSON output mismatch.\nGot: %s\nWant: %s", string(data), expected)
				}
			}
		}
	})

	// Subtest: Variadic add with multiple errors
	t.Run("VariadicAdd", func(t *testing.T) {
		m := NewMultiError(WithLimit(10))
		err1 := New("error1").WithCode(400)
		err2 := errors.New("error2")
		err3 := errors.New("error3")
		m.Add(err1, err2, err3, nil, err2) // Mix of unique, nil, and duplicate errors

		if m.Count() != 3 {
			t.Errorf("Expected 3 errors, got %d", m.Count())
		}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		expected := `{
			"count":3,
			"limit":10,
			"errors":[
				{"error":{"message":"error1","code":400}},
				{"error":"error2"},
				{"error":"error3"}
			]
		}`
		var expectedJSON, actualJSON interface{}
		if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
			t.Fatalf("Failed to parse expected JSON: %v", err)
		}
		if err := json.Unmarshal(data, &actualJSON); err != nil {
			t.Fatalf("Failed to parse actual JSON: %v", err)
		}

		if !reflect.DeepEqual(expectedJSON, actualJSON) {
			t.Errorf("JSON output mismatch.\nGot: %s\nWant: %s", string(data), expected)
		}
	})
}
