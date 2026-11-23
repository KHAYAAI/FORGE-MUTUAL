output "eks_cluster_name" {
  value = aws_eks_cluster.main.name
}

output "postgres_endpoint" {
  value = aws_db_instance.postgres.address
}

output "kafka_bootstrap_brokers_tls" {
  value = aws_msk_cluster.main.bootstrap_brokers_sasl_scram
}

output "tigerbeetle_private_ips" {
  description = "Addresses for TB_ADDRESSES in the ledger's configuration"
  value       = aws_instance.tigerbeetle[*].private_ip
}

output "service_ca_arn" {
  value = aws_acmpca_certificate_authority.services.arn
}

output "worm_buckets" {
  value = { for k, b in aws_s3_bucket.worm : k => b.id }
}
