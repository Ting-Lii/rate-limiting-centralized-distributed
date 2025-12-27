// Package main provides eventual consistency testing for DynamoDB-backed shopping cart API.
//
// Tests three read-after-write scenarios (10 iterations each):
//  1. create_then_read: Create cart, immediately read it
//  2. add_item_then_read: Add item to cart, immediately read cart
//  3. rapid_updates: Perform 5 rapid item additions, then read cart
//
// Measures: consistency success rate, attempts needed, time to consistency (ms).
// Results saved as NDJSON format with summary statistics.
//
// Usage:
//
//	go run test_eventual_consistency.go <baseURL> [output_file]
//
// Example:
//
//	go run test_eventual_consistency.go localhost:8080 consistency_test_results.jsonl
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type ConsistencyTestResult struct {
	TestScenario          string    `json:"test_scenario"`
	WriteOperation        string    `json:"write_operation"`
	ReadOperation         string    `json:"read_operation"`
	WriteResponseTime     float64   `json:"write_response_time_ms"`
	ReadResponseTime      float64   `json:"read_response_time_ms"`
	DelayBetweenOps       float64   `json:"delay_between_ops_ms"`
	ConsistencySuccess    bool      `json:"consistency_success"`
	Attempts              int       `json:"attempts"`
	TotalTimeToConsistent float64   `json:"total_time_to_consistent_ms"`
	Timestamp             time.Time `json:"timestamp"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func testCreateThenRead(baseURL string) ConsistencyTestResult {
	fmt.Println("Test 1: Create cart then immediately read it...")

	// Create cart
	startWrite := time.Now()
	body := `{"customer_id":9999}`
	resp, err := httpClient.Post(
		baseURL+"/shopping-carts",
		"application/json",
		bytes.NewBufferString(body),
	)
	writeTime := time.Since(startWrite).Milliseconds()

	var cartID int
	if err == nil && resp != nil && resp.StatusCode == 201 {
		defer resp.Body.Close()
		var cartResp map[string]int
		if err := json.NewDecoder(resp.Body).Decode(&cartResp); err == nil {
			cartID = cartResp["shopping_cart_id"]
		}
	}

	if cartID == 0 {
		return ConsistencyTestResult{
			TestScenario:       "create_then_read",
			ConsistencySuccess: false,
			Attempts:           0,
		}
	}

	// Immediately try to read the cart
	delayStart := time.Now()
	startRead := time.Now()

	readResp, err := httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
	readTime := time.Since(startRead).Milliseconds()
	delayTime := time.Since(delayStart).Milliseconds()

	consistent := false
	attempts := 1

	// If not consistent, retry up to 5 times with small delays
	for attempts <= 5 && !consistent {
		if err == nil && readResp != nil {
			if readResp.StatusCode == 200 {
				consistent = true
				break
			}
			readResp.Body.Close()
		}

		if !consistent && attempts < 5 {
			time.Sleep(100 * time.Millisecond) // Wait 100ms before retry
			attempts++
			startRead = time.Now()
			readResp, err = httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
			readTime = time.Since(startRead).Milliseconds()
			delayTime = time.Since(delayStart).Milliseconds()
		}
	}

	return ConsistencyTestResult{
		TestScenario:          "create_then_read",
		WriteOperation:        "POST /shopping-carts",
		ReadOperation:         "GET /shopping-carts/{id}",
		WriteResponseTime:     float64(writeTime),
		ReadResponseTime:      float64(readTime),
		DelayBetweenOps:       float64(delayTime),
		ConsistencySuccess:    consistent,
		Attempts:              attempts,
		TotalTimeToConsistent: float64(delayTime),
		Timestamp:             time.Now().UTC(),
	}
}

func testAddItemThenRead(baseURL string) ConsistencyTestResult {
	fmt.Println("Test 2: Add item then immediately read cart...")

	// First create a cart
	body := `{"customer_id":8888}`
	resp, err := httpClient.Post(
		baseURL+"/shopping-carts",
		"application/json",
		bytes.NewBufferString(body),
	)

	var cartID int
	if err == nil && resp != nil && resp.StatusCode == 201 {
		defer resp.Body.Close()
		var cartResp map[string]int
		if err := json.NewDecoder(resp.Body).Decode(&cartResp); err == nil {
			cartID = cartResp["shopping_cart_id"]
		}
	}

	if cartID == 0 {
		return ConsistencyTestResult{
			TestScenario:       "add_item_then_read",
			ConsistencySuccess: false,
			Attempts:           0,
		}
	}

	// Add item immediately
	startWrite := time.Now()
	addBody := `{"product_id":1,"quantity":5}`
	addResp, err := httpClient.Post(
		fmt.Sprintf("%s/shopping-carts/%d/items", baseURL, cartID),
		"application/json",
		bytes.NewBufferString(addBody),
	)
	writeTime := time.Since(startWrite).Milliseconds()

	if err != nil || addResp == nil || addResp.StatusCode != 204 {
		if addResp != nil {
			addResp.Body.Close()
		}
		return ConsistencyTestResult{
			TestScenario:       "add_item_then_read",
			ConsistencySuccess: false,
			Attempts:           0,
		}
	}
	addResp.Body.Close()

	// Immediately try to read the cart
	delayStart := time.Now()
	startRead := time.Now()

	readResp, err := httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
	readTime := time.Since(startRead).Milliseconds()
	delayTime := time.Since(delayStart).Milliseconds()

	consistent := false
	attempts := 1

	// Check if item appears in cart (verify consistency)
	for attempts <= 10 && !consistent {
		if err == nil && readResp != nil && readResp.StatusCode == 200 {
			defer readResp.Body.Close()
			var cart map[string]interface{}
			if err := json.NewDecoder(readResp.Body).Decode(&cart); err == nil {
				if items, ok := cart["items"].(map[string]interface{}); ok {
					if len(items) > 0 {
						// Item found in cart
						consistent = true
						break
					}
				}
			}
		}

		if !consistent && attempts < 10 {
			time.Sleep(100 * time.Millisecond)
			attempts++
			startRead = time.Now()
			readResp, err = httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
			readTime = time.Since(startRead).Milliseconds()
			delayTime = time.Since(delayStart).Milliseconds()
		} else if readResp != nil {
			readResp.Body.Close()
		}
	}

	return ConsistencyTestResult{
		TestScenario:          "add_item_then_read",
		WriteOperation:        "POST /shopping-carts/{id}/items",
		ReadOperation:         "GET /shopping-carts/{id}",
		WriteResponseTime:     float64(writeTime),
		ReadResponseTime:      float64(readTime),
		DelayBetweenOps:       float64(delayTime),
		ConsistencySuccess:    consistent,
		Attempts:              attempts,
		TotalTimeToConsistent: float64(delayTime),
		Timestamp:             time.Now().UTC(),
	}
}

func testRapidUpdates(baseURL string) ConsistencyTestResult {
	fmt.Println("Test 3: Rapid updates to same cart...")

	// Create cart
	body := `{"customer_id":7777}`
	resp, err := httpClient.Post(
		baseURL+"/shopping-carts",
		"application/json",
		bytes.NewBufferString(body),
	)

	var cartID int
	if err == nil && resp != nil && resp.StatusCode == 201 {
		defer resp.Body.Close()
		var cartResp map[string]int
		if err := json.NewDecoder(resp.Body).Decode(&cartResp); err == nil {
			cartID = cartResp["shopping_cart_id"]
		}
	}

	if cartID == 0 {
		return ConsistencyTestResult{
			TestScenario:       "rapid_updates",
			ConsistencySuccess: false,
			Attempts:           0,
		}
	}

	// Make 5 rapid updates immediately (no delay to test eventual consistency)
	startWrite := time.Now()
	successCount := 0
	for i := 1; i <= 5; i++ {
		addBody := fmt.Sprintf(`{"product_id":%d,"quantity":%d}`, i, i*2)
		addResp, err := httpClient.Post(
			fmt.Sprintf("%s/shopping-carts/%d/items", baseURL, cartID),
			"application/json",
			bytes.NewBufferString(addBody),
		)
		if err == nil && addResp != nil && addResp.StatusCode == 204 {
			successCount++
			addResp.Body.Close()
		} else if addResp != nil {
			addResp.Body.Close()
		}
		// Small delay between writes
		time.Sleep(10 * time.Millisecond)
	}
	writeTime := time.Since(startWrite).Milliseconds()

	// Immediately read cart
	delayStart := time.Now()
	startRead := time.Now()
	readResp, err := httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
	readTime := time.Since(startRead).Milliseconds()
	delayTime := time.Since(delayStart).Milliseconds()

	consistent := false
	attempts := 1
	expectedItems := 5

	for attempts <= 10 && !consistent {
		if err == nil && readResp != nil && readResp.StatusCode == 200 {
			defer readResp.Body.Close()
			var cart map[string]interface{}
			if err := json.NewDecoder(readResp.Body).Decode(&cart); err == nil {
				if items, ok := cart["items"].(map[string]interface{}); ok {
					if len(items) >= expectedItems {
						consistent = true
						break
					}
				}
			}
		}

		if !consistent && attempts < 10 {
			time.Sleep(100 * time.Millisecond)
			attempts++
			startRead = time.Now()
			readResp, err = httpClient.Get(fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartID))
			readTime = time.Since(startRead).Milliseconds()
			delayTime = time.Since(delayStart).Milliseconds()
		} else if readResp != nil {
			readResp.Body.Close()
		}
	}

	return ConsistencyTestResult{
		TestScenario:          "rapid_updates",
		WriteOperation:        "POST /shopping-carts/{id}/items (5x)",
		ReadOperation:         "GET /shopping-carts/{id}",
		WriteResponseTime:     float64(writeTime),
		ReadResponseTime:      float64(readTime),
		DelayBetweenOps:       float64(delayTime),
		ConsistencySuccess:    consistent,
		Attempts:              attempts,
		TotalTimeToConsistent: float64(delayTime),
		Timestamp:             time.Now().UTC(),
	}
}

func runConsistencyTests(baseURL string, outputFile string) {
	results := []ConsistencyTestResult{}

	// Run each test 10 times to get average behavior
	for i := 0; i < 10; i++ {
		fmt.Printf("\n--- Test Run %d/10 ---\n", i+1)

		result1 := testCreateThenRead(baseURL)
		results = append(results, result1)
		fmt.Printf("  Result: Consistent=%v, Attempts=%d, Time=%.2fms\n",
			result1.ConsistencySuccess, result1.Attempts, result1.TotalTimeToConsistent)

		time.Sleep(500 * time.Millisecond) // Delay between test runs

		result2 := testAddItemThenRead(baseURL)
		results = append(results, result2)
		fmt.Printf("  Result: Consistent=%v, Attempts=%d, Time=%.2fms\n",
			result2.ConsistencySuccess, result2.Attempts, result2.TotalTimeToConsistent)

		time.Sleep(500 * time.Millisecond)

		result3 := testRapidUpdates(baseURL)
		results = append(results, result3)
		fmt.Printf("  Result: Consistent=%v, Attempts=%d, Time=%.2fms\n",
			result3.ConsistencySuccess, result3.Attempts, result3.TotalTimeToConsistent)

		time.Sleep(500 * time.Millisecond)
	}

	// Save results
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, r := range results {
		_ = enc.Encode(r)
	}

	// Summary statistics
	var consistentCount int
	var totalAttempts int
	var totalDelayTime float64
	var scenarioStats = make(map[string]struct {
		count         int
		consistent    int
		totalAttempts int
		totalTime     float64
	})

	for _, r := range results {
		if r.ConsistencySuccess {
			consistentCount++
		}
		totalAttempts += r.Attempts
		totalDelayTime += r.TotalTimeToConsistent

		stat := scenarioStats[r.TestScenario]
		stat.count++
		if r.ConsistencySuccess {
			stat.consistent++
		}
		stat.totalAttempts += r.Attempts
		stat.totalTime += r.TotalTimeToConsistent
		scenarioStats[r.TestScenario] = stat
	}

	fmt.Printf("\n=== Consistency Test Summary ===\n")
	fmt.Printf("Total tests: %d\n", len(results))
	fmt.Printf("Consistent immediately: %d (%.1f%%)\n", consistentCount, float64(consistentCount)/float64(len(results))*100)
	fmt.Printf("Average attempts to consistency: %.2f\n", float64(totalAttempts)/float64(len(results)))
	fmt.Printf("Average time to consistency: %.2f ms\n", totalDelayTime/float64(len(results)))

	fmt.Printf("\n--- By Scenario ---\n")
	for scenario, stat := range scenarioStats {
		avgTime := stat.totalTime / float64(stat.count)
		avgAttempts := float64(stat.totalAttempts) / float64(stat.count)
		consistencyRate := float64(stat.consistent) / float64(stat.count) * 100
		fmt.Printf("%s:\n", scenario)
		fmt.Printf("  Consistency rate: %.1f%% (%d/%d)\n", consistencyRate, stat.consistent, stat.count)
		fmt.Printf("  Avg attempts: %.2f\n", avgAttempts)
		fmt.Printf("  Avg time: %.2f ms\n", avgTime)
	}

	fmt.Printf("\nResults saved to %s\n", outputFile)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_eventual_consistency.go <baseURL or host:port> [output_file]")
		fmt.Println("Example: go run test_eventual_consistency.go localhost:8080 consistency_test_results.jsonl")
		os.Exit(1)
	}

	baseURL := os.Args[1]
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	outputFile := "consistency_test_results.jsonl"
	if len(os.Args) >= 3 {
		outputFile = os.Args[2]
	}

	runConsistencyTests(baseURL, outputFile)
}
