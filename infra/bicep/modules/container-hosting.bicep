// Container Hosting Module - Azure Container Instances for cost-effective deployment
@description('Location for all resources')
param location string

@description('Environment name')
param environment string

@description('Unique suffix for resource names')
param uniqueSuffix string

@description('Key Vault resource ID for secrets')
param keyVaultResourceId string

@description('ACR login server')
param acrLoginServer string

@description('MySQL connection string')
@secure()
param mysqlConnectionString string

@description('Application Insights connection string')
param appInsightsConnectionString string

@description('Admin email for FYPhish')
param adminEmail string

@description('Allowed domain for SSO')
param allowedDomain string

@description('Resource tags')
param tags object

// Variables
var containerGroupName = 'aci-fyphish-${environment}-${uniqueSuffix}'
var dnsNameLabel = 'fyphish-${environment}-${uniqueSuffix}'
var imageName = '${acrLoginServer}/fyphish:latest'

// Get Key Vault reference for secrets
resource keyVault 'Microsoft.KeyVault/vaults@2023-02-01' existing = {
  name: split(keyVaultResourceId, '/')[8]
}

// Container Group for FYPhish application
resource containerGroup 'Microsoft.ContainerInstance/containerGroups@2023-05-01' = {
  name: containerGroupName
  location: location
  tags: tags
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${keyVaultResourceId}/../userAssignedIdentities/id-fyphish-${environment}-${uniqueSuffix}': {}
    }
  }
  properties: {
    containers: [
      {
        name: 'fyphish-app'
        properties: {
          image: imageName
          ports: [
            {
              port: 3333
              protocol: 'TCP'
            }
            {
              port: 8080
              protocol: 'TCP'
            }
          ]
          environmentVariables: [
            {
              name: 'GO_ENV'
              value: environment == 'prod' ? 'production' : 'development'
            }
            {
              name: 'DB_TYPE'
              value: 'mysql'
            }
            {
              name: 'MYSQL_CONNECTION_STRING'
              secureValue: mysqlConnectionString
            }
            {
              name: 'ADMIN_EMAIL'
              value: adminEmail
            }
            {
              name: 'ALLOWED_DOMAIN'
              value: allowedDomain
            }
            {
              name: 'ADMIN_DOMAIN'
              value: split(adminEmail, '@')[1]
            }
            {
              name: 'APPLICATIONINSIGHTS_CONNECTION_STRING'
              value: appInsightsConnectionString
            }
            {
              name: 'AZURE_CLIENT_ID'
              value: reference('${keyVaultResourceId}/../userAssignedIdentities/id-fyphish-${environment}-${uniqueSuffix}', '2023-01-31').clientId
            }
            {
              name: 'KEY_VAULT_URL'
              value: keyVault.properties.vaultUri
            }
            // OAuth secrets will be loaded from Key Vault at runtime
            {
              name: 'MICROSOFT_CLIENT_ID'
              value: '@Microsoft.KeyVault(VaultName=${keyVault.name};SecretName=microsoft-client-id)'
            }
            {
              name: 'MICROSOFT_CLIENT_SECRET'
              value: '@Microsoft.KeyVault(VaultName=${keyVault.name};SecretName=microsoft-client-secret)'
            }
            {
              name: 'MICROSOFT_TENANT_ID'
              value: '@Microsoft.KeyVault(VaultName=${keyVault.name};SecretName=microsoft-tenant-id)'
            }
            {
              name: 'SESSION_SIGNING_KEY'
              value: '@Microsoft.KeyVault(VaultName=${keyVault.name};SecretName=session-signing-key)'
            }
            {
              name: 'SESSION_ENCRYPTION_KEY'
              value: '@Microsoft.KeyVault(VaultName=${keyVault.name};SecretName=session-encryption-key)'
            }
          ]
          resources: {
            requests: {
              cpu: environment == 'prod' ? 2 : 1
              memoryInGB: environment == 'prod' ? 4 : 2
            }
          }
          livenessProbe: {
            httpGet: {
              path: '/health'
              port: 3333
              scheme: 'HTTP'
            }
            initialDelaySeconds: 30
            periodSeconds: 30
            timeoutSeconds: 10
            failureThreshold: 3
          }
          readinessProbe: {
            httpGet: {
              path: '/ready'
              port: 3333
              scheme: 'HTTP'
            }
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
          }
        }
      }
    ]
    imageRegistryCredentials: [
      {
        server: acrLoginServer
        identity: '${keyVaultResourceId}/../userAssignedIdentities/id-fyphish-${environment}-${uniqueSuffix}'
      }
    ]
    restartPolicy: 'Always'
    ipAddress: {
      type: 'Public'
      dnsNameLabel: dnsNameLabel
      ports: [
        {
          port: 3333
          protocol: 'TCP'
        }
        {
          port: 8080
          protocol: 'TCP'
        }
      ]
    }
    osType: 'Linux'
  }
}

// Outputs
output containerGroupResourceId string = containerGroup.id
output containerGroupName string = containerGroup.name
output applicationUrl string = 'http://${containerGroup.properties.ipAddress.fqdn}:3333'
output applicationFqdn string = containerGroup.properties.ipAddress.fqdn
output publicIpAddress string = containerGroup.properties.ipAddress.ip