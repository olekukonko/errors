// Package errmgr provides error monitoring functionality.
package errmgr

import (
	"github.com/olekukonko/errors"
	"sync"
)

// alertChannel wraps a channel with synchronization
type alertChannel struct {
	ch     chan *errors.Error
	closed bool
	mu     sync.Mutex
}

// Monitor represents an error monitoring channel for a specific error name.
// It receives alerts when the error count exceeds a configured threshold.
type Monitor struct {
	name string
	ac   *alertChannel
}

// NewMonitor creates a new Monitor for the given error name.
// It returns an existing channel if one is already registered, or creates a new one with a buffer of 10.
func NewMonitor(name string) *Monitor {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if existing, ok := registry.alerts.Load(name); ok {
		return &Monitor{name: name, ac: existing.(*alertChannel)}
	}

	ac := &alertChannel{
		ch: make(chan *errors.Error, 10),
	}
	registry.alerts.Store(name, ac)
	return &Monitor{name: name, ac: ac}
}

// Alerts returns the channel for receiving error alerts.
// Alerts are sent when the error count exceeds the threshold set by SetThreshold.
func (m *Monitor) Alerts() <-chan *errors.Error {
	return m.ac.ch
}

// Close shuts down the monitor channel and removes it from the registry.
// Safe to call multiple times; subsequent calls have no effect.
func (m *Monitor) Close() {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if existing, ok := registry.alerts.Load(m.name); ok {
		if ac, ok := existing.(*alertChannel); ok && ac == m.ac {
			ac.mu.Lock()
			if !ac.closed {
				close(ac.ch)
				ac.closed = true
			}
			ac.mu.Unlock()
			registry.alerts.Delete(m.name)
		}
	}
}
