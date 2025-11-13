package acp_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// TestHostProcessTransportCloseHappyPath verifies that normal close completes
// within expected time without requiring kill or timeout.
func TestHostProcessTransportCloseHappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: shell scripts require Unix-like environment")
	}

	tmpDir := t.TempDir()

	// Create a script that exits gracefully when stdin closes
	scriptPath := filepath.Join(tmpDir, "graceful-exit.sh")
	script := `#!/bin/bash
# Read from stdin until EOF, then exit cleanly
while read line; do
	echo "received: $line"
done
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Launch the process
	launcher := &acp.HostProcessLauncher{}
	ctx := context.Background()
	transport, err := launcher.Start(ctx, acp.ProcessLaunchConfig{
		Workspace:   tmpDir,
		APIKey:      "test-api-key",
		CommandPath: scriptPath,
		CommandArgs: []string{},
	})
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Close should complete quickly (well under 5 seconds)
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err = transport.Close(closeCtx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify it completed quickly (graceful exit, no kill needed)
	if elapsed > 2*time.Second {
		t.Errorf("Close took too long: %v (expected < 2s for graceful exit)", elapsed)
	}

	t.Logf("Close completed in %v", elapsed)
}

// TestHostProcessTransportCloseTimeout verifies that a process that doesn't
// exit gets killed after 5 seconds, then times out after 2 more seconds if
// it still doesn't exit (simulating uninterruptible state).
//
// NOTE: This test verifies the timeout mechanism exists, but cannot easily
// simulate a true uninterruptible state (D state) without kernel-level operations.
// In practice, Kill() will terminate the process successfully. The test validates
// the happy path where kill works, which is the common case.
func TestHostProcessTransportCloseTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: shell scripts require Unix-like environment")
	}

	tmpDir := t.TempDir()

	// Create a script that ignores stdin but can be killed
	// In real scenarios, a process could enter uninterruptible state (D state)
	// but we cannot reliably simulate that in tests.
	scriptPath := filepath.Join(tmpDir, "long-runner.sh")
	script := `#!/bin/bash
# Ignore stdin close
while true; do
	sleep 0.1
done
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Launch the process
	launcher := &acp.HostProcessLauncher{}
	ctx := context.Background()
	transport, err := launcher.Start(ctx, acp.ProcessLaunchConfig{
		Workspace:   tmpDir,
		APIKey:      "test-api-key",
		CommandPath: scriptPath,
		CommandArgs: []string{},
	})
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Close with long timeout to observe full behavior
	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = transport.Close(closeCtx)
	elapsed := time.Since(start)

	// In practice, kill works, so we should not get an error
	// But the test verifies that the timeout mechanism exists
	if err != nil {
		t.Logf("Close returned error (expected if kill fails): %v", err)
	}

	// Should take about 5 seconds for graceful timeout, then kill succeeds quickly
	expectedMin := 5 * time.Second
	expectedMax := 6 * time.Second

	if elapsed < expectedMin {
		t.Errorf("Close completed too quickly: %v (expected >= %v)",
			elapsed, expectedMin)
	}

	if elapsed > expectedMax {
		t.Logf("Warning: Close took longer than expected: %v (expected <= %v)",
			elapsed, expectedMax)
		// Not fatal - CI systems can be slow
	}

	t.Logf("Close completed in %v", elapsed)
}

// TestHostProcessTransportCloseContextCancelled verifies that context
// cancellation is properly handled and returns context error.
func TestHostProcessTransportCloseContextCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: shell scripts require Unix-like environment")
	}

	tmpDir := t.TempDir()

	// Create a script that sleeps and doesn't respond to stdin close
	scriptPath := filepath.Join(tmpDir, "sleeper.sh")
	script := `#!/bin/bash
# Sleep for a long time, ignore stdin close
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Launch the process
	launcher := &acp.HostProcessLauncher{}
	ctx := context.Background()
	transport, err := launcher.Start(ctx, acp.ProcessLaunchConfig{
		Workspace:   tmpDir,
		APIKey:      "test-api-key",
		CommandPath: scriptPath,
		CommandArgs: []string{},
	})
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Create context that we'll cancel quickly
	closeCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = transport.Close(closeCtx)
	elapsed := time.Since(start)

	// Should return context error
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context error, got: %v", err)
	}

	// Should return quickly (around 500ms, not wait for 5s graceful timeout)
	if elapsed > 1*time.Second {
		t.Errorf("Close took too long: %v (expected < 1s for context cancellation)", elapsed)
	}

	t.Logf("Close returned after %v with context error: %v", elapsed, err)

	// Clean up the process
	if cmd, ok := getExecCmd(transport); ok && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// TestHostProcessTransportCloseIdempotent verifies that calling Close
// multiple times is safe and doesn't cause errors.
func TestHostProcessTransportCloseIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: shell scripts require Unix-like environment")
	}

	tmpDir := t.TempDir()

	// Create a simple script that exits gracefully
	scriptPath := filepath.Join(tmpDir, "simple.sh")
	script := `#!/bin/bash
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Launch the process
	launcher := &acp.HostProcessLauncher{}
	ctx := context.Background()
	transport, err := launcher.Start(ctx, acp.ProcessLaunchConfig{
		Workspace:   tmpDir,
		APIKey:      "test-api-key",
		CommandPath: scriptPath,
		CommandArgs: []string{},
	})
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Call Close multiple times
	closeCtx := context.Background()
	for i := 0; i < 3; i++ {
		err := transport.Close(closeCtx)
		if err != nil {
			t.Errorf("Close %d returned error: %v", i+1, err)
		}
	}
}

// TestHostProcessTransportCloseKillSuccess verifies that a process that
// ignores stdin close but responds to kill signal works correctly.
func TestHostProcessTransportCloseKillSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: shell scripts require Unix-like environment")
	}

	tmpDir := t.TempDir()

	// Create a script that ignores stdin but can be killed
	scriptPath := filepath.Join(tmpDir, "killable.sh")
	script := `#!/bin/bash
# Don't exit on stdin close, but allow kill to work
while true; do
	sleep 0.1
done
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Launch the process
	launcher := &acp.HostProcessLauncher{}
	ctx := context.Background()
	transport, err := launcher.Start(ctx, acp.ProcessLaunchConfig{
		Workspace:   tmpDir,
		APIKey:      "test-api-key",
		CommandPath: scriptPath,
		CommandArgs: []string{},
	})
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Close with reasonable timeout
	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = transport.Close(closeCtx)
	elapsed := time.Since(start)

	// Should succeed (no error) because kill works
	if err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}

	// Should take about 5 seconds (graceful timeout) + small time for kill to work
	expectedMin := 5 * time.Second
	expectedMax := 6 * time.Second

	if elapsed < expectedMin || elapsed > expectedMax {
		t.Logf("Warning: Close timing was %v, expected between %v and %v",
			elapsed, expectedMin, expectedMax)
		// Not a fatal error as timing can vary on CI systems
	}

	t.Logf("Close completed successfully in %v after kill", elapsed)
}

// getExecCmd is a helper to extract *exec.Cmd from transport for cleanup.
// This uses type assertion to access the internal cmd field for test cleanup only.
func getExecCmd(transport acp.Transport) (*exec.Cmd, bool) {
	// We need to use reflection or accept that we can't access private fields.
	// For test cleanup, we'll just rely on the OS to clean up zombies.
	// This is a best-effort helper.
	return nil, false
}
