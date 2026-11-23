# Secrets Manager holds each service's secrets; External Secrets Operator copies them into the Kubernetes Secrets the
# manifests mount as files (`<name>-secrets`, deploy/k8s/README.md item 3). Values are never in Terraform: only the empty
# containers are created, and a person (or a rotation) fills them.
locals {
  service_secrets = toset([
    "party", "consent", "policy", "onboarding", "consolidated", "notifications", "audit", "ledger", "fx", "deposits",
    "lending", "acquiring", "rails", "partner-router", "payments", "payouts", "recon", "treasury", "compliance",
  ])
}

resource "aws_secretsmanager_secret" "service" {
  for_each                = local.service_secrets
  name                    = "forge/${var.environment}/${each.key}"
  kms_key_id              = aws_kms_key.main["secrets"].arn
  recovery_window_in_days = 30
}

# The role External Secrets runs as: it may read only forge/<environment>/*, nothing else in the account.
data "aws_iam_policy_document" "external_secrets" {
  statement {
    actions   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
    resources = ["arn:aws:secretsmanager:${var.region}:${data.aws_caller_identity.current.account_id}:secret:forge/${var.environment}/*"]
  }
  statement {
    actions   = ["kms:Decrypt"]
    resources = [aws_kms_key.main["secrets"].arn]
  }
}

resource "aws_iam_role" "external_secrets" {
  name = "forge-${var.environment}-external-secrets"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.eks.arn }
      Condition = {
        StringEquals = {
          "${replace(aws_iam_openid_connect_provider.eks.url, "https://", "")}:sub" = "system:serviceaccount:external-secrets:external-secrets"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "external_secrets" {
  role   = aws_iam_role.external_secrets.id
  policy = data.aws_iam_policy_document.external_secrets.json
}
