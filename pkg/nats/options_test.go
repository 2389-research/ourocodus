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
			opt:  WithRetryBackoff(newExponentialBackoff(100*time.Millisecond, time.Second, nil)),
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
	backoff := newExponentialBackoff(100*time.Millisecond, time.Second, nil)

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

// TestExponentialBackoff_DeterministicJitter verifies deterministic jitter calculation.
func TestExponentialBackoff_DeterministicJitter(t *testing.T) {
	// Use fixed random source returning 0.5
	backoff := newExponentialBackoff(100*time.Millisecond, 5*time.Second, fixedRandomSource{0.5})

	// First attempt: base = 100ms, jitter = 100ms * 0.25 * (0.5 + 0.5*0.5) = 18.75ms
	// Total = 100ms + 18.75ms = 118.75ms
	duration := backoff.Next(1)
	expected := 100*time.Millisecond + time.Duration(float64(100*time.Millisecond)*0.25*(0.5+0.5*0.5))
	if duration != expected {
		t.Errorf("jitter calculation with fixed random should be deterministic: got %v, want %v", duration, expected)
	}

	// Reset and verify consistency
	backoff.Reset()
	duration2 := backoff.Next(1)
	if duration2 != expected {
		t.Errorf("fixed random should produce same result: got %v, want %v", duration2, expected)
	}
}

// TestExponentialBackoff_RandomJitter verifies jitter varies across multiple attempts.
func TestExponentialBackoff_RandomJitter(t *testing.T) {
	backoff := newExponentialBackoff(100*time.Millisecond, 5*time.Second, nil)

	durations := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		backoff.Reset()
		durations[backoff.Next(1)] = true
	}

	if len(durations) < 10 {
		t.Errorf("jitter should produce varied delays: got %d unique values, want >= 10", len(durations))
	}
}

// TestExponentialBackoff_JitterBounds verifies jitter stays within 12.5-25% bounds.
func TestExponentialBackoff_JitterBounds(t *testing.T) {
	tests := []struct {
		name        string
		randomValue float64
	}{
		{"min random (0.0)", 0.0}, // 0.25 * (0.5 + 0.5*0.0) = 0.125 -> 12.5%
		{"mid random (0.5)", 0.5}, // 0.25 * (0.5 + 0.5*0.5) = 0.1875 -> 18.75%
		{"max random (1.0)", 1.0}, // 0.25 * (0.5 + 0.5*1.0) = 0.25 -> 25%
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := newExponentialBackoff(100*time.Millisecond, 5*time.Second, fixedRandomSource{tt.randomValue})

			duration := backoff.Next(1)
			baseBackoff := 100 * time.Millisecond

			// Calculate expected jitter percentage
			jitterPercent := float64(duration-baseBackoff) / float64(baseBackoff)
			expectedPercent := 0.25 * (0.5 + 0.5*tt.randomValue)

			// Allow small floating point tolerance (0.1%)
			tolerance := 0.001
			if jitterPercent < expectedPercent-tolerance || jitterPercent > expectedPercent+tolerance {
				t.Errorf("jitter should be within bounds for random=%f: got %.4f%%, want %.4f%%",
					tt.randomValue, jitterPercent*100, expectedPercent*100)
			}
		})
	}
}
