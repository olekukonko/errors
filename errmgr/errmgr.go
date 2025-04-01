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
	DisableErrMgr bool // Disables counting and tracking
}

var (
	configMu sync.RWMutex
	config   = Config{DisableErrMgr: false}
	registry = errorRegistry{counts: shardedCounter{}}
	codes    = codeRegistry{m: make(map[string]int)}
)

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

// Configure sets configuration options for the errmgr package.
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
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

// Define registers an error template and returns a function to create errors.
func Define(name, template string) func(...interface{}) *errors.Error {
	registry.templates.Store(name, template)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableErrMgr {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *errors.Error {
		var buf strings.Builder
		buf.Grow(len(template) + len(name) + len(args)*10)
		fmt.Fprintf(&buf, template, args...)
		err := errors.New(buf.String()).WithName(name).WithTemplate(template)
		configMu.RLock()
		defer configMu.RUnlock()
		if !config.DisableErrMgr {
			registry.counts.Inc(name)
		}
		return err
	}
}

// Coded registers a template with an HTTP code and returns a function to create errors.
func Coded(name string, code int, template string) func(...interface{}) *errors.Error {
	codes.mu.Lock()
	codes.m[name] = code
	codes.mu.Unlock()
	f := Define(name, template)
	return func(args ...interface{}) *errors.Error {
		err := f(args...)
		err.WithCode(code)
		return err
	}
}

// Categorized creates a categorized error template and returns a function to create errors.
func Categorized(category errors.ErrorCategory, name, template string) func(...interface{}) *errors.Error {
	f := Define(name, template)
	return func(args ...interface{}) *errors.Error {
		return f(args...).WithCategory(category)
	}
}

// Callable registers a custom error function and returns it.
func Callable(name string, fn func(...interface{}) *errors.Error) func(...interface{}) *errors.Error {
	registry.funcs.Store(name, fn)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableErrMgr {
		registry.counts.RegisterName(name)
	}
	return func(args ...interface{}) *errors.Error {
		configMu.RLock()
		defer configMu.RUnlock()
		if !config.DisableErrMgr {
			registry.counts.Inc(name)
		}
		return fn(args...)
	}
}

// Metrics returns a snapshot of error counts for monitoring systems.
func Metrics() map[string]uint64 {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableErrMgr {
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

// ResetCounter resets the occurrence counter for an error type.
func ResetCounter(name string) {
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableErrMgr {
		registry.counts.Reset(name)
	}
}

func Reset() {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisableErrMgr {
		return
	}
	registry.counts.counts.Range(func(key, _ interface{}) bool {
		registry.counts.Reset(key.(string))
		registry.counts.counts.Delete(key) // Remove the key entirely
		return true
	})
}
