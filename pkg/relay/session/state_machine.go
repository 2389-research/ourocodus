package session

import "fmt"

// Event represents an event that can trigger a state transition
type Event string

const (
	// EventSpawn indicates we're starting to spawn the ACP process
	EventSpawn Event = "SPAWN"

	// EventActivate indicates ACP process successfully spawned and ready
	EventActivate Event = "ACTIVATE"

	// EventTerminate indicates session should begin cleanup
	EventTerminate Event = "TERMINATE"

	// EventClean indicates all resources freed, session can be removed
	EventClean Event = "CLEAN"
)

// String returns the string representation of Event
func (e Event) String() string {
	return string(e)
}

// TransitionError represents an invalid state transition attempt
type TransitionError struct {
	CurrentState SessionState
	Event        Event
	Reason       string
}

// Error implements the error interface
func (e TransitionError) Error() string {
	return fmt.Sprintf("invalid transition: state=%s event=%s reason=%s",
		e.CurrentState, e.Event, e.Reason)
}

// NewTransitionError creates a TransitionError
func NewTransitionError(state SessionState, event Event, reason string) TransitionError {
	return TransitionError{
		CurrentState: state,
		Event:        event,
		Reason:       reason,
	}
}

// NextState computes the next state given current state and event
// Pure function - no side effects, deterministic, fully testable
// Returns new state or TransitionError if transition is invalid
func NextState(current SessionState, event Event) (SessionState, error) {
	switch current {
	case StateCreated:
		switch event {
		case EventSpawn:
			return StateSpawning, nil
		case EventTerminate:
			// Allow termination before spawning (e.g., user cancels immediately)
			return StateTerminating, nil
		default:
			return current, NewTransitionError(current, event,
				"can only spawn or terminate from CREATED state")
		}

	case StateSpawning:
		switch event {
		case EventActivate:
			return StateActive, nil
		case EventTerminate:
			// Spawn failure or user cancellation during spawn
			return StateTerminating, nil
		default:
			return current, NewTransitionError(current, event,
				"can only activate or terminate from SPAWNING state")
		}

	case StateActive:
		switch event {
		case EventTerminate:
			return StateTerminating, nil
		default:
			return current, NewTransitionError(current, event,
				"can only terminate from ACTIVE state")
		}

	case StateTerminating:
		switch event {
		case EventClean:
			return StateCleaned, nil
		case EventTerminate:
			// Idempotent - repeated terminate requests are safe
			return StateTerminating, nil
		default:
			return current, NewTransitionError(current, event,
				"can only clean from TERMINATING state")
		}

	case StateCleaned:
		// Terminal state - no transitions allowed
		return current, NewTransitionError(current, event,
			"CLEANED is terminal state, no transitions allowed")

	default:
		return current, NewTransitionError(current, event,
			fmt.Sprintf("unknown state: %s", current))
	}
}

// CanTransition checks if a transition is valid without performing it
// Useful for validation before attempting state change
func CanTransition(current SessionState, event Event) bool {
	_, err := NextState(current, event)
	return err == nil
}

// IsTerminalState returns true if state is terminal (no further transitions)
func IsTerminalState(state SessionState) bool {
	return state == StateCleaned
}

// IsActiveState returns true if session is actively processing messages
func IsActiveState(state SessionState) bool {
	return state == StateActive
}
