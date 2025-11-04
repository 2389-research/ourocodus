package relay_test

import (
	"strings"
	"testing"

	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/stretchr/testify/assert"
)

func TestSessionCreated(t *testing.T) {
	tests := []struct {
		name          string
		userSessionID string
		want          string
	}{
		{
			name:          "normal user session ID",
			userSessionID: "sess-abc123",
			want:          "sessions.sess-abc123.session.created",
		},
		{
			name:          "user session ID with dots",
			userSessionID: "sess.abc.123",
			want:          "sessions.sess_abc_123.session.created",
		},
		{
			name:          "UUID user session ID",
			userSessionID: "550e8400-e29b-41d4-a716-446655440000",
			want:          "sessions.550e8400-e29b-41d4-a716-446655440000.session.created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionCreated(tt.userSessionID)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSessionTerminated(t *testing.T) {
	tests := []struct {
		name          string
		userSessionID string
		want          string
	}{
		{
			name:          "normal user session ID",
			userSessionID: "test-session",
			want:          "sessions.test-session.session.terminated",
		},
		{
			name:          "user session ID with dots",
			userSessionID: "test.session",
			want:          "sessions.test_session.session.terminated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionTerminated(tt.userSessionID)
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
	// Create a user session ID longer than 200 chars
	longID := strings.Repeat("a", 201)

	_, err := relay.SessionCreated(longID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, relay.ErrUserSessionIDTooLong)
}

func TestSanitizeID_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		userSessionID string
		want          string
	}{
		{
			name:          "multiple consecutive dots",
			userSessionID: "sess...123",
			want:          "sessions.sess___123.session.created",
		},
		{
			name:          "dots at boundaries",
			userSessionID: ".sess123.",
			want:          "sessions._sess123_.session.created",
		},
		{
			name:          "only dots",
			userSessionID: "...",
			want:          "sessions.___.session.created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relay.SessionCreated(tt.userSessionID)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
