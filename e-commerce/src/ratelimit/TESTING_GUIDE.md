# Redis Limiter Testing Guide

## Prerequisites

### 1. Install and Start Redis

**Option A: Local Redis Installation**
```bash
# macOS
brew install redis
brew services start redis

# Linux (Ubuntu/Debian)
sudo apt-get install redis-server
sudo systemctl start redis

# Verify Redis is running
redis-cli ping
# Should return: PONG
```

**Option B: Docker (Recommended for Testing)**
```bash
# Start Redis in Docker
docker run -d -p 6379:6379 --name redis-test redis:latest

# Verify it's running
docker ps | grep redis

# Test connection
docker exec -it redis-test redis-cli ping
# Should return: PONG
```

## Testing Steps

### Step 1: Run Unit Tests

```bash
cd e-commerce/src

# Run all rate limiter tests
go test ./ratelimit -v

# Run specific test
go test ./ratelimit -v -run TestRedisLimiter_CheckLimit

# Run with coverage
go test ./ratelimit -v -cover
```

**Expected Output:**
```
=== RUN   TestRedisLimiter_CheckLimit
--- PASS: TestRedisLimiter_CheckLimit (0.05s)
=== RUN   TestRedisLimiter_Reset
--- PASS: TestRedisLimiter_Reset (0.02s)
=== RUN   TestRedisLimiter_ConcurrentRequests
--- PASS: TestRedisLimiter_ConcurrentRequests (0.10s)
=== RUN   TestRedisLimiter_DifferentUsers
--- PASS: TestRedisLimiter_DifferentUsers (0.03s)
PASS
ok      hw8-ecommerce-api/ratelimit    0.200s
```

### Step 2: Manual Testing with Redis CLI

```bash
# Connect to Redis
redis-cli

# Monitor Redis commands in real-time (in another terminal)
redis-cli MONITOR
```

**Test Rate Limiting:**
```bash
# In Redis CLI, check keys
KEYS ratelimit:*

# Check a specific key value
GET ratelimit:test-user-1:60:1234567890

# Check TTL (time to live)
TTL ratelimit:test-user-1:60:1234567890

# Clear all rate limit keys (for testing)
KEYS ratelimit:* | xargs redis-cli DEL
```

### Step 3: Integration Test with HTTP Server

Create a simple test server to verify rate limiting works with HTTP requests:

```bash
# Create test script (see test_integration.go below)
# Then run:
go run test_integration.go
```

In another terminal:
```bash
# Test allowed requests
for i in {1..5}; do
  curl -H "X-User-ID: test-user" http://localhost:8080/test
  echo ""
done

# Test rate limit exceeded
curl -H "X-User-ID: test-user" http://localhost:8080/test
# Should return: 429 Too Many Requests
```

### Step 4: Performance Testing

```bash
# Install Apache Bench (if not installed)
# macOS: brew install httpd
# Linux: sudo apt-get install apache2-utils

# Run performance test
ab -n 1000 -c 10 -H "X-User-ID: perf-test" http://localhost:8080/test

# Or use wrk (better for high concurrency)
# Install: brew install wrk (macOS) or sudo apt-get install wrk (Linux)
wrk -t4 -c100 -d30s -H "X-User-ID: perf-test" http://localhost:8080/test
```

## Test Scenarios

### Scenario 1: Basic Rate Limiting
- **Setup:** Limit = 5 requests/minute
- **Action:** Send 6 requests
- **Expected:** First 5 allowed (200 OK), 6th denied (429)

### Scenario 2: Window Reset
- **Setup:** Limit = 5 requests/60 seconds
- **Action:** 
  1. Send 5 requests (all allowed)
  2. Wait 61 seconds
  3. Send 1 request
- **Expected:** Request after wait should be allowed (window reset)

### Scenario 3: Concurrent Requests
- **Setup:** Limit = 10 requests/minute
- **Action:** Send 20 concurrent requests
- **Expected:** Exactly 10 allowed, 10 denied

### Scenario 4: Different Users
- **Setup:** Limit = 5 requests/minute per user
- **Action:** 
  1. User A sends 5 requests (all allowed)
  2. User B sends 1 request
- **Expected:** User B's request allowed (independent limits)

### Scenario 5: Reset Functionality
- **Setup:** Limit = 5 requests/minute
- **Action:**
  1. Send 5 requests (all allowed)
  2. Call Reset API
  3. Send 1 request
- **Expected:** Request after reset should be allowed

## Troubleshooting

### Issue: "Redis not available" error
```bash
# Check if Redis is running
redis-cli ping

# If not running, start it:
# macOS: brew services start redis
# Linux: sudo systemctl start redis
# Docker: docker start redis-test
```

### Issue: Tests pass but integration fails
- Check Redis connection string matches your setup
- Verify middleware is properly integrated
- Check HTTP headers (X-User-ID) are being read correctly

### Issue: Rate limits not resetting
- Check TTL on Redis keys: `TTL ratelimit:user:60:timestamp`
- Verify window calculation is correct
- Check system clock synchronization

### Issue: Performance issues
- Monitor Redis with: `redis-cli --latency`
- Check network latency between app and Redis
- Consider Redis connection pooling

## Monitoring Redis

```bash
# Real-time monitoring
redis-cli MONITOR

# Check memory usage
redis-cli INFO memory

# Check connected clients
redis-cli CLIENT LIST

# Check slow commands
redis-cli SLOWLOG GET 10
```

## Cleanup

```bash
# Clear all rate limit keys (for testing)
redis-cli --scan --pattern "ratelimit:*" | xargs redis-cli DEL

# Or in Redis CLI:
FLUSHDB  # Clears current database (use with caution!)
```

