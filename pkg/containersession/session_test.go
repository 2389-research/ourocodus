package containersession

import (
	"testing"
	"time"
)

func TestNewContainerSession(t *testing.T) {
	t.Run("creates session in PENDING state", func(t *testing.T) {
		id := "test-session"
		workspace := "/tmp/workspace"
		labels := map[string]string{"key": "value"}
		createdAt := time.Now()

		session := NewContainerSession(id, workspace, labels, createdAt)

		if session.ID() != id {
			t.Errorf("Expected ID %s, got %s", id, session.ID())
		}

		if session.WorkspacePath() != workspace {
			t.Errorf("Expected workspace %s, got %s", workspace, session.WorkspacePath())
		}

		if session.State() != StatePending {
			t.Errorf("Expected state PENDING, got %s", session.State())
		}

		if session.ContainerID() != "" {
			t.Errorf("Expected empty container ID, got %s", session.ContainerID())
		}

		returnedLabels := session.Labels()
		if returnedLabels["key"] != "value" {
			t.Error("Labels not properly set")
		}
	})
}

func TestContainerSession_ContainerID(t *testing.T) {
	t.Run("sets and gets container ID", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		session.SetContainerID("container-123")

		if session.ContainerID() != "container-123" {
			t.Errorf("Expected container ID 'container-123', got %s", session.ContainerID())
		}
	})

	t.Run("thread-safe access", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		// Concurrent writes and reads
		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				session.SetContainerID("container-1")
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = session.ContainerID()
			}
			done <- true
		}()

		<-done
		<-done
	})
}

func TestContainerSession_State(t *testing.T) {
	t.Run("transitions through states", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		if session.State() != StatePending {
			t.Error("Expected initial state PENDING")
		}

		session.SetState(StateRunning)
		if session.State() != StateRunning {
			t.Error("Expected state RUNNING after SetState")
		}

		session.SetState(StateStopped)
		if session.State() != StateStopped {
			t.Error("Expected state STOPPED after SetState")
		}
	})

	t.Run("thread-safe state access", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				session.SetState(StateRunning)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = session.State()
			}
			done <- true
		}()

		<-done
		<-done
	})
}

func TestContainerSession_Labels(t *testing.T) {
	t.Run("returns copy of labels", func(t *testing.T) {
		originalLabels := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		session := NewContainerSession("test", "/tmp", originalLabels, time.Now())

		labels := session.Labels()

		// Modify returned labels
		labels["key3"] = "value3"
		delete(labels, "key1")

		// Original labels should be unchanged
		sessionLabels := session.Labels()
		if len(sessionLabels) != 2 {
			t.Errorf("Expected 2 labels, got %d", len(sessionLabels))
		}

		if sessionLabels["key1"] != "value1" {
			t.Error("Original labels were modified")
		}

		if _, exists := sessionLabels["key3"]; exists {
			t.Error("Added label should not exist in session")
		}
	})
}

func TestContainerSession_Error(t *testing.T) {
	t.Run("sets error and transitions to FAILED", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		session.SetError("something went wrong")

		if session.Error() != "something went wrong" {
			t.Errorf("Expected error message 'something went wrong', got %s", session.Error())
		}

		if session.State() != StateFailed {
			t.Errorf("Expected state FAILED after SetError, got %s", session.State())
		}
	})

	t.Run("empty error by default", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		if session.Error() != "" {
			t.Errorf("Expected empty error, got %s", session.Error())
		}
	})
}

func TestContainerSession_MarkStarted(t *testing.T) {
	t.Run("marks session as started", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		startTime := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
		session.MarkStarted(startTime)

		if session.State() != StateRunning {
			t.Errorf("Expected state RUNNING after MarkStarted, got %s", session.State())
		}

		// Note: startedAt is internal, we verify it indirectly through state
	})
}

func TestContainerSession_MarkStopped(t *testing.T) {
	t.Run("marks session as stopped", func(t *testing.T) {
		session := NewContainerSession("test", "/tmp", map[string]string{}, time.Now())

		stopTime := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		session.MarkStopped(stopTime)

		if session.State() != StateStopped {
			t.Errorf("Expected state STOPPED after MarkStopped, got %s", session.State())
		}
	})
}

func TestContainerSession_WorkspacePath(t *testing.T) {
	t.Run("returns workspace path", func(t *testing.T) {
		workspace := "/tmp/test-workspace"
		session := NewContainerSession("test", workspace, map[string]string{}, time.Now())

		if session.WorkspacePath() != workspace {
			t.Errorf("Expected workspace %s, got %s", workspace, session.WorkspacePath())
		}
	})
}

func TestSessionState_String(t *testing.T) {
	t.Run("state constants have correct values", func(t *testing.T) {
		if StatePending != "PENDING" {
			t.Errorf("Expected StatePending to be 'PENDING', got %s", StatePending)
		}

		if StateRunning != "RUNNING" {
			t.Errorf("Expected StateRunning to be 'RUNNING', got %s", StateRunning)
		}

		if StateStopped != "STOPPED" {
			t.Errorf("Expected StateStopped to be 'STOPPED', got %s", StateStopped)
		}

		if StateFailed != "FAILED" {
			t.Errorf("Expected StateFailed to be 'FAILED', got %s", StateFailed)
		}
	})
}
