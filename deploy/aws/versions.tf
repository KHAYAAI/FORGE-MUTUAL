# AWS landing zone for Forge Mutual (production and staging). DRAFT: never planned or applied (no AWS account is
# connected to this work yet). Region: af-south-1 (Cape Town), the only AWS region in South Africa as far as we know
# (VERIFY with the lawyer and AWS). See docs/HOSTING.md and deploy/aws/README.md.
terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
  }
  # State lives in the security account's bucket, encrypted, with locking. Fill in once the accounts exist.
  backend "s3" {
    bucket         = "PLACEHOLDER-forge-terraform-state"
    key            = "landing-zone/terraform.tfstate"
    region         = "af-south-1"
    encrypt        = true
    dynamodb_table = "PLACEHOLDER-forge-terraform-locks"
  }
}

provider "aws" {
  region = var.region
  # Only this region may be used; org/scp.tf denies every other.
  default_tags {
    tags = {
      Project     = "forge-mutual"
      Environment = var.environment
      ManagedBy   = "terraform"
      DataClass   = "restricted"
    }
  }
}
