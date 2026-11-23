# Detective controls: who did what (CloudTrail, to WORM storage), what looks wrong (GuardDuty, Security Hub), and whether
# the configuration drifted (Config). The same account holds none of the workloads; in the target design these live in
# the security account (README), which the workload accounts can write to but not edit.
resource "aws_cloudtrail" "main" {
  name                          = "forge-${var.environment}"
  s3_bucket_name                = aws_s3_bucket.worm["cloudtrail"].id
  include_global_service_events = true
  is_multi_region_trail         = true
  enable_log_file_validation    = true # tamper evidence for the log files themselves
  kms_key_id                    = aws_kms_key.main["logs"].arn
  depends_on                    = [aws_s3_bucket_policy.cloudtrail_write]
}

data "aws_caller_identity" "current" {}

data "aws_iam_policy_document" "cloudtrail_write" {
  statement {
    sid       = "AclCheck"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.worm["cloudtrail"].arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }
  statement {
    sid       = "Write"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.worm["cloudtrail"].arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
}

# CloudTrail needs to write to its bucket, so this adds to the TLS-only policy in storage.tf rather than replacing it.
resource "aws_s3_bucket_policy" "cloudtrail_write" {
  bucket = aws_s3_bucket.worm["cloudtrail"].id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = concat(
      jsondecode(data.aws_iam_policy_document.tls_only["cloudtrail"].json).Statement,
      jsondecode(data.aws_iam_policy_document.cloudtrail_write.json).Statement,
    )
  })
}

resource "aws_guardduty_detector" "main" {
  enable = true
  datasources {
    s3_logs {
      enable = true
    }
    kubernetes {
      audit_logs {
        enable = true
      }
    }
  }
}

resource "aws_securityhub_account" "main" {}

resource "aws_securityhub_standards_subscription" "foundational" {
  standards_arn = "arn:aws:securityhub:${var.region}::standards/aws-foundational-security-best-practices/v/1.0.0"
  depends_on    = [aws_securityhub_account.main]
}

resource "aws_config_configuration_recorder" "main" {
  name     = "forge-${var.environment}"
  role_arn = aws_iam_role.config.arn
  recording_group {
    all_supported                 = true
    include_global_resource_types = true
  }
}

resource "aws_iam_role" "config" {
  name = "forge-${var.environment}-config"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "config.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "config" {
  role       = aws_iam_role.config.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWS_ConfigRole"
}
