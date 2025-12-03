// pkg/acp/protocol.go
package acp

// Protocol method constants
const (
	MethodInitialize    = "initialize"
	MethodSessionNew    = "session/new"
	MethodSessionPrompt = "session/prompt"
)

// ClientInfo identifies the client to the ACP server
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams for the initialize handshake
type InitializeParams struct {
	ProtocolVersion int            `json:"protocolVersion"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
	Capabilities    map[string]any `json:"capabilities"`
}

// InitializeResult from the initialize handshake
type InitializeResult struct {
	ProtocolVersion   int            `json:"protocolVersion"`
	AgentCapabilities map[string]any `json:"agentCapabilities,omitempty"`
	AgentInfo         AgentInfo      `json:"agentInfo"`
}

// AgentInfo describes the ACP agent
type AgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// SessionNewParams for creating a new session
type SessionNewParams struct {
	Cwd        string `json:"cwd"`
	MCPServers []any  `json:"mcpServers"`
}

// SessionNewResult from creating a new session
type SessionNewResult struct {
	SessionID string         `json:"sessionId"`
	Models    map[string]any `json:"models,omitempty"`
	Modes     map[string]any `json:"modes,omitempty"`
}

// SessionPromptParams for sending a prompt
type SessionPromptParams struct {
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
}
