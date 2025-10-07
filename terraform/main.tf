# Resource group
resource "azurerm_resource_group" "rg" {
  name     = "fyphish-rg"
  location = "southeastasia"
}

# Container Registry
resource "azurerm_container_registry" "acr" {
  name                = "fyphishacr"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  sku                 = "Basic"
  admin_enabled       = true
}

# Log Analytics Workspace (for Container Apps)
resource "azurerm_log_analytics_workspace" "logs" {
  name                = "workspace-fyphishrgCCsT"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

# Container Apps Managed Environment
resource "azurerm_container_app_environment" "env" {
  name                       = "fyphish-env"
  resource_group_name        = azurerm_resource_group.rg.name
  location                   = azurerm_resource_group.rg.location
  log_analytics_workspace_id = azurerm_log_analytics_workspace.logs.id
}

# PostgreSQL Flexible Server
resource "azurerm_postgresql_flexible_server" "db" {
  name                = "fyphish-db"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  # Basic configuration
  administrator_login    = var.postgres_admin_login
  administrator_password = var.postgres_admin_password

  sku_name   = "B_Standard_B1ms"
  storage_mb = 32768
  version    = "16"

  # Network and security settings
  backup_retention_days        = 7
  geo_redundant_backup_enabled = false

  lifecycle {
    ignore_changes = [
      zone,  # Azure assigns zone automatically
    ]
  }
}

# PostgreSQL Database - n8n (in shared fyphish-db server)
resource "azurerm_postgresql_flexible_server_database" "n8n" {
  name      = "n8n"
  server_id = azurerm_postgresql_flexible_server.db.id
  charset   = "UTF8"
  collation = "en_US.utf8"
}

# Azure Redis Cache for n8n Queue Management
resource "azurerm_redis_cache" "n8n" {
  name                = "fyphish-n8n-redis"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  sku_name = "Basic"
  family   = "C"
  capacity = 0  # 250MB (smallest size, cost-effective)

  minimum_tls_version = "1.2"
  public_network_access_enabled = true
}

# Container App (FYPhish)
resource "azurerm_container_app" "app" {
  name                         = "fyp-app"
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.env.id
  revision_mode                = "Single"
  workload_profile_name        = "Consumption"

  template {
    min_replicas = 1
    max_replicas = 1

    container {
      name   = "fyp-app"
      image  = "${azurerm_container_registry.acr.login_server}/fyphish:latest"
      cpu    = 2.0
      memory = "4Gi"

      # Match the exact order from Azure
      env {
        name  = "ADMIN_EMAIL"
        value = var.admin_email
      }

      env {
        name  = "MICROSOFT_CLIENT_ID"
        value = var.microsoft_client_id
      }

      env {
        name  = "MICROSOFT_CLIENT_SECRET"
        value = var.microsoft_client_secret
      }

      env {
        name  = "MICROSOFT_TENANT_ID"
        value = var.microsoft_tenant_id
      }

      env {
        name  = "ADMIN_DOMAIN"
        value = var.admin_domain
      }

      env {
        name  = "SESSION_SIGNING_KEY"
        value = var.session_signing_key
      }

      env {
        name  = "SESSION_ENCRYPTION_KEY"
        value = var.session_encryption_key
      }

      env {
        name  = "GO_ENV"
        value = var.go_env
      }

      env {
        name  = "ADMIN_LISTEN_URL"
        value = var.admin_listen_url
      }

      env {
        name  = "ADMIN_TRUSTED_ORIGINS"
        value = var.admin_trusted_origins
      }

      env {
        name  = "POSTGRES_CONNECTION_STRING"
        value = var.postgres_connection_string
      }

      env {
        name  = "ADMIN_BASE_URL"
        value = var.admin_base_url
      }

      env {
        name  = "FORCE_UPDATE"
        value = var.force_update
      }

      env {
        name  = "N8N_WEBHOOK_URL"
        value = var.n8n_webhook_url
      }

      env {
        name  = "N8N_JWT_SECRET"
        value = var.n8n_jwt_secret
      }
    }
  }

  ingress {
    external_enabled = true
    target_port      = 3333

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  registry {
    server               = azurerm_container_registry.acr.login_server
    username             = azurerm_container_registry.acr.admin_username
    password_secret_name = "fyphishacrazurecrio-fyphishacr"
  }

  secret {
    name  = "fyphishacrazurecrio-fyphishacr"
    value = azurerm_container_registry.acr.admin_password
  }
}

# Container App - n8n Main (Web UI)
resource "azurerm_container_app" "n8n" {
  name                         = "n8n-app"
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.env.id
  revision_mode                = "Single"
  workload_profile_name        = "Consumption"

  template {
    min_replicas = 1
    max_replicas = 1

    container {
      name   = "n8n"
      image  = "docker.n8n.io/n8nio/n8n:latest"
      cpu    = 4.0
      memory = "8Gi"

      env {
        name  = "DB_TYPE"
        value = "postgresdb"
      }

      env {
        name  = "DB_POSTGRESDB_HOST"
        value = azurerm_postgresql_flexible_server.db.fqdn
      }

      env {
        name  = "DB_POSTGRESDB_PORT"
        value = "5432"
      }

      env {
        name  = "DB_POSTGRESDB_DATABASE"
        value = "n8n"
      }

      env {
        name  = "DB_POSTGRESDB_USER"
        value = var.postgres_admin_login
      }

      env {
        name  = "DB_POSTGRESDB_PASSWORD"
        value = var.postgres_admin_password
      }

      env {
        name  = "DB_POSTGRESDB_SSL_ENABLED"
        value = "true"
      }

      env {
        name  = "EXECUTIONS_MODE"
        value = "queue"
      }

      env {
        name  = "QUEUE_BULL_REDIS_HOST"
        value = azurerm_redis_cache.n8n.hostname
      }

      env {
        name  = "QUEUE_BULL_REDIS_PORT"
        value = "6380"
      }

      env {
        name  = "QUEUE_BULL_REDIS_PASSWORD"
        value = azurerm_redis_cache.n8n.primary_access_key
      }

      env {
        name  = "QUEUE_BULL_REDIS_TLS"
        value = "true"
      }

      env {
        name  = "QUEUE_HEALTH_CHECK_ACTIVE"
        value = "true"
      }

      env {
        name  = "N8N_ENCRYPTION_KEY"
        value = var.n8n_encryption_key
      }

      env {
        name  = "N8N_HOST"
        value = var.n8n_host
      }

      env {
        name  = "N8N_PROTOCOL"
        value = "https"
      }

      env {
        name  = "WEBHOOK_URL"
        value = var.n8n_webhook_url
      }
    }
  }

  ingress {
    external_enabled = true
    target_port      = 5678

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }
}

# Container App - n8n Worker (Queue Processing)
resource "azurerm_container_app" "n8n_worker" {
  name                         = "n8n-worker"
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.env.id
  revision_mode                = "Single"
  workload_profile_name        = "Consumption"

  template {
    min_replicas = 1
    max_replicas = 3  # Auto-scale based on queue load

    container {
      name    = "n8n-worker"
      image   = "docker.n8n.io/n8nio/n8n:latest"
      command = ["n8n", "worker"]
      cpu     = 4.0
      memory  = "8Gi"

      env {
        name  = "DB_TYPE"
        value = "postgresdb"
      }

      env {
        name  = "DB_POSTGRESDB_HOST"
        value = azurerm_postgresql_flexible_server.db.fqdn
      }

      env {
        name  = "DB_POSTGRESDB_PORT"
        value = "5432"
      }

      env {
        name  = "DB_POSTGRESDB_DATABASE"
        value = "n8n"
      }

      env {
        name  = "DB_POSTGRESDB_USER"
        value = var.postgres_admin_login
      }

      env {
        name  = "DB_POSTGRESDB_PASSWORD"
        value = var.postgres_admin_password
      }

      env {
        name  = "DB_POSTGRESDB_SSL_ENABLED"
        value = "true"
      }

      env {
        name  = "EXECUTIONS_MODE"
        value = "queue"
      }

      env {
        name  = "QUEUE_BULL_REDIS_HOST"
        value = azurerm_redis_cache.n8n.hostname
      }

      env {
        name  = "QUEUE_BULL_REDIS_PORT"
        value = "6380"
      }

      env {
        name  = "QUEUE_BULL_REDIS_PASSWORD"
        value = azurerm_redis_cache.n8n.primary_access_key
      }

      env {
        name  = "QUEUE_BULL_REDIS_TLS"
        value = "true"
      }

      env {
        name  = "N8N_ENCRYPTION_KEY"
        value = var.n8n_encryption_key
      }
    }
  }
}