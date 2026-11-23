# The certificate authority behind the services' mutual TLS (deploy/k8s: ClusterIssuer forge-service-ca). cert-manager's
# AWS Private CA issuer asks this CA for each service's 24-hour certificate naming spiffe://forge/svc/<identity>.
# The root stays offline in the security account; this is the subordinate that signs. VERIFY: Private CA availability in
# af-south-1; if it is not offered, run an in-cluster CA (cert-manager CA issuer, keys in KMS) instead.
resource "aws_acmpca_certificate_authority" "services" {
  type = "SUBORDINATE"
  certificate_authority_configuration {
    key_algorithm     = "EC_prime256v1"
    signing_algorithm = "SHA256WITHECDSA"
    subject {
      common_name  = "Forge service CA (${var.environment})"
      organization = "Forge Mutual"
      country      = "ZA"
    }
  }
  permanent_deletion_time_in_days = 30
}
