// Package cli provides common CLI error handling and exit code management for ourocodus tools.
//
// # Exit Code Conventions
//
// This package follows standard Unix/POSIX exit code conventions to ensure consistent
// error reporting across all ourocodus CLI tools:
//
//   - 0: Success
//   - 1: General/unspecified error
//   - 2: Usage/argument error (matches bash convention)
//   - 78: Configuration error (from BSD sysexits.h EX_CONFIG)
//   - 130: Interrupted by SIGINT (128 + signal 2, per POSIX)
//
// These conventions ensure that scripts and automation tools can reliably detect and
// categorize failures. Exit codes 64-78 follow the BSD sysexits.h standard for
// program error codes.
//
// Reference: https://man.freebsd.org/cgi/man.cgi?query=sysexits
package cli

import (
	"errors"
	"fmt"
)

// Exit codes for CLI applications.
// These codes provide consistent error reporting across all ourocodus tools.
const (
	// ExitSuccess indicates the command completed successfully.
	// All operations finished without errors.
	ExitSuccess = 0

	// ExitGenericError indicates an unspecified error occurred.
	// This is the default error code when a more specific code is not available.
	// Used for unexpected failures that don't fit other categories.
	ExitGenericError = 1

	// ExitUsageError indicates incorrect command usage.
	// Examples: invalid flags, missing required arguments, conflicting options.
	// This matches the bash/Unix convention for command line syntax errors.
	ExitUsageError = 2

	// ExitConfigError indicates a configuration problem.
	// Value 78 corresponds to EX_CONFIG from BSD sysexits.h.
	// Examples: missing configuration file, invalid settings, malformed config.
	ExitConfigError = 78

	// ExitIOError indicates a network or I/O error.
	// This is a custom extension to the standard exit codes.
	ExitIOError = 4

	// ExitInterrupted indicates the process was interrupted by SIGINT (Ctrl+C).
	// Calculated as 128 + signal number, per POSIX convention (SIGINT = 2, so 128 + 2 = 130).
	// This allows shell scripts to detect user interruption vs program errors.
	ExitInterrupted = 130

	// ExitContextError indicates the CLI context was not available.
	// This is a programming error - commands must be wrapped by cli.App.
	ExitContextError = 70 // EX_SOFTWARE from sysexits.h
)

// Error types for categorizing errors.
var (
	// ErrUsage indicates invalid command usage or arguments.
	ErrUsage = errors.New("usage error")
	// ErrConfig indicates a configuration error.
	ErrConfig = errors.New("configuration error")
	// ErrIO indicates a network or I/O error.
	ErrIO = errors.New("I/O error")
	// ErrContext indicates CLI context is not available.
	ErrContext = errors.New("context error")
)

// ContextError returns a typed error for missing CLI context.
// This is the standard error to return when cli.FromContext() returns nil.
func ContextError() error {
	return fmt.Errorf("%w: cli context not available - ensure cli.App is wrapping your command", ErrContext)
}

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
	if errors.Is(err, ErrContext) {
		return ExitContextError
	}

	return ExitGenericError
}
