# ACP Protocol Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate from deprecated `agent/sendMessage` protocol to real ACP protocol with streaming support.

**Architecture:** Add new protocol types and methods to pkg/acp, update ACPBridge for container mode, modify server handlers for streaming, update WebSocket message format.

**Tech Stack:** Go 1.24, JSON-RPC 2.0, WebSocket, Docker

---

## Phase 1: Foundation (pkg/acp)

### Task 1: Add ACP Protocol Types

**Files:**
- Create: `pkg/acp/protocol.go`
- Test: `pkg/acp/protocol_test.go`

**Step 1: Write the failing test**

```go
// pkg/acp/protocol_test.go
package acp

import (
	"encoding/json"
	"testing"
)

func TestInitializeParams_Marshal(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: 1,
		ClientInfo: ClientInfo{
			Name:    "ourocodus",
			Version: "1.0",
		},
		Capabilities: map[string]any{},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"protocolVersion":1,"clientInfo":{"name":"ourocodus","version":"1.0"},"capabilities":{}}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}
}

func TestSessionNewParams_Marshal(t *testing.T) {
	params := SessionNewParams{
		Cwd:        "/workspace",
		MCPServers: []any{},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"cwd":"/workspace","mcpServers":[]}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}
}

func TestSessionNewResult_Unmarshal(t *testing.T) {
	data := `{"sessionId":"test-123","models":{},"modes":{}}`

	var result SessionNewResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if result.SessionID != "test-123" {
		t.Errorf("got sessionId %s, want test-123", result.SessionID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestInitialize|TestSessionNew" -v`
Expected: FAIL with "undefined: InitializeParams"

**Step 3: Write minimal implementation**

```go
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
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestInitialize|TestSessionNew" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/acp/protocol.go pkg/acp/protocol_test.go
git commit -m "feat(acp): add protocol types for initialize and session/new"
```

---

### Task 2: Add Event Types

**Files:**
- Create: `pkg/acp/event.go`
- Test: `pkg/acp/event_test.go`

**Step 1: Write the failing test**

```go
// pkg/acp/event_test.go
package acp

import (
	"encoding/json"
	"testing"
)

func TestEvent_TextDelta(t *testing.T) {
	event := Event{
		Type: EventTextDelta,
		Text: "Hello world",
	}

	if event.Type != EventTextDelta {
		t.Errorf("got type %s, want %s", event.Type, EventTextDelta)
	}
	if event.Text != "Hello world" {
		t.Errorf("got text %s, want Hello world", event.Text)
	}
}

func TestEvent_ToolCall(t *testing.T) {
	args := json.RawMessage(`{"file_path":"/src/main.go"}`)
	event := Event{
		Type:       EventToolCall,
		ToolCallID: "call-1",
		ToolName:   "Read",
		ToolArgs:   args,
	}

	if event.Type != EventToolCall {
		t.Errorf("got type %s, want %s", event.Type, EventToolCall)
	}
	if event.ToolName != "Read" {
		t.Errorf("got tool name %s, want Read", event.ToolName)
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		et   EventType
		want string
	}{
		{EventTextDelta, "text.delta"},
		{EventToolCall, "tool.call"},
		{EventToolResult, "tool.result"},
		{EventError, "error"},
		{EventSessionEnd, "session.end"},
	}

	for _, tt := range tests {
		if string(tt.et) != tt.want {
			t.Errorf("got %s, want %s", tt.et, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestEvent" -v`
Expected: FAIL with "undefined: Event"

**Step 3: Write minimal implementation**

```go
// pkg/acp/event.go
package acp

import "encoding/json"

// EventType represents the type of streaming event
type EventType string

// Event types for streaming responses
const (
	EventTextDelta  EventType = "text.delta"
	EventToolCall   EventType = "tool.call"
	EventToolResult EventType = "tool.result"
	EventError      EventType = "error"
	EventSessionEnd EventType = "session.end"
)

// Event represents a streaming event from ACP
type Event struct {
	Type       EventType       `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolArgs   json.RawMessage `json:"toolArgs,omitempty"`
	ToolResult json.RawMessage `json:"toolResult,omitempty"`
	Error      string          `json:"error,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestEvent" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/acp/event.go pkg/acp/event_test.go
git commit -m "feat(acp): add Event types for streaming responses"
```

---

### Task 3: Add Session Update Parser

