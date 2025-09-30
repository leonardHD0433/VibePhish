# Outputs for FYPhish Infrastructure
output "resource_group_name" {
  description = "Name of the resource group"
  value       = azurerm_resource_group.main.name
}

output "resource_group_location" {
  description = "Location of the resource group"
  value       = azurerm_resource_group.main.location
}

output "app_url" {
  description = "URL of the deployed application"
  value       = "https://${azurerm_container_app.main.latest_revision_fqdn}"
}

output "container_app_name" {
  description = "Name of the container app"
  value       = azurerm_container_app.main.name
}

output "container_app_fqdn" {
  description = "FQDN of the container app"
  value       = azurerm_container_app.main.latest_revision_fqdn
}

output "container_registry_name" {
  description = "Name of the container registry"
  value       = azurerm_container_registry.main.name
}

output "container_registry_login_server" {
  description = "Login server of the container registry"
  value       = azurerm_container_registry.main.login_server
}

output "container_registry_admin_username" {
  description = "Admin username for container registry"
  value       = azurerm_container_registry.main.admin_username
  sensitive   = true
}

output "container_registry_admin_password" {
  description = "Admin password for container registry"
  value       = azurerm_container_registry.main.admin_password
  sensitive   = true
}

output "database_server_name" {
  description = "Name of the PostgreSQL server"
  value       = azurerm_postgresql_flexible_server.main.name
}

output "database_server_fqdn" {
  description = "FQDN of the PostgreSQL server"
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "database_name" {
  description = "Name of the database"
  value       = azurerm_postgresql_flexible_server_database.main.name
}

output "database_admin_username" {
  description = "Admin username for PostgreSQL"
  value       = azurerm_postgresql_flexible_server.main.administrator_login
  sensitive   = true
}

output "database_connection_string" {
  description = "Database connection string"
  value       = azurerm_key_vault_secret.database_url.value
  sensitive   = true
}

output "key_vault_name" {
  description = "Name of the Key Vault"
  value       = azurerm_key_vault.main.name
}

output "key_vault_uri" {
  description = "URI of the Key Vault"
  value       = azurerm_key_vault.main.vault_uri
}

output "application_insights_name" {
  description = "Name of Application Insights"
  value       = azurerm_application_insights.main.name
}

output "application_insights_instrumentation_key" {
  description = "Application Insights instrumentation key"
  value       = azurerm_application_insights.main.instrumentation_key
  sensitive   = true
}

output "application_insights_connection_string" {
  description = "Application Insights connection string"
  value       = azurerm_application_insights.main.connection_string
  sensitive   = true
}

output "log_analytics_workspace_id" {
  description = "ID of the Log Analytics workspace"
  value       = azurerm_log_analytics_workspace.main.id
}

output "log_analytics_workspace_name" {
  description = "Name of the Log Analytics workspace"
  value       = azurerm_log_analytics_workspace.main.name
}

output "container_app_environment_id" {
  description = "ID of the Container Apps environment"
  value       = azurerm_container_app_environment.main.id
}

output "container_app_identity_principal_id" {
  description = "Principal ID of the container app's managed identity"
  value       = azurerm_container_app.main.identity[0].principal_id
}

output "oauth_callback_url" {
  description = "OAuth callback URL for Azure AD app registration"
  value       = "https://${azurerm_container_app.main.latest_revision_fqdn}/auth/microsoft/callback"
}

output "health_check_url" {
  description = "Health check endpoint URL"
  value       = "https://${azurerm_container_app.main.latest_revision_fqdn}/health"
}

output "admin_url" {
  description = "Admin interface URL"
  value       = "https://${azurerm_container_app.main.latest_revision_fqdn}/login"
}

output "cost_budget_id" {
  description = "ID of the cost management budget"
  value       = local.account.cost_alerts_enabled ? azurerm_consumption_budget_resource_group.main[0].id : null
}

output "automation_account_name" {
  description = "Name of the automation account (for auto-shutdown)"
  value       = local.account.auto_shutdown ? azurerm_automation_account.main[0].name : null
}

output "deployment_summary" {
  description = "Summary of the deployment configuration"
  value = {
    environment           = var.environment
    azure_account        = var.azure_account
    location             = local.account.location
    app_version          = var.app_version
    resource_prefix      = local.account.resource_prefix
    cost_alerts_enabled  = local.account.cost_alerts_enabled
    auto_shutdown        = local.account.auto_shutdown
    replica_count        = local.env_config.replica_count
    auto_scale_enabled   = local.env_config.auto_scale_enabled
    database_sku         = local.env_config.database_sku
    backup_retention     = local.env_config.backup_retention
  }
}

# Environment-specific outputs for CI/CD pipeline integration
output "environment_config" {
  description = "Environment-specific configuration for CI/CD integration"
  value = {
    oauth_callback_urls = [
      "https://${azurerm_container_app.main.latest_revision_fqdn}/auth/microsoft/callback"
    ]
    health_endpoints = [
      "https://${azurerm_container_app.main.latest_revision_fqdn}/health",
      "https://${azurerm_container_app.main.latest_revision_fqdn}/health/database",
      "https://${azurerm_container_app.main.latest_revision_fqdn}/health/ready"
    ]
    monitoring_endpoints = [
      azurerm_application_insights.main.connection_string
    ]
    deployment_slots = {
      primary = {
        name = azurerm_container_app.main.name
        url  = "https://${azurerm_container_app.main.latest_revision_fqdn}"
      }
    }
  }
}

# Resource information for cost tracking
output "resource_inventory" {
  description = "Inventory of deployed resources for cost tracking"
  value = {
    resource_group = {
      name     = azurerm_resource_group.main.name
      location = azurerm_resource_group.main.location
    }
    container_app = {
      name         = azurerm_container_app.main.name
      replica_min  = local.env_config.replica_count
      replica_max  = local.env_config.auto_scale_enabled ? local.env_config.replica_count * 3 : local.env_config.replica_count
      cpu_requests = local.env_config.cpu_requests
      memory_requests = local.env_config.memory_requests
    }
    database = {
      name           = azurerm_postgresql_flexible_server.main.name
      sku            = local.env_config.database_sku
      storage_mb     = local.env_config.database_storage * 1024
      backup_retention = local.env_config.backup_retention
    }
    container_registry = {
      name = azurerm_container_registry.main.name
      sku  = "Basic"
    }
    key_vault = {
      name = azurerm_key_vault.main.name
      sku  = "standard"
    }
    log_analytics = {
      name              = azurerm_log_analytics_workspace.main.name
      retention_days    = 30
      sku               = "PerGB2018"
    }
    application_insights = {
      name = azurerm_application_insights.main.name
      type = "web"
    }
  }
}