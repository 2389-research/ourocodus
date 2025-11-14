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

	// Lifecycle management for background goroutines
	stderrCtx    context.Context
	stderrCancel context.CancelFunc
	stderrDone   chan struct{} // Closed when logStderr exits
	readLoopDone chan struct{} // Closed when readLoop exits

	// Lock hierarchy (prevent deadlocks):
	// SendMessage: opMu → writeMu (brief) → pendingMu (brief) → wait
	// readLoop: pendingMu only
	// Close: never takes opMu
	closedMu sync.RWMutex
	opMu     sync.Mutex // Serialize entire SendMessage operations
	writeMu  sync.Mutex // Narrow: nextID + transport.Write only
	nextID   int
	closed   bool

	// Response demultiplexing
	pendingMu sync.Mutex
	pending   map[int]chan responseResult
	inFlight  sync.WaitGroup
	done      chan struct{} // Closed on shutdown to wake waiters
}

// responseResult carries a response or error from readLoop to SendMessage
type responseResult struct {
	msg *AgentMessage
	err error
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
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		transport:    transport,
		stderr:       transport.Stderr(),
		scanner:      bufio.NewScanner(transport),
		logger:       logger,
		nextID:       1,
		stderrCtx:    ctx,
		stderrCancel: cancel,
		stderrDone:   make(chan struct{}),
		readLoopDone: make(chan struct{}),
		pending:      make(map[int]chan responseResult),
		done:         make(chan struct{}),
	}

	client.scanner.Buffer(make([]byte, 64*1024), 5*1024*1024)

	go client.logStderr()
	go client.readLoop()

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
		close(c.stderrDone)
		return
	}
	defer close(c.stderrDone)

	// IMPORTANT: This goroutine relies on the transport properly closing stderr when
	// Close() is called. scanner.Scan() is a blocking operation that will only return
	// when EOF is received or an error occurs. The transport.Close() implementation
	// must close the stderr reader to unblock this goroutine. The cancellation check
	// after Scan() provides defense-in-depth but cannot interrupt a blocked Scan().
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// Check for cancellation (defense-in-depth - won't interrupt blocked Scan())
		select {
		case <-c.stderrCtx.Done():
			return
		default:
		}

		c.logger.Printf("[ACP stderr] %s", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		c.logger.Printf("[ACP stderr] scanner error: %v", err)
	}
}

// broadcastError sends an error to all pending waiters (must be called with pendingMu NOT held)
func (c *Client) broadcastError(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for id, ch := range c.pending {
		select {
		case ch <- responseResult{err: err}:
		default:
			c.logger.Printf("[ACP read] could not notify waiter id=%d", id)
		}
	}
}

// SendMessage sends a message to the agent and returns the response with context support.
//
// Context usage (fixes issue #226):
//   - Enables timeout/cancellation for the entire request/response cycle
//   - Returns ctx.Err() on timeout or cancellation
//   - Prevents indefinite blocking if agent hangs
//
// Thread safety: Uses three-lock architecture (fixes issue #229):
//   - opMu: Serializes entire SendMessage operations (one at a time)
//   - writeMu: Protects only nextID increment and transport.Write (very brief)
//   - closedMu: RWMutex for concurrent closed flag checks
//   - Lock ordering: opMu → writeMu (brief) → pendingMu (brief) → wait
//
// The readLoop goroutine handles all reading, enabling context-aware waiting via select.
// This prevents the SendMessage/Close race that occurred when holding a lock across scanner.Scan().
func (c *Client) SendMessage(ctx context.Context, content string) (*AgentMessage, error) {
	// Fast closed check (no lock held during check)
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.closedMu.RUnlock()

	// Serialize entire operation (maintains one-at-a-time semantics)
	c.opMu.Lock()
	defer c.opMu.Unlock()

	// Allocate ID (brief writeMu hold)
	c.writeMu.Lock()
	id := c.nextID
	c.nextID++
	c.writeMu.Unlock()

	// Register response channel
	respCh := make(chan responseResult, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()
	c.inFlight.Add(1)
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		c.inFlight.Done()
	}()

	// Construct JSON-RPC request
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  MethodSendMessage,
		Params: SendMessageParams{
			Content: content,
		},
	}

	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	data = append(data, '\n')

	// Write with closed re-check (brief writeMu hold)
	c.writeMu.Lock()
	c.closedMu.RLock()
	closed := c.closed
	c.closedMu.RUnlock()
	if closed {
		c.writeMu.Unlock()
		return nil, fmt.Errorf("client is closed")
	}
	_, err = c.transport.Write(data)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response or cancellation (opMu held, writeMu released)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("client closed")
	case res := <-respCh:
		return res.msg, res.err
	}
}

