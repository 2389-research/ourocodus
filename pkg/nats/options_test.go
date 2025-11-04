package nats

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// TestOptions verifies various configuration options.
func TestOptions(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
		test func(*testing.T, *ClientConfig)
	}{
		{
			name: "WithURL-multiple",
			opt:  WithURL("nats://server1:4222", "nats://server2:4222"),
			test: func(t *testing.T, cfg *ClientConfig) {
				if len(cfg.URLs) != 2 {
					t.Errorf("len(URLs) = %d, want 2", len(cfg.URLs))
				}
			},
		},
		{
			name: "WithCredentials",
			opt:  WithCredentials("/path/to/creds"),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Credentials != "/path/to/creds" {
					t.Errorf("Credentials = %q, want %q", cfg.Credentials, "/path/to/creds")
				}
			},
		},
		{
			name: "WithJWT",
			opt:  WithJWT("jwt-token", "nkey-seed"),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.JWT != "jwt-token" {
					t.Errorf("JWT = %q, want %q", cfg.JWT, "jwt-token")
				}
				if cfg.NKey != "nkey-seed" {
					t.Errorf("NKey = %q, want %q", cfg.NKey, "nkey-seed")
				}
			},
		},
		{
			name: "WithTLS",
			opt:  WithTLS(&tls.Config{ServerName: "test"}),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.TLS == nil {
					t.Fatal("TLS = nil, want non-nil")
				}
				if cfg.TLS.ServerName != "test" {
					t.Errorf("TLS.ServerName = %q, want %q", cfg.TLS.ServerName, "test")
				}
			},
		},
		{
			name: "WithRetryAttempts",
			opt:  WithRetryAttempts(5),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.RetryAttempts != 5 {
					t.Errorf("RetryAttempts = %d, want %d", cfg.RetryAttempts, 5)
				}
			},
		},
		{
			name: "WithRetryBackoff",
			opt:  WithRetryBackoff(newExponentialBackoff(100*time.Millisecond, time.Second)),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.RetryBackoff == nil {
					t.Fatal("RetryBackoff = nil, want non-nil")
				}
			},
		},
		{
			name: "WithCorrelationHeader",
			opt:  WithCorrelationHeader("X-Request-ID"),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.CorrelationHeader != "X-Request-ID" {
					t.Errorf("CorrelationHeader = %q, want %q", cfg.CorrelationHeader, "X-Request-ID")
				}
			},
		},
		{
			name: "WithIDGenerator",
			opt: WithIDGenerator(func() string {
				return "custom-id"
			}),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.GenerateID == nil {
					t.Fatal("GenerateID = nil, want non-nil")
				}
				if id := cfg.GenerateID(); id != "custom-id" {
					t.Errorf("GenerateID() = %q, want %q", id, "custom-id")
				}
			},
		},
		{
			name: "WithMetrics",
			opt:  WithMetrics("custom", "subsys"),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.MetricsNamespace != "custom" {
					t.Errorf("MetricsNamespace = %q, want %q", cfg.MetricsNamespace, "custom")
				}
				if cfg.MetricsSubsystem != "subsys" {
					t.Errorf("MetricsSubsystem = %q, want %q", cfg.MetricsSubsystem, "subsys")
				}
				if !cfg.MetricsEnabled {
					t.Error("MetricsEnabled = false, want true")
				}
			},
		},
		{
			name: "WithMetricsDisabled",
			opt:  WithMetricsDisabled(),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.MetricsEnabled {
					t.Error("MetricsEnabled = true, want false")
				}
			},
		},
		{
			name: "WithDisconnectedCallback",
			opt: WithDisconnectedCallback(func(nc *nats.Conn, err error) {
				// callback
			}),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.DisconnectedCB == nil {
					t.Fatal("DisconnectedCB = nil, want non-nil")
				}
			},
		},
		{
			name: "WithClosedCallback",
			opt: WithClosedCallback(func(nc *nats.Conn) {
				// callback
			}),
			test: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ClosedCB == nil {
					t.Fatal("ClosedCB = nil, want non-nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultClientConfig()
			if err := tt.opt(cfg); err != nil {
				t.Fatalf("option() error = %v", err)
			}
			tt.test(t, cfg)
		})
	}
}

// TestSubOptions verifies subscription options.
func TestSubOptions(t *testing.T) {
	opts := defaultSubOptions()

	WithQueueGroup("workers")(opts)
	if opts.queueGroup != "workers" {
		t.Errorf("queueGroup = %q, want %q", opts.queueGroup, "workers")
	}

	WithMaxInflight(100)(opts)
	if opts.maxInflight != 100 {
		t.Errorf("maxInflight = %d, want %d", opts.maxInflight, 100)
	}
}

