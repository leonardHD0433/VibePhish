# ============================================================================
# FYPhish DigitalOcean Infrastructure
# ============================================================================
# Phase 1: n8n and Databases Only (FYPhish app runs locally)
#
# This configuration currently deploys:
# - App Platform for n8n workflow automation (main + worker)
# - Container Registry for Docker images
# - Managed PostgreSQL database (shared: FYPhish local + n8n)
# - Managed Valkey cache (Redis-compatible, for n8n queue mode)
# - VPC for private networking
# - Project for resource organization
#
# FYPhish app is commented out - running locally with DigitalOcean PostgreSQL
# Uncomment the FYPhish service block when ready to deploy to cloud
# ============================================================================

# DigitalOcean Project (equivalent to Azure Resource Group)
resource "digitalocean_project" "fyphish" {
  name        = "fyphish"
  description = "FYPhish - Security-Enhanced Phishing Simulation Platform"
  purpose     = "Educational Project"
  environment = var.environment

  resources = [
    digitalocean_app.fyphish.urn,
    digitalocean_database_cluster.postgres.urn,
    digitalocean_database_cluster.redis.urn,
  ]
}

# ============================================================================
# Container Registry (equivalent to Azure Container Registry)
# ============================================================================
resource "digitalocean_container_registry" "main" {
  name                   = var.registry_name
  subscription_tier_slug = var.registry_tier  # "basic" = $5/month, "professional" = $20/month
  region                 = var.registry_region # "sgp1" for Singapore
}

# Container Registry Docker Credentials (for App Platform to pull images)
resource "digitalocean_container_registry_docker_credentials" "main" {
  registry_name = digitalocean_container_registry.main.name
  write         = false  # Read-only access for App Platform
}

# ============================================================================
# PostgreSQL Managed Database (equivalent to Azure PostgreSQL Flexible Server)
# ============================================================================
resource "digitalocean_database_cluster" "postgres" {
  name       = var.postgres_cluster_name
  engine     = "pg"
  version    = "16"
  size       = var.postgres_size  # "db-s-1vcpu-1gb" = $15/month
  region     = var.region         # "sgp1" for Singapore
  node_count = 1

  # Security: Enable private networking
  private_network_uuid = digitalocean_vpc.main.id

  # Maintenance window (Sunday 2 AM UTC)
  maintenance_window {
    day  = "sunday"
    hour = "02:00:00"
  }

  tags = ["fyphish", "database", var.environment]
}

# PostgreSQL Database for n8n
resource "digitalocean_database_db" "n8n" {
  cluster_id = digitalocean_database_cluster.postgres.id
  name       = "n8n"
}

# PostgreSQL Database for FYPhish (main application)
resource "digitalocean_database_db" "fyphish" {
  cluster_id = digitalocean_database_cluster.postgres.id
  name       = "fyphish"
}

# PostgreSQL User for FYPhish
resource "digitalocean_database_user" "fyphish" {
  cluster_id = digitalocean_database_cluster.postgres.id
  name       = var.postgres_user
}

# Grant database permissions to FYPhish user
resource "null_resource" "grant_fyphish_permissions" {
  depends_on = [
    digitalocean_database_user.fyphish,
    digitalocean_database_db.fyphish
  ]

  triggers = {
    user_id = digitalocean_database_user.fyphish.id
    db_id   = digitalocean_database_db.fyphish.id
  }

  provisioner "local-exec" {
    command = <<-EOT
      docker run --rm postgres:16 psql \
        "postgresql://${digitalocean_database_cluster.postgres.user}:${digitalocean_database_cluster.postgres.password}@${digitalocean_database_cluster.postgres.host}:${digitalocean_database_cluster.postgres.port}/${digitalocean_database_db.fyphish.name}?sslmode=require" \
        -c "GRANT USAGE ON SCHEMA public TO ${var.postgres_user};" \
        -c "GRANT CREATE ON SCHEMA public TO ${var.postgres_user};" \
        -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${var.postgres_user};" \
        -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${var.postgres_user};" \
        -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${var.postgres_user};" \
        -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${var.postgres_user};"
    EOT
  }
}

