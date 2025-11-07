package helpers

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

func TestUUIDGenerator_Generate(t *testing.T) {
	gen := &UUIDGenerator{}

	// Generate two UUIDs
	id1 := gen.Generate()
	id2 := gen.Generate()

	// Check they're not empty
	if id1 == "" {
		t.Error("Generated UUID is empty")
	}
	if id2 == "" {
		t.Error("Generated UUID is empty")
	}

	// Check they're different
	if id1 == id2 {
		t.Errorf("Generated UUIDs are identical: %s", id1)
	}

	// Check format (UUID v4 has dashes in specific positions)
	if len(id1) != 36 {
		t.Errorf("UUID has wrong length: got %d, want 36", len(id1))
	}
	if !strings.Contains(id1, "-") {
		t.Error("UUID doesn't contain dashes")
	}
}

func TestSystemClock_Now(t *testing.T) {
	clock := &SystemClock{}

	before := time.Now()
	result := clock.Now()
	after := time.Now()

	// Check result is between before and after
	if result.Before(before) || result.After(after) {
		t.Errorf("Clock.Now() returned time outside expected range: %v", result)
	}
}

func TestStdLogger_Printf(t *testing.T) {
	// Create a logger that writes to a buffer we can inspect
	var buf strings.Builder
	logger := &StdLogger{
		Logger: log.New(&buf, "", 0),
	}

	// Log a message
	logger.Printf("test message: %s", "hello")

	// Check output contains our message
	output := buf.String()
	if !strings.Contains(output, "test message: hello") {
		t.Errorf("Logger output doesn't contain expected message, got: %s", output)
	}
}

func TestCreateDockerClient(t *testing.T) {
	ctx := context.Background()

	client, err := CreateDockerClient(ctx)
	if err != nil {
		// Skip test if Docker is not available
		t.Skipf("Docker not available: %v", err)
		return
	}
	defer client.Close()

	// Verify we can ping Docker
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = client.Ping(pingCtx)
	if err != nil {
		t.Errorf("Failed to ping Docker: %v", err)
	}
}
