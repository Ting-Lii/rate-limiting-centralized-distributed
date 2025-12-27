# Rate Limiting Implementation Guide

## Quick Answer: DynamoDB vs MySQL

**Use DynamoDB** if you need persistence/analytics for rate limiting. However, **Redis is the primary store** for real-time rate limiting.

## Architecture Decision Tree

```
Do you need real-time rate limiting?
├─ YES → Use Redis (primary)
│   └─ Do you need persistence/analytics?
│       ├─ YES → Use DynamoDB (secondary, async writes)
│       └─ NO → Redis only
│
└─ NO (just analytics) → DynamoDB or MySQL
    └─ For high throughput → DynamoDB
    └─ For complex queries → MySQL
```

## Implementation Steps

### Step 1: Add Redis Dependency

```bash
cd e-commerce/src
go get github.com/redis/go-redis/v9
```

### Step 2: Create Rate Limiter Interfaces

```go
// ratelimit/limiter.go
package ratelimit

type Limiter interface {
    CheckLimit(userID string, limit int, windowSeconds int) (allowed bool, remaining int, err error)
    Reset(userID string) error
}
```

### Step 3: Implement Redis Limiter (Centralized)

```go
// ratelimit/redis_limiter.go
package ratelimit

import (
    "context"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
    client *redis.Client
}

func NewRedisLimiter(addr string) (*RedisLimiter, error) {
    rdb := redis.NewClient(&redis.Options{
        Addr: addr,
    })
    
    ctx := context.Background()
    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, err
    }
    
    return &RedisLimiter{client: rdb}, nil
}

func (r *RedisLimiter) CheckLimit(userID string, limit int, windowSeconds int) (bool, int, error) {
    key := fmt.Sprintf("ratelimit:%s:%d", userID, windowSeconds)
    ctx := context.Background()
    
    // Atomic increment
    count, err := r.client.Incr(ctx, key).Result()
    if err != nil {
        return false, 0, err
    }
    
    // Set expiration on first request
    if count == 1 {
        r.client.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)
    }
    
    remaining := limit - int(count)
    if remaining < 0 {
        remaining = 0
    }
    
    allowed := count <= int64(limit)
    return allowed, remaining, nil
}
```

### Step 4: Implement Local Limiter (Distributed)

```go
// ratelimit/local_limiter.go
package ratelimit

import (
    "sync"
    "time"
)

type LocalLimiter struct {
    mu      sync.RWMutex
    buckets map[string]*TokenBucket
}

type TokenBucket struct {
    tokens     int
    maxTokens  int
    refillRate int // tokens per second
    lastRefill time.Time
    mu         sync.Mutex
}

func NewLocalLimiter() *LocalLimiter {
    return &LocalLimiter{
        buckets: make(map[string]*TokenBucket),
    }
}

func (l *LocalLimiter) CheckLimit(userID string, limit int, windowSeconds int) (bool, int, error) {
    l.mu.Lock()
    bucket, exists := l.buckets[userID]
    if !exists {
        bucket = &TokenBucket{
            tokens:     limit,
            maxTokens:  limit,
            refillRate: limit / windowSeconds,
            lastRefill: time.Now(),
        }
        l.buckets[userID] = bucket
    }
    l.mu.Unlock()
    
    bucket.mu.Lock()
    defer bucket.mu.Unlock()
    
    // Refill tokens
    now := time.Now()
    elapsed := now.Sub(bucket.lastRefill).Seconds()
    tokensToAdd := int(elapsed * float64(bucket.refillRate))
    if tokensToAdd > 0 {
        bucket.tokens = min(bucket.tokens+tokensToAdd, bucket.maxTokens)
        bucket.lastRefill = now
    }
    
    // Check limit
    if bucket.tokens > 0 {
        bucket.tokens--
        return true, bucket.tokens, nil
    }
    
    return false, 0, nil
}
```

### Step 5: Optional - DynamoDB Persistence

