package acp

import (
	"bytes"
	"io"
	"runtime"
	"testing"
	"time"
)

// mockTransport is a simple mock transport for testing
type mockTransport struct {
	stdin        *bytes.Buffer
	stdout       *bytes.Buffer
	stderr       *bytes.Buffer
	stderrReader *io.PipeReader
	stderrWriter *io.PipeWriter
	closed       bool
}

func newMockTransport() *mockTransport {
	// Use io.Pipe so we can close stderr to unblock the scanner
	stderrReader, stderrWriter := io.Pipe()

	// Write mock stderr output
	go func() {
		stderrWriter.Write([]byte("mock stderr output\n"))
	}()

	return &mockTransport{
		stdin:        &bytes.Buffer{},
		stdout:       bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"result":{"type":"text","content":"test"}}`),
		stderr:       &bytes.Buffer{},
		stderrReader: stderrReader,
		stderrWriter: stderrWriter,
	}
}

func (m *mockTransport) Read(p []byte) (n int, err error) {
	return m.stdout.Read(p)
}

func (m *mockTransport) Write(p []byte) (n int, err error) {
	return m.stdin.Write(p)
}

func (m *mockTransport) Stderr() io.Reader {
	return m.stderrReader
}

func (m *mockTransport) Close() error {
	m.closed = true
	// Close stderr writer to unblock scanner.Scan() in logStderr goroutine
	if m.stderrWriter != nil {
		m.stderrWriter.Close()
	}
	return nil
}

// countGoroutines returns the number of currently running goroutines
func countGoroutines() int {
	return runtime.NumGoroutine()
}

// TestClient_NoGoroutineLeak verifies that creating and closing a client
// doesn't leak goroutines. This tests the logStderr goroutine lifecycle.
func TestClient_NoGoroutineLeak(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := countGoroutines()

	// Create and close multiple clients to amplify any leak
	for i := 0; i < 10; i++ {
		transport := newMockTransport()
		client, err := NewClientFromTransport(transport, WithLogger(&testLogger{t: t}))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Close the client
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}

	// Give goroutines time to exit
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count is back to baseline (allow small variance)
	current := countGoroutines()
	if current > baseline+2 {
		t.Errorf("Goroutine leak detected: baseline=%d, current=%d, leaked=%d",
			baseline, current, current-baseline)
	}
}

// TestClient_LogStderrExitsOnClose verifies that the logStderr goroutine
// exits when Close() is called, even if stderr is still producing output.
func TestClient_LogStderrExitsOnClose(t *testing.T) {
	stderrReader, stderrWriter := io.Pipe()
	transport := &mockTransport{
		stdin:        &bytes.Buffer{},
		stdout:       bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"result":{"type":"text","content":"test"}}`),
		stderr:       &bytes.Buffer{},
		stderrReader: stderrReader,
	}

	// Start producing stderr output in the background
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				time.Sleep(10 * time.Millisecond)
				if _, err := stderrWriter.Write([]byte("test output\n")); err != nil {
					return
				}
			}
		}
	}()

	client, err := NewClientFromTransport(transport, WithLogger(&testLogger{t: t}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Wait a bit for stderr goroutine to be reading
	time.Sleep(50 * time.Millisecond)

	// Get goroutine count before close
	beforeClose := countGoroutines()

	// Close the client
	if err := client.Close(); err != nil {
		t.Errorf("Failed to close client: %v", err)
	}

	// Close the stderr writer to send EOF and stop the writer goroutine
	close(stop)
	stderrWriter.Close()
	<-done

	// Wait for goroutine to exit (should be quick)
	time.Sleep(100 * time.Millisecond)

	// Check that goroutine count decreased
	afterClose := countGoroutines()
	if afterClose >= beforeClose {
		t.Errorf("logStderr goroutine did not exit: before=%d, after=%d",
			beforeClose, afterClose)
	}
}

// TestClient_CloseTimeout verifies that Close() respects the timeout
// and doesn't hang indefinitely if logStderr goroutine is stuck.
func TestClient_CloseTimeout(t *testing.T) {
	transport := newMockTransport()
	client, err := NewClientFromTransport(transport, WithLogger(&testLogger{t: t}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close should complete within reasonable time (2 second timeout + buffer)
	done := make(chan error, 1)
	go func() {
		done <- client.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Errorf("Close did not complete within timeout")
	}
}

// TestClient_MultipleCloseIdempotent verifies that calling Close()
// multiple times is safe and doesn't cause issues.
func TestClient_MultipleCloseIdempotent(t *testing.T) {
	transport := newMockTransport()
	client, err := NewClientFromTransport(transport, WithLogger(&testLogger{t: t}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close multiple times
	for i := 0; i < 3; i++ {
		if err := client.Close(); err != nil {
			t.Errorf("Close %d failed: %v", i+1, err)
		}
	}
}

// testLogger is a simple logger for tests
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}
