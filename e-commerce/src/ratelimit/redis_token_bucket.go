package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTokenBucketLimiter implements token bucket algorithm using Redis
// Uses Lua script for atomic token refill and consumption
type RedisTokenBucketLimiter struct {
	client     *redis.Client
	capacity   int           // maximum tokens
	refillRate float64       // tokens per second
	window     time.Duration // used to calculate refill rate
}

// NewRedisTokenBucketLimiter creates a new Redis-based token bucket limiter
// capacity: maximum number of tokens (burst size)
// window: time window for rate calculation (e.g., 60s means capacity tokens per 60 seconds)
func NewRedisTokenBucketLimiter(addr string, capacity int, window time.Duration) (*RedisTokenBucketLimiter, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	refillRate := float64(capacity) / window.Seconds()

	return &RedisTokenBucketLimiter{
		client:     rdb,
		capacity:   capacity,
		refillRate: refillRate,
		window:     window,
	}, nil
}

// Lua script for atomic token bucket operations
// KEYS[1] = token key (stores current tokens)
// KEYS[2] = timestamp key (stores last refill time)
// ARGV[1] = capacity (max tokens)
// ARGV[2] = refill_rate (tokens per second)
// ARGV[3] = current timestamp (milliseconds)
// ARGV[4] = requested tokens (usually 1)
//
// Returns: [allowed (0/1), remaining_tokens, retry_after_ms]
var tokenBucketScript = `
local token_key = KEYS[1]
local ts_key = KEYS[2]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

-- Get current state
local tokens = tonumber(redis.call('GET', token_key))
local last_refill = tonumber(redis.call('GET', ts_key))

-- Initialize if not exists
if not tokens then
    tokens = capacity
end
if not last_refill then
    last_refill = now
end

-- Calculate refill
local elapsed_seconds = (now - last_refill) / 1000.0
local refill = elapsed_seconds * refill_rate
tokens = math.min(tokens + refill, capacity)

-- Try to consume tokens
local allowed = 0
if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
end

-- Save state with expiration (2x window to prevent premature deletion)
local ttl = math.ceil(capacity / refill_rate * 2)
redis.call('SET', token_key, tokens, 'EX', ttl)
redis.call('SET', ts_key, now, 'EX', ttl)

-- Calculate retry_after (time to get 1 token back)
local retry_after_ms = 0
if allowed == 0 then
    retry_after_ms = math.ceil((requested - tokens) / refill_rate * 1000)
end

return {allowed, math.floor(tokens), retry_after_ms}
`

// Allow checks if a request is allowed under token bucket rate limiting
func (r *RedisTokenBucketLimiter) Allow(userID string) (bool, int, time.Duration, error) {
	ctx := context.Background()
	now := time.Now().UnixMilli()

	tokenKey := fmt.Sprintf("ratelimit:tb:tokens:%s", userID)
	tsKey := fmt.Sprintf("ratelimit:tb:ts:%s", userID)

	// Execute Lua script atomically
	result, err := r.client.Eval(ctx, tokenBucketScript, []string{tokenKey, tsKey},
		r.capacity,   // ARGV[1]
		r.refillRate, // ARGV[2]
		now,          // ARGV[3]
		1,            // ARGV[4] - request 1 token
	).Result()

	if err != nil {
		return false, 0, 0, fmt.Errorf("redis token bucket script error: %w", err)
	}

	// Parse result
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 3 {
		return false, 0, 0, fmt.Errorf("unexpected script result format")
	}

	allowed := resultSlice[0].(int64) == 1
	remaining := int(resultSlice[1].(int64))
	retryAfterMs := resultSlice[2].(int64)
	retryAfter := time.Duration(retryAfterMs) * time.Millisecond

	return allowed, remaining, retryAfter, nil
}

// Reset clears the token bucket for a user (refills to full capacity)
func (r *RedisTokenBucketLimiter) Reset(userID string) error {
	ctx := context.Background()
	tokenKey := fmt.Sprintf("ratelimit:tb:tokens:%s", userID)
	tsKey := fmt.Sprintf("ratelimit:tb:ts:%s", userID)

	pipe := r.client.Pipeline()
	pipe.Del(ctx, tokenKey)
	pipe.Del(ctx, tsKey)
	_, err := pipe.Exec(ctx)

	return err
}
