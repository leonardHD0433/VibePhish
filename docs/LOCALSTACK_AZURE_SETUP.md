# LocalStack for Azure Setup Guide for FYPhish

## Overview

This guide provides comprehensive setup instructions for using LocalStack for Azure to create a complete local Azure environment that matches production for FYPhish development and testing.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [LocalStack for Azure Installation](#localstack-for-azure-installation)
3. [Service-Specific Setup](#service-specific-setup)
4. [Network Configuration](#network-configuration)
5. [Azure CLI Configuration](#azure-cli-configuration)
6. [FYPhish Integration](#fyphish-integration)
7. [Development Workflow](#development-workflow)
8. [Testing and Debugging](#testing-and-debugging)
9. [Troubleshooting](#troubleshooting)
10. [Performance Optimization](#performance-optimization)

## Prerequisites

### System Requirements
- Docker Desktop 4.0+ with at least 4GB RAM allocated
- Python 3.8+ with pip
- Azure CLI 2.50+
- Go 1.23+ (for FYPhish development)
- Node.js 18+ (for frontend build tools)

### LocalStack Pro License
LocalStack for Azure requires a Pro license. Sign up at [LocalStack Pro](https://localstack.cloud/pricing/).

```bash
# Set your LocalStack API key
export LOCALSTACK_API_KEY="your-api-key-here"
```

## LocalStack for Azure Installation

### 1. Install LocalStack CLI

```bash
# Install LocalStack CLI
pip install localstack[azure]

# Alternative: Use pip3 if default Python is 2.x
pip3 install localstack[azure]

# Verify installation
localstack --version
```

### 2. Install Azure Extensions

```bash
# Install Azure provider extensions
pip install localstack-ext[azure]

# Install additional Azure SDK dependencies
pip install azure-identity azure-keyvault-secrets azure-storage-blob azure-cosmos
```

### 3. Create LocalStack Configuration

Create `docker-compose.localstack.yml`:

```yaml
version: '3.8'

services:
  localstack:
    container_name: localstack-azure
    image: localstack/localstack-pro:latest
    ports:
      - "4566:4566"            # LocalStack Gateway
      - "4510-4559:4510-4559"  # External services port range
      - "4569:4569"            # Edge service (deprecated but may be needed)
    environment:
      # LocalStack Configuration
      - LOCALSTACK_API_KEY=${LOCALSTACK_API_KEY}
      - DEBUG=1
      - PERSISTENCE=1
      - LAMBDA_EXECUTOR=docker-reuse
      - DOCKER_HOST=unix:///var/run/docker.sock
      - HOST_TMP_FOLDER=${TMPDIR:-/tmp/}localstack

      # Azure-specific Configuration
      - PROVIDER=azure
      - AZURE_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
      - AZURE_TENANT_ID=00000000-0000-0000-0000-000000000000
      - AZURE_CLIENT_ID=localstack-client-id
      - AZURE_CLIENT_SECRET=localstack-client-secret

      # Service Configuration
      - SERVICES=keyvault,cosmosdb,storage,containerregistry,appservice,network
      - AZURE_KEYVAULT_URL=https://localhost:4566
      - AZURE_STORAGE_ACCOUNT=fyphishdev
      - AZURE_COSMOSDB_ACCOUNT=fyphish-cosmos-dev

      # Network Configuration
      - AZURE_VNET_NAME=fyphish-vnet
      - AZURE_SUBNET_NAME=fyphish-subnet
      - AZURE_NSG_NAME=fyphish-nsg

      # Development Settings
      - SKIP_SSL_CERT_DOWNLOAD=1
      - DISABLE_CUSTOM_CORE_LISTENER=1
      - EAGER_SERVICE_LOADING=1

    volumes:
      - "${LOCALSTACK_VOLUME_DIR:-./volume}:/var/lib/localstack"
      - "/var/run/docker.sock:/var/run/docker.sock"
    networks:
      - fyphish-network

networks:
  fyphish-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

### 4. Environment Configuration

Create `.env.localstack`:

```bash
# LocalStack Configuration
LOCALSTACK_API_KEY=your-api-key-here
LOCALSTACK_VOLUME_DIR=./localstack-volume

# Azure Mock Configuration
AZURE_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
AZURE_TENANT_ID=00000000-0000-0000-0000-000000000000
AZURE_RESOURCE_GROUP=fyphish-dev-rg
AZURE_LOCATION=eastus

# Service Names (used in scripts)
AZURE_KEYVAULT_NAME=fyphish-kv-dev
AZURE_STORAGE_ACCOUNT=fyphishdev
AZURE_COSMOSDB_ACCOUNT=fyphish-cosmos-dev
AZURE_CONTAINER_REGISTRY=fyphishacr
AZURE_APP_SERVICE_PLAN=fyphish-asp-dev
AZURE_APP_SERVICE=fyphish-app-dev

# Network Configuration
AZURE_VNET_NAME=fyphish-vnet
AZURE_SUBNET_NAME=fyphish-subnet
AZURE_NSG_NAME=fyphish-nsg
AZURE_NSG_RULE_SSH=ssh-rule
AZURE_NSG_RULE_HTTP=http-rule
AZURE_NSG_RULE_HTTPS=https-rule

# Development URLs
FYPHISH_LOCAL_URL=http://localhost:3333
FYPHISH_CALLBACK_URL=http://localhost:3333/auth/microsoft/callback
```

### 5. Start LocalStack

```bash
# Source environment variables
source .env.localstack

# Start LocalStack in background
docker-compose -f docker-compose.localstack.yml up -d

# Check status
localstack status services

# View logs
docker-compose -f docker-compose.localstack.yml logs -f localstack
```

## Service-Specific Setup

### 1. Azure Key Vault Configuration

Create setup script `scripts/setup-keyvault.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

# Set LocalStack endpoint
export AZURE_KEYVAULT_URL="https://localhost:4566"

echo "Setting up Azure Key Vault for FYPhish..."

# Create resource group
az group create \
  --name $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --endpoint-url http://localhost:4566

# Create Key Vault
az keyvault create \
  --name $AZURE_KEYVAULT_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --endpoint-url http://localhost:4566

# Set Key Vault secrets for FYPhish
echo "Setting up FYPhish secrets..."

# OAuth Configuration
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "microsoft-client-id" \
  --value "localstack-oauth-client-id" \
  --endpoint-url http://localhost:4566

az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "microsoft-client-secret" \
  --value "localstack-oauth-client-secret" \
  --endpoint-url http://localhost:4566

az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "microsoft-tenant-id" \
  --value "common" \
  --endpoint-url http://localhost:4566

# Session Security Keys
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "session-signing-key" \
  --value "$(openssl rand -hex 64)" \
  --endpoint-url http://localhost:4566

az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "session-encryption-key" \
  --value "$(openssl rand -hex 32)" \
  --endpoint-url http://localhost:4566

# Database Connection String
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "database-connection-string" \
  --value "host=localhost port=5432 user=fyphish password=localdev dbname=fyphish sslmode=disable" \
  --endpoint-url http://localhost:4566

# Admin Configuration
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "admin-email" \
  --value "admin@fyphish.local" \
  --endpoint-url http://localhost:4566

echo "Key Vault setup completed!"
echo "Vault URL: https://localhost:4566/subscriptions/$AZURE_SUBSCRIPTION_ID/resourceGroups/$AZURE_RESOURCE_GROUP/providers/Microsoft.KeyVault/vaults/$AZURE_KEYVAULT_NAME"
```

### 2. Database Setup (PostgreSQL via CosmosDB)

Create setup script `scripts/setup-database.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

echo "Setting up CosmosDB for FYPhish database..."

# Create CosmosDB account
az cosmosdb create \
  --name $AZURE_COSMOSDB_ACCOUNT \
  --resource-group $AZURE_RESOURCE_GROUP \
  --kind GlobalDocumentDB \
  --default-consistency-level Session \
  --locations regionName=$AZURE_LOCATION failoverPriority=0 isZoneRedundant=False \
  --endpoint-url http://localhost:4566

# Create database
az cosmosdb sql database create \
  --account-name $AZURE_COSMOSDB_ACCOUNT \
  --resource-group $AZURE_RESOURCE_GROUP \
  --name fyphish \
  --endpoint-url http://localhost:4566

# Create containers for FYPhish tables
containers=("users" "campaigns" "email_requests" "events" "groups" "landing_pages" "smtp" "templates" "webhooks" "authorized_emails")

for container in "${containers[@]}"; do
  echo "Creating container: $container"
  az cosmosdb sql container create \
    --account-name $AZURE_COSMOSDB_ACCOUNT \
    --database-name fyphish \
    --resource-group $AZURE_RESOURCE_GROUP \
    --name $container \
    --partition-key-path "/id" \
    --throughput 400 \
    --endpoint-url http://localhost:4566
done

echo "Database setup completed!"
```

### 3. Container Registry Setup

Create setup script `scripts/setup-registry.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

echo "Setting up Azure Container Registry..."

# Create Container Registry
az acr create \
  --name $AZURE_CONTAINER_REGISTRY \
  --resource-group $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --sku Basic \
  --admin-enabled true \
  --endpoint-url http://localhost:4566

# Get admin credentials
ACR_USERNAME=$(az acr credential show --name $AZURE_CONTAINER_REGISTRY --resource-group $AZURE_RESOURCE_GROUP --query username --output tsv --endpoint-url http://localhost:4566)
ACR_PASSWORD=$(az acr credential show --name $AZURE_CONTAINER_REGISTRY --resource-group $AZURE_RESOURCE_GROUP --query "passwords[0].value" --output tsv --endpoint-url http://localhost:4566)

echo "Container Registry setup completed!"
echo "Registry: $AZURE_CONTAINER_REGISTRY.azurecr.io"
echo "Username: $ACR_USERNAME"
echo "Password: $ACR_PASSWORD"

# Store credentials in Key Vault
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "acr-username" \
  --value "$ACR_USERNAME" \
  --endpoint-url http://localhost:4566

az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "acr-password" \
  --value "$ACR_PASSWORD" \
  --endpoint-url http://localhost:4566
```

### 4. App Service Setup

Create setup script `scripts/setup-appservice.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

echo "Setting up Azure App Service..."

# Create App Service Plan
az appservice plan create \
  --name $AZURE_APP_SERVICE_PLAN \
  --resource-group $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --sku B1 \
  --is-linux \
  --endpoint-url http://localhost:4566

# Create Web App
az webapp create \
  --name $AZURE_APP_SERVICE \
  --resource-group $AZURE_RESOURCE_GROUP \
  --plan $AZURE_APP_SERVICE_PLAN \
  --runtime "GO|1.20" \
  --endpoint-url http://localhost:4566

# Configure app settings
az webapp config appsettings set \
  --name $AZURE_APP_SERVICE \
  --resource-group $AZURE_RESOURCE_GROUP \
  --settings \
    GO_ENV=development \
    AZURE_KEYVAULT_URL="https://localhost:4566" \
    AZURE_KEYVAULT_NAME=$AZURE_KEYVAULT_NAME \
    PORT=8080 \
  --endpoint-url http://localhost:4566

# Set up health check endpoint
az webapp config set \
  --name $AZURE_APP_SERVICE \
  --resource-group $AZURE_RESOURCE_GROUP \
  --health-check-path "/health" \
  --endpoint-url http://localhost:4566

echo "App Service setup completed!"
echo "App URL: https://$AZURE_APP_SERVICE.azurewebsites.net"
```

## Network Configuration

Create setup script `scripts/setup-network.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

echo "Setting up Azure Network configuration..."

# Create Virtual Network
az network vnet create \
  --name $AZURE_VNET_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --address-prefix 10.0.0.0/16 \
  --endpoint-url http://localhost:4566

# Create Subnet
az network vnet subnet create \
  --name $AZURE_SUBNET_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --vnet-name $AZURE_VNET_NAME \
  --address-prefix 10.0.1.0/24 \
  --endpoint-url http://localhost:4566

# Create Network Security Group
az network nsg create \
  --name $AZURE_NSG_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --location $AZURE_LOCATION \
  --endpoint-url http://localhost:4566

# Create security rules
az network nsg rule create \
  --name $AZURE_NSG_RULE_SSH \
  --nsg-name $AZURE_NSG_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --priority 1001 \
  --source-address-prefixes '*' \
  --source-port-ranges '*' \
  --destination-address-prefixes '*' \
  --destination-port-ranges 22 \
  --access Allow \
  --protocol Tcp \
  --description "Allow SSH" \
  --endpoint-url http://localhost:4566

az network nsg rule create \
  --name $AZURE_NSG_RULE_HTTP \
  --nsg-name $AZURE_NSG_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --priority 1002 \
  --source-address-prefixes '*' \
  --source-port-ranges '*' \
  --destination-address-prefixes '*' \
  --destination-port-ranges 80 \
  --access Allow \
  --protocol Tcp \
  --description "Allow HTTP" \
  --endpoint-url http://localhost:4566

az network nsg rule create \
  --name $AZURE_NSG_RULE_HTTPS \
  --nsg-name $AZURE_NSG_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --priority 1003 \
  --source-address-prefixes '*' \
  --source-port-ranges '*' \
  --destination-address-prefixes '*' \
  --destination-port-ranges 443 \
  --access Allow \
  --protocol Tcp \
  --description "Allow HTTPS" \
  --endpoint-url http://localhost:4566

az network nsg rule create \
  --name "allow-fyphish" \
  --nsg-name $AZURE_NSG_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --priority 1004 \
  --source-address-prefixes '*' \
  --source-port-ranges '*' \
  --destination-address-prefixes '*' \
  --destination-port-ranges 3333 \
  --access Allow \
  --protocol Tcp \
  --description "Allow FYPhish port 3333" \
  --endpoint-url http://localhost:4566

# Associate NSG with subnet
az network vnet subnet update \
  --name $AZURE_SUBNET_NAME \
  --resource-group $AZURE_RESOURCE_GROUP \
  --vnet-name $AZURE_VNET_NAME \
  --network-security-group $AZURE_NSG_NAME \
  --endpoint-url http://localhost:4566

echo "Network configuration completed!"
```

## Azure CLI Configuration

### 1. Configure Azure CLI for LocalStack

Create configuration script `scripts/configure-azure-cli.sh`:

```bash
#!/bin/bash
set -e

source .env.localstack

echo "Configuring Azure CLI for LocalStack..."

# Add LocalStack as a cloud
az cloud register \
  --name LocalStackAzure \
  --endpoint-resource-manager "http://localhost:4566" \
  --endpoint-sql-management "http://localhost:4566" \
  --endpoint-gallery "http://localhost:4566" \
  --endpoint-active-directory "http://localhost:4566" \
  --endpoint-active-directory-resource-id "http://localhost:4566"

# Set LocalStack as active cloud
az cloud set --name LocalStackAzure

# Login to LocalStack (will use mock authentication)
az login \
  --service-principal \
  --username $AZURE_CLIENT_ID \
  --password $AZURE_CLIENT_SECRET \
  --tenant $AZURE_TENANT_ID

# Set default subscription
az account set --subscription $AZURE_SUBSCRIPTION_ID

echo "Azure CLI configured for LocalStack!"
echo "Active cloud: $(az cloud show --query name -o tsv)"
echo "Active subscription: $(az account show --query name -o tsv)"
```

### 2. Create Helper Aliases

Add to your `.bashrc` or `.zshrc`:

```bash
# LocalStack Azure aliases
alias az-local="az --endpoint-url http://localhost:4566"
alias localstack-status="localstack status services"
alias localstack-logs="docker-compose -f docker-compose.localstack.yml logs -f localstack"
alias localstack-restart="docker-compose -f docker-compose.localstack.yml restart localstack"

# FYPhish development aliases
alias fyphish-build="go build -o bin/fyphish ./gophish.go"
alias fyphish-run="./bin/fyphish"
alias fyphish-test="go test ./..."
alias fyphish-clean="rm -rf bin/ && go clean -cache"
```

### 3. Environment Switching Script

Create `scripts/switch-environment.sh`:

```bash
#!/bin/bash

ENVIRONMENT=${1:-local}

case $ENVIRONMENT in
  "local")
    echo "Switching to LocalStack environment..."
    az cloud set --name LocalStackAzure
    export AZURE_KEYVAULT_URL="https://localhost:4566"
    export AZURE_ENDPOINT_URL="http://localhost:4566"
    source .env.localstack
    echo "✅ LocalStack environment active"
    ;;
  "azure")
    echo "Switching to Azure production environment..."
    az cloud set --name AzureCloud
    unset AZURE_ENDPOINT_URL
    source .env.production
    echo "✅ Azure production environment active"
    ;;
  *)
    echo "Usage: $0 [local|azure]"
    echo "Current cloud: $(az cloud show --query name -o tsv)"
    exit 1
    ;;
esac
```

## FYPhish Integration

### 1. Azure Key Vault Integration

Create `pkg/azure/keyvault.go`:

```go
package azure

import (
    "context"
    "fmt"
    "os"

    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
    "github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

type KeyVaultClient struct {
    client   *azsecrets.Client
    vaultURL string
}

func NewKeyVaultClient() (*KeyVaultClient, error) {
    vaultURL := os.Getenv("AZURE_KEYVAULT_URL")
    if vaultURL == "" {
        vaultURL = fmt.Sprintf("https://%s.vault.azure.net/", os.Getenv("AZURE_KEYVAULT_NAME"))
    }

    // Use environment credentials for LocalStack
    var cred azidentity.ChainedTokenCredential
    if os.Getenv("AZURE_ENDPOINT_URL") != "" {
        // LocalStack environment - use environment credentials
        envCred, err := azidentity.NewEnvironmentCredential(nil)
        if err != nil {
            return nil, fmt.Errorf("failed to create environment credential: %w", err)
        }
        cred = azidentity.ChainedTokenCredential{Sources: []azidentity.TokenCredential{envCred}}
    } else {
        // Production environment - use managed identity
        managedCred, err := azidentity.NewDefaultAzureCredential(nil)
        if err != nil {
            return nil, fmt.Errorf("failed to create managed identity credential: %w", err)
        }
        cred = azidentity.ChainedTokenCredential{Sources: []azidentity.TokenCredential{managedCred}}
    }

    client, err := azsecrets.NewClient(vaultURL, &cred, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
    }

    return &KeyVaultClient{
        client:   client,
        vaultURL: vaultURL,
    }, nil
}

func (kv *KeyVaultClient) GetSecret(ctx context.Context, name string) (string, error) {
    resp, err := kv.client.GetSecret(ctx, name, "", nil)
    if err != nil {
        return "", fmt.Errorf("failed to get secret %s: %w", name, err)
    }

    return *resp.Value, nil
}

func (kv *KeyVaultClient) SetSecret(ctx context.Context, name, value string) error {
    _, err := kv.client.SetSecret(ctx, name, azsecrets.SetSecretParameters{Value: &value}, nil)
    if err != nil {
        return fmt.Errorf("failed to set secret %s: %w", name, err)
    }

    return nil
}
```

### 2. Enhanced Configuration Loading

Update `config/sso_config.go` to add Azure Key Vault support:

```go
// Add to existing sso_config.go

import (
    "context"
    "time"

    "github.com/leonardHD0433/FYPhish/pkg/azure"
)

// LoadSecretsFromKeyVault loads OAuth secrets from Azure Key Vault
func (c *Config) LoadSecretsFromKeyVault() error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    kv, err := azure.NewKeyVaultClient()
    if err != nil {
        log.Warn("Key Vault not available, falling back to environment variables: ", err)
        return c.LoadSecretsFromEnv()
    }

    if c.SSO == nil || c.SSO.Providers == nil {
        return nil
    }

    // Load Microsoft OAuth secrets from Key Vault
    if ms := c.SSO.Providers["microsoft"]; ms != nil {
        if clientID, err := kv.GetSecret(ctx, "microsoft-client-id"); err == nil {
            ms.ClientID = clientID
        }
        if clientSecret, err := kv.GetSecret(ctx, "microsoft-client-secret"); err == nil {
            ms.ClientSecret = clientSecret
        }
        if tenantID, err := kv.GetSecret(ctx, "microsoft-tenant-id"); err == nil {
            ms.TenantID = tenantID
        }
    }

    return nil
}

// LoadSecretsFromEnvOrKeyVault tries Key Vault first, then falls back to environment
func (c *Config) LoadSecretsFromEnvOrKeyVault() {
    if err := c.LoadSecretsFromKeyVault(); err != nil {
        log.Info("Loading secrets from environment variables...")
        c.LoadSecretsFromEnv()
    } else {
        log.Info("Secrets loaded from Azure Key Vault")
    }
}
```

### 3. Session Key Management

Create `pkg/azure/session.go`:

```go
package azure

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "time"
)

type SessionKeyManager struct {
    kv *KeyVaultClient
}

func NewSessionKeyManager() (*SessionKeyManager, error) {
    kv, err := NewKeyVaultClient()
    if err != nil {
        return nil, err
    }

    return &SessionKeyManager{kv: kv}, nil
}

func (s *SessionKeyManager) GetOrCreateSessionKeys() (signing, encryption string, err error) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Try to get existing keys from Key Vault
    signing, err = s.kv.GetSecret(ctx, "session-signing-key")
    if err != nil {
        // Generate new signing key
        signing = generateSecureKey(64) // 128 hex chars
        if setErr := s.kv.SetSecret(ctx, "session-signing-key", signing); setErr != nil {
            return "", "", fmt.Errorf("failed to store signing key: %w", setErr)
        }
    }

    encryption, err = s.kv.GetSecret(ctx, "session-encryption-key")
    if err != nil {
        // Generate new encryption key
        encryption = generateSecureKey(32) // 64 hex chars
        if setErr := s.kv.SetSecret(ctx, "session-encryption-key", encryption); setErr != nil {
            return "", "", fmt.Errorf("failed to store encryption key: %w", setErr)
        }
    }

    return signing, encryption, nil
}

func generateSecureKey(bytes int) string {
    key := make([]byte, bytes)
    rand.Read(key)
    return hex.EncodeToString(key)
}

// GetSessionKeysForEnvironment returns session keys based on environment
func GetSessionKeysForEnvironment() (signing, encryption string) {
    // Try Key Vault first if available
    if manager, err := NewSessionKeyManager(); err == nil {
        if s, e, err := manager.GetOrCreateSessionKeys(); err == nil {
            return s, e
        }
    }

    // Fall back to environment variables
    signing = os.Getenv("SESSION_SIGNING_KEY")
    encryption = os.Getenv("SESSION_ENCRYPTION_KEY")

    // Use development defaults if nothing is configured
    if signing == "" {
        signing = "development-signing-key-not-for-production-use-only-local-dev"
    }
    if encryption == "" {
        encryption = "dev-encryption-key-not-for-production"
    }

    return signing, encryption
}
```

### 4. Health Check Endpoint

Create `pkg/health/health.go`:

```go
package health

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/leonardHD0433/FYPhish/pkg/azure"
    "github.com/leonardHD0433/FYPhish/models"
)

type HealthStatus struct {
    Status     string            `json:"status"`
    Timestamp  time.Time         `json:"timestamp"`
    Services   map[string]string `json:"services"`
    Version    string            `json:"version"`
    Uptime     time.Duration     `json:"uptime"`
}

var startTime = time.Now()

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    status := HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Services:  make(map[string]string),
        Version:   "1.0.0", // Get from build
        Uptime:    time.Since(startTime),
    }

    // Check database connectivity
    if err := models.CheckDatabaseHealth(ctx); err != nil {
        status.Services["database"] = "unhealthy: " + err.Error()
        status.Status = "degraded"
    } else {
        status.Services["database"] = "healthy"
    }

    // Check Key Vault connectivity
    if kv, err := azure.NewKeyVaultClient(); err != nil {
        status.Services["keyvault"] = "unhealthy: " + err.Error()
        status.Status = "degraded"
    } else {
        if _, err := kv.GetSecret(ctx, "microsoft-client-id"); err != nil {
            status.Services["keyvault"] = "unhealthy: " + err.Error()
            status.Status = "degraded"
        } else {
            status.Services["keyvault"] = "healthy"
        }
    }

    // Set HTTP status based on overall health
    httpStatus := http.StatusOK
    if status.Status == "degraded" {
        httpStatus = http.StatusServiceUnavailable
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(httpStatus)
    json.NewEncoder(w).Encode(status)
}

func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func LivenessHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
```

### 5. OAuth Callback URL Configuration

Update your OAuth configuration for LocalStack:

```go
// In auth/oauth_handlers.go, add environment-aware callback URL generation

func getCallbackURL() string {
    if os.Getenv("AZURE_ENDPOINT_URL") != "" {
        // LocalStack environment
        return "http://localhost:3333/auth/microsoft/callback"
    }

    // Production environment
    baseURL := os.Getenv("FYPHISH_BASE_URL")
    if baseURL == "" {
        baseURL = "https://your-app.azurewebsites.net"
    }
    return baseURL + "/auth/microsoft/callback"
}
```

## Development Workflow

### 1. Complete Setup Script

Create `scripts/setup-localstack-complete.sh`:

```bash
#!/bin/bash
set -e

echo "🚀 Setting up complete LocalStack Azure environment for FYPhish..."

# Load environment
source .env.localstack

# Start LocalStack
echo "📦 Starting LocalStack..."
docker-compose -f docker-compose.localstack.yml up -d

# Wait for LocalStack to be ready
echo "⏳ Waiting for LocalStack to be ready..."
sleep 30

# Configure Azure CLI
echo "🔧 Configuring Azure CLI..."
./scripts/configure-azure-cli.sh

# Set up all services
echo "🗄️  Setting up Key Vault..."
./scripts/setup-keyvault.sh

echo "💾 Setting up Database..."
./scripts/setup-database.sh

echo "📦 Setting up Container Registry..."
./scripts/setup-registry.sh

echo "🌐 Setting up Network..."
./scripts/setup-network.sh

echo "🚀 Setting up App Service..."
./scripts/setup-appservice.sh

# Build and test FYPhish
echo "🔨 Building FYPhish..."
go mod tidy
go build -o bin/fyphish ./gophish.go

echo "🧪 Running tests..."
go test ./...

echo "✅ LocalStack Azure environment setup complete!"
echo ""
echo "🎯 Next steps:"
echo "1. Start FYPhish: ./bin/fyphish"
echo "2. Access at: http://localhost:3333"
echo "3. Login with: admin@fyphish.local"
echo "4. Check health: curl http://localhost:3333/health"
echo ""
echo "📊 Management commands:"
echo "- Status: localstack status services"
echo "- Logs: localstack-logs"
echo "- Restart: localstack-restart"
```

### 2. Development Environment File

Create `.env.development`:

```bash
# Development environment for FYPhish with LocalStack
GO_ENV=development

# LocalStack Azure configuration
AZURE_ENDPOINT_URL=http://localhost:4566
AZURE_KEYVAULT_URL=https://localhost:4566
AZURE_KEYVAULT_NAME=fyphish-kv-dev

# Database
DB_TYPE=postgres
DATABASE_URL=postgres://fyphish:localdev@localhost:5432/fyphish?sslmode=disable

# FYPhish Configuration
ADMIN_EMAIL=admin@fyphish.local
ALLOWED_DOMAIN=fyphish.local
ADMIN_DOMAIN=fyphish.local

# OAuth Configuration (loaded from Key Vault)
MICROSOFT_TENANT_ID=common

# Security (will be loaded from Key Vault)
HIDE_LOCAL_LOGIN=false
EMERGENCY_ACCESS=true

# Server Configuration
LISTEN_URL=127.0.0.1:3333
USE_TLS=false

# Logging
LOG_LEVEL=debug
LOG_FORMAT=json
```

### 3. Makefile for Development

Create `Makefile`:

```makefile
.PHONY: localstack-start localstack-stop localstack-setup build test run clean health

# Default target
all: localstack-setup build test

# LocalStack management
localstack-start:
	@echo "Starting LocalStack..."
	docker-compose -f docker-compose.localstack.yml up -d
	@echo "Waiting for LocalStack to be ready..."
	sleep 30

localstack-stop:
	@echo "Stopping LocalStack..."
	docker-compose -f docker-compose.localstack.yml down

localstack-restart: localstack-stop localstack-start

localstack-setup: localstack-start
	@echo "Setting up LocalStack services..."
	./scripts/setup-localstack-complete.sh

localstack-status:
	@localstack status services

# Build and test
build:
	@echo "Building FYPhish..."
	go mod tidy
	go build -o bin/fyphish ./gophish.go

test:
	@echo "Running tests..."
	go test -v ./...

test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./...

# Run application
run: build
	@echo "Starting FYPhish..."
	source .env.development && ./bin/fyphish

run-production:
	@echo "Starting FYPhish in production mode..."
	source .env.production && ./bin/fyphish

# Health checks
health:
	@curl -s http://localhost:3333/health | jq .

readiness:
	@curl -s http://localhost:3333/ready | jq .

liveness:
	@curl -s http://localhost:3333/live | jq .

# Cleanup
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	go clean -cache
	docker-compose -f docker-compose.localstack.yml down -v

# Development workflow
dev-setup: localstack-setup build
	@echo "Development environment ready!"
	@echo "Run 'make run' to start FYPhish"

dev-reset: clean localstack-setup build
	@echo "Development environment reset complete!"

# Debugging
logs:
	docker-compose -f docker-compose.localstack.yml logs -f localstack

shell:
	docker exec -it localstack-azure bash

# Azure CLI shortcuts
az-local:
	az --endpoint-url http://localhost:4566 $(filter-out $@,$(MAKECMDGOALS))

# Container operations
docker-build:
	docker build -t fyphish:latest .

docker-run:
	docker run -p 3333:3333 --env-file .env.development fyphish:latest

docker-push:
	docker tag fyphish:latest $(AZURE_CONTAINER_REGISTRY).azurecr.io/fyphish:latest
	docker push $(AZURE_CONTAINER_REGISTRY).azurecr.io/fyphish:latest
```

## Testing and Debugging

### 1. Integration Test Suite

Create `tests/integration/localstack_test.go`:

```go
//go:build integration
// +build integration

package integration

import (
    "context"
    "net/http"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/leonardHD0433/FYPhish/pkg/azure"
    "github.com/leonardHD0433/FYPhish/config"
)

func TestLocalStackIntegration(t *testing.T) {
    // Ensure LocalStack is running
    require.True(t, isLocalStackRunning(), "LocalStack must be running for integration tests")

    t.Run("KeyVaultClient", testKeyVaultClient)
    t.Run("ConfigLoading", testConfigLoading)
    t.Run("HealthEndpoints", testHealthEndpoints)
    t.Run("OAuthCallback", testOAuthCallback)
}

func testKeyVaultClient(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    kv, err := azure.NewKeyVaultClient()
    require.NoError(t, err, "Should create Key Vault client")

    // Test setting and getting a secret
    testSecret := "test-secret-value"
    err = kv.SetSecret(ctx, "test-secret", testSecret)
    require.NoError(t, err, "Should set secret")

    retrievedSecret, err := kv.GetSecret(ctx, "test-secret")
    require.NoError(t, err, "Should get secret")
    assert.Equal(t, testSecret, retrievedSecret, "Secret values should match")
}

func testConfigLoading(t *testing.T) {
    cfg, err := config.LoadConfigWithSSO("../../config.json")
    require.NoError(t, err, "Should load config")

    cfg.LoadSecretsFromEnvOrKeyVault()

    assert.True(t, cfg.IsSSOEnabled(), "SSO should be enabled")
    assert.True(t, cfg.IsProviderEnabled("microsoft"), "Microsoft provider should be enabled")
}

func testHealthEndpoints(t *testing.T) {
    baseURL := "http://localhost:3333"

    endpoints := []string{"/health", "/ready", "/live"}

    for _, endpoint := range endpoints {
        t.Run(endpoint, func(t *testing.T) {
            resp, err := http.Get(baseURL + endpoint)
            require.NoError(t, err, "Should make HTTP request")
            defer resp.Body.Close()

            assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200")
            assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Should return JSON")
        })
    }
}

func testOAuthCallback(t *testing.T) {
    // Test OAuth callback URL configuration
    callbackURL := getCallbackURL()

    if isLocalStackEnvironment() {
        assert.Equal(t, "http://localhost:3333/auth/microsoft/callback", callbackURL)
    } else {
        assert.Contains(t, callbackURL, "/auth/microsoft/callback")
    }
}

func isLocalStackRunning() bool {
    resp, err := http.Get("http://localhost:4566/_localstack/health")
    return err == nil && resp.StatusCode == 200
}

func isLocalStackEnvironment() bool {
    return os.Getenv("AZURE_ENDPOINT_URL") == "http://localhost:4566"
}
```

### 2. Debugging Scripts

Create `scripts/debug-localstack.sh`:

```bash
#!/bin/bash

echo "🔍 LocalStack Debug Information"
echo "================================"

# Check LocalStack status
echo "📊 LocalStack Status:"
localstack status services
echo ""

# Check running containers
echo "🐳 Running Containers:"
docker ps --filter "name=localstack"
echo ""

# Check LocalStack logs for errors
echo "📝 Recent LocalStack Logs (last 50 lines):"
docker-compose -f docker-compose.localstack.yml logs --tail=50 localstack
echo ""

# Test Azure CLI connectivity
echo "🔧 Azure CLI Test:"
az account show --output table 2>/dev/null || echo "❌ Azure CLI not configured for LocalStack"
echo ""

# Test Key Vault connectivity
echo "🔐 Key Vault Test:"
az keyvault secret list --vault-name $AZURE_KEYVAULT_NAME --endpoint-url http://localhost:4566 --output table 2>/dev/null || echo "❌ Key Vault not accessible"
echo ""

# Test FYPhish health endpoint
echo "🏥 FYPhish Health Check:"
curl -s http://localhost:3333/health | jq . 2>/dev/null || echo "❌ FYPhish not running or health endpoint not available"
echo ""

# Check environment variables
echo "🌍 Environment Variables:"
echo "AZURE_ENDPOINT_URL: $AZURE_ENDPOINT_URL"
echo "AZURE_KEYVAULT_URL: $AZURE_KEYVAULT_URL"
echo "AZURE_KEYVAULT_NAME: $AZURE_KEYVAULT_NAME"
echo "LOCALSTACK_API_KEY: ${LOCALSTACK_API_KEY:0:10}..." # Show only first 10 chars
echo ""

# Network connectivity test
echo "🌐 Network Connectivity:"
curl -s http://localhost:4566/_localstack/health > /dev/null && echo "✅ LocalStack Gateway accessible" || echo "❌ LocalStack Gateway not accessible"
curl -s http://localhost:3333/live > /dev/null && echo "✅ FYPhish accessible" || echo "❌ FYPhish not accessible"
```

### 3. Performance Testing

Create `scripts/performance-test.sh`:

```bash
#!/bin/bash

echo "⚡ LocalStack Performance Testing"
echo "================================="

# Test Key Vault performance
echo "🔐 Testing Key Vault operations..."
time {
    for i in {1..10}; do
        az keyvault secret set --vault-name $AZURE_KEYVAULT_NAME --name "perf-test-$i" --value "test-value-$i" --endpoint-url http://localhost:4566 > /dev/null
    done
}

echo "🔍 Testing Key Vault retrieval..."
time {
    for i in {1..10}; do
        az keyvault secret show --vault-name $AZURE_KEYVAULT_NAME --name "perf-test-$i" --endpoint-url http://localhost:4566 > /dev/null
    done
}

# Test FYPhish endpoints
echo "🏥 Testing FYPhish endpoints..."
echo "Health endpoint (10 requests):"
time {
    for i in {1..10}; do
        curl -s http://localhost:3333/health > /dev/null
    done
}

echo "Login page (10 requests):"
time {
    for i in {1..10}; do
        curl -s http://localhost:3333/login > /dev/null
    done
}

# Cleanup performance test secrets
echo "🧹 Cleaning up test secrets..."
for i in {1..10}; do
    az keyvault secret delete --vault-name $AZURE_KEYVAULT_NAME --name "perf-test-$i" --endpoint-url http://localhost:4566 > /dev/null 2>&1
done

echo "✅ Performance testing complete!"
```

## Troubleshooting

### Common LocalStack Azure Service Limitations

#### 1. Key Vault Limitations

**Issue**: Limited Key Vault operations support
**Symptoms**:
- Secret operations work but certificate/key operations fail
- Advanced access policies not supported
- Managed identity simulation issues

**Workarounds**:
```bash
# Use environment variables as fallback for complex scenarios
export AZURE_CLIENT_ID="localstack-client-id"
export AZURE_CLIENT_SECRET="localstack-client-secret"
export AZURE_TENANT_ID="00000000-0000-0000-0000-000000000000"

# Test Key Vault connectivity before using
function test_keyvault() {
    if ! az keyvault secret list --vault-name $AZURE_KEYVAULT_NAME --endpoint-url http://localhost:4566 &>/dev/null; then
        echo "⚠️  Key Vault not available, using environment variables"
        return 1
    fi
    return 0
}
```

#### 2. Database Service Limitations

**Issue**: CosmosDB in LocalStack has limited SQL API support
**Symptoms**:
- Complex queries fail
- Stored procedures not supported
- Triggers and UDFs unavailable

**Workarounds**:
```bash
# Use PostgreSQL container for complex database operations
cat > docker-compose.postgres.yml << 'EOF'
version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    container_name: fyphish-postgres
    environment:
      POSTGRES_DB: fyphish
      POSTGRES_USER: fyphish
      POSTGRES_PASSWORD: localdev
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    networks:
      - fyphish-network

volumes:
  postgres_data:

networks:
  fyphish-network:
    external: true
EOF

# Start PostgreSQL alongside LocalStack
docker-compose -f docker-compose.postgres.yml up -d
```

#### 3. OAuth/Authentication Issues

**Issue**: Microsoft OAuth doesn't work in LocalStack
**Symptoms**:
- OAuth redirects fail
- Token validation errors
- Authentication loops

**Workarounds**:
```bash
# Create mock OAuth provider for testing
cat > scripts/setup-mock-oauth.sh << 'EOF'
#!/bin/bash

# Start mock OAuth server for testing
docker run -d \
  --name oauth-mock \
  --network fyphish-network \
  -p 9999:9999 \
  -e MOCK_OAUTH_PORT=9999 \
  -e MOCK_OAUTH_CLIENT_ID=localstack-oauth-client-id \
  -e MOCK_OAUTH_CLIENT_SECRET=localstack-oauth-client-secret \
  ghcr.io/oauth-mock/oauth-mock:latest

echo "Mock OAuth server started on port 9999"
EOF
```

### Development Workflow Best Practices

#### 1. Environment Isolation

Create `.env.local-override` for personal development settings:
```bash
# Personal development overrides
ADMIN_EMAIL=your-email@company.com
LOCALSTACK_API_KEY=your-personal-api-key
DEBUG_LEVEL=trace

# Custom service ports for conflict resolution
POSTGRES_PORT=5433
LOCALSTACK_PORT=4567
```

#### 2. Rapid Development Cycle

```bash
# Create development workflow script
cat > scripts/dev-cycle.sh << 'EOF'
#!/bin/bash
set -e

echo "🔄 Starting development cycle..."

# Stop running services
make localstack-stop 2>/dev/null || true
pkill -f fyphish 2>/dev/null || true

# Start fresh LocalStack
make localstack-start

# Quick service setup (minimal for development)
./scripts/setup-keyvault.sh
./scripts/setup-network.sh

# Build and run with auto-reload
echo "🚀 Starting FYPhish with auto-reload..."
while true; do
    go build -o bin/fyphish ./gophish.go
    ./bin/fyphish &
    FYPHISH_PID=$!

    # Watch for Go file changes
    inotifywait -r -e modify,move,create,delete --include='\.go$' . 2>/dev/null

    echo "📝 Changes detected, restarting..."
    kill $FYPHISH_PID 2>/dev/null || true
    sleep 2
done
EOF

chmod +x scripts/dev-cycle.sh
```

#### 3. Configuration Testing

```bash
# Create configuration validation script
cat > scripts/validate-config.sh << 'EOF'
#!/bin/bash

echo "🔍 Validating FYPhish configuration..."

# Test environment variables
required_vars=("AZURE_KEYVAULT_NAME" "AZURE_SUBSCRIPTION_ID" "ADMIN_EMAIL")
for var in "${required_vars[@]}"; do
    if [[ -z "${!var}" ]]; then
        echo "❌ Missing required variable: $var"
        exit 1
    else
        echo "✅ $var: ${!var}"
    fi
done

# Test LocalStack connectivity
if curl -sf http://localhost:4566/_localstack/health >/dev/null; then
    echo "✅ LocalStack is accessible"
else
    echo "❌ LocalStack is not accessible"
    exit 1
fi

# Test Key Vault access
if az keyvault secret list --vault-name $AZURE_KEYVAULT_NAME --endpoint-url http://localhost:4566 >/dev/null 2>&1; then
    echo "✅ Key Vault is accessible"
else
    echo "❌ Key Vault is not accessible"
    exit 1
fi

# Test FYPhish configuration loading
if go run ./gophish.go --test-config; then
    echo "✅ FYPhish configuration is valid"
else
    echo "❌ FYPhish configuration is invalid"
    exit 1
fi

echo "🎯 All configuration tests passed!"
EOF

chmod +x scripts/validate-config.sh
```

### Performance Optimization for Local Testing

#### 1. LocalStack Optimization

```bash
# Optimize LocalStack configuration
cat > docker-compose.localstack-optimized.yml << 'EOF'
version: '3.8'

services:
  localstack:
    container_name: localstack-azure-optimized
    image: localstack/localstack-pro:latest
    ports:
      - "4566:4566"
    environment:
      # Performance optimizations
      - LOCALSTACK_API_KEY=${LOCALSTACK_API_KEY}
      - EAGER_SERVICE_LOADING=1
      - SKIP_INFRA_DOWNLOADS=1
      - DISABLE_EVENTS=1
      - DISABLE_CORS_CHECKS=1
      - DISABLE_CUSTOM_CORE_LISTENER=1

      # Memory optimization
      - LAMBDA_EXECUTOR=local
      - LAMBDA_REMOTE_DOCKER=0
      - LAMBDA_REMOVE_CONTAINERS=1

      # Service-specific optimizations
      - SERVICES=keyvault,storage,cosmosdb,containerregistry
      - PERSISTENCE=0  # Disable for faster startup in development

      # Azure configuration
      - PROVIDER=azure
      - AZURE_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
      - AZURE_TENANT_ID=00000000-0000-0000-0000-000000000000

    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
    deploy:
      resources:
        limits:
          memory: 2G
        reservations:
          memory: 1G
    networks:
      - fyphish-network

networks:
  fyphish-network:
    driver: bridge
EOF
```

#### 2. Caching Strategies

```bash
# Create caching layer for development
cat > pkg/cache/dev_cache.go << 'EOF'
package cache

import (
    "context"
    "sync"
    "time"
)

type DevCache struct {
    secrets map[string]cacheItem
    mu      sync.RWMutex
    ttl     time.Duration
}

type cacheItem struct {
    value     string
    timestamp time.Time
}

func NewDevCache(ttl time.Duration) *DevCache {
    return &DevCache{
        secrets: make(map[string]cacheItem),
        ttl:     ttl,
    }
}

func (c *DevCache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    item, exists := c.secrets[key]
    if !exists || time.Since(item.timestamp) > c.ttl {
        return "", false
    }

    return item.value, true
}

func (c *DevCache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.secrets[key] = cacheItem{
        value:     value,
        timestamp: time.Now(),
    }
}

func (c *DevCache) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.secrets = make(map[string]cacheItem)
}
EOF
```

#### 3. Parallel Service Setup

```bash
# Create parallel setup script for faster initialization
cat > scripts/setup-localstack-parallel.sh << 'EOF'
#!/bin/bash
set -e

source .env.localstack

echo "🚀 Setting up LocalStack services in parallel..."

# Start LocalStack
docker-compose -f docker-compose.localstack-optimized.yml up -d

# Wait for LocalStack
echo "⏳ Waiting for LocalStack..."
while ! curl -sf http://localhost:4566/_localstack/health >/dev/null; do
    sleep 2
done

# Configure Azure CLI
./scripts/configure-azure-cli.sh

# Run service setup in parallel
echo "🔧 Setting up services in parallel..."
(
    echo "Setting up Key Vault..."
    ./scripts/setup-keyvault.sh
) &

(
    echo "Setting up Network..."
    ./scripts/setup-network.sh
) &

(
    echo "Setting up Container Registry..."
    ./scripts/setup-registry.sh
) &

# Wait for all background jobs
wait

echo "✅ Parallel setup completed!"
EOF

chmod +x scripts/setup-localstack-parallel.sh
```

### Debugging Techniques for LocalStack Services

#### 1. Service-Specific Debugging

```bash
# Create comprehensive debugging script
cat > scripts/debug-services.sh << 'EOF'
#!/bin/bash

echo "🔍 Debugging LocalStack Services"
echo "================================="

# Function to test service endpoint
test_service() {
    local service=$1
    local endpoint=$2
    local description=$3

    echo "Testing $description..."
    if curl -sf "$endpoint" >/dev/null 2>&1; then
        echo "✅ $service: OK"
    else
        echo "❌ $service: FAILED"
        echo "   Endpoint: $endpoint"
        echo "   Trying to diagnose..."

        # Additional debugging
        curl -v "$endpoint" 2>&1 | head -10
    fi
    echo
}

# Test core LocalStack
test_service "LocalStack" "http://localhost:4566/_localstack/health" "LocalStack core health"

# Test Key Vault
test_service "KeyVault" "http://localhost:4566/_localstack/health" "Key Vault service"

# Test specific Azure CLI commands
echo "🔧 Testing Azure CLI commands..."

# Key Vault operations
echo "Testing Key Vault operations:"
if az keyvault list --endpoint-url http://localhost:4566 >/dev/null 2>&1; then
    echo "✅ Key Vault list: OK"
else
    echo "❌ Key Vault list: FAILED"
    az keyvault list --endpoint-url http://localhost:4566 --debug 2>&1 | tail -5
fi

# Storage operations
echo "Testing Storage operations:"
if az storage account list --endpoint-url http://localhost:4566 >/dev/null 2>&1; then
    echo "✅ Storage account list: OK"
else
    echo "❌ Storage account list: FAILED"
fi

# Container Registry
echo "Testing Container Registry:"
if az acr list --endpoint-url http://localhost:4566 >/dev/null 2>&1; then
    echo "✅ Container Registry list: OK"
else
    echo "❌ Container Registry list: FAILED"
fi

echo
echo "📊 LocalStack Resource Usage:"
docker stats localstack-azure-optimized --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"

echo
echo "📝 Recent LocalStack Errors:"
docker logs localstack-azure-optimized --tail=20 2>&1 | grep -i error || echo "No recent errors found"
EOF

chmod +x scripts/debug-services.sh
```

#### 2. Network Debugging

```bash
# Create network debugging script
cat > scripts/debug-network.sh << 'EOF'
#!/bin/bash

echo "🌐 Network Debugging for LocalStack"
echo "==================================="

# Check port availability
echo "📡 Port Status:"
ports=(4566 3333 5432 9999)
for port in "${ports[@]}"; do
    if lsof -i :$port >/dev/null 2>&1; then
        echo "✅ Port $port: In use"
        lsof -i :$port | head -2
    else
        echo "❌ Port $port: Available"
    fi
    echo
done

# Check container networking
echo "🐳 Container Network Status:"
docker network ls | grep fyphish || echo "No fyphish networks found"
echo

# Test connectivity between containers
echo "🔗 Container Connectivity:"
if docker ps --format "{{.Names}}" | grep -q localstack; then
    echo "Testing LocalStack internal connectivity..."
    docker exec localstack-azure-optimized curl -sf http://localhost:4566/_localstack/health >/dev/null && echo "✅ LocalStack internal: OK" || echo "❌ LocalStack internal: FAILED"
fi

# Test host connectivity
echo "🖥️  Host Connectivity:"
curl -sf http://localhost:4566/_localstack/health >/dev/null && echo "✅ Host to LocalStack: OK" || echo "❌ Host to LocalStack: FAILED"
curl -sf http://localhost:3333/health >/dev/null && echo "✅ Host to FYPhish: OK" || echo "❌ Host to FYPhish: FAILED"

# DNS resolution test
echo "🔍 DNS Resolution:"
nslookup localhost >/dev/null && echo "✅ localhost resolution: OK" || echo "❌ localhost resolution: FAILED"
EOF

chmod +x scripts/debug-network.sh
```

### Integration Testing Strategies

#### 1. End-to-End Testing

```bash
# Create E2E test script
cat > scripts/test-e2e.sh << 'EOF'
#!/bin/bash
set -e

echo "🧪 Running End-to-End Tests"
echo "==========================="

# Start clean environment
make clean
make localstack-setup

echo "⏳ Waiting for services to be ready..."
sleep 10

# Test 1: Key Vault integration
echo "Test 1: Key Vault Integration"
echo "------------------------------"
export TEST_SECRET_VALUE="e2e-test-$(date +%s)"

# Set secret via Azure CLI
az keyvault secret set \
  --vault-name $AZURE_KEYVAULT_NAME \
  --name "e2e-test-secret" \
  --value "$TEST_SECRET_VALUE" \
  --endpoint-url http://localhost:4566

# Retrieve secret via FYPhish Go code
RETRIEVED_VALUE=$(go run -tags=integration ./tests/integration/keyvault_test.go)

if [[ "$RETRIEVED_VALUE" == "$TEST_SECRET_VALUE" ]]; then
    echo "✅ Key Vault integration: PASSED"
else
    echo "❌ Key Vault integration: FAILED"
    echo "   Expected: $TEST_SECRET_VALUE"
    echo "   Got: $RETRIEVED_VALUE"
    exit 1
fi

# Test 2: Health endpoints
echo
echo "Test 2: Health Endpoints"
echo "-------------------------"
make build run &
FYPHISH_PID=$!

sleep 5

for endpoint in "/health" "/ready" "/live"; do
    if curl -sf "http://localhost:3333$endpoint" | jq . >/dev/null; then
        echo "✅ $endpoint: PASSED"
    else
        echo "❌ $endpoint: FAILED"
        kill $FYPHISH_PID 2>/dev/null || true
        exit 1
    fi
done

# Test 3: OAuth flow simulation
echo
echo "Test 3: OAuth Flow Simulation"
echo "------------------------------"
# Test OAuth initiation endpoint
if curl -sf "http://localhost:3333/auth/microsoft" | grep -q "redirect"; then
    echo "✅ OAuth initiation: PASSED"
else
    echo "❌ OAuth initiation: FAILED"
fi

# Cleanup
kill $FYPHISH_PID 2>/dev/null || true

echo
echo "🎉 All E2E tests completed successfully!"
EOF

chmod +x scripts/test-e2e.sh
```

#### 2. Load Testing

```bash
# Create load testing script using wrk or curl
cat > scripts/test-load.sh << 'EOF'
#!/bin/bash

echo "⚡ Load Testing FYPhish with LocalStack"
echo "======================================="

# Ensure FYPhish is running
if ! curl -sf http://localhost:3333/health >/dev/null; then
    echo "❌ FYPhish is not running. Start it first with 'make run'"
    exit 1
fi

# Test 1: Health endpoint load
echo "Test 1: Health Endpoint Load Test"
echo "----------------------------------"
echo "Running 1000 requests with 10 concurrent connections..."

if command -v wrk >/dev/null; then
    wrk -t10 -c10 -d30s --latency http://localhost:3333/health
else
    echo "wrk not found, using curl..."
    time {
        for i in {1..100}; do
            curl -sf http://localhost:3333/health >/dev/null &
            if (( i % 10 == 0 )); then
                wait  # Limit concurrent requests
            fi
        done
        wait
    }
fi

# Test 2: Key Vault operations load
echo
echo "Test 2: Key Vault Operations Load Test"
echo "---------------------------------------"
echo "Testing 50 Key Vault operations..."

time {
    for i in {1..50}; do
        SECRET_NAME="load-test-$i"
        SECRET_VALUE="value-$i-$(date +%s)"

        # Set secret
        az keyvault secret set \
          --vault-name $AZURE_KEYVAULT_NAME \
          --name "$SECRET_NAME" \
          --value "$SECRET_VALUE" \
          --endpoint-url http://localhost:4566 >/dev/null &

        if (( i % 10 == 0 )); then
            wait  # Limit concurrent operations
        fi
    done
    wait
}

# Cleanup load test secrets
echo
echo "🧹 Cleaning up load test secrets..."
for i in {1..50}; do
    az keyvault secret delete \
      --vault-name $AZURE_KEYVAULT_NAME \
      --name "load-test-$i" \
      --endpoint-url http://localhost:4566 >/dev/null 2>&1 &
done
wait

echo "✅ Load testing completed!"
EOF

chmod +x scripts/test-load.sh
```

## Advanced LocalStack Configuration

### 1. Production-Like Environment

```bash
# Create production simulation configuration
cat > docker-compose.localstack-prod.yml << 'EOF'
version: '3.8'

services:
  localstack:
    container_name: localstack-azure-prod-sim
    image: localstack/localstack-pro:latest
    ports:
      - "4566:4566"
      - "443:443"    # HTTPS simulation
    environment:
      - LOCALSTACK_API_KEY=${LOCALSTACK_API_KEY}

      # Production simulation settings
      - PERSISTENCE=1
      - DATA_DIR=/tmp/localstack/data
      - DISABLE_CORS_CHECKS=0
      - ENFORCE_IAM=1
      - STRICT_SERVICE_LOADING=1

      # HTTPS/TLS simulation
      - USE_SSL=1
      - SSL_CERT_PATH=/tmp/localstack/ssl/cert.pem
      - SSL_KEY_PATH=/tmp/localstack/ssl/key.pem

      # Azure production-like settings
      - AZURE_SUBSCRIPTION_ID=${AZURE_SUBSCRIPTION_ID}
      - AZURE_TENANT_ID=${AZURE_TENANT_ID}
      - AZURE_ENFORCE_RBAC=1
      - AZURE_KEYVAULT_ENFORCE_HTTPS=1

      # Service configuration
      - SERVICES=keyvault,cosmosdb,storage,containerregistry,appservice,network

    volumes:
      - "${LOCALSTACK_VOLUME_DIR:-./localstack-volume}:/tmp/localstack"
      - "/var/run/docker.sock:/var/run/docker.sock"
      - "./certs:/tmp/localstack/ssl"
    networks:
      - fyphish-prod-network

networks:
  fyphish-prod-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.21.0.0/16
EOF
```

### 2. SSL Certificate Generation

```bash
# Create SSL certificate generation script
cat > scripts/generate-ssl-certs.sh << 'EOF'
#!/bin/bash

echo "🔐 Generating SSL certificates for LocalStack..."

mkdir -p certs

# Generate private key
openssl genrsa -out certs/key.pem 2048

# Generate certificate signing request
openssl req -new -key certs/key.pem -out certs/cert.csr -subj "/C=US/ST=CA/L=SF/O=LocalStack/CN=localhost"

# Generate self-signed certificate
openssl x509 -req -days 365 -in certs/cert.csr -signkey certs/key.pem -out certs/cert.pem

# Set appropriate permissions
chmod 600 certs/key.pem
chmod 644 certs/cert.pem

echo "✅ SSL certificates generated in ./certs/"
echo "   Certificate: ./certs/cert.pem"
echo "   Private Key: ./certs/key.pem"

# Clean up CSR
rm certs/cert.csr
EOF

chmod +x scripts/generate-ssl-certs.sh
```

### 3. Monitoring and Observability

```bash
# Create monitoring setup for LocalStack
cat > docker-compose.monitoring.yml << 'EOF'
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: fyphish-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - fyphish-network

  grafana:
    image: grafana/grafana:latest
    container_name: fyphish-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./monitoring/grafana/datasources:/etc/grafana/provisioning/datasources
    networks:
      - fyphish-network

volumes:
  grafana_data:

networks:
  fyphish-network:
    external: true
EOF

# Create Prometheus configuration
mkdir -p monitoring
cat > monitoring/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'localstack'
    static_configs:
      - targets: ['localstack-azure:4566']
    metrics_path: '/_localstack/health'
    scrape_interval: 30s

  - job_name: 'fyphish'
    static_configs:
      - targets: ['host.docker.internal:3333']
    metrics_path: '/metrics'
    scrape_interval: 15s
EOF
```

## Quick Reference Commands

### Daily Development Commands
```bash
# Start development environment
make dev-setup

# Run with auto-reload
./scripts/dev-cycle.sh

# Debug issues
./scripts/debug-services.sh
./scripts/debug-network.sh

# Run tests
make test
make test-integration
./scripts/test-e2e.sh

# Performance testing
./scripts/test-load.sh

# Clean and restart
make clean && make dev-setup
```

### Troubleshooting Commands
```bash
# Check LocalStack status
localstack status services

# View LocalStack logs
docker logs localstack-azure-optimized

# Test connectivity
curl http://localhost:4566/_localstack/health
curl http://localhost:3333/health

# Reset environment
docker-compose -f docker-compose.localstack.yml down -v
docker system prune -f
make dev-setup
```

### Azure CLI LocalStack Commands
```bash
# Switch to LocalStack
az cloud set --name LocalStackAzure

# Key Vault operations
az keyvault secret list --vault-name $AZURE_KEYVAULT_NAME --endpoint-url http://localhost:4566

# Storage operations
az storage account list --endpoint-url http://localhost:4566

# Container Registry operations
az acr list --endpoint-url http://localhost:4566
```

This comprehensive LocalStack for Azure setup guide provides everything needed to create a robust local development environment for FYPhish that closely mirrors Azure production services. The setup includes service configurations, integration code, testing strategies, and troubleshooting guides specifically tailored for your Go-based phishing toolkit with Microsoft SSO integration.