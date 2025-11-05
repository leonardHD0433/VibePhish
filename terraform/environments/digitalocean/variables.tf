# ============================================================================
# FYPhish DigitalOcean Variables
# ============================================================================
# Set actual values in terraform.tfvars (gitignored for security)
# See terraform.tfvars.example for a template
# ============================================================================

# ============================================================================
# DigitalOcean Authentication
# ============================================================================

variable "do_token" {
  description = "DigitalOcean API token - Get from: https://cloud.digitalocean.com/account/api/tokens"
  type        = string
  sensitive   = true
}

# Optional: DigitalOcean Spaces credentials for remote state
variable "spaces_access_key_id" {
  description = "DigitalOcean Spaces access key for remote state (optional)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "spaces_secret_access_key" {
  description = "DigitalOcean Spaces secret key for remote state (optional)"
  type        = string
  default     = ""
  sensitive   = true
}

# ============================================================================
# General Configuration
# ============================================================================

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "fyphish"
}

variable "environment" {
  description = "Environment name (development, staging, production)"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["development", "staging", "production"], var.environment)
    error_message = "Environment must be development, staging, or production"
  }
}

variable "region" {
  description = "DigitalOcean region - sgp1 (Singapore), nyc1/3 (New York), sfo2/3 (San Francisco), lon1 (London), fra1 (Frankfurt)"
  type        = string
  default     = "sgp1"  # Singapore - close to Southeast Asia (your Azure region)

  validation {
    condition     = can(regex("^(nyc[1-3]|sfo[2-3]|ams[2-3]|sgp1|lon1|fra1|tor1|blr1|syd1)$", var.region))
    error_message = "Invalid DigitalOcean region. Common regions: sgp1, nyc1, sfo2, lon1, fra1"
  }
}

# ============================================================================
# Container Registry Configuration
# ============================================================================

variable "registry_name" {
  description = "Name for the Container Registry (must be globally unique, lowercase, alphanumeric + hyphens)"
  type        = string
  default     = "fyphish-registry"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.registry_name))
    error_message = "Registry name must be lowercase alphanumeric with hyphens only"
  }
}

variable "registry_tier" {
  description = "Container Registry tier - starter (free, 500MB), basic ($5/month, 5GB), professional ($20/month, 100GB)"
  type        = string
  default     = "basic"

  validation {
    condition     = contains(["starter", "basic", "professional"], var.registry_tier)
    error_message = "Registry tier must be starter, basic, or professional"
  }
}

variable "registry_region" {
  description = "Container Registry region (should match main region for best performance)"
  type        = string
  default     = "sgp1"
}

# ============================================================================
# App Platform Configuration
# ============================================================================

variable "app_name" {
  description = "App Platform application name"
  type        = string
  default     = "fyphish-app"
}

# FYPhish Application Configuration
variable "fyphish_instance_count" {
  description = "Number of FYPhish app instances (1-10)"
  type        = number
  default     = 1

  validation {
    condition     = var.fyphish_instance_count >= 1 && var.fyphish_instance_count <= 10
    error_message = "Instance count must be between 1 and 10"
  }
}

variable "fyphish_instance_size" {
  description = "FYPhish instance size (2025 pricing) - apps-s-1vcpu-1gb ($12), apps-s-1vcpu-2gb ($25), apps-s-2vcpu-4gb ($50)"
  type        = string
  default     = "apps-s-1vcpu-1gb"  # 1GB RAM, 1 vCPU

  validation {
    condition     = can(regex("^(apps-s-1vcpu-0.5gb|apps-s-1vcpu-1gb-fixed|apps-s-1vcpu-1gb|apps-s-1vcpu-2gb|apps-s-2vcpu-4gb|apps-d-.+)$", var.fyphish_instance_size))
    error_message = "Invalid instance size. Use apps-s-1vcpu-1gb, apps-s-1vcpu-2gb, apps-s-2vcpu-4gb, or apps-d-* for dedicated instances."
  }
}

variable "fyphish_image_repository" {
  description = "Docker image repository name in DOCR"
  type        = string
  default     = "fyphish"
}

variable "fyphish_image_tag" {
  description = "Docker image tag to deploy"
  type        = string
  default     = "latest"
}

# n8n Configuration
variable "n8n_instance_size" {
  description = "n8n main service instance size (2025 pricing) - apps-s-1vcpu-2gb ($25) recommended"
  type        = string
  default     = "apps-s-1vcpu-2gb"  # 2GB RAM, 1 vCPU

  validation {
    condition     = can(regex("^(apps-s-1vcpu-0.5gb|apps-s-1vcpu-1gb-fixed|apps-s-1vcpu-1gb|apps-s-1vcpu-2gb|apps-s-2vcpu-4gb|apps-d-.+)$", var.n8n_instance_size))
    error_message = "Invalid instance size. Use apps-s-1vcpu-1gb, apps-s-1vcpu-2gb, apps-s-2vcpu-4gb, or apps-d-* for dedicated instances."
  }
}

