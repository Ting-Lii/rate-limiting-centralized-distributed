package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"hw8-ecommerce-api/dynamodb"
	"hw8-ecommerce-api/mysqldb"
	"hw8-ecommerce-api/ratelimit"
	"hw8-ecommerce-api/repository"

	"github.com/gorilla/mux"
)

// ==================== Models ====================

// Product represents the product model
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// InventoryItem represents warehouse inventory
type InventoryItem struct {
	ProductID int `json:"product_id"`
	Available int `json:"available"`
	Reserved  int `json:"reserved"`
}

// ErrorResponse represents the error response model
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ==================== Stores ====================

// ProductStore manages product storage in memory
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{
		products: make(map[int]Product),
	}
}

func (ps *ProductStore) Get(id int) (Product, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	product, exists := ps.products[id]
	return product, exists
}

func (ps *ProductStore) Set(id int, product Product) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.products[id] = product
}

// WarehouseStore manages inventory
type WarehouseStore struct {
	mu        sync.RWMutex
	inventory map[int]*InventoryItem
}

func NewWarehouseStore() *WarehouseStore {
	return &WarehouseStore{
		inventory: make(map[int]*InventoryItem),
	}
}

func (ws *WarehouseStore) InitializeInventory(productID, quantity int) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.inventory[productID] = &InventoryItem{
		ProductID: productID,
		Available: quantity,
		Reserved:  0,
	}
}

func (ws *WarehouseStore) Reserve(productID, quantity int) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	item, exists := ws.inventory[productID]
	if !exists {
		return &ValidationError{"product not found in warehouse"}
	}

	if item.Available < quantity {
		return &ValidationError{"insufficient inventory"}
	}

	item.Available -= quantity
	item.Reserved += quantity

	return nil
}

func (ws *WarehouseStore) Ship(productID, quantity int) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	item, exists := ws.inventory[productID]
	if !exists {
		return &ValidationError{"product not found in warehouse"}
	}

	if item.Reserved < quantity {
		return &ValidationError{"insufficient reserved inventory"}
	}

	item.Reserved -= quantity

	return nil
}

// OrderStore manages orders
type OrderStore struct {
	mu     sync.RWMutex
	nextID int
}

func NewOrderStore() *OrderStore {
	return &OrderStore{
		nextID: 1000,
	}
}

func (os *OrderStore) CreateOrder() int {
	os.mu.Lock()
	defer os.mu.Unlock()

	orderID := os.nextID
	os.nextID++

	return orderID
}

// Global stores
var (
	productStore   *ProductStore
	warehouseStore *WarehouseStore
	// orderStore     *OrderStore  // Reserved for future checkout functionality
	cartRepo repository.CartRepositoryInterface
)

// ==================== Main ====================

