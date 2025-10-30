package nats

import (
	"github.com/nats-io/nats.go"
)

// Message wraps a NATS message with additional metadata.
type Message struct {
	Subject string
	Data    []byte
	Headers map[string]string

	// Correlation ID extracted from headers
	CorrelationID string

	// Internal NATS message for ack/nak operations
	msg *nats.Msg
}

// wrapNatsMessage creates a Message from a nats.Msg.
func wrapNatsMessage(msg *nats.Msg) *Message {
	m := &Message{
		Subject: msg.Subject,
		Data:    msg.Data,
		Headers: make(map[string]string),
		msg:     msg,
	}

	// Extract headers
	for key := range msg.Header {
		m.Headers[key] = msg.Header.Get(key)
	}

	// Extract correlation ID
	m.CorrelationID = msg.Header.Get("Correlation-Id")

	return m
}

// Ack acknowledges the message (for JetStream).
func (m *Message) Ack() error {
	if m.msg == nil {
		return nil
	}
	return m.msg.Ack()
}

// Nak negatively acknowledges the message (for JetStream).
func (m *Message) Nak() error {
	if m.msg == nil {
		return nil
	}
	return m.msg.Nak()
}

// Term terminates the message processing (for JetStream).
func (m *Message) Term() error {
	if m.msg == nil {
		return nil
	}
	return m.msg.Term()
}

// InProgress indicates that work is ongoing (for JetStream).
func (m *Message) InProgress() error {
	if m.msg == nil {
		return nil
	}
	return m.msg.InProgress()
}

// Respond sends a response to the message's reply subject.
func (m *Message) Respond(data []byte) error {
	if m.msg == nil {
		return nil
	}
	return m.msg.Respond(data)
}