**Files:**
- Modify: `pkg/acp/event.go`
- Modify: `pkg/acp/event_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/acp/event_test.go

func TestParseSessionUpdate_TextDelta(t *testing.T) {
	// Real session/update notification from claude-code-acp
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"assistant","message":{"type":"text","text":"Hello"}}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventTextDelta {
		t.Errorf("got type %s, want %s", event.Type, EventTextDelta)
	}
	if event.Text != "Hello" {
		t.Errorf("got text %s, want Hello", event.Text)
	}
}

func TestParseSessionUpdate_ToolCall(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"assistant","message":{"type":"tool_use","id":"call-1","name":"Read","input":{"file_path":"/src/main.go"}}}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventToolCall {
		t.Errorf("got type %s, want %s", event.Type, EventToolCall)
	}
	if event.ToolName != "Read" {
		t.Errorf("got tool name %s, want Read", event.ToolName)
	}
	if event.ToolCallID != "call-1" {
		t.Errorf("got tool call id %s, want call-1", event.ToolCallID)
	}
}

func TestParseSessionUpdate_SessionEnd(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test-123","update":{"type":"result","subtype":"success"}}}`

	event, err := ParseSessionUpdate([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if event.Type != EventSessionEnd {
		t.Errorf("got type %s, want %s", event.Type, EventSessionEnd)
	}
}

func TestParseSessionUpdate_NotSessionUpdate(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"result":{}}`

	_, err := ParseSessionUpdate([]byte(raw))
	if err == nil {
		t.Error("expected error for non-session/update message")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestParseSessionUpdate" -v`
Expected: FAIL with "undefined: ParseSessionUpdate"

**Step 3: Write minimal implementation**

```go
// Add to pkg/acp/event.go

import (
	"encoding/json"
	"fmt"
)

// sessionUpdateNotification represents the JSON-RPC notification structure
type sessionUpdateNotification struct {
	Method string `json:"method"`
	Params struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	} `json:"params"`
}

// sessionUpdate represents the update payload
type sessionUpdate struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Message struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"message,omitempty"`
}

