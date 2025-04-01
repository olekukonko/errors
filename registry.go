package errors

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// errorRegistry holds registered errors
type errorRegistry struct {
	templates  map[string]string
	funcs      map[string]func(...interface{}) *Error
	codes      map[string]int
	counts     map[string]*uint64
	thresholds map[string]uint64
	alerts     map[string]chan *Error
	mu         sync.RWMutex
}

var registry = errorRegistry{
	templates:  make(map[string]string),
	funcs:      make(map[string]func(...interface{}) *Error),
	codes:      make(map[string]int),
	counts:     make(map[string]*uint64),
	thresholds: make(map[string]uint64),
	alerts:     make(map[string]chan *Error),
}

// newTemplateError creates error from template
func newTemplateError(name, template string, args ...interface{}) *Error {
	incrementCount(name)
	err := &Error{
		name:     name,
		template: template,
		msg:      fmt.Sprintf(template, args...),
		stack:    captureStack(1),
	}
	updateLastError(err)
	return err
}

// Define registers an error template
func Define(name, template string) func(...interface{}) *Error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.templates[name] = template
	initErrorCount(name)
	return func(args ...interface{}) *Error {
		return newTemplateError(name, template, args...)
	}
}

// Callable registers a custom error function
func Callable(name string, fn func(...interface{}) *Error) func(...interface{}) *Error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.funcs[name] = fn
	initErrorCount(name)
	return func(args ...interface{}) *Error {
		incrementCount(name)
		err := fn(args...)
		updateLastError(err)
		return err
	}
}

// Coded registers template with HTTP code
func Coded(name string, code int, template string) func(...interface{}) *Error {
	registry.mu.Lock()
	registry.codes[name] = code
	registry.mu.Unlock()
	return Define(name, template)
}

// Func creates function-bound error
func Func(fn interface{}, msg string) *Error {
	name := getFuncName(fn)
	registry.mu.Lock()
	if _, exists := registry.counts[name]; !exists {
		registry.counts[name] = new(uint64)
	}
	registry.mu.Unlock()
	incrementCount(name)
	err := &Error{
		name:  name,
		msg:   fmt.Sprintf("%s: %s", name, msg),
		stack: captureStack(1),
	}
	updateLastError(err)
	return err
}

// SetThreshold configures alerting threshold for an error type
func SetThreshold(name string, count uint64) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.thresholds[name] = count
}

// Monitor creates a channel for error alerts
func Monitor(name string) <-chan *Error {
	registry.mu.Lock()
	if registry.alerts[name] == nil {
		registry.alerts[name] = make(chan *Error, 10) // Buffered channel
	}
	ch := registry.alerts[name]
	registry.mu.Unlock()
	return ch
}

// lastErrors tracks the most recent instance of each named error
var lastErrors struct {
	m  map[string]*Error
	mu sync.RWMutex
}

func init() {
	lastErrors.m = make(map[string]*Error)
}

// GetLastError returns the most recent instance of a named error
func GetLastError(name string) *Error {
	lastErrors.mu.RLock()
	defer lastErrors.mu.RUnlock()
	return lastErrors.m[name]
}

// updateLastError stores the error as the most recent instance
func updateLastError(e *Error) {
	if e == nil || e.name == "" {
		return
	}
	lastErrors.mu.Lock()
	defer lastErrors.mu.Unlock()
	lastErrors.m[e.name] = e
}

// initErrorCount sets up atomic counter (called with lock held)
func initErrorCount(name string) {
	if _, exists := registry.counts[name]; !exists {
		registry.counts[name] = new(uint64)
	}
}

// incrementCount safely increments error counter
func incrementCount(name string) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if counter, exists := registry.counts[name]; exists {
		atomic.AddUint64(counter, 1)
		checkThreshold(name)
	}
}

// checkThreshold triggers alerts if needed
func checkThreshold(name string) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if threshold, ok := registry.thresholds[name]; ok {
		if counter, exists := registry.counts[name]; exists {
			if count := atomic.LoadUint64(counter); count >= threshold {
				triggerAlert(name)
			}
		}
	}
}

// triggerAlert sends error to alert channel
func triggerAlert(name string) {
	if ch, ok := registry.alerts[name]; ok {
		lastErr := GetLastError(name)
		if lastErr != nil {
			select {
			case ch <- lastErr:
			default:
				// Non-blocking send
			}
		}
	}
}