variable "n8n_worker_count" {
  description = "Number of n8n worker instances for queue processing (1-5)"
  type        = number
  default     = 1

  validation {
    condition     = var.n8n_worker_count >= 1 && var.n8n_worker_count <= 5
    error_message = "Worker count must be between 1 and 5"
  }
}

variable "n8n_worker_size" {
  description = "n8n worker instance size (2025 pricing) - apps-s-1vcpu-2gb ($25) recommended"
  type        = string
  default     = "apps-s-1vcpu-2gb"  # 2GB RAM, 1 vCPU

  validation {
    condition     = can(regex("^(apps-s-1vcpu-0.5gb|apps-s-1vcpu-1gb-fixed|apps-s-1vcpu-1gb|apps-s-1vcpu-2gb|apps-s-2vcpu-4gb|apps-d-.+)$", var.n8n_worker_size))
    error_message = "Invalid instance size. Use apps-s-1vcpu-1gb, apps-s-1vcpu-2gb, apps-s-2vcpu-4gb, or apps-d-* for dedicated instances."
  }
}

# ============================================================================
# PostgreSQL Database Configuration
# ============================================================================

variable "postgres_cluster_name" {
  description = "PostgreSQL cluster name"
  type        = string
  default     = "fyphish-postgres"
}

variable "postgres_size" {
  description = "PostgreSQL cluster size - db-s-1vcpu-1gb ($15), db-s-1vcpu-2gb ($30), db-s-2vcpu-4gb ($60)"
  type        = string
  default     = "db-s-1vcpu-1gb"  # Basic tier - suitable for development/testing

  validation {
    condition     = can(regex("^db-s-[0-9]+vcpu-[0-9]+gb$", var.postgres_size))
    error_message = "Invalid PostgreSQL size. Format: db-s-XvcpuYgb (e.g., db-s-1vcpu-1gb)"
  }
}

variable "postgres_user" {
  description = "PostgreSQL username for FYPhish application"
  type        = string
  default     = "fyphish_user"
}

# ============================================================================
# Redis Configuration
# ============================================================================

variable "redis_cluster_name" {
  description = "Redis cluster name"
  type        = string
  default     = "fyphish-redis"
}

variable "redis_size" {
  description = "Redis cluster size - db-s-1vcpu-1gb ($15), db-s-1vcpu-2gb ($30)"
  type        = string
  default     = "db-s-1vcpu-1gb"

  validation {
    condition     = can(regex("^db-s-[0-9]+vcpu-[0-9]+gb$", var.redis_size))
    error_message = "Invalid Redis size. Format: db-s-XvcpuYgb"
  }
}

# ============================================================================
# FYPhish Application Variables
# ============================================================================

variable "admin_email" {
  description = "Admin email for FYPhish (will have admin privileges)"
  type        = string
}

variable "admin_domain" {
  description = "Admin domain for email authentication (e.g., outlook.com)"
  type        = string
  default     = "outlook.com"
}

variable "microsoft_client_id" {
  description = "Microsoft OAuth Client ID from Azure App Registration"
  type        = string
}

variable "microsoft_client_secret" {
  description = "Microsoft OAuth Client Secret from Azure App Registration"
  type        = string
  sensitive   = true
}

variable "microsoft_tenant_id" {
  description = "Microsoft OAuth Tenant ID (or 'common' for multi-tenant)"
  type        = string
}

variable "session_signing_key" {
  description = "Session signing key (generate with: openssl rand -hex 64)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.session_signing_key) >= 64
    error_message = "Session signing key must be at least 64 characters"
  }
}

variable "session_encryption_key" {
  description = "Session encryption key (generate with: openssl rand -hex 32)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.session_encryption_key) >= 32
    error_message = "Session encryption key must be at least 32 characters"
  }
}

variable "go_env" {
  description = "Go environment mode (production or development)"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["production", "development"], var.go_env)
    error_message = "GO_ENV must be production or development"
  }
}

# ============================================================================
# n8n Configuration
# ============================================================================

variable "n8n_encryption_key" {
  description = "n8n encryption key for credentials (generate with: openssl rand -hex 32)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.n8n_encryption_key) >= 32
    error_message = "n8n encryption key must be at least 32 characters"
  }
}

variable "n8n_jwt_secret" {
  description = "JWT secret for authenticating with n8n webhooks (generate with: openssl rand -hex 32)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.n8n_jwt_secret) >= 32
    error_message = "JWT secret must be at least 32 characters"
  }
}

# ============================================================================
# Optional: Custom Domain Configuration
# ============================================================================

variable "custom_domain" {
  description = "Custom domain for FYPhish (optional, requires DNS configuration)"
  type        = string
  default     = ""
}

# ============================================================================
# Optional: Admin IP for Database Access
# ============================================================================

variable "admin_ip_address" {
  description = "Your IP address for database management access (optional)"
  type        = string
  default     = ""
}
