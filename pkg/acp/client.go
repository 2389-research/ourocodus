package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Client manages communication with a single claude-code-acp runtime transport.
type Client struct {
	transport Transport
	stderr    io.Reader
	scanner   *bufio.Scanner
	logger    Logger
	closedMu  sync.RWMutex
	reqMu     sync.Mutex // Protects entire request/response cycle
	nextID    int
	closed    bool
}

// ClientOption configures a Client
type ClientOption func(*clientConfig)

type clientConfig struct {
	commandPath string
	commandArgs []string
	logger      Logger
	launcher    ProcessLauncher
	env         map[string]string
	launchCtx   context.Context
}

// WithCommand sets a custom command path and args for the ACP process
// Useful for testing or custom installations
func WithCommand(path string, args ...string) ClientOption {
	return func(c *clientConfig) {
		c.commandPath = path
		c.commandArgs = args
	}
}

// WithLogger sets a custom logger for ACP stderr output
func WithLogger(logger Logger) ClientOption {
	return func(c *clientConfig) {
		if logger == nil {
			c.logger = noOpLogger{}
			return
		}
		c.logger = logger
	}
}

// WithLaunchContext sets the context used for launching the ACP process.
// This enables cancellation and timeouts for docker exec operations.
// If not provided, defaults to context.Background().
func WithLaunchContext(ctx context.Context) ClientOption {
	return func(c *clientConfig) {
		if ctx == nil {
			c.launchCtx = context.Background()
			return
		}
		c.launchCtx = ctx
	}
}

// WithProcessLauncher overrides the default host launcher (used for custom runtimes).
func WithProcessLauncher(launcher ProcessLauncher) ClientOption {
	return func(c *clientConfig) {
		c.launcher = launcher
	}
}

// NewClient spawns a claude-code-acp process and returns a client to communicate with it
func NewClient(workspace string, apiKey string, opts ...ClientOption) (*Client, error) {
	if workspace == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Apply options
	cfg := &clientConfig{
		commandPath: "claude-code-acp",
		logger:      noOpLogger{},
		launcher:    &HostProcessLauncher{},
		launchCtx:   context.Background(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = noOpLogger{}
	}
	if len(cfg.commandArgs) == 0 {
		cfg.commandArgs = []string{"--workspace", workspace}
	}
	if cfg.launcher == nil {
		cfg.launcher = &HostProcessLauncher{}
	}
	if cfg.env == nil {
		cfg.env = make(map[string]string)
	}
	if cfg.launchCtx == nil {
		cfg.launchCtx = context.Background()
	}

	launchCfg := ProcessLaunchConfig{
		Workspace:   workspace,
		APIKey:      apiKey,
		CommandPath: cfg.commandPath,
		CommandArgs: cfg.commandArgs,
		Env:         cloneEnvMap(cfg.env),
	}

	transport, err := cfg.launcher.Start(cfg.launchCtx, launchCfg)
	if err != nil {
		return nil, err
	}

	return newClientFromTransport(transport, cfg.logger)
}

// NewClientFromTransport constructs a client using an existing transport implementation.
//
// This function is useful when you want to provide a custom transport instead of having
// the client spawn a new process. Common use cases include:
//
// - Testing: Provide a mock transport for unit testing ACP protocol handling
// - Custom transports: Use websockets, network sockets, or other custom IPC mechanisms
// - Process reuse: Connect to an already-running ACP process without spawning a new one
// - Advanced scenarios: Pre-configure the transport with specific security or logging requirements
//
// Parameters:
//   - transport: An existing Transport implementation (stdin/stdout/stderr streams). Must not be nil.
//   - opts: Optional configuration via ClientOption functions (e.g., WithLogger)
//
// Returns:
//   - *Client: Configured ACP client ready to send/receive messages
//   - error: Non-nil if transport is nil or client initialization fails
//
// Example - Testing:
//
//	mockTransport := &MockTransport{...}
//	client, err := acp.NewClientFromTransport(mockTransport, acp.WithLogger(logger))
//
// Example - Reusing a process:
//
//	existingTransport := &ProcessTransport{cmd: runningCmd}
//	client, err := acp.NewClientFromTransport(existingTransport)
func NewClientFromTransport(transport Transport, opts ...ClientOption) (*Client, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}

	cfg := &clientConfig{logger: noOpLogger{}}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = noOpLogger{}
	}

	return newClientFromTransport(transport, cfg.logger)
}

