# Distributed Rate Limiting System

## Elevatror Pitch
https://northeastern.zoom.us/rec/share/3zj9ZxH-dIL6HhOnYmeDHMSU3y5n6TIL7CR_Z7SCceOr3qJ29la1M2HN5m9zNGeO.W2MATKlYTNIeXxqX 
Passcode: dNp@VH3n

## Overview

This project implements a distributed rate limiting system for the e-commerce API, comparing centralized (Redis-based) and distributed (local in-memory) approaches.

### Core Challenge

The core challenge is ensuring a user doesn't exceed N requests per time window when their requests are distributed across multiple backend servers. In a distributed system, requests from the same user may hit different servers, making it difficult to accurately track and enforce rate limits.

### Core Idea

Compare centralized (Redis-based shared counter) versus distributed (local token bucket per server) approaches, measuring the trade-offs between:
- **Accuracy**: Exact limit enforcement vs approximate limits
- **Latency**: Network overhead vs in-memory speed
- **Fault Tolerance**: Single point of failure vs independent servers

### Three Main Experiments

1. **Performance Comparison**
   - Measure latency overhead of rate limit checking
   - Centralized Redis: 2-5ms overhead, exact limit enforcement
   - Distributed local counters: 0.01ms overhead, approximate limit
   - Test under increasing load: 100 to 10,000 requests/second

2. **Consistency Under Load**
   - Test accuracy with 5 servers and different traffic distributions
   - Centralized: Enforces exactly 100 requests/minute across all servers
   - Distributed: Allows approximately 100-150 requests/minute (no coordination)
   - Measure variance under uniform vs skewed load distribution

3. **Failure Scenarios**
   - Test resilience and recovery behavior
   - Centralized: What happens when Redis fails? Measure recovery time and user impact
   - Distributed: What happens when traffic unevenly distributes across servers?
   - Compare fail-open vs fail-closed strategies

### Optional: Hybrid Approach

Implement hybrid approach using local buckets with periodic Redis synchronization, comparing it to pure centralized and pure distributed strategies. This combines low latency of local counters with consistency of centralized storage.

## Architecture

### Understanding "Local Distributed" vs "Centralized"

**"Local"** = Storage location (in-memory on each server)  
**"Distributed"** = Multiple servers running the API  
**"Centralized"** = Shared storage (Redis) used by all servers

**Important:** The API can be deployed anywhere (AWS ECS, local machine, Kubernetes, etc.). The terms refer to where the rate limit counter is stored, not where the API runs.

### Architecture Diagrams

#### 1. Centralized (Redis-Based), CP system
```
┌─────────────────────────────────────────────────────┐
│ Deployment: AWS ECS (5 servers) OR Local (5 servers)│
├─────────────────────────────────────────────────────┤
│                                                     │
│  Server 1      Server 2      Server 3      Server 4 │
│  ┌──────┐      ┌──────┐      ┌──────┐      ┌──────┐ │
│  │ API  │      │ API  │      │ API  │      │ API  │ │
│  └──┬───┘      └──┬───┘      └──┬───┘      └──┬───┘ │
│     │             │             │             │     │
│     └─────────────┼─────────────┼─────────────┘     │
│                   │             │                   │
│                   └─────────────┘                   │
│                           │                         │
│                    ┌──────▼──────┐                  │
│                    │    Redis    │                  │
│                    │  (Shared)   │                  │
│                    │  Counter: 50│                  │
│                    └─────────────┘                  │
│                                                     │
│  All servers share ONE counter in Redis             │
│  Exact limit enforcement (100 req/min total)        │
│  Network latency to Redis (2-5ms)                   │
└─────────────────────────────────────────────────────┘
```

#### 2. Distributed (Local In-Memory), AP system
```
┌─────────────────────────────────────────────────────┐
│ Deployment: AWS ECS (5 servers) OR Local (5 servers)│
├─────────────────────────────────────────────────────┤
│                                                     │
│  Server 1      Server 2      Server 3      Server 4 │
│  ┌──────┐      ┌──────┐      ┌──────┐      ┌──────┐ │
│  │ API  │      │ API  │      │ API  │      │ API  │ │
│  │      │      │      │      │      │      │      │ │
│  │ Mem: │      │ Mem: │      │ Mem: │      │ Mem: │ │
│  │ 20   │      │ 30   │      │ 25   │      │ 15   │ │
│  └──────┘      └──────┘      └──────┘      └──────┘ │
│                                                     │
│  Each server has its OWN in-memory counter          │
│  Ultra-fast (0.01ms, no network)                    │
│  Approximate limits (100-150 req/min total)         │
│  No coordination between servers                    │
└─────────────────────────────────────────────────────┘
```

### Key Differences

| Aspect | Centralized (Redis) | Distributed (Local) |
|--------|---------------------|---------------------|
| **Storage Location** | Shared Redis instance | In-memory on each server |
| **API Deployment** | Can be AWS, local, anywhere | Can be AWS, local, anywhere |
| **Counter Location** | External (Redis) | Internal (server memory) |
| **Accuracy** | Exact (100 req/min) | Approximate (100-150 req/min) |
| **Latency** | 2-5ms (network call) | 0.01ms (no network) |
| **Failure Impact** | All servers affected if Redis fails | Each server independent |

