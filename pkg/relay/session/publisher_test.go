package session_test

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/stretchr/testify/assert"
)

func TestNoOpPublisher(t *testing.T) {
	pub := session.NewNoOpPublisher()
	ctx := context.Background()

	// All methods should succeed and do nothing
	assert.NoError(t, pub.PublishSessionCreated(ctx, "test-session"))
	assert.NoError(t, pub.PublishSessionTerminated(ctx, "test-session"))
	assert.NoError(t, pub.PublishAgentSpawned(ctx, "test-session", "coder", "/workspace"))
	assert.NoError(t, pub.PublishAgentTerminated(ctx, "test-session", "coder", 0))
}

func TestNoOpPublisher_Concurrent(t *testing.T) {
	pub := session.NewNoOpPublisher()
	ctx := context.Background()

	// Test concurrent calls don't panic
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			pub.PublishSessionCreated(ctx, "test")
			pub.PublishSessionTerminated(ctx, "test")
			pub.PublishAgentSpawned(ctx, "test", "role", "/ws")
			pub.PublishAgentTerminated(ctx, "test", "role", 0)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
