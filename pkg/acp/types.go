package acp

// JSON-RPC 2.0 message structures for ACP

// Request represents a JSON-RPC 2.0 request
type Request struct {
	ID      interface{} `json:"id"`
	Params  interface{} `json:"params,omitempty"`
	JSONRPC string      `json:"jsonrpc"` // Always "2.0"
	Method  string      `json:"method"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	JSONRPC string      `json:"jsonrpc"` // Always "2.0"
}

// Error represents a JSON-RPC 2.0 error object
type Error struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
}

// ACP-specific methods
const (
	MethodSendMessage = "agent/sendMessage"
	MethodGetContext  = "agent/getContext"
	MethodToolCall    = "agent/toolCall"
)

// SendMessageParams represents parameters for sending a message to the agent
type SendMessageParams struct {
	Images  []string `json:"images,omitempty"`
	Content string   `json:"content"`
}

// AgentMessage represents a message from the agent
type AgentMessage struct {
	ToolCall *ToolCall `json:"toolCall,omitempty"`
	Type     string    `json:"type"` // "text" or "toolCall"
	Content  string    `json:"content,omitempty"`
}

// ToolCall represents a tool invocation from the agent
type ToolCall struct {
	Args map[string]interface{} `json:"args"`
	Name string                 `json:"name"`
}

// Logger abstracts logging operations for the ACP client
type Logger interface {
	Printf(format string, v ...interface{})
}

// noOpLogger discards all log output when no logger is provided
type noOpLogger struct{}

func (noOpLogger) Printf(format string, v ...interface{}) {}
