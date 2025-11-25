package theme

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

// GetLogo returns the Ourocodus ASCII art logo in the specified size
func GetLogo(size LogoSize) string {
	switch size {
	case LogoSmall:
		return ` ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄
▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘
▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄
▐  ▌▐  ▌▐ ▝▖▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▌
 ▙▟ ▝▄▄▘▐  ▘ ▙▟  ▚▄▘ ▙▟ ▐▄▞ ▝▄▄▘▝▄▟▘`

	case LogoMedium:
		return ` ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄
▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘
▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄
▐  ▌▐  ▌▐ ▝▖▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▌
 ▙▟ ▝▄▄▘▐  ▘ ▙▟  ▚▄▘ ▙▟ ▐▄▞ ▝▄▄▘▝▄▟▘

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
