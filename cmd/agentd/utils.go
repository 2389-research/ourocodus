package main

import (
	"fmt"
	"time"
)

// formatDuration formats time.Duration as human-readable string with " ago" suffix
func formatDuration(d time.Duration) string {
	return formatDurationWithSuffix(d, " ago")
}

// formatDurationWithoutSuffix formats time.Duration as human-readable string without suffix
func formatDurationWithoutSuffix(d time.Duration) string {
	return formatDurationWithSuffix(d, "")
}

// formatDurationWithSuffix formats time.Duration as human-readable string with custom suffix
func formatDurationWithSuffix(d time.Duration, suffix string) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds%s", int(d.Seconds()), suffix)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%s", int(d.Minutes()), suffix)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%s", int(d.Hours()), suffix)
	}
	return fmt.Sprintf("%dd%s", int(d.Hours()/24), suffix)
}
