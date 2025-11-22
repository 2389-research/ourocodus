// Package session provides ACP communication bridge for CLI agents.
package session

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// ACPBridge provides bidirectional ACP communication with a containerized agent.
// It handles the Docker exec session, stdout/stderr demultiplexing, and JSON-RPC
// request/response correlation.
//
// The bridge supports a single in-flight request at a time (matching the agent's
// single-threaded model), but maintains a dedicated reader goroutine to continuously
// drain stdout and handle cancellation cleanly.
type ACPBridge struct {
	containerID string
	agentID     string

	// Docker hijacked connection for writes
	conn net.Conn

	// Demultiplexed stdout/stderr readers
	stdoutR io.Reader
	stderrR io.Reader
	scan    *bufio.Scanner

	// Write serialization
	writeMu sync.Mutex

	// Single in-flight pending request
	pendMu  sync.Mutex
	pending *pendingReq

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool
	wg     sync.WaitGroup // Tracks background goroutines (readLoop, logStderr, demux)

	// Optional: notifications channel for agent-initiated messages
	notifCh chan []byte
}

// pendingReq represents a single in-flight request awaiting response.
type pendingReq struct {
	id       string
	respCh   chan []byte
	canceled atomic.Bool
}

// NewACPBridge creates a new ACP bridge to communicate with a containerized agent.
//
// The bridge establishes a Docker exec session to the agent container and sets up
// bidirectional communication over stdin/stdout. It spawns background goroutines to:
// - Continuously read and parse JSON-RPC responses from stdout
// - Log stderr output from the agent
//
// The bridge must be closed via Close() when done to release resources.
func NewACPBridge(ctx context.Context, containerID, agentID string) (*ACPBridge, error) {
	// Validate input parameters
	if err := ValidateNonEmpty(containerID, fmt.Errorf("containerID cannot be empty")); err != nil {
		return nil, err
	}
	if err := ValidateNonEmpty(agentID, fmt.Errorf("agentID cannot be empty")); err != nil {
		return nil, err
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Attach directly to the container's stdin/stdout
	// The container is already running the ACP agent process, so we attach to its I/O streams
	// rather than executing a new command.
	attachResp, err := cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to container: %w", err)
	}

	// Create bridge context
	bridgeCtx, cancel := context.WithCancel(ctx)

	bridge := &ACPBridge{
		containerID: containerID,
		agentID:     agentID,
		conn:        attachResp.Conn,
		ctx:         bridgeCtx,
		cancel:      cancel,
		notifCh:     make(chan []byte, 10),
	}

	// Set up stdout/stderr demultiplexing
	// Docker exec with Tty=false multiplexes stdout/stderr; stdcopy separates them
	prOut, pwOut := io.Pipe()
	prErr, pwErr := io.Pipe()

	bridge.stdoutR = prOut
	bridge.stderrR = prErr

	// Demux goroutine with context-aware cancellation
	bridge.wg.Add(1)
	go func() {
		defer bridge.wg.Done()
		defer func() {
			_ = pwOut.Close()
			_ = pwErr.Close()
		}()

		// Run StdCopy in a separate goroutine to allow context cancellation
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Ignore copy errors; readLoop will detect EOF
			_, _ = stdcopy.StdCopy(pwOut, pwErr, attachResp.Reader)
		}()

		// Wait for either completion or context cancellation
		select {
		case <-done:
			// StdCopy finished normally
		case <-bridgeCtx.Done():
			// Context cancelled: force close to unblock StdCopy
			attachResp.Close()
			<-done // Wait for StdCopy to exit
		}
	}()

	// Set up scanner with larger buffer for big payloads
	scanner := bufio.NewScanner(bridge.stdoutR)
	buf := make([]byte, 256*1024)    // 256KB initial buffer
	scanner.Buffer(buf, 4*1024*1024) // 4MB max buffer
	bridge.scan = scanner

	// Start stderr logger goroutine
	bridge.wg.Add(1)
	go bridge.logStderr()

	// Start stdout reader goroutine
	bridge.wg.Add(1)
	go bridge.readLoop()

	return bridge, nil
}

