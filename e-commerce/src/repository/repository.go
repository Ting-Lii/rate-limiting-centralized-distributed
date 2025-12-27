package repository

import "hw8-ecommerce-api/models"

// CartRepositoryInterface defines the interface for cart repository operations
// This allows switching between MySQL and DynamoDB implementations
type CartRepositoryInterface interface {
	CreateCart(customerID int) (int, error)
	GetCartWithItems(cartID int) (*models.ShoppingCart, error)
	AddOrUpdateItem(cartID, productID, quantity int) error
}
