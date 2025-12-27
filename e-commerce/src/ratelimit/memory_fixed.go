package ratelimit

import (
	"sync"
	"time"
)

type MemoryFixedWindowLimiter struct {
	mu     sync.Mutex
	data   map[string]*entry
	window time.Duration
	limit  int
}

type entry struct {
	count       int
	windowStart int64
}

func NewMemoryFixedWindowLimiter(limit int, window time.Duration) *MemoryFixedWindowLimiter {
	return &MemoryFixedWindowLimiter{
		data:   make(map[string]*entry),
		window: window,
		limit:  limit,
	}
}

func (m *MemoryFixedWindowLimiter) Allow(key string) (bool, int, time.Duration, error) {
	now := time.Now()
	windowStart := now.Unix() / int64(m.window.Seconds())

	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.data[key]
	if !ok || e.windowStart != windowStart {
		// new window
		m.data[key] = &entry{
			count:       1,
			windowStart: windowStart,
		}
		return true, m.limit - 1, m.window, nil
	}

	// same window
	e.count++
	remaining := m.limit - e.count
	if remaining < 0 {
		remaining = 0
	}

	allowed := e.count <= m.limit
	nextWindowStart := (e.windowStart + 1) * int64(m.window.Seconds())
	secondsUntilReset := nextWindowStart - now.Unix()
	retryAfter := time.Duration(secondsUntilReset) * time.Second
	return allowed, remaining, retryAfter, nil
}
