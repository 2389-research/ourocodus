package session

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Errors related to attach token verification
var (
	ErrMissingAttachToken = errors.New("attach token is required")
	ErrInvalidAttachToken = errors.New("invalid attach token")
	ErrTokenFileNotFound  = errors.New("token file not found - agent may not exist or token was not generated")
)

// verifyAttachToken verifies that the provided token matches the stored token for the agent.
// Uses constant-time comparison to prevent timing attacks.
//
// Phase 4: Security Hardening
// - Tokens are generated during agent spawn and stored in .agentd/session/{agent-id}.token
// - This prevents unauthorized attachment to CLI-spawned agents
// - Constant-time comparison prevents timing side-channel attacks
func verifyAttachToken(agentID, providedToken string) error {
	// Validate inputs
	if strings.TrimSpace(providedToken) == "" {
		return ErrMissingAttachToken
	}
	if strings.TrimSpace(agentID) == "" {
		return fmt.Errorf("agentID cannot be empty")
	}

	// Read expected token from file
	tokenPath := filepath.Join(".agentd", "session", agentID+".token")
	expectedToken, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrTokenFileNotFound
		}
		return fmt.Errorf("failed to read token file: %w", err)
	}

	// Trim any whitespace from stored token
	expectedTokenStr := strings.TrimSpace(string(expectedToken))
	providedTokenStr := strings.TrimSpace(providedToken)

	// Constant-time comparison to prevent timing attacks
	// This ensures that comparing valid vs invalid tokens takes the same amount of time
	if subtle.ConstantTimeCompare([]byte(providedTokenStr), []byte(expectedTokenStr)) != 1 {
		return ErrInvalidAttachToken
	}

	return nil
}
