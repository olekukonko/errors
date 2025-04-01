//go:build go1.24
// +build go1.24

package errors

import (
	"runtime"
	"sync"
)

var (
	errorPool sync.Pool
)

func init() {
	errorPool = sync.Pool{
		New: func() interface{} {
			e := &Error{pooled: true}
			if currentConfig.autofree {
				runtime.AddCleanup(e, func(_ *struct{}) {
					if e.pooled && !currentConfig.disablePooling {
						e.Reset()
						errorPool.Put(e)
					}
				}, nil)
			}
			return e
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

func getPooledError() *Error {
	if currentConfig.disablePooling {
		return &Error{pooled: false}
	}

	e := errorPool.Get().(*Error)
	e.pooled = true // Mark as from pool
	e.Reset()
	return e
}
