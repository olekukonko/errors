//go:build go1.24
// +build go1.24

package errors

import (
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	errorPool   sync.Pool
	cleanupDone uint32 // Atomic flag to track cleanup registration
)

func init() {
	errorPool = sync.Pool{
		New: func() interface{} {
			err := &Error{}
			// Register cleanup with proper signature
			runtime.AddCleanup(err, func(_ *struct{}) {
				if !currentConfig.disablePooling {
					err.Reset()
					errorPool.Put(err)
				}
			}, nil) // Using nil as arg to avoid ptr==arg issue
			return err
		},
	}

	currentConfig = cachedConfig{
		stackDepth:     32,
		contextSize:    2,
		disableStack:   false,
		disablePooling: false,
		filterInternal: true,
	}
	WarmPool(100)
}

// getPooledError retrieves an error instance with proper cleanup
func getPooledError() *Error {
	if currentConfig.disablePooling {
		return &Error{}
	}

	var err *Error
	if pooled := errorPool.Get(); pooled != nil {
		atomic.AddUint64(&poolHits, 1)
		err = pooled.(*Error)
		err.Reset()
	} else {
		atomic.AddUint64(&poolMisses, 1)
		err = &Error{}
		// Register cleanup with dummy argument
		runtime.AddCleanup(err, func(_ *struct{}) {
			if !currentConfig.disablePooling {
				err.Reset()
				errorPool.Put(err)
			}
		}, nil)
	}
	return err
}
