// Package errmgr provides functionality for managing error templates, counts, thresholds,
// and alerts in a thread-safe manner, building on the core errors package.
package errmgr

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/olekukonko/errors"
)

// Config holds configuration for the errmgr package.
type Config struct {
	DisableMetrics bool // Disables counting and tracking if true
}

// cachedConfig holds the current configuration, updated only on Configure().
type cachedConfig struct {
	disableErrMgr bool
}

var (
	currentConfig cachedConfig
	configMu      sync.RWMutex
	registry      = errorRegistry{counts: shardedCounter{}}
	codes         = codeRegistry{m: make(map[string]int)}
)

func init() {
	currentConfig = cachedConfig{disableErrMgr: false}
}

// errorRegistry holds registered errors and their metadata.
type errorRegistry struct {
	templates  sync.Map       // map[string]string: Error templates
	funcs      sync.Map       // map[string]func(...interface{}) *errors.Error: Custom error functions
	counts     shardedCounter // Sharded counter for error occurrences
	thresholds sync.Map       // map[string]uint64: Alert thresholds
	alerts     sync.Map       // map[string]chan *errors.Error: Alert channels
	mu         sync.RWMutex   // Protects alerts map
}

// codeRegistry manages error codes with explicit locking.
type codeRegistry struct {
	m  map[string]int
	mu sync.RWMutex
}

// shardedCounter provides a low-contention counter for error occurrences.
type shardedCounter struct {
	counts sync.Map // map[string]*[8]struct{value uint64; pad [56]byte}
}

// Configure updates the global configuration for the errmgr package.
// It is thread-safe and applies immediately to all operations.
func Configure(cfg Config) {
	configMu.Lock()
	currentConfig = cachedConfig{disableErrMgr: cfg.DisableMetrics}
	configMu.Unlock()
}

// Inc increments the counter for a specific name in a shard and checks thresholds.
// Returns the new total count for the name.
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
	newCount := atomic.AddUint64(&shards[shard].value, 1)

	if thresh, ok := registry.thresholds.Load(name); ok {
		total := c.Value(name)
		if total >= thresh.(uint64) {
			if ch, ok := registry.alerts.Load(name); ok {
				ac := ch.(*alertChannel)
				ac.mu.Lock()
				if !ac.closed {
					alert := errors.New(fmt.Sprintf("%s count exceeded threshold: %d", name, total)).
						WithName(name)
					for i := uint64(0); i < total; i++ {
						_ = alert.Increment()
					}
					select {
					case ac.ch <- alert:
					default: // Drop if channel is full
					}
				}
				ac.mu.Unlock()
			}
		}
	}
	return newCount
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

// RegisterName ensures a counter exists for the name without incrementing it.
func (c *shardedCounter) RegisterName(name string) {
	c.counts.LoadOrStore(name, &[8]struct {
		value uint64
		pad   [56]byte
	}{})
}

// ListNames returns all registered error names in the counter.
func (c *shardedCounter) ListNames() []string {
	var names []string
	c.counts.Range(func(key, _ interface{}) bool {
		names = append(names, key.(string))
		return true
	})
	return names
}

// Define creates a templated error that formats a message with provided arguments.
// The error is tracked in the registry if error management is enabled.
func Define(name, template string) func(...interface{}) *errors.Error {
	registry.templates.Store(name, template)
	if !currentConfig.disableErrMgr {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *errors.Error {
		var buf strings.Builder
		buf.Grow(len(template) + len(name) + len(args)*10)
		fmt.Fprintf(&buf, template, args...)
		err := errors.New(buf.String()).WithName(name).WithTemplate(template)
		if !currentConfig.disableErrMgr {
			registry.counts.Inc(name)
		}
		return err
	}
}

// Coded creates a templated error with a specific HTTP status code.
// It wraps Define and applies the code to each error instance returned.
func Coded(name, template string, code int) func(...interface{}) *errors.Error {
	base := Define(name, template)
	return func(args ...interface{}) *errors.Error {
		err := base(args...)
		return err.WithCode(code)
	}
}

// Categorized creates a categorized error template and returns a function to create errors.
// The returned function applies the category to each error instance.
func Categorized(category errors.ErrorCategory, name, template string) func(...interface{}) *errors.Error {
	f := Define(name, template)
	return func(args ...interface{}) *errors.Error {
		return f(args...).WithCategory(category)
	}
}

// Tracked registers a custom error function and tracks its occurrences in the registry.
// The returned function increments the error count each time it is called.
func Tracked(name string, fn func(...interface{}) *errors.Error) func(...interface{}) *errors.Error {
	registry.funcs.Store(name, fn)
	if !currentConfig.disableErrMgr {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *errors.Error {
		if !currentConfig.disableErrMgr {
			registry.counts.Inc(name)
		}
		return fn(args...)
	}
}

// Metrics returns a snapshot of error counts for monitoring systems.
// Returns nil if error management is disabled.
func Metrics() map[string]uint64 {
	if currentConfig.disableErrMgr {
		return nil
	}
	counts := make(map[string]uint64)
	registry.counts.counts.Range(func(key, value interface{}) bool {
		name := key.(string)
		count := registry.counts.Value(name)
		if count > 0 {
			counts[name] = count
		}
		return true
	})
	return counts
}

// ResetCounter resets the occurrence counter for a specific error type.
// Has no effect if error management is disabled.
func ResetCounter(name string) {
	if !currentConfig.disableErrMgr {
		registry.counts.Reset(name)
	}
}

// Reset clears all counters and removes their registrations.
// Has no effect if error management is disabled.
func Reset() {
	if currentConfig.disableErrMgr {
		return
	}
	registry.counts.counts.Range(func(key, _ interface{}) bool {
		registry.counts.Reset(key.(string))
		registry.counts.counts.Delete(key)
		return true
	})
}

// SetThreshold sets a count threshold for an error name, triggering alerts when exceeded.
// Alerts are sent to the Monitor channel if one exists for the name.
func SetThreshold(name string, threshold uint64) {
	registry.thresholds.Store(name, threshold)
}

// Err creates a new instance of a predefined static error, ensuring immutability of originals.
// Use this for static errors; templated errors should be called directly with arguments.
func Err(err *errors.Error) *errors.Error {
	return err.Copy()
}
