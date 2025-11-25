package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// Test that canceling the context makes run exit promptly even if no input arrives.
func TestRun_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := strings.NewReader("") // no input
	var out, errBuf bytes.Buffer

	done := make(chan struct{})
	go func() {
		_ = run(ctx, in, &out, &errBuf)
		close(done)
	}()

	// Cancel shortly after start
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("run did not exit after context cancel")
	}
}

// Test that a valid sendMessage line produces an echo response.
func TestRun_EchoResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := `{"jsonrpc":"2.0","id":1,"method":"agent/sendMessage","params":{"content":"hi"}}` + "\n"
	in := strings.NewReader(req)
	var out, errBuf bytes.Buffer

	if err := run(ctx, in, &out, &errBuf); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), `Echo: hi`) {
		t.Fatalf("expected echo response, got: %s (stderr: %s)", out.String(), errBuf.String())
	}
}
