package nats

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Client provides a high-level interface to NATS with automatic reconnection,
// correlation ID propagation, metrics, and graceful shutdown capabilities.
type Client interface {
	// Core NATS operations
	Publish(ctx context.Context, subject string, data []byte, opts ...PubOption) error
	Subscribe(ctx context.Context, subject string, handler MsgHandler, opts ...SubOption) (*Subscription, error)
	Request(ctx context.Context, subject string, data []byte, opts ...ReqOption) (*Message, error)

	// JetStream access
	JS() (JSClient, error)

	// Health & lifecycle
	Health() HealthStatus
	Ready() error
	Drain(ctx context.Context) error
	Close() error

	// Raw access for advanced use cases
	Raw() *nats.Conn
}

// JSClient provides JetStream-specific operations for durable message processing.
type JSClient interface {
	EnsureStream(ctx context.Context, cfg StreamConfig) error
	EnsureConsumer(ctx context.Context, stream string, cfg ConsumerConfig) error
	PullConsume(ctx context.Context, cfg PullConsumerConfig, handler MsgHandler) (*Consumer, error)
	PublishAsync(ctx context.Context, subject string, data []byte, opts ...PubOption) (*PubAck, error)
}

// MsgHandler is called for each received message.
type MsgHandler func(ctx context.Context, msg *Message) error

// client implements the Client interface.
type client struct {
	conn    *nats.Conn
	config  *ClientConfig
	metrics *metricsCollector
	health  *healthTracker

	js     JSClient
	jsErr  error // Stores JetStream initialization error
	jsMu   sync.Mutex
	jsOnce sync.Once

	mu       sync.RWMutex
	closed   bool
	draining bool
}

// NewClient creates a new NATS client with the provided options.
func NewClient(opts ...Option) (Client, error) {
	cfg := defaultClientConfig()

	// Apply options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	c := &client{
		config:  cfg,
		metrics: newMetricsCollector(cfg),
		health:  newHealthTracker(),
	}

	// Build NATS connection options
	natsOpts := c.buildNatsOptions()

	// Connect to NATS with all configured URLs (comma-separated for failover)
	urls := strings.Join(cfg.URLs, ",")
	conn, err := nats.Connect(urls, natsOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	c.conn = conn
	c.health.setConnected()

	return c, nil
}

// buildNatsOptions constructs nats.Option slice from ClientConfig.
func (c *client) buildNatsOptions() []nats.Option {
	opts := []nats.Option{
		nats.Name(c.config.Name),
		nats.ReconnectWait(c.config.ReconnectWait),
		nats.MaxReconnects(c.config.MaxReconnects),
		nats.ReconnectBufSize(c.config.ReconnectBufSize),
		nats.Timeout(c.config.ConnectTimeout),
	}

	// Add credentials
	if c.config.Credentials != "" {
		opts = append(opts, nats.UserCredentials(c.config.Credentials))
	}
	if c.config.JWT != "" && c.config.NKey != "" {
		opts = append(opts, nats.UserJWTAndSeed(c.config.JWT, c.config.NKey))
	}

	// Add TLS
	if c.config.TLS != nil {
		opts = append(opts, nats.Secure(c.config.TLS))
	}

	// Add reconnection callbacks
	opts = append(opts,
		nats.ReconnectHandler(c.handleReconnected),
		nats.DisconnectErrHandler(c.handleDisconnected),
		nats.ClosedHandler(c.handleClosed),
	)

	return opts
}

// handleReconnected is called when the connection is re-established.
func (c *client) handleReconnected(nc *nats.Conn) {
	// Update metrics if enabled
	if c.metrics != nil && c.metrics.reconnects != nil {
		c.metrics.reconnects.Inc()
	}
	if c.metrics != nil && c.metrics.connectionUp != nil {
		c.metrics.connectionUp.Set(1)
	}

	c.health.setConnected()

	if c.config.ReconnectedCB != nil {
		c.config.ReconnectedCB(nc)
	}
}

// handleDisconnected is called when the connection is lost.
func (c *client) handleDisconnected(nc *nats.Conn, err error) {
	// Update metrics if enabled
	if c.metrics != nil && c.metrics.connectionUp != nil {
		c.metrics.connectionUp.Set(0)
	}

	c.health.setDisconnected(err)

	if c.config.DisconnectedCB != nil {
		c.config.DisconnectedCB(nc, err)
	}
}

// handleClosed is called when the connection is permanently closed.
func (c *client) handleClosed(nc *nats.Conn) {
	c.health.setClosed()

	if c.config.ClosedCB != nil {
		c.config.ClosedCB(nc)
	}
}

