package ratelimit

import "time"

// The interface stays minimal and universal;
// algorithm-specific features belong inside each limiter's implementation.
type Limiter interface {
	Allow(key string) (allowed bool, remaining int, retryAfter time.Duration, err error)
}
