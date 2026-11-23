variable "region" {
  description = "The only region used. Personal data stays in South Africa (docs/regulatory/data-classes.csv)."
  type        = string
  default     = "af-south-1"
  validation {
    condition     = var.region == "af-south-1"
    error_message = "Forge runs in af-south-1 only. A different region is an offshoring decision (Directive 3/2018) and needs the board and a PA notice first."
  }
}

variable "environment" {
  type = string
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "environment must be staging or production."
  }
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}

variable "azs" {
  description = "Three availability zones."
  type        = list(string)
  default     = ["af-south-1a", "af-south-1b", "af-south-1c"]
}

variable "kubernetes_version" {
  type    = string
  default = "1.31" # PLACEHOLDER: use the newest version EKS supports when applying
}

variable "postgres_instance_class" {
  type    = string
  default = "db.r6g.large" # PLACEHOLDER: size from the load test (make load-ledger) on production hardware
}

variable "kafka_instance_type" {
  type    = string
  default = "kafka.m5.large" # PLACEHOLDER
}

variable "tigerbeetle_instance_type" {
  description = "Dedicated hosts for the ledger. Six replicas, two per zone. Must have the io_uring-capable kernel TigerBeetle needs."
  type        = string
  default     = "i3en.xlarge" # PLACEHOLDER: confirm availability in af-south-1 and size from measurement
}

variable "admin_cidrs" {
  description = "Addresses allowed to reach the EKS API. Empty means private access only (recommended)."
  type        = list(string)
  default     = []
}

variable "log_retention_days" {
  description = "CloudWatch retention. Audit evidence is in the WORM bucket, not here."
  type        = number
  default     = 400
}