## Testing Strategy: Local First, Then AWS

### Phase 1: Local Testing (except dynamodb here which you can change to local dynamodb to achive fully local)

#### Step 1: Test Rate Limiter with Local Redis

```bash
# Start Redis
docker run -d -p 6379:6379 --name redis-test redis:latest

#  Create DynamoDB table (if not exists)
cd e-commerce/terraform

# creates  DynamoDB table in AWS.
terraform apply -target=module.dynamodb

# Start API with rate limiting
cd e-commerce/src
export DATABASE_TYPE="dynamodb"
export DYNAMODB_TABLE_NAME="cs6650-hw8-ecommerce-shopping-carts"
export AWS_REGION="us-west-2"
export RATE_LIMIT_TYPE="redis"
export REDIS_ADDR="localhost:6379"
go run main.go
```

#### Step 2: Test Add Cart Endpoint with Rate Limiting

```bash
# Test: Create cart (should succeed)
curl -X POST http://localhost:8080/shopping-carts \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user-1" \
  -d '{"customer_id": 123}'

# Test: Add items for existing cart (should succeed)
curl -X POST http://localhost:8080/shopping-carts/123456/items \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user-1" \
  -d '{"product_id": 1, "quantity": 2}'

# Test: Exceed rate limit (send 101 requests, limit is 100/min)
for i in {1..101}; do
  curl -s -X POST http://localhost:8080/shopping-carts \
    -H "Content-Type: application/json" \
    -H "X-User-ID: test-user-1" \
    -d '{"customer_id": 123}' | grep -o "shopping_cart_id\|error"
  if [ $((i % 10)) -eq 0 ]; then
    echo "Sent $i requests..."
  fi
done
```

**Expected:**
- First 100 requests: `201 Created` (cart created)/no error
- 101st request: `429 Too Many Requests` (rate limit exceeded)/error

#### Step 3: Verify Rate Limiting Works

```bash
# Check Redis keys (make API requests first to create keys)
docker exec redis-test redis-cli KEYS "ratelimit:*"

# Get a specific key value
docker exec redis-test redis-cli GET "ratelimit:test-user-1:60:28894400"

# Check TTL (time to live) of a key
docker exec redis-test redis-cli TTL "ratelimit:test-user-1:60:28894400"

# Monitor Redis operations in real-time (optional, for debugging)
docker exec redis-test redis-cli MONITOR
```

### Phase 2: AWS Deployment

#### Step 1: Deploy to AWS (Chill, it really takes a while!)

```bash
cd e-commerce/terraform
terraform init
terraform plan
terraform apply
```

#### Step 2: Test with AWS API

```bash
# Get the public IP of the running ECS task
TASK_ARN=$(aws ecs list-tasks --cluster cs6650-hw8-ecommerce-cluster --service-name cs6650-hw8-ecommerce --region us-west-2 --query 'taskArns[0]' --output text)
ENI_ID=$(aws ecs describe-tasks --cluster cs6650-hw8-ecommerce-cluster --tasks $TASK_ARN --region us-west-2 --query 'tasks[0].attachments[0].details[?name==`networkInterfaceId`].value' --output text)
TASK_IP=$(aws ec2 describe-network-interfaces --network-interface-ids $ENI_ID --region us-west-2 --query 'NetworkInterfaces[0].Association.PublicIp' --output text)
echo "API URL: http://$TASK_IP:8080"

# Test add cart with rate limiting
curl -X POST http://$TASK_IP:8080/shopping-carts \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user-1" \
  -d '{"customer_id": 123}'

# Test rate limit
for i in {1..101}; do
  curl -s -X POST http://$TASK_IP:8080/shopping-carts \
    -H "Content-Type: application/json" \
    -H "X-User-ID: test-user-1" \
    -d '{"customer_id": 123}' | grep -o "shopping_cart_id\|error"
  if [ $((i % 10)) -eq 0 ]; then
    echo "Sent $i requests..."
  fi
done
```

**Alternative: Get task IP manually**

```bash
# List running tasks
aws ecs list-tasks --cluster cs6650-hw8-ecommerce-cluster --service-name cs6650-hw8-ecommerce

# Get task details (replace TASK_ARN with actual task ARN from above)
aws ecs describe-tasks --cluster cs6650-hw8-ecommerce-cluster --tasks TASK_ARN

# Extract public IP from task details, then use:
# curl -X POST http://TASK_PUBLIC_IP:8080/shopping-carts ...
```

## Implementation Details

### Redis Rate Limiter

- **Rate Limit:** 100 requests/minute per user
- **User Identification:** Via `X-User-ID` header (falls back to IP address)
- **Storage:** Redis with atomic INCR operations and TTL expiration
- **Key Format:** `ratelimit:{userID}:{windowSeconds}:{windowStart}`

### Infrastructure

- **Local Testing:** Docker Redis container
- **AWS Deployment:** ElastiCache Redis (via Terraform)
- **ECS Integration:** Environment variables passed to ECS tasks

## Experiments

1. **Performance Comparison:** Measure latency overhead of Redis (2-5ms) vs local (0.01ms)
2. **Consistency Under Load:** Test accuracy with 5 servers and different traffic distributions
3. **Failure Scenarios:** Test resilience when Redis fails vs uneven traffic distribution
