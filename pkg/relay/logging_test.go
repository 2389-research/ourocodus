package relay

import (
	"testing"
)

func TestExtractMessageType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "valid message with type",
			input:    []byte(`{"type":"agent:spawn","payload":{"role":"echo"}}`),
			expected: "agent:spawn",
		},
		{
			name:     "valid JSON without type",
			input:    []byte(`{"action":"test","data":"value"}`),
			expected: "unknown",
		},
		{
			name:     "invalid JSON",
			input:    []byte(`{invalid json`),
			expected: "unknown",
		},
		{
			name:     "empty message",
			input:    []byte(``),
			expected: "unknown",
		},
		{
			name:     "type with special characters",
			input:    []byte(`{"type":"test<script>alert(1)</script>"}`),
			expected: "test_script_alert_1__/script_",
		},
		{
			name:     "very long type",
			input:    []byte(`{"type":"` + string(make([]byte, 100)) + `"}`),
			expected: "", // Will check length limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMessageType(tt.input)
			if tt.name == "very long type" {
				if len(result) > 48 {
					t.Errorf("expected type length <= 48, got %d", len(result))
				}
			} else if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "safe string",
			input:    "agent:spawn",
			maxLen:   48,
			expected: "agent:spawn",
		},
		{
			name:     "with special chars",
			input:    "test<script>",
			maxLen:   48,
			expected: "test_script_",
		},
		{
			name:     "truncates at maxLen",
			input:    "verylongtypename" + string(make([]byte, 50)),
			maxLen:   20,
			expected: "", // Will verify length <= 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeType(tt.input, tt.maxLen)
			if len(result) > tt.maxLen {
				t.Errorf("expected length <= %d, got %d", tt.maxLen, len(result))
			}
			if tt.name != "truncates at maxLen" && result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
