package errors

import (
	"encoding/json"
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
	Configure(Config{DisableStack: true}) // Replace DisableStack = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test error")
		err.Free()
	}
}

// BenchmarkFastNew measures the lightweight FastNew performance.
func BenchmarkFastNew(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Fast("test error")
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

// BenchmarkErrorWithContext measures adding context performance with small cache.
func BenchmarkErrorWithContext(b *testing.B) {
	err := New("base error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.With("key", i) // Should use smallContext
	}
	err.Free()
}

// BenchmarkErrorWithOne measures adding a single context item performance.
func BenchmarkErrorWithOne(b *testing.B) {
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

// BenchmarkErrorStackCapture measures lazy stack capture performance.
func BenchmarkErrorStackCapture(b *testing.B) {
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

func BenchmarkJSONMarshal(b *testing.B) {
	err := New("test").With("key1", "value1").With("key2", 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(err)
	}
}

func BenchmarkConcurrentErrorCreation(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := New("concurrent error")
			err.Free()
		}
	})
}
