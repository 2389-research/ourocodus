package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AssetManifest maps original filenames to hashed filenames
type AssetManifest map[string]string

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <web-dir> <manifest-output>\n", os.Args[0])
		os.Exit(1)
	}

	webDir := os.Args[1]
	manifestPath := os.Args[2]

	// Files to hash
	filesToHash := []string{
		"app.js",
		"logger.js",
		"tailwind.css",
		"styles.min.css",
	}

	manifest := make(AssetManifest)

	for _, filename := range filesToHash {
		originalPath := filepath.Join(webDir, filename)

		// Check if file exists
		if _, err := os.Stat(originalPath); os.IsNotExist(err) {
			fmt.Printf("Skipping %s (not found)\n", filename)
			continue
		}

		// Read file and compute hash
		file, err := os.Open(originalPath) // nolint:gosec // G304: CLI tool operates on web asset files
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", filename, err)
			continue
		}

		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			fmt.Fprintf(os.Stderr, "Error hashing %s: %v\n", filename, err)
			continue
		}
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", filename, err)
		}

		// Get short hash (first 8 chars)
		hash := hex.EncodeToString(hasher.Sum(nil))[:8]

		// Generate hashed filename: app.js -> app.a3f2b9c8.js
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		hashedName := fmt.Sprintf("%s.%s%s", base, hash, ext)

		// Copy original file to hashed name
		hashedPath := filepath.Join(webDir, hashedName)
		if err := copyFile(originalPath, hashedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying %s: %v\n", filename, err)
			continue
		}

		// Add to manifest
		manifest[filename] = hashedName
		fmt.Printf("✓ %s -> %s\n", filename, hashedName)
	}

	// Write manifest
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating manifest: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(manifestPath, manifestJSON, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Manifest written to %s\n", manifestPath)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src) // nolint:gosec // G304: CLI tool copies asset files
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst) // nolint:gosec // G304: CLI tool creates asset files
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
