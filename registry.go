// Package errors provides registry functionality for tracking error templates,
// counts, thresholds, and alerts in a thread-safe manner.
package errors

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// errorRegistry holds registered errors and their metadata.
type errorRegistry struct {
	templates  sync.Map       // map[string]string: Error templates
	funcs      sync.Map       // map[string]func(...interface{}) *Error: Custom error functions
	counts     shardedCounter // Sharded counter for error occurrences
	thresholds sync.Map       // map[string]uint64: Alert thresholds
	alerts     sync.Map       // map[string]chan *Error: Alert channels
	mu         sync.RWMutex   // Protects alerts map and initialization
}

// codeRegistry manages error codes with explicit locking.
type codeRegistry struct {
	m  map[string]int
	mu sync.RWMutex
}

// shardedCounter provides a low-contention counter for error occurrences.
// It maintains a map of counters, each with 8 shards per error name.
type shardedCounter struct {
	counts sync.Map // map[string]*[8]struct{value uint64; pad [56]byte}: Per-name sharded counters
}

// Inc increments the counter for a specific name in a shard.
func (c *shardedCounter) Inc(name string) uint64 {
	shardPtr, _ := c.counts.LoadOrStore(name, &[8]struct {
		value uint64
		pad   [56]byte
	}{})
	shards := shardPtr.(*[8]struct {
		value uint64
		pad   [56]byte
	})
	shard := uint64(uintptr(unsafe.Pointer(&shards))) % 8
	return atomic.AddUint64(&shards[shard].value, 1)
}

// Value returns the total count for a specific name across all shards.
func (c *shardedCounter) Value(name string) uint64 {
	if shardPtr, ok := c.counts.Load(name); ok {
		shards := shardPtr.(*[8]struct {
			value uint64
			pad   [56]byte
		})
		var total uint64
		for i := range shards {
			total += atomic.LoadUint64(&shards[i].value)
		}
		return total
	}
	return 0
}

// Reset resets the counter for a specific name across all shards.
func (c *shardedCounter) Reset(name string) {
	if shardPtr, ok := c.counts.Load(name); ok {
		shards := shardPtr.(*[8]struct {
			value uint64
			pad   [56]byte
		})
		for i := range shards {
			atomic.StoreUint64(&shards[i].value, 0)
		}
	}
}

// RegisterName ensures a counter exists for the name.
func (c *shardedCounter) RegisterName(name string) {
	c.counts.LoadOrStore(name, &[8]struct {
		value uint64
		pad   [56]byte
	}{})
}

// ListNames returns all registered error names.
func (c *shardedCounter) ListNames() []string {
	var names []string
	c.counts.Range(func(key, _ interface{}) bool {
		names = append(names, key.(string))
		return true
	})
	return names
}

// lastErrors tracks the most recent instance of each named error.
var (
	registry   = errorRegistry{counts: shardedCounter{}}
	codes      = codeRegistry{m: make(map[string]int)}
	lastErrors = struct {
		m  map[string]*Error
		mu sync.RWMutex
	}{
		m: make(map[string]*Error),
	}
	locales       = make(map[string]string) // Dynamic locale map
	droppedAlerts uint64                    // Counter for dropped alerts
)

func init() {
	lastErrors.m = make(map[string]*Error)
}

// newTemplateError creates an error from a template with optimized string building.
func newTemplateError(name, template string, args ...interface{}) *Error {
	err := getPooledError()
	err.name = name
	err.template = template

	var buf strings.Builder
	buf.Grow(len(template) + len(name) + len(args)*10)
	fmt.Fprintf(&buf, template, args...)
	err.msg = buf.String()

	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	if !config.DisableRegistry {
		incrementCount(name)
		updateLastError(err)
	}
	return err
}

