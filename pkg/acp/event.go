// pkg/acp/event.go
package acp

import "encoding/json"

// EventType represents the type of streaming event
type EventType string

// Event types for streaming responses
const (
	EventTextDelta  EventType = "text.delta"
	EventToolCall   EventType = "tool.call"
	EventToolResult EventType = "tool.result"
	EventError      EventType = "error"
	EventSessionEnd EventType = "session.end"
)

// Event represents a streaming event from ACP
type Event struct {
	Type       EventType       `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolArgs   json.RawMessage `json:"toolArgs,omitempty"`
	ToolResult json.RawMessage `json:"toolResult,omitempty"`
	Error      string          `json:"error,omitempty"`
}
