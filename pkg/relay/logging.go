package relay

import (
	"encoding/json"
	"regexp"
)

var unsafeCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9:/_-]`)

// extractMessageType safely extracts the "type" field from a JSON message.
// Returns "unknown" if the message is invalid JSON or missing the type field.
// The returned type is sanitized to prevent log injection.
func extractMessageType(data []byte) string {
	// Limit parsing to reasonable message size to avoid DoS
	if len(data) > 1024*1024 { // 1MB
		return "unknown_large"
	}

	var msg struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return "unknown"
	}

	if msg.Type == "" {
		return "unknown"
	}

	return sanitizeType(msg.Type, 48)
}

// sanitizeType removes unsafe characters and truncates to maxLen.
// This prevents log injection attacks and keeps logs readable.
func sanitizeType(s string, maxLen int) string {
	// Replace unsafe characters with underscores
	safe := unsafeCharsRegex.ReplaceAllString(s, "_")

	// Truncate if needed
	if len(safe) > maxLen {
		return safe[:maxLen]
	}

	return safe
}
