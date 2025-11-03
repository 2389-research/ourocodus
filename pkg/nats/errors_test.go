package nats

import (
	"errors"
	"testing"
)

// TestTransientError verifies transient error wrapper.
func TestTransientError(t *testing.T) {
	baseErr := errors.New("connection failed")
	err := WrapTransientError("publish", "test.subject", baseErr)

	// Check error message
	if err.Error() == "" {
		t.Error("Error() returned empty string")
	}

	// Check IsTransient using type assertion
	var te *TransientError
	if !errors.As(err, &te) {
		t.Fatal("error is not a *TransientError")
	}
	if !te.IsTransient() {
		t.Error("IsTransient() = false, want true")
	}

	// Check Unwrap
	if unwrapped := te.Unwrap(); unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	// Check errors.Is
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is(err, baseErr) = false, want true")
	}

	// Check global helper
	if !IsTransientError(err) {
		t.Error("IsTransientError(err) = false, want true")
	}
}

// TestPermanentError verifies permanent error wrapper.
func TestPermanentError(t *testing.T) {
	baseErr := errors.New("invalid configuration")
	err := WrapPermanentError("connect", "test.subject", baseErr)

	// Check error message
	if err.Error() == "" {
		t.Error("Error() returned empty string")
	}

	// Check IsPermanent using type assertion
	var pe *PermanentError
	if !errors.As(err, &pe) {
		t.Fatal("error is not a *PermanentError")
	}
	if !pe.IsPermanent() {
		t.Error("IsPermanent() = false, want true")
	}

	// Check Unwrap
	if unwrapped := pe.Unwrap(); unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	// Check errors.Is
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is(err, baseErr) = false, want true")
	}

	// Check global helper
	if !IsPermanentError(err) {
		t.Error("IsPermanentError(err) = false, want true")
	}
}

// TestPredefinedErrors verifies predefined error variables.
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrClientClosed", ErrClientClosed},
		{"ErrSubscriptionClosed", ErrSubscriptionClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() returned empty string", tt.name)
			}
		})
	}
}
