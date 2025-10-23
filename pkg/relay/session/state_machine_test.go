package session

import (
	"testing"
)

func TestNextState_ValidTransitions(t *testing.T) {
	tests := []struct {
		name          string
		currentState  SessionState
		event         Event
		expectedState SessionState
	}{
		// From CREATED
		{
			name:          "CREATED + SPAWN → SPAWNING",
			currentState:  StateCreated,
			event:         EventSpawn,
			expectedState: StateSpawning,
		},
		{
			name:          "CREATED + TERMINATE → TERMINATING (early cancel)",
			currentState:  StateCreated,
			event:         EventTerminate,
			expectedState: StateTerminating,
		},

		// From SPAWNING
		{
			name:          "SPAWNING + ACTIVATE → ACTIVE",
			currentState:  StateSpawning,
			event:         EventActivate,
			expectedState: StateActive,
		},
		{
			name:          "SPAWNING + TERMINATE → TERMINATING (spawn failure)",
			currentState:  StateSpawning,
			event:         EventTerminate,
			expectedState: StateTerminating,
		},

		// From ACTIVE
		{
			name:          "ACTIVE + TERMINATE → TERMINATING",
			currentState:  StateActive,
			event:         EventTerminate,
			expectedState: StateTerminating,
		},

		// From TERMINATING
		{
			name:          "TERMINATING + CLEAN → CLEANED",
			currentState:  StateTerminating,
			event:         EventClean,
			expectedState: StateCleaned,
		},
		{
			name:          "TERMINATING + TERMINATE → TERMINATING (idempotent)",
			currentState:  StateTerminating,
			event:         EventTerminate,
			expectedState: StateTerminating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextState, err := NextState(tt.currentState, tt.event)
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if nextState != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, nextState)
			}
		})
	}
}

func TestNextState_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name         string
		currentState SessionState
		event        Event
	}{
		// Invalid from CREATED
		{
			name:         "CREATED + ACTIVATE (must spawn first)",
			currentState: StateCreated,
			event:        EventActivate,
		},
		{
			name:         "CREATED + CLEAN (must terminate first)",
			currentState: StateCreated,
			event:        EventClean,
		},

		// Invalid from SPAWNING
		{
			name:         "SPAWNING + SPAWN (already spawning)",
			currentState: StateSpawning,
			event:        EventSpawn,
		},
		{
			name:         "SPAWNING + CLEAN (must terminate first)",
			currentState: StateSpawning,
			event:        EventClean,
		},

		// Invalid from ACTIVE
		{
			name:         "ACTIVE + SPAWN (already active)",
			currentState: StateActive,
			event:        EventSpawn,
		},
		{
			name:         "ACTIVE + ACTIVATE (already active)",
			currentState: StateActive,
			event:        EventActivate,
		},
		{
			name:         "ACTIVE + CLEAN (must terminate first)",
			currentState: StateActive,
			event:        EventClean,
		},

		// Invalid from TERMINATING
		{
			name:         "TERMINATING + SPAWN (terminating)",
			currentState: StateTerminating,
			event:        EventSpawn,
		},
		{
			name:         "TERMINATING + ACTIVATE (terminating)",
			currentState: StateTerminating,
			event:        EventActivate,
		},

		// Invalid from CLEANED (terminal state)
		{
			name:         "CLEANED + SPAWN (terminal)",
			currentState: StateCleaned,
			event:        EventSpawn,
		},
		{
			name:         "CLEANED + ACTIVATE (terminal)",
			currentState: StateCleaned,
			event:        EventActivate,
		},
		{
			name:         "CLEANED + TERMINATE (terminal)",
			currentState: StateCleaned,
			event:        EventTerminate,
		},
		{
			name:         "CLEANED + CLEAN (terminal)",
			currentState: StateCleaned,
			event:        EventClean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextState, err := NextState(tt.currentState, tt.event)
			if err == nil {
				t.Errorf("expected error for invalid transition, got nil (nextState=%s)", nextState)
			}

			// Verify it returns TransitionError
			if _, ok := err.(TransitionError); !ok {
				t.Errorf("expected TransitionError, got: %T", err)
			}

			// State should remain unchanged on error
			if nextState != tt.currentState {
				t.Errorf("expected state to remain %s on error, got %s", tt.currentState, nextState)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name          string
		currentState  SessionState
		event         Event
		canTransition bool
	}{
		// Valid transitions
		{"CREATED → SPAWN", StateCreated, EventSpawn, true},
		{"SPAWNING → ACTIVATE", StateSpawning, EventActivate, true},
		{"ACTIVE → TERMINATE", StateActive, EventTerminate, true},
		{"TERMINATING → CLEAN", StateTerminating, EventClean, true},

		// Invalid transitions
		{"CREATED → ACTIVATE", StateCreated, EventActivate, false},
		{"ACTIVE → SPAWN", StateActive, EventSpawn, false},
		{"CLEANED → SPAWN", StateCleaned, EventSpawn, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransition(tt.currentState, tt.event)
			if result != tt.canTransition {
				t.Errorf("expected %v, got %v", tt.canTransition, result)
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	tests := []struct {
		state      SessionState
		isTerminal bool
	}{
		{StateCreated, false},
		{StateSpawning, false},
		{StateActive, false},
		{StateTerminating, false},
		{StateCleaned, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			result := IsTerminalState(tt.state)
			if result != tt.isTerminal {
				t.Errorf("expected %v for state %s, got %v", tt.isTerminal, tt.state, result)
			}
		})
	}
}

func TestIsActiveState(t *testing.T) {
	tests := []struct {
		state    SessionState
		isActive bool
	}{
		{StateCreated, false},
		{StateSpawning, false},
		{StateActive, true},
		{StateTerminating, false},
		{StateCleaned, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			result := IsActiveState(tt.state)
			if result != tt.isActive {
				t.Errorf("expected %v for state %s, got %v", tt.isActive, tt.state, result)
			}
		})
	}
}

func TestSessionState_String(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{StateCreated, "CREATED"},
		{StateSpawning, "SPAWNING"},
		{StateActive, "ACTIVE"},
		{StateTerminating, "TERMINATING"},
		{StateCleaned, "CLEANED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}

func TestSessionState_IsValid(t *testing.T) {
	validStates := []SessionState{
		StateCreated,
		StateSpawning,
		StateActive,
		StateTerminating,
		StateCleaned,
	}

	for _, state := range validStates {
		t.Run(string(state), func(t *testing.T) {
			if !state.IsValid() {
				t.Errorf("expected %s to be valid", state)
			}
		})
	}

	// Test invalid state
	invalid := SessionState("INVALID")
	if invalid.IsValid() {
		t.Error("expected INVALID to be invalid")
	}
}

func TestEvent_String(t *testing.T) {
	tests := []struct {
		event    Event
		expected string
	}{
		{EventSpawn, "SPAWN"},
		{EventActivate, "ACTIVATE"},
		{EventTerminate, "TERMINATE"},
		{EventClean, "CLEAN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.event.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.event.String())
			}
		})
	}
}

func TestTransitionError_Error(t *testing.T) {
	err := NewTransitionError(StateActive, EventSpawn, "cannot spawn from active")
	expected := "invalid transition: state=ACTIVE event=SPAWN reason=cannot spawn from active"

	if err.Error() != expected {
		t.Errorf("expected error message:\n%s\ngot:\n%s", expected, err.Error())
	}
}
