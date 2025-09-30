// Monitoring and Alerting Module for FYPhish
@description('Location for all resources')
param location string

@description('Environment name')
param environment string

@description('Unique suffix for resource names')
param uniqueSuffix string

@description('Application Insights resource ID')
param appInsightsResourceId string

@description('Log Analytics workspace resource ID')
param logAnalyticsResourceId string

@description('Admin email for alerts')
param adminEmail string

@description('Resource tags')
param tags object

// Variables
var actionGroupName = 'ag-fyphish-${environment}-${uniqueSuffix}'
var alertRulePrefix = 'alert-fyphish-${environment}'

// Action Group for alert notifications
resource actionGroup 'Microsoft.Insights/actionGroups@2023-01-01' = {
  name: actionGroupName
  location: 'Global'
  tags: tags
  properties: {
    groupShortName: 'FYPhish'
    enabled: true
    emailReceivers: [
      {
        name: 'AdminEmail'
        emailAddress: adminEmail
        useCommonAlertSchema: true
      }
    ]
    azureAppPushReceivers: []
    itsmReceivers: []
    azureAppPushReceivers: []
    automationRunbookReceivers: []
    voiceReceivers: []
    logicAppReceivers: []
    azureFunctionReceivers: []
    armRoleReceivers: []
  }
}

// Alert Rules

// High CPU Usage Alert
resource cpuAlert 'Microsoft.Insights/metricAlerts@2018-03-01' = {
  name: '${alertRulePrefix}-high-cpu'
  location: 'Global'
  tags: tags
  properties: {
    description: 'Alert when CPU usage is high'
    severity: 2
    enabled: true
    scopes: [
      appInsightsResourceId
    ]
    evaluationFrequency: 'PT5M'
    windowSize: 'PT15M'
    criteria: {
      'odata.type': 'Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria'
      allOf: [
        {
          name: 'HighCPU'
          metricName: 'performanceCounters/processCpuPercentage'
          operator: 'GreaterThan'
          threshold: 80
          timeAggregation: 'Average'
        }
      ]
    }
    actions: [
      {
        actionGroupId: actionGroup.id
      }
    ]
  }
}

// High Memory Usage Alert
resource memoryAlert 'Microsoft.Insights/metricAlerts@2018-03-01' = {
  name: '${alertRulePrefix}-high-memory'
  location: 'Global'
  tags: tags
  properties: {
    description: 'Alert when memory usage is high'
    severity: 2
    enabled: true
    scopes: [
      appInsightsResourceId
    ]
    evaluationFrequency: 'PT5M'
    windowSize: 'PT15M'
    criteria: {
      'odata.type': 'Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria'
      allOf: [
        {
          name: 'HighMemory'
          metricName: 'performanceCounters/memoryAvailableBytes'
          operator: 'LessThan'
          threshold: 104857600 // 100MB
          timeAggregation: 'Average'
        }
      ]
    }
    actions: [
      {
        actionGroupId: actionGroup.id
      }
    ]
  }
}

// High Response Time Alert
resource responseTimeAlert 'Microsoft.Insights/metricAlerts@2018-03-01' = {
  name: '${alertRulePrefix}-high-response-time'
  location: 'Global'
  tags: tags
  properties: {
    description: 'Alert when response time is high'
    severity: 2
    enabled: true
    scopes: [
      appInsightsResourceId
    ]
    evaluationFrequency: 'PT5M'
    windowSize: 'PT15M'
    criteria: {
      'odata.type': 'Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria'
      allOf: [
        {
          name: 'HighResponseTime'
          metricName: 'requests/duration'
          operator: 'GreaterThan'
          threshold: 5000 // 5 seconds
          timeAggregation: 'Average'
        }
      ]
    }
    actions: [
      {
        actionGroupId: actionGroup.id
      }
    ]
  }
}

// Failed Requests Alert
resource failedRequestsAlert 'Microsoft.Insights/metricAlerts@2018-03-01' = {
  name: '${alertRulePrefix}-failed-requests'
  location: 'Global'
  tags: tags
  properties: {
    description: 'Alert when failed request rate is high'
    severity: 1
    enabled: true
    scopes: [
      appInsightsResourceId
    ]
    evaluationFrequency: 'PT5M'
    windowSize: 'PT15M'
    criteria: {
      'odata.type': 'Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria'
      allOf: [
        {
          name: 'FailedRequests'
          metricName: 'requests/failed'
          operator: 'GreaterThan'
          threshold: 10
          timeAggregation: 'Total'
        }
      ]
    }
    actions: [
      {
        actionGroupId: actionGroup.id
      }
    ]
  }
}

