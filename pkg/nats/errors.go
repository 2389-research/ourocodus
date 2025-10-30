package nats

import (
	"errors"
	"fmt"
)

var (
	// ErrClientClosed is returned when operations are attempted on a closed client.
	ErrClientClosed = errors.New("client is closed")

	// ErrSubscriptionClosed is returned when operations are attempted on a closed subscription.
	ErrSubscriptionClosed = errors.New("subscription is closed")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")
)

// TransientError represents an error that may succeed if retried.
type TransientError struct {
	Op      string
	Subject string
	Err     error
}

// Error returns the error message.
func (e *TransientError) Error() string {
	if e.Subject != "" {
		return fmt.Sprintf("%s on subject %q: %v (transient)", e.Op, e.Subject, e.Err)
	}
	return fmt.Sprintf("%s: %v (transient)", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *TransientError) Unwrap() error {
	return e.Err
}

// IsTransient returns true if the error is transient.
func (e *TransientError) IsTransient() bool {
	return true
}

// PermanentError represents an error that will not succeed if retried.
type PermanentError struct {
	Op      string
	Subject string
	Err     error
}

// Error returns the error message.
func (e *PermanentError) Error() string {
	if e.Subject != "" {
		return fmt.Sprintf("%s on subject %q: %v (permanent)", e.Op, e.Subject, e.Err)
	}
	return fmt.Sprintf("%s: %v (permanent)", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *PermanentError) Unwrap() error {
	return e.Err
}

// IsPermanent returns true if the error is permanent.
func (e *PermanentError) IsPermanent() bool {
	return true
}

// WrapTransientError wraps an error as a transient error.
func WrapTransientError(op, subject string, err error) error {
	return &TransientError{
		Op:      op,
		Subject: subject,
		Err:     err,
	}
}

// WrapPermanentError wraps an error as a permanent error.
func WrapPermanentError(op, subject string, err error) error {
	return &PermanentError{
		Op:      op,
		Subject: subject,
		Err:     err,
	}
}

// IsTransientError checks if an error is transient.
func IsTransientError(err error) bool {
	var te *TransientError
	return errors.As(err, &te)
}

// IsPermanentError checks if an error is permanent.
func IsPermanentError(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}
