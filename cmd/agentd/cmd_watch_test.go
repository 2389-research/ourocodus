package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
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
