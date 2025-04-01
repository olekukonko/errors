// Package errmgr provides error monitoring functionality.
package errmgr

import "github.com/olekukonko/errors"

// Monitor represents an error monitoring channel for a specific error name.
type Monitor struct {
	name string
	ch   chan *errors.Error
}

// NewMonitor creates a new Monitor for the given error name.
// It receives errors when their count exceeds the threshold.
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

// Chan returns the channel for receiving error alerts.
func (m *Monitor) Chan() <-chan *errors.Error {
	return m.ch
}

// Close shuts down the monitor channel.
func (m *Monitor) Close() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if ch, ok := registry.alerts.LoadAndDelete(m.name); ok {
		if chanPtr, ok := ch.(chan *errors.Error); ok && chanPtr != nil {
			close(chanPtr)
		}
	}
}