# Allow connections from App Platform to PostgreSQL
resource "digitalocean_database_firewall" "postgres" {
  cluster_id = digitalocean_database_cluster.postgres.id

  # Allow access from App Platform (apps in the same region)
  rule {
    type  = "app"
    value = digitalocean_app.fyphish.id
  }

  # Allow entire VPC CIDR range (10.10.0.0/16) - any resource in VPC can connect
  rule {
    type  = "ip_addr"
    value = digitalocean_vpc.main.ip_range
  }

  # Allow from local development machine (when admin_ip_address is set)
  dynamic "rule" {
    for_each = var.admin_ip_address != "" ? [1] : []
    content {
      type  = "ip_addr"
      value = var.admin_ip_address
    }
  }
}

# ============================================================================
# Valkey Managed Database (equivalent to Azure Cache for Redis)
# ============================================================================
# Note: DigitalOcean migrated from Redis to Valkey (Redis-compatible fork)
resource "digitalocean_database_cluster" "redis" {
  name       = var.redis_cluster_name
  engine     = "valkey"  # Redis-compatible (DigitalOcean uses Valkey instead of Redis)
  version    = "8"
  size       = var.redis_size  # "db-s-1vcpu-1gb" = $15/month
  region     = var.region
  node_count = 1

  # Security: Enable private networking
  private_network_uuid = digitalocean_vpc.main.id

  # Eviction policy for cache behavior
  eviction_policy = "allkeys_lru"  # Least Recently Used eviction

  tags = ["fyphish", "cache", var.environment]
}

# Allow connections from App Platform to Valkey (Redis-compatible)
resource "digitalocean_database_firewall" "redis" {
  cluster_id = digitalocean_database_cluster.redis.id

  rule {
    type  = "app"
    value = digitalocean_app.fyphish.id
  }
}

# ============================================================================
# VPC (Virtual Private Cloud) for secure networking
# ============================================================================
resource "digitalocean_vpc" "main" {
  name        = "${var.project_name}-vpc"
  region      = var.region
  description = "Private network for FYPhish infrastructure"
  ip_range    = "10.10.0.0/16"
}