// ParseSessionUpdate parses a session/update notification into an Event
func ParseSessionUpdate(data []byte) (*Event, error) {
	var notif sessionUpdateNotification
	if err := json.Unmarshal(data, &notif); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	if notif.Method != "session/update" {
		return nil, fmt.Errorf("not a session/update notification: %s", notif.Method)
	}

	var update sessionUpdate
	if err := json.Unmarshal(notif.Params.Update, &update); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update: %w", err)
	}

	// Handle result type (session end)
	if update.Type == "result" {
		return &Event{Type: EventSessionEnd}, nil
	}

	// Handle assistant messages
	if update.Type == "assistant" {
		switch update.Message.Type {
		case "text":
			return &Event{
				Type: EventTextDelta,
				Text: update.Message.Text,
			}, nil
		case "tool_use":
			return &Event{
				Type:       EventToolCall,
				ToolCallID: update.Message.ID,
				ToolName:   update.Message.Name,
				ToolArgs:   update.Message.Input,
			}, nil
		case "tool_result":
			return &Event{
				Type:       EventToolResult,
				ToolCallID: update.Message.ID,
				ToolResult: update.Message.Input,
			}, nil
		}
	}

	// Unknown update type - return as-is for debugging
	return &Event{
		Type:  EventTextDelta,
		Text:  string(notif.Params.Update),
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... -run "TestParseSessionUpdate" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/acp/event.go pkg/acp/event_test.go
git commit -m "feat(acp): add ParseSessionUpdate for streaming notifications"
```

---

## Phase 2: Container Mode (ACPBridge)

### Task 4: Add ACPSessionID to AgentSession

**Files:**
- Modify: `pkg/relay/session/models.go`
- Modify: `pkg/relay/session/models_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/models_test.go

func TestAgentSession_ACPSessionID(t *testing.T) {
	now := time.Now()
	agent := NewAgentSession("test-agent", "/workspace", now)

	// Initially empty
	if agent.GetACPSessionID() != "" {
		t.Errorf("expected empty ACPSessionID, got %s", agent.GetACPSessionID())
	}

	// Set and get
	agent.SetACPSessionID("session-123")
	if agent.GetACPSessionID() != "session-123" {
		t.Errorf("expected session-123, got %s", agent.GetACPSessionID())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestAgentSession_ACPSessionID" -v`
Expected: FAIL with "agent.GetACPSessionID undefined"

**Step 3: Write minimal implementation**

```go
// Add to AgentSession struct in pkg/relay/session/models.go (after errorMsg field):
	acpSessionID string // ACP session ID (1:1 with this agent)

// Add methods to pkg/relay/session/models.go:

// GetACPSessionID returns the ACP session ID for this agent
func (a *AgentSession) GetACPSessionID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.acpSessionID
}

// SetACPSessionID sets the ACP session ID for this agent
func (a *AgentSession) SetACPSessionID(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acpSessionID = sessionID
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestAgentSession_ACPSessionID" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/models.go pkg/relay/session/models_test.go
git commit -m "feat(session): add ACPSessionID to AgentSession"
```

---

### Task 5: Add Initialize Method to ACPBridge

**Files:**
- Modify: `pkg/relay/session/acp_bridge.go`
- Modify: `pkg/relay/session/acp_bridge_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/acp_bridge_test.go

func TestACPBridge_InitializeACP(t *testing.T) {
	// Create mock transport that responds to initialize
	mockStdout := &bytes.Buffer{}
	mockStdin := &bytes.Buffer{}

	// Write mock response
	response := `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentInfo":{"name":"test","version":"1.0"}}}` + "\n"
	mockStdout.WriteString(response)

	bridge := &ACPBridge{
		agentID:    "test",
		logger:     log.New(io.Discard, "", 0),
		stdoutR:    mockStdout,
		scan:       bufio.NewScanner(mockStdout),
		notifCh:    make(chan []byte, 100),
		pending:    nil,
		ctx:        context.Background(),
		reqCounter: 0,
	}

	// Need to mock the write path
	bridge.conn = &mockHijackedResponse{stdin: mockStdin}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := bridge.InitializeACP(ctx)
	if err != nil {
		t.Fatalf("InitializeACP failed: %v", err)
	}

	if result.ProtocolVersion != 1 {
		t.Errorf("expected protocol version 1, got %d", result.ProtocolVersion)
	}
}

// mockHijackedResponse implements types.HijackedResponse for testing
type mockHijackedResponse struct {
	stdin  io.Writer
	closed bool
}

func (m *mockHijackedResponse) Read(p []byte) (int, error)  { return 0, io.EOF }
func (m *mockHijackedResponse) Write(p []byte) (int, error) { return m.stdin.Write(p) }
func (m *mockHijackedResponse) Close() error                { m.closed = true; return nil }
func (m *mockHijackedResponse) CloseWrite() error           { return nil }
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_InitializeACP" -v`
Expected: FAIL with "bridge.InitializeACP undefined"

**Step 3: Write minimal implementation**

```go
// Add to pkg/relay/session/acp_bridge.go

import (
	"github.com/2389-research/ourocodus/pkg/acp"
)

// InitializeACP performs the protocol handshake with the ACP server
func (b *ACPBridge) InitializeACP(ctx context.Context) (*acp.InitializeResult, error) {
	reqID := b.generateRequestID()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  acp.MethodInitialize,
		"params": acp.InitializeParams{
			ProtocolVersion: 1,
			ClientInfo: acp.ClientInfo{
				Name:    "ourocodus",
				Version: "1.0",
			},
			Capabilities: map[string]any{},
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	respBytes, err := b.sendRaw(ctx, reqBytes, reqID)
	if err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	var resp struct {
		Result acp.InitializeResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse initialize response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp.Result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_InitializeACP" -v`
Expected: PASS (or adjust mock as needed)

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/acp_bridge.go pkg/relay/session/acp_bridge_test.go
git commit -m "feat(bridge): add InitializeACP method"
```

---

### Task 6: Add CreateSession Method to ACPBridge

**Files:**
- Modify: `pkg/relay/session/acp_bridge.go`
- Modify: `pkg/relay/session/acp_bridge_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/acp_bridge_test.go

func TestACPBridge_CreateSession(t *testing.T) {
	// Similar mock setup as InitializeACP test
	mockStdout := &bytes.Buffer{}
	mockStdin := &bytes.Buffer{}

	response := `{"jsonrpc":"2.0","id":1,"result":{"sessionId":"session-abc-123","models":{},"modes":{}}}` + "\n"
	mockStdout.WriteString(response)

	bridge := &ACPBridge{
		agentID:    "test",
		logger:     log.New(io.Discard, "", 0),
		stdoutR:    mockStdout,
		scan:       bufio.NewScanner(mockStdout),
		notifCh:    make(chan []byte, 100),
		ctx:        context.Background(),
		reqCounter: 0,
	}
	bridge.conn = &mockHijackedResponse{stdin: mockStdin}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionID, err := bridge.CreateSession(ctx, "/workspace")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if sessionID != "session-abc-123" {
		t.Errorf("expected session-abc-123, got %s", sessionID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_CreateSession" -v`
Expected: FAIL with "bridge.CreateSession undefined"

**Step 3: Write minimal implementation**

```go
// Add to pkg/relay/session/acp_bridge.go

// CreateSession creates a new ACP session with bypassPermissions mode
func (b *ACPBridge) CreateSession(ctx context.Context, cwd string) (string, error) {
	reqID := b.generateRequestID()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  acp.MethodSessionNew,
		"params": acp.SessionNewParams{
			Cwd:        cwd,
			MCPServers: []any{},
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session/new request: %w", err)
	}

	respBytes, err := b.sendRaw(ctx, reqBytes, reqID)
	if err != nil {
		return "", fmt.Errorf("session/new failed: %w", err)
	}

	var resp struct {
		Result acp.SessionNewResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("failed to parse session/new response: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("session/new error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result.SessionID, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_CreateSession" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/acp_bridge.go pkg/relay/session/acp_bridge_test.go
git commit -m "feat(bridge): add CreateSession method"
```

---

### Task 7: Add SendPrompt Method to ACPBridge

**Files:**
- Modify: `pkg/relay/session/acp_bridge.go`
- Modify: `pkg/relay/session/acp_bridge_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/acp_bridge_test.go

func TestACPBridge_SendPrompt(t *testing.T) {
	mockStdout := &bytes.Buffer{}
	mockStdin := &bytes.Buffer{}

	// SendPrompt doesn't wait for response - it triggers streaming
	bridge := &ACPBridge{
		agentID:    "test",
		logger:     log.New(io.Discard, "", 0),
		stdoutR:    mockStdout,
		scan:       bufio.NewScanner(mockStdout),
		notifCh:    make(chan []byte, 100),
		ctx:        context.Background(),
		reqCounter: 0,
	}
	bridge.conn = &mockHijackedResponse{stdin: mockStdin}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bridge.SendPrompt(ctx, "session-123", "What is 2+2?")
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}

	// Verify the request was sent correctly
	sent := mockStdin.String()
	if !strings.Contains(sent, "session/prompt") {
		t.Errorf("expected session/prompt in sent data, got: %s", sent)
	}
	if !strings.Contains(sent, "What is 2+2?") {
		t.Errorf("expected prompt text in sent data, got: %s", sent)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_SendPrompt" -v`
Expected: FAIL with "bridge.SendPrompt undefined"

**Step 3: Write minimal implementation**

```go
// Add to pkg/relay/session/acp_bridge.go

// SendPrompt sends a prompt to an existing ACP session
// This triggers streaming - use Stream() to receive events
func (b *ACPBridge) SendPrompt(ctx context.Context, sessionID, prompt string) error {
	reqID := b.generateRequestID()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  acp.MethodSessionPrompt,
		"params": acp.SessionPromptParams{
			SessionID: sessionID,
			Prompt:    prompt,
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal session/prompt request: %w", err)
	}

	// Send request (fire-and-forget for streaming)
	reqBytes = append(reqBytes, '\n')

	b.writeMu.Lock()
	_, err = b.conn.Write(reqBytes)
	b.writeMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send prompt: %w", err)
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_SendPrompt" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/acp_bridge.go pkg/relay/session/acp_bridge_test.go
git commit -m "feat(bridge): add SendPrompt method"
```

---

### Task 8: Add Stream Method to ACPBridge

**Files:**
- Modify: `pkg/relay/session/acp_bridge.go`
- Modify: `pkg/relay/session/acp_bridge_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/acp_bridge_test.go

func TestACPBridge_Stream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifCh := make(chan []byte, 10)
	bridge := &ACPBridge{
		agentID: "test",
		logger:  log.New(io.Discard, "", 0),
		notifCh: notifCh,
		ctx:     ctx,
	}

	// Send test notifications
	go func() {
		notifCh <- []byte(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test","update":{"type":"assistant","message":{"type":"text","text":"Hello"}}}}`)
		notifCh <- []byte(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"test","update":{"type":"result","subtype":"success"}}}`)
		close(notifCh)
	}()

	eventCh := bridge.Stream(ctx)

	// First event should be text delta
	event1 := <-eventCh
	if event1.Type != acp.EventTextDelta {
		t.Errorf("expected text.delta, got %s", event1.Type)
	}
	if event1.Text != "Hello" {
		t.Errorf("expected Hello, got %s", event1.Text)
	}

	// Second event should be session end
	event2 := <-eventCh
	if event2.Type != acp.EventSessionEnd {
		t.Errorf("expected session.end, got %s", event2.Type)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_Stream" -v`
Expected: FAIL with "bridge.Stream undefined"

**Step 3: Write minimal implementation**

```go
// Add to pkg/relay/session/acp_bridge.go

// Stream returns a channel of parsed events from session/update notifications
func (b *ACPBridge) Stream(ctx context.Context) <-chan acp.Event {
	eventCh := make(chan acp.Event, 100)

	go func() {
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return
			case raw, ok := <-b.notifCh:
				if !ok {
					return
				}

				event, err := acp.ParseSessionUpdate(raw)
				if err != nil {
					b.logf("[ACPBridge] Failed to parse notification: %v", err)
					continue
				}

				select {
				case eventCh <- *event:
				case <-ctx.Done():
					return
				}

				// Stop streaming on session end
				if event.Type == acp.EventSessionEnd {
					return
				}
			}
		}
	}()

	return eventCh
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPBridge_Stream" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/acp_bridge.go pkg/relay/session/acp_bridge_test.go
git commit -m "feat(bridge): add Stream method for parsed events"
```

---

## Phase 3: Manager Integration

### Task 9: Update SpawnAgent to Initialize ACP Session

**Files:**
- Modify: `pkg/relay/session/manager.go`
- Modify: `pkg/relay/session/manager_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/manager_test.go

func TestManager_SpawnAgent_InitializesACPSession(t *testing.T) {
	// This test verifies the new initialization sequence
	// We need to check that after SpawnAgent:
	// 1. ACP client exists
	// 2. ACPSessionID is set on the agent

	mgr := setupTestManager(t) // Use existing test helper

	ctx := context.Background()
	userSessionID, err := mgr.CreateUserSession(ctx, "test-user")
	if err != nil {
		t.Fatalf("CreateUserSession failed: %v", err)
	}

	// Create mock ACP client that tracks calls
	mockFactory := &mockACPClientFactory{
		sessionID: "mock-session-123",
	}
	mgr.clientFactory = mockFactory

	err = mgr.SpawnAgent(ctx, userSessionID, "test-agent", "/tmp/workspace")
	if err != nil {
		t.Fatalf("SpawnAgent failed: %v", err)
	}

	agent, err := mgr.GetAgent(userSessionID, "test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	// Verify ACPSessionID was set
	if agent.GetACPSessionID() != "mock-session-123" {
		t.Errorf("expected ACPSessionID mock-session-123, got %s", agent.GetACPSessionID())
	}
}

// mockACPClientFactory for testing
type mockACPClientFactory struct {
	sessionID string
}

func (m *mockACPClientFactory) NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
	return &mockStreamingACPClient{sessionID: m.sessionID}, nil
}

type mockStreamingACPClient struct {
	sessionID string
}

func (m *mockStreamingACPClient) SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error) {
	return &acp.AgentMessage{Content: "mock response"}, nil
}

func (m *mockStreamingACPClient) InitializeACP(ctx context.Context) (*acp.InitializeResult, error) {
	return &acp.InitializeResult{ProtocolVersion: 1}, nil
}

func (m *mockStreamingACPClient) CreateSession(ctx context.Context, cwd string) (string, error) {
	return m.sessionID, nil
}

func (m *mockStreamingACPClient) Close() error {
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestManager_SpawnAgent_InitializesACPSession" -v`
Expected: FAIL with interface mismatch or missing methods

**Step 3: Update ACPClient interface and SpawnAgent**

```go
// Update ACPClient interface in pkg/relay/session/interfaces.go:

type ACPClient interface {
	SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error)
	InitializeACP(ctx context.Context) (*acp.InitializeResult, error)
	CreateSession(ctx context.Context, cwd string) (string, error)
	Close() error
}

// Update SpawnAgent in pkg/relay/session/manager.go
// Add after "m.logger.Printf("[SESSION] ✓ ACP client created successfully")":

	// Initialize ACP protocol
	m.logger.Printf("[SESSION] ├─ Initializing ACP protocol...")
	initResult, err := acpClient.InitializeACP(ctx)
	if err != nil {
		m.logger.Printf("[SESSION] ✗ ACP initialization failed: %v", err)
		return m.spawnFailure(ctx, agentSession, agentID, userSessionID,
			fmt.Sprintf("failed to initialize ACP: %v", err), true)
	}
	m.logger.Printf("[SESSION] ✓ ACP initialized (protocol v%d, agent: %s)",
		initResult.ProtocolVersion, initResult.AgentInfo.Name)

	// Create ACP session
	m.logger.Printf("[SESSION] ├─ Creating ACP session...")
	acpSessionID, err := acpClient.CreateSession(ctx, absPath)
	if err != nil {
		m.logger.Printf("[SESSION] ✗ ACP session creation failed: %v", err)
		return m.spawnFailure(ctx, agentSession, agentID, userSessionID,
			fmt.Sprintf("failed to create ACP session: %v", err), true)
	}
	m.logger.Printf("[SESSION] ✓ ACP session created: %s", acpSessionID)

	// Store session ID on agent
	agentSession.SetACPSessionID(acpSessionID)
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestManager_SpawnAgent_InitializesACPSession" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/interfaces.go pkg/relay/session/manager.go pkg/relay/session/manager_test.go
git commit -m "feat(manager): initialize ACP session during SpawnAgent"
```

---

## Phase 4: WebSocket Streaming

### Task 10: Add Streaming Message Type

**Files:**
- Modify: `pkg/relay/message.go`
- Modify: `pkg/relay/message_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/message_test.go

func TestAgentStreamDelta_Marshal(t *testing.T) {
	msg := AgentStreamDelta{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:stream-delta"},
		UserSessionID: "sess-123",
		AgentID:       "auth",
		EventType:     "text.delta",
		Delta:         "Hello",
		Final:         false,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["type"] != "agent:stream-delta" {
		t.Errorf("expected type agent:stream-delta, got %v", result["type"])
	}
	if result["eventType"] != "text.delta" {
		t.Errorf("expected eventType text.delta, got %v", result["eventType"])
	}
	if result["delta"] != "Hello" {
		t.Errorf("expected delta Hello, got %v", result["delta"])
	}
}

func TestAgentStreamDelta_ToolCall(t *testing.T) {
	msg := AgentStreamDelta{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:stream-delta"},
		UserSessionID: "sess-123",
		AgentID:       "auth",
		EventType:     "tool.call",
		ToolCall: &StreamToolCall{
			ID:   "call-1",
			Name: "Read",
			Args: json.RawMessage(`{"file_path":"/src/main.go"}`),
		},
		Final: false,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if !strings.Contains(string(data), "tool.call") {
		t.Errorf("expected tool.call in output")
	}
	if !strings.Contains(string(data), "Read") {
		t.Errorf("expected Read in output")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/... -run "TestAgentStreamDelta" -v`
Expected: FAIL with "undefined: AgentStreamDelta"

**Step 3: Write minimal implementation**

```go
// Add to pkg/relay/message.go

// AgentStreamDelta for streaming responses
type AgentStreamDelta struct {
	BaseMessage
	UserSessionID string          `json:"userSessionId"`
	AgentID       string          `json:"agentId"`
	EventType     string          `json:"eventType"`
	Delta         string          `json:"delta,omitempty"`
	ToolCall      *StreamToolCall `json:"toolCall,omitempty"`
	Final         bool            `json:"final"`
}

// StreamToolCall represents a tool call in streaming
type StreamToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// NewAgentStreamDelta creates a new streaming delta message
func NewAgentStreamDelta(userSessionID, agentID, eventType, delta string, final bool) *AgentStreamDelta {
	return &AgentStreamDelta{
		BaseMessage:   BaseMessage{Version: ProtocolVersion, Type: "agent:stream-delta"},
		UserSessionID: userSessionID,
		AgentID:       agentID,
		EventType:     eventType,
		Delta:         delta,
		Final:         final,
	}
}

// NewAgentStreamDeltaToolCall creates a tool call streaming message
func NewAgentStreamDeltaToolCall(userSessionID, agentID string, toolCall *StreamToolCall) *AgentStreamDelta {
	return &AgentStreamDelta{
		BaseMessage:   BaseMessage{Version: ProtocolVersion, Type: "agent:stream-delta"},
		UserSessionID: userSessionID,
		AgentID:       agentID,
		EventType:     "tool.call",
		ToolCall:      toolCall,
		Final:         false,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/... -run "TestAgentStreamDelta" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/message.go pkg/relay/message_test.go
git commit -m "feat(relay): add AgentStreamDelta message type"
```

---

### Task 11: Add Streaming ACPClient Interface Methods

**Files:**
- Modify: `pkg/relay/session/interfaces.go`
- Modify: `pkg/relay/session/client_factory.go`

**Step 1: Write the failing test**

```go
// Add to pkg/relay/session/client_factory_test.go

func TestACPClientAdapter_ImplementsStreamingInterface(t *testing.T) {
	// Verify the adapter implements the full interface
	var _ ACPClient = (*acpClientAdapter)(nil)

	// The interface should have these methods
	var client ACPClient
	_ = client.SendMessage
	_ = client.InitializeACP
	_ = client.CreateSession
	_ = client.SendPrompt
	_ = client.Stream
	_ = client.Close
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPClientAdapter_ImplementsStreamingInterface" -v`
Expected: FAIL with missing methods

**Step 3: Update interface and adapter**

```go
// Update ACPClient interface in pkg/relay/session/interfaces.go:

type ACPClient interface {
	// Legacy method (deprecated, for backward compat)
	SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error)

	// New ACP protocol methods
	InitializeACP(ctx context.Context) (*acp.InitializeResult, error)
	CreateSession(ctx context.Context, cwd string) (string, error)
	SendPrompt(ctx context.Context, sessionID, prompt string) error
	Stream(ctx context.Context) <-chan acp.Event

	Close() error
}

// Update acpClientAdapter in pkg/relay/session/client_factory.go:

func (a *acpClientAdapter) InitializeACP(ctx context.Context) (*acp.InitializeResult, error) {
	// For host mode, the existing client doesn't support new protocol
	// Return a stub result for now - container mode uses ACPBridge
	return &acp.InitializeResult{
		ProtocolVersion: 1,
		AgentInfo: acp.AgentInfo{
			Name:    "echo-agent",
			Version: "1.0",
		},
	}, nil
}

func (a *acpClientAdapter) CreateSession(ctx context.Context, cwd string) (string, error) {
	// For host mode, generate a local session ID
	return fmt.Sprintf("host-%d", time.Now().UnixNano()), nil
}

func (a *acpClientAdapter) SendPrompt(ctx context.Context, sessionID, prompt string) error {
	// For host mode, delegate to legacy SendMessage
	_, err := a.client.SendMessage(ctx, prompt)
	return err
}

func (a *acpClientAdapter) Stream(ctx context.Context) <-chan acp.Event {
	// Host mode doesn't support streaming - return empty channel
	ch := make(chan acp.Event)
	close(ch)
	return ch
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/relay/session/... -run "TestACPClientAdapter_ImplementsStreamingInterface" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add pkg/relay/session/interfaces.go pkg/relay/session/client_factory.go pkg/relay/session/client_factory_test.go
git commit -m "feat(session): add streaming methods to ACPClient interface"
```

---

### Task 12: Run All Tests and Verify

**Files:** None (verification only)

**Step 1: Run all core tests**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go test ./pkg/acp/... ./pkg/relay/... -v 2>&1 | tail -50`
Expected: All tests PASS

**Step 2: Run linter**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && mise exec -- golangci-lint run --timeout=5m ./pkg/acp/... ./pkg/relay/...`
Expected: No errors

**Step 3: Verify build**

Run: `cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration && go build ./...`
Expected: Success

**Step 4: Commit any fixes**

If any issues found, fix and commit:

```bash
cd ~/.config/superpowers/worktrees/ourocodus/acp-protocol-migration
git add -A
git commit -m "fix: address linter and test issues"
```

---

## Summary

This plan implements the ACP protocol migration in 12 tasks across 4 phases:

1. **Phase 1 (Tasks 1-3):** Foundation types in pkg/acp
2. **Phase 2 (Tasks 4-8):** ACPBridge methods for container mode
3. **Phase 3 (Task 9):** Manager integration for initialization
4. **Phase 4 (Tasks 10-12):** WebSocket streaming and verification

Each task follows TDD: write failing test, implement minimal code, verify, commit.

**Total estimated tasks:** 12
**Files modified:** ~15
**New files created:** 2-3
