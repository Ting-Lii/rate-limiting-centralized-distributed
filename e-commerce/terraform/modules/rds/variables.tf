variable "service_name" {
  description = "Name of the service"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where RDS will be deployed"
  type        = string
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs for RDS subnet group"
  type        = list(string)
}

variable "ecs_security_group_id" {
  description = "Security group ID of ECS tasks that need database access"
  type        = string
}

variable "db_name" {
  description = "Name of the initial database"
  type        = string
  default     = "shoppingcart"
}

variable "db_username" {
  description = "Master username for the database"
  type        = string
  default     = "admin"
}

variable "db_password" {
  description = "Master password for the database"
  type        = string
  sensitive   = true
}