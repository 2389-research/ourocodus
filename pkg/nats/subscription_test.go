package nats

import (
	"context"
	"testing"
	"time"
)

// TestSubscription_Methods verifies subscription utility methods.
func TestSubscription_Methods(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-subscription"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	sub, err := client.Subscribe(context.Background(), "test.methods", func(ctx context.Context, msg *Message) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Test Subject
	if subject := sub.Subject(); subject != "test.methods" {
		t.Errorf("Subject() = %q, want %q", subject, "test.methods")
	}

	// Test IsValid (should be valid)
	if !sub.IsValid() {
		t.Error("IsValid() = false, want true")
	}

	// Test Stop
	if err := sub.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Test IsValid after stop (should be invalid)
	if sub.IsValid() {
		t.Error("IsValid() = true after Stop(), want false")
	}

	// Test Stop again (should return error)
	if err := sub.Stop(context.Background()); err != ErrSubscriptionClosed {
		t.Errorf("Stop() after Stop() error = %v, want ErrSubscriptionClosed", err)
	}
}

// TestSubscription_MessageHandler verifies message handler wrapping.
func TestSubscription_MessageHandler(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-handler"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	received := make(chan *Message, 1)
	sub, err := client.Subscribe(context.Background(), "test.handler", func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer sub.Stop(context.Background())

	// Publish a message
	if err := client.Publish(context.Background(), "test.handler", []byte("test")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.Subject != "test.handler" {
			t.Errorf("message.Subject = %q, want %q", msg.Subject, "test.handler")
		}
		if string(msg.Data) != "test" {
			t.Errorf("message.Data = %q, want %q", string(msg.Data), "test")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestSubscription_HandlerCancellation verifies handlers detect context cancellation.
func TestSubscription_HandlerCancellation(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-cancellation"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Handler that blocks until context is cancelled
	handlerCalled := make(chan struct{})
	handlerDone := make(chan error, 1)

	sub, err := client.Subscribe(ctx, "test.cancel", func(ctx context.Context, msg *Message) error {
		close(handlerCalled)
		// Block until context is cancelled
		<-ctx.Done()
		handlerDone <- ctx.Err()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer sub.Stop(context.Background())

	// Publish a message to trigger handler
	if err := client.Publish(context.Background(), "test.cancel", []byte("test")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for handler to be called
	select {
	case <-handlerCalled:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler to be called")
	}

	// Cancel the parent context
	cancel()

	// Verify handler detects cancellation
	select {
	case err := <-handlerDone:
		if err != context.Canceled {
			t.Errorf("handler error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler to exit")
	}
}

// TestSubscription_StopCancelsHandlers verifies Stop() cancels in-flight handlers.
func TestSubscription_StopCancelsHandlers(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-stop-cancel"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Handler that blocks and checks context
	handlerCalled := make(chan struct{})
	handlerDone := make(chan error, 1)

	sub, err := client.Subscribe(context.Background(), "test.stop", func(ctx context.Context, msg *Message) error {
		close(handlerCalled)
		// Block until context is cancelled
		<-ctx.Done()
		handlerDone <- ctx.Err()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish a message to trigger handler
	if err := client.Publish(context.Background(), "test.stop", []byte("test")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Wait for handler to be called
	select {
	case <-handlerCalled:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler to be called")
	}

	// Stop the subscription (should cancel handlers)
	if err := sub.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Verify handler detected cancellation
	select {
	case err := <-handlerDone:
		if err != context.Canceled {
			t.Errorf("handler error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler to exit")
	}
}
