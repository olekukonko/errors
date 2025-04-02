// error_pool.go
package errors

import (
	"sync"
	"sync/atomic"
)

// ErrorPool is a high-performance error pool
type ErrorPool struct {
	pool      sync.Pool
	poolStats struct {
		hits   int64
		misses int64
	}
}

// NewErrorPool creates a new error pool
func NewErrorPool() *ErrorPool {
	return &ErrorPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Error{
					smallContext: [contextSize]contextItem{},
				}
			},
		},
	}
}

// Get retrieves an error from the pool
func (ep *ErrorPool) Get() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}

	e := ep.pool.Get().(*Error)
	if e == nil {
		atomic.AddInt64(&ep.poolStats.misses, 1)
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}
	atomic.AddInt64(&ep.poolStats.hits, 1)
	return e
}

// Put returns an error to the pool
func (ep *ErrorPool) Put(e *Error) {
	if e == nil || currentConfig.disablePooling {
		return
	}

	// Properly reset while maintaining capacity
	e.Reset()

	// Handle stack slice properly
	if e.stack != nil {
		e.stack = e.stack[:0] // reset length but keep capacity
	}

	ep.pool.Put(e)
}

// Stats returns pool statistics
func (ep *ErrorPool) Stats() (hits, misses int64) {
	return atomic.LoadInt64(&ep.poolStats.hits),
		atomic.LoadInt64(&ep.poolStats.misses)
}
