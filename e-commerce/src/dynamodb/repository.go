// Package dynamodb implements the CartRepository interface for DynamoDB storage.
//
// This package provides DynamoDB operations for shopping carts:
//   - CreateCart: Generates UUID (main table partition key), creates integer ID from UUID hash (GSI partition key), stores both in DynamoDB
//   - GetCartWithItems: Queries GSI by integer cart ID to retrieve cart
//   - AddOrUpdateItem: Queries GSI to get UUID, then updates cart items using main table partition key
package dynamodb

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"hw8-ecommerce-api/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// CartRepository handles DynamoDB operations for shopping carts
type CartRepository struct {
	client    *DynamoDBClient
	tableName string
}

// NewCartRepository creates a new DynamoDB cart repository
func NewCartRepository(client *DynamoDBClient) *CartRepository {
	return &CartRepository{
		client:    client,
		tableName: client.GetTableName(),
	}
}

// CreateCart creates a new shopping cart in DynamoDB
// Returns cart ID as integer for API compatibility with MySQL version
// Strategy: Use UUID string as partition key for even distribution in DynamoDB,
// but generate a deterministic integer ID from UUID hash for API compatibility
func (r *CartRepository) CreateCart(customerID int) (int, error) {
	// Generate UUID for even partition distribution in DynamoDB
	cartUUID := uuid.New()
	cartIDStr := cartUUID.String()

	// Generate deterministic integer ID from UUID hash for API compatibility
	// Hash the UUID and use first 4 bytes as integer (ensures positive values)
	hash := sha256.Sum256([]byte(cartIDStr))
	cartIDInt := int(uint32(hash[0])<<24 | uint32(hash[1])<<16 | uint32(hash[2])<<8 | uint32(hash[3]))
	// Ensure positive integer (take absolute value)
	if cartIDInt < 0 {
		cartIDInt = -cartIDInt
	}
	if cartIDInt == 0 {
		cartIDInt = 1 // Ensure non-zero
	}

	// Prepare cart item for DynamoDB
	// Store both UUID (partition key) and integer ID for lookups
	cart := map[string]interface{}{
		"cart_id":     cartIDStr, // Partition key (UUID for even distribution)
		"cart_id_int": cartIDInt, // Integer ID for API compatibility
		"customer_id": customerID,
		"items":       make(map[string]interface{}), // Empty items map
		"created_at":  time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(cart)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal cart: %w", err)
	}

	// Put item into DynamoDB
	_, err = r.client.GetClient().PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create cart in DynamoDB: %w", err)
	}

	return cartIDInt, nil
}

// GetCartWithItems retrieves a cart with all its items from DynamoDB
// Uses GSI on cart_id_int for fast query instead of table scan
func (r *CartRepository) GetCartWithItems(cartID int) (*models.ShoppingCart, error) {
	// Query using GSI on cart_id_int (much faster than Scan)
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("CartIdIntIndex"),
		KeyConditionExpression: aws.String("cart_id_int = :cartId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cartId": &types.AttributeValueMemberN{Value: strconv.Itoa(cartID)},
		},
		Limit: aws.Int32(1), // We only need one result
	}

	result, err := r.client.GetClient().Query(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to query cart in DynamoDB: %w", err)
	}

	if len(result.Items) > 0 {
		// Found the cart
		var cartData struct {
			CartID     string                     `dynamodbav:"cart_id"`
			CartIDInt  int                        `dynamodbav:"cart_id_int"`
			CustomerID int                        `dynamodbav:"customer_id"`
			Items      map[string]models.CartItem `dynamodbav:"items"`
			CreatedAt  string                     `dynamodbav:"created_at"`
		}

		err = attributevalue.UnmarshalMap(result.Items[0], &cartData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cart: %w", err)
		}

		// Parse created_at timestamp
		createdAt, err := time.Parse(time.RFC3339, cartData.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Convert items map from string keys to int keys
		items := make(map[int]models.CartItem)
		for keyStr, item := range cartData.Items {
			keyInt, err := strconv.Atoi(keyStr)
			if err != nil {
				continue
			}
			items[keyInt] = item
		}

		cart := &models.ShoppingCart{
			ID:         cartID,
			CustomerID: cartData.CustomerID,
			Items:      items,
			CreatedAt:  createdAt,
		}
		return cart, nil
	}

	return nil, fmt.Errorf("cart not found")
}

// AddOrUpdateItem adds or updates an item in an existing cart
// Uses GSI to quickly find cart UUID by integer ID, then updates cart items using UUID partition key
func (r *CartRepository) AddOrUpdateItem(cartID, productID, quantity int) error {
	// Query GSI to get the cart UUID (much faster than Scan)
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("CartIdIntIndex"),
		KeyConditionExpression: aws.String("cart_id_int = :cartId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cartId": &types.AttributeValueMemberN{Value: strconv.Itoa(cartID)},
		},
		ProjectionExpression: aws.String("cart_id"), // Only fetch cart_id (UUID)
		Limit:                aws.Int32(1),
	}

	queryResult, err := r.client.GetClient().Query(context.TODO(), queryInput)
	if err != nil {
		return fmt.Errorf("failed to find cart: %w", err)
	}

	if len(queryResult.Items) == 0 {
		return fmt.Errorf("cart not found")
	}

	// Extract UUID from query result
	var cartData struct {
		CartID string `dynamodbav:"cart_id"`
	}
	err = attributevalue.UnmarshalMap(queryResult.Items[0], &cartData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cart UUID: %w", err)
	}

	cartUUID := cartData.CartID
	if cartUUID == "" {
		return fmt.Errorf("cart UUID not found")
	}

	productIDStr := strconv.Itoa(productID)

	// Use UpdateItem to add/update the item in the nested map
	// Note: "items" is a reserved keyword, so we must use ExpressionAttributeNames
	// Since CreateCart initializes items as empty map, we can directly set nested values
	// DynamoDB doesn't allow overlapping paths (e.g., setting both #items and #items.#productId)
	updateExpression := "SET #items.#productId = :item"
	expressionAttributeNames := map[string]string{
		"#items":     "items",      // Escape reserved keyword "items"
		"#productId": productIDStr, // Product ID as map key
	}

	// Prepare the item value (CartItem structure)
	itemValue := map[string]interface{}{
		"product_id": productID,
		"quantity":   quantity,
	}

	itemAV, err := attributevalue.MarshalMap(itemValue)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	// Convert to AttributeValue
	itemAttr := &types.AttributeValueMemberM{Value: itemAV}

	expressionAttributeValues := map[string]types.AttributeValue{
		":item": itemAttr,
	}

	// Update the item in DynamoDB using UUID partition key
	_, err = r.client.GetClient().UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartUUID},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
	})
	if err != nil {
		return fmt.Errorf("failed to add/update item in DynamoDB: %w", err)
	}

	return nil
}
