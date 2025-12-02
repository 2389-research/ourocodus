// Package format provides utility functions for formatting output in CLI applications.
// It includes functions for formatting durations, container IDs, session IDs, and paths
// in a consistent and human-readable manner.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// FormatDuration formats time.Duration as human-readable string with " ago" suffix.
// Examples: "5s ago", "2m ago", "3h ago", "7d ago"
func FormatDuration(d time.Duration) string {
	return FormatDurationWithSuffix(d, " ago")
}

// FormatDurationShort formats time.Duration as human-readable string without suffix.
// Examples: "5s", "2m", "3h", "7d"
func FormatDurationShort(d time.Duration) string {
	return FormatDurationWithSuffix(d, "")
}

// FormatDurationWithSuffix formats time.Duration as human-readable string with custom suffix.
// Negative durations are converted to positive values.
// The function uses milliseconds for durations < 1s, seconds for < 1m,
// minutes for < 1h, hours for < 24h, and days for longer durations.
func FormatDurationWithSuffix(d time.Duration, suffix string) string {
	if d < 0 {
		d = -d
	}

	if d < time.Second {
		return fmt.Sprintf("%dms%s", d.Milliseconds(), suffix)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs%s", d.Seconds(), suffix)
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm%s", d.Minutes(), suffix)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh%s", d.Hours(), suffix)
	}
	return fmt.Sprintf("%dd%s", int(d.Hours()/24), suffix)
}

