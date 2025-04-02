//go:build !go1.24
// +build !go1.24

package errors

func (ep *ErrorPool) setupCleanup(e *Error) {
	if currentConfig.autoFree {
		runtime.SetFinalizer(e, func(e *Error) {
			if !currentConfig.disablePooling {
				ep.Put(e)
			}
		})
	}
}

func (ep *ErrorPool) clearCleanup(e *Error) {
	runtime.SetFinalizer(e, nil)
}