// SendMessage sends an ACP sendMessage request and waits for the response.
// This method is blocking and serialized - only one request can be in-flight at a time.
//
// The context is used for timeout/cancellation. If the context is canceled before
// the response arrives, the pending request is marked as canceled and the eventual
// late response will be discarded to prevent misalignment with future requests.
func (b *ACPBridge) SendMessage(ctx context.Context, content string) (interface{}, error) {
	// Generate unique request ID
	reqID := generateRequestID()

	// Build ACP JSON-RPC request
	// Note: ACP protocol uses "agent/sendMessage" method (see pkg/acp/types.go)
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "agent/sendMessage",
		"params": map[string]interface{}{
			"content": content,
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send and wait for response
	respBytes, err := b.sendRaw(ctx, reqBytes, reqID)
	if err != nil {
		return nil, err
	}

	// Parse response
	type rpcError struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"` // Additional error context from agent
	}
	var resp struct {
		Result interface{} `json:"result"`
		Error  *rpcError   `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		// Include data field in error if present
		if resp.Error.Data != nil {
			return nil, fmt.Errorf("agent error %d: %s (data: %v)", resp.Error.Code, resp.Error.Message, resp.Error.Data)
		}
		return nil, fmt.Errorf("agent error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// sendRaw sends a raw JSON-RPC request and waits for the response.
// This is the low-level method that enforces single in-flight semantics.
func (b *ACPBridge) sendRaw(ctx context.Context, raw []byte, id string) ([]byte, error) {
	if b.closed.Load() {
		return nil, io.ErrClosedPipe
	}

	// Create pending request
	pend := &pendingReq{
		id:     id,
		respCh: make(chan []byte, 1),
	}

	// Set as the single pending request
	b.pendMu.Lock()
	if b.pending != nil {
		b.pendMu.Unlock()
		return nil, errors.New("agent busy: request already in-flight")
	}
	b.pending = pend
	b.pendMu.Unlock()

	// Write NDJSON line to stdin
	b.writeMu.Lock()
	line := append(bytes.TrimRight(raw, "\n"), '\n')
	_, werr := b.conn.Write(line)
	b.writeMu.Unlock()

	if werr != nil {
		b.clearPendingOnError(pend)
		return nil, fmt.Errorf("write failed: %w", werr)
	}

	// Wait for response or timeout
	select {
	case resp := <-pend.respCh:
		return resp, nil

	case <-ctx.Done():
		// Request canceled/timeout - mark as canceled so readLoop drops late response
		pend.canceled.Store(true)
		b.clearPendingOnError(pend)
		return nil, ctx.Err()

	case <-b.ctx.Done():
		// Bridge closed
		pend.canceled.Store(true)
		b.clearPendingOnError(pend)
		return nil, io.EOF
	}
}

// readLoop continuously reads JSON-RPC responses from stdout and dispatches them
// to the pending request waiter or the notifications channel.
func (b *ACPBridge) readLoop() {
	defer b.wg.Done()
	// Goroutine owns channel lifetime - close when exiting
	defer close(b.notifCh)

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		if !b.scan.Scan() {
			// EOF or scanner error - check for scanner error first
			if err := b.scan.Err(); err != nil {
				// TODO: Use proper logger when available
				fmt.Fprintf(os.Stderr, "[ACPBridge] Scanner error for agent %s: %v\n", b.agentID, err)
			}

			// Signal shutdown without waiting on wg to avoid self-deadlock
			// Note: Cannot call Close() here as it waits on wg.Wait() which includes this goroutine
			if b.closed.CompareAndSwap(false, true) {
				b.cancel()
				if b.conn != nil {
					_ = b.conn.Close()
				}
				b.pendMu.Lock()
				if b.pending != nil {
					b.pending.canceled.Store(true)
					close(b.pending.respCh)
					b.pending = nil
				}
				b.pendMu.Unlock()
			}
			return
		}

		line := append([]byte(nil), b.scan.Bytes()...) // Copy line

		// Extract JSON-RPC id to correlate with pending request
		id := extractJSONRPCID(line)

		b.pendMu.Lock()
		pend := b.pending
		if pend != nil && pend.id == id {
			// This response matches the pending request
			b.pending = nil // Clear pending
			b.pendMu.Unlock()

			// Check if request was canceled (timeout)
			if pend.canceled.Load() {
				// Drop late response to prevent misalignment
				continue
			}

			// Deliver response
			select {
			case pend.respCh <- line:
			default:
			}
			continue
		}
		b.pendMu.Unlock()

		// Unsolicited line (notification or unexpected response)
		// Forward to notifications channel if anyone is listening
		select {
		case b.notifCh <- line:
		default:
			// Drop if channel is full (apply backpressure policy)
		}
	}
}

// logStderr reads and logs stderr output from the agent.
func (b *ACPBridge) logStderr() {
	defer b.wg.Done()
	scanner := bufio.NewScanner(b.stderrR)
	for scanner.Scan() {
		// TODO: Use proper logger when available
		// For now, stderr from agent is silently consumed
		// In production, forward to relay logger with agentID context
	}
}

// clearPendingOnError clears the pending request if it matches the given request.
func (b *ACPBridge) clearPendingOnError(pend *pendingReq) {
	b.pendMu.Lock()
	if b.pending == pend {
		b.pending = nil
	}
	b.pendMu.Unlock()
}

// Close closes the ACP bridge and releases all resources.
// This method is safe to call multiple times.
// The provided context controls the grace period for goroutine shutdown.
func (b *ACPBridge) Close(ctx context.Context) error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Signal all goroutines to exit
	b.cancel()

	// Close hijacked connection to unblock I/O operations
	if b.conn != nil {
		_ = b.conn.Close()
	}

	// Fail any pending request
	b.pendMu.Lock()
	if b.pending != nil {
		b.pending.canceled.Store(true)
		close(b.pending.respCh)
		b.pending = nil
	}
	b.pendMu.Unlock()

	// Wait for all goroutines to exit with context timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-ctx.Done():
		// Context timeout - goroutines will be leaked but we return
		return fmt.Errorf("timeout waiting for goroutines to exit: %w", ctx.Err())
	}

	// Note: notifCh is closed by readLoop goroutine on exit (owns channel lifetime)

	return nil
}

// Notifications returns a channel that receives agent-initiated messages.
// This can be used to forward notifications to the WebSocket client.
func (b *ACPBridge) Notifications() <-chan []byte {
	return b.notifCh
}

// extractJSONRPCID extracts the "id" field from a JSON-RPC response.
// Returns empty string if id is not present or cannot be extracted.
// Handles string, numeric, and null IDs per JSON-RPC 2.0 spec.
func extractJSONRPCID(data []byte) string {
	var partial struct {
		ID interface{} `json:"id"`
	}
	if err := json.Unmarshal(data, &partial); err != nil {
		return ""
	}

	switch id := partial.ID.(type) {
	case string:
		return id
	case float64:
		// JSON numbers are float64 by default
		return fmt.Sprintf("%.0f", id)
	case nil:
		return ""
	default:
		// Fallback for other types
		return fmt.Sprintf("%v", id)
	}
}

// generateRequestID generates a unique request ID for JSON-RPC requests.
func generateRequestID() string {
	// Simple counter-based ID for now
	// Could use UUID or other scheme if needed
	// Note: Since requests are serialized, a simple counter is sufficient
	return fmt.Sprintf("req-%d", requestCounter.Add(1))
}

var requestCounter atomic.Uint64