// Publish publishes data to the specified subject.
func (c *client) Publish(ctx context.Context, subject string, data []byte, opts ...PubOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	start := time.Now()

	// Apply options
	pubOpts := defaultPubOptions()
	for _, opt := range opts {
		opt(pubOpts)
	}

	// Create message with headers
	msg := nats.NewMsg(subject)
	msg.Header = nats.Header{} // Initialize header map to prevent nil panic
	msg.Data = data

	// Add correlation ID
	c.addCorrelationHeaders(ctx, msg, pubOpts)

	// Publish with retry
	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryBackoff.Next(attempt)):
			}
		}

		err := c.conn.PublishMsg(msg)
		if err == nil {
			c.metrics.recordPublish(subject, time.Since(start), nil)
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isTransientError(err) {
			c.metrics.recordPublish(subject, time.Since(start), err)
			return WrapPermanentError("publish", subject, err)
		}

		// Check context before retrying
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	c.metrics.recordPublish(subject, time.Since(start), lastErr)
	return WrapTransientError("publish", subject, lastErr)
}

// Subscribe creates a subscription to the specified subject.
func (c *client) Subscribe(ctx context.Context, subject string, handler MsgHandler, opts ...SubOption) (*Subscription, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	// Apply options
	subOpts := defaultSubOptions()
	for _, opt := range opts {
		opt(subOpts)
	}

	// Create subscription
	sub := newSubscription(c, subject, handler, subOpts)

	// Start the subscription
	if err := sub.start(ctx); err != nil {
		return nil, fmt.Errorf("start subscription: %w", err)
	}

	return sub, nil
}

// Request sends a request and waits for a response.
func (c *client) Request(ctx context.Context, subject string, data []byte, opts ...ReqOption) (*Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	start := time.Now()

	// Apply options
	reqOpts := defaultReqOptions()
	for _, opt := range opts {
		opt(reqOpts)
	}

	// Determine timeout
	timeout := c.config.RequestTimeout
	if reqOpts.timeout > 0 {
		timeout = reqOpts.timeout
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
		// Return immediately if context deadline has already passed
		if timeout <= 0 {
			return nil, context.DeadlineExceeded
		}
	}

	// Create request message with headers
	msg := nats.NewMsg(subject)
	msg.Header = nats.Header{} // Initialize header map to prevent nil panic
	msg.Data = data

	// Add correlation ID
	pubOpts := &pubOptions{correlationID: reqOpts.correlationID}
	c.addCorrelationHeaders(ctx, msg, pubOpts)

	// Send request
	resp, err := c.conn.RequestMsg(msg, timeout)
	duration := time.Since(start)

	if err != nil {
		c.metrics.recordRequest(subject, duration, err)
		if isTransientError(err) {
			return nil, WrapTransientError("request", subject, err)
		}
		return nil, WrapPermanentError("request", subject, err)
	}

	c.metrics.recordRequest(subject, duration, nil)

	return wrapNatsMessage(resp, c.config.CorrelationHeader), nil
}

// JS returns the JetStream client interface.
func (c *client) JS() (JSClient, error) {
	c.jsOnce.Do(func() {
		c.jsMu.Lock()
		defer c.jsMu.Unlock()

		// Create JetStream context
		js, err := c.conn.JetStream()
		if err != nil {
			// Store error for subsequent calls
			c.jsErr = fmt.Errorf("create jetstream context: %w", err)
			// Also store in health tracker
			c.health.recordError(c.jsErr)
			return
		}

		c.js = newJSClient(c, js)
	})

	return c.js, c.jsErr
}

// Health returns the current health status of the client.
func (c *client) Health() HealthStatus {
	return c.health.status()
}

// Ready returns an error if the client is not ready to handle traffic.
func (c *client) Ready() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	if !c.conn.IsConnected() {
		return fmt.Errorf("not connected to NATS server")
	}

	return nil
}

// Drain gracefully drains the connection with the configured timeout.
func (c *client) Drain(ctx context.Context) error {
	// Validate state and mark as draining
	conn, err := c.validateDrainState(ctx)
	if err != nil {
		return err
	}

	// Determine appropriate timeout
	timeout, err := c.determineDrainTimeout(ctx)
	if err != nil {
		c.clearDraining()
		return err
	}

	// Execute drain and wait for completion
	drainCompleted, err := c.waitForDrain(ctx, conn, timeout)

	// Update flags based on result
	c.mu.Lock()
	if drainCompleted {
		// Drain completed successfully - mark connection as closed
		c.draining = false
		c.closed = true
	} else {
		// Drain didn't complete (timeout/context) - keep draining flag set
		// and spawn goroutine to clear it when background drain finishes
		go c.waitForBackgroundDrain(conn)
	}
	c.mu.Unlock()

	return err
}

