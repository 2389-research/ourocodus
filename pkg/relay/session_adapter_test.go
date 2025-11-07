package relay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkspaceBaseDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	t.Run("accepts valid paths", func(t *testing.T) {
		validPaths := []string{
			tempDir,
			filepath.Join(tempDir, "subdir"),
			"/tmp/workspace",
			"/home/user/workspace",
		}

		for _, path := range validPaths {
			t.Run(path, func(t *testing.T) {
				err := validateWorkspaceBaseDir(path)
				if err != nil {
					t.Errorf("Expected valid path %q to pass validation, got error: %v", path, err)
				}
			})
		}
	})

	t.Run("rejects dangerous system directories", func(t *testing.T) {
		dangerousPaths := []string{
			"/",
			"/etc",
			"/sys",
			"/proc",
			"/dev",
			"/root",
			"/boot",
			"/bin",
			"/sbin",
			"/usr",
			"/var",
		}

		for _, path := range dangerousPaths {
			t.Run(path, func(t *testing.T) {
				err := validateWorkspaceBaseDir(path)
				if err == nil {
					t.Errorf("Expected dangerous path %q to fail validation", path)
				}
			})
		}
	})

	t.Run("rejects paths with parent traversal sequences", func(t *testing.T) {
		traversalPaths := []string{
			"../etc",
			"/tmp/../etc",
			"/home/user/../../root",
			"workspace/../../../etc",
			"/tmp/workspace/../..",
		}

		for _, path := range traversalPaths {
			t.Run(path, func(t *testing.T) {
				err := validateWorkspaceBaseDir(path)
				if err == nil {
					t.Errorf("Expected traversal path %q to fail validation", path)
				}
			})
		}
	})

	t.Run("accepts subdirectories of dangerous paths", func(t *testing.T) {
		// These should be allowed - they're subdirectories, not the system dirs themselves
		allowedPaths := []string{
			"/tmp",     // Not in the dangerous list
			"/var/tmp", // Subdirectory of /var
			"/usr/local/workspace",
		}

		for _, path := range allowedPaths {
			t.Run(path, func(t *testing.T) {
				err := validateWorkspaceBaseDir(path)
				if err != nil {
					t.Errorf("Expected safe subdirectory %q to pass validation, got error: %v", path, err)
				}
			})
		}
	})

	t.Run("handles relative paths", func(t *testing.T) {
		// Relative paths without ".." should work
		relativePaths := []string{
			"workspace",
			"./workspace",
			"my/workspace/dir",
		}

		for _, path := range relativePaths {
			t.Run(path, func(t *testing.T) {
				// Change to temp directory for testing
				originalWd, err := os.Getwd()
				if err != nil {
					t.Fatalf("Failed to get working directory: %v", err)
				}
				defer os.Chdir(originalWd)

				if err := os.Chdir(tempDir); err != nil {
					t.Fatalf("Failed to change directory: %v", err)
				}

				err = validateWorkspaceBaseDir(path)
				if err != nil {
					t.Errorf("Expected relative path %q to pass validation, got error: %v", path, err)
				}
			})
		}
	})

	t.Run("rejects relative paths with traversal", func(t *testing.T) {
		// Relative paths with ".." should fail
		traversalPaths := []string{
			"../workspace",
			"..",
			"../../etc",
		}

		for _, path := range traversalPaths {
			t.Run(path, func(t *testing.T) {
				err := validateWorkspaceBaseDir(path)
				if err == nil {
					t.Errorf("Expected relative traversal path %q to fail validation", path)
				}
			})
		}
	})
}
