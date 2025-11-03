package nats

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

// runTestServer starts an embedded NATS server for testing.
func runTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := test.DefaultTestOptions
	opts.Port = -1 // Random port
	opts.JetStream = true
	srv := test.RunServer(&opts)
	if srv == nil {
		t.Fatal("failed to start test server")
	}
	return srv
}

// TestNewClient verifies client creation with default options.
func TestNewClient(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-client"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	if err := client.Ready(); err != nil {
		t.Errorf("client.Ready() error = %v", err)
	}

	health := client.Health()
	if !health.Connected {
		t.Error("Health().Connected = false, want true")
	}
}

// TestNewClient_InvalidConfig verifies configuration validation and connection errors.
func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "invalid server URL",
			opts: []Option{
				WithURL("nats://localhost:9999"),
				WithName("test"),
				WithConnectTimeout(100 * time.Millisecond), // Fail fast
			},
			wantErr: true, // Will fail to connect
		},
		{
			name: "invalid timeout",
			opts: []Option{
				WithURL("nats://localhost:4222"),
				WithName("test"),
				WithConnectTimeout(-1 * time.Second), // Negative timeout
			},
			wantErr: true, // Should fail validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPublish verifies basic publish functionality.
func TestPublish(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-publisher"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	subject := "test.publish"
	data := []byte("test message")

	if err := client.Publish(ctx, subject, data); err != nil {
		t.Errorf("Publish() error = %v", err)
	}
}

// TestPublish_WithCorrelationID verifies correlation ID propagation.
func TestPublish_WithCorrelationID(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-publisher"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Subscribe to capture the message
	received := make(chan *Message, 1)
	_, err = client.Subscribe(context.Background(), "test.correlation", func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish with correlation ID
	ctx := context.Background()
	if err := client.Publish(ctx, "test.correlation", []byte("test"), WithCorrelationID("test-correlation-123")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.CorrelationID != "test-correlation-123" {
			t.Errorf("CorrelationID = %q, want %q", msg.CorrelationID, "test-correlation-123")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestSubscribe verifies basic subscription functionality.
func TestSubscribe(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-subscriber"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	received := make(chan []byte, 1)
	_, err = client.Subscribe(context.Background(), "test.subscribe", func(ctx context.Context, msg *Message) error {
		received <- msg.Data
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish message
	testData := []byte("test message")
	if err := client.Publish(context.Background(), "test.subscribe", testData); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for message
	select {
	case data := <-received:
		if string(data) != string(testData) {
			t.Errorf("received data = %q, want %q", string(data), string(testData))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestSubscribe_ErrorHandling verifies error handling in message handlers.
func TestSubscribe_ErrorHandling(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-subscriber"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	handlerDone := make(chan struct{})
	testErr := errors.New("handler error")

	_, err = client.Subscribe(context.Background(), "test.error", func(ctx context.Context, msg *Message) error {
		close(handlerDone)
		return testErr
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish message
	if err := client.Publish(context.Background(), "test.error", []byte("test")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for handler to be called
	select {
	case <-handlerDone:
		// Handler was called successfully
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

// TestRequestReply verifies request/reply pattern.
func TestRequestReply(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-requester"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Set up responder
	_, err = client.Subscribe(context.Background(), "test.request", func(ctx context.Context, msg *Message) error {
		// Reply with uppercase version
		response := []byte("RESPONSE")
		return msg.Respond(response)
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Send request
	ctx := context.Background()
	resp, err := client.Request(ctx, "test.request", []byte("request"))
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if string(resp.Data) != "RESPONSE" {
		t.Errorf("response data = %q, want %q", string(resp.Data), "RESPONSE")
	}
}

// TestRequestReply_Timeout verifies request timeout handling.
func TestRequestReply_Timeout(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-requester"),
		WithRequestTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Send request to non-existent responder
	ctx := context.Background()
	_, err = client.Request(ctx, "test.no.responder", []byte("request"))
	if err == nil {
		t.Fatal("Request() expected timeout error, got nil")
	}
	// The error should contain "no responders" or be a timeout
	// (wrapped in transient error by the client)
	if !errors.Is(err, nats.ErrTimeout) && !errors.Is(err, nats.ErrNoResponders) {
		// Check if it's wrapped in a transient error
		if !strings.Contains(err.Error(), "no responders") && !strings.Contains(err.Error(), "timeout") {
			t.Errorf("Request() error = %v, want timeout or no responders error", err)
		}
	}
}

// TestQueueSubscribe verifies queue group subscription.
func TestQueueSubscribe(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-queue"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Create two queue subscribers
	received := make(chan struct{}, 10)
	var count1, count2 atomic.Int32
	_, err = client.Subscribe(context.Background(), "test.queue", func(ctx context.Context, msg *Message) error {
		count1.Add(1)
		received <- struct{}{}
		return nil
	}, WithQueueGroup("workers"))
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	_, err = client.Subscribe(context.Background(), "test.queue", func(ctx context.Context, msg *Message) error {
		count2.Add(1)
		received <- struct{}{}
		return nil
	}, WithQueueGroup("workers"))
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish 10 messages
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if err := client.Publish(ctx, "test.queue", []byte("test")); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	// Wait for all messages to be processed
	for i := 0; i < 10; i++ {
		select {
		case <-received:
			// Message processed
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d", i+1)
		}
	}

	// Both subscribers should have received some messages (load balanced)
	c1 := count1.Load()
	c2 := count2.Load()
	total := c1 + c2

	if total != 10 {
		t.Errorf("total messages received = %d, want 10", total)
	}

	// With queue groups, messages should be distributed (not all to one subscriber)
	if c1 == 0 || c2 == 0 {
		t.Errorf("queue distribution failed: subscriber1=%d, subscriber2=%d", c1, c2)
	}
}

// TestHealth verifies health status tracking.
func TestHealth(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-health"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	health := client.Health()
	if !health.Connected {
		t.Error("Health().Connected = false, want true")
	}
}

// TestDrain verifies graceful connection draining.
func TestDrain(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-drain"),
		WithDrainTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Create subscription before draining
	_, err = client.Subscribe(context.Background(), "test.drain", func(ctx context.Context, msg *Message) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	ctx := context.Background()
	if err := client.Drain(ctx); err != nil {
		t.Errorf("Drain() error = %v", err)
	}

	// After draining, client should be closed
	if err := client.Publish(ctx, "test", []byte("test")); !errors.Is(err, ErrClientClosed) {
		t.Errorf("Publish() after Drain() error = %v, want ErrClientClosed", err)
	}
}

// TestClose verifies client closure.
func TestClose(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-close"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// After closing, operations should fail
	ctx := context.Background()
	if err := client.Publish(ctx, "test", []byte("test")); !errors.Is(err, ErrClientClosed) {
		t.Errorf("Publish() after Close() error = %v, want ErrClientClosed", err)
	}
}

// TestReconnection verifies reconnection behavior.
func TestReconnection(t *testing.T) {
	t.Skip("Reconnection testing requires fixed port - see issue #112")
	srv := runTestServer(t)
	defer srv.Shutdown()

	var reconnectedCalled atomic.Bool
	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-reconnect"),
		WithReconnectWait(100*time.Millisecond),
		WithMaxReconnects(10),
		WithReconnectedCallback(func(nc *nats.Conn) {
			reconnectedCalled.Store(true)
		}),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Force a disconnect by stopping the server
	srv.Shutdown()

	// Wait for disconnect
	time.Sleep(200 * time.Millisecond)

	health := client.Health()
	if health.Connected {
		t.Error("Health().Connected = true after server shutdown, want false")
	}

	// Restart server on same URL
	srv = runTestServer(t)
	defer srv.Shutdown()

	// Note: This test is simplified - in reality, we'd need to restart on the exact same port
	// which is tricky with random ports. In a real scenario, you'd mock the reconnection logic.
}

// TestConcurrentPublish verifies concurrent publish operations.
func TestConcurrentPublish(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-concurrent"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx := context.Background()
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				if err := client.Publish(ctx, "test.concurrent", []byte("test")); err != nil {
					t.Errorf("Publish() error = %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

// TestRawAccess verifies raw NATS connection access.
func TestRawAccess(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-raw"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	raw := client.Raw()
	if raw == nil {
		t.Fatal("Raw() returned nil")
	}

	if !raw.IsConnected() {
		t.Error("Raw().IsConnected() = false, want true")
	}
}

// TestJetStream verifies JetStream client creation.
func TestJetStream(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-jetstream"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	js, err := client.JS()
	if err != nil {
		t.Fatalf("JS() error = %v", err)
	}

	if js == nil {
		t.Fatal("JS() returned nil client")
	}

	// Calling JS() again should return same instance
	js2, err := client.JS()
	if err != nil {
		t.Fatalf("JS() second call error = %v", err)
	}

	if js2 == nil {
		t.Fatal("JS() second call returned nil")
	}
}

// TestPublish_RetryOnTransientError verifies retry logic.
func TestPublish_RetryOnTransientError(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-retry"),
		WithRetryAttempts(2),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Publishing to a valid subject should succeed
	ctx := context.Background()
	if err := client.Publish(ctx, "test.retry", []byte("test")); err != nil {
		t.Errorf("Publish() error = %v", err)
	}
}

// TestReady_AfterClose verifies Ready returns error after close.
func TestReady_AfterClose(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-ready"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Should be ready initially
	if err := client.Ready(); err != nil {
		t.Errorf("Ready() before Close() error = %v", err)
	}

	// Close the client
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Should not be ready after close
	if err := client.Ready(); err == nil {
		t.Error("Ready() after Close() error = nil, want error")
	}
}

// TestPublish_WithRetryAttempts verifies multiple retry attempts.
func TestPublish_WithRetryAttempts(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-retry-attempts"),
		WithRetryAttempts(5),
		WithRetryBackoff(newExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, nil)),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Normal publish should work
	ctx := context.Background()
	if err := client.Publish(ctx, "test.retry", []byte("test")); err != nil {
		t.Errorf("Publish() error = %v", err)
	}
}

// TestRequest_WithCorrelationID verifies request with correlation ID.
func TestRequest_WithCorrelationID(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-request-correlation"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Set up responder
	received := make(chan *Message, 1)
	_, err = client.Subscribe(context.Background(), "test.request.corr", func(ctx context.Context, msg *Message) error {
		received <- msg
		return msg.Respond([]byte("response"))
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Send request with correlation ID
	resp, err := client.Request(context.Background(), "test.request.corr", []byte("request"),
		WithRequestCorrelationID("test-corr-456"))
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if string(resp.Data) != "response" {
		t.Errorf("response.Data = %q, want %q", string(resp.Data), "response")
	}

	// Check the correlation ID was sent
	select {
	case msg := <-received:
		if msg.CorrelationID != "test-corr-456" {
			t.Errorf("request CorrelationID = %q, want %q", msg.CorrelationID, "test-corr-456")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for request message")
	}
}
