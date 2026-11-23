# The data layer the Kubernetes manifests expect (deploy/k8s/README.md, items 2 and 3): Postgres and Kafka. Temporal runs
# in the cluster (or on Temporal Cloud, an outsourcing decision), and TigerBeetle on its own hosts (tigerbeetle.tf).
resource "aws_security_group" "postgres" {
  name        = "forge-${var.environment}-postgres"
  description = "Postgres: from the cluster's nodes only"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group_rule" "postgres_from_nodes" {
  type                     = "ingress"
  security_group_id        = aws_security_group.postgres.id
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  source_security_group_id = aws_eks_cluster.main.vpc_config[0].cluster_security_group_id
  description              = "Postgres from the EKS nodes"
}

resource "aws_db_subnet_group" "main" {
  name       = "forge-${var.environment}"
  subnet_ids = aws_subnet.data[*].id
}

resource "aws_db_parameter_group" "postgres" {
  name   = "forge-${var.environment}-pg17"
  family = "postgres17"
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
  parameter {
    name         = "log_statement"
    value        = "ddl"
    apply_method = "immediate"
  }
}

resource "aws_db_instance" "postgres" {
  identifier                      = "forge-${var.environment}"
  engine                          = "postgres"
  engine_version                  = "17"
  instance_class                  = var.postgres_instance_class
  allocated_storage               = 200
  max_allocated_storage           = 1000
  storage_type                    = "gp3"
  storage_encrypted               = true
  kms_key_id                      = aws_kms_key.main["data"].arn
  multi_az                        = true
  db_subnet_group_name            = aws_db_subnet_group.main.name
  vpc_security_group_ids          = [aws_security_group.postgres.id]
  parameter_group_name            = aws_db_parameter_group.postgres.name
  publicly_accessible             = false
  backup_retention_period         = 35 # point-in-time recovery for 35 days
  backup_window                   = "01:00-02:00"
  deletion_protection             = true
  skip_final_snapshot             = false
  final_snapshot_identifier       = "forge-${var.environment}-final"
  auto_minor_version_upgrade      = true
  iam_database_authentication_enabled = false # services use per-service passwords from Secrets Manager
  performance_insights_enabled    = true
  performance_insights_kms_key_id = aws_kms_key.main["data"].arn
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
  manage_master_user_password     = true
  master_user_secret_kms_key_id   = aws_kms_key.main["secrets"].arn
  username                        = "forge_admin"
}

resource "aws_security_group" "kafka" {
  name        = "forge-${var.environment}-kafka"
  description = "Kafka: TLS from the cluster's nodes only"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group_rule" "kafka_from_nodes" {
  type                     = "ingress"
  security_group_id        = aws_security_group.kafka.id
  from_port                = 9096
  to_port                  = 9096
  protocol                 = "tcp"
  source_security_group_id = aws_eks_cluster.main.vpc_config[0].cluster_security_group_id
  description              = "Kafka TLS with SASL/SCRAM from the EKS nodes"
}

resource "aws_msk_cluster" "main" {
  cluster_name           = "forge-${var.environment}"
  kafka_version          = "3.8.1" # PLACEHOLDER: the services are tested on 3.9; use the nearest version MSK offers
  number_of_broker_nodes = 3       # one per zone; replication 3, two in sync (the outbox is tested against this shape)
  broker_node_group_info {
    instance_type   = var.kafka_instance_type
    client_subnets  = aws_subnet.data[*].id
    security_groups = [aws_security_group.kafka.id]
    storage_info {
      ebs_storage_info {
        volume_size = 500
      }
    }
  }
  encryption_info {
    encryption_at_rest_kms_key_arn = aws_kms_key.main["data"].arn
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }
  client_authentication {
    sasl {
      scram = true
    }
  }
  configuration_info {
    arn      = aws_msk_configuration.main.arn
    revision = aws_msk_configuration.main.latest_revision
  }
  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.msk.name
      }
    }
  }
}

resource "aws_msk_configuration" "main" {
  name           = "forge-${var.environment}"
  kafka_versions = ["3.8.1"]
  server_properties = <<-EOT
    default.replication.factor=3
    min.insync.replicas=2
    unclean.leader.election.enable=false
    auto.create.topics.enable=false
  EOT
}

resource "aws_cloudwatch_log_group" "msk" {
  name              = "/forge/${var.environment}/msk"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.main["logs"].arn
}
