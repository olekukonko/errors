package errors

import (
	"errors"
	"testing"
)

// BenchmarkNewError measures the default New performance.
func BenchmarkNewError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test error")
		err.Free()
	}
}

// BenchmarkNewNoStack measures New without stack traces.
func BenchmarkNewNoStack(b *testing.B) {
	DisableStack = true
	defer func() { DisableStack = false }()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test error")
		err.Free()
	}
}

// BenchmarkStdlibNewError measures stdlib errors.New performance.
func BenchmarkStdlibNewError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = errors.New("test error")
	}
}

// BenchmarkNamedError measures Named performance.
func BenchmarkNamedError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Named("test_error")
		err.Free()
	}
}

// BenchmarkErrorWithContext measures adding context performance.
func BenchmarkErrorWithContext(b *testing.B) {
	err := New("base error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.With("key", i)
	}
	err.Free()
}

// BenchmarkErrorWrapping measures wrapping performance.
func BenchmarkErrorWrapping(b *testing.B) {
	baseErr := New("base error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("wrapper").Wrap(baseErr)
		err.Free()
	}
	baseErr.Free()
}

// BenchmarkTemplateError measures templated error performance.
func BenchmarkTemplateError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ErrDatabase("connection failed")
		err.Free()
	}
}

// BenchmarkErrorStackCapture measures lazy stack capture performance.
func BenchmarkErrorStackCapture(b *testing.B) {
	DisableStack = true // Start without stack
	defer func() { DisableStack = false }()
	err := New("test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Stack()
	}
	err.Free()
}

// BenchmarkIs measures Is performance.
func BenchmarkIs(b *testing.B) {
	err := New("wrapper").Wrap(Named("target"))
	target := Named("target")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Is(err, target)
	}
	err.Free()
	target.Free()
}

// BenchmarkAs measures As performance.
func BenchmarkAs(b *testing.B) {
	err := New("wrapper").Wrap(Named("target"))
	var target *Error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = As(err, &target)
	}
	err.Free()
	if target != nil {
		target.Free()
	}
}

// BenchmarkCount measures Count performance with registry enabled.
func BenchmarkCount(b *testing.B) {
	err := Named("test_count")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Count()
	}
	err.Free()
}

// BenchmarkCountNoRegistry measures Count with registry disabled.
func BenchmarkCountNoRegistry(b *testing.B) {
	DisableRegistry = true
	defer func() { DisableRegistry = false }()
	err := Named("test_count")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Count()
	}
	err.Free()
}
