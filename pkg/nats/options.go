package nats

import (
	"crypto/tls"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

// ClientConfig holds the configuration for the NATS client.
type ClientConfig struct {
	URLs              []string
	Name              string
	Credentials       string
	JWT               string
	NKey              string
	TLS               *tls.Config
	ReconnectWait     time.Duration
	MaxReconnects     int
	ReconnectBufSize  int
	ConnectTimeout    time.Duration
	RequestTimeout    time.Duration
	DrainTimeout      time.Duration
	RetryAttempts     int
	RetryBackoff      BackoffStrategy
	CorrelationHeader string
	TraceparentHeader string
	GenerateID        func() string
	MetricsNamespace  string
	MetricsSubsystem  string
	MetricsEnabled    bool
	ReconnectedCB     func(*nats.Conn)
	DisconnectedCB    func(*nats.Conn, error)
	ClosedCB          func(*nats.Conn)
}

// Validate checks if the configuration is valid.
func (c *ClientConfig) Validate() error {
	if len(c.URLs) == 0 {
		return fmt.Errorf("at least one NATS URL is required")
	}
	if c.Name == "" {
		return fmt.Errorf("client name is required")
	}
	if c.ConnectTimeout <= 0 {
		return fmt.Errorf("connect timeout must be positive")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if c.DrainTimeout <= 0 {
		return fmt.Errorf("drain timeout must be positive")
	}
	if c.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}
	return nil
}

// defaultClientConfig returns a ClientConfig with sensible defaults.
func defaultClientConfig() *ClientConfig {
	return &ClientConfig{
		URLs:              []string{getEnv("NATS_URL", nats.DefaultURL)},
		Name:              "nats-client",
		Credentials:       getEnv("NATS_CREDENTIALS", ""),
		ReconnectWait:     getEnvDuration("NATS_RECONNECT_WAIT", 2*time.Second),
		MaxReconnects:     getEnvInt("NATS_MAX_RECONNECTS", -1),
		ReconnectBufSize:  8 * 1024 * 1024, // 8MB
		ConnectTimeout:    getEnvDuration("NATS_CONNECT_TIMEOUT", 10*time.Second),
		RequestTimeout:    getEnvDuration("NATS_REQUEST_TIMEOUT", 5*time.Second),
		DrainTimeout:      30 * time.Second,
		RetryAttempts:     3,
		RetryBackoff:      newExponentialBackoff(200*time.Millisecond, 5*time.Second),
		CorrelationHeader: "Correlation-Id",
		TraceparentHeader: "traceparent",
		GenerateID:        defaultGenerateID,
		MetricsNamespace:  "nats_client",
		MetricsSubsystem:  "",
		MetricsEnabled:    true,
	}
}

// Option is a functional option for configuring the client.
type Option func(*ClientConfig) error

// WithURL sets the NATS server URL(s).
func WithURL(urls ...string) Option {
	return func(c *ClientConfig) error {
		if len(urls) == 0 {
			return fmt.Errorf("at least one URL is required")
		}
		c.URLs = urls
		return nil
	}
}

// WithName sets the client name.
func WithName(name string) Option {
	return func(c *ClientConfig) error {
		c.Name = name
		return nil
	}
}

// WithCredentials sets the credentials file path.
func WithCredentials(path string) Option {
	return func(c *ClientConfig) error {
		c.Credentials = path
		return nil
	}
}

// WithJWT sets the JWT and NKey for authentication.
func WithJWT(jwt, nkey string) Option {
	return func(c *ClientConfig) error {
		c.JWT = jwt
		c.NKey = nkey
		return nil
	}
}

// WithTLS sets the TLS configuration.
func WithTLS(cfg *tls.Config) Option {
	return func(c *ClientConfig) error {
		c.TLS = cfg
		return nil
	}
}

// WithReconnectWait sets the reconnection wait time.
func WithReconnectWait(wait time.Duration) Option {
	return func(c *ClientConfig) error {
		c.ReconnectWait = wait
		return nil
	}
}

// WithMaxReconnects sets the maximum number of reconnection attempts (-1 for unlimited).
func WithMaxReconnects(max int) Option {
	return func(c *ClientConfig) error {
		c.MaxReconnects = max
		return nil
	}
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(timeout time.Duration) Option {
	return func(c *ClientConfig) error {
		c.ConnectTimeout = timeout
		return nil
	}
}

// WithRequestTimeout sets the default request timeout.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *ClientConfig) error {
		c.RequestTimeout = timeout
		return nil
	}
}

// WithDrainTimeout sets the drain timeout.
func WithDrainTimeout(timeout time.Duration) Option {
	return func(c *ClientConfig) error {
		c.DrainTimeout = timeout
		return nil
	}
}

// WithRetryAttempts sets the number of retry attempts for transient errors.
func WithRetryAttempts(attempts int) Option {
	return func(c *ClientConfig) error {
		if attempts < 0 {
			return fmt.Errorf("retry attempts cannot be negative")
		}
		c.RetryAttempts = attempts
		return nil
	}
}

