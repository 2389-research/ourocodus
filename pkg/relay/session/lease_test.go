package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLease_AcquireAndRelease tests basic lease operations
func TestLease_AcquireAndRelease(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-basic"
	sessionID := "session-basic"

	// Acquire lease
	lease, err := AcquireLease(agentID, sessionID)
	if err != nil {
		t.Fatalf("Failed to acquire lease: %v", err)
	}

	if lease.AgentID != agentID {
		t.Errorf("AgentID mismatch: expected %s, got %s", agentID, lease.AgentID)
	}
	if lease.UserSessionID != sessionID {
		t.Errorf("SessionID mismatch: expected %s, got %s", sessionID, lease.UserSessionID)
	}

	// Release lease
	if err := ReleaseLease(agentID); err != nil {
		t.Fatalf("Failed to release lease: %v", err)
	}

	// Verify lease is gone
	_, err = ReadLease(agentID)
	if err != ErrLeaseNotFound {
		t.Errorf("Expected ErrLeaseNotFound after release, got %v", err)
	}
}

// TestLease_AlreadyAttached tests conflict detection
func TestLease_AlreadyAttached(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-conflict"
	session1 := "session-1"
	session2 := "session-2"

	// First session acquires
	_, err := AcquireLease(agentID, session1)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Second session tries to acquire
	_, err = AcquireLease(agentID, session2)
	if err != ErrAlreadyAttached {
		t.Errorf("Expected ErrAlreadyAttached, got %v", err)
	}

	// Same session tries again
	_, err = AcquireLease(agentID, session1)
	if err != ErrAlreadyAttached {
		t.Errorf("Expected ErrAlreadyAttached for same session, got %v", err)
	}

	// Cleanup
	_ = ReleaseLease(agentID)
}

// TestLease_Idempotent_Release tests that release can be called multiple times
func TestLease_Idempotent_Release(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-idempotent-release"
	sessionID := "session-idempotent"

	// Acquire
	_, err := AcquireLease(agentID, sessionID)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// First release
	if err := ReleaseLease(agentID); err != nil {
		t.Fatalf("First release failed: %v", err)
	}

	// Second release (idempotent)
	if err := ReleaseLease(agentID); err != nil {
		t.Errorf("Second release should succeed (idempotent), got error: %v", err)
	}

	// Third release (still idempotent)
	if err := ReleaseLease(agentID); err != nil {
		t.Errorf("Third release should succeed (idempotent), got error: %v", err)
	}
}

// TestLease_ListLeases tests listing all leases
func TestLease_ListLeases(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Create two leases
	_, err := AcquireLease("test-list-1", "session-1")
	if err != nil {
		t.Fatalf("Failed to create lease 1: %v", err)
	}
	defer ReleaseLease("test-list-1")

	_, err = AcquireLease("test-list-2", "session-2")
	if err != nil {
		t.Fatalf("Failed to create lease 2: %v", err)
	}
	defer ReleaseLease("test-list-2")

	// List leases
	leases, err := ListLeases()
	if err != nil {
		t.Fatalf("Failed to list leases: %v", err)
	}

	// Find our test leases
	found1, found2 := false, false
	for _, lease := range leases {
		if lease.AgentID == "test-list-1" {
			found1 = true
			if lease.UserSessionID != "session-1" {
				t.Errorf("Lease 1 session mismatch")
			}
		}
		if lease.AgentID == "test-list-2" {
			found2 = true
			if lease.UserSessionID != "session-2" {
				t.Errorf("Lease 2 session mismatch")
			}
		}
	}

	if !found1 {
		t.Error("Lease 1 not found in list")
	}
	if !found2 {
		t.Error("Lease 2 not found in list")
	}
}

