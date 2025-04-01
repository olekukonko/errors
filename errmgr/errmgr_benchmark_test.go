package errmgr

import (
	"fmt"
	"testing"
)

// BenchmarkTemplateError measures templated error performance.
func BenchmarkTemplateError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ErrDBConnection(fmt.Sprintf("connection failed %d", i))
		err.Free()
	}
}
