package session

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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

func TestNewHeartbeatMonitor(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()

	tests := []struct {
		name      string
		natsURL   string
		wantError bool
	}{
		{
			name:      "successful connection",
			natsURL:   natsURL,
			wantError: false,
		},
		{
			name:      "invalid NATS URL",
			natsURL:   "nats://invalid-host:9999",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor, err := NewHeartbeatMonitor(tt.natsURL)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if monitor.nats == nil {
				t.Error("nats connection is nil")
			}

			if monitor.lastSeen == nil {
				t.Error("lastSeen map is nil")
			}

			monitor.Stop()
		})
	}
}

func TestHeartbeatMonitor_ReceivesHeartbeat(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	// Create monitor
	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Create publisher to send heartbeat
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect publisher: %v", err)
	}
	defer nc.Close()

	// Publish heartbeat
	hb := struct {
		AgentID   string    `json:"agentId"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
	}{
		AgentID:   agentID,
		Timestamp: time.Now(),
		Status:    "active",
	}

	data, _ := json.Marshal(hb)
	subject := "agent.heartbeat." + agentID

	if err := nc.Publish(subject, data); err != nil {
		t.Fatalf("failed to publish heartbeat: %v", err)
	}

	// Wait a bit for message processing
	time.Sleep(100 * time.Millisecond)

	// Check lastSeen was updated
	lastSeen := monitor.GetLastSeen(agentID)
	if lastSeen.IsZero() {
		t.Error("lastSeen was not updated after heartbeat")
	}
}

func TestHeartbeatMonitor_RenewLeaseOnHeartbeat(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	// Setup temporary lease directory
	tmpDir := t.TempDir()
	originalLeaseDir := LeaseDir
	LeaseDir = filepath.Join(tmpDir, "session")
	defer func() { LeaseDir = originalLeaseDir }()

	natsURL := ns.ClientURL()
	agentID := "test-agent"
	sessionID := "sess-123"

	// Create a lease for the agent
	lease, err := AcquireLease(agentID, sessionID)
	if err != nil {
		t.Fatalf("failed to acquire lease: %v", err)
	}
	originalExpiry := lease.ExpiresAt

	// Create monitor
	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Wait a tiny bit to ensure lease creation time != renewal time
	time.Sleep(10 * time.Millisecond)

	// Create publisher to send heartbeat
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect publisher: %v", err)
	}
	defer nc.Close()

	// Publish heartbeat
	hb := struct {
		AgentID   string    `json:"agentId"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
	}{
		AgentID:   agentID,
		Timestamp: time.Now(),
		Status:    "active",
	}

	data, _ := json.Marshal(hb)
	subject := "agent.heartbeat." + agentID

	if err := nc.Publish(subject, data); err != nil {
		t.Fatalf("failed to publish heartbeat: %v", err)
	}

	// Wait for lease renewal
	time.Sleep(100 * time.Millisecond)

	// Read lease and check expiry was extended
	renewed, err := ReadLease(agentID)
	if err != nil {
		t.Fatalf("failed to read renewed lease: %v", err)
	}

	if !renewed.ExpiresAt.After(originalExpiry) {
		t.Errorf("lease expiry was not extended: original=%v, renewed=%v",
			originalExpiry, renewed.ExpiresAt)
	}
}

func TestHeartbeatMonitor_NoRenewalForDetachedAgent(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	// Setup temporary lease directory
	tmpDir := t.TempDir()
	originalLeaseDir := LeaseDir
	LeaseDir = filepath.Join(tmpDir, "session")
	defer func() { LeaseDir = originalLeaseDir }()

	natsURL := ns.ClientURL()
	agentID := "test-agent"

	// Do NOT create a lease (agent is detached)

	// Create monitor
	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	// Capture log output
	var logBuf bytes.Buffer
	monitor.SetLogger(log.New(&logBuf, "", 0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Create publisher to send heartbeat
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect publisher: %v", err)
	}
	defer nc.Close()

	// Publish heartbeat
	hb := struct {
		AgentID   string    `json:"agentId"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
	}{
		AgentID:   agentID,
		Timestamp: time.Now(),
		Status:    "active",
	}

	data, _ := json.Marshal(hb)
	subject := "agent.heartbeat." + agentID

	if err := nc.Publish(subject, data); err != nil {
		t.Fatalf("failed to publish heartbeat: %v", err)
	}

	// Wait for message processing
	time.Sleep(100 * time.Millisecond)

	// Verify no lease file was created
	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	if _, err := os.Stat(leasePath); err == nil {
		t.Error("lease file should not exist for detached agent")
	}

	// Verify no renewal errors logged (since there's no lease to renew)
	logOutput := logBuf.String()
	if bytes.Contains([]byte(logOutput), []byte("Failed to renew lease")) {
		t.Errorf("unexpected renewal error logged: %s", logOutput)
	}
}

func TestHeartbeatMonitor_ReapExpiredLeases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping reaper test in short mode")
	}

	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	// Setup temporary lease directory
	tmpDir := t.TempDir()
	originalLeaseDir := LeaseDir
	LeaseDir = filepath.Join(tmpDir, "session")
	defer func() { LeaseDir = originalLeaseDir }()

	natsURL := ns.ClientURL()
	agentID := "test-agent"
	sessionID := "sess-123"

	// Create an expired lease
	lease, err := AcquireLease(agentID, sessionID)
	if err != nil {
		t.Fatalf("failed to acquire lease: %v", err)
	}

	// Manually expire the lease
	lease.ExpiresAt = time.Now().Add(-1 * time.Minute)
	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	data, _ := json.Marshal(lease)
	if err := os.WriteFile(leasePath, data, 0o600); err != nil {
		t.Fatalf("failed to write expired lease: %v", err)
	}

	// Create monitor with shorter reaper interval for testing
	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Manually trigger reaper instead of waiting
	monitor.removeExpiredLeases()

	// Verify lease was removed
	if _, err := os.Stat(leasePath); err == nil {
		t.Error("expired lease should have been reaped")
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error checking lease file: %v", err)
	}
}

func TestHeartbeatMonitor_GracefulStop(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()

	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic
	monitor.Stop()

	// Multiple stops should not panic
	monitor.Stop()
}

func TestHeartbeatMonitor_InvalidHeartbeatMessage(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	natsURL := ns.ClientURL()

	// Create monitor
	monitor, err := NewHeartbeatMonitor(natsURL)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	// Capture log output
	var logBuf bytes.Buffer
	monitor.SetLogger(log.New(&logBuf, "", 0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Create publisher
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect publisher: %v", err)
	}
	defer nc.Close()

	// Publish invalid JSON
	subject := "agent.heartbeat.test-agent"
	if err := nc.Publish(subject, []byte("invalid json")); err != nil {
		t.Fatalf("failed to publish invalid message: %v", err)
	}

	// Wait for message processing
	time.Sleep(100 * time.Millisecond)

	// Check error was logged
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("Failed to unmarshal heartbeat")) {
		t.Errorf("expected unmarshal error in log, got: %s", logOutput)
	}
}
