package audit

import (
	"encoding/json"
	"log"
	"time"
)

// EventType defines the type of audit event
type EventType string

const (
	// EventAgentAttach indicates an agent attachment operation
	EventAgentAttach EventType = "agent:attach"

	// EventAgentDetach indicates an agent detachment operation
	EventAgentDetach EventType = "agent:detach"

	// EventAuthFailure indicates an authentication failure
	EventAuthFailure EventType = "auth:failure"
)

// Event represents a single audit log entry
type Event struct {
	Timestamp time.Time         `json:"timestamp"`
	Type      EventType         `json:"type"`
	UserID    string            `json:"userId"`
	AgentID   string            `json:"agentId,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Log writes an audit event to the audit log
// All security-sensitive operations should be logged through this function
func Log(event Event) {
	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Marshal to JSON for structured logging
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[AUDIT] ERROR: Failed to marshal audit event: %v", err)
		return
	}

	// Write to log
	log.Printf("[AUDIT] %s", data)
}

// LogAgentAttach logs an agent attachment operation
func LogAgentAttach(userID, agentID string, success bool, err error) {
	event := Event{
		Type:    EventAgentAttach,
		UserID:  userID,
		AgentID: agentID,
		Success: success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	Log(event)
}

// LogAgentDetach logs an agent detachment operation
func LogAgentDetach(userID, agentID string, success bool, err error) {
	event := Event{
		Type:    EventAgentDetach,
		UserID:  userID,
		AgentID: agentID,
		Success: success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	Log(event)
}

// LogAuthFailure logs an authentication failure
func LogAuthFailure(userID, agentID, reason string, metadata map[string]string) {
	event := Event{
		Type:     EventAuthFailure,
		UserID:   userID,
		AgentID:  agentID,
		Success:  false,
		Error:    reason,
		Metadata: metadata,
	}

	Log(event)
}
