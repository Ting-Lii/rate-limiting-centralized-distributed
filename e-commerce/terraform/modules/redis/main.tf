# ElastiCache Redis for Rate Limiting

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.service_name}-redis-subnet-group"
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "redis" {
  name        = "${var.service_name}-redis-sg"
  description = "Security group for Redis ElastiCache"
  vpc_id      = var.vpc_id

  ingress {
    description     = "Redis from ECS"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [var.ecs_security_group_id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.service_name}-redis-sg"
  }
}

# Use cache cluster for single node (simpler and cheaper)
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${var.service_name}-redis"
  engine               = "redis"
  node_type            = var.node_type
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  engine_version       = "7.0"
  port                 = 6379
  
  subnet_group_name    = aws_elasticache_subnet_group.redis.name
  security_group_ids   = [aws_security_group.redis.id]
  
  tags = {
    Name = "${var.service_name}-redis"
  }
}