// TestReqOptions verifies request options.
func TestReqOptions(t *testing.T) {
	opts := defaultReqOptions()

	WithTimeout(5 * time.Second)(opts)
	if opts.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want %v", opts.timeout, 5*time.Second)
	}

	WithRequestCorrelationID("test-123")(opts)
	if opts.correlationID != "test-123" {
		t.Errorf("correlationID = %q, want %q", opts.correlationID, "test-123")
	}
}

// TestExponentialBackoff verifies backoff strategy.
func TestExponentialBackoff(t *testing.T) {
	backoff := newExponentialBackoff(100*time.Millisecond, time.Second)

	// First attempt should be close to initial
	d1 := backoff.Next(1)
	if d1 < 50*time.Millisecond || d1 > 200*time.Millisecond {
		t.Errorf("Next(1) = %v, want ~100ms", d1)
	}

	// Second attempt should be higher (exponential)
	d2 := backoff.Next(2)
	if d2 <= d1 {
		t.Errorf("Next(2) = %v, should be > Next(1) = %v", d2, d1)
	}

	// Should be close to max (may have jitter)
	d10 := backoff.Next(10)
	// Allow up to 1.5x max due to jitter
	if d10 > time.Second*3/2 {
		t.Errorf("Next(10) = %v, should be close to max = 1s", d10)
	}

	// Reset should work
	backoff.Reset()
	d1_after := backoff.Next(1)
	if d1_after < 50*time.Millisecond || d1_after > 200*time.Millisecond {
		t.Errorf("Next(1) after Reset() = %v, want ~100ms", d1_after)
	}
}

// TestConfigValidation verifies configuration validation rules.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*ClientConfig)
		wantErr   bool
	}{
		{
			name:      "valid config",
			setupFunc: func(cfg *ClientConfig) {},
			wantErr:   false,
		},
		{
			name: "empty URLs",
			setupFunc: func(cfg *ClientConfig) {
				cfg.URLs = []string{}
			},
			wantErr: true,
		},
		{
			name: "empty name",
			setupFunc: func(cfg *ClientConfig) {
				cfg.Name = ""
			},
			wantErr: true,
		},
		{
			name: "zero connect timeout",
			setupFunc: func(cfg *ClientConfig) {
				cfg.ConnectTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "zero request timeout",
			setupFunc: func(cfg *ClientConfig) {
				cfg.RequestTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "zero drain timeout",
			setupFunc: func(cfg *ClientConfig) {
				cfg.DrainTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "negative retry attempts",
			setupFunc: func(cfg *ClientConfig) {
				cfg.RetryAttempts = -1
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultClientConfig()
			tt.setupFunc(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultSubOptions_PendingLimits(t *testing.T) {
	opts := defaultSubOptions()

	// Verify NATS default values
	if opts.pendingLimitMsgs != 512*1024 {
		t.Errorf("pendingLimitMsgs = %d, want %d (NATS default)", opts.pendingLimitMsgs, 512*1024)
	}
	if opts.pendingLimitBytes != 64*1024*1024 {
		t.Errorf("pendingLimitBytes = %d, want %d (NATS default)", opts.pendingLimitBytes, 64*1024*1024)
	}
}

func TestWithPendingLimits(t *testing.T) {
	opts := defaultSubOptions()

	// Apply custom limits
	WithPendingLimits(1000, 5*1024*1024)(opts)

	if opts.pendingLimitMsgs != 1000 {
		t.Errorf("pendingLimitMsgs = %d, want %d", opts.pendingLimitMsgs, 1000)
	}
	if opts.pendingLimitBytes != 5*1024*1024 {
		t.Errorf("pendingLimitBytes = %d, want %d", opts.pendingLimitBytes, 5*1024*1024)
	}
}

func TestWithUnlimitedPending(t *testing.T) {
	opts := defaultSubOptions()

	// Apply unlimited
	WithUnlimitedPending()(opts)

	if opts.pendingLimitMsgs != -1 {
		t.Errorf("pendingLimitMsgs = %d, want -1 (unlimited)", opts.pendingLimitMsgs)
	}
	if opts.pendingLimitBytes != -1 {
		t.Errorf("pendingLimitBytes = %d, want -1 (unlimited)", opts.pendingLimitBytes)
	}
}

func TestWithPendingLimits_NegativeValues(t *testing.T) {
	opts := defaultSubOptions()

	// Can explicitly set -1 via WithPendingLimits
	WithPendingLimits(-1, -1)(opts)

	if opts.pendingLimitMsgs != -1 {
		t.Errorf("pendingLimitMsgs = %d, want -1", opts.pendingLimitMsgs)
	}
	if opts.pendingLimitBytes != -1 {
		t.Errorf("pendingLimitBytes = %d, want -1", opts.pendingLimitBytes)
	}
}
