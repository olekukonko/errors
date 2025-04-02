//go:build !go1.24
// +build !go1.24

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
				runtime.SetFinalizer(e, func(e *Error) {
					if !currentConfig.disablePooling {
						e.Reset()
						// Keep pre-allocated memory
						e.stack = e.stack[:0]
						errorPool.Put(e)
					}
				})
			}
			return e
		},
	}

	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: true,
		filterInternal: true,
		autofree:       false,
	}
	WarmPool(warmUpSize)
}

func getPooledError() *Error {
	if currentConfig.disablePooling {
		return &Error{}
	}
	e := errorPool.Get().(*Error)
	e.Reset()
	runtime.SetFinalizer(e, nil) // Remove temporary finalizer
	return e
}
