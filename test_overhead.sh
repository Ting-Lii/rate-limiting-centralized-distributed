#!/bin/bash

TASK_IP="34.211.15.195" # Replace with your ECS task IP if needed
USER_ID="test-user-$(date +%s)"
NUM_REQUESTS=101

echo "Sending $NUM_REQUESTS requests to measure rate limiter overhead..."
echo "Using User ID: $USER_ID"
echo ""

# File to store overhead values
OVERHEAD_FILE="overhead_results.txt"
> $OVERHEAD_FILE

for i in $(seq 1 $NUM_REQUESTS); do
    # Send request and extract overhead header
    OVERHEAD=$(curl -s -X POST http://$TASK_IP:8080/shopping-carts \
        -H "Content-Type: application/json" \
        -H "X-User-ID: $USER_ID" \
        -d '{"customer_id": 123}' \
        -i 2>&1 | grep -i "X-Ratelimit-Overhead-Ms:" | awk '{print $2}' | tr -d '\r')
    
    if [ ! -z "$OVERHEAD" ]; then
        echo "$OVERHEAD" >> $OVERHEAD_FILE
        echo "Request $i: ${OVERHEAD} ms"
    else
        echo "Request $i: Failed to get overhead"
    fi
    
    # Small delay to avoid overwhelming the server
    sleep 0.1
done

echo ""
echo "========================================="
echo "Rate Limiter Overhead Statistics"
echo "========================================="

# Sort the results
SORTED_FILE="sorted_overhead.txt"
sort -n $OVERHEAD_FILE > $SORTED_FILE

# Calculate statistics
TOTAL=$(wc -l < $OVERHEAD_FILE | tr -d ' ')
AVG=$(awk '{sum+=$1} END {printf "%.6f", sum/NR}' $OVERHEAD_FILE)
MIN=$(head -1 $SORTED_FILE)
P50=$(sed -n "$(($TOTAL * 50 / 100))p" $SORTED_FILE)
P90=$(sed -n "$(($TOTAL * 90 / 100))p" $SORTED_FILE)
P95=$(sed -n "$(($TOTAL * 95 / 100))p" $SORTED_FILE)
P99=$(sed -n "$(($TOTAL * 99 / 100))p" $SORTED_FILE)
MAX=$(tail -1 $SORTED_FILE)

echo "Total Requests: $TOTAL"
echo "Average:        $AVG ms"
echo "Min:            $MIN ms"
echo "P50 (Median):   $P50 ms"
echo "P90:            $P90 ms"
echo "P95:            $P95 ms"
echo "P99:            $P99 ms"
echo "Max:            $MAX ms"

echo "========================================="
echo ""
echo "Raw data saved to: $OVERHEAD_FILE"

# Clean up sorted file
rm -f $SORTED_FILE
