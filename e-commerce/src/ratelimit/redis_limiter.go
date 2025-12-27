package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRedisLimiter(addr string, limit int, window time.Duration) (*RedisLimiter, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisLimiter{
		client: rdb,
		limit:  limit,
		window: window,
	}, nil
}

func (r *RedisLimiter) Allow(userID string) (bool, int, time.Duration, error) {
	now := time.Now()
	windowSeconds := int64(r.window.Seconds())

	// Current window bucket
	windowStart := now.Unix() / windowSeconds

	key := fmt.Sprintf("ratelimit:%s:%d", userID, windowStart)
	ctx := context.Background()

	// Pipeline: INCR + EXPIRE
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("redis pipeline error: %w", err)
	}

	count := incr.Val()

	// remaining allowed requests
	remaining := r.limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	allowed := count <= int64(r.limit)

	// compute retryAfter only when rejected
	if allowed {
		return true, remaining, 0, nil
	}

	// next window: (windowStart+1)*windowSeconds
	nextWindowUnix := (windowStart + 1) * windowSeconds
	retrySeconds := nextWindowUnix - now.Unix()
	retryAfter := time.Duration(retrySeconds) * time.Second

	return false, remaining, retryAfter, nil
}

// // Reset clears the rate limit for a user
// func (r *RedisLimiter) Reset(userID string) error {
// 	ctx := context.Background()
// 	// Use pattern matching to find all rate limit keys for this user
// 	pattern := fmt.Sprintf("ratelimit:%s:*", userID)

// 	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
// 	for iter.Next(ctx) {
// 		if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
// 			return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
// 		}
// 	}

// 	if err := iter.Err(); err != nil {
// 		return fmt.Errorf("scan error: %w", err)
// 	}

// 	return nil
// }
