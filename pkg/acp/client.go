package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Client manages communication with a single claude-code-acp process
type Client struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	scanner  *bufio.Scanner
	closedMu sync.RWMutex
	reqMu    sync.Mutex // Protects entire request/response cycle
	nextID   int
	closed   bool
}

// NewClient spawns a claude-code-acp process and returns a client to communicate with it
func NewClient(workspace string, apiKey string) (*Client, error) {
	if workspace == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Create command to spawn claude-code-acp process
	cmd := exec.Command("claude-code-acp", "--workspace", workspace)

	// Set API key via environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("ANTHROPIC_API_KEY=%s", apiKey))

	// Setup stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Setup stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Setup stderr pipe for debugging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("failed to start claude-code-acp: %w", err)
	}

	client := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
		closed:  false,
	}

	// Start goroutine to log stderr (for debugging)
	go client.logStderr()

	return client, nil
}

// logStderr reads stderr and logs it for debugging purposes
func (c *Client) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// In production, use proper logging. For now, just ignore stderr.
		// fmt.Fprintf(os.Stderr, "[ACP stderr] %s\n", scanner.Text())
		_ = scanner.Text()
	}
}

// SendMessage sends a message to the agent and returns the response
// This method is thread-safe - the entire request/response cycle is protected by a mutex
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
	if _, err = c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response from stdout
	return c.readResponse()
}

// readResponse reads a single JSON-RPC response from stdout
// Must be called with reqMu held (called from SendMessage)
func (c *Client) readResponse() (*AgentMessage, error) {
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

// Close terminates the claude-code-acp process and cleans up resources
func (c *Client) Close() error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()

	// Close stdin to signal the process to exit
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	// Wait for process to exit
	if err := c.cmd.Wait(); err != nil {
		// Process may exit with non-zero status, which is acceptable
		// Only return error if it's a system error, not exit status
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("failed to wait for process: %w", err)
		}
	}

	// Close remaining pipes
	_ = c.stdout.Close()
	_ = c.stderr.Close()

	return nil
}
