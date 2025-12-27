#!/bin/bash

# Redis Failure and Recovery Test
# Tests application behavior when Redis is unavailable

set -e

export AWS_PROFILE=myisb_IsbUsersPS-003816847575 # Replace with your AWS profile name if needed

TASK_IP="52.12.97.153" # Replace with your ECS task IP if needed
REDIS_CLUSTER_ID="cs6650-hw8-ecommerce-redis"
REGION="us-west-2"
OUTPUT_FILE="redis_failure_results.txt"

echo "=== Redis Failure and Recovery Test ===" | tee $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE
echo "Task IP: $TASK_IP" | tee -a $OUTPUT_FILE
echo "Redis Cluster: $REDIS_CLUSTER_ID" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

# Function to send requests and record results
send_requests() {
    local phase=$1
    local user_id=$2
    local num_requests=$3
    
    echo "[$phase] Sending $num_requests requests..." | tee -a $OUTPUT_FILE
    
    local success=0
    local failed=0
    local total_time=0
    
    for i in $(seq 1 $num_requests); do
        START=$(python3 -c "import time; print(int(time.time() * 1000))")
        
        HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null --max-time 5 \
            -X POST "http://$TASK_IP:8080/shopping-carts" \
            -H "Content-Type: application/json" \
            -H "X-User-ID: $user_id" \
            -d '{"customer_id": 999}' 2>/dev/null || echo "000")
        
        END=$(python3 -c "import time; print(int(time.time() * 1000))")
        LATENCY=$((END - START))
        total_time=$((total_time + LATENCY))
        
        if [ "$HTTP_CODE" == "201" ]; then
            ((success++))
            echo "  Request $i: HTTP $HTTP_CODE (${LATENCY}ms) ✓" | tee -a $OUTPUT_FILE
        else
            ((failed++))
            echo "  Request $i: HTTP $HTTP_CODE (${LATENCY}ms) ✗" | tee -a $OUTPUT_FILE
        fi
        
        sleep 0.5
    done
    
    local avg_latency=$((total_time / num_requests))
    
    echo "" | tee -a $OUTPUT_FILE
    echo "[$phase] Results:" | tee -a $OUTPUT_FILE
    echo "  Success: $success/$num_requests" | tee -a $OUTPUT_FILE
    echo "  Failed: $failed/$num_requests" | tee -a $OUTPUT_FILE
    echo "  Average Latency: ${avg_latency}ms" | tee -a $OUTPUT_FILE
    echo "" | tee -a $OUTPUT_FILE
    
    # Store result in global variable instead of return (bash return is limited to 0-255)
    LAST_SUCCESS_COUNT=$success
}

# Function to check Redis status
check_redis_status() {
    aws elasticache describe-cache-clusters \
        --cache-cluster-id $REDIS_CLUSTER_ID \
        --region $REGION \
        --query 'CacheClusters[0].CacheClusterStatus' \
        --output text 2>/dev/null || echo "unknown"
}

# ============================================
# Phase 1: Normal Operation (Before Failure)
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Phase 1: Normal Operation" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

REDIS_STATUS=$(check_redis_status)
echo "Redis Status: $REDIS_STATUS" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

send_requests "BEFORE_FAILURE" "test-before-failure" 10
SUCCESS_BEFORE=$LAST_SUCCESS_COUNT

# ============================================
# Phase 2: Initiate Redis Reboot
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Phase 2: Rebooting Redis" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

echo "Initiating Redis reboot..." | tee -a $OUTPUT_FILE
aws elasticache reboot-cache-cluster \
    --cache-cluster-id $REDIS_CLUSTER_ID \
    --cache-node-ids-to-reboot "0001" \
    --region $REGION > /dev/null 2>&1

echo "Reboot command sent!" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

# Wait 5 seconds for reboot to start
echo "Waiting 5 seconds for reboot to start..." | tee -a $OUTPUT_FILE
sleep 5

REDIS_STATUS=$(check_redis_status)
echo "Redis Status: $REDIS_STATUS" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE
# ============================================
# Phase 3: Test During Redis Downtime
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Phase 3: Testing During Downtime" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

REDIS_STATUS=$(check_redis_status)
echo "Redis Status: $REDIS_STATUS" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

