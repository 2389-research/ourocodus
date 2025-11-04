package relay_test

import (
	"strings"
	"testing"

	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/stretchr/testify/assert"
)

func TestSessionCreated(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{
			name:      "normal session ID",
			sessionID: "sess-abc123",
			want:      "sessions.sess-abc123.session.created",
		},
		{
			name:      "session ID with dots",
			sessionID: "sess.abc.123",
			want:      "sessions.sess_abc_123.session.created",
		},
		{
			name:      "UUID session ID",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
			want:      "sessions.550e8400-e29b-41d4-a716-446655440000.session.created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionCreated(tt.sessionID)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSessionTerminated(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{
			name:      "normal session ID",
			sessionID: "test-session",
			want:      "sessions.test-session.session.terminated",
		},
		{
			name:      "session ID with dots",
			sessionID: "test.session",
			want:      "sessions.test_session.session.terminated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionTerminated(tt.sessionID)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAgentSpawned(t *testing.T) {
	got, err := relay.AgentSpawned("test-session")
	assert.NoError(t, err)
	assert.Equal(t, "sessions.test-session.agent.spawned", got)
}

func TestAgentTerminated(t *testing.T) {
	got, err := relay.AgentTerminated("test-session")
	assert.NoError(t, err)
	assert.Equal(t, "sessions.test-session.agent.terminated", got)
}

func TestSanitizeID_TooLong(t *testing.T) {
	// Create a session ID longer than 200 chars
	longID := strings.Repeat("a", 201)

	_, err := relay.SessionCreated(longID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, relay.ErrSessionIDTooLong)
}

func TestSanitizeID_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{
			name:      "multiple consecutive dots",
			sessionID: "sess...123",
			want:      "sessions.sess___123.session.created",
		},
		{
			name:      "dots at boundaries",
			sessionID: ".sess123.",
			want:      "sessions._sess123_.session.created",
		},
		{
			name:      "only dots",
			sessionID: "...",
			want:      "sessions.___.session.created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionCreated(tt.sessionID)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
