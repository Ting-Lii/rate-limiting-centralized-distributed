package models

import (
	"time"
)

type ShoppingCart struct {
	ID         int              `json:"shopping_cart_id"`
	CustomerID int              `json:"customer_id"`
	Items      map[int]CartItem `json:"items"`
	CreatedAt  time.Time        `json:"created_at"`
}

// CartItem represents an item in the shopping cart
type CartItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}
