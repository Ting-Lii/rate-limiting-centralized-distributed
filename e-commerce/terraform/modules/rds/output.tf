output "db_endpoint" {
  description = "The connection endpoint"
  value       = aws_db_instance.this.endpoint
}

output "db_name" {
  description = "The database name"
  value       = aws_db_instance.this.db_name
}

output "db_username" {
  description = "The master username"
  value       = aws_db_instance.this.username
  sensitive   = true
}

output "db_password" {
  description = "The master password"
  value       = aws_db_instance.this.password
  sensitive   = true
}