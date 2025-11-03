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
