// Package testutil provides common testing utilities for the ourocodus project.
package testutil

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// StartTestNATSServer starts an embedded NATS server for testing.
// The server listens on a random available port (127.0.0.1:0).
// The caller should defer ns.Shutdown() to ensure cleanup.
func StartTestNATSServer(t *testing.T) *server.Server {
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
