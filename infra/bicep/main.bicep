// FYPhish Azure Infrastructure - Main Bicep Template
// This template creates all Azure resources for FYPhish deployment
// Supports easy migration between Azure subscriptions

targetScope = 'subscription'

@description('Environment name (dev, test, prod)')
@allowed(['dev', 'test', 'prod'])
param environment string = 'dev'

@description('Location for all resources')
param location string = 'East US'

@description('Unique suffix for resource names (use organization/project code)')
@minLength(3)
@maxLength(6)
param uniqueSuffix string

@description('Admin email for FYPhish access')
param adminEmail string

@description('Allowed domain for SSO access')
param allowedDomain string = 'outlook.com'

@description('Enable MySQL read replica for production')
param enableReadReplica bool = false

@description('Enable Application Gateway for production')
param enableAppGateway bool = false

// Variables
var resourceGroupName = 'rg-fyphish-${environment}-${uniqueSuffix}'
var keyVaultName = 'kv-fyphish-${environment}-${uniqueSuffix}'
var acrName = 'acrfyphish${environment}${uniqueSuffix}'
var appInsightsName = 'ai-fyphish-${environment}-${uniqueSuffix}'
var logAnalyticsName = 'law-fyphish-${environment}-${uniqueSuffix}'
var mysqlServerName = 'mysql-fyphish-${environment}-${uniqueSuffix}'
var storageAccountName = 'stfyphish${environment}${uniqueSuffix}'

// Tags for all resources
var commonTags = {
  Environment: environment
  Project: 'FYPhish'
  CreatedBy: 'IaC-Bicep'
  ManagedBy: 'Azure-DevOps'
  CostCenter: 'SecurityTraining'
}

// Create Resource Group
resource resourceGroup 'Microsoft.Resources/resourceGroups@2021-04-01' = {
  name: resourceGroupName
  location: location
  tags: commonTags
}

// Deploy core infrastructure
module coreInfrastructure 'modules/core-infrastructure.bicep' = {
  scope: resourceGroup
  name: 'coreInfrastructure'
  params: {
    location: location
    environment: environment
    uniqueSuffix: uniqueSuffix
    keyVaultName: keyVaultName
    acrName: acrName
    appInsightsName: appInsightsName
    logAnalyticsName: logAnalyticsName
    storageAccountName: storageAccountName
    tags: commonTags
  }
}

// Deploy MySQL database
module database 'modules/mysql-database.bicep' = {
  scope: resourceGroup
  name: 'database'
  params: {
    location: location
    environment: environment
    mysqlServerName: mysqlServerName
    enableReadReplica: enableReadReplica
    tags: commonTags
  }
}

// Deploy application hosting (Container Instances for cost optimization)
module appHosting 'modules/container-hosting.bicep' = {
  scope: resourceGroup
  name: 'appHosting'
  params: {
    location: location
    environment: environment
    uniqueSuffix: uniqueSuffix
    keyVaultResourceId: coreInfrastructure.outputs.keyVaultResourceId
    acrLoginServer: coreInfrastructure.outputs.acrLoginServer
    mysqlConnectionString: database.outputs.connectionString
    appInsightsConnectionString: coreInfrastructure.outputs.appInsightsConnectionString
    adminEmail: adminEmail
    allowedDomain: allowedDomain
    tags: commonTags
  }
}

// Deploy Application Gateway for production environments
module appGateway 'modules/application-gateway.bicep' = if (enableAppGateway && environment == 'prod') {
  scope: resourceGroup
  name: 'appGateway'
  params: {
    location: location
    environment: environment
    uniqueSuffix: uniqueSuffix
    backendFqdn: appHosting.outputs.applicationFqdn
    tags: commonTags
  }
}

// Deploy monitoring and alerting
module monitoring 'modules/monitoring.bicep' = {
  scope: resourceGroup
  name: 'monitoring'
  params: {
    location: location
    environment: environment
    uniqueSuffix: uniqueSuffix
    appInsightsResourceId: coreInfrastructure.outputs.appInsightsResourceId
    logAnalyticsResourceId: coreInfrastructure.outputs.logAnalyticsResourceId
    adminEmail: adminEmail
    tags: commonTags
  }
}

// Outputs for CI/CD pipeline
output resourceGroupName string = resourceGroupName
output keyVaultName string = keyVaultName
output acrName string = acrName
output acrLoginServer string = coreInfrastructure.outputs.acrLoginServer
output mysqlServerName string = mysqlServerName
output applicationUrl string = appHosting.outputs.applicationUrl
output applicationFqdn string = appHosting.outputs.applicationFqdn
output appInsightsInstrumentationKey string = coreInfrastructure.outputs.appInsightsInstrumentationKey
output logAnalyticsWorkspaceId string = coreInfrastructure.outputs.logAnalyticsWorkspaceId