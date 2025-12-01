package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCodes(t *testing.T) {
	// Verify constants match expected values
	assert.Equal(t, 0, ExitSuccess)
	assert.Equal(t, 1, ExitError)
	assert.Equal(t, 2, ExitUsageError)
	assert.Equal(t, 3, ExitConfigError)
	assert.Equal(t, 4, ExitIOError)
	assert.Equal(t, 130, ExitInterrupted)
}

func TestUsageError(t *testing.T) {
	err := UsageError("invalid argument")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUsage))
	assert.Contains(t, err.Error(), "invalid argument")
}

func TestConfigError(t *testing.T) {
	err := ConfigError("missing config file")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrConfig))
	assert.Contains(t, err.Error(), "missing config file")
}

func TestIOError(t *testing.T) {
	err := IOError("network timeout")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrIO))
	assert.Contains(t, err.Error(), "network timeout")
}

func TestExitCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, ExitSuccess},
		{"generic error", errors.New("something went wrong"), ExitError},
		{"usage error", UsageError("bad args"), ExitUsageError},
		{"config error", ConfigError("bad config"), ExitConfigError},
		{"io error", IOError("network fail"), ExitIOError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := ExitCodeFromError(tt.err)
			assert.Equal(t, tt.expected, code)
		})
	}
}
