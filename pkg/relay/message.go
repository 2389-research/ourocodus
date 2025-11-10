package relay

import (
	"encoding/json"
	"fmt"
)

const (
	// ProtocolVersion is the current WebSocket protocol version
	ProtocolVersion = "1.0"
)

// Error Codes and Recoverability Semantics
//
// The relay server uses typed errors from the session layer and maps them to protocol error codes.
// Each error code has defined recoverability semantics that determine whether the client should
// close the connection or can retry the operation.
//
// Non-Recoverable Errors (client must close connection or create missing resources):
//   - VERSION_MISMATCH: Client protocol version incompatible with server (upgrade/downgrade required)
//   - SESSION_NOT_FOUND: Session ID does not exist (client must create session first)
//   - AGENT_NOT_FOUND: Agent role not found in session (client must spawn agent first)
//
// Recoverable Errors (client may retry or handle gracefully):
//   - INVALID_MESSAGE: Malformed JSON or missing required fields (fix and retry)
//   - SESSION_CREATE_FAILED: Temporary failure creating session (retry with backoff)
//   - AGENT_SPAWN_FAILED: Temporary failure spawning agent (retry with backoff)
//   - AGENT_NOT_READY: Agent exists but not in ACTIVE state (wait and retry)
//   - AGENT_MESSAGE_FAILED: Temporary failure sending message to agent (retry)
//   - INTERNAL_ERROR: Unexpected server error (retry with backoff, report if persistent)
//
// Error Mapping:
// The server.mapError() function centralizes error mapping from session layer to protocol layer:
//   - session.ErrSessionNotFound → SESSION_NOT_FOUND (non-recoverable)
//   - session.ErrAgentNotFound → AGENT_NOT_FOUND (non-recoverable)
//   - ValidationError → preserves code and recoverability from validation layer
//   - Unknown errors → INTERNAL_ERROR (recoverable, allows retry)

// BaseMessage contains fields common to all protocol messages
type BaseMessage struct {
	Version string `json:"version"`
	Type    string `json:"type"`
}

// ConnectionEstablishedMessage is sent when a WebSocket connection is established
type ConnectionEstablishedMessage struct {
	BaseMessage
	ServerID  string `json:"serverId"`
	Timestamp string `json:"timestamp"`
}

// ValidationError represents different types of validation failures
type ValidationError struct {
	Code        string
	Message     string
	Recoverable bool
}

func (e ValidationError) Error() string {
	return e.Message
}

// parseMessage parses JSON into BaseMessage (pure function)
func parseMessage(data []byte) (BaseMessage, error) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return base, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return base, nil
}

// validateRequiredFields checks for required fields (pure function)
func validateRequiredFields(base BaseMessage) error {
	if base.Version == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: version",
			Recoverable: true,
		}
	}

	if base.Type == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: type",
			Recoverable: true,
		}
	}

	return nil
}

// validateVersion checks protocol version compatibility (pure function)
func validateVersion(version string) error {
	if version != ProtocolVersion {
		return ValidationError{
			Code:        "VERSION_MISMATCH",
			Message:     fmt.Sprintf("Protocol version %s not supported (server supports %s)", version, ProtocolVersion),
			Recoverable: false,
		}
	}
	return nil
}

// parseSessionCreateMessage parses JSON into SessionCreateMessage (pure function)
func parseSessionCreateMessage(data []byte) (SessionCreateMessage, error) {
	var msg SessionCreateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return msg, nil
}

// parseAgentSpawnMessage parses JSON into AgentSpawnMessage (pure function)
func parseAgentSpawnMessage(data []byte) (AgentSpawnMessage, error) {
	var msg AgentSpawnMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return msg, nil
}

// parseAgentMessageRequest parses JSON into AgentMessageRequest (pure function)
func parseAgentMessageRequest(data []byte) (AgentMessageRequest, error) {
	var msg AgentMessageRequest
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return msg, nil
}

// parseSessionEndMessage parses JSON into SessionEndMessage (pure function)
func parseSessionEndMessage(data []byte) (SessionEndMessage, error) {
	var msg SessionEndMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return msg, nil
}

// parseAgentTerminateMessage parses JSON into AgentTerminateMessage (pure function)
func parseAgentTerminateMessage(data []byte) (AgentTerminateMessage, error) {
	var msg AgentTerminateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
			Recoverable: true,
		}
	}
	return msg, nil
}

// validateSessionCreateMessage validates SessionCreateMessage has no additional requirements
// beyond base message validation (pure function)
func validateSessionCreateMessage(msg SessionCreateMessage) error {
	// SessionCreateMessage only needs base validation (version + type)
	// which is done by ValidateMessage
	return nil
}

// validateAgentSpawnMessage validates AgentSpawnMessage required fields (pure function)
func validateAgentSpawnMessage(msg AgentSpawnMessage) error {
	if msg.UserSessionID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: userSessionId",
			Recoverable: true,
		}
	}
	if msg.AgentID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: agentId",
			Recoverable: true,
		}
	}
	if msg.Workspace == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: workspace",
			Recoverable: true,
		}
	}
	return nil
}

// validateAgentMessageRequest validates AgentMessageRequest required fields (pure function)
func validateAgentMessageRequest(msg AgentMessageRequest) error {
	if msg.UserSessionID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: userSessionId",
			Recoverable: true,
		}
	}
	if msg.AgentID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: agentId",
			Recoverable: true,
		}
	}
	if msg.Content == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: content",
			Recoverable: true,
		}
	}
	return nil
}

