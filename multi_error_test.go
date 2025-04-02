// multi_error_test.go
package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestMultiError_Basic(t *testing.T) {
	m := NewMultiError()
	if m.Has() {
		t.Error("New MultiError should be empty")
	}

	m.Add(nil)
	if m.Has() {
		t.Error("Adding nil should not create error")
	}

	err1 := errors.New("error 1")
	m.Add(err1)
	if !m.Has() {
		t.Error("Should detect errors")
	}
	if m.Count() != 1 {
		t.Error("Count should be 1")
	}
	if m.First() != err1 || m.Last() != err1 {
		t.Error("First/Last should match")
	}
}

func TestMultiError_Sampling(t *testing.T) {
	m := NewMultiError(WithSampling(50))
	total := 1000
	added := 0

	for i := 0; i < total; i++ {
		m.Add(errors.New("test"))
		added++
	}

	count := m.Count()
	ratio := float64(count) / float64(total)

	// Check if sampling is roughly 50%
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("Sampling ratio %v not within expected range (40-60%%)", ratio)
	}
}

func TestMultiError_Limit(t *testing.T) {
	limit := 10
	m := NewMultiError(WithLimit(limit))

	for i := 0; i < limit*2; i++ {
		m.Add(errors.New("test"))
	}

	if m.Count() != limit {
		t.Errorf("Should cap at %d errors, got %d", limit, m.Count())
	}
}

func TestMultiError_Formatting(t *testing.T) {
	customFormat := func(errs []error) string {
		return fmt.Sprintf("custom: %d", len(errs))
	}

	m := NewMultiError(WithFormatter(customFormat))
	m.Add(errors.New("test1"))
	m.Add(errors.New("test2"))

	expected := "custom: 2"
	if m.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, m.Error())
	}
}

func TestMultiError_Filter(t *testing.T) {
	m := NewMultiError()
	m.Add(errors.New("error1"))
	m.Add(errors.New("skip"))
	m.Add(errors.New("error2"))

	filtered := m.Filter(func(err error) bool {
		return err.Error() != "skip"
	})

	if filtered.Count() != 2 {
		t.Error("Should filter out one error")
	}
}

func TestMultiError_AsSingle(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		m := NewMultiError()
		if m.Single() != nil {
			t.Error("Empty should return nil")
		}
	})

	t.Run("Single", func(t *testing.T) {
		m := NewMultiError()
		err := errors.New("test")
		m.Add(err)
		if m.Single() != err {
			t.Error("Should return single error")
		}
	})

	t.Run("Multiple", func(t *testing.T) {
		m := NewMultiError()
		m.Add(errors.New("test1"))
		m.Add(errors.New("test2"))
		if m.Single() != m {
			t.Error("Should return self for multiple errors")
		}
	})
}
