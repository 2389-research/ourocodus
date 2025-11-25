package output

import (
	"fmt"
	"time"
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
func FormatContainerID(containerID string) string {
	if len(containerID) > 12 {
		return containerID[:12]
	}
	return containerID
}

// FormatPath truncates a path to fit within a max width.
func FormatPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

// FormatSessionID truncates session ID for display.
func FormatSessionID(sessionID string) string {
	if len(sessionID) <= 16 {
		return sessionID
	}
	return sessionID[:13] + "..."
}