DOWNTIME_START=$(date +%s)
send_requests "DURING_FAILURE" "test-during-failure" 20
SUCCESS_DURING=$LAST_SUCCESS_COUNT
DOWNTIME_END=$(date +%s)
DOWNTIME_DURATION=$((DOWNTIME_END - DOWNTIME_START))

# ============================================
# Phase 4: Monitor Redis Recovery
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Phase 4: Monitoring Redis Recovery" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

echo "Checking Redis status every 10 seconds..." | tee -a $OUTPUT_FILE
RECOVERY_START=$(date +%s)
RECOVERED=false

for attempt in {1..12}; do
    REDIS_STATUS=$(check_redis_status)
    echo "  Attempt $attempt: Redis status = $REDIS_STATUS" | tee -a $OUTPUT_FILE
    
    if [ "$REDIS_STATUS" == "available" ]; then
        RECOVERY_END=$(date +%s)
        RECOVERY_TIME=$((RECOVERY_END - RECOVERY_START))
        echo "" | tee -a $OUTPUT_FILE
        echo "✓ Redis recovered after ${RECOVERY_TIME}s!" | tee -a $OUTPUT_FILE
        RECOVERED=true
        break
    fi
    
    sleep 10
done

if [ "$RECOVERED" = false ]; then
    echo "" | tee -a $OUTPUT_FILE
    echo "✗ Redis not recovered after 2 minutes" | tee -a $OUTPUT_FILE
    RECOVERY_TIME="120+"
fi

echo "" | tee -a $OUTPUT_FILE

# Wait additional 10 seconds to ensure full recovery
echo "Waiting 10 more seconds to ensure stable recovery..." | tee -a $OUTPUT_FILE
sleep 10
echo "" | tee -a $OUTPUT_FILE

# ============================================
# Phase 5: Test After Recovery
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Phase 5: Testing After Recovery" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

REDIS_STATUS=$(check_redis_status)
echo "Redis Status: $REDIS_STATUS" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

send_requests "AFTER_RECOVERY" "test-after-recovery" 10
SUCCESS_AFTER=$LAST_SUCCESS_COUNT

# ============================================
# Final Summary
# ============================================
echo "========================================" | tee -a $OUTPUT_FILE
echo "Test Summary" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE
echo "Phase 1 - Before Failure:   $SUCCESS_BEFORE/10 requests succeeded" | tee -a $OUTPUT_FILE
echo "Phase 3 - During Failure:   $SUCCESS_DURING/20 requests succeeded" | tee -a $OUTPUT_FILE
echo "Phase 5 - After Recovery:   $SUCCESS_AFTER/10 requests succeeded" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE
echo "Downtime Duration:          ${DOWNTIME_DURATION}s (Phase 3 test duration)" | tee -a $OUTPUT_FILE
echo "Recovery Time:              ${RECOVERY_TIME}s (until Redis status = available)" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE

# Calculate success rate during failure
if [ $SUCCESS_DURING -eq 20 ]; then
    echo "Behavior: FAIL-OPEN (100% success during Redis downtime)" | tee -a $OUTPUT_FILE
elif [ $SUCCESS_DURING -eq 0 ]; then
    echo "Behavior: FAIL-CLOSED (0% success during Redis downtime)" | tee -a $OUTPUT_FILE
else
    FAILURE_RATE=$(((20 - SUCCESS_DURING) * 100 / 20))
    echo "Behavior: PARTIAL FAILURE (${FAILURE_RATE}% failure rate during downtime)" | tee -a $OUTPUT_FILE
fi

echo "" | tee -a $OUTPUT_FILE
echo "Full results saved to: $OUTPUT_FILE" | tee -a $OUTPUT_FILE
echo "" | tee -a $OUTPUT_FILE
echo "========================================" | tee -a $OUTPUT_FILE

# Check application logs for errors
echo "" | tee -a $OUTPUT_FILE
echo "Checking application logs for Redis errors..." | tee -a $OUTPUT_FILE
aws logs tail /ecs/cs6650-hw8-ecommerce \
    --since 5m \
    --region $REGION \
    --format short 2>/dev/null | \
    grep -iE "redis.*error|connection.*failed|dial.*error|timeout|during-failure" | \
    tail -10 | tee -a $OUTPUT_FILE

echo "" | tee -a $OUTPUT_FILE
echo "Test complete!"
