package acp

// JSON-RPC 2.0 message structures for ACP

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"` // Always "2.0"
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"` // Always "2.0"
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ACP-specific methods
const (
	MethodSendMessage = "agent/sendMessage"
	MethodGetContext  = "agent/getContext"
	MethodToolCall    = "agent/toolCall"
)

// SendMessageParams represents parameters for sending a message to the agent
type SendMessageParams struct {
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// AgentMessage represents a message from the agent
type AgentMessage struct {
	Type     string    `json:"type"` // "text" or "toolCall"
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"toolCall,omitempty"`
}

// ToolCall represents a tool invocation from the agent
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}
