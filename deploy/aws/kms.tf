# One key per purpose, rotated yearly, so a leak of one does not open the others, and so each can be scoped in policy.
locals {
  key_purposes = toset(["data", "ledger", "secrets", "logs", "backups"])
}

resource "aws_kms_key" "main" {
  for_each                = local.key_purposes
  description             = "Forge ${var.environment}: ${each.key}"
  enable_key_rotation     = true
  deletion_window_in_days = 30
  tags                    = { Purpose = each.key }
}

resource "aws_kms_alias" "main" {
  for_each      = local.key_purposes
  name          = "alias/forge-${var.environment}-${each.key}"
  target_key_id = aws_kms_key.main[each.key].key_id
}
