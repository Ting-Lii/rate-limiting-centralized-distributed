// Package dynamodb provides AWS DynamoDB client initialization and configuration.
//
// This package handles the creation and setup of DynamoDB clients for the shopping cart
// repository implementation. It reads configuration from environment variables:
//   - DYNAMODB_TABLE_NAME: Name of the DynamoDB table (defaults to "shopping-carts")
//   - AWS_REGION: AWS region for DynamoDB operations (defaults to "us-west-2")
//
// The DynamoDBClient wraps the AWS SDK v2 DynamoDB client and provides access to the
// underlying client instance and table name for use by the repository implementation.
package dynamodb

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DynamoDBClient wraps the AWS DynamoDB client
type DynamoDBClient struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBClient creates a new DynamoDB client
func NewDynamoDBClient() (*DynamoDBClient, error) {
	// Get table name from environment variable
	tableName := os.Getenv("DYNAMODB_TABLE_NAME")
	if tableName == "" {
		tableName = "shopping-carts" // Default fallback
	}

	// Get AWS region from environment variable
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2" // Default fallback
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	return &DynamoDBClient{
		client:    client,
		tableName: tableName,
	}, nil
}

// GetClient returns the underlying DynamoDB client
func (d *DynamoDBClient) GetClient() *dynamodb.Client {
	return d.client
}

// GetTableName returns the DynamoDB table name
func (d *DynamoDBClient) GetTableName() string {
	return d.tableName
}
