package nats

import "testing"

func TestNormalizeSubject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"sessions with ID", "sessions.abc123.events", "sessions.*.events"},
		{"sessions multiple segments", "sessions.test-id.work.extra", "sessions.*.work.extra"},
		{"sessions with results", "sessions.sess_456.results", "sessions.*.results"},
		{"sessions with approvals", "sessions.xyz.approvals", "sessions.*.approvals"},
		{"agents with IDs", "agents.sess1.agent2.heartbeat", "agents.*.*.heartbeat"},
		{"agents partial", "agents.sess1", "agents.*"},
		{"unknown prefix", "custom.topic.data", "custom.topic.data"},
		{"single token", "test", "test"},
		{"empty string", "", ""},
		{"sessions only", "sessions", "sessions"},
		{"agents only", "agents", "agents"},
		{"no dots", "singleword", "singleword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSubject(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeSubject(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
