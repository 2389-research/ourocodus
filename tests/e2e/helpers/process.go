package helpers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RelayServer manages the relay server process
type RelayServer struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// StartRelay starts the relay server as a background process
func StartRelay(ctx context.Context, binaryPath string, port string) (*RelayServer, error) {
	// Find project root
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	// Make binary path absolute if it's not already
	if !filepath.IsAbs(binaryPath) {
		binaryPath = filepath.Join(projectRoot, binaryPath)
	}

	fmt.Printf("[Server] Starting relay server: %s\n", binaryPath)
	fmt.Printf("[Server] Working directory: %s\n", projectRoot)
	fmt.Printf("[Server] Port: %s\n", port)

	// Create a context with cancel for the server process
	serverCtx, cancel := context.WithCancel(ctx)

	// Start the relay server
	cmd := exec.CommandContext(serverCtx, binaryPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%s", port),
		"WORKSPACE_BASE_DIR=./agent", // Configure relay to use ./agent instead of ./workspaces
	)

	// Set working directory to project root so relay can find ./web and ./agent
	cmd.Dir = projectRoot

	// Capture stdout/stderr for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start relay server: %w", err)
	}

	server := &RelayServer{
		cmd:    cmd,
		cancel: cancel,
	}

	// Wait for server to be ready (shorter timeout for faster failure)
	if err := server.WaitForHealth(fmt.Sprintf("http://localhost:%s", port), 10*time.Second); err != nil {
		if stopErr := server.Stop(); stopErr != nil {
			return nil, fmt.Errorf("relay server failed to become healthy: %w; additionally failed to stop cleanly: %v", err, stopErr)
		}
		return nil, fmt.Errorf("relay server failed to become healthy: %w", err)
	}

	return server, nil
}

// WaitForHealth polls the health endpoint until it responds or times out
func (s *RelayServer) WaitForHealth(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	// Try the root endpoint as a health check
	healthURL := baseURL

	attempts := 0
	for time.Now().Before(deadline) {
		attempts++
		resp, err := client.Get(healthURL)
		if err == nil {
			_ = resp.Body.Close() // Ignore close error in health check
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("[Health Check] Server is healthy after %d attempts\n", attempts)
				return nil
			}
			fmt.Printf("[Health Check] Attempt %d: Got status %d\n", attempts, resp.StatusCode)
		} else {
			fmt.Printf("[Health Check] Attempt %d: Error: %v\n", attempts, err)
		}

		// Wait a bit before retrying
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for relay server health at %s after %d attempts", healthURL, attempts)
}

// Stop gracefully stops the relay server
func (s *RelayServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()

		select {
		case <-time.After(5 * time.Second):
			// Force kill if graceful shutdown takes too long
			if err := s.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill relay process: %w", err)
			}
			return fmt.Errorf("relay process killed after timeout")
		case err := <-done:
			// Process exited
			if err != nil && err.Error() != "signal: killed" {
				return fmt.Errorf("relay process exited with error: %w", err)
			}
			return nil
		}
	}

	return nil
}

// FindProjectRoot finds the project root by looking for go.mod
func FindProjectRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root without finding go.mod
			return "", fmt.Errorf("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}

// BuildRelay compiles the relay binary
func BuildRelay(ctx context.Context, outputPath string) error {
	// Find project root
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Make output path absolute if it's not already
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(projectRoot, outputPath)
	}

	fmt.Printf("[Build] Building relay binary to %s\n", outputPath)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./cmd/relay")

	// Set working directory to project root
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build relay: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("[Build] Relay binary built successfully\n")
	return nil
}

// validateScriptPath validates that a script path is safe to execute
// It ensures the path:
// 1. Is within the project's scripts directory
// 2. Does not contain path traversal sequences
// 3. Is an absolute path (for security)
func validateScriptPath(scriptPath string) error {
	// Ensure path is absolute
	if !filepath.IsAbs(scriptPath) {
		return fmt.Errorf("script path must be absolute: %s", scriptPath)
	}

	// Clean the path to resolve any . or .. sequences
	cleanPath := filepath.Clean(scriptPath)

	// Find project root
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Construct the allowed scripts directory
	allowedDir := filepath.Join(projectRoot, "scripts")

	// Check if the cleaned path is within the allowed directory
	// Use filepath.Rel to ensure no path traversal escapes the allowed directory
	relPath, err := filepath.Rel(allowedDir, cleanPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If the relative path starts with "..", it's trying to escape the allowed directory
	if len(relPath) >= 2 && relPath[0:2] == ".." {
		return fmt.Errorf("script path must be within project scripts directory: %s", scriptPath)
	}

	// Verify the file exists (using Lstat to detect symlinks)
	info, err := os.Lstat(cleanPath)
	if err != nil {
		return fmt.Errorf("script file does not exist: %w", err)
	}

	// Reject symlinks explicitly (check before IsRegular since symlinks fail IsRegular)
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("script path must not be a symlink: %s", scriptPath)
	}

	// Verify it's a regular file (not a directory)
	if !info.Mode().IsRegular() {
		return fmt.Errorf("script path must be a regular file: %s", scriptPath)
	}

	return nil
}

// RunWorktreeSetup runs the worktree setup script
// The script path is validated to ensure it's within the project's scripts directory
func RunWorktreeSetup(ctx context.Context, scriptPath string) error {
	// Validate the script path for security
	if err := validateScriptPath(scriptPath); err != nil {
		return fmt.Errorf("invalid script path: %w", err)
	}

	cmd := exec.CommandContext(ctx, scriptPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run worktree setup: %w\nOutput: %s", err, string(output))
	}

	return nil
}