// Log Alert for Authentication Failures
resource authFailureAlert 'Microsoft.Insights/scheduledQueryRules@2023-03-15-preview' = {
  name: '${alertRulePrefix}-auth-failures'
  location: location
  tags: tags
  properties: {
    description: 'Alert on authentication failures'
    severity: 1
    enabled: true
    evaluationFrequency: 'PT5M'
    scopes: [
      logAnalyticsResourceId
    ]
    targetResourceTypes: [
      'Microsoft.OperationalInsights/workspaces'
    ]
    windowSize: 'PT15M'
    criteria: {
      allOf: [
        {
          query: 'traces | where message contains "authentication failed" or message contains "unauthorized access" | summarize count() by bin(timestamp, 5m)'
          timeAggregation: 'Total'
          dimensions: []
          operator: 'GreaterThan'
          threshold: 5
          failingPeriods: {
            numberOfEvaluationPeriods: 1
            minFailingPeriodsToAlert: 1
          }
        }
      ]
    }
    actions: {
      actionGroups: [
        actionGroup.id
      ]
    }
  }
}

// Cost Management Alert (if spending exceeds threshold)
resource costAlert 'Microsoft.CostManagement/scheduledActions@2023-11-01' = if (environment != 'dev') {
  name: '${alertRulePrefix}-cost-alert'
  properties: {
    displayName: 'FYPhish ${environment} Cost Alert'
    status: 'Enabled'
    schedule: {
      frequency: 'Daily'
      hourOfDay: 9
      daysOfWeek: [
        'Monday'
        'Tuesday'
        'Wednesday'
        'Thursday'
        'Friday'
      ]
    }
    scope: subscription().id
    kind: 'Budget'
    definition: {
      type: 'ActualCost'
      timeframe: 'MonthToDate'
      dataset: {
        granularity: 'Daily'
        filter: {
          dimensions: {
            name: 'ResourceGroup'
            operator: 'In'
            values: [
              resourceGroup().name
            ]
          }
        }
      }
    }
    notification: {
      to: [
        adminEmail
      ]
      subject: 'FYPhish ${environment} - Cost Alert'
      message: 'Your FYPhish deployment costs are approaching the threshold.'
    }
  }
}

// Dashboard for monitoring
resource dashboard 'Microsoft.Portal/dashboards@2020-09-01-preview' = {
  name: 'dashboard-fyphish-${environment}-${uniqueSuffix}'
  location: location
  tags: tags
  properties: {
    lenses: [
      {
        order: 0
        parts: [
          {
            position: {
              x: 0
              y: 0
              rowSpan: 4
              colSpan: 6
            }
            metadata: {
              inputs: [
                {
                  name: 'resourceTypeMode'
                  isOptional: true
                }
                {
                  name: 'ComponentId'
                  value: appInsightsResourceId
                  isOptional: true
                }
                {
                  name: 'Scope'
                  value: {
                    resourceIds: [
                      appInsightsResourceId
                    ]
                  }
                  isOptional: true
                }
                {
                  name: 'PartId'
                  value: '1c38a923-16a8-4a6b-8f25-8eb90e14df70'
                  isOptional: true
                }
                {
                  name: 'Version'
                  value: '2.0'
                  isOptional: true
                }
                {
                  name: 'TimeRange'
                  value: 'P1D'
                  isOptional: true
                }
                {
                  name: 'DashboardId'
                  isOptional: true
                }
                {
                  name: 'DashboardTimeRange'
                  value: {
                    relative: {
                      duration: 24
                      timeUnit: 1
                    }
                  }
                  isOptional: true
                }
              ]
              type: 'Extension/HubsExtension/PartType/MonitorChartPart'
              settings: {
                content: {
                  options: {
                    chart: {
                      metrics: [
                        {
                          resourceMetadata: {
                            id: appInsightsResourceId
                          }
                          name: 'requests/rate'
                          aggregationType: 4
                          namespace: 'microsoft.insights/components'
                          metricVisualization: {
                            displayName: 'Server requests'
                            resourceDisplayName: 'FYPhish'
                          }
                        }
                      ]
                      title: 'Request Rate'
                      titleKind: 1
                      visualization: {
                        chartType: 2
                        legendVisualization: {
                          isVisible: true
                          position: 2
                          hideSubtitle: false
                        }
                        axisVisualization: {
                          x: {
                            isVisible: true
                            axisType: 2
                          }
                          y: {
                            isVisible: true
                            axisType: 1
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        ]
      }
    ]
  }
}

// Outputs
output actionGroupResourceId string = actionGroup.id
output actionGroupName string = actionGroup.name
output dashboardResourceId string = dashboard.id