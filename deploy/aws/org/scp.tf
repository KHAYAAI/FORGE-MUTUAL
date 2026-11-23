# Organisation-level guardrails (Service Control Policies), applied in the management account. A workload account cannot
# lift them, whatever its own administrators do. DRAFT: never applied.
variable "workload_ou_id" {
  type = string # PLACEHOLDER: the organisational unit holding the staging and production accounts
}

# Nothing may run outside South Africa: a different region is an offshoring decision (Directive 3/2018).
resource "aws_organizations_policy" "region_lock" {
  name = "forge-region-lock"
  type = "SERVICE_CONTROL_POLICY"
  content = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "DenyOutsideSouthAfrica"
      Effect    = "Deny"
      NotAction = ["iam:*", "organizations:*", "sts:*", "support:*", "cloudfront:*", "route53:*", "budgets:*", "ce:*"]
      Resource  = "*"
      Condition = { StringNotEquals = { "aws:RequestedRegion" = ["af-south-1"] } }
    }]
  })
}

# The audit trail and the detective controls cannot be switched off from inside a workload account.
resource "aws_organizations_policy" "keep_logging" {
  name = "forge-keep-logging"
  type = "SERVICE_CONTROL_POLICY"
  content = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "DenyDisablingAudit"
      Effect = "Deny"
      Action = [
        "cloudtrail:StopLogging", "cloudtrail:DeleteTrail", "cloudtrail:UpdateTrail",
        "guardduty:DeleteDetector", "guardduty:DisassociateFromMasterAccount",
        "securityhub:DisableSecurityHub", "config:StopConfigurationRecorder", "config:DeleteConfigurationRecorder",
        "s3:PutBucketObjectLockConfiguration", "s3:BypassGovernanceRetention",
      ]
      Resource = "*"
    }]
  })
}

# No public data stores and no weakening of encryption.
resource "aws_organizations_policy" "no_public_data" {
  name = "forge-no-public-data"
  type = "SERVICE_CONTROL_POLICY"
  content = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "DenyPublicS3"
        Effect   = "Deny"
        Action   = ["s3:PutAccountPublicAccessBlock", "s3:DeletePublicAccessBlock", "s3:PutBucketPublicAccessBlock"]
        Resource = "*"
        Condition = { StringEquals = { "s3:PublicAccessBlockConfiguration/BlockPublicAcls" = "false" } }
      },
      {
        Sid      = "DenyUnencryptedRds"
        Effect   = "Deny"
        Action   = ["rds:CreateDBInstance", "rds:CreateDBCluster"]
        Resource = "*"
        Condition = { Bool = { "rds:StorageEncrypted" = "false" } }
      },
    ]
  })
}

resource "aws_organizations_policy_attachment" "all" {
  for_each  = { region = aws_organizations_policy.region_lock.id, logging = aws_organizations_policy.keep_logging.id, data = aws_organizations_policy.no_public_data.id }
  policy_id = each.value
  target_id = var.workload_ou_id
}
