package ratelimit

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type RateLimitMiddleware struct {
	limiter Limiter
}

func NewRateLimitMiddleware(limiter Limiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{limiter: limiter}
}

func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			userID = r.RemoteAddr
		}

		// Measure rate limiter overhead
		startTime := time.Now()
		allowed, remaining, retryAfter, err := m.limiter.Allow(userID)
		rateLimiterDuration := time.Since(startTime)

		// Log rate limiter overhead
		log.Printf("[RateLimit] User: %s, Overhead: %v, Allowed: %v", userID, rateLimiterDuration, allowed)

		if err != nil {
			http.Error(w, "rate limit error", http.StatusInternalServerError)
			return
		}

		// Add rate limiter overhead to response headers for monitoring
		w.Header().Set("X-RateLimit-Overhead-Ms", fmt.Sprintf("%.6f", rateLimiterDuration.Seconds()*1000))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(remaining))
		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))

		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Rate limit exceeded",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