# ============================================================================
# App Platform (equivalent to Azure Container Apps)
# ============================================================================
resource "digitalocean_app" "fyphish" {
  spec {
    name   = var.app_name
    region = var.region

    # Domain configuration (optional)
    # domain {
    #   name = var.custom_domain
    #   type = "PRIMARY"
    # }

    # VPC Configuration - Connect app to private network for database access
    vpc {
      id = digitalocean_vpc.main.id
    }

    # Egress Configuration - Enable outbound traffic to VPC resources
    egress {
      type = "AUTOASSIGN"
    }

    # Ingress Configuration - Route external traffic to n8n service
    ingress {
      rule {
        component {
          name = "n8n"
        }
        match {
          path {
            prefix = "/"
          }
        }
      }
    }

    # ========================================================================
    # FYPhish Main Application Service - COMMENTED OUT FOR LOCAL DEVELOPMENT
    # ========================================================================
    # Uncomment when ready to deploy FYPhish app to DigitalOcean
    # service {
    #   name               = "fyp-app"
    #   instance_count     = var.fyphish_instance_count
    #   instance_size_slug = var.fyphish_instance_size  # "professional-xs" = $12/month

    #   # Docker image from DigitalOcean Container Registry
    #   image {
    #     registry_type = "DOCR"
    #     registry      = digitalocean_container_registry.main.name
    #     repository    = var.fyphish_image_repository
    #     tag           = var.fyphish_image_tag
    #   }

    #   # Health check endpoint
    #   health_check {
    #     http_path             = "/login"
    #     initial_delay_seconds = 30
    #     period_seconds        = 10
    #     timeout_seconds       = 3
    #     success_threshold     = 1
    #     failure_threshold     = 3
    #   }

    #   # HTTP routing
    #   http_port = 3333

    #   routes {
    #     path = "/"
    #   }

    #   # ======================================================================
    #   # Environment Variables for FYPhish
    #   # ======================================================================

    #   # Admin Configuration
    #   env {
    #     key   = "ADMIN_EMAIL"
    #     value = var.admin_email
    #   }

    #   env {
    #     key   = "ADMIN_DOMAIN"
    #     value = var.admin_domain
    #   }

    #   # Microsoft OAuth Configuration
    #   env {
    #     key   = "MICROSOFT_CLIENT_ID"
    #     value = var.microsoft_client_id
    #   }

    #   env {
    #     key   = "MICROSOFT_CLIENT_SECRET"
    #     value = var.microsoft_client_secret
    #     type  = "SECRET"
    #   }

    #   env {
    #     key   = "MICROSOFT_TENANT_ID"
    #     value = var.microsoft_tenant_id
    #   }

    #   # Session Security
    #   env {
    #     key   = "SESSION_SIGNING_KEY"
    #     value = var.session_signing_key
    #     type  = "SECRET"
    #   }

    #   env {
    #     key   = "SESSION_ENCRYPTION_KEY"
    #     value = var.session_encryption_key
    #     type  = "SECRET"
    #   }

    #   # Application Configuration
    #   env {
    #     key   = "GO_ENV"
    #     value = var.go_env
    #   }

    #   env {
    #     key   = "ADMIN_LISTEN_URL"
    #     value = "0.0.0.0:3333"
    #   }

    #   # PostgreSQL Connection (auto-generated from managed database)
    #   env {
    #     key   = "POSTGRES_CONNECTION_STRING"
    #     value = "postgres://${digitalocean_database_user.fyphish.name}:${digitalocean_database_user.fyphish.password}@${digitalocean_database_cluster.postgres.private_host}:${digitalocean_database_cluster.postgres.port}/${digitalocean_database_db.fyphish.name}?sslmode=require"
    #     type  = "SECRET"
    #   }

    #   # n8n Integration
    #   env {
    #     key   = "N8N_WEBHOOK_URL"
    #     value = "https://${var.app_name}-n8n.ondigitalocean.app"
    #   }

    #   env {
    #     key   = "N8N_JWT_SECRET"
    #     value = var.n8n_jwt_secret
    #     type  = "SECRET"
    #   }

    #   # Auto-generated URLs (will be available after first deployment)
    #   env {
    #     key   = "ADMIN_BASE_URL"
    #     value = "https://${var.app_name}.ondigitalocean.app"
    #   }

    #   env {
    #     key   = "ADMIN_TRUSTED_ORIGINS"
    #     value = "https://${var.app_name}.ondigitalocean.app"
    #   }
    # }

    # ========================================================================
    # n8n Main Service (Web UI)
    # ========================================================================
    service {
      name               = "n8n"
      instance_count     = 1
      instance_size_slug = var.n8n_instance_size  # "professional-s" = $24/month

      # Official n8n Docker image from Docker Hub
      # Note: For public images, registry field must be set to the Docker Hub organization
      image {
        registry_type = "DOCKER_HUB"
        registry      = "n8nio"
        repository    = "n8n"
        tag           = "latest"
      }

      # Health check
      health_check {
        http_path             = "/healthz/readiness"
        initial_delay_seconds = 120  # Wait 2 minutes for database connection and VPC networking
        period_seconds        = 10
        timeout_seconds       = 5
        success_threshold     = 1
        failure_threshold     = 3
      }

      http_port = 5678

      # n8n Environment Variables
      env {
        key   = "DB_TYPE"
        value = "postgresdb"
      }

      env {
        key   = "DB_POSTGRESDB_HOST"
        value = digitalocean_database_cluster.postgres.host
      }

      env {
        key   = "DB_POSTGRESDB_PORT"
        value = tostring(digitalocean_database_cluster.postgres.port)
      }

      env {
        key   = "DB_POSTGRESDB_DATABASE"
        value = digitalocean_database_db.n8n.name
      }

      env {
        key   = "DB_POSTGRESDB_USER"
        value = digitalocean_database_cluster.postgres.user
      }

      env {
        key   = "DB_POSTGRESDB_PASSWORD"
        value = digitalocean_database_cluster.postgres.password
        type  = "SECRET"
      }

      env {
        key   = "DB_POSTGRESDB_SSL_REJECT_UNAUTHORIZED"
        value = "false"  # Allow connection to DigitalOcean managed DB without strict cert validation
      }

      env {
        key   = "DB_POSTGRESDB_CONNECTION_TIMEOUT"
        value = "180000"  # 180 seconds (3 minutes) to allow VPC networking to initialize
      }

      # Queue Mode Configuration (using Redis)
      env {
        key   = "EXECUTIONS_MODE"
        value = "queue"
      }

      env {
        key   = "QUEUE_BULL_REDIS_HOST"
        value = digitalocean_database_cluster.redis.host
      }

      env {
        key   = "QUEUE_BULL_REDIS_PORT"
        value = tostring(digitalocean_database_cluster.redis.port)
      }

      env {
        key   = "QUEUE_BULL_REDIS_USERNAME"
        value = digitalocean_database_cluster.redis.user
      }

      env {
        key   = "QUEUE_BULL_REDIS_PASSWORD"
        value = digitalocean_database_cluster.redis.password
        type  = "SECRET"
      }

      env {
        key   = "QUEUE_BULL_REDIS_DB"
        value = "0"
      }

      env {
        key   = "QUEUE_BULL_REDIS_TLS"
        value = "true"
      }

      env {
        key   = "QUEUE_HEALTH_CHECK_ACTIVE"
        value = "true"
      }

      # n8n Encryption and Configuration
      env {
        key   = "N8N_ENCRYPTION_KEY"
        value = var.n8n_encryption_key
        type  = "SECRET"
      }

      # NOTE: WEBHOOK_URL, N8N_HOST, N8N_EDITOR_BASE_URL, N8N_PUSH_BACKEND added via API
      # Terraform cannot reference live_url within the same resource (self-referential block error)
      # These are managed outside Terraform after initial app creation
      # Reference: https://community.n8n.io/t/oauth-redirect-url-is-always-localhost-and-cannot-be-modified/13669

      env {
        key   = "N8N_PROTOCOL"
        value = "https"
      }

      env {
        key   = "N8N_PORT"
        value = "5678"
      }

      env {
        key   = "N8N_METRICS"
        value = "true"
      }
    }

    # ========================================================================
    # n8n Worker Service (Queue Processing - Auto-scaling)
    # ========================================================================
    worker {
      name               = "n8n-worker"
      instance_count     = var.n8n_worker_count  # Can be 1-3 for auto-scaling
      instance_size_slug = var.n8n_worker_size   # "professional-s" = $24/month

      image {
        registry_type = "DOCKER_HUB"
        registry      = "n8nio"
        repository    = "n8n"
        tag           = "latest"
      }

      # Override command to run as worker
      run_command = "n8n worker"

      # Worker environment variables (same database/Redis config as main n8n)
      env {
        key   = "DB_TYPE"
        value = "postgresdb"
      }

      env {
        key   = "DB_POSTGRESDB_HOST"
        value = digitalocean_database_cluster.postgres.host
      }

      env {
        key   = "DB_POSTGRESDB_PORT"
        value = tostring(digitalocean_database_cluster.postgres.port)
      }

      env {
        key   = "DB_POSTGRESDB_DATABASE"
        value = digitalocean_database_db.n8n.name
      }

      env {
        key   = "DB_POSTGRESDB_USER"
        value = digitalocean_database_cluster.postgres.user
      }

      env {
        key   = "DB_POSTGRESDB_PASSWORD"
        value = digitalocean_database_cluster.postgres.password
        type  = "SECRET"
      }

      env {
        key   = "DB_POSTGRESDB_SSL_REJECT_UNAUTHORIZED"
        value = "false"  # Allow connection to DigitalOcean managed DB without strict cert validation
      }

      env {
        key   = "DB_POSTGRESDB_CONNECTION_TIMEOUT"
        value = "180000"  # 180 seconds (3 minutes) to allow VPC networking to initialize
      }

      env {
        key   = "EXECUTIONS_MODE"
        value = "queue"
      }

      env {
        key   = "QUEUE_BULL_REDIS_HOST"
        value = digitalocean_database_cluster.redis.host
      }

      env {
        key   = "QUEUE_BULL_REDIS_PORT"
        value = tostring(digitalocean_database_cluster.redis.port)
      }

      env {
        key   = "QUEUE_BULL_REDIS_USERNAME"
        value = digitalocean_database_cluster.redis.user
      }

      env {
        key   = "QUEUE_BULL_REDIS_PASSWORD"
        value = digitalocean_database_cluster.redis.password
        type  = "SECRET"
      }

      env {
        key   = "QUEUE_BULL_REDIS_DB"
        value = "0"
      }

      env {
        key   = "QUEUE_BULL_REDIS_TLS"
        value = "true"
      }

      env {
        key   = "N8N_ENCRYPTION_KEY"
        value = var.n8n_encryption_key
        type  = "SECRET"
      }

      # NOTE: WEBHOOK_URL, N8N_HOST, N8N_EDITOR_BASE_URL, N8N_PUSH_BACKEND added via API
      # Worker needs same URL configuration as main service

      env {
        key   = "N8N_PROTOCOL"
        value = "https"
      }

      env {
        key   = "N8N_PORT"
        value = "5678"
      }
    }
  }
}
