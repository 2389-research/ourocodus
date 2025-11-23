package ratelimit

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrRateLimitExceeded is returned when a rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Limiter implements a token bucket rate limiter
// Thread-safe for concurrent use
type Limiter struct {
	// maxTokens is the maximum number of tokens in the bucket
	maxTokens int

	// refillRate is how many tokens to add per second
	refillRate int

	// buckets maps user session ID to their token bucket
	buckets map[string]*bucket

	mu sync.RWMutex
}

// bucket represents a token bucket for a single user session
type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewLimiter creates a new rate limiter
// maxTokens: maximum number of tokens (burst capacity)
// refillRate: tokens to add per second
//
// Example: NewLimiter(10, 1) allows bursts of 10 requests, then 1 per second
func NewLimiter(maxTokens, refillRate int) *Limiter {
	return &Limiter{
		maxTokens:  maxTokens,
		refillRate: refillRate,
		buckets:    make(map[string]*bucket),
	}
}

// Allow checks if a request from the given user session should be allowed
// Returns true if allowed, false if rate limit exceeded
func (l *Limiter) Allow(userSessionID string) bool {
	l.mu.RLock()
	b, exists := l.buckets[userSessionID]
	l.mu.RUnlock()

	if !exists {
		// First request from this session - create bucket
		l.mu.Lock()
		// Double-check after acquiring write lock
		if b, exists = l.buckets[userSessionID]; !exists {
			b = &bucket{
				tokens:     l.maxTokens - 1, // Consume 1 token for this request
				lastRefill: time.Now(),
			}
			l.buckets[userSessionID] = b
			l.mu.Unlock()
			return true
		}
		l.mu.Unlock()
	}

	// Refill tokens based on time elapsed
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(l.refillRate))

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > l.maxTokens {
			b.tokens = l.maxTokens
		}
		b.lastRefill = now
	}

	// Try to consume a token
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// Reset clears the rate limit state for a specific user session
// Useful for testing or when a session terminates
func (l *Limiter) Reset(userSessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, userSessionID)
}

// ResetAll clears all rate limit state
// Useful for testing or system maintenance
func (l *Limiter) ResetAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}

// Cleanup removes buckets for sessions that haven't been active recently
// Should be called periodically to prevent memory leaks
// maxAge: buckets older than this will be removed
func (l *Limiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for sessionID, b := range l.buckets {
		b.mu.Lock()
		age := now.Sub(b.lastRefill)
		b.mu.Unlock()

		if age > maxAge {
			delete(l.buckets, sessionID)
		}
	}
}
