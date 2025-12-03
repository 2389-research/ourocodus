package session

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// TestExtractJSONRPCID tests the extractJSONRPCID function with various ID types.
func TestExtractJSONRPCID(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "string id",
			data:     []byte(`{"jsonrpc":"2.0","id":"req-123","result":{}}`),
			expected: "req-123",
		},
		{
			name:     "numeric id",
			data:     []byte(`{"jsonrpc":"2.0","id":42,"result":{}}`),
			expected: "42",
		},
		{
			name:     "null id",
			data:     []byte(`{"jsonrpc":"2.0","id":null,"result":{}}`),
			expected: "",
		},
		{
			name:     "missing id",
			data:     []byte(`{"jsonrpc":"2.0","result":{}}`),
			expected: "",
		},
		{
			name:     "invalid json",
			data:     []byte(`{not valid json`),
			expected: "",
		},
		{
			name:     "float numeric id",
			data:     []byte(`{"jsonrpc":"2.0","id":123.0,"result":{}}`),
			expected: "123",
		},
		{
			name:     "empty object",
			data:     []byte(`{}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONRPCID(tt.data)
			if got != tt.expected {
				t.Errorf("extractJSONRPCID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestACPBridge_GenerateRequestID tests that request IDs are unique and sequential.
func TestACPBridge_GenerateRequestID(t *testing.T) {
	bridge := &ACPBridge{}

	// Generate several IDs and verify they're unique and follow expected pattern
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := bridge.generateRequestID()
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %s", id)
		}
		ids[id] = true

		// Verify format: "req-N" where N is sequential
		if id[:4] != "req-" {
			t.Errorf("Unexpected ID format: %s", id)
		}
	}

	if len(ids) != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", len(ids))
	}
}

// TestACPBridge_GenerateRequestID_PerInstance tests that each bridge has its own counter.
func TestACPBridge_GenerateRequestID_PerInstance(t *testing.T) {
	bridge1 := &ACPBridge{}
	bridge2 := &ACPBridge{}

	// Both bridges should start their counters at 1
	id1 := bridge1.generateRequestID()
	id2 := bridge2.generateRequestID()

	if id1 != "req-1" {
		t.Errorf("Bridge 1 first ID = %s, want req-1", id1)
	}
	if id2 != "req-1" {
		t.Errorf("Bridge 2 first ID = %s, want req-1", id2)
	}

	// Incrementing one bridge shouldn't affect the other
	_ = bridge1.generateRequestID()
	_ = bridge1.generateRequestID()
	id1Next := bridge1.generateRequestID() // Should be req-4
	id2Next := bridge2.generateRequestID() // Should be req-2

	if id1Next != "req-4" {
		t.Errorf("Bridge 1 fourth ID = %s, want req-4", id1Next)
	}
	if id2Next != "req-2" {
		t.Errorf("Bridge 2 second ID = %s, want req-2", id2Next)
	}
}

// TestACPBridge_InitializeACP_Exists verifies the InitializeACP method signature exists and compiles.
func TestACPBridge_InitializeACP_Exists(t *testing.T) {
	// This test verifies the method signature exists and is type-safe
	var bridge *ACPBridge
	var _ func(context.Context) (*acp.InitializeResult, error) = bridge.InitializeACP
}
