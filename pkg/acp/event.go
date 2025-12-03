// pkg/acp/event.go
package acp

import (
	"encoding/json"
	"fmt"
)

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

// sessionUpdateNotification represents the JSON-RPC notification structure
type sessionUpdateNotification struct {
	Method string `json:"method"`
	Params struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	} `json:"params"`
}

// sessionUpdate represents the update payload
type sessionUpdate struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Message struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"message,omitempty"`
}

// ParseSessionUpdate parses a session/update notification into an Event
func ParseSessionUpdate(data []byte) (*Event, error) {
	var notif sessionUpdateNotification
	if err := json.Unmarshal(data, &notif); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	if notif.Method != "session/update" {
		return nil, fmt.Errorf("not a session/update notification: %s", notif.Method)
	}

	var update sessionUpdate
	if err := json.Unmarshal(notif.Params.Update, &update); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update: %w", err)
	}

	// Handle result type (session end)
	if update.Type == "result" {
		return &Event{Type: EventSessionEnd}, nil
	}

	// Handle assistant messages
	if update.Type == "assistant" {
		switch update.Message.Type {
		case "text":
			return &Event{
				Type: EventTextDelta,
				Text: update.Message.Text,
			}, nil
		case "tool_use":
			return &Event{
				Type:       EventToolCall,
				ToolCallID: update.Message.ID,
				ToolName:   update.Message.Name,
				ToolArgs:   update.Message.Input,
			}, nil
		case "tool_result":
			return &Event{
				Type:       EventToolResult,
				ToolCallID: update.Message.ID,
				ToolResult: update.Message.Input,
			}, nil
		}
	}

	// Unknown update type - return as-is for debugging
	return &Event{
		Type: EventTextDelta,
		Text: string(notif.Params.Update),
	}, nil
}
