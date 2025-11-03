package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Subscription represents an active subscription.
type Subscription struct {
	client  *client
	subject string
	handler MsgHandler
	opts    *subOptions

	natsSub *nats.Subscription

	mu     sync.RWMutex
	closed bool
	stopCh chan struct{}
}

// newSubscription creates a new subscription.
func newSubscription(c *client, subject string, handler MsgHandler, opts *subOptions) *Subscription {
	return &Subscription{
		client:  c,
		subject: subject,
		handler: handler,
		opts:    opts,
		stopCh:  make(chan struct{}),
	}
}

// start begins the subscription.
func (s *Subscription) start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSubscriptionClosed
	}

	// Create NATS subscription
	var err error
	if s.opts.queueGroup != "" {
		s.natsSub, err = s.client.conn.QueueSubscribe(s.subject, s.opts.queueGroup, s.messageHandler)
	} else {
		s.natsSub, err = s.client.conn.Subscribe(s.subject, s.messageHandler)
	}

	if err != nil {
		return fmt.Errorf("subscribe to %q: %w", s.subject, err)
	}

	// Set pending limits from options (defaults: 524,288 msgs (512 KiMsgs), 64 MiB bytes)
	if err := s.natsSub.SetPendingLimits(s.opts.pendingLimitMsgs, s.opts.pendingLimitBytes); err != nil {
		_ = s.natsSub.Unsubscribe()
		return fmt.Errorf("set pending limits: %w", err)
	}

	return nil
}

// messageHandler wraps the user handler with metrics and error handling.
func (s *Subscription) messageHandler(msg *nats.Msg) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	start := time.Now()

	// Wrap message
	wrappedMsg := wrapNatsMessage(msg)

	// Create context for handler
	ctx := context.Background()

	// Call user handler
	err := s.handler(ctx, wrappedMsg)

	// Record metrics
	s.client.metrics.recordMessageReceived(s.subject, time.Since(start), err)
}

// Stop gracefully stops the subscription.
func (s *Subscription) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSubscriptionClosed
	}

	s.closed = true
	close(s.stopCh)

	if s.natsSub != nil {
		// Unsubscribe and drain
		if err := s.natsSub.Drain(); err != nil {
			return fmt.Errorf("drain subscription: %w", err)
		}
	}

	return nil
}

// Subject returns the subscription's subject.
func (s *Subscription) Subject() string {
	return s.subject
}

// IsValid returns true if the subscription is active.
func (s *Subscription) IsValid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return !s.closed && s.natsSub != nil && s.natsSub.IsValid()
}
