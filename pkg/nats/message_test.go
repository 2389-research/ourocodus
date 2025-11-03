package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
)

// TestMessage_Respond verifies message response.
func TestMessage_Respond(t *testing.T) {
	srv := runTestServer(t)
	defer srv.Shutdown()

	client, err := NewClient(
		WithURL(srv.ClientURL()),
		WithName("test-message"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// Set up responder
	_, err = client.Subscribe(context.Background(), "test.respond", func(ctx context.Context, msg *Message) error {
		return msg.Respond([]byte("pong"))
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Send request
	resp, err := client.Request(context.Background(), "test.respond", []byte("ping"))
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if string(resp.Data) != "pong" {
		t.Errorf("response.Data = %q, want %q", string(resp.Data), "pong")
	}
}

// TestMessage_AckNak tests message acknowledgement methods.
// Note: These methods are no-ops for non-JetStream messages.
func TestMessage_AckNak(t *testing.T) {
	msg := &Message{
		Subject: "test",
		Data:    []byte("test"),
		Headers: make(map[string]string),
		msg:     nil, // Non-JetStream message
	}

	// These should not error on nil msg
	if err := msg.Ack(); err != nil {
		t.Errorf("Ack() error = %v", err)
	}

	if err := msg.Nak(); err != nil {
		t.Errorf("Nak() error = %v", err)
	}

	if err := msg.Term(); err != nil {
		t.Errorf("Term() error = %v", err)
	}

	if err := msg.InProgress(); err != nil {
		t.Errorf("InProgress() error = %v", err)
	}
}

// TestMessage_WrapNatsMessage verifies message wrapping.
func TestMessage_WrapNatsMessage(t *testing.T) {
	natsMsg := nats.NewMsg("test.subject")
	natsMsg.Data = []byte("test data")
	natsMsg.Header = nats.Header{}
	natsMsg.Header.Set("Correlation-Id", "test-123")
	natsMsg.Header.Set("Custom-Header", "custom-value")

	wrapped := wrapNatsMessage(natsMsg, "Correlation-Id")

	if wrapped.Subject != "test.subject" {
		t.Errorf("Subject = %q, want %q", wrapped.Subject, "test.subject")
	}

	if string(wrapped.Data) != "test data" {
		t.Errorf("Data = %q, want %q", string(wrapped.Data), "test data")
	}

	if wrapped.CorrelationID != "test-123" {
		t.Errorf("CorrelationID = %q, want %q", wrapped.CorrelationID, "test-123")
	}

	if wrapped.Headers["Custom-Header"] != "custom-value" {
		t.Errorf("Headers[Custom-Header] = %q, want %q", wrapped.Headers["Custom-Header"], "custom-value")
	}
}

// TestMessage_WrapNatsMessage_CustomCorrelationHeader verifies custom correlation header extraction.
func TestMessage_WrapNatsMessage_CustomCorrelationHeader(t *testing.T) {
	natsMsg := nats.NewMsg("test.subject")
	natsMsg.Header = nats.Header{}
	natsMsg.Header.Set("X-Custom-Correlation", "custom-id-456")

	wrapped := wrapNatsMessage(natsMsg, "X-Custom-Correlation")

	if wrapped.CorrelationID != "custom-id-456" {
		t.Errorf("CorrelationID = %q, want %q", wrapped.CorrelationID, "custom-id-456")
	}
}