// FormatDurationHuman formats time.Duration as a friendly human-readable string.
// Very recent times (< 1 minute) are shown as "just now".
// Examples: "just now", "5m ago", "3h ago", "7d ago"
func FormatDurationHuman(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// FormatContainerID truncates a container ID to 12 characters for display.
// This matches Docker's convention of showing container IDs in a short format.
// If the ID is already 12 characters or shorter, it is returned unchanged.
func FormatContainerID(containerID string) string {
	if len(containerID) > 12 {
		return containerID[:12]
	}
	return containerID
}

// FormatPath truncates a path to fit within a max width.
// If the path is longer than maxWidth, it is truncated from the beginning
// and prefixed with "..." to indicate truncation.
func FormatPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

// FormatSessionID truncates session ID for display.
// Session IDs longer than 16 characters are truncated to 13 characters
// with "..." appended to indicate truncation.
func FormatSessionID(sessionID string) string {
	if len(sessionID) <= 16 {
		return sessionID
	}
	return sessionID[:13] + "..."
}

// FormatAge formats a time.Time as a relative age string (e.g., "5m ago", "2h ago").
// Returns "–" for zero time values.
func FormatAge(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	return FormatDurationHuman(time.Since(t))
}

// FormatAgeCompact formats a time.Time as a compact relative age (e.g., "5m", "2h").
// Similar to FormatAge but without "ago" suffix for space-constrained displays.
// Returns "–" for zero time values, "now" for < 1 minute.
func FormatAgeCompact(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatLastBeat formats a heartbeat timestamp for display.
// Uses compact format without "ago" suffix (e.g., "5s", "2m").
// Returns "now" for times less than 1 second ago.
// Returns "–" for zero time values.
func FormatLastBeat(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	d := time.Since(t)
	if d < time.Second {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatLastBeatVerbose formats a heartbeat timestamp with "ago" suffix.
// Returns "now" for times less than 1 second ago.
// Returns "–" for zero time values.
func FormatLastBeatVerbose(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	d := time.Since(t)
	if d < time.Second {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// FormatTimestamp formats a time as HH:MM:SS for log display.
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}

// JSONHighlightColors defines colors for JSON syntax highlighting.
type JSONHighlightColors struct {
	Key     lipgloss.Color // Color for object keys
	String  lipgloss.Color // Color for string values
	Number  lipgloss.Color // Color for numeric values
	Bool    lipgloss.Color // Color for boolean values (true/false)
	Null    lipgloss.Color // Color for null values
	Bracket lipgloss.Color // Color for brackets, braces, colons, commas
}

// DefaultJSONColors returns default syntax highlighting colors.
func DefaultJSONColors() JSONHighlightColors {
	return JSONHighlightColors{
		Key:     lipgloss.Color("#88C0D0"), // Light blue for keys
		String:  lipgloss.Color("#A3BE8C"), // Green for strings
		Number:  lipgloss.Color("#B48EAD"), // Purple for numbers
		Bool:    lipgloss.Color("#EBCB8B"), // Yellow for booleans
		Null:    lipgloss.Color("#BF616A"), // Red for null
		Bracket: lipgloss.Color("#D8DEE9"), // Light gray for punctuation
	}
}

// IsJSON checks if a string is valid JSON (object or array).
func IsJSON(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Quick check: must start with { or [
	if s[0] != '{' && s[0] != '[' {
		return false
	}
	return json.Valid([]byte(s))
}

// jsonStyles holds pre-computed lipgloss styles for JSON highlighting.
type jsonStyles struct {
	key     lipgloss.Style
	str     lipgloss.Style
	number  lipgloss.Style
	boolean lipgloss.Style
	null    lipgloss.Style
	bracket lipgloss.Style
}

// newJSONStyles creates styles from colors.
func newJSONStyles(colors *JSONHighlightColors) jsonStyles {
	return jsonStyles{
		key:     lipgloss.NewStyle().Foreground(colors.Key),
		str:     lipgloss.NewStyle().Foreground(colors.String),
		number:  lipgloss.NewStyle().Foreground(colors.Number),
		boolean: lipgloss.NewStyle().Foreground(colors.Bool),
		null:    lipgloss.NewStyle().Foreground(colors.Null),
		bracket: lipgloss.NewStyle().Foreground(colors.Bracket),
	}
}

// jsonPatterns holds compiled regex patterns for JSON token matching.
type jsonPatterns struct {
	key     *regexp.Regexp // Matches "key":
	str     *regexp.Regexp // Matches "string"
	number  *regexp.Regexp // Matches numbers
	boolean *regexp.Regexp // Matches true/false
	null    *regexp.Regexp // Matches null
	bracket *regexp.Regexp // Matches {}[]:,
}

// newJSONPatterns compiles and returns all JSON token patterns.
func newJSONPatterns() jsonPatterns {
	return jsonPatterns{
		key:     regexp.MustCompile(`"([^"\\]|\\.)*"\s*:`),
		str:     regexp.MustCompile(`"([^"\\]|\\.)*"`),
		number:  regexp.MustCompile(`-?\d+\.?\d*([eE][+-]?\d+)?`),
		boolean: regexp.MustCompile(`\b(true|false)\b`),
		null:    regexp.MustCompile(`\bnull\b`),
		bracket: regexp.MustCompile(`[{}\[\]:,]`),
	}
}

// tokenMatch represents a matched token with its length and rendered output.
type tokenMatch struct {
	length int    // Number of characters consumed
	output string // Rendered/styled output
}

// tryMatchKey attempts to match a JSON key pattern ("key":) at the current position.
// Returns nil if no match at position 0.
func tryMatchKey(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.key.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	match := s[:loc[1]]
	colonIdx := strings.LastIndex(match, ":")
	keyPart := match[:colonIdx]
	colonPart := match[colonIdx:]

	return &tokenMatch{
		length: loc[1],
		output: styles.key.Render(keyPart) + styles.bracket.Render(colonPart),
	}
}

// tryMatchString attempts to match a JSON string value at the current position.
// Returns nil if no match at position 0.
func tryMatchString(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.str.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	return &tokenMatch{
		length: loc[1],
		output: styles.str.Render(s[:loc[1]]),
	}
}

// tryMatchBoolean attempts to match true/false at the current position.
// Returns nil if no match at position 0.
func tryMatchBoolean(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.boolean.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	return &tokenMatch{
		length: loc[1],
		output: styles.boolean.Render(s[:loc[1]]),
	}
}

// tryMatchNull attempts to match null at the current position.
// Returns nil if no match at position 0.
func tryMatchNull(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.null.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	return &tokenMatch{
		length: loc[1],
		output: styles.null.Render(s[:loc[1]]),
	}
}

// tryMatchNumber attempts to match a number at the current position.
// Returns nil if no match at position 0.
func tryMatchNumber(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.number.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	return &tokenMatch{
		length: loc[1],
		output: styles.number.Render(s[:loc[1]]),
	}
}

// tryMatchBracket attempts to match a bracket or punctuation at the current position.
// Returns nil if no match at position 0.
func tryMatchBracket(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	loc := patterns.bracket.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return nil
	}

	return &tokenMatch{
		length: loc[1],
		output: styles.bracket.Render(s[:loc[1]]),
	}
}

// tryMatchToken attempts to match any JSON token at the current position.
// Returns nil if no token matches at position 0.
// Order matters: keys must be checked before strings to distinguish "key": from "value".
func tryMatchToken(s string, patterns jsonPatterns, styles jsonStyles) *tokenMatch {
	// Check key first (must come before string to handle "key": vs "value")
	if m := tryMatchKey(s, patterns, styles); m != nil {
		return m
	}
	if m := tryMatchString(s, patterns, styles); m != nil {
		return m
	}
	if m := tryMatchBoolean(s, patterns, styles); m != nil {
		return m
	}
	if m := tryMatchNull(s, patterns, styles); m != nil {
		return m
	}
	if m := tryMatchNumber(s, patterns, styles); m != nil {
		return m
	}
	if m := tryMatchBracket(s, patterns, styles); m != nil {
		return m
	}
	return nil
}

// prettyPrintJSON formats JSON with indentation for readability.
// Returns the original input if formatting fails.
func prettyPrintJSON(input string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(input), "", "  "); err != nil {
		return input
	}
	return buf.String()
}

// highlightTokens applies syntax highlighting to pre-formatted JSON.
func highlightTokens(formatted string, patterns jsonPatterns, styles jsonStyles) string {
	var result strings.Builder
	i := 0

	for i < len(formatted) {
		if m := tryMatchToken(formatted[i:], patterns, styles); m != nil {
			result.WriteString(m.output)
			i += m.length
		} else {
			// No token match - pass through whitespace/other characters
			result.WriteByte(formatted[i])
			i++
		}
	}

	return result.String()
}

// HighlightJSON applies syntax highlighting to a JSON string with pretty-printing.
// If the input is not valid JSON, it is returned unchanged.
// Colors parameter is optional; if nil, default colors are used.
func HighlightJSON(input string, colors *JSONHighlightColors) string {
	if !IsJSON(input) {
		return input
	}

	if colors == nil {
		c := DefaultJSONColors()
		colors = &c
	}

	formatted := prettyPrintJSON(input)
	if formatted == input && !IsJSON(formatted) {
		return input
	}

	styles := newJSONStyles(colors)
	patterns := newJSONPatterns()

	return highlightTokens(formatted, patterns, styles)
}

// HighlightJSONCompact applies syntax highlighting to a JSON string without pretty-printing.
// The JSON remains on a single line. If the input is not valid JSON, it is returned unchanged.
// Colors parameter is optional; if nil, default colors are used.
func HighlightJSONCompact(input string, colors *JSONHighlightColors) string {
	if !IsJSON(input) {
		return input
	}

	if colors == nil {
		c := DefaultJSONColors()
		colors = &c
	}

	styles := newJSONStyles(colors)
	patterns := newJSONPatterns()

	return highlightTokens(input, patterns, styles)
}
