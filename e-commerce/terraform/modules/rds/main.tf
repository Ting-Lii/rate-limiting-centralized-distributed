resource "aws_db_instance" "this" {
    identifier = "${var.service_name}-mysql"

    # Free tier configuration
    engine         = "mysql"
    engine_version = "8.0"
    instance_class = "db.t3.micro"
    
    # Storage
    allocated_storage = 20
    storage_type      = "gp2"

    # Database configuration
    db_name  = var.db_name
    username = var.db_username
    password = var.db_password == "" ? random_password.db_password.result : var.db_password
    
    # Network configuration
    db_subnet_group_name   = aws_db_subnet_group.this.name
    vpc_security_group_ids = [aws_security_group.rds.id]
    publicly_accessible    = false  
    
    # Backup and maintenance (following assignment requirements)
    skip_final_snapshot       = true  # For assignmenn, not for production!
    delete_automated_backups  = true
    backup_retention_period   = 0     # Disable backups for assignment
    
    tags = {
        Name = "${var.service_name}-mysql"
    }

}


# Generate a random password for the database
resource "random_password" "db_password" {
  length  = 16
  special = true
  # Exclude problematic characters for RDS
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# Create a subnet group for RDS (needs at least 2 subnets in different AZs)
resource "aws_db_subnet_group" "this" {
  name       = "${var.service_name}-db-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.service_name} DB subnet group"
  }
}

# Security group for RDS
resource "aws_security_group" "rds" {
  name        = "${var.service_name}-rds-sg"
  description = "Security group for RDS MySQL instance"
  vpc_id      = var.vpc_id

  # Allow inbound MySQL traffic from ECS tasks only
  ingress {
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [var.ecs_security_group_id]
    description     = "MySQL access from ECS tasks"
  }

  # Allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }

  tags = {
    Name = "${var.service_name}-rds-sg"
  }
}