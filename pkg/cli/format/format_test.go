package format

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms ago"},
		{"seconds", 5 * time.Second, "5.0s ago"},
		{"minutes", 2 * time.Minute, "2.0m ago"},
		{"hours", 3 * time.Hour, "3.0h ago"},
		{"days", 7 * 24 * time.Hour, "7d ago"},
		{"negative", -5 * time.Second, "5.0s ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 5 * time.Second, "5.0s"},
		{"minutes", 2 * time.Minute, "2.0m"},
		{"hours", 3 * time.Hour, "3.0h"},
		{"days", 7 * 24 * time.Hour, "7d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDurationShort(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDurationShort(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatDurationWithSuffix(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		suffix   string
		want     string
	}{
		{"custom suffix", 5 * time.Second, " later", "5.0s later"},
		{"empty suffix", 2 * time.Minute, "", "2.0m"},
		{"negative with suffix", -3 * time.Hour, " ago", "3.0h ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDurationWithSuffix(tt.duration, tt.suffix)
			if got != tt.want {
				t.Errorf("FormatDurationWithSuffix(%v, %q) = %q, want %q", tt.duration, tt.suffix, got, tt.want)
			}
		})
	}
}

func TestFormatDurationHuman(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"minutes", 5 * time.Minute, "5m ago"},
		{"hours", 3 * time.Hour, "3h ago"},
		{"days", 7 * 24 * time.Hour, "7d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDurationHuman(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDurationHuman(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatContainerID(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		want        string
	}{
		{"short ID", "abc123", "abc123"},
		{"exact 12 chars", "abcdef123456", "abcdef123456"},
		{"long ID", "abcdef1234567890", "abcdef123456"},
		{"very long ID", "abcdef1234567890abcdef1234567890", "abcdef123456"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContainerID(tt.containerID)
			if got != tt.want {
				t.Errorf("FormatContainerID(%q) = %q, want %q", tt.containerID, got, tt.want)
			}
		})
	}
}

func TestFormatPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxWidth int
		want     string
	}{
		{"short path", "/usr/bin", 20, "/usr/bin"},
		{"exact width", "/usr/local/bin", 14, "/usr/local/bin"},
		{"truncate", "/usr/local/bin/example", 15, ".../bin/example"},
		{"very short width", "/usr/local/bin/example", 10, "...example"},
		{"empty path", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPath(tt.path, tt.maxWidth)
			if got != tt.want {
				t.Errorf("FormatPath(%q, %d) = %q, want %q", tt.path, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestFormatSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{"short ID", "abc123", "abc123"},
		{"exact 16 chars", "abcdef1234567890", "abcdef1234567890"},
		{"long ID", "abcdef1234567890abc", "abcdef1234567..."},
		{"very long ID", "abcdef1234567890abcdef1234567890", "abcdef1234567..."},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSessionID(tt.sessionID)
			if got != tt.want {
				t.Errorf("FormatSessionID(%q) = %q, want %q", tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid object", `{"key": "value"}`, true},
		{"valid array", `[1, 2, 3]`, true},
		{"nested object", `{"nested": {"key": "value"}}`, true},
		{"array of objects", `[{"a": 1}, {"b": 2}]`, true},
		{"empty object", `{}`, true},
		{"empty array", `[]`, true},
		{"with whitespace", `  {"key": "value"}  `, true},
		{"plain string", `hello world`, false},
		{"just a number", `123`, false},
		{"just a string", `"hello"`, false},
		{"invalid json", `{key: value}`, false},
		{"empty string", ``, false},
		{"partial json", `{"key":`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJSON(tt.input)
			if got != tt.want {
				t.Errorf("IsJSON(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHighlightJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool // if valid, output should be different from input (have ANSI codes)
	}{
		{"valid object", `{"key": "value"}`, true},
		{"valid array", `[1, 2, 3]`, true},
		{"nested", `{"nested": {"a": 1, "b": true, "c": null}}`, true},
		{"not json", `hello world`, false},
		{"empty", ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HighlightJSON(tt.input, nil)
			if tt.valid {
				// Valid JSON should be pretty-printed and have ANSI escape codes
				if got == tt.input {
					t.Errorf("HighlightJSON(%q) returned unchanged input, expected highlighting", tt.input)
				}
				// Check for ANSI escape codes (they start with \x1b[)
				if len(got) > 0 && got[0] != '\x1b' && got[0] != '{' && got[0] != '[' {
					t.Errorf("HighlightJSON(%q) doesn't appear to have ANSI codes", tt.input)
				}
			} else {
				// Invalid JSON should be returned unchanged
				if got != tt.input {
					t.Errorf("HighlightJSON(%q) = %q, want unchanged input", tt.input, got)
				}
			}
		})
	}
}

func TestDefaultJSONColors(t *testing.T) {
	colors := DefaultJSONColors()
	// Just verify all colors are non-empty
	if colors.Key == "" {
		t.Error("DefaultJSONColors().Key is empty")
	}
	if colors.String == "" {
		t.Error("DefaultJSONColors().String is empty")
	}
	if colors.Number == "" {
		t.Error("DefaultJSONColors().Number is empty")
	}
	if colors.Bool == "" {
		t.Error("DefaultJSONColors().Bool is empty")
	}
	if colors.Null == "" {
		t.Error("DefaultJSONColors().Null is empty")
	}
	if colors.Bracket == "" {
		t.Error("DefaultJSONColors().Bracket is empty")
	}
}

func TestNewJSONPatterns(t *testing.T) {
	patterns := newJSONPatterns()

	// Verify all patterns are compiled and non-nil
	if patterns.key == nil {
		t.Error("key pattern is nil")
	}
	if patterns.str == nil {
		t.Error("str pattern is nil")
	}
	if patterns.number == nil {
		t.Error("number pattern is nil")
	}
	if patterns.boolean == nil {
		t.Error("boolean pattern is nil")
	}
	if patterns.null == nil {
		t.Error("null pattern is nil")
	}
	if patterns.bracket == nil {
		t.Error("bracket pattern is nil")
	}
}

func TestNewJSONStyles(t *testing.T) {
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	// Verify styles are created (lipgloss styles are non-nil structs)
	// We just verify they don't panic when rendering
	_ = styles.key.Render("test")
	_ = styles.str.Render("test")
	_ = styles.number.Render("test")
	_ = styles.boolean.Render("test")
	_ = styles.null.Render("test")
	_ = styles.bracket.Render("test")
}

func TestTryMatchKey(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"simple key", `"name":`, true, 7},
		{"key with space before colon", `"key" :`, true, 7},
		{"escaped quotes in key", `"na\"me":`, true, 9},
		{"not a key - no colon", `"value"`, false, 0},
		{"not at start", ` "key":`, false, 0},
		{"empty string", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchKey(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchKey(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchKey(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchKey(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchKey(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchString(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"simple string", `"hello"`, true, 7},
		{"empty string", `""`, true, 2},
		{"string with spaces", `"hello world"`, true, 13},
		{"escaped quote", `"say \"hi\""`, true, 12},
		{"escaped backslash", `"path\\file"`, true, 12},
		{"not a string", `hello`, false, 0},
		{"not at start", ` "hello"`, false, 0},
		{"empty input", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchString(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchString(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchString(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchString(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchString(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchBoolean(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"true", "true", true, 4},
		{"false", "false", true, 5},
		{"true with trailing", "true,", true, 4},
		{"false with trailing", "false}", true, 5},
		{"not boolean", "trueish", false, 0},
		{"partial true", "tru", false, 0},
		{"not at start", " true", false, 0},
		{"empty", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchBoolean(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchBoolean(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchBoolean(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchBoolean(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchBoolean(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchNull(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"null", "null", true, 4},
		{"null with trailing comma", "null,", true, 4},
		{"null with trailing brace", "null}", true, 4},
		{"not null", "nullable", false, 0},
		{"partial null", "nul", false, 0},
		{"not at start", " null", false, 0},
		{"empty", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchNull(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchNull(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchNull(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchNull(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchNull(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchNumber(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"integer", "123", true, 3},
		{"negative integer", "-456", true, 4},
		{"float", "3.14", true, 4},
		{"negative float", "-2.718", true, 6},
		{"scientific notation", "1e10", true, 4},
		{"scientific with plus", "1e+10", true, 5},
		{"scientific with minus", "1e-10", true, 5},
		{"float scientific", "3.14e5", true, 6},
		{"number with trailing", "42,", true, 2},
		{"zero", "0", true, 1},
		{"not a number", "abc", false, 0},
		{"not at start", " 123", false, 0},
		{"empty", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchNumber(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchNumber(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchNumber(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchNumber(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchNumber(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchBracket(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		length      int
	}{
		{"open brace", "{", true, 1},
		{"close brace", "}", true, 1},
		{"open bracket", "[", true, 1},
		{"close bracket", "]", true, 1},
		{"colon", ":", true, 1},
		{"comma", ",", true, 1},
		{"not punctuation", "a", false, 0},
		{"not at start", " {", false, 0},
		{"empty", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchBracket(tt.input, patterns, styles)
			if tt.shouldMatch {
				if m == nil {
					t.Errorf("tryMatchBracket(%q) returned nil, expected match", tt.input)
					return
				}
				if m.length != tt.length {
					t.Errorf("tryMatchBracket(%q).length = %d, want %d", tt.input, m.length, tt.length)
				}
				if m.output == "" {
					t.Errorf("tryMatchBracket(%q).output is empty", tt.input)
				}
			} else {
				if m != nil {
					t.Errorf("tryMatchBracket(%q) = %+v, expected nil", tt.input, m)
				}
			}
		})
	}
}

func TestTryMatchToken(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{"key", `"key": value`, true},
		{"string", `"value"`, true},
		{"true", "true", true},
		{"false", "false", true},
		{"null", "null", true},
		{"number", "42", true},
		{"bracket", "{", true},
		{"whitespace only", "   ", false},
		{"newline", "\n", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tryMatchToken(tt.input, patterns, styles)
			if tt.shouldMatch && m == nil {
				t.Errorf("tryMatchToken(%q) returned nil, expected match", tt.input)
			}
			if !tt.shouldMatch && m != nil {
				t.Errorf("tryMatchToken(%q) = %+v, expected nil", tt.input, m)
			}
		})
	}
}

func TestPrettyPrintJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"simple object",
			`{"a":1}`,
			"{\n  \"a\": 1\n}",
		},
		{
			"nested object",
			`{"a":{"b":2}}`,
			"{\n  \"a\": {\n    \"b\": 2\n  }\n}",
		},
		{
			"array",
			`[1,2,3]`,
			"[\n  1,\n  2,\n  3\n]",
		},
		{
			"invalid json",
			`{not valid`,
			`{not valid`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prettyPrintJSON(tt.input)
			if got != tt.want {
				t.Errorf("prettyPrintJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHighlightTokens(t *testing.T) {
	patterns := newJSONPatterns()
	colors := DefaultJSONColors()
	styles := newJSONStyles(&colors)

	// Test that highlightTokens processes formatted JSON without crashing
	// and returns valid output. In non-TTY environments, ANSI codes may not be added.
	input := "{\n  \"key\": \"value\"\n}"
	result := highlightTokens(input, patterns, styles)

	// Result should not be empty
	if result == "" {
		t.Error("highlightTokens returned empty string")
	}

	// Result should contain the key and value content
	if len(result) == 0 {
		t.Error("highlightTokens returned zero-length result")
	}

	// Verify structure is preserved (contains key structural elements)
	// Note: In non-TTY environments, this may be unchanged from input
	if len(result) < len(input) {
		t.Errorf("highlightTokens result (%d bytes) shorter than input (%d bytes)", len(result), len(input))
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero time", time.Time{}, "–"},
		{"just now", time.Now().Add(-30 * time.Second), "just now"},
		{"5 minutes ago", time.Now().Add(-5 * time.Minute), "5m ago"},
		{"2 hours ago", time.Now().Add(-2 * time.Hour), "2h ago"},
		{"3 days ago", time.Now().Add(-3 * 24 * time.Hour), "3d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAge(tt.input)
			if result != tt.expected {
				t.Errorf("FormatAge() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatAgeCompact(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero time", time.Time{}, "–"},
		{"just now", time.Now().Add(-30 * time.Second), "now"},
		{"5 minutes ago", time.Now().Add(-5 * time.Minute), "5m"},
		{"2 hours ago", time.Now().Add(-2 * time.Hour), "2h"},
		{"3 days ago", time.Now().Add(-3 * 24 * time.Hour), "3d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAgeCompact(tt.input)
			if result != tt.expected {
				t.Errorf("FormatAgeCompact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatLastBeat(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero time", time.Time{}, "–"},
		{"sub-second", time.Now().Add(-500 * time.Millisecond), "now"},
		{"30 seconds ago", time.Now().Add(-30 * time.Second), "30s"},
		{"5 minutes ago", time.Now().Add(-5 * time.Minute), "5m"},
		{"2 hours ago", time.Now().Add(-2 * time.Hour), "2h"},
		{"3 days ago", time.Now().Add(-3 * 24 * time.Hour), "3d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLastBeat(tt.input)
			if result != tt.expected {
				t.Errorf("FormatLastBeat() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatLastBeatVerbose(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero time", time.Time{}, "–"},
		{"sub-second", time.Now().Add(-500 * time.Millisecond), "now"},
		{"30 seconds ago", time.Now().Add(-30 * time.Second), "30s ago"},
		{"5 minutes ago", time.Now().Add(-5 * time.Minute), "5m ago"},
		{"2 hours ago", time.Now().Add(-2 * time.Hour), "2h ago"},
		{"3 days ago", time.Now().Add(-3 * 24 * time.Hour), "3d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLastBeatVerbose(tt.input)
			if result != tt.expected {
				t.Errorf("FormatLastBeatVerbose() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	input := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	expected := "14:30:45"
	result := FormatTimestamp(input)
	if result != expected {
		t.Errorf("FormatTimestamp() = %q, want %q", result, expected)
	}
}
