package nats

import (
	"sync"
	"time"
)

// HealthStatus represents the health status of the client.
type HealthStatus struct {
	Connected     bool
	LastError     error
	LastErrorTime time.Time
	LastReconnect time.Time
	RTT           time.Duration
}

// healthTracker tracks the health status of the client.
type healthTracker struct {
	mu            sync.RWMutex
	connected     bool
	lastError     error
	lastErrorTime time.Time
	lastReconnect time.Time
}

// newHealthTracker creates a new health tracker.
func newHealthTracker() *healthTracker {
	return &healthTracker{}
}

// setConnected marks the client as connected.
func (h *healthTracker) setConnected() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connected = true
	h.lastReconnect = time.Now()
}

// setDisconnected marks the client as disconnected.
func (h *healthTracker) setDisconnected(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connected = false
	h.lastError = err
	h.lastErrorTime = time.Now()
}

// setClosed marks the client as closed.
func (h *healthTracker) setClosed() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connected = false
}

// recordError records an error.
func (h *healthTracker) recordError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastError = err
	h.lastErrorTime = time.Now()
}

// status returns the current health status.
func (h *healthTracker) status() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HealthStatus{
		Connected:     h.connected,
		LastError:     h.lastError,
		LastErrorTime: h.lastErrorTime,
		LastReconnect: h.lastReconnect,
		RTT:           0, // TODO: Implement RTT tracking
	}
}
