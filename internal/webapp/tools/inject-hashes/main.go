package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// AssetManifest maps original filenames to hashed filenames
type AssetManifest map[string]string

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <manifest-path> <html-file>\n", os.Args[0])
		os.Exit(1)
	}

	manifestPath := os.Args[1]
	htmlPath := os.Args[2]

	// Read manifest
	manifestData, err := os.ReadFile(manifestPath) // nolint:gosec // G304: CLI tool takes file path as argument
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading manifest: %v\n", err)
		os.Exit(1)
	}

	var manifest AssetManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing manifest: %v\n", err)
		os.Exit(1)
	}

	// Read HTML file
	htmlContent, err := os.ReadFile(htmlPath) // nolint:gosec // G304: CLI tool takes file path as argument
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading HTML: %v\n", err)
		os.Exit(1)
	}

	html := string(htmlContent)
	updated := injectHashes(html, manifest)

	// Log replacements
	for original, hashed := range manifest {
		fmt.Printf("✓ %s -> %s\n", original, hashed)
	}

	// Write updated HTML
	if updated != html {
		// Create backup
		backupPath := htmlPath + ".bak"
		if err := os.WriteFile(backupPath, []byte(html), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating backup: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(htmlPath, []byte(updated), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Updated %s (backup: %s)\n", htmlPath, backupPath)
	} else {
		fmt.Printf("No changes needed in %s\n", filepath.Base(htmlPath))
	}
}

// injectHashes replaces asset references in HTML with their hashed equivalents.
// It handles:
// - Original filenames: src="app.js" -> src="app.abc12345.js"
// - Query parameters: href="styles.css?v=1" -> href="styles.abc12345.css"
// - Already-hashed files: src="app.old12345.js" -> src="app.new67890.js"
func injectHashes(html string, manifest AssetManifest) string {
	updated := html

	for original, hashed := range manifest {
		// Extract base name and extension
		ext := filepath.Ext(original)
		base := original[:len(original)-len(ext)]

		// Pattern 1: Original filename with optional query params
		// Matches: src="app.js?v=6" or href="styles.css"
		pattern1 := regexp.MustCompile(
			fmt.Sprintf(`((?:src|href)=")%s(?:\?[^"]*)?(")`, regexp.QuoteMeta(original)))
		updated = pattern1.ReplaceAllString(updated, fmt.Sprintf(`${1}%s${2}`, hashed))

		// Pattern 2: Already-hashed filename
		// Matches: src="app.abc123.js"
		pattern2 := regexp.MustCompile(
			fmt.Sprintf(`((?:src|href)=")%s\.[a-f0-9]{8}%s(")`, regexp.QuoteMeta(base), regexp.QuoteMeta(ext)))
		updated = pattern2.ReplaceAllString(updated, fmt.Sprintf(`${1}%s${2}`, hashed))
	}

	return updated
}
