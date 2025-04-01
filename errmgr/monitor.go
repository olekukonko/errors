// Package errmgr provides error monitoring functionality.
package errmgr

import "github.com/olekukonko/errors"

// Monitor represents an error monitoring channel for a specific error name.
// It receives alerts when the error count exceeds a configured threshold.
type Monitor struct {
	name string
	ch   chan *errors.Error
}

// NewMonitor creates a new Monitor for the given error name.
// It returns an existing channel if one is already registered, or creates a new one with a buffer of 10.
func NewMonitor(name string) *Monitor {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if ch, ok := registry.alerts.Load(name); ok {
		return &Monitor{name: name, ch: ch.(chan *errors.Error)}
	}
	ch := make(chan *errors.Error, 10)
	registry.alerts.Store(name, ch)
	return &Monitor{name: name, ch: ch}
}

// Alerts returns the channel for receiving error alerts.
// Alerts are sent when the error count exceeds the threshold set by SetThreshold.
func (m *Monitor) Alerts() <-chan *errors.Error {
	return m.ch
}

// Close shuts down the monitor channel and removes it from the registry.
// Safe to call multiple times; subsequent calls have no effect.
func (m *Monitor) Close() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if ch, ok := registry.alerts.LoadAndDelete(m.name); ok {
		if chanPtr, ok := ch.(chan *errors.Error); ok && chanPtr != nil {
			close(chanPtr)
		}
	}
}