func newClientFromTransport(transport Transport, logger Logger) (*Client, error) {
	client := &Client{
		transport: transport,
		stderr:    transport.Stderr(),
		scanner:   bufio.NewScanner(transport),
		logger:    logger,
		nextID:    1,
	}

	client.scanner.Buffer(make([]byte, 64*1024), 5*1024*1024)

	go client.logStderr()

	return client, nil
}

func cloneEnvMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
}

// logStderr reads stderr and logs it for debugging purposes
func (c *Client) logStderr() {
	if c.stderr == nil {
		return
	}
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		c.logger.Printf("[ACP stderr] %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		c.logger.Printf("[ACP stderr] scanner error: %v", err)
	}
}

// SendMessage sends a message to the agent and returns the response.
// Thread safety: Uses two-level locking strategy:
//  1. closedMu (RLock) - Quick check if client is closed
//  2. reqMu (Lock) - Protects entire request/response cycle
//
// Why two locks?
//   - closedMu allows concurrent Close() checks without blocking active requests
//   - reqMu serializes request/response pairs to prevent interleaving
//   - Example: Thread A sends request ID=1, Thread B sends ID=2; without reqMu, responses could mismatch
func (c *Client) SendMessage(content string) (*AgentMessage, error) {
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.closedMu.RUnlock()

	// Lock for entire request/response cycle to prevent interleaving
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	// Generate message ID (no longer needs separate lock since reqMu protects it)
	id := c.nextID
	c.nextID++

	// Construct JSON-RPC request
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  MethodSendMessage,
		Params: SendMessageParams{
			Content: content,
		},
	}

	// Marshal request to JSON
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request to stdin (with newline as delimiter)
	data = append(data, '\n')
	if _, err = c.transport.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response from stdout and verify it matches the request ID
	return c.readResponse(id)
}

// readResponse reads a single JSON-RPC response from stdout and validates the ID
// Must be called with reqMu held (called from SendMessage)
func (c *Client) readResponse(expectedID int) (*AgentMessage, error) {
	// Read next line from stdout (protected by reqMu from caller)
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("no response from agent (EOF)")
	}
	line := c.scanner.Bytes()

	// Parse JSON-RPC response
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Verify response ID matches request ID
	if respID, ok := resp.ID.(float64); ok {
		if int(respID) != expectedID {
			return nil, fmt.Errorf("mismatched response id: got %v, want %d", resp.ID, expectedID)
		}
	} else if respID, ok := resp.ID.(int); ok {
		if respID != expectedID {
			return nil, fmt.Errorf("mismatched response id: got %v, want %d", resp.ID, expectedID)
		}
	} else if resp.ID != expectedID {
		return nil, fmt.Errorf("mismatched response id: got %v, want %d", resp.ID, expectedID)
	}

	// Check for JSON-RPC error
	if resp.Error != nil {
		return nil, fmt.Errorf("ACP error (code %d): %s", resp.Error.Code, resp.Error.Message)
	}

	// Parse result as AgentMessage
	var msg AgentMessage
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(resultData, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent message: %w", err)
	}

	return &msg, nil
}

// Close terminates the claude-code-acp process and cleans up resources.
// Deprecated: Use CloseWithContext to avoid indefinite blocking during shutdown (issue #211).
func (c *Client) Close() error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()

	if c.transport != nil {
		if err := c.transport.Close(); err != nil {
			return fmt.Errorf("failed to close transport: %w", err)
		}
	}

	return nil
}

// CloseWithContext terminates the claude-code-acp process with timeout support.
// This prevents indefinite blocking during shutdown by respecting the context deadline.
// If the context is canceled or times out before Close completes, an error is returned.
func (c *Client) CloseWithContext(ctx context.Context) error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()

	if c.transport == nil {
		return nil
	}

	// Run transport close in goroutine with timeout
	type closeResult struct {
		err error
	}
	resultChan := make(chan closeResult, 1)

	go func() {
		err := c.transport.Close()
		if err != nil {
			err = fmt.Errorf("failed to close transport: %w", err)
		}
		resultChan <- closeResult{err: err}
	}()

	// Wait for either close to complete or context cancellation
	select {
	case result := <-resultChan:
		return result.err
	case <-ctx.Done():
		// Context canceled/timed out - return error but leave cleanup goroutine running
		// The transport will eventually close, but we won't wait indefinitely
		return fmt.Errorf("close timed out: %w", ctx.Err())
	}
}
