# FYPhish Infrastructure - Azure Multi-Account Support
# Terraform configuration for deploying FYPhish across multiple Azure accounts
terraform {
  required_version = ">= 1.6"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.70"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 2.40"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.4"
    }
  }
  backend "azurerm" {
    # Backend configuration provided via CLI during terraform init
  }
}

# Configure the Azure Provider
provider "azurerm" {
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
    key_vault {
      purge_soft_delete_on_destroy    = true
      recover_soft_deleted_key_vaults = true
    }
  }
}

# Configure the Azure AD Provider
provider "azuread" {}

# Local values for consistent naming and configuration
locals {
  # Environment-specific configuration
  environment_config = {
    development = {
      sku_name           = "B1"
      replica_count      = 1
      cpu_requests       = "0.25"
      memory_requests    = "0.5Gi"
      cpu_limits         = "0.5"
      memory_limits      = "1Gi"
      database_sku       = "GP_Gen5_1"
      database_storage   = 20
      auto_scale_enabled = false
      backup_retention   = 7
    }
    staging = {
      sku_name           = "B2"
      replica_count      = 2
      cpu_requests       = "0.5"
      memory_requests    = "1Gi"
      cpu_limits         = "1"
      memory_limits      = "2Gi"
      database_sku       = "GP_Gen5_2"
      database_storage   = 50
      auto_scale_enabled = true
      backup_retention   = 14
    }
    production = {
      sku_name           = "S1"
      replica_count      = 3
      cpu_requests       = "1"
      memory_requests    = "2Gi"
      cpu_limits         = "2"
      memory_limits      = "4Gi"
      database_sku       = "GP_Gen5_4"
      database_storage   = 100
      auto_scale_enabled = true
      backup_retention   = 30
    }
  }

  # Account-specific configuration for cost optimization
  account_config = {
    account1 = {
      location            = "East US"
      cost_alerts_enabled = true
      auto_shutdown       = var.environment == "development"
      resource_prefix     = "fyp1"
    }
    account2 = {
      location            = "West US 2"
      cost_alerts_enabled = true
      auto_shutdown       = var.environment == "development"
      resource_prefix     = "fyp2"
    }
    account3 = {
      location            = "Central US"
      cost_alerts_enabled = true
      auto_shutdown       = var.environment == "development"
      resource_prefix     = "fyp3"
    }
  }

  # Current environment and account configuration
  env_config     = local.environment_config[var.environment]
  account        = local.account_config[var.azure_account]
  resource_name  = "${local.account.resource_prefix}-fyphish-${var.environment}"
  common_tags = {
    Project           = "FYPhish"
    Environment       = var.environment
    AzureAccount      = var.azure_account
    ManagedBy         = "Terraform"
    CostCenter        = "FYPhish-${var.environment}"
    AutoShutdown      = local.account.auto_shutdown
    CreatedDate       = formatdate("YYYY-MM-DD", timestamp())
  }
}

# Data sources
data "azurerm_client_config" "current" {}

# Random password for database
resource "random_password" "database_password" {
  length  = 16
  special = true
}

# Random string for unique naming
resource "random_string" "unique_suffix" {
  length  = 6
  special = false
  upper   = false
}

# Resource Group
resource "azurerm_resource_group" "main" {
  name     = "rg-${local.resource_name}-${random_string.unique_suffix.result}"
  location = local.account.location
  tags     = local.common_tags
}

# Log Analytics Workspace for monitoring
resource "azurerm_log_analytics_workspace" "main" {
  name                = "log-${local.resource_name}-${random_string.unique_suffix.result}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = local.common_tags
}

# Application Insights
resource "azurerm_application_insights" "main" {
  name                = "appi-${local.resource_name}-${random_string.unique_suffix.result}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  workspace_id        = azurerm_log_analytics_workspace.main.id
  application_type    = "web"
  tags                = local.common_tags
}

