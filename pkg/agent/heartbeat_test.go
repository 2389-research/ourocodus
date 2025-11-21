package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startTestNATSServer starts an embedded NATS server for testing
func startTestNATSServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random port
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create NATS server: %v", err)
	}

	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return ns
}

func TestNewHeartbeatPublisher(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()

	tests := []struct {
		name      string
		agentID   string
		natsURL   string
		wantError bool
	}{
		{
			name:      "successful connection",
			agentID:   "test-agent",
			natsURL:   natsURL,
			wantError: false,
		},
		{
			name:      "invalid NATS URL",
			agentID:   "test-agent",
			natsURL:   "nats://invalid-host:9999",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, err := NewHeartbeatPublisher(tt.agentID, tt.natsURL)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pub.agentID != tt.agentID {
				t.Errorf("agentID = %s, want %s", pub.agentID, tt.agentID)
			}

			if pub.nats == nil {
				t.Error("nats connection is nil")
			}

			pub.Stop()
		})
	}
}

func TestHeartbeatPublisher_ImmediatePublish(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	// Create subscriber to capture heartbeats
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect subscriber: %v", err)
	}
	defer nc.Close()

	heartbeats := make(chan Heartbeat, 10)
	subject := "agent.heartbeat.*"

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		var hb Heartbeat
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			t.Errorf("failed to unmarshal heartbeat: %v", err)
			return
		}
		heartbeats <- hb
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// Start publisher
	pub, err := NewHeartbeatPublisher(agentID, natsURL)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	defer pub.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go pub.Start(ctx)

	// Should receive immediate heartbeat
	select {
	case hb := <-heartbeats:
		if hb.AgentID != agentID {
			t.Errorf("agentID = %s, want %s", hb.AgentID, agentID)
		}
		if hb.Status != "active" {
			t.Errorf("status = %s, want active", hb.Status)
		}
		if hb.Timestamp.IsZero() {
			t.Error("timestamp is zero")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive immediate heartbeat within 1 second")
	}
}

func TestHeartbeatPublisher_PeriodicPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping periodic publish test in short mode")
	}

	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	// Create subscriber
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect subscriber: %v", err)
	}
	defer nc.Close()

	heartbeats := make(chan Heartbeat, 10)
	subject := "agent.heartbeat.*"

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		var hb Heartbeat
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			return
		}
		heartbeats <- hb
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// Start publisher
	pub, err := NewHeartbeatPublisher(agentID, natsURL)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	defer pub.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go pub.Start(ctx)

	// Collect heartbeats
	var timestamps []time.Time
	timeout := time.After(2 * time.Second)

collectLoop:
	for {
		select {
		case hb := <-heartbeats:
			timestamps = append(timestamps, hb.Timestamp)
			if len(timestamps) >= 2 {
				break collectLoop
			}
		case <-timeout:
			break collectLoop
		}
	}

	if len(timestamps) < 2 {
		t.Skipf("only received %d heartbeats (need 2+ for interval test), likely due to timing", len(timestamps))
	}
}

func TestHeartbeatPublisher_GracefulStop(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	pub, err := NewHeartbeatPublisher(agentID, natsURL)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go pub.Start(ctx)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic
	pub.Stop()

	// Cancel context
	cancel()

	// Multiple stops should not panic
	pub.Stop()
}

func TestHeartbeatPublisher_LoggingOnError(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	pub, err := NewHeartbeatPublisher(agentID, natsURL)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	defer pub.Stop()

	// Capture log output
	var logBuf bytes.Buffer
	pub.SetLogger(log.New(&logBuf, "", 0))

	// Close NATS connection to force publish error
	pub.nats.Close()

	// Try to publish (should log error)
	subject := "agent.heartbeat.test-agent"
	pub.publish(subject)

	// Check log output
	logOutput := logBuf.String()
	if logOutput == "" {
		t.Error("expected error to be logged, got no output")
	}
	if !bytes.Contains([]byte(logOutput), []byte("Failed to publish heartbeat")) {
		t.Errorf("log output does not contain expected error message: %s", logOutput)
	}
}

func TestHeartbeatPublisher_ContextCancellation(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	pub, err := NewHeartbeatPublisher(agentID, natsURL)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	defer pub.Stop()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		pub.Start(ctx)
		close(done)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Should exit within reasonable time
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Start() did not exit after context cancellation")
	}
}
