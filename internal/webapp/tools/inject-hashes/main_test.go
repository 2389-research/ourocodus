package main

import (
	"encoding/json"
	"testing"
)

func TestInjectHashes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		manifest AssetManifest
		expected string
	}{
		{
			name: "inject hashed CSS with href",
			input: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.css">
</head>
</html>`,
			manifest: AssetManifest{
				"styles.css": "styles.abc12345.css",
			},
			expected: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.abc12345.css">
</head>
</html>`,
		},
		{
			name: "inject hashed JS with src",
			input: `<!DOCTYPE html>
<html>
<body>
    <script src="app.js"></script>
</body>
</html>`,
			manifest: AssetManifest{
				"app.js": "app.def67890.js",
			},
			expected: `<!DOCTYPE html>
<html>
<body>
    <script src="app.def67890.js"></script>
</body>
</html>`,
		},
		{
			name: "preserve multiple assets",
			input: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.min.css">
    <link rel="stylesheet" href="tailwind.css">
</head>
<body>
    <script src="logger.js"></script>
    <script src="app.js"></script>
</body>
</html>`,
			manifest: AssetManifest{
				"styles.min.css": "styles.min.55e70520.css",
				"tailwind.css":   "tailwind.fc562ecc.css",
				"logger.js":      "logger.54809809.js",
				"app.js":         "app.493c9c44.js",
			},
			expected: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.min.55e70520.css">
    <link rel="stylesheet" href="tailwind.fc562ecc.css">
</head>
<body>
    <script src="logger.54809809.js"></script>
    <script src="app.493c9c44.js"></script>
</body>
</html>`,
		},
		{
			name: "handle query parameters",
			input: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.css?v=1">
</head>
</html>`,
			manifest: AssetManifest{
				"styles.css": "styles.abc12345.css",
			},
			expected: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="styles.abc12345.css">
</head>
</html>`,
		},
		{
			name: "update already-hashed filenames",
			input: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="app.abc12345.css">
</head>
</html>`,
			manifest: AssetManifest{
				"app.css": "app.def67890.css",
			},
			expected: `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="app.def67890.css">
</head>
</html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectHashes(tt.input, tt.manifest)
			if result != tt.expected {
				t.Errorf("injectHashes() failed\nInput:\n%s\n\nExpected:\n%s\n\nGot:\n%s", tt.input, tt.expected, result)
			}

			// Regression check: ensure attributes are preserved
			if !contains(result, `href="`) && contains(tt.expected, `href="`) {
				t.Error("Output is missing href attributes")
			}
			if !contains(result, `src="`) && contains(tt.expected, `src="`) {
				t.Error("Output is missing src attributes")
			}
		})
	}
}

func TestInjectHashesPreservesAttributes(t *testing.T) {
	// Specific regression test for the bug where attributes were stripped
	input := `<link rel="stylesheet" href="styles.min.css">`
	manifest := AssetManifest{
		"styles.min.css": "styles.min.55e70520.css",
	}

	result := injectHashes(input, manifest)

	// Must contain the full attribute with proper quoting
	if !contains(result, `href="styles.min.55e70520.css"`) {
		t.Errorf("Expected output to contain full href attribute, got: %s", result)
	}

	// Must NOT contain malformed output like `.min.55e70520.css""`
	if contains(result, `.min.55e70520.css""`) {
		t.Error("Output contains malformed double quotes")
	}

	// Must NOT strip attribute names
	if !contains(result, `href=`) {
		t.Error("Output is missing href attribute name")
	}
}

func TestManifestParsing(t *testing.T) {
	manifestJSON := []byte(`{
		"app.js": "app.493c9c44.js",
		"logger.js": "logger.54809809.js",
		"styles.min.css": "styles.min.55e70520.css",
		"tailwind.css": "tailwind.fc562ecc.css"
	}`)

	var manifest AssetManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatalf("Failed to parse manifest: %v", err)
	}

	expected := map[string]string{
		"app.js":         "app.493c9c44.js",
		"logger.js":      "logger.54809809.js",
		"styles.min.css": "styles.min.55e70520.css",
		"tailwind.css":   "tailwind.fc562ecc.css",
	}

	for original, hashed := range expected {
		if manifest[original] != hashed {
			t.Errorf("Manifest[%s] = %s, want %s", original, manifest[original], hashed)
		}
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