# Key Vault for secrets management
resource "azurerm_key_vault" "main" {
  name                       = "kv-${local.resource_name}-${random_string.unique_suffix.result}"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = "standard"
  soft_delete_retention_days = 7
  purge_protection_enabled   = false

  access_policy {
    tenant_id = data.azurerm_client_config.current.tenant_id
    object_id = data.azurerm_client_config.current.object_id

    secret_permissions = [
      "Get",
      "List",
      "Set",
      "Delete",
      "Recover",
      "Backup",
      "Restore",
      "Purge"
    ]
  }

  tags = local.common_tags
}

# Container Registry
resource "azurerm_container_registry" "main" {
  name                = "acr${replace(local.resource_name, "-", "")}${random_string.unique_suffix.result}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "Basic"
  admin_enabled       = true

  tags = local.common_tags
}

# PostgreSQL Flexible Server
resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "psql-${local.resource_name}-${random_string.unique_suffix.result}"
  resource_group_name    = azurerm_resource_group.main.name
  location               = azurerm_resource_group.main.location
  version                = "15"
  administrator_login    = "fyphish_admin"
  administrator_password = random_password.database_password.result
  backup_retention_days  = local.env_config.backup_retention
  storage_mb             = local.env_config.database_storage * 1024

  sku_name = local.env_config.database_sku

  tags = local.common_tags
}

# PostgreSQL Database
resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = "fyphish"
  server_id = azurerm_postgresql_flexible_server.main.id
  collation = "en_US.utf8"
  charset   = "utf8"
}

