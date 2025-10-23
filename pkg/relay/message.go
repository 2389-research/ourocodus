package relay

import (
	"encoding/json"
	"fmt"
)

const (
	// ProtocolVersion is the current WebSocket protocol version
	ProtocolVersion = "1.0"
)

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
