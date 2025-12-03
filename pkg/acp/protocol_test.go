// pkg/acp/protocol_test.go
package acp

import (
	"encoding/json"
	"testing"
)

func TestInitializeParams_Marshal(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: 1,
		ClientInfo: ClientInfo{
			Name:    "ourocodus",
			Version: "1.0",
		},
		Capabilities: map[string]any{},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"protocolVersion":1,"clientInfo":{"name":"ourocodus","version":"1.0"},"capabilities":{}}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}
}

func TestSessionNewParams_Marshal(t *testing.T) {
	params := SessionNewParams{
		Cwd:        "/workspace",
		MCPServers: []any{},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"cwd":"/workspace","mcpServers":[]}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}
}

func TestSessionNewResult_Unmarshal(t *testing.T) {
	data := `{"sessionId":"test-123","models":{},"modes":{}}`

	var result SessionNewResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if result.SessionID != "test-123" {
		t.Errorf("got sessionId %s, want test-123", result.SessionID)
	}
}
