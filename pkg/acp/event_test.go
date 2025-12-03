// pkg/acp/event_test.go
package acp

import (
	"encoding/json"
	"testing"
)

func TestEvent_TextDelta(t *testing.T) {
	event := Event{
		Type: EventTextDelta,
		Text: "Hello world",
	}

	if event.Type != EventTextDelta {
		t.Errorf("got type %s, want %s", event.Type, EventTextDelta)
	}
	if event.Text != "Hello world" {
		t.Errorf("got text %s, want Hello world", event.Text)
	}
}

func TestEvent_ToolCall(t *testing.T) {
	args := json.RawMessage(`{"file_path":"/src/main.go"}`)
	event := Event{
		Type:       EventToolCall,
		ToolCallID: "call-1",
		ToolName:   "Read",
		ToolArgs:   args,
	}

	if event.Type != EventToolCall {
		t.Errorf("got type %s, want %s", event.Type, EventToolCall)
	}
	if event.ToolName != "Read" {
		t.Errorf("got tool name %s, want Read", event.ToolName)
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		et   EventType
		want string
	}{
		{EventTextDelta, "text.delta"},
		{EventToolCall, "tool.call"},
		{EventToolResult, "tool.result"},
		{EventError, "error"},
		{EventSessionEnd, "session.end"},
	}

	for _, tt := range tests {
		if string(tt.et) != tt.want {
			t.Errorf("got %s, want %s", tt.et, tt.want)
		}
	}
}