// validateSessionEndMessage validates SessionEndMessage required fields (pure function)
func validateSessionEndMessage(msg SessionEndMessage) error {
	if msg.UserSessionID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: userSessionId",
			Recoverable: true,
		}
	}
	return nil
}

// validateAgentTerminateMessage validates AgentTerminateMessage required fields (pure function)
func validateAgentTerminateMessage(msg AgentTerminateMessage) error {
	if msg.UserSessionID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: userSessionId",
			Recoverable: true,
		}
	}
	if msg.AgentID == "" {
		return ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Missing required field: agentId",
			Recoverable: true,
		}
	}
	return nil
}

// ValidateMessage checks if a message has required fields and valid version
// Composes pure validation functions
func ValidateMessage(data []byte) error {
	base, err := parseMessage(data)
	if err != nil {
		return err
	}

	if err := validateRequiredFields(base); err != nil {
		return err
	}

	if err := validateVersion(base.Version); err != nil {
		return err
	}

	return nil
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// ErrorMessage is sent when an error occurs
type ErrorMessage struct {
	BaseMessage
	Error ErrorDetail `json:"error"`
}

// SessionCreateMessage is sent by PWA to create a new user session
type SessionCreateMessage struct {
	BaseMessage
}

// SessionCreatedMessage is sent by relay confirming session creation
type SessionCreatedMessage struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	Timestamp     string `json:"timestamp"`
}

// AgentSpawnMessage is sent by PWA to spawn an agent in a user session
type AgentSpawnMessage struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	AgentID       string `json:"agentId"`
	Workspace     string `json:"workspace"`
}

// AgentReadyMessage is sent by relay confirming agent is ready
type AgentReadyMessage struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	AgentID       string `json:"agentId"`
}

// AgentMessageRequest is sent by PWA to send a message to an agent
type AgentMessageRequest struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	AgentID       string `json:"agentId"`
	Content       string `json:"content"`
}

// AgentMessageResponse is sent by relay with agent's response
type AgentMessageResponse struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	AgentID       string `json:"agentId"`
	Content       string `json:"content"`
	Timestamp     string `json:"timestamp"`
}

// SessionEndMessage is sent by PWA to terminate all agents and end the session
type SessionEndMessage struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
}

// SessionEndedMessage is sent by relay confirming session termination
type SessionEndedMessage struct {
	BaseMessage
	UserSessionID    string `json:"userSessionId"`
	AgentsTerminated int    `json:"agentsTerminated"`
	CleanupStatus    string `json:"cleanupStatus"`
}

// AgentTerminateMessage is sent by PWA to terminate a specific agent
type AgentTerminateMessage struct {
	BaseMessage
	UserSessionID string `json:"userSessionId"`
	AgentID       string `json:"agentId"`
}

// AgentTerminatedMessage is sent by relay confirming agent termination
type AgentTerminatedMessage struct {
	BaseMessage
	UserSessionID    string `json:"userSessionId"`
	AgentID          string `json:"agentId"`
	WorkspaceCleaned bool   `json:"workspaceCleaned"`
}

// NewConnectionEstablished creates a connection established message (pure function)
func NewConnectionEstablished(serverID, timestamp string) ConnectionEstablishedMessage {
	return ConnectionEstablishedMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "connection:established",
		},
		ServerID:  serverID,
		Timestamp: timestamp,
	}
}

// NewErrorMessage creates an error message
func NewErrorMessage(code, message string, recoverable bool) ErrorMessage {
	return ErrorMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "error",
		},
		Error: ErrorDetail{
			Code:        code,
			Message:     message,
			Recoverable: recoverable,
		},
	}
}

// NewSessionCreatedMessage creates a session created message (pure function)
func NewSessionCreatedMessage(userSessionID, timestamp string) SessionCreatedMessage {
	return SessionCreatedMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "session:created",
		},
		UserSessionID: userSessionID,
		Timestamp:     timestamp,
	}
}

// NewAgentReadyMessage creates an agent ready message (pure function)
func NewAgentReadyMessage(userSessionID, agentID string) AgentReadyMessage {
	return AgentReadyMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "agent:ready",
		},
		UserSessionID: userSessionID,
		AgentID:       agentID,
	}
}

// NewAgentMessageResponse creates an agent response message (pure function)
func NewAgentMessageResponse(userSessionID, agentID, content, timestamp string) AgentMessageResponse {
	return AgentMessageResponse{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "agent:response",
		},
		UserSessionID: userSessionID,
		AgentID:       agentID,
		Content:       content,
		Timestamp:     timestamp,
	}
}

// NewSessionEndedMessage creates a session ended message (pure function)
func NewSessionEndedMessage(userSessionID string, agentsTerminated int, cleanupStatus string) SessionEndedMessage {
	return SessionEndedMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "session:ended",
		},
		UserSessionID:    userSessionID,
		AgentsTerminated: agentsTerminated,
		CleanupStatus:    cleanupStatus,
	}
}

// NewAgentTerminatedMessage creates an agent terminated message (pure function)
func NewAgentTerminatedMessage(userSessionID, agentID string, workspaceCleaned bool) AgentTerminatedMessage {
	return AgentTerminatedMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "agent:terminated",
		},
		UserSessionID:    userSessionID,
		AgentID:          agentID,
		WorkspaceCleaned: workspaceCleaned,
	}
}
