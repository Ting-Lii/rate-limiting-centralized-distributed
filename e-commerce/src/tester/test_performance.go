// Package main provides performance testing for the e-commerce API.
//
// This test script measures response times for three shopping cart operations:
//   - create_cart: POST /shopping-carts (creates a new shopping cart)
//   - add_items: POST /shopping-carts/{id}/items (adds items to a cart)
//   - get_cart: GET /shopping-carts/{id} (retrieves cart with all items)
//
// The script works with both MySQL and DynamoDB backends, as the API interface
// remains the same regardless of the underlying database.
//
// Usage:
//
//	go run test_performance.go <baseURL> <output_file>
//
// Example:
//
//	go run test_performance.go http://localhost:8080 dynamodb_test_results.jsonl
//	go run test_performance.go 35.95.119.4:8080 aws_test_results.jsonl
//
// Output:
//   - Results are saved as newline-delimited JSON (NDJSON) format
//   - Each line contains: operation, response_time (ms), success, status_code, timestamp
//   - Prints summary statistics: overall average response time and get_cart average response time
//
// Configuration:
//   - testIterations: Number of times each operation is tested (default: 50)
//   - Change this value to adjust test duration: 10 = fast testing, 50 = full performance test
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

type TestResult struct {
	Operation    string    `json:"operation"`
	ResponseTime float64   `json:"response_time"` // ms
	Success      bool      `json:"success"`
	StatusCode   int       `json:"status_code"`
	Timestamp    time.Time `json:"timestamp"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// testIterations controls how many times each operation type is tested
// Change this value to adjust test duration: 10 = fast testing, 50 = full performance test
var testIterations = 50

func runPerformanceTest(baseURL string, outputFile string) {
	totalOps := testIterations * 3 // create + add + get
	results := make([]TestResult, 0, totalOps)
	cartIDs := make([]int, 0, testIterations)

	// 1) Create carts
	for i := 0; i < testIterations; i++ {
		start := time.Now()
		body := fmt.Sprintf(`{"customer_id":%d}`, i+1)
		resp, err := httpClient.Post(
			baseURL+"/shopping-carts",
			"application/json",
			bytes.NewBufferString(body),
		)

		elapsed := time.Since(start).Milliseconds()
		tr := TestResult{
			Operation:    "create_cart",
			ResponseTime: float64(elapsed),
			Success:      err == nil && resp != nil && resp.StatusCode == http.StatusCreated,
			StatusCode:   codeOf(resp),
			Timestamp:    time.Now().UTC(),
		}
		results = append(results, tr)

		if err != nil || resp == nil {
			continue
		}
		func() {
			defer resp.Body.Close()
			if tr.Success {
				var cartResp map[string]int
				if err := json.NewDecoder(resp.Body).Decode(&cartResp); err == nil {
					if id, ok := cartResp["shopping_cart_id"]; ok {
						cartIDs = append(cartIDs, id)
					}
				}
			}
		}()
	}

	// 2) Add items to carts
	for i := 0; i < testIterations && i < len(cartIDs); i++ {
		start := time.Now()
		url := fmt.Sprintf("%s/shopping-carts/%d/items", baseURL, cartIDs[i])

		resp, err := httpClient.Post(
			url,
			"application/json",
			bytes.NewBufferString(`{"product_id":1,"quantity":5}`),
		)

		elapsed := time.Since(start).Milliseconds()
		tr := TestResult{
			Operation:    "add_items",
			ResponseTime: float64(elapsed),
			Success:      err == nil && resp != nil && resp.StatusCode == http.StatusNoContent,
			StatusCode:   codeOf(resp),
			Timestamp:    time.Now().UTC(),
		}
		results = append(results, tr)
		if resp != nil {
			resp.Body.Close()
		}
	}

	// 3) Get carts
	for i := 0; i < testIterations && i < len(cartIDs); i++ {
		start := time.Now()
		url := fmt.Sprintf("%s/shopping-carts/%d", baseURL, cartIDs[i])

		resp, err := httpClient.Get(url)

		elapsed := time.Since(start).Milliseconds()
		tr := TestResult{
			Operation:    "get_cart",
			ResponseTime: float64(elapsed),
			Success:      err == nil && resp != nil && resp.StatusCode == http.StatusOK,
			StatusCode:   codeOf(resp),
			Timestamp:    time.Now().UTC(),
		}
		results = append(results, tr)
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Save results: one JSON object per line (NDJSON)
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Failed to open results file: %v\n", err)
	} else {
		enc := json.NewEncoder(f)
		for _, r := range results {
			_ = enc.Encode(r) // each line: {"operation": "...", ...}
		}
		_ = f.Close()
	}

	// Summary
	var total float64
	var okCount int
	var getCartTotal float64
	var getCartCount int

	for _, r := range results {
		total += r.ResponseTime
		if r.Success {
			okCount++
		}

		// Calculate get_cart specific metrics
		if r.Operation == "get_cart" {
			getCartTotal += r.ResponseTime
			getCartCount++
		}
	}

	overallAvg := 0.0
	if len(results) > 0 {
		overallAvg = total / float64(len(results))
	}

	getCartAvg := 0.0
	if getCartCount > 0 {
		getCartAvg = getCartTotal / float64(getCartCount)
	}

	fmt.Printf("Test Complete!\n")
	fmt.Printf("Total operations: %d\n", len(results))
	fmt.Printf("Successful: %d\n", okCount)
	fmt.Printf("Overall Average response time: %.2f ms\n", overallAvg)
	fmt.Printf("Average response time of get_cart: %.2f ms\n", getCartAvg)
	fmt.Printf("Results saved to %s (newline-delimited JSON)\n", outputFile)
}

func codeOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run test_performance.go <baseURL or host:port> <output_file>")
		fmt.Println("Example: go run test_performance.go localhost:8080 dynamodb_test_results.jsonl")
		os.Exit(1)
	}
	baseURL := os.Args[1]
	outputFile := os.Args[2]
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	runPerformanceTest(baseURL, outputFile)
}
