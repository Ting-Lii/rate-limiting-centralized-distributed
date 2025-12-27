output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "dynamodb_table_name" {
  description = "Name of the DynamoDB table"
  value       = module.dynamodb.table_name
}