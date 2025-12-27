package mysqldb

import (
	"database/sql"
	"fmt"
	"hw8-ecommerce-api/models"
)

type CartRepository struct {
	db *sql.DB
}

func NewCartRepository(db *sql.DB) *CartRepository {
	return &CartRepository{db: db}
}

// Create a new cart
func (r *CartRepository) CreateCart(customerID int) (int, error) {
	result, err := r.db.Exec(
		"INSERT INTO shopping_carts (customer_id) VALUES (?)",
		customerID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create cart: %w", err)
	}

	cartID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get cart ID: %w", err)
	}

	return int(cartID), nil
}

// Get cart with all items (testing <50ms requirement)
func (r *CartRepository) GetCartWithItems(cartID int) (*models.ShoppingCart, error) {
	// First get the cart
	var cart models.ShoppingCart
	err := r.db.QueryRow(
		"SELECT id, customer_id, created_at FROM shopping_carts WHERE id = ?",
		cartID,
	).Scan(&cart.ID, &cart.CustomerID, &cart.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cart not found")
	}
	if err != nil {
		return nil, err
	}

	// Then get items - single query with JOIN would be more efficient
	rows, err := r.db.Query(
		"SELECT product_id, quantity FROM cart_items WHERE cart_id = ?",
		cartID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cart.Items = make(map[int]models.CartItem)
	for rows.Next() {
		var item models.CartItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, err
		}
		cart.Items[item.ProductID] = item
	}

	return &cart, nil
}

// Add or update item (handles DUPLICATE KEY)
func (r *CartRepository) AddOrUpdateItem(cartID, productID, quantity int) error {
	_, err := r.db.Exec(`
        INSERT INTO cart_items (cart_id, product_id, quantity) 
        VALUES (?, ?, ?)
        ON DUPLICATE KEY UPDATE quantity = quantity + VALUES(quantity)`,
		cartID, productID, quantity,
	)
	return err
}

// Get all carts for a customer (purchase history)
func (r *CartRepository) GetCustomerCarts(customerID int) ([]models.ShoppingCart, error) {
	rows, err := r.db.Query(
		"SELECT id, customer_id, created_at FROM shopping_carts WHERE customer_id = ? ORDER BY created_at DESC",
		customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carts []models.ShoppingCart
	for rows.Next() {
		var cart models.ShoppingCart
		if err := rows.Scan(&cart.ID, &cart.CustomerID, &cart.CreatedAt); err != nil {
			return nil, err
		}
		carts = append(carts, cart)
	}

	return carts, nil
}
