//go:build !go1.24
// +build !go1.24

package errors

import (
	"sync"
	"sync/atomic"
)

var errorPool = sync.Pool{
	New: func() interface{} {
		return &Error{}
	},
}

func init() {
	currentConfig = cachedConfig{
		stackDepth:     32,
		contextSize:    2,
		disableStack:   false,
		disablePooling: true, // Disabled by default for Go < 1.24
		filterInternal: true,
	}
	WarmPool(100)
}

// getPooledError retrieves an error instance without cleanup for Go < 1.24
func getPooledError() *Error {
	if currentConfig.disablePooling {
		return &Error{}
	}
	if pooled := errorPool.Get(); pooled != nil {
		atomic.AddUint64(&poolHits, 1)
		err := pooled.(*Error)
		err.Reset()
		return err
	}
	atomic.AddUint64(&poolMisses, 1)
	return &Error{}
}
