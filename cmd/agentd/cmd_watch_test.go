package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasLeaseChanged(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name string
		old  *session.Lease
		new  *session.Lease
		want bool
	}{
		{
			name: "both nil - no change",
			old:  nil,
			new:  nil,
			want: false,
		},
		{
			name: "old nil, new not nil - changed",
			old:  nil,
			new:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			want: true,
		},
		{
			name: "old not nil, new nil - changed",
			old:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			new:  nil,
			want: true,
		},
		{
			name: "same agent and session - no change",
			old:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			new:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			want: false,
		},
		{
			name: "same agent and session, different expiry - no change (expiry jitter ignored)",
			old:  &session.Lease{AgentID: "alice", UserSessionID: "session-1", ExpiresAt: now},
			new:  &session.Lease{AgentID: "alice", UserSessionID: "session-1", ExpiresAt: later},
			want: false,
		},
		{
			name: "different agent - changed",
			old:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			new:  &session.Lease{AgentID: "bob", UserSessionID: "session-1"},
			want: true,
		},
		{
			name: "different session - changed",
			old:  &session.Lease{AgentID: "alice", UserSessionID: "session-1"},
			new:  &session.Lease{AgentID: "alice", UserSessionID: "session-2"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasLeaseChanged(tt.old, tt.new)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHeartbeatEventJSON(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	event := HeartbeatEvent{
		Type:      "heartbeat",
		AgentID:   "alice",
		Timestamp: now,
		Status:    "idle",
		LagMs:     150,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Unmarshal back to verify round-trip
	var decoded HeartbeatEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "heartbeat", decoded.Type)
	assert.Equal(t, "alice", decoded.AgentID)
	assert.Equal(t, "idle", decoded.Status)
	assert.Equal(t, int64(150), decoded.LagMs)
	assert.True(t, decoded.Timestamp.Equal(now))

	// Verify JSON field names
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	assert.Contains(t, raw, "type")
	assert.Contains(t, raw, "agentId")
	assert.Contains(t, raw, "timestamp")
	assert.Contains(t, raw, "status")
	assert.Contains(t, raw, "lag")
}

func TestLeaseDetachedEventJSON(t *testing.T) {
	event := LeaseDetachedEvent{
		Type: "lease_detached",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "lease_detached", decoded["type"])
	assert.Len(t, decoded, 1) // Only type field
}

func TestLeaseRenewalEventJSON(t *testing.T) {
	expiresAt := time.Date(2025, 1, 15, 13, 0, 0, 0, time.UTC)

	event := LeaseRenewalEvent{
		Type:            "lease_renewal",
		AgentID:         "alice",
		UserSessionID:   "session-abc123",
		ExpiresAt:       expiresAt,
		TimeUntilExpiry: 3600.5,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Verify JSON field names match camelCase convention
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "lease_renewal", raw["type"])
	assert.Equal(t, "alice", raw["agentId"])
	assert.Equal(t, "session-abc123", raw["userSessionId"])
	assert.Contains(t, raw, "expiresAt")
	assert.Equal(t, 3600.5, raw["timeUntilExpiry"])
}

func TestDetectWatchOutputMode(t *testing.T) {
	// Save original flag values
	origJSON := watchJSON
	origPlain := watchPlain
	defer func() {
		watchJSON = origJSON
		watchPlain = origPlain
	}()

	tests := []struct {
		name      string
		jsonFlag  bool
		plainFlag bool
		want      string // Use string for comparison since output.Mode is internal
	}{
		{
			name:      "JSON flag takes precedence",
			jsonFlag:  true,
			plainFlag: false,
			want:      "json",
		},
		{
			name:      "JSON flag takes precedence over plain",
			jsonFlag:  true,
			plainFlag: true,
			want:      "json",
		},
		{
			name:      "plain flag when no JSON",
			jsonFlag:  false,
			plainFlag: true,
			want:      "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watchJSON = tt.jsonFlag
			watchPlain = tt.plainFlag

			mode := detectWatchOutputMode()

			switch tt.want {
			case "json":
				assert.True(t, mode.IsJSON(), "expected JSON mode")
			case "plain":
				assert.True(t, mode.IsPlain(), "expected plain mode")
			case "rich":
				assert.True(t, mode.IsRich(), "expected rich mode")
			}
		})
	}
}

func TestStartLogStreamer(t *testing.T) {
	// Save original flag value
	origWatchLogs := watchLogs
	defer func() {
		watchLogs = origWatchLogs
	}()

	t.Run("returns nil when watchLogs is false", func(t *testing.T) {
		watchLogs = false
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use JSON mode to prevent actual log streaming attempts
		logCancel := startLogStreamer(ctx, "test-agent", output.ModeJSON)
		assert.Nil(t, logCancel, "expected nil cancel func when watchLogs is false")
	})

	t.Run("returns cancel func when watchLogs is true", func(t *testing.T) {
		watchLogs = true
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use JSON mode - streamAgentLogs returns early for JSON mode
		logCancel := startLogStreamer(ctx, "test-agent", output.ModeJSON)
		assert.NotNil(t, logCancel, "expected non-nil cancel func when watchLogs is true")

		// Clean up
		logCancel()
	})
}

func TestRunWatchEventLoop(t *testing.T) {
	t.Run("exits on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		msgChan := make(chan *nats.Msg, 1)
		leaseChan := make(chan *session.Lease, 1)

		done := make(chan struct{})
		go func() {
			runWatchEventLoop(ctx, output.ModeJSON, msgChan, leaseChan, nil)
			close(done)
		}()

		// Cancel context
		cancel()

		// Should exit quickly
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("event loop did not exit on context cancellation")
		}
	})

	t.Run("calls logCancel on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		msgChan := make(chan *nats.Msg, 1)
		leaseChan := make(chan *session.Lease, 1)

		logCancelCalled := false
		logCancel := func() { logCancelCalled = true }

		done := make(chan struct{})
		go func() {
			runWatchEventLoop(ctx, output.ModeJSON, msgChan, leaseChan, logCancel)
			close(done)
		}()

		cancel()

		select {
		case <-done:
			assert.True(t, logCancelCalled, "expected logCancel to be called")
		case <-time.After(time.Second):
			t.Fatal("event loop did not exit")
		}
	})

	t.Run("processes lease changes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		msgChan := make(chan *nats.Msg, 1)
		leaseChan := make(chan *session.Lease, 1)

		// Start event loop
		go func() {
			runWatchEventLoop(ctx, output.ModeJSON, msgChan, leaseChan, nil)
		}()

		// Send a lease change
		leaseChan <- &session.Lease{
			AgentID:       "test-agent",
			UserSessionID: "test-session",
			ExpiresAt:     time.Now().Add(time.Hour),
		}

		// Give time for processing
		time.Sleep(50 * time.Millisecond)

		// If we get here without panic, the lease was processed
		// (output goes to stdout which we don't capture in this test)
	})
}