// readLoop is the background goroutine that continuously reads responses from the transport.
// It demultiplexes responses by ID and dispatches them to waiting SendMessage calls via channels.
// This pattern enables context-aware cancellation (issue #226) and prevents SendMessage/Close races (issue #229).
//
// On EOF or scanner error, readLoop broadcasts the error to all pending waiters and closes the done channel.
func (c *Client) readLoop() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()

		// Decode JSON-RPC response
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// JSON parse error is a protocol violation - terminate read loop
			c.logger.Printf("[ACP read] fatal: invalid JSON: %v", err)
			c.broadcastError(fmt.Errorf("invalid JSON response: %w", err))
			break
		}

		// Extract response ID
		var id int
		switch v := resp.ID.(type) {
		case float64:
			id = int(v)
		case int:
			id = v
		default:
			c.logger.Printf("[ACP read] unexpected id type: %T", resp.ID)
			continue
		}

		// Build responseResult
		rr := responseResult{}
		if resp.Error != nil {
			rr.err = fmt.Errorf("ACP error (code %d): %s", resp.Error.Code, resp.Error.Message)
		} else {
			var msg AgentMessage
			data, err := json.Marshal(resp.Result)
			if err != nil {
				rr.err = fmt.Errorf("failed to marshal result: %w", err)
			} else if err := json.Unmarshal(data, &msg); err != nil {
				rr.err = fmt.Errorf("failed to unmarshal agent message: %w", err)
			} else {
				rr.msg = &msg
			}
		}

		// Dispatch to waiter (non-blocking send for robustness)
		c.pendingMu.Lock()
		ch := c.pending[id]
		c.pendingMu.Unlock()

		if ch != nil {
			select {
			case ch <- rr:
			default:
				// Waiter already exited (context canceled or timeout)
				c.logger.Printf("[ACP read] waiter gone for id=%d", id)
			}
		} else {
			// Unsolicited or late response
			c.logger.Printf("[ACP read] no waiter for id=%d", id)
		}
	}

	// Scanner ended - determine error
	endErr := c.scanner.Err()
	if endErr == nil {
		endErr = io.EOF
	}
	c.logger.Printf("[ACP read] loop ended: %v", endErr)

	// Broadcast termination to all pending waiters
	c.broadcastError(fmt.Errorf("read loop ended: %w", endErr))

	// Signal shutdown (double-close protection)
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	// Signal readLoop termination (nil-safe for tests)
	if c.readLoopDone != nil {
		close(c.readLoopDone)
	}
}

// Close terminates the claude-code-acp process and cleans up resources with context-aware timeout support.
// This prevents indefinite blocking during shutdown by respecting the context deadline.
// If the context is canceled or times out before Close completes, an error is returned.
//
// Coordination with SendMessage (fixes issue #229):
//   - Sets closed flag to prevent new writes
//   - Closes done channel to wake waiting SendMessage operations
//   - Waits for in-flight request to drain (bounded by context)
//   - Never takes opMu (so Close never blocks on SendMessage's opMu)
//
// The implementation directly calls transport.Close(ctx) without goroutine wrapper,
// eliminating potential goroutine leaks (issue #212). The transport layer is responsible
// for respecting the context timeout.
func (c *Client) Close(ctx context.Context) error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()

	// Signal logStderr goroutine to stop
	if c.stderrCancel != nil {
		c.stderrCancel()
	}

	// Wake waiting operations (double-close protection)
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	// Wait for in-flight request to drain (bounded by context)
	// Use a goroutine to make Wait cancellable, but ensure it always completes
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone) // Always close to prevent goroutine leak
		c.inFlight.Wait()
	}()

	select {
	case <-waitDone:
		// Clean drain - in-flight request completed
	case <-ctx.Done():
		// Context expired before drain completed
		// The goroutine will still complete when inFlight is decremented,
		// but we proceed with shutdown to respect the deadline
		c.logger.Printf("[WARN] in-flight request not drained before context deadline")
	}

	// Close transport to unblock readLoop (fixes #212)
	// This must happen BEFORE waiting for readLoop, otherwise readLoop will be stuck in scanner.Scan()
	var transportErr error
	if c.transport != nil {
		transportErr = c.transport.Close(ctx)
		if transportErr != nil {
			transportErr = fmt.Errorf("failed to close transport: %w", transportErr)
		}
	}

	// Wait for readLoop goroutine to exit after transport close
	// Closing transport above will cause scanner.Scan() to return, allowing readLoop to exit
	if c.readLoopDone != nil {
		select {
		case <-c.readLoopDone:
			// ReadLoop exited cleanly after transport close
		case <-ctx.Done():
			c.logger.Printf("[WARN] readLoop did not exit before context deadline")
		}
	}

	// Wait for logStderr goroutine to exit, respecting context deadline
	if c.stderrDone != nil {
		select {
		case <-c.stderrDone:
			// Clean shutdown
		case <-ctx.Done():
			c.logger.Printf("[WARN] logStderr goroutine did not exit before context deadline")
		}
	}

	return transportErr
}
