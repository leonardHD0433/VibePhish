// MySQL Database Module for FYPhish
@description('Location for all resources')
param location string

@description('Environment name')
param environment string

@description('MySQL server name')
param mysqlServerName string

@description('Enable read replica for production')
param enableReadReplica bool = false

@description('Resource tags')
param tags object

// MySQL Administrator credentials (stored in Key Vault)
@secure()
param administratorLogin string = 'fyphish_admin'

@secure()
param administratorLoginPassword string = newGuid()

// MySQL Server
resource mysqlServer 'Microsoft.DBforMySQL/flexibleServers@2023-06-30' = {
  name: mysqlServerName
  location: location
  tags: tags
  sku: {
    name: environment == 'prod' ? 'Standard_B2s' : 'Standard_B1ms'
    tier: 'Burstable'
  }
  properties: {
    administratorLogin: administratorLogin
    administratorLoginPassword: administratorLoginPassword
    version: '8.0.21'
    storage: {
      storageSizeGB: environment == 'prod' ? 100 : 20
      iops: environment == 'prod' ? 400 : 360
      autoGrow: 'Enabled'
      autoIoScaling: 'Enabled'
    }
    network: {
      publicNetworkAccess: 'Enabled'
    }
    backup: {
      backupRetentionDays: environment == 'prod' ? 35 : 7
      geoRedundantBackup: environment == 'prod' ? 'Enabled' : 'Disabled'
    }
    highAvailability: {
      mode: environment == 'prod' ? 'ZoneRedundant' : 'Disabled'
    }
    maintenanceWindow: {
      customWindow: 'Enabled'
      dayOfWeek: 1 // Monday
      startHour: 3
      startMinute: 0
    }
  }
}

// MySQL Database for FYPhish
resource fyphishDatabase 'Microsoft.DBforMySQL/flexibleServers/databases@2023-06-30' = {
  parent: mysqlServer
  name: 'fyphish'
  properties: {
    charset: 'utf8mb4'
    collation: 'utf8mb4_0900_ai_ci'
  }
}

// Firewall rule to allow Azure services
resource allowAzureServices 'Microsoft.DBforMySQL/flexibleServers/firewallRules@2023-06-30' = {
  parent: mysqlServer
  name: 'AllowAzureServices'
  properties: {
    startIpAddress: '0.0.0.0'
    endIpAddress: '0.0.0.0'
  }
}

// Firewall rule for development (remove in production)
resource allowDevelopment 'Microsoft.DBforMySQL/flexibleServers/firewallRules@2023-06-30' = if (environment != 'prod') {
  parent: mysqlServer
  name: 'AllowDevelopment'
  properties: {
    startIpAddress: '0.0.0.0'
    endIpAddress: '255.255.255.255'
  }
}

// MySQL Configuration optimizations
resource mysqlConfigInnodbBufferPool 'Microsoft.DBforMySQL/flexibleServers/configurations@2023-06-30' = {
  parent: mysqlServer
  name: 'innodb_buffer_pool_size'
  properties: {
    value: environment == 'prod' ? '1073741824' : '268435456' // 1GB for prod, 256MB for dev/test
    source: 'user-override'
  }
}

resource mysqlConfigMaxConnections 'Microsoft.DBforMySQL/flexibleServers/configurations@2023-06-30' = {
  parent: mysqlServer
  name: 'max_connections'
  properties: {
    value: environment == 'prod' ? '200' : '50'
    source: 'user-override'
  }
}

resource mysqlConfigSlowQueryLog 'Microsoft.DBforMySQL/flexibleServers/configurations@2023-06-30' = {
  parent: mysqlServer
  name: 'slow_query_log'
  properties: {
    value: 'ON'
    source: 'user-override'
  }
}

resource mysqlConfigLongQueryTime 'Microsoft.DBforMySQL/flexibleServers/configurations@2023-06-30' = {
  parent: mysqlServer
  name: 'long_query_time'
  properties: {
    value: '2'
    source: 'user-override'
  }
}

// Read replica for production workloads
resource mysqlReadReplica 'Microsoft.DBforMySQL/flexibleServers@2023-06-30' = if (enableReadReplica && environment == 'prod') {
  name: '${mysqlServerName}-replica'
  location: location
  tags: union(tags, { Role: 'ReadReplica' })
  sku: mysqlServer.sku
  properties: {
    createMode: 'Replica'
    sourceServerResourceId: mysqlServer.id
  }
}

// Outputs
output mysqlServerResourceId string = mysqlServer.id
output mysqlServerName string = mysqlServer.name
output mysqlServerFqdn string = mysqlServer.properties.fullyQualifiedDomainName
output databaseName string = fyphishDatabase.name
output administratorLogin string = administratorLogin
output connectionString string = 'Server=${mysqlServer.properties.fullyQualifiedDomainName};Database=${fyphishDatabase.name};Uid=${administratorLogin};Pwd=${administratorLoginPassword};SslMode=Required;'
output connectionStringKeyVaultFormat string = 'Server=${mysqlServer.properties.fullyQualifiedDomainName};Database=${fyphishDatabase.name};Uid=${administratorLogin};Pwd={mysql-admin-password};SslMode=Required;'