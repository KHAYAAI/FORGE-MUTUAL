# WORM storage (README: VERIFY-WORM). Object Lock in COMPLIANCE mode means nobody, including the root user, can delete
# or overwrite an object before its retention ends. Audit anchors, evidence packs, regulatory returns, VPC flow logs and
# CloudTrail go here. Retention: five years (PROPOSED; the retention schedule decides, docs/regulatory/policies).
locals {
  worm_buckets = {
    audit      = { retain_days = 1826, purpose = "audit anchors and evidence packs" }
    returns    = { retain_days = 1826, purpose = "regulatory returns as submitted" }
    flowlogs   = { retain_days = 400, purpose = "VPC flow logs" }
    cloudtrail = { retain_days = 1826, purpose = "CloudTrail" }
    backups    = { retain_days = 35, purpose = "database base backups and WAL archive" }
  }
}

resource "aws_s3_bucket" "worm" {
  for_each            = local.worm_buckets
  bucket              = "forge-${var.environment}-${each.key}"
  object_lock_enabled = true
  tags                = { Purpose = each.value.purpose }
}

resource "aws_s3_bucket_versioning" "worm" {
  for_each = local.worm_buckets
  bucket   = aws_s3_bucket.worm[each.key].id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_object_lock_configuration" "worm" {
  for_each = local.worm_buckets
  bucket   = aws_s3_bucket.worm[each.key].id
  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = each.value.retain_days
    }
  }
  depends_on = [aws_s3_bucket_versioning.worm]
}

resource "aws_s3_bucket_server_side_encryption_configuration" "worm" {
  for_each = local.worm_buckets
  bucket   = aws_s3_bucket.worm[each.key].id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.main[each.key == "backups" ? "backups" : "logs"].arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "worm" {
  for_each                = local.worm_buckets
  bucket                  = aws_s3_bucket.worm[each.key].id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Refuse any request that is not over TLS.
data "aws_iam_policy_document" "tls_only" {
  for_each = local.worm_buckets
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.worm[each.key].arn,
      "${aws_s3_bucket.worm[each.key].arn}/*",
    ]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

# The CloudTrail bucket takes its policy from security.tf, which adds CloudTrail's write permission to this one.
resource "aws_s3_bucket_policy" "worm" {
  for_each = { for k, v in local.worm_buckets : k => v if k != "cloudtrail" }
  bucket   = aws_s3_bucket.worm[each.key].id
  policy   = data.aws_iam_policy_document.tls_only[each.key].json
}