func main() {
	// Determine which database to use based on environment variable
	dbType := os.Getenv("DATABASE_TYPE")
	if dbType == "" {
		dbType = "mysql" // Default to MySQL
	}

	log.Printf("Initializing with database type: %s", dbType)

	switch dbType {
	case "dynamodb":
		// Initialize DynamoDB repository
		client, err := dynamodb.NewDynamoDBClient()
		if err != nil {
			log.Fatal("Failed to initialize DynamoDB client:", err)
		}
		cartRepo = dynamodb.NewCartRepository(client)
		log.Println("Using DynamoDB for shopping cart storage")

	case "mysql", "": // Default to MySQL
		// Initialize MySQL repository
		database := mysqldb.Connect()
		// Initialize schema
		if err := mysqldb.InitSchema(database); err != nil {
			log.Fatal("Failed to initialize schema:", err)
		}
		defer database.Close()
		cartRepo = mysqldb.NewCartRepository(database)
		log.Println("Using MySQL for shopping cart storage")
	default:
		log.Fatalf("Unknown DATABASE_TYPE: %s. Use 'mysql' or 'dynamodb'", dbType)
	}

	// Initialize stores
	productStore = NewProductStore()
	warehouseStore = NewWarehouseStore()
	// orderStore = NewOrderStore()  // Reserved for future checkout functionality

	// Initialize rate limiter (optional, based on environment)
	router := mux.NewRouter()
	rateLimitType := os.Getenv("RATE_LIMIT_TYPE")
	var limiter ratelimit.Limiter

	switch rateLimitType {

	case "redis":
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}

		var err error
		limiter, err = ratelimit.NewRedisLimiter(redisAddr, 100, 1*time.Minute)
		if err != nil {
			log.Printf("Redis limiter init failed: %v. Rate limiting disabled.", err)
		} else {
			log.Println("Using Redis rate limiter")
		}

	case "local":
		// Local in-memory fixed window: 100 req/min
		limiter = ratelimit.NewMemoryFixedWindowLimiter(
			100,           // limit
			1*time.Minute, // window
		)
		log.Println("Using local in-memory rate limiter")
	case "local_token_bucket":
		limiter = ratelimit.NewLocalTokenBucketLimiter(100, 60) // 100 tokens per 60 seconds
		log.Println("Using local token bucket rate limiter")

	case "redis_token_bucket":
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}

		var err error
		limiter, err = ratelimit.NewRedisTokenBucketLimiter(redisAddr, 100, 1*time.Minute) // 100 tokens per minute
		if err != nil {
			log.Printf("Redis token bucket limiter init failed: %v. Rate limiting disabled.", err)
		} else {
			log.Println("Using Redis token bucket rate limiter")
		}

	default:
		log.Println("Rate limiting disabled")
	}

	if limiter != nil {
		rateLimitMiddleware := ratelimit.NewRateLimitMiddleware(limiter)
		router.Use(rateLimitMiddleware.Handler)
	}

	// Testing purpose
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")

	// Product endpoints
	router.HandleFunc("/products/{productId}", getProduct).Methods("GET")
	router.HandleFunc("/products/{productId}/details", addProductDetails).Methods("POST")

	// Shopping Cart endpoints
	router.HandleFunc("/shopping-carts", createShoppingCart).Methods("POST")
	router.HandleFunc("/shopping-carts/{shoppingCartId:[0-9]+}/items", addItemsToCart).Methods("POST")
	router.HandleFunc("/shopping-carts/{shoppingCartId:[0-9]+}", getShoppingCart).Methods("GET")

	// Warehouse endpoints
	router.HandleFunc("/warehouse/reserve", reserveInventory).Methods("POST")
	router.HandleFunc("/warehouse/ship", shipProduct).Methods("POST")

	// Payment endpoints
	// router.HandleFunc("/payments/checkout", processPayment).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, router))
}

// ==================== Product Handlers ====================

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productIDStr := vars["productId"]

	productID, err := strconv.Atoi(productIDStr)
	if err != nil || productID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_PRODUCT_ID",
			"Product ID must be a positive integer", "")
		return
	}

	product, exists := productStore.Get(productID)
	if !exists {
		respondWithError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND",
			"Product not found", "")
		return
	}

	respondWithJSON(w, http.StatusOK, product)
}

func addProductDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productIDStr := vars["productId"]

	productID, err := strconv.Atoi(productIDStr)
	if err != nil || productID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_PRODUCT_ID",
			"Product ID must be a positive integer", "")
		return
	}

	var product Product
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&product); err != nil {
		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
			"Invalid JSON format", err.Error())
		return
	}
	defer r.Body.Close()

	if err := validateProduct(&product); err != nil {
		respondWithError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"Invalid product data", err.Error())
		return
	}

	if product.ProductID != productID {
		respondWithError(w, http.StatusBadRequest, "PRODUCT_ID_MISMATCH",
			"Product ID in URL does not match product ID in body", "")
		return
	}

	productStore.Set(productID, product)

	// Initialize inventory for this product (default 100 units)
	warehouseStore.InitializeInventory(productID, 100)

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Shopping Cart Handlers ====================

