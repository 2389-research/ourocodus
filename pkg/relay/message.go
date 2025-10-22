package relay

import (
	"encoding/json"
	"fmt"
	"time"
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

// ValidateMessage checks if a message has required fields and valid version
func ValidateMessage(data []byte) error {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if base.Version == "" {
		return fmt.Errorf("missing required field: version")
	}

	if base.Version != ProtocolVersion {
		return fmt.Errorf("protocol version %s not supported (server supports %s)", base.Version, ProtocolVersion)
	}

	return nil
}

// NewConnectionEstablished creates a connection established message
func NewConnectionEstablished(serverID string) ConnectionEstablishedMessage {
	return ConnectionEstablishedMessage{
		BaseMessage: BaseMessage{
			Version: ProtocolVersion,
			Type:    "connection:established",
		},
		ServerID:  serverID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