// Define registers an error template and returns a function to create errors.
func Define(name, template string) func(...interface{}) *Error {
	registry.templates.Store(name, template)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableRegistry {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *Error {
		return newTemplateError(name, template, args...)
	}
}

// Callable registers a custom error function and returns it.
func Callable(name string, fn func(...interface{}) *Error) func(...interface{}) *Error {
	registry.funcs.Store(name, fn)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableRegistry {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *Error {
		configMu.RLock()
		defer configMu.RUnlock()
		if !config.DisableRegistry {
			incrementCount(name)
		}
		err := fn(args...)
		if !config.DisableRegistry {
			updateLastError(err)
		}
		return err
	}
}

// Coded registers a template with an HTTP code and returns a function to create errors.
func Coded(name string, code int, template string) func(...interface{}) *Error {
	codes.mu.Lock()
	codes.m[name] = code
	codes.mu.Unlock()
	return Define(name, template)
}

// Categorized creates a categorized error template and returns a function to create errors.
func Categorized(category ErrorCategory, name, template string) func(...interface{}) *Error {
	f := Define(name, template)
	return func(args ...interface{}) *Error {
		return f(args...).WithCategory(category)
	}
}

// Func creates a function-bound error with the given message.
func Func(fn interface{}, msg string) *Error {
	name := getFuncName(fn)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableRegistry {
		registry.counts.RegisterName(name)
		incrementCount(name)
	}
	err := getPooledError()
	err.name = name
	err.msg = fmt.Sprintf("%s: %s", name, msg)
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	if !config.DisableRegistry {
		updateLastError(err)
	}
	return err
}

// SetThreshold configures an alerting threshold for an error type.
func SetThreshold(name string, count uint64) {
	registry.thresholds.Store(name, count)
}

// CountReset resets the occurrence counter and last error for an error type.
func CountReset(name string) {
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableRegistry {
		registry.counts.Reset(name)
		lastErrors.mu.Lock()
		delete(lastErrors.m, name)
		lastErrors.mu.Unlock()
	}
}

// ResetRegistry clears all counts and last errors from the registry.
// This is useful for testing or resetting global state.
func ResetRegistry() {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableRegistry {
		return
	}
	registry.counts.counts.Range(func(key, _ interface{}) bool {
		registry.counts.Reset(key.(string))
		registry.counts.counts.Delete(key) // Remove the key entirely
		return true
	})
	lastErrors.mu.Lock()
	for k := range lastErrors.m {
		delete(lastErrors.m, k)
	}
	lastErrors.mu.Unlock()
}

// SetLocales registers locale-specific error messages.
func SetLocales(localeMap map[string]string) {
	for key, value := range localeMap {
		locales[key] = value
	}
}

// GetLastError returns the most recent instance of a named error.
func GetLastError(name string) *Error {
	lastErrors.mu.RLock()
	defer lastErrors.mu.RUnlock()
	return lastErrors.m[name]
}

// Names returns a list of all registered error names.
func Names() []string {
	return registry.counts.ListNames()
}

// Metrics returns a snapshot of error counts for monitoring systems.
func Metrics() map[string]uint64 {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableRegistry {
		return nil
	}
	counts := make(map[string]uint64)
	registry.counts.counts.Range(func(key, value interface{}) bool {
		name := key.(string)
		count := registry.counts.Value(name)
		if count > 0 { // Only include non-zero counts
			counts[name] = count
		}
		return true
	})
	return counts
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

// incrementCount safely increments the error counter and checks thresholds.
func incrementCount(name string) {
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableRegistry {
		registry.counts.Inc(name)
		checkThreshold(name)
	}
}

// checkThreshold triggers alerts if the count exceeds the threshold.
func checkThreshold(name string) {
	if threshold, ok := registry.thresholds.Load(name); ok {
		count := registry.counts.Value(name)
		if count >= threshold.(uint64) {
			triggerAlert(name, count)
		}
	}
}

// triggerAlert sends the last error with the current count to the alert channel.
func triggerAlert(name string, count uint64) {
	if ch, ok := registry.alerts.Load(name); ok {
		lastErr := GetLastError(name)
		if lastErr != nil {
			// Create a new error with the current count explicitly set in context
			alertErr := lastErr.Copy()
			alertErr.With("count", count) // Add count to context for verification
			currentCount := registry.counts.Value(name)
			if currentCount != count {
				panic(fmt.Sprintf("Count mismatch: passed %d, current %d", count, currentCount))
			}
			select {
			case ch.(chan *Error) <- alertErr:
			default:
				atomic.AddUint64(&droppedAlerts, 1)
			}
		}
	}
}