// waitForBackgroundDrain waits for a background drain to complete and clears the draining flag
func (c *client) waitForBackgroundDrain(conn *nats.Conn) {
	// Poll connection status until drain completes
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		// Check if drain completed (connection is no longer draining or is closed)
		if !conn.IsDraining() || conn.IsClosed() {
			c.mu.Lock()
			c.draining = false
			if conn.IsClosed() {
				c.closed = true
			}
			c.mu.Unlock()
			return
		}
	}
}

// validateDrainState checks if drain can proceed and marks client as draining
func (c *client) validateDrainState(ctx context.Context) (*nats.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already closed or draining
	if c.closed {
		return nil, ErrClientClosed
	}
	if c.draining {
		return nil, fmt.Errorf("drain already in progress")
	}
	if c.conn == nil {
		return nil, fmt.Errorf("no connection established")
	}

	// Also check if NATS connection itself is already draining (prevents re-entry)
	if c.conn.IsDraining() {
		return nil, fmt.Errorf("NATS connection already draining")
	}

	// Check context before starting drain
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Mark as draining and capture connection reference
	c.draining = true
	return c.conn, nil
}

// determineDrainTimeout calculates the appropriate timeout from config and context
func (c *client) determineDrainTimeout(ctx context.Context) (time.Duration, error) {
	timeout := c.config.DrainTimeout
	deadline, ok := ctx.Deadline()
	if !ok {
		return timeout, nil
	}

	timeUntilDeadline := time.Until(deadline)

	// Check if context deadline has already passed
	if timeUntilDeadline <= 0 {
		return 0, ctx.Err()
	}

	// Use the shorter of configured timeout or time until deadline
	if timeout <= 0 || timeUntilDeadline < timeout {
		timeout = timeUntilDeadline
	}

	return timeout, nil
}

// waitForDrain executes the drain operation and waits for completion
func (c *client) waitForDrain(ctx context.Context, conn *nats.Conn, timeout time.Duration) (bool, error) {
	// Create done channel for drain result
	done := make(chan error, 1)

	// Start drain in goroutine
	go func() {
		done <- conn.Drain()
	}()

	// Wait for drain with proper timeout and context handling
	var err error
	var drainCompleted bool

	if timeout > 0 {
		select {
		case err = <-done:
			drainCompleted = true
		case <-time.After(timeout):
			err = fmt.Errorf("drain timeout after %v", timeout)
		case <-ctx.Done():
			err = ctx.Err()
		}
	} else {
		// No timeout - wait for drain or context cancellation
		select {
		case err = <-done:
			drainCompleted = true
		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	return drainCompleted, err
}

// clearDraining safely clears the draining flag
func (c *client) clearDraining() {
	c.mu.Lock()
	c.draining = false
	c.mu.Unlock()
}

// Close immediately closes the connection.
// Note: If Drain() is in progress, Close() returns an error so callers can wait
// for the graceful drain to finish before closing.
func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrClientClosed
	}

	// If drain is in progress, we need to wait for it to complete.
	// This prevents Close() from interfering with the graceful drain.
	// We can't wait while holding the lock, so we'll just check and return
	// an error telling the caller to wait for drain to complete.
	if c.draining {
		return fmt.Errorf("drain in progress, wait for drain to complete before closing")
	}

	c.closed = true
	c.conn.Close()
	return nil
}

// Raw returns the underlying nats.Conn for advanced use cases.
func (c *client) Raw() *nats.Conn {
	return c.conn
}

// addCorrelationHeaders adds correlation and tracing headers to the message.
func (c *client) addCorrelationHeaders(ctx context.Context, msg *nats.Msg, opts *pubOptions) {
	// Get or generate correlation ID
	correlationID := opts.correlationID
	if correlationID == "" {
		correlationID = c.config.GenerateID()
	}

	// Set correlation header
	msg.Header.Set(c.config.CorrelationHeader, correlationID)

	// Set traceparent header if available
	if traceparent := extractTraceparent(ctx); traceparent != "" {
		msg.Header.Set(c.config.TraceparentHeader, traceparent)
	}
}

// extractTraceparent extracts W3C traceparent from context.
// TODO: Implement proper tracing integration (OpenTelemetry)
func extractTraceparent(ctx context.Context) string {
	// Placeholder for tracing integration
	return ""
}

// isTransientError checks if an error should be retried.
func isTransientError(err error) bool {
	switch err {
	case nats.ErrTimeout,
		nats.ErrNoResponders,
		nats.ErrConnectionClosed:
		return true
	}
	return false
}

// defaultGenerateID generates a correlation ID using UUID v4.
func defaultGenerateID() string {
	return uuid.New().String()
}
