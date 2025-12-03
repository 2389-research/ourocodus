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
	if event.ToolCallID != "call-1" {
		t.Errorf("got tool call id %s, want call-1", event.ToolCallID)
	}
	if string(event.ToolArgs) != string(args) {
		t.Errorf("got tool args %s, want %s", event.ToolArgs, args)
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

func TestParseSessionUpdate_TextDelta(t *testing.T) {
	// Real session/update notification from claude-code-acp
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"assistant","message":{"type":"text","text":"Hello"}}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventTextDelta {
		t.Errorf("got type %s, want %s", event.Type, EventTextDelta)
	}
	if event.Text != "Hello" {
		t.Errorf("got text %s, want Hello", event.Text)
	}
}

func TestParseSessionUpdate_ToolCall(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"assistant","message":{"type":"tool_use","id":"call-1","name":"Read","input":{"file_path":"/src/main.go"}}}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventToolCall {
		t.Errorf("got type %s, want %s", event.Type, EventToolCall)
	}
	if event.ToolName != "Read" {
		t.Errorf("got tool name %s, want Read", event.ToolName)
	}
	if event.ToolCallID != "call-1" {
		t.Errorf("got tool call id %s, want call-1", event.ToolCallID)
	}
}

func TestParseSessionUpdate_SessionEnd(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"result","subtype":"success"}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventSessionEnd {
		t.Errorf("got type %s, want %s", event.Type, EventSessionEnd)
	}
}

func TestParseSessionUpdate_NotSessionUpdate(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"result":{}}`

	_, err := ParseSessionUpdate([]byte(raw))
	if err == nil {
		t.Error("expected error for non-session/update message")
	}
}
