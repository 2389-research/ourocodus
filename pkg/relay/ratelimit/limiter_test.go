package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// TestLimiter_BasicAllowDeny tests basic allow/deny behavior
func TestLimiter_BasicAllowDeny(t *testing.T) {
	// Create limiter: 3 tokens max, 1 per second refill
	limiter := NewLimiter(3, 1)

	// First 3 requests should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !limiter.Allow("user-1") {
			t.Errorf("Request %d should be allowed (burst)", i+1)
		}
	}

	// 4th request should be denied (bucket empty)
	if limiter.Allow("user-1") {
		t.Error("4th request should be denied (bucket empty)")
	}
}

// TestLimiter_Refill tests that tokens refill over time
func TestLimiter_Refill(t *testing.T) {
	// Create limiter: 2 tokens max, 2 per second refill
	limiter := NewLimiter(2, 2)

	// Consume both tokens
	limiter.Allow("user-1")
	limiter.Allow("user-1")

	// Should be denied immediately
	if limiter.Allow("user-1") {
		t.Error("Should be denied immediately after consuming tokens")
	}

	// Wait 1 second for refill (should get 2 tokens back)
	time.Sleep(1100 * time.Millisecond)

	// Next 2 requests should be allowed
	if !limiter.Allow("user-1") {
		t.Error("Should be allowed after refill (1st)")
	}
	if !limiter.Allow("user-1") {
		t.Error("Should be allowed after refill (2nd)")
	}

	// 3rd should be denied
	if limiter.Allow("user-1") {
		t.Error("3rd request after refill should be denied")
	}
}

// TestLimiter_IndependentSessions tests that different sessions have independent limits
func TestLimiter_IndependentSessions(t *testing.T) {
	limiter := NewLimiter(2, 1)

	// User 1 consumes both tokens
	limiter.Allow("user-1")
	limiter.Allow("user-1")

	// User 1 should be denied
	if limiter.Allow("user-1") {
		t.Error("User 1 should be denied")
	}

	// User 2 should still be allowed (independent bucket)
	if !limiter.Allow("user-2") {
		t.Error("User 2 should be allowed (1st)")
	}
	if !limiter.Allow("user-2") {
		t.Error("User 2 should be allowed (2nd)")
	}

	// User 2 should now be denied
	if limiter.Allow("user-2") {
		t.Error("User 2 should be denied after consuming tokens")
	}
}

// TestLimiter_Reset tests that Reset clears a session's bucket
func TestLimiter_Reset(t *testing.T) {
	limiter := NewLimiter(2, 1)

	// Consume both tokens
	limiter.Allow("user-1")
	limiter.Allow("user-1")

	// Should be denied
	if limiter.Allow("user-1") {
		t.Error("Should be denied before reset")
	}

	// Reset the bucket
	limiter.Reset("user-1")

	// Should be allowed again (fresh bucket)
	if !limiter.Allow("user-1") {
		t.Error("Should be allowed after reset (1st)")
	}
	if !limiter.Allow("user-1") {
		t.Error("Should be allowed after reset (2nd)")
	}
}

// TestLimiter_ResetAll tests that ResetAll clears all buckets
func TestLimiter_ResetAll(t *testing.T) {
	limiter := NewLimiter(1, 1)

	// Consume tokens for multiple users
	limiter.Allow("user-1")
	limiter.Allow("user-2")
	limiter.Allow("user-3")

	// All should be denied
	if limiter.Allow("user-1") {
		t.Error("User 1 should be denied before reset")
	}
	if limiter.Allow("user-2") {
		t.Error("User 2 should be denied before reset")
	}
	if limiter.Allow("user-3") {
		t.Error("User 3 should be denied before reset")
	}

	// Reset all buckets
	limiter.ResetAll()

	// All should be allowed again
	if !limiter.Allow("user-1") {
		t.Error("User 1 should be allowed after reset")
	}
	if !limiter.Allow("user-2") {
		t.Error("User 2 should be allowed after reset")
	}
	if !limiter.Allow("user-3") {
		t.Error("User 3 should be allowed after reset")
	}
}

// TestLimiter_Cleanup tests that Cleanup removes old buckets
func TestLimiter_Cleanup(t *testing.T) {
	limiter := NewLimiter(2, 1)

	// Create bucket for user-1
	limiter.Allow("user-1")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Create bucket for user-2
	limiter.Allow("user-2")

	// Cleanup buckets older than 50ms (should remove user-1 but not user-2)
	limiter.Cleanup(50 * time.Millisecond)

	// User-1 should have fresh bucket (reset by cleanup)
	if !limiter.Allow("user-1") {
		t.Error("User 1 should have fresh bucket after cleanup (1st)")
	}
	if !limiter.Allow("user-1") {
		t.Error("User 1 should have fresh bucket after cleanup (2nd)")
	}

	// User-2 should still have their partially consumed bucket
	if !limiter.Allow("user-2") {
		t.Error("User 2 should still have tokens (1st)")
	}
	// User-2 consumed 1 during setup, has 1 left, this is the 2nd
	if limiter.Allow("user-2") {
		t.Error("User 2 should be out of tokens (already consumed 2)")
	}
}

// TestLimiter_ConcurrentAccess tests thread-safety
func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewLimiter(100, 10)

	const numGoroutines = 10
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	allowedCount := make([]int, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.Allow("user-concurrent") {
					allowedCount[id]++
				}
			}
		}(i)
	}

	wg.Wait()

	// Count total allowed requests
	total := 0
	for _, count := range allowedCount {
		total += count
	}

	// Should allow at most 100 (maxTokens) in the burst
	if total > 100 {
		t.Errorf("Too many requests allowed: %d (max 100)", total)
	}

	// Should allow at least some requests (basic sanity check)
	if total == 0 {
		t.Error("No requests were allowed")
	}

	t.Logf("Allowed %d out of %d concurrent requests", total, numGoroutines*requestsPerGoroutine)
}

// TestLimiter_ZeroRefillRate tests limiter with no refill (fixed capacity only)
func TestLimiter_ZeroRefillRate(t *testing.T) {
	limiter := NewLimiter(3, 0)

	// Should allow 3 requests
	for i := 0; i < 3; i++ {
		if !limiter.Allow("user-1") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if limiter.Allow("user-1") {
		t.Error("4th request should be denied")
	}

	// Wait and verify no refill happens
	time.Sleep(100 * time.Millisecond)

	// Still should be denied (no refill)
	if limiter.Allow("user-1") {
		t.Error("Should still be denied after waiting (no refill)")
	}
}

// TestLimiter_HighRefillRate tests limiter with fast refill
func TestLimiter_HighRefillRate(t *testing.T) {
	// 5 tokens max, 100 per second refill (very fast)
	limiter := NewLimiter(5, 100)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		limiter.Allow("user-1")
	}

	// Should be denied
	if limiter.Allow("user-1") {
		t.Error("Should be denied immediately after burst")
	}

	// Wait 100ms (should refill 10 tokens, but max is 5)
	time.Sleep(100 * time.Millisecond)

	// Should allow 5 more requests (refilled to max)
	for i := 0; i < 5; i++ {
		if !limiter.Allow("user-1") {
			t.Errorf("Request %d after refill should be allowed", i+1)
		}
	}

	// 6th should be denied
	if limiter.Allow("user-1") {
		t.Error("6th request after refill should be denied")
	}
}