```go
// ratelimit/dynamodb_persistence.go
package ratelimit

import (
    "context"
    "fmt"
    "time"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBPersistence struct {
    client    *dynamodb.Client
    tableName string
}

func (d *DynamoDBPersistence) LogRateLimitEvent(userID string, allowed bool, limit int, windowSeconds int) error {
    // Async write for analytics (don't block rate limit check)
    go func() {
        ctx := context.Background()
        windowStart := time.Now().Unix() / int64(windowSeconds) * int64(windowSeconds)
        
        key := fmt.Sprintf("%s:%d", userID, windowStart)
        
        _, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
            TableName: aws.String(d.tableName),
            Key: map[string]types.AttributeValue{
                "user_id":         &types.AttributeValueMemberS{Value: userID},
                "window_timestamp": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
            },
            UpdateExpression: aws.String("ADD request_count :inc SET ttl = :ttl"),
            ExpressionAttributeValues: map[string]types.AttributeValue{
                ":inc": &types.AttributeValueMemberN{Value: "1"},
                ":ttl": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())},
            },
        })
        
        if err != nil {
            // Log error but don't fail rate limit check
            fmt.Printf("Failed to persist rate limit event: %v\n", err)
        }
    }()
    
    return nil
}
```

### Step 6: HTTP Middleware

```go
// ratelimit/middleware.go
package ratelimit

import (
    "encoding/json"
    "net/http"
    "strconv"
)

type RateLimitMiddleware struct {
    limiter Limiter
    limit   int
    window  int
}

func NewRateLimitMiddleware(limiter Limiter, limit int, windowSeconds int) *RateLimitMiddleware {
    return &RateLimitMiddleware{
        limiter: limiter,
        limit:   limit,
        window:  windowSeconds,
    }
}

func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract user ID from request (header, JWT, etc.)
        userID := r.Header.Get("X-User-ID")
        if userID == "" {
            userID = r.RemoteAddr // Fallback to IP
        }
        
        allowed, remaining, err := m.limiter.CheckLimit(userID, m.limit, m.window)
        if err != nil {
            http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
            return
        }
        
        // Set rate limit headers
        w.Header().Set("X-RateLimit-Limit", strconv.Itoa(m.limit))
        w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
        w.Header().Set("X-RateLimit-Reset", strconv.Itoa(m.window))
        
        if !allowed {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusTooManyRequests)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Rate limit exceeded",
            })
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Step 7: Integrate with Main Application

```go
// main.go (additions)
import (
    "hw8-ecommerce-api/ratelimit"
)

func main() {
    // ... existing code ...
    
    // Initialize rate limiter based on environment
    var limiter ratelimit.Limiter
    rateLimitType := os.Getenv("RATE_LIMIT_TYPE") // "redis" or "local"
    
    switch rateLimitType {
    case "redis":
        redisAddr := os.Getenv("REDIS_ADDR")
        if redisAddr == "" {
            redisAddr = "localhost:6379"
        }
        var err error
        limiter, err = ratelimit.NewRedisLimiter(redisAddr)
        if err != nil {
            log.Fatal("Failed to initialize Redis limiter:", err)
        }
        log.Println("Using Redis for rate limiting")
        
    case "local", "":
        limiter = ratelimit.NewLocalLimiter()
        log.Println("Using local in-memory rate limiting")
        
    default:
        log.Fatalf("Unknown RATE_LIMIT_TYPE: %s", rateLimitType)
    }
    
    // Create rate limit middleware
    rateLimitMiddleware := ratelimit.NewRateLimitMiddleware(limiter, 100, 60) // 100 req/min
    
    router := mux.NewRouter()
    
    // Apply rate limiting to all routes
    router.Use(rateLimitMiddleware.Handler)
    
    // ... existing routes ...
}
```

## Testing Your Three Experiments

### Experiment 1: Performance Comparison

```bash
# Test Redis (centralized)
RATE_LIMIT_TYPE=redis REDIS_ADDR=localhost:6379 go run main.go

# Test Local (distributed)
RATE_LIMIT_TYPE=local go run main.go

# Run performance test
go run tester/test_performance.go localhost:8080
```

### Experiment 2: Consistency Under Load

Run 5 servers with different rate limit types and measure accuracy.

### Experiment 3: Failure Scenarios

```bash
# Test Redis failure
# Stop Redis, measure fallback behavior

# Test uneven traffic distribution
# Send 80% traffic to one server, 20% to others
```

## Summary

- **Primary**: Redis for centralized rate limiting
- **Secondary**: DynamoDB for persistence/analytics (if needed)
- **Alternative**: Local in-memory for distributed approach
- **Not Recommended**: MySQL for real-time rate limiting (too slow)

