package ratelimit

import (
	"testing"
)

// TestRedisLimiter requires a running Redis instance
// Run: redis-server (or use Docker: docker run -d -p 6379:6379 redis:latest)
func TestRedisLimiter_CheckLimit(t *testing.T) {
	// Skip if Redis is not available
	limiter, err := NewRedisLimiter("localhost:6379")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	userID := "test-user-1"
	limit := 5
	windowSeconds := 60

	// Test: Should allow requests within limit
	for i := 0; i < limit; i++ {
		allowed, remaining, err := limiter.CheckLimit(userID, limit, windowSeconds)
		if err != nil {
			t.Fatalf("CheckLimit failed: %v", err)
		}
		if !allowed {
			t.Errorf("Request %d should be allowed, but was denied", i+1)
		}
		expectedRemaining := limit - (i + 1)
		if remaining != expectedRemaining {
			t.Errorf("Expected remaining %d, got %d", expectedRemaining, remaining)
		}
	}

	// Test: Should deny request exceeding limit
	allowed, remaining, err := limiter.CheckLimit(userID, limit, windowSeconds)
	if err != nil {
		t.Fatalf("CheckLimit failed: %v", err)
	}
	if allowed {
		t.Error("Request exceeding limit should be denied, but was allowed")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining 0, got %d", remaining)
	}
}

func TestRedisLimiter_Reset(t *testing.T) {
	limiter, err := NewRedisLimiter("localhost:6379")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	userID := "test-user-reset"
	limit := 3
	windowSeconds := 60

	// Exhaust the limit
	for i := 0; i < limit; i++ {
		limiter.CheckLimit(userID, limit, windowSeconds)
	}

	// Verify limit is exhausted
	allowed, _, _ := limiter.CheckLimit(userID, limit, windowSeconds)
	if allowed {
		t.Error("Limit should be exhausted")
	}

	// Reset
	err = limiter.Reset(userID)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Verify limit is reset
	allowed, remaining, err := limiter.CheckLimit(userID, limit, windowSeconds)
	if err != nil {
		t.Fatalf("CheckLimit failed: %v", err)
	}
	if !allowed {
		t.Error("Request should be allowed after reset")
	}
	if remaining != limit-1 {
		t.Errorf("Expected remaining %d, got %d", limit-1, remaining)
	}
}

func TestRedisLimiter_ConcurrentRequests(t *testing.T) {
	limiter, err := NewRedisLimiter("localhost:6379")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	userID := "test-user-concurrent"
	limit := 10
	windowSeconds := 60

	// Reset first
	limiter.Reset(userID)

	// Simulate concurrent requests
	results := make(chan bool, limit*2)
	for i := 0; i < limit*2; i++ {
		go func() {
			allowed, _, _ := limiter.CheckLimit(userID, limit, windowSeconds)
			results <- allowed
		}()
	}

	// Collect results
	allowedCount := 0
	deniedCount := 0
	for i := 0; i < limit*2; i++ {
		if <-results {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	// Should have exactly 'limit' allowed requests
	if allowedCount != limit {
		t.Errorf("Expected %d allowed requests, got %d", limit, allowedCount)
	}
	if deniedCount != limit {
		t.Errorf("Expected %d denied requests, got %d", limit, deniedCount)
	}
}

func TestRedisLimiter_DifferentUsers(t *testing.T) {
	limiter, err := NewRedisLimiter("localhost:6379")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	limit := 3
	windowSeconds := 60

	// Test different users should have independent limits
	user1 := "user-1"
	user2 := "user-2"

	// Exhaust limit for user1
	for i := 0; i < limit; i++ {
		limiter.CheckLimit(user1, limit, windowSeconds)
	}

	// User1 should be denied
	allowed, _, _ := limiter.CheckLimit(user1, limit, windowSeconds)
	if allowed {
		t.Error("User1 should be denied")
	}

	// User2 should still be allowed
	allowed, remaining, err := limiter.CheckLimit(user2, limit, windowSeconds)
	if err != nil {
		t.Fatalf("CheckLimit failed: %v", err)
	}
	if !allowed {
		t.Error("User2 should be allowed")
	}
	if remaining != limit-1 {
		t.Errorf("Expected remaining %d, got %d", limit-1, remaining)
	}
}