// WithRetryBackoff sets the backoff strategy for retries.
func WithRetryBackoff(strategy BackoffStrategy) Option {
	return func(c *ClientConfig) error {
		c.RetryBackoff = strategy
		return nil
	}
}

// WithCorrelationHeader sets the correlation ID header name.
func WithCorrelationHeader(name string) Option {
	return func(c *ClientConfig) error {
		c.CorrelationHeader = name
		return nil
	}
}

// WithIDGenerator sets the correlation ID generator function.
func WithIDGenerator(fn func() string) Option {
	return func(c *ClientConfig) error {
		c.GenerateID = fn
		return nil
	}
}

// WithMetrics configures metrics collection.
func WithMetrics(namespace, subsystem string) Option {
	return func(c *ClientConfig) error {
		c.MetricsNamespace = namespace
		c.MetricsSubsystem = subsystem
		c.MetricsEnabled = true
		return nil
	}
}

// WithMetricsDisabled disables metrics collection.
func WithMetricsDisabled() Option {
	return func(c *ClientConfig) error {
		c.MetricsEnabled = false
		return nil
	}
}

// WithReconnectedCallback sets the reconnection callback.
func WithReconnectedCallback(cb func(*nats.Conn)) Option {
	return func(c *ClientConfig) error {
		c.ReconnectedCB = cb
		return nil
	}
}

// WithDisconnectedCallback sets the disconnection callback.
func WithDisconnectedCallback(cb func(*nats.Conn, error)) Option {
	return func(c *ClientConfig) error {
		c.DisconnectedCB = cb
		return nil
	}
}

// WithClosedCallback sets the closed callback.
func WithClosedCallback(cb func(*nats.Conn)) Option {
	return func(c *ClientConfig) error {
		c.ClosedCB = cb
		return nil
	}
}

// PubOption is a functional option for Publish operations.
type PubOption func(*pubOptions)

type pubOptions struct {
	correlationID string
}

func defaultPubOptions() *pubOptions {
	return &pubOptions{}
}

// WithCorrelationID sets a specific correlation ID for the message.
func WithCorrelationID(id string) PubOption {
	return func(opts *pubOptions) {
		opts.correlationID = id
	}
}

// SubOption is a functional option for Subscribe operations.
type SubOption func(*subOptions)

type subOptions struct {
	queueGroup  string
	maxInflight int
}

func defaultSubOptions() *subOptions {
	return &subOptions{
		maxInflight: 1,
	}
}

// WithQueueGroup sets the queue group for the subscription.
func WithQueueGroup(group string) SubOption {
	return func(opts *subOptions) {
		opts.queueGroup = group
	}
}

// WithMaxInflight sets the maximum number of in-flight messages.
func WithMaxInflight(max int) SubOption {
	return func(opts *subOptions) {
		opts.maxInflight = max
	}
}

// ReqOption is a functional option for Request operations.
type ReqOption func(*reqOptions)

type reqOptions struct {
	timeout       time.Duration
	correlationID string
}

func defaultReqOptions() *reqOptions {
	return &reqOptions{}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) ReqOption {
	return func(opts *reqOptions) {
		opts.timeout = timeout
	}
}

// WithRequestCorrelationID sets a specific correlation ID for the request.
func WithRequestCorrelationID(id string) ReqOption {
	return func(opts *reqOptions) {
		opts.correlationID = id
	}
}

// BackoffStrategy defines the interface for retry backoff strategies.
type BackoffStrategy interface {
	Next(attempt int) time.Duration
	Reset()
}

// RandomSource provides random number generation for jitter calculations.
type RandomSource interface {
	Float64() float64
}

// defaultRandomSource uses the global math/rand/v2 random source.
type defaultRandomSource struct{}

func (defaultRandomSource) Float64() float64 {
	return rand.Float64()
}

// fixedRandomSource always returns the same value (for testing).
type fixedRandomSource struct {
	value float64
}

func (f fixedRandomSource) Float64() float64 {
	return f.value
}

// exponentialBackoff implements exponential backoff with jitter.
type exponentialBackoff struct {
	initial time.Duration
	max     time.Duration
}

// newExponentialBackoff creates a new exponential backoff strategy.
func newExponentialBackoff(initial, max time.Duration) BackoffStrategy {
	return &exponentialBackoff{
		initial: initial,
		max:     max,
	}
}

// Next returns the next backoff duration.
func (e *exponentialBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff: initial * 2^(attempt-1)
	// Limit attempt to prevent overflow
	exp := attempt - 1
	if exp > 30 {
		exp = 30 // Cap at 2^30 to prevent overflow
	}
	backoff := e.initial * (1 << exp)
	if backoff > e.max {
		backoff = e.max
	}

	// Add jitter: 0-25% of backoff
	jitter := time.Duration(float64(backoff) * 0.25 * (0.5 + (0.5 * randomFloat())))

	return backoff + jitter
}

// Reset is a no-op for exponential backoff.
func (e *exponentialBackoff) Reset() {}

// Helper functions for environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// randomFloat returns a random float64 in [0.0, 1.0).
// This is a simple implementation; consider using math/rand for production.
func randomFloat() float64 {
	return 0.5 // Placeholder - implement proper random for production
}
