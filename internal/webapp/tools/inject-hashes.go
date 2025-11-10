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
	manifestData, err := os.ReadFile(manifestPath)
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
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading HTML: %v\n", err)
		os.Exit(1)
	}

	html := string(htmlContent)
	updated := html

	// Replace asset references in HTML
	// Pattern: src="file.js?v=X" or href="file.css?v=X" or src="file.oldhash.js"
	// Replace with: src="file.hash.js" or href="file.hash.css"

	for original, hashed := range manifest {
		// Extract base name and extension
		ext := filepath.Ext(original)
		base := original[:len(original)-len(ext)]

		// Match:
		// 1. Original filename with optional query params: app.js?v=6
		// 2. Already-hashed filename: app.abc123.js
		patterns := []string{
			// Original with optional query params
			fmt.Sprintf(`(src|href)="%s(\?[^"]*)?`, regexp.QuoteMeta(original)),
			// Already hashed version: basename.*.ext
			fmt.Sprintf(`(src|href)="%s\.[a-f0-9]{8}%s`, regexp.QuoteMeta(base), regexp.QuoteMeta(ext)),
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			replacement := fmt.Sprintf(`$1="%s"`, hashed)
			updated = re.ReplaceAllString(updated, replacement)
		}

		fmt.Printf("✓ %s -> %s\n", original, hashed)
	}

	// Write updated HTML
	if updated != html {
		// Create backup
		backupPath := htmlPath + ".bak"
		if err := os.WriteFile(backupPath, []byte(html), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating backup: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(htmlPath, []byte(updated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Updated %s (backup: %s)\n", htmlPath, backupPath)
	} else {
		fmt.Printf("No changes needed in %s\n", filepath.Base(htmlPath))
	}
}
