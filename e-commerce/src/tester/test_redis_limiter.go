package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"hw8-ecommerce-api/ratelimit"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize Redis limiter
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	limiter, err := ratelimit.NewRedisLimiter(redisAddr, 5, 1*time.Minute)
	if err != nil {
		log.Fatalf("Failed to initialize Redis limiter: %v", err)
	}

	// Create rate limit middleware
	rateLimitMiddleware := ratelimit.NewRateLimitMiddleware(limiter) // 5 req/min for testing

	router := mux.NewRouter()

	// Test endpoint
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Request allowed",
			"status":  "success",
		})
	}).Methods("GET")

	// Apply rate limiting to all routes
	router.Use(rateLimitMiddleware.Handler)

	log.Println("Test server starting on :8080")
	log.Println("Test with: curl -H 'X-User-ID: test-user' http://localhost:8080/test")
	log.Fatal(http.ListenAndServe(":8080", router))
}
