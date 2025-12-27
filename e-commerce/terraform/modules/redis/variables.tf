variable "service_name" {
  description = "Name of the service"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where Redis will be deployed"
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs for Redis subnet group"
  type        = list(string)
}

variable "ecs_security_group_id" {
  description = "Security group ID of ECS tasks (for ingress rule)"
  type        = string
}

variable "node_type" {
  description = "ElastiCache node type"
  type        = string
  default     = "cache.t3.micro"  # Smallest/cheapest option for testing
}

