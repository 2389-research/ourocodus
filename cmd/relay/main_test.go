package main

import (
	"strings"
	"testing"
)

// TestRedactNATSURL tests URL credential redaction for issue #217
func TestRedactNATSURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with username and password",
			input:    "nats://user:password@localhost:4222",
			expected: "nats://***:***@localhost:4222",
		},
		{
			name:     "URL with complex password",
			input:    "nats://admin:P@ssw0rd!@nats.example.com:4222",
			expected: "nats://***:***@nats.example.com:4222",
		},
		{
			name:     "URL without credentials",
			input:    "nats://localhost:4222",
			expected: "nats://localhost:4222",
		},
		{
			name:     "URL with hostname only",
			input:    "nats://nats.example.com",
			expected: "nats://nats.example.com",
		},
		{
			name:     "URL with token authentication",
			input:    "nats://token:secret_token_abc123@cluster.nats.io:4222",
			expected: "nats://***:***@cluster.nats.io:4222",
		},
		{
			name:     "URL with username only (no password)",
			input:    "nats://user@localhost:4222",
			expected: "nats://***:***@localhost:4222",
		},
		{
			name:     "URL with empty password",
			input:    "nats://user:@localhost:4222",
			expected: "nats://***:***@localhost:4222",
		},
		{
			name:     "URL with special characters in credentials",
			input:    "nats://user%40example:p%40ss%3Aword@server:4222",
			expected: "nats://***:***@server:4222",
		},
		{
			name:     "Malformed URL with credentials (missing //)",
			input:    "nats:admin:secret@host:4222",
			expected: "INVALID_NATS_URL",
		},
		{
			name:     "Short URL",
			input:    "nats://",
			expected: "nats://",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Multi-host HA configuration with credentials",
			input:    "nats://user:pass@host1:4222,nats://admin:secret@host2:4222",
			expected: "nats://***:***@host1:4222,nats://***:***@host2:4222",
		},
		{
			name:     "Multi-host HA with mixed credentials",
			input:    "nats://user:pass@host1:4222,nats://host2:4222,nats://admin:secret@host3:4222",
			expected: "nats://***:***@host1:4222,nats://host2:4222,nats://***:***@host3:4222",
		},
		{
			name:     "Multi-host HA with spaces",
			input:    "nats://user:pass@host1:4222, nats://admin:secret@host2:4222",
			expected: "nats://***:***@host1:4222,nats://***:***@host2:4222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactNATSURL(tt.input)
			if result != tt.expected {
				t.Errorf("redactNATSURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			// Verify that the original password is not in the output
			if tt.input != tt.expected {
				// Extract password portion (between : and @)
				credStart := len("nats://")
				atPos := -1
				for i := credStart; i < len(tt.input); i++ {
					if tt.input[i] == '@' {
						atPos = i
						break
					}
				}
				if atPos > credStart {
					// Find the password (after first :)
					for i := credStart; i < atPos; i++ {
						if tt.input[i] == ':' && i+1 < atPos {
							password := tt.input[i+1 : atPos]
							if len(password) > 0 && result != tt.input {
								// Verify password is not in redacted output
								// (unless the entire URL is unchanged)
								for j := 0; j < len(result)-len(password)+1; j++ {
									if result[j:j+len(password)] == password {
										t.Errorf("Password %q still visible in redacted URL: %q", password, result)
									}
								}
							}
							break
						}
					}
				}
			}
		})
	}
}

// TestRedactNATSURL_PreservesHostInformation tests that redaction preserves connectivity info
func TestRedactNATSURL_PreservesHostInformation(t *testing.T) {
	input := "nats://admin:secret@production.nats.example.com:4222"
	result := redactNATSURL(input)

	// Verify host and port are preserved
	if result != "nats://***:***@production.nats.example.com:4222" {
		t.Errorf("Host information not preserved: got %q", result)
	}

	// Verify credentials are not visible
	if result == input {
		t.Error("URL was not redacted")
	}

	// Verify "admin" and "secret" are not in output
	if strings.Contains(result, "admin") {
		t.Error("Username 'admin' still visible in redacted URL")
	}
	if strings.Contains(result, "secret") {
		t.Error("Password 'secret' still visible in redacted URL")
	}
}
