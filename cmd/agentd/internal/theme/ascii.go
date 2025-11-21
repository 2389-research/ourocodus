package theme

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// LogoSize represents the size variant of the logo
type LogoSize int

const (
	LogoSmall LogoSize = iota
	LogoMedium
	LogoLarge
)

// AgentStatus represents the status of an agent
type AgentStatus string

const (
	StatusRunning AgentStatus = "running"
	StatusPaused  AgentStatus = "paused"
	StatusStopped AgentStatus = "stopped"
	StatusIdle    AgentStatus = "idle"
)

// MessageCategory represents types of vintage messages
type MessageCategory int

const (
	MsgConnecting MessageCategory = iota
	MsgSuccess
	MsgError
	MsgLoading
)

// GetLogo returns the Ourocodus ASCII art logo in the specified size
func GetLogo(size LogoSize) string {
	switch size {
	case LogoSmall:
		return `┌────────────────────────────────┐
│                                │
│ ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄ │
│▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘│
│▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄ │
│▐  ▌▐  ▌▐ ▝▖▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▌│
│ ▙▟ ▝▄▄▘▐  ▘ ▙▟  ▚▄▘ ▙▟ ▐▄▞ ▝▄▄▘▝▄▟▘│
│                                │
│                                │
└────────────────────────────────┘`

	case LogoMedium:
		return `┌────────────────────────────────┐
│                                │
│ ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄ │
│▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘│
│▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄ │
│▐  ▌▐  ▌▐ ▝▖▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▌│
│ ▙▟ ▝▄▄▘▐  ▘ ▙▟  ▚▄▘ ▙▟ ▐▄▞ ▝▄▄▘▝▄▟▘│
│                                │
│                                │
└────────────────────────────────┘
   Multi-Agent Coordination Platform`

	case LogoLarge:
		return `┌────────────────────────────────┐
│                                │
│ ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄ │
│▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘│
│▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄ │
│▐  ▌▐  ▌▐ ▝▖▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▌│
│ ▙▟ ▝▄▄▘▐  ▘ ▙▟  ▚▄▘ ▙▟ ▐▄▞ ▝▄▄▘▝▄▟▘│
│                                │
│                                │
└────────────────────────────────┘

       🐉 Multi-Agent Coordination Platform 🐉
    Git Worktrees • Docker Isolation • NATS Messaging`

	default:
		return GetLogo(LogoMedium)
	}
}

// GetAgentStatusIcon returns an emoji/symbol for the agent status
// When unicode is false, returns ASCII-safe fallback characters
func GetAgentStatusIcon(status AgentStatus, unicode bool) string {
	if !unicode {
		// ASCII fallbacks for terminals without unicode support
		switch status {
		case StatusRunning:
			return ">"
		case StatusPaused:
			return "||"
		case StatusStopped:
			return "X"
		case StatusIdle:
			return "~"
		default:
			return "?"
		}
	}

	// Unicode emoji for modern terminals
	switch status {
	case StatusRunning:
		return "⚡"
	case StatusPaused:
		return "⏸"
	case StatusStopped:
		return "✗"
	case StatusIdle:
		return "💤"
	default:
		return "?"
	}
}

// DrawBox draws a box with title and content using box-drawing characters
func DrawBox(title, content string, width int) string {
	var sb strings.Builder

	// Top border
	sb.WriteString("┌")
	if title != "" {
		titlePart := fmt.Sprintf("─ %s ", title)
		sb.WriteString(titlePart)
		remaining := width - len(titlePart) - 2
		if remaining > 0 {
			sb.WriteString(strings.Repeat("─", remaining))
		}
	} else {
		sb.WriteString(strings.Repeat("─", width-2))
	}
	sb.WriteString("┐\n")

	// Content
	for _, line := range strings.Split(content, "\n") {
		sb.WriteString("│ ")
		sb.WriteString(line)
		padding := width - len(line) - 4
		if padding > 0 {
			sb.WriteString(strings.Repeat(" ", padding))
		}
		sb.WriteString(" │\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteString("┘")

	return sb.String()
}

// DrawHeader draws a header with double-line borders
func DrawHeader(text string) string {
	width := len(text) + 8
	var sb strings.Builder

	sb.WriteString("╔")
	sb.WriteString(strings.Repeat("═", width-2))
	sb.WriteString("╗\n")

	sb.WriteString("║ ")
	sb.WriteString(strings.Repeat(" ", (width-len(text)-4)/2))
	sb.WriteString(text)
	sb.WriteString(strings.Repeat(" ", (width-len(text)-3)/2))
	sb.WriteString(" ║\n")

	sb.WriteString("╚")
	sb.WriteString(strings.Repeat("═", width-2))
	sb.WriteString("╝")

	return sb.String()
}

// GetVintageMessage returns a random vintage-style message for the category
func GetVintageMessage(category MessageCategory) string {
	messages := map[MessageCategory][]string{
		MsgConnecting: {
			"INITIALIZING PROTOCOLS",
			"CARRIER DETECTED",
			"ESTABLISHING LINK",
			"HANDSHAKE IN PROGRESS",
		},
		MsgSuccess: {
			"SYNC COMPLETE",
			"OPERATION NOMINAL",
			"TRANSFER SUCCESSFUL",
			"ACKNOWLEDGED",
		},
		MsgError: {
			"FAULT DETECTED",
			"PROTOCOL VIOLATION",
			"TRANSMISSION ERROR",
			"SYSTEM MALFUNCTION",
		},
		MsgLoading: {
			"LOADING DATASTREAM",
			"BUFFERING",
			"PROCESSING REQUEST",
			"STANDBY",
		},
	}

	choices := messages[category]
	if len(choices) == 0 {
		return "SYSTEM READY"
	}

	// #nosec G404 -- Using math/rand/v2 for non-cryptographic random message selection
	return choices[rand.IntN(len(choices))]
}
