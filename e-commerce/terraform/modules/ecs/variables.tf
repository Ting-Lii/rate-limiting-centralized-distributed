variable "service_name" {
  type        = string
  description = "Base name for ECS resources"
}

variable "image" {
  type        = string
  description = "ECR image URI (with tag)"
}

variable "container_port" {
  type        = number
  description = "Port your app listens on"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for FARGATE tasks"
}

variable "security_group_ids" {
  type        = list(string)
  description = "SGs for FARGATE tasks"
}

variable "execution_role_arn" {
  type        = string
  description = "ECS Task Execution Role ARN"
}

variable "task_role_arn" {
  type        = string
  description = "IAM Role ARN for app permissions"
}

variable "log_group_name" {
  type        = string
  description = "CloudWatch log group name"
}

variable "ecs_count" {
  type        = number
  default     = 1
  description = "Desired Fargate task count"
}

variable "region" {
  type        = string
  description = "AWS region (for awslogs driver)"
}

variable "cpu" {
  type        = string
  default     = "256"
  description = "vCPU units"
}

variable "memory" {
  type        = string
  default     = "512"
  description = "Memory (MiB)"
}

variable "db_endpoint" {
  description = "Database endpoint"
  type        = string
  default     = ""
}

variable "db_name" {
  description = "Database name"
  type        = string
  default     = ""
}

variable "db_username" {
  description = "Database username"
  type        = string
  default     = ""
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
  default     = ""
}

variable "dynamodb_table_name" {
  description = "DynamoDB table name for shopping carts"
  type        = string
  default     = ""
}

variable "redis_address" {
  description = "Redis address (host:port) for rate limiting"
  type        = string
  default     = ""
}