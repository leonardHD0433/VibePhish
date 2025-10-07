# Variables for FYPhish Terraform configuration
# Set actual values in terraform.tfvars (which is gitignored)

# Azure Service Principal Credentials
variable "arm_client_id" {
  description = "Azure Service Principal Client ID"
  type        = string
}

variable "arm_client_secret" {
  description = "Azure Service Principal Client Secret"
  type        = string
  sensitive   = true
}

variable "arm_subscription_id" {
  description = "Azure Subscription ID"
  type        = string
}

variable "arm_tenant_id" {
  description = "Azure Tenant ID"
  type        = string
}

variable "postgres_admin_login" {
  description = "PostgreSQL administrator login username"
  type        = string
  default     = "fyphish_admin"
}

variable "postgres_admin_password" {
  description = "PostgreSQL administrator password"
  type        = string
  sensitive   = true
}

variable "postgres_connection_string" {
  description = "PostgreSQL connection string for FYPhish"
  type        = string
  sensitive   = true
}

variable "admin_email" {
  description = "Admin email for FYPhish"
  type        = string
}

variable "microsoft_client_id" {
  description = "Microsoft OAuth Client ID"
  type        = string
}

variable "microsoft_client_secret" {
  description = "Microsoft OAuth Client Secret"
  type        = string
  sensitive   = true
}

variable "microsoft_tenant_id" {
  description = "Microsoft OAuth Tenant ID"
  type        = string
}

variable "admin_domain" {
  description = "Admin domain for FYPhish"
  type        = string
}

variable "session_signing_key" {
  description = "Session signing key for FYPhish"
  type        = string
  sensitive   = true
}

variable "session_encryption_key" {
  description = "Session encryption key for FYPhish"
  type        = string
  sensitive   = true
}

variable "go_env" {
  description = "Go environment (production/development)"
  type        = string
  default     = "production"
}

variable "admin_listen_url" {
  description = "Admin listen URL for FYPhish"
  type        = string
  default     = "0.0.0.0:3333"
}

variable "admin_trusted_origins" {
  description = "Trusted origins for CORS"
  type        = string
}

variable "admin_base_url" {
  description = "Admin base URL for FYPhish"
  type        = string
}

variable "force_update" {
  description = "Force update timestamp"
  type        = string
  default     = ""
}

# ============================================================================
# n8n Variables
# ============================================================================

variable "n8n_encryption_key" {
  description = "n8n encryption key for credentials (from local setup)"
  type        = string
  sensitive   = true
}

variable "n8n_host" {
  description = "n8n host URL (will be Azure Container Apps URL)"
  type        = string
}

variable "n8n_webhook_url" {
  description = "n8n webhook URL for external integrations"
  type        = string
}

variable "n8n_jwt_secret" {
  description = "JWT secret for authenticating with n8n webhooks"
  type        = string
  sensitive   = true
}
