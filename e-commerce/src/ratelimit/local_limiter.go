package ratelimit

import (
	"sync"
	"time"
)

type tokenBucket struct {
	capacity   int       // maximum tokens
	tokens     float64   // current remaining tokens (float for precise refill)
	refillRate float64   // tokens added per second = limit / windowSeconds
	lastRefill time.Time // last refill time
	mu         sync.Mutex
}

type LocalTokenBucketLimiter struct {
	buckets map[string]*tokenBucket
	mu      sync.RWMutex

	limit  int
	window int
}

func NewLocalTokenBucketLimiter(limit int, windowSeconds int) *LocalTokenBucketLimiter {
	return &LocalTokenBucketLimiter{
		buckets: make(map[string]*tokenBucket),
		limit:   limit,
		window:  windowSeconds,
	}
}

func (l *LocalTokenBucketLimiter) getBucket(userID string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[userID]
	if !exists {
		bucket = &tokenBucket{
			capacity:   l.limit,
			tokens:     float64(l.limit),
			refillRate: float64(l.limit) / float64(l.window),
			lastRefill: time.Now(),
		}
		l.buckets[userID] = bucket
	}
	return bucket
}
func (l *LocalTokenBucketLimiter) CheckLimit(userID string) (bool, int, error) {
	bucket := l.getBucket(userID)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()

	if elapsed > 0 {
		bucket.tokens += elapsed * bucket.refillRate
		if bucket.tokens > float64(bucket.capacity) {
			bucket.tokens = float64(bucket.capacity)
		}
		bucket.lastRefill = now
	}

	if bucket.tokens >= 1 {
		bucket.tokens -= 1
		return true, int(bucket.tokens), nil
	}

	return false, int(bucket.tokens), nil
}

func (l *LocalTokenBucketLimiter) Reset(userID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, userID)
	return nil
}

func (l *LocalTokenBucketLimiter) Allow(key string) (bool, int, time.Duration, error) {
	allowed, remaining, err := l.CheckLimit(key)
	if err != nil {
		return false, remaining, 0, err
	}

	if !allowed {
		bucket := l.getBucket(key)

		bucket.mu.Lock()
		defer bucket.mu.Unlock()

		missing := 1 - bucket.tokens
		retrySeconds := missing / bucket.refillRate
		retry := time.Duration(retrySeconds * float64(time.Second))

		return false, remaining, retry, nil
	}

	return true, remaining, 0, nil
}
