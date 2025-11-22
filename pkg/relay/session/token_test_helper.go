package session

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
)

// generateTestAttachToken generates a test attach token for the given agent ID.
// This is a test-only helper that creates a token file in .agentd/session/
// so that AttachAgent() can verify it.
//
// Returns the generated token string.
func generateTestAttachToken(agentID string) (string, error) {
	// Generate 32 random bytes (256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	// Encode as base64url
	tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

	// Ensure session directory exists
	sessionDir := filepath.Join(".agentd", "session")
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return "", err
	}

	// Write token to file
	tokenPath := filepath.Join(sessionDir, agentID+".token")
	if err := os.WriteFile(tokenPath, []byte(tokenStr), 0600); err != nil {
		return "", err
	}

	return tokenStr, nil
}
