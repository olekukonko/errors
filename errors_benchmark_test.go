package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// Basic error creation benchmarks
func BenchmarkBasic_New(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test error")
		err.Free()
	}
}

func BenchmarkBasic_NewNoFree(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New("test error") // Test without Free()
	}
}

func BenchmarkBasic_StdlibComparison(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = errors.New("test error") // Stdlib baseline
	}
}

// Stack trace related benchmarks
func BenchmarkStack_WithStack(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("test").WithStack()
		err.Free()
	}
}

func BenchmarkStack_Trace(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Trace("test error")
		err.Free()
	}
}

func BenchmarkStack_Capture(b *testing.B) {
	err := New("test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Stack() // Measure stack trace generation
	}
	err.Free()
}

// Context operation benchmarks
func BenchmarkContext_Small(b *testing.B) {
	err := New("base")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.With("key", i).With("key2", i+1) // Fits in smallContext array
	}
	err.Free()
}

func BenchmarkContext_Map(b *testing.B) {
	err := New("base")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Exceeds smallContext capacity
		_ = err.With("k1", i).With("k2", i+1).With("k3", i+2)
	}
	err.Free()
}

func BenchmarkContext_Reuse(b *testing.B) {
	err := New("base").With("init", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.With("key", i) // Test with existing context
	}
	err.Free()
}

// Error wrapping benchmarks
func BenchmarkWrapping_Simple(b *testing.B) {
	base := New("base")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := New("wrapper").Wrap(base)
		err.Free()
	}
	base.Free()
}

func BenchmarkWrapping_Deep(b *testing.B) {
	var err *Error
	for i := 0; i < 10; i++ {
		err = New("level").Wrap(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Unwrap() // Measure unwrapping performance
	}
	err.Free()
}

// Type assertion benchmarks
func BenchmarkTypeAssertion_Is(b *testing.B) {
	target := Named("target")
	err := New("wrapper").Wrap(target)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Is(err, target)
	}
	target.Free()
}

func BenchmarkTypeAssertion_As(b *testing.B) {
	err := New("wrapper").Wrap(Named("target"))
	var target *Error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = As(err, &target)
	}
	if target != nil {
		target.Free()
	}
}

// Serialization benchmarks
func BenchmarkSerialization_JSON(b *testing.B) {
	err := New("test").With("key", "value").With("num", 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(err)
	}
}

func BenchmarkSerialization_JSONWithStack(b *testing.B) {
	err := Trace("test").With("key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(err)
	}
}

// Concurrency benchmarks
func BenchmarkConcurrency_Creation(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := New("parallel error")
			err.Free()
		}
	})
}

func BenchmarkConcurrency_Context(b *testing.B) {
	base := New("base")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = base.With("key", "value")
		}
	})
	base.Free()
}

// Special case benchmarks
func BenchmarkSpecial_Named(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Named("test_error")
		err.Free()
	}
}

func BenchmarkSpecial_Format(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Errorf("formatted %s %d", "error", i)
		err.Free()
	}
}

func BenchmarkPoolGetPut(b *testing.B) {
	e := &Error{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errorPool.Put(e)
		e = errorPool.Get()
	}
}

func BenchmarkStackAlloc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = make([]uintptr, 0, currentConfig.stackDepth)
	}
}

func BenchmarkContext_Concurrent(b *testing.B) {
	err := New("base")
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			err.With(fmt.Sprintf("key%d", i%10), i)
			i++
		}
	})
}

func BenchmarkPoolWarmup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		errorPool = NewErrorPool() // Reset pool
		WarmPool(100)
	}
}
