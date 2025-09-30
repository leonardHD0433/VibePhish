# Variables for FYPhish Infrastructure
variable "environment" {
  description = "Environment name (development, staging, production)"
  type        = string
  validation {
    condition     = contains(["development", "staging", "production"], var.environment)
    error_message = "Environment must be one of: development, staging, production."
  }
}

variable "azure_account" {
  description = "Azure account identifier for multi-account deployment"
  type        = string
  default     = "account1"
  validation {
    condition     = contains(["account1", "account2", "account3"], var.azure_account)
    error_message = "Azure account must be one of: account1, account2, account3."
  }
}

variable "app_version" {
  description = "Application version to deploy"
  type        = string
  default     = "latest"
}

variable "alert_email" {
  description = "Email address for cost and resource alerts"
  type        = string
  default     = "admin@fyphish.com"
}

variable "microsoft_tenant_id" {
  description = "Microsoft Azure AD tenant ID for OAuth"
  type        = string
  default     = "common"
}

variable "allowed_domains" {
  description = "List of allowed email domains for SSO"
  type        = list(string)
  default     = []
}

variable "admin_domains" {
  description = "List of admin email domains"
  type        = list(string)
  default     = ["outlook.com"]
}

variable "enable_public_access" {
  description = "Enable public access to the application"
  type        = bool
  default     = true
}

variable "backup_retention_days" {
  description = "Database backup retention period in days"
  type        = number
  default     = null # Uses environment-specific default
}

variable "enable_monitoring" {
  description = "Enable comprehensive monitoring and alerting"
  type        = bool
  default     = true
}

variable "custom_domain" {
  description = "Custom domain name for the application"
  type        = string
  default     = null
}

variable "ssl_certificate_source" {
  description = "SSL certificate source (managed, byoc)"
  type        = string
  default     = "managed"
  validation {
    condition     = contains(["managed", "byoc"], var.ssl_certificate_source)
    error_message = "SSL certificate source must be either 'managed' or 'byoc' (bring your own certificate)."
  }
}

variable "enable_waf" {
  description = "Enable Web Application Firewall"
  type        = bool
  default     = false
}

variable "enable_ddos_protection" {
  description = "Enable DDoS protection"
  type        = bool
  default     = false
}

variable "database_high_availability" {
  description = "Enable database high availability"
  type        = bool
  default     = false
}

variable "container_app_revisions" {
  description = "Number of container app revisions to keep"
  type        = number
  default     = 10
}

variable "log_retention_days" {
  description = "Log retention period in days"
  type        = number
  default     = 30
}

variable "enable_geo_replication" {
  description = "Enable geo-replication for container registry"
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}