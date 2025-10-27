package session

import "testing"

// TestUserSessionState_String tests String() method for UserSessionState
func TestUserSessionState_String(t *testing.T) {
	tests := []struct {
		state    UserSessionState
		expected string
	}{
		{StateActive, "ACTIVE"},
		{StateTerminated, "TERMINATED"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

// TestUserSessionState_IsValid tests IsValid() method for UserSessionState
func TestUserSessionState_IsValid(t *testing.T) {
	tests := []struct {
		state UserSessionState
		valid bool
	}{
		{StateActive, true},
		{StateTerminated, true},
		{UserSessionState("INVALID"), false},
		{UserSessionState(""), false},
	}

	for _, tt := range tests {
		if tt.state.IsValid() != tt.valid {
			t.Errorf("State %s: expected IsValid()=%v, got %v", tt.state, tt.valid, tt.state.IsValid())
		}
	}
}

// TestAgentState_String tests String() method for AgentState
func TestAgentState_String(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected string
	}{
		{AgentSpawning, "SPAWNING"},
		{AgentActive, "ACTIVE"},
		{AgentFailed, "FAILED"},
		{AgentTerminated, "TERMINATED"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

// TestAgentState_IsValid tests IsValid() method for AgentState
func TestAgentState_IsValid(t *testing.T) {
	tests := []struct {
		state AgentState
		valid bool
	}{
		{AgentSpawning, true},
		{AgentActive, true},
		{AgentFailed, true},
		{AgentTerminated, true},
		{AgentState("INVALID"), false},
		{AgentState(""), false},
	}

	for _, tt := range tests {
		if tt.state.IsValid() != tt.valid {
			t.Errorf("State %s: expected IsValid()=%v, got %v", tt.state, tt.valid, tt.state.IsValid())
		}
	}
}
