package containersession

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWorkspace(t *testing.T) {
	t.Run("creates workspace successfully", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "test-session-123"

		path, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Expected workspace directory to exist")
		}

		// Verify path is under base directory
		expectedPath := filepath.Join(baseDir, sessionID)
		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	})

	t.Run("prevents directory traversal with ..", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "../../../etc/passwd"

		_, err := PrepareWorkspace(baseDir, sessionID)
		if err == nil {
			t.Error("Expected error for directory traversal attempt")
		}

		if err != nil && err != ErrInvalidWorkspacePath {
			// Should wrap ErrInvalidWorkspacePath
			t.Logf("Error: %v", err)
		}
	})

	t.Run("handles absolute path in sessionID", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "/etc/passwd"

		// The function will clean this and make it relative
		// This test documents current behavior - absolute paths get cleaned
		path, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Logf("Got error (expected): %v", err)
		} else {
			// If no error, verify path is still under baseDir using filepath.Rel
			relPath, relErr := filepath.Rel(baseDir, path)
			if relErr != nil || (len(relPath) >= 2 && relPath[:2] == "..") {
				t.Error("Absolute path should still be constrained under baseDir")
			}
		}
	})

	t.Run("handles directory name with dots", func(t *testing.T) {
		baseDir := t.TempDir()
		// Directory name that looks suspicious but is actually valid
		sessionID := "..data"

		path, err := PrepareWorkspace(baseDir, sessionID)
		// This might error depending on validation - either outcome is acceptable
		if err != nil {
			t.Logf("Got error for suspicious but valid name: %v", err)
			return
		}

		// If no error, should be under base directory using filepath.Rel
		relPath, relErr := filepath.Rel(baseDir, path)
		if relErr != nil || (len(relPath) >= 2 && relPath[:2] == "..") {
			t.Errorf("Path %s should be under base directory %s", path, baseDir)
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "nested/sub/dir"

		path, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify all directories were created
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Expected nested workspace directory to exist")
		}
	})

	t.Run("idempotent - directory already exists", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "test-session"

		// Create once
		path1, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Fatalf("First creation failed: %v", err)
		}

		// Create again - should succeed
		path2, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Fatalf("Second creation failed: %v", err)
		}

		if path1 != path2 {
			t.Errorf("Paths should be identical: %s != %s", path1, path2)
		}
	})

	t.Run("directory has correct permissions", func(t *testing.T) {
		baseDir := t.TempDir()
		sessionID := "test-perms"

		path, err := PrepareWorkspace(baseDir, sessionID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check permissions
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat directory: %v", err)
		}

		// Should have 0700 permissions (owner-only)
		mode := info.Mode().Perm()
		if mode != 0o700 {
			t.Errorf("Expected permissions 0700, got %o", mode)
		}
	})
}

func TestCleanupWorkspace(t *testing.T) {
	t.Run("removes workspace directory", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-cleanup")

		// Create directory
		err := os.MkdirAll(workspacePath, 0o700)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Create a file inside
		testFile := filepath.Join(workspacePath, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Cleanup
		logger := &mockLogger{}
		err = CleanupWorkspace(workspacePath, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify directory is gone
		if _, err := os.Stat(workspacePath); !os.IsNotExist(err) {
			t.Error("Expected workspace directory to be removed")
		}
	})

	t.Run("idempotent - directory does not exist", func(t *testing.T) {
		baseDir := t.TempDir()
		nonExistentPath := filepath.Join(baseDir, "does-not-exist")

		logger := &mockLogger{}
		err := CleanupWorkspace(nonExistentPath, logger)
		if err != nil {
			t.Errorf("Expected no error for non-existent directory, got %v", err)
		}
	})

	t.Run("removes nested directories", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-nested")

		// Create nested structure
		nestedPath := filepath.Join(workspacePath, "sub1", "sub2")
		err := os.MkdirAll(nestedPath, 0o700)
		if err != nil {
			t.Fatalf("Failed to create nested directories: %v", err)
		}

		// Cleanup
		logger := &mockLogger{}
		err = CleanupWorkspace(workspacePath, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify entire tree is gone
		if _, err := os.Stat(workspacePath); !os.IsNotExist(err) {
			t.Error("Expected entire workspace tree to be removed")
		}
	})
}
