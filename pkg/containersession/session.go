package containersession

import (
	"sync"
	"time"
)

// SessionState represents the lifecycle state of a container session
type SessionState string

const (
	// StatePending indicates the session is created but container not started
	StatePending SessionState = "PENDING"
	// StateRunning indicates the container is running
	StateRunning SessionState = "RUNNING"
	// StateStopped indicates the container stopped gracefully
	StateStopped SessionState = "STOPPED"
	// StateFailed indicates the container failed or an error occurred
	StateFailed SessionState = "FAILED"
)

// ContainerSession represents a single container session with lifecycle management
type ContainerSession struct {
	id                 string
	containerID        string
	workspacePath      string
	labels             map[string]string
	state              SessionState
	createdAt          time.Time
	startedAt          *time.Time
	stoppedAt          *time.Time
	errorMsg           string
	skipOutputLogging  bool // Skip attaching for output logging

	// Synchronization
	mu sync.RWMutex
}

// NewContainerSession creates a new container session in PENDING state
func NewContainerSession(id, workspacePath string, labels map[string]string, createdAt time.Time) *ContainerSession {
	// Create defensive copy of labels to prevent external mutation
	labelsCopy := make(map[string]string, len(labels))
	for k, v := range labels {
		labelsCopy[k] = v
	}
	return &ContainerSession{
		id:            id,
		workspacePath: workspacePath,
		labels:        labelsCopy,
		state:         StatePending,
		createdAt:     createdAt,
	}
}

// ID returns the session ID (thread-safe, immutable)
func (s *ContainerSession) ID() string {
	return s.id
}

// ContainerID returns the Docker container ID
func (s *ContainerSession) ContainerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containerID
}

// SetContainerID sets the Docker container ID (internal use)
func (s *ContainerSession) SetContainerID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containerID = id
}

// State returns the current session state
func (s *ContainerSession) State() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState transitions the session to a new state
func (s *ContainerSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// WorkspacePath returns the workspace directory path
func (s *ContainerSession) WorkspacePath() string {
	return s.workspacePath
}

// Labels returns a copy of the session labels
func (s *ContainerSession) Labels() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	labelsCopy := make(map[string]string, len(s.labels))
	for k, v := range s.labels {
		labelsCopy[k] = v
	}
	return labelsCopy
}

// SetError records an error message and transitions to FAILED state
func (s *ContainerSession) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorMsg = err
	s.state = StateFailed
}

// Error returns the error message (if any)
func (s *ContainerSession) Error() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorMsg
}

// MarkStarted records the start time and transitions to RUNNING
func (s *ContainerSession) MarkStarted(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedAt = &t
	s.state = StateRunning
}

// MarkStopped records the stop time and transitions to STOPPED
func (s *ContainerSession) MarkStopped(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stoppedAt = &t
	s.state = StateStopped
}

// CreatedAt returns the session creation timestamp
func (s *ContainerSession) CreatedAt() time.Time {
	return s.createdAt
}

// StartedAt returns the session start timestamp (zero value if not started)
func (s *ContainerSession) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.startedAt == nil {
		return time.Time{}
	}
	return *s.startedAt
}

// StoppedAt returns the session stop timestamp (zero value if not stopped)
func (s *ContainerSession) StoppedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stoppedAt == nil {
		return time.Time{}
	}
	return *s.stoppedAt
}
