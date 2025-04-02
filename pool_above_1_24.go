//go:build go1.24
// +build go1.24

package errors

import "runtime"

func (ep *ErrorPool) setupCleanup(e *Error) {
	if currentConfig.autoFree {
		runtime.AddCleanup(e, func(_ *struct{}) {
			if !currentConfig.disablePooling {
				ep.Put(e)
			}
		}, nil)
	}
}

func (ep *ErrorPool) clearCleanup(e *Error) {
	// No-op for Go 1.24+
}
