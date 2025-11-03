package nats

import (
	"errors"
	"testing"
	"time"
)

// TestHealthTracker verifies health status tracking.
func TestHealthTracker(t *testing.T) {
	tracker := newHealthTracker()

	// Initial state
	status := tracker.status()
	if status.Connected {
		t.Error("Connected = true initially, want false")
	}

	// Set connected
	tracker.setConnected()
	status = tracker.status()
	if !status.Connected {
		t.Error("Connected = false after setConnected(), want true")
	}
	if status.LastReconnect.IsZero() {
		t.Error("LastReconnect not set after setConnected()")
	}

	// Record error
	testErr := errors.New("test error")
	tracker.recordError(testErr)
	status = tracker.status()
	if status.LastError != testErr {
		t.Errorf("LastError = %v, want %v", status.LastError, testErr)
	}
	if status.LastErrorTime.IsZero() {
		t.Error("LastErrorTime not set after recordError()")
	}

	// Set disconnected
	tracker.setDisconnected(testErr)
	status = tracker.status()
	if status.Connected {
		t.Error("Connected = true after setDisconnected(), want false")
	}
	if status.LastError != testErr {
		t.Errorf("LastError = %v, want %v", status.LastError, testErr)
	}

	// Set closed
	tracker.setClosed()
	status = tracker.status()
	if status.Connected {
		t.Error("Connected = true after setClosed(), want false")
	}
}

// TestHealthStatus verifies HealthStatus structure.
func TestHealthStatus(t *testing.T) {
	status := HealthStatus{
		Connected:     true,
		LastError:     errors.New("test"),
		LastErrorTime: time.Now(),
		LastReconnect: time.Now(),
		RTT:           time.Millisecond,
	}

	if !status.Connected {
		t.Error("Connected = false, want true")
	}

	if status.LastError == nil {
		t.Error("LastError = nil, want error")
	}

	if status.LastErrorTime.IsZero() {
		t.Error("LastErrorTime is zero, want non-zero")
	}

	if status.LastReconnect.IsZero() {
		t.Error("LastReconnect is zero, want non-zero")
	}

	if status.RTT != time.Millisecond {
		t.Errorf("RTT = %v, want %v", status.RTT, time.Millisecond)
	}
}