// POST /shopping-carts
func createShoppingCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID int `json:"customer_id"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
			"Invalid JSON format", err.Error())
		return
	}
	defer r.Body.Close()

	if req.CustomerID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_CUSTOMER_ID",
			"Customer ID must be a positive integer", "")
		return
	}

	// USE DATABASE REPOSITORY
	cartID, err := cartRepo.CreateCart(req.CustomerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "DB_ERROR",
			"Failed to create cart", err.Error())
		return
	}

	response := map[string]int{"shopping_cart_id": cartID}
	respondWithJSON(w, http.StatusCreated, response)
}

// GET /shopping-carts/{shoppingCartId}
func getShoppingCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartIDStr := vars["shoppingCartId"]

	cartID, err := strconv.Atoi(cartIDStr)
	if err != nil || cartID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_CART_ID",
			"Shopping cart ID must be a positive integer", "")
		return
	}

	// USE DATABASE REPOSITORY
	cart, err := cartRepo.GetCartWithItems(cartID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "CART_NOT_FOUND",
			"Shopping cart not found", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, cart)
}

// POST /shopping-carts/{shoppingCartId}/items
func addItemsToCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartIDStr := vars["shoppingCartId"]

	cartID, err := strconv.Atoi(cartIDStr)
	if err != nil || cartID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_CART_ID",
			"Shopping cart ID must be a positive integer", "")
		return
	}

	var req struct {
		ProductID int `json:"product_id"`
		Quantity  int `json:"quantity"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
			"Invalid JSON format", err.Error())
		return
	}
	defer r.Body.Close()

	err = cartRepo.AddOrUpdateItem(cartID, req.ProductID, req.Quantity)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "DB_ERROR",
			"Failed to add item", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// func checkoutCart(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	cartIDStr := vars["shoppingCartId"]

// 	cartID, err := strconv.Atoi(cartIDStr)
// 	if err != nil || cartID < 1 {
// 		respondWithError(w, http.StatusBadRequest, "INVALID_CART_ID",
// 			"Shopping cart ID must be a positive integer", "")
// 		return
// 	}

// 	cart, exists := mysqldb.GetCartWithItems(cartID)
// 	if !exists {
// 		respondWithError(w, http.StatusNotFound, "CART_NOT_FOUND",
// 			"Shopping cart not found", "")
// 		return
// 	}

// 	if len(cart.Items) == 0 {
// 		respondWithError(w, http.StatusBadRequest, "EMPTY_CART",
// 			"Cannot checkout an empty shopping cart", "")
// 		return
// 	}

// 	// Create order
// 	orderID := orderStore.CreateOrder()

// 	response := map[string]int{"order_id": orderID}
// 	respondWithJSON(w, http.StatusOK, response)
// }

// ==================== Warehouse Handlers ====================

func reserveInventory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID int `json:"product_id"`
		Quantity  int `json:"quantity"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
			"Invalid JSON format", err.Error())
		return
	}
	defer r.Body.Close()

	if req.ProductID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_PRODUCT_ID",
			"Product ID must be a positive integer", "")
		return
	}

	if req.Quantity < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_QUANTITY",
			"Quantity must be a positive integer", "")
		return
	}

	// Check if product exists
	_, exists := productStore.Get(req.ProductID)
	if !exists {
		respondWithError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND",
			"Product not found", "")
		return
	}

	// Reserve inventory
	if err := warehouseStore.Reserve(req.ProductID, req.Quantity); err != nil {
		respondWithError(w, http.StatusBadRequest, "RESERVATION_FAILED",
			"Failed to reserve inventory", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func shipProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID int `json:"product_id"`
		Quantity  int `json:"quantity"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
			"Invalid JSON format", err.Error())
		return
	}
	defer r.Body.Close()

	if req.ProductID < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_PRODUCT_ID",
			"Product ID must be a positive integer", "")
		return
	}

	if req.Quantity < 1 {
		respondWithError(w, http.StatusBadRequest, "INVALID_QUANTITY",
			"Quantity must be a positive integer", "")
		return
	}

	// Check if product exists
	_, exists := productStore.Get(req.ProductID)
	if !exists {
		respondWithError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND",
			"Product not found", "")
		return
	}

	// Ship product
	if err := warehouseStore.Ship(req.ProductID, req.Quantity); err != nil {
		respondWithError(w, http.StatusBadRequest, "SHIPPING_FAILED",
			"Failed to ship product", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== Payment Handlers ====================

// func processPayment(w http.ResponseWriter, r *http.Request) {
// 	var req struct {
// 		CreditCardNumber string `json:"credit_card_number"`
// 		ShoppingCartID   int    `json:"shopping_cart_id"`
// 	}

// 	decoder := json.NewDecoder(r.Body)
// 	if err := decoder.Decode(&req); err != nil {
// 		respondWithError(w, http.StatusBadRequest, "INVALID_JSON",
// 			"Invalid JSON format", err.Error())
// 		return
// 	}
// 	defer r.Body.Close()

// 	// Validate credit card number (13-19 digits)
// 	if len(req.CreditCardNumber) < 13 || len(req.CreditCardNumber) > 19 {
// 		respondWithError(w, http.StatusBadRequest, "INVALID_CARD_NUMBER",
// 			"Credit card number must be between 13 and 19 digits", "")
// 		return
// 	}

// 	// Validate all characters are digits
// 	for _, c := range req.CreditCardNumber {
// 		if c < '0' || c > '9' {
// 			respondWithError(w, http.StatusBadRequest, "INVALID_CARD_NUMBER",
// 				"Credit card number must contain only digits", "")
// 			return
// 		}
// 	}

// 	if req.ShoppingCartID < 1 {
// 		respondWithError(w, http.StatusBadRequest, "INVALID_CART_ID",
// 			"Shopping cart ID must be a positive integer", "")
// 		return
// 	}

// 	// Check if cart exists
// 	cart, exists := shoppingCartStore.Get(req.ShoppingCartID)
// 	if !exists {
// 		respondWithError(w, http.StatusNotFound, "CART_NOT_FOUND",
// 			"Shopping cart not found", "")
// 		return
// 	}

// 	// Check if cart is empty
// 	if len(cart.Items) == 0 {
// 		respondWithError(w, http.StatusBadRequest, "EMPTY_CART",
// 			"Cannot process payment for empty cart", "")
// 		return
// 	}

// 	// Simulate payment processing
// 	// In a real system, this would integrate with a payment gateway

// 	// For demo: decline cards ending in "0000"
// 	if len(req.CreditCardNumber) >= 4 && req.CreditCardNumber[len(req.CreditCardNumber)-4:] == "0000" {
// 		respondWithError(w, http.StatusPaymentRequired, "PAYMENT_DECLINED",
// 			"Payment was declined", "")
// 		return
// 	}

// 	// Generate transaction ID
// 	transactionID := "TXN-" + strconv.FormatInt(time.Now().Unix(), 10)

// 	response := map[string]interface{}{
// 		"success":        true,
// 		"transaction_id": transactionID,
// 	}

// 	respondWithJSON(w, http.StatusOK, response)
// }

// // ==================== Validation ====================

func validateProduct(p *Product) error {
	if p.ProductID < 1 {
		return &ValidationError{"product_id must be a positive integer"}
	}
	if len(p.SKU) < 1 || len(p.SKU) > 100 {
		return &ValidationError{"sku must be between 1 and 100 characters"}
	}
	if len(p.Manufacturer) < 1 || len(p.Manufacturer) > 200 {
		return &ValidationError{"manufacturer must be between 1 and 200 characters"}
	}
	if p.CategoryID < 1 {
		return &ValidationError{"category_id must be a positive integer"}
	}
	if p.Weight < 0 {
		return &ValidationError{"weight must be non-negative"}
	}
	if p.SomeOtherID < 1 {
		return &ValidationError{"some_other_id must be a positive integer"}
	}
	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

// ==================== Response Helpers ====================

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "JSON_ENCODING_ERROR",
			"Failed to encode response", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, errorCode, message, details string) {
	errorResponse := ErrorResponse{
		Error:   errorCode,
		Message: message,
		Details: details,
	}

	response, _ := json.Marshal(errorResponse)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