# PostgreSQL Firewall Rule for Azure Services
resource "azurerm_postgresql_flexible_server_firewall_rule" "azure_services" {
  name             = "AllowAzureServices"
  server_id        = azurerm_postgresql_flexible_server.main.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

# Container Apps Environment
resource "azurerm_container_app_environment" "main" {
  name                       = "cae-${local.resource_name}-${random_string.unique_suffix.result}"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id

  tags = local.common_tags
}

# Key Vault Secrets
resource "azurerm_key_vault_secret" "database_url" {
  name         = "database-url"
  value        = "postgres://${azurerm_postgresql_flexible_server.main.administrator_login}:${random_password.database_password.result}@${azurerm_postgresql_flexible_server.main.fqdn}:5432/${azurerm_postgresql_flexible_server_database.main.name}?sslmode=require"
  key_vault_id = azurerm_key_vault.main.id
  tags         = local.common_tags
}

resource "azurerm_key_vault_secret" "session_signing_key" {
  name         = "session-signing-key"
  value        = random_string.session_signing_key.result
  key_vault_id = azurerm_key_vault.main.id
  tags         = local.common_tags
}

resource "azurerm_key_vault_secret" "session_encryption_key" {
  name         = "session-encryption-key"
  value        = random_string.session_encryption_key.result
  key_vault_id = azurerm_key_vault.main.id
  tags         = local.common_tags
}

# Generate session keys
resource "random_string" "session_signing_key" {
  length  = 128
  special = false
}

resource "random_string" "session_encryption_key" {
  length  = 64
  special = false
}

# Container App
resource "azurerm_container_app" "main" {
  name                         = "ca-${local.resource_name}-${random_string.unique_suffix.result}"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"

  template {
    min_replicas = local.env_config.replica_count
    max_replicas = local.env_config.auto_scale_enabled ? local.env_config.replica_count * 3 : local.env_config.replica_count

    container {
      name   = "fyphish"
      image  = "mcr.microsoft.com/azuredocs/containerapps-helloworld:latest" # Placeholder - updated by CI/CD
      cpu    = local.env_config.cpu_requests
      memory = local.env_config.memory_requests

      env {
        name  = "GO_ENV"
        value = var.environment
      }

      env {
        name  = "APP_VERSION"
        value = var.app_version
      }

      env {
        name  = "APPLICATIONINSIGHTS_CONNECTION_STRING"
        value = azurerm_application_insights.main.connection_string
      }

      env {
        name        = "DATABASE_URL"
        secret_name = "database-url"
      }

      env {
        name        = "SESSION_SIGNING_KEY"
        secret_name = "session-signing-key"
      }

      env {
        name        = "SESSION_ENCRYPTION_KEY"
        secret_name = "session-encryption-key"
      }

      env {
        name  = "AZURE_KEYVAULT_URL"
        value = azurerm_key_vault.main.vault_uri
      }

      liveness_probe {
        transport = "HTTP"
        port      = 3333
        path      = "/health"
      }

      readiness_probe {
        transport = "HTTP"
        port      = 3333
        path      = "/health/ready"
      }
    }
  }

  secret {
    name  = "database-url"
    value = azurerm_key_vault_secret.database_url.value
  }

  secret {
    name  = "session-signing-key"
    value = azurerm_key_vault_secret.session_signing_key.value
  }

  secret {
    name  = "session-encryption-key"
    value = azurerm_key_vault_secret.session_encryption_key.value
  }

  ingress {
    allow_insecure_connections = false
    external_enabled           = true
    target_port                = 3333

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  identity {
    type = "SystemAssigned"
  }

  tags = local.common_tags
}

# Key Vault Access Policy for Container App
resource "azurerm_key_vault_access_policy" "container_app" {
  key_vault_id = azurerm_key_vault.main.id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = azurerm_container_app.main.identity[0].principal_id

  secret_permissions = [
    "Get",
    "List"
  ]
}

# Cost Management Budget Alert
resource "azurerm_consumption_budget_resource_group" "main" {
  count           = local.account.cost_alerts_enabled ? 1 : 0
  name            = "budget-${local.resource_name}"
  resource_group_id = azurerm_resource_group.main.id

  amount     = 50
  time_grain = "Monthly"

  time_period {
    start_date = formatdate("YYYY-MM-01T00:00:00Z", timestamp())
    end_date   = formatdate("YYYY-MM-01T00:00:00Z", timeadd(timestamp(), "8760h")) # 1 year
  }

  notification {
    enabled   = true
    threshold = 80.0
    operator  = "GreaterThan"

    contact_emails = [
      var.alert_email
    ]
  }

  notification {
    enabled   = true
    threshold = 100.0
    operator  = "GreaterThan"

    contact_emails = [
      var.alert_email
    ]
  }
}

# Auto-shutdown automation for development environment
resource "azurerm_automation_account" "main" {
  count               = local.account.auto_shutdown ? 1 : 0
  name                = "aa-${local.resource_name}-${random_string.unique_suffix.result}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku_name            = "Basic"

  tags = local.common_tags
}

# Auto-shutdown runbook
resource "azurerm_automation_runbook" "shutdown" {
  count                   = local.account.auto_shutdown ? 1 : 0
  name                    = "Shutdown-FYPhish-Dev"
  location                = azurerm_resource_group.main.location
  resource_group_name     = azurerm_resource_group.main.name
  automation_account_name = azurerm_automation_account.main[0].name
  log_verbose             = true
  log_progress            = true
  runbook_type            = "PowerShell"

  content = templatefile("${path.module}/scripts/shutdown-runbook.ps1", {
    resource_group_name  = azurerm_resource_group.main.name
    container_app_name   = azurerm_container_app.main.name
  })

  tags = local.common_tags
}

# Schedule for auto-shutdown (9 PM UTC on weekdays)
resource "azurerm_automation_schedule" "shutdown" {
  count                   = local.account.auto_shutdown ? 1 : 0
  name                    = "Shutdown-Schedule"
  resource_group_name     = azurerm_resource_group.main.name
  automation_account_name = azurerm_automation_account.main[0].name
  frequency               = "Week"
  interval                = 1
  start_time              = formatdate("YYYY-MM-DD'T'21:00:00Z", timestamp())
  timezone                = "UTC"
  week_days               = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
}

# Link runbook to schedule
resource "azurerm_automation_job_schedule" "shutdown" {
  count                   = local.account.auto_shutdown ? 1 : 0
  resource_group_name     = azurerm_resource_group.main.name
  automation_account_name = azurerm_automation_account.main[0].name
  schedule_name           = azurerm_automation_schedule.shutdown[0].name
  runbook_name            = azurerm_automation_runbook.shutdown[0].name
}