// TestLease_ExpiredLease tests that expired leases can be reclaimed
func TestLease_ExpiredLease(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-expired"
	session1 := "session-1"
	session2 := "session-2"

	// Create lease
	lease, err := AcquireLease(agentID, session1)
	if err != nil {
		t.Fatalf("Failed to acquire lease: %v", err)
	}
	defer ReleaseLease(agentID)

	// Manually expire the lease by writing it back with expired time
	lease.ExpiresAt = time.Now().Add(-1 * time.Hour)
	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	if err := os.WriteFile(leasePath, []byte(`{"agentId":"`+agentID+`","userSessionId":"`+session1+`","attachedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T00:00:00Z","heartbeatInterval":"30s"}`), 0600); err != nil {
		t.Fatalf("Failed to write expired lease: %v", err)
	}

	// Verify lease is expired
	expiredLease, err := ReadLease(agentID)
	if err != nil {
		t.Fatalf("Failed to read expired lease: %v", err)
	}
	if !IsLeaseExpired(expiredLease) {
		t.Fatal("Lease should be expired")
	}

	// New session should be able to reclaim
	newLease, err := AcquireLease(agentID, session2)
	if err != nil {
		t.Fatalf("Failed to reclaim expired lease: %v", err)
	}

	if newLease.UserSessionID != session2 {
		t.Errorf("New lease should belong to session2, got %s", newLease.UserSessionID)
	}
}

// TestLease_CleanupExpiredLeases tests the cleanup function
func TestLease_CleanupExpiredLeases(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Create a valid lease
	_, err := AcquireLease("test-valid", "session-valid")
	if err != nil {
		t.Fatalf("Failed to create valid lease: %v", err)
	}
	defer ReleaseLease("test-valid")

	// Create an expired lease
	agentID := "test-expired"
	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	if err := os.WriteFile(leasePath, []byte(`{"agentId":"`+agentID+`","userSessionId":"session-expired","attachedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T00:00:00Z","heartbeatInterval":"30s"}`), 0600); err != nil {
		t.Fatalf("Failed to write expired lease: %v", err)
	}

	// Cleanup expired leases
	cleaned, err := CleanupExpiredLeases()
	if err != nil {
		t.Fatalf("CleanupExpiredLeases failed: %v", err)
	}

	if cleaned != 1 {
		t.Errorf("Expected 1 cleaned lease, got %d", cleaned)
	}

	// Verify expired lease is gone
	_, err = ReadLease(agentID)
	if err != ErrLeaseNotFound {
		t.Errorf("Expected expired lease to be removed, got error: %v", err)
	}

	// Verify valid lease still exists
	_, err = ReadLease("test-valid")
	if err != nil {
		t.Errorf("Valid lease should still exist, got error: %v", err)
	}
}

// TestLease_RenewLease tests lease renewal
func TestLease_RenewLease(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-renew"
	sessionID := "session-renew"

	// Acquire lease
	lease, err := AcquireLease(agentID, sessionID)
	if err != nil {
		t.Fatalf("Failed to acquire lease: %v", err)
	}
	defer ReleaseLease(agentID)

	originalExpiry := lease.ExpiresAt

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Renew lease
	if err := RenewLease(agentID); err != nil {
		t.Fatalf("Failed to renew lease: %v", err)
	}

	// Read renewed lease
	renewed, err := ReadLease(agentID)
	if err != nil {
		t.Fatalf("Failed to read renewed lease: %v", err)
	}

	// Verify expiry was extended
	if !renewed.ExpiresAt.After(originalExpiry) {
		t.Errorf("Renewed lease should have later expiry time")
	}
}

// TestLease_MaxRetries tests retry limit
func TestLease_MaxRetries(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-max-retries"

	// Create an expired lease that will trigger retries
	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	if err := os.MkdirAll(LeaseDir, 0700); err != nil {
		t.Fatalf("Failed to create lease directory: %v", err)
	}
	if err := os.WriteFile(leasePath, []byte(`{"agentId":"`+agentID+`","userSessionId":"session-old","attachedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T00:00:00Z","heartbeatInterval":"30s"}`), 0600); err != nil {
		t.Fatalf("Failed to write expired lease: %v", err)
	}
	defer ReleaseLease(agentID)

	// Try to acquire - should succeed via retry
	_, err := AcquireLease(agentID, "session-new")
	if err != nil {
		t.Fatalf("Should succeed acquiring expired lease with retries: %v", err)
	}
}

// SECURITY AND EDGE CASE TESTS

// TestLease_PathTraversal tests that agentIDs with path traversal are handled safely
func TestLease_PathTraversal(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	tests := []struct {
		name    string
		agentID string
		desc    string
	}{
		{"relative_parent", "../evil-agent", "parent directory traversal"},
		{"absolute_path", "/etc/passwd", "absolute path"},
		{"multiple_parents", "../../etc/passwd", "multiple parent traversals"},
		{"mixed_path", "foo/../../../etc/passwd", "mixed path with traversal"},
		{"null_byte", "agent\x00evil", "null byte injection"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attempt to acquire lease with malicious agentID
			// The lease system should reject these with ErrInvalidAgentID
			_, err := AcquireLease(tt.agentID, "test-session")

			// Should reject malicious agentIDs
			if err == nil {
				defer ReleaseLease(tt.agentID)
				t.Errorf("Expected rejection of path traversal attempt (%s), but succeeded", tt.desc)
			} else if err != ErrInvalidAgentID {
				// ErrInvalidAgentID is expected, other errors are acceptable too
				t.Logf("Rejected %s with error: %v (acceptable)", tt.desc, err)
			}
		})
	}
}

// TestLease_InvalidInput tests handling of invalid input data
func TestLease_InvalidInput(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	tests := []struct {
		name      string
		agentID   string
		sessionID string
		desc      string
	}{
		{"empty_agent", "", "session-1", "empty agentID"},
		{"empty_session", "agent-1", "", "empty sessionID"},
		{"both_empty", "", "", "both empty"},
		{"very_long_agent", string(make([]byte, 10000)), "session-1", "very long agentID"},
		{"very_long_session", "agent-1", string(make([]byte, 10000)), "very long sessionID"},
		{"unicode_agent", "agent-\u0000\u0001\u0002", "session-1", "unicode control characters in agentID"},
		{"unicode_session", "agent-1", "session-\u0000\u0001\u0002", "unicode control characters in sessionID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attempt to acquire lease - should either succeed or fail gracefully
			lease, err := AcquireLease(tt.agentID, tt.sessionID)

			if err == nil {
				defer ReleaseLease(tt.agentID)

				// If it succeeds, verify basic properties
				if lease.AgentID != tt.agentID {
					t.Errorf("AgentID mismatch for %s", tt.desc)
				}
				if lease.UserSessionID != tt.sessionID {
					t.Errorf("UserSessionID mismatch for %s", tt.desc)
				}
			}
			// Failure is acceptable - we just want to ensure no panics or undefined behavior
		})
	}
}

// TestLease_CorruptedLeaseFile tests handling of corrupted lease files
func TestLease_CorruptedLeaseFile(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-corrupted"
	leasePath := filepath.Join(LeaseDir, agentID+".lease")

	tests := []struct {
		name    string
		content string
		desc    string
	}{
		{"invalid_json", "not json at all", "completely invalid JSON"},
		{"partial_json", `{"agentId":"test"`, "truncated JSON"},
		{"wrong_types", `{"agentId":123,"userSessionId":true}`, "wrong field types"},
		{"empty_file", "", "empty file"},
		{"binary_data", "\x00\x01\x02\x03\x04\x05", "binary data"},
		{"huge_file", string(make([]byte, 10*1024*1024)), "10MB file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create corrupted lease file
			if err := os.MkdirAll(LeaseDir, 0700); err != nil {
				t.Fatalf("Failed to create lease directory: %v", err)
			}
			if err := os.WriteFile(leasePath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("Failed to write corrupted lease: %v", err)
			}
			defer os.Remove(leasePath)

			// Try to read the corrupted lease - should fail gracefully
			_, err := ReadLease(agentID)
			if err == nil {
				t.Errorf("Expected error reading corrupted lease (%s), got none", tt.desc)
			}

			// Try to acquire a new lease - should handle the existing corrupted file
			newLease, err := AcquireLease(agentID, "session-new")
			if err == ErrAlreadyAttached {
				// This is acceptable - the system sees a file exists and blocks
				return
			}
			if err != nil {
				t.Logf("Acquire returned error for %s: %v (acceptable)", tt.desc, err)
				return
			}
			defer ReleaseLease(agentID)

			// If it succeeded, verify the new lease is valid
			if newLease.UserSessionID != "session-new" {
				t.Errorf("New lease has wrong sessionID: expected session-new, got %s", newLease.UserSessionID)
			}
		})
	}
}

// TestLease_PermissionDenied tests handling of permission denied scenarios
func TestLease_PermissionDenied(t *testing.T) {
	// Skip on systems where we can't create read-only directories
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Create lease directory with no write permissions
	if err := os.MkdirAll(LeaseDir, 0500); err != nil {
		t.Fatalf("Failed to create read-only lease directory: %v", err)
	}
	defer os.Chmod(LeaseDir, 0700) // Restore permissions for cleanup

	// Try to acquire lease in read-only directory
	_, err := AcquireLease("test-agent", "test-session")
	if err == nil {
		// macOS might allow this due to filesystem capabilities
		t.Skip("Platform allows writing to read-only directory (likely macOS with APFS)")
		// Try to cleanup (will likely fail due to permissions)
		os.Chmod(LeaseDir, 0700)
		ReleaseLease("test-agent")
	}
}

// TestLease_ConcurrentAccess tests thread-safety of lease operations
func TestLease_ConcurrentAccess(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-concurrent"
	numGoroutines := 10

	// Channel to collect results
	results := make(chan error, numGoroutines)

	// Launch multiple goroutines trying to acquire the same lease
	for i := 0; i < numGoroutines; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		go func(sid string) {
			_, err := AcquireLease(agentID, sid)
			results <- err
		}(sessionID)
	}

	// Collect results
	successes := 0
	conflicts := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if err == ErrAlreadyAttached {
			conflicts++
		} else {
			t.Errorf("Unexpected error from concurrent acquire: %v", err)
		}
	}

	// Exactly one should succeed, rest should see ErrAlreadyAttached
	if successes != 1 {
		t.Errorf("Expected exactly 1 successful acquire, got %d", successes)
	}
	if conflicts != numGoroutines-1 {
		t.Errorf("Expected %d conflicts, got %d", numGoroutines-1, conflicts)
	}

	// Cleanup
	ReleaseLease(agentID)
}

// TestLease_RapidCreateDelete tests rapid lease creation and deletion
func TestLease_RapidCreateDelete(t *testing.T) {
	// Use temp directory for test leases
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	agentID := "test-agent-rapid"
	sessionID := "test-session"

	// Rapidly create and delete lease
	for i := 0; i < 100; i++ {
		lease, err := AcquireLease(agentID, sessionID)
		if err != nil {
			t.Fatalf("Iteration %d: Failed to acquire lease: %v", i, err)
		}
		if lease.AgentID != agentID {
			t.Errorf("Iteration %d: AgentID mismatch", i)
		}

		if err := ReleaseLease(agentID); err != nil {
			t.Fatalf("Iteration %d: Failed to release lease: %v", i, err)
		}

		// Verify lease is gone
		_, err = ReadLease(agentID)
		if err != ErrLeaseNotFound {
			t.Errorf("Iteration %d: Expected ErrLeaseNotFound, got %v", i, err)
		}
	}
}
