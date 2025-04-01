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
			e := &Error{
				smallContext: [contextSize]contextItem{},
				stack:        make([]uintptr, 0, currentConfig.stackDepth),
			}
			if currentConfig.autofree {
				runtime.AddCleanup(e, func(_ *struct{}) {
					if !currentConfig.disablePooling {
						e.Reset()
						// Keep the pre-allocated memory
						e.stack = e.stack[:0]
						errorPool.Put(e)
					}
				}, nil)
			}
			return e
		},
	}

	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: false,
		filterInternal: true,
		autofree:       true,
	}
	WarmPool(warmUpSize)
}

func getPooledError() *Error {
	if currentConfig.disablePooling {
		return &Error{}
	}

	e := errorPool.Get().(*Error)
	e.Reset()
	return e
}
