package errors

import (
	"sync"
)

type ErrorPool struct {
	pool sync.Pool
}

func NewErrorPool() *ErrorPool {
	ep := &ErrorPool{}
	ep.pool.New = func() interface{} {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil, // Ensure no stack is allocated by default
		}
	}
	return ep
}

func (ep *ErrorPool) Get() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
	}

	// Fast path without reset for new allocations
	e := ep.pool.Get().(*Error)
	if e.msg != "" { // Quick check if needs reset
		e.Reset()
		ep.clearCleanup(e)
	}
	return e
}

func (ep *ErrorPool) Put(e *Error) {
	if currentConfig.disablePooling {
		return
	}
	e.Reset()
	if e.stack != nil {
		e.stack = e.stack[:0] // Keep capacity but reset length
	}
	ep.pool.Put(e)
}
