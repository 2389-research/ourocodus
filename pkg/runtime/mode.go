package runtime

import (
	"fmt"
	"os"
)

// GetACPRuntimeMode reads and validates the OUROCODUS_ACP_RUNTIME environment variable.
// Returns "host" (default), "container", or an error for invalid values.
//
// This is used throughout the codebase to determine whether ACP should run:
// - "host": ACP runs on the host machine (default)
// - "container": ACP runs inside the agent Docker container
func GetACPRuntimeMode() (string, error) {
	mode := os.Getenv("OUROCODUS_ACP_RUNTIME")
	if mode == "" {
		return "host", nil // default
	}
	if mode == "host" || mode == "container" {
		return mode, nil
	}
	return "", fmt.Errorf("invalid OUROCODUS_ACP_RUNTIME value: %q (must be 'host' or 'container')", mode)
}

// IsContainerMode returns true if the runtime mode is set to "container".
// Returns false for "host" mode (default) or if the environment variable is invalid.
func IsContainerMode() bool {
	mode, _ := GetACPRuntimeMode()
	return mode == "container"
}
