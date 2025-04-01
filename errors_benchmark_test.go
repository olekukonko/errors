package errors

import (
	"errors"
	"testing"
)

func BenchmarkNewError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New("test error")
	}
}

func BenchmarkStdlibNewError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = errors.New("test error")
	}
}

func BenchmarkNamedError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Named("test_error")
	}
}

func BenchmarkErrorWithContext(b *testing.B) {
	err := New("base error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.With("key", i)
	}
}

func BenchmarkErrorWrapping(b *testing.B) {
	baseErr := New("base error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New("wrapper").Wrap(baseErr)
	}
}

func BenchmarkTemplateError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ErrDatabase("connection failed")
	}
}

func BenchmarkErrorStackCapture(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test")
		_ = err.Stack()
	}
}

func BenchmarkIs(b *testing.B) {
	err := New("wrapper").Wrap(Named("target"))
	target := Named("target")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Is(err, target)
	}
}

func BenchmarkAs(b *testing.B) {
	err := New("wrapper").Wrap(Named("target"))
	var target *Error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = As(err, &target)
	}
}

func BenchmarkCount(b *testing.B) {
	err := Named("test_count")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Count()
	}
}
