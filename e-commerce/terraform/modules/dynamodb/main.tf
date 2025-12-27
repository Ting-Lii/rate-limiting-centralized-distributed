# DynamoDB table for shopping carts
resource "aws_dynamodb_table" "shopping_carts" {
  name           = "${var.service_name}-shopping-carts"
  billing_mode   = "PAY_PER_REQUEST"  # On-demand pricing (cost-efficient for variable workloads)
  hash_key       = "cart_id"           # Partition key

  # Attribute definitions
  attribute {
    name = "cart_id"
    type = "S"  # String (we'll use UUID strings for even distribution)
  }
  
  attribute {
    name = "cart_id_int"
    type = "N"  # Number (integer ID for API compatibility)
  }

  # Global Secondary Index for querying by integer cart ID
  # This allows fast lookups by cart_id_int instead of scanning the entire table
  global_secondary_index {
    name            = "CartIdIntIndex"
    hash_key        = "cart_id_int"
    projection_type = "ALL"  # Include all attributes for fast access
  }

  # Point-in-time recovery for development/testing
  point_in_time_recovery {
    enabled = false  # Disable for assignment cost savings
  }

  # Ensure table can be deleted (no deletion protection)
  deletion_protection_enabled = false  # Allow destruction for assignment cleanup

  # Tags
  tags = {
    Name        = "${var.service_name}-shopping-carts"
    Service     = var.service_name
    Environment = "assignment"
  }
}

