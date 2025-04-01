package errors

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// errorRegistry holds registered errors using sync.Map for concurrent access.
var registry struct {
	templates  sync.Map     // map[string]string
	funcs      sync.Map     // map[string]func(...interface{}) *Error
	codes      sync.Map     // map[string]int
	counts     sync.Map     // map[string]*uint64
	thresholds sync.Map     // map[string]uint64
	alerts     sync.Map     // map[string]chan *Error
	mu         sync.RWMutex // Retained for Monitor initialization
}

// lastErrors tracks the most recent instance of each named error.
var lastErrors struct {
	m  map[string]*Error
	mu sync.RWMutex
}

func init() {
	lastErrors.m = make(map[string]*Error)
}

// newTemplateError creates an error from a template.
func newTemplateError(name, template string, args ...interface{}) *Error {
	if !DisableRegistry {
		incrementCount(name)
	}
	err := errorPool.Get().(*Error)
	err.Reset()
	err.name = name
	err.template = template
	err.msg = fmt.Sprintf(template, args...)
	if !DisableStack {
		err.stack = captureStack(1)
	}
	if !DisableRegistry {
		updateLastError(err)
	}
	return err
}

// Define registers an error template and returns a function to create errors.
func Define(name, template string) func(...interface{}) *Error {
	registry.templates.Store(name, template)
	if !DisableRegistry {
		initErrorCount(name)
	}
	return func(args ...interface{}) *Error {
		return newTemplateError(name, template, args...)
	}
}

// Callable registers a custom error function and returns it.
func Callable(name string, fn func(...interface{}) *Error) func(...interface{}) *Error {
	registry.funcs.Store(name, fn)
	if !DisableRegistry {
		initErrorCount(name)
	}
	return func(args ...interface{}) *Error {
		if !DisableRegistry {
			incrementCount(name)
		}
		err := fn(args...)
		if !DisableRegistry {
			updateLastError(err)
		}
		return err
	}
}

// Coded registers a template with an HTTP code and returns a function to create errors.
func Coded(name string, code int, template string) func(...interface{}) *Error {
	registry.codes.Store(name, code)
	return Define(name, template)
}

// Func creates a function-bound error.
func Func(fn interface{}, msg string) *Error {
	name := getFuncName(fn)
	if !DisableRegistry {
		initErrorCount(name)
		incrementCount(name)
	}
	err := errorPool.Get().(*Error)
	err.Reset()
	err.name = name
	err.msg = fmt.Sprintf("%s: %s", name, msg)
	if !DisableStack {
		err.stack = captureStack(1)
	}
	if !DisableRegistry {
		updateLastError(err)
	}
	return err
}

// SetThreshold configures an alerting threshold for an error type.
func SetThreshold(name string, count uint64) {
	registry.thresholds.Store(name, count)
}

// Monitor creates a channel for error alerts, initializing it if needed.
func Monitor(name string) <-chan *Error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if ch, ok := registry.alerts.Load(name); ok {
		return ch.(chan *Error)
	}
	ch := make(chan *Error, 10) // Buffered channel
	registry.alerts.Store(name, ch)
	return ch
}

// GetLastError returns the most recent instance of a named error.
func GetLastError(name string) *Error {
	lastErrors.mu.RLock()
	defer lastErrors.mu.RUnlock()
	return lastErrors.m[name]
}

// updateLastError stores the error as the most recent instance.
func updateLastError(e *Error) {
	if e == nil || e.name == "" {
		return
	}
	lastErrors.mu.Lock()
	defer lastErrors.mu.Unlock()
	lastErrors.m[e.name] = e
}

// initErrorCount sets up an atomic counter for an error name.
func initErrorCount(name string) {
	if _, loaded := registry.counts.LoadOrStore(name, new(uint64)); !loaded {
		// Counter was just stored, no further action needed
	}
}

// incrementCount safely increments the error counter.
func incrementCount(name string) {
	if counter, ok := registry.counts.Load(name); ok {
		atomic.AddUint64(counter.(*uint64), 1)
		checkThreshold(name)
	}
}

// checkThreshold triggers alerts if the count exceeds the threshold.
func checkThreshold(name string) {
	if threshold, ok := registry.thresholds.Load(name); ok {
		if counter, ok := registry.counts.Load(name); ok {
			if count := atomic.LoadUint64(counter.(*uint64)); count >= threshold.(uint64) {
				triggerAlert(name)
			}
		}
	}
}

// triggerAlert sends the last error to the alert channel.
func triggerAlert(name string) {
	if ch, ok := registry.alerts.Load(name); ok {
		lastErr := GetLastError(name)
		if lastErr != nil {
			select {
			case ch.(chan *Error) <- lastErr:
			default:
				// Non-blocking send
			}
		}
	}
}
