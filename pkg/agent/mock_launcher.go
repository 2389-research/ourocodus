package agent

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// MockLauncher is an in-memory implementation of AgentLauncher for testing.
// It tracks spawned agents and provides helpers for simulating various scenarios.
type MockLauncher struct {
	mu     sync.RWMutex
	agents map[string]*MockHandle
	nextID int

	// SpawnError, if set, will be returned by Spawn instead of creating an agent.
	SpawnError error
	// AttachError, if set, will be returned by Attach.
	AttachError error
	// StopError, if set, will be returned by Stop.
	StopError error
}

// NewMockLauncher creates a new MockLauncher.
func NewMockLauncher() *MockLauncher {
	return &MockLauncher{
		agents: make(map[string]*MockHandle),
		nextID: 1,
	}
}

// Spawn creates a mock agent with the given configuration.
func (m *MockLauncher) Spawn(ctx context.Context, config *SpawnConfig) (AgentHandle, error) {
	if m.SpawnError != nil {
		return nil, m.SpawnError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("mock-agent-%d", m.nextID)
	m.nextID++

	handle := &MockHandle{
		id:          id,
		workspace:   config.Workspace,
		containerID: fmt.Sprintf("mock-container-%s", id),
		role:        config.Role,
		stdin:       &mockPipe{},
		stdout:      &mockPipe{},
		stderr:      &mockPipe{},
		waitCh:      make(chan error, 1),
	}

	m.agents[id] = handle
	return handle, nil
}

// Attach reconnects to an existing mock agent by ID.
func (m *MockLauncher) Attach(ctx context.Context, id string) (AgentHandle, error) {
	if m.AttachError != nil {
		return nil, m.AttachError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	handle, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return handle, nil
}

// Stop stops a mock agent.
func (m *MockLauncher) Stop(ctx context.Context, handle AgentHandle) error {
	if m.StopError != nil {
		return m.StopError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	mockHandle, ok := handle.(*MockHandle)
	if !ok {
		return fmt.Errorf("invalid handle type")
	}

	delete(m.agents, mockHandle.id)

	// Signal that the agent has stopped
	select {
	case mockHandle.waitCh <- nil:
	default:
	}

	return nil
}

// GetSpawnedAgents returns a slice of all currently spawned agent handles.
// Useful for test assertions.
func (m *MockLauncher) GetSpawnedAgents() []AgentHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handles := make([]AgentHandle, 0, len(m.agents))
	for _, handle := range m.agents {
		handles = append(handles, handle)
	}
	return handles
}

// Reset clears all spawned agents and errors.
func (m *MockLauncher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.agents = make(map[string]*MockHandle)
	m.nextID = 1
	m.SpawnError = nil
	m.AttachError = nil
	m.StopError = nil
}

// MockHandle is an in-memory implementation of AgentHandle for testing.
type MockHandle struct {
	mu          sync.RWMutex
	id          string
	workspace   string
	containerID string
	role        string
	stdin       *mockPipe
	stdout      *mockPipe
	stderr      *mockPipe
	waitCh      chan error
	closed      bool
}

// ID returns the mock agent's ID.
func (h *MockHandle) ID() string {
	return h.id
}

// Workspace returns the mock agent's workspace path.
func (h *MockHandle) Workspace() string {
	return h.workspace
}

// ContainerID returns the mock agent's container ID.
func (h *MockHandle) ContainerID() string {
	return h.containerID
}

// Stdin returns a writer for the mock agent's stdin.
func (h *MockHandle) Stdin() io.WriteCloser {
	return h.stdin
}

// Stdout returns a reader for the mock agent's stdout.
func (h *MockHandle) Stdout() io.ReadCloser {
	return h.stdout
}

// Stderr returns a reader for the mock agent's stderr.
func (h *MockHandle) Stderr() io.ReadCloser {
	return h.stderr
}

// Wait blocks until the mock agent exits or the context is canceled.
func (h *MockHandle) Wait(ctx context.Context) error {
	select {
	case err := <-h.waitCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the mock handle.
func (h *MockHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("handle already closed")
	}

	h.closed = true
	_ = h.stdin.Close()
	_ = h.stdout.Close()
	_ = h.stderr.Close()

	return nil
}

// SimulateExit causes the mock agent to exit with the given error.
// This unblocks any Wait() calls.
func (h *MockHandle) SimulateExit(err error) {
	select {
	case h.waitCh <- err:
	default:
	}
}

// WriteToStdout writes data to the mock agent's stdout.
// This simulates the agent producing output.
func (h *MockHandle) WriteToStdout(data []byte) (int, error) {
	return h.stdout.Write(data)
}

// WriteToStderr writes data to the mock agent's stderr.
// This simulates the agent producing error output.
func (h *MockHandle) WriteToStderr(data []byte) (int, error) {
	return h.stderr.Write(data)
}

// ReadFromStdin reads data from the mock agent's stdin.
// This simulates the agent reading input.
func (h *MockHandle) ReadFromStdin(buf []byte) (int, error) {
	return h.stdin.Read(buf)
}

// mockPipe is a simple in-memory pipe for testing I/O operations.
type mockPipe struct {
	mu     sync.Mutex
	buf    []byte
	closed bool
}

func (p *mockPipe) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0, io.ErrClosedPipe
	}

	p.buf = append(p.buf, data...)
	return len(data), nil
}

func (p *mockPipe) Read(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.buf) == 0 {
		if p.closed {
			return 0, io.EOF
		}
		return 0, nil
	}

	n := copy(buf, p.buf)
	p.buf = p.buf[n:]
	return n, nil
}

func (p *mockPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	return nil
}
