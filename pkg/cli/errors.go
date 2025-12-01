package cli

import (
	"errors"
	"fmt"
)

// Exit codes for CLI applications.
// These codes provide consistent error reporting across all ourocodus tools.
const (
	// ExitSuccess indicates the command completed successfully.
	ExitSuccess = 0
	// ExitError indicates a general error occurred.
	ExitError = 1
	// ExitUsageError indicates invalid usage or arguments.
	ExitUsageError = 2
	// ExitConfigError indicates a configuration error.
	ExitConfigError = 3
	// ExitIOError indicates a network or I/O error.
	ExitIOError = 4
	// ExitInterrupted indicates the command was interrupted (SIGINT).
	ExitInterrupted = 130
)

// Error types for categorizing errors.
var (
	// ErrUsage indicates invalid command usage or arguments.
	ErrUsage = errors.New("usage error")
	// ErrConfig indicates a configuration error.
	ErrConfig = errors.New("configuration error")
	// ErrIO indicates a network or I/O error.
	ErrIO = errors.New("I/O error")
)

// UsageError wraps an error as a usage error.
func UsageError(msg string) error {
	return fmt.Errorf("%w: %s", ErrUsage, msg)
}

// ConfigError wraps an error as a configuration error.
func ConfigError(msg string) error {
	return fmt.Errorf("%w: %s", ErrConfig, msg)
}

// IOError wraps an error as an I/O error.
func IOError(msg string) error {
	return fmt.Errorf("%w: %s", ErrIO, msg)
}

// ExitCodeFromError returns the appropriate exit code for an error.
// Returns ExitSuccess (0) if err is nil.
func ExitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	if errors.Is(err, ErrUsage) {
		return ExitUsageError
	}
	if errors.Is(err, ErrConfig) {
		return ExitConfigError
	}
	if errors.Is(err, ErrIO) {
		return ExitIOError
	}

	return ExitError
}
