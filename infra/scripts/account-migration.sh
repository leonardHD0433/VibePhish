#!/bin/bash
# Azure Account Migration Script for FYPhish
# Migrates FYPhish deployment from one Azure subscription/account to another

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATION_LOG="/tmp/fyphish-account-migration.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$MIGRATION_LOG"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$MIGRATION_LOG"
    exit 1
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$MIGRATION_LOG"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$MIGRATION_LOG"
}

# Help function
show_help() {
    cat << EOF
Azure Account Migration Script for FYPhish

This script helps migrate FYPhish deployment from one Azure subscription to another.
It handles infrastructure export, data backup, and deployment recreation.

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    export              Export current infrastructure and data
    validate-target     Validate target Azure subscription/account
    deploy-target       Deploy infrastructure to target subscription
    migrate-data        Migrate application data
    complete-migration  Complete migration and cleanup source
    rollback           Rollback migration to source account

Options:
    --source-subscription ID       Source Azure subscription ID
    --target-subscription ID       Target Azure subscription ID
    --source-resource-group NAME   Source resource group name
    --target-resource-group NAME   Target resource group name
    --environment ENV              Environment (dev/test/prod)
    --backup-location PATH         Backup storage location
    --target-location REGION       Target Azure region
    --dry-run                     Show what would be done
    --force                       Force migration without confirmations
    --skip-data                   Skip data migration (infrastructure only)

Examples:
    # Complete migration workflow
    $0 export --source-subscription 12345 --source-resource-group rg-fyphish-prod
    $0 validate-target --target-subscription 67890 --target-location "West US 2"
    $0 deploy-target --target-subscription 67890 --environment prod
    $0 migrate-data --source-subscription 12345 --target-subscription 67890
    $0 complete-migration --source-subscription 12345

    # Infrastructure-only migration
    $0 export --source-subscription 12345 --skip-data
    $0 deploy-target --target-subscription 67890 --skip-data

EOF
}

# Default values
COMMAND=""
SOURCE_SUBSCRIPTION=""
TARGET_SUBSCRIPTION=""
SOURCE_RESOURCE_GROUP=""
TARGET_RESOURCE_GROUP=""
ENVIRONMENT="prod"
BACKUP_LOCATION="./migration-backup"
TARGET_LOCATION="East US"
DRY_RUN=false
FORCE=false
SKIP_DATA=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        export|validate-target|deploy-target|migrate-data|complete-migration|rollback)
            COMMAND="$1"
            shift
            ;;
        --source-subscription)
            SOURCE_SUBSCRIPTION="$2"
            shift 2
            ;;
        --target-subscription)
            TARGET_SUBSCRIPTION="$2"
            shift 2
            ;;
        --source-resource-group)
            SOURCE_RESOURCE_GROUP="$2"
            shift 2
            ;;
        --target-resource-group)
            TARGET_RESOURCE_GROUP="$2"
            shift 2
            ;;
        --environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --backup-location)
            BACKUP_LOCATION="$2"
            shift 2
            ;;
        --target-location)
            TARGET_LOCATION="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --skip-data)
            SKIP_DATA=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Validate command
if [[ -z "$COMMAND" ]]; then
    error "No command specified. Use --help for usage information."
fi

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    # Check Azure CLI
    if ! command -v az &> /dev/null; then
        error "Azure CLI is required but not installed"
    fi

    # Check if logged in to Azure
    if ! az account show &> /dev/null; then
        error "Please login to Azure CLI first: az login"
    fi

    # Check jq for JSON processing
    if ! command -v jq &> /dev/null; then
        error "jq is required but not installed"
    fi

    success "Prerequisites check passed"
}

# Create backup directory
create_backup_directory() {
    log "Creating backup directory: $BACKUP_LOCATION"
    mkdir -p "$BACKUP_LOCATION"

    # Create subdirectories
    mkdir -p "$BACKUP_LOCATION/infrastructure"
    mkdir -p "$BACKUP_LOCATION/data"
    mkdir -p "$BACKUP_LOCATION/configuration"
    mkdir -p "$BACKUP_LOCATION/logs"
}

# Export infrastructure configuration
export_infrastructure() {
    log "Exporting infrastructure configuration..."

    if [[ -z "$SOURCE_SUBSCRIPTION" || -z "$SOURCE_RESOURCE_GROUP" ]]; then
        error "Source subscription and resource group must be specified for export"
    fi

    # Set source subscription
    az account set --subscription "$SOURCE_SUBSCRIPTION"

    # Export resource group as ARM template
    local template_file="$BACKUP_LOCATION/infrastructure/fyphish-template.json"
    local parameters_file="$BACKUP_LOCATION/infrastructure/fyphish-parameters.json"

    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would export resource group $SOURCE_RESOURCE_GROUP"
        return
    fi

    log "Exporting ARM template for resource group: $SOURCE_RESOURCE_GROUP"
    az group export \
        --resource-group "$SOURCE_RESOURCE_GROUP" \
        --include-comments \
        --include-parameter-default-value > "$template_file"

    # Extract parameters
    jq '.parameters' "$template_file" > "$parameters_file"

    # Export resource tags and metadata
    local metadata_file="$BACKUP_LOCATION/infrastructure/metadata.json"
    az group show --resource-group "$SOURCE_RESOURCE_GROUP" > "$metadata_file"

    # Export Key Vault secrets (names only for security)
    export_keyvault_metadata

    # Export container registry images
    export_container_images

    success "Infrastructure export completed"
}

# Export Key Vault metadata
export_keyvault_metadata() {
    log "Exporting Key Vault metadata..."

    local keyvault_name=$(az keyvault list \
        --resource-group "$SOURCE_RESOURCE_GROUP" \
        --query '[0].name' -o tsv 2>/dev/null || echo "")

    if [[ -n "$keyvault_name" ]]; then
        local keyvault_file="$BACKUP_LOCATION/configuration/keyvault-secrets.json"

        # Export secret names and properties (not values)
        az keyvault secret list \
            --vault-name "$keyvault_name" \
            --query '[].{name:name, enabled:attributes.enabled, created:attributes.created}' \
            -o json > "$keyvault_file"

        log "Key Vault metadata exported: $keyvault_file"
    else
        warn "No Key Vault found in source resource group"
    fi
}

# Export container images
export_container_images() {
    log "Exporting container registry information..."

    local acr_name=$(az acr list \
        --resource-group "$SOURCE_RESOURCE_GROUP" \
        --query '[0].name' -o tsv 2>/dev/null || echo "")

    if [[ -n "$acr_name" ]]; then
        local acr_file="$BACKUP_LOCATION/infrastructure/acr-images.json"

        # List repositories and tags
        az acr repository list \
            --name "$acr_name" \
            --output json > "$acr_file"

        # Export image manifests for the main application
        local images_dir="$BACKUP_LOCATION/infrastructure/images"
        mkdir -p "$images_dir"

        az acr repository show-tags \
            --name "$acr_name" \
            --repository fyphish \
            --output json > "$images_dir/fyphish-tags.json" 2>/dev/null || true

        log "Container registry information exported"
    else
        warn "No Azure Container Registry found in source resource group"
    fi
}

# Export application data
export_data() {
    if [[ "$SKIP_DATA" == true ]]; then
        log "Skipping data export (--skip-data specified)"
        return
    fi

    log "Exporting application data..."

    # Export MySQL database
    export_mysql_database

    # Export application configuration
    export_application_config

    success "Data export completed"
}

# Export MySQL database
export_mysql_database() {
    log "Exporting MySQL database..."

    local mysql_server=$(az mysql flexible-server list \
        --resource-group "$SOURCE_RESOURCE_GROUP" \
        --query '[0].name' -o tsv 2>/dev/null || echo "")

    if [[ -n "$mysql_server" ]]; then
        local backup_file="$BACKUP_LOCATION/data/mysql-backup-$(date +%Y%m%d_%H%M%S).sql"

        # Note: This requires proper network access and credentials
        log "MySQL server found: $mysql_server"
        log "Manual step required: Export MySQL database to $backup_file"
        log "Use: mysqldump -h $mysql_server.mysql.database.azure.com -u admin -p fyphish > $backup_file"

        # Create instructions file
        cat > "$BACKUP_LOCATION/data/mysql-export-instructions.txt" << EOF
Manual MySQL Export Required:

1. Ensure your IP is allowed in MySQL firewall rules
2. Run the following command:
   mysqldump -h $mysql_server.mysql.database.azure.com -u admin -p fyphish > $backup_file

3. Verify the backup file was created successfully
4. Store the admin password securely for target deployment
EOF

    else
        warn "No MySQL server found in source resource group"
    fi
}

# Export application configuration
export_application_config() {
    log "Exporting application configuration..."

    # Export container instance environment variables (excluding secrets)
    local container_groups=$(az container list \
        --resource-group "$SOURCE_RESOURCE_GROUP" \
        --query '[].name' -o tsv)

    for cg in $container_groups; do
        local config_file="$BACKUP_LOCATION/configuration/$cg-config.json"
        az container show \
            --resource-group "$SOURCE_RESOURCE_GROUP" \
            --name "$cg" \
            --query 'containers[0].environmentVariables' \
            -o json > "$config_file"
    done
}

# Validate target subscription
validate_target() {
    log "Validating target Azure subscription..."

    if [[ -z "$TARGET_SUBSCRIPTION" ]]; then
        error "Target subscription must be specified"
    fi

    # Switch to target subscription
    az account set --subscription "$TARGET_SUBSCRIPTION"

    # Check subscription access
    local subscription_info=$(az account show --query '{id:id, name:name, state:state}' -o json)
    log "Target subscription: $(echo $subscription_info | jq -r '.name')"

    # Check available regions
    local available_regions=$(az account list-locations \
        --query "[?name=='$TARGET_LOCATION'].displayName" -o tsv)

    if [[ -z "$available_regions" ]]; then
        error "Target location '$TARGET_LOCATION' is not available in this subscription"
    fi

    # Check quota and limits
    log "Checking compute quotas in target region..."
    local quotas=$(az vm list-usage --location "$TARGET_LOCATION" \
        --query "[?contains(name.value, 'standardDSv3Family')].{name:name.localizedValue, current:currentValue, limit:limit}" \
        -o json)

    log "Available quotas: $(echo $quotas | jq -c '.')"

    success "Target subscription validation completed"
}

# Deploy to target subscription
deploy_target() {
    log "Deploying infrastructure to target subscription..."

    if [[ -z "$TARGET_SUBSCRIPTION" ]]; then
        error "Target subscription must be specified"
    fi

    # Switch to target subscription
    az account set --subscription "$TARGET_SUBSCRIPTION"

    # Generate unique suffix for new deployment
    local unique_suffix=$(echo $RANDOM | md5sum | head -c 6)
    local new_resource_group="${TARGET_RESOURCE_GROUP:-rg-fyphish-$ENVIRONMENT-$unique_suffix}"

    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would deploy infrastructure to $new_resource_group"
        return
    fi

    # Deploy using Bicep templates
    log "Deploying Bicep template to target subscription..."

    local deployment_name="fyphish-migration-$(date +%Y%m%d%H%M%S)"

    az deployment sub create \
        --location "$TARGET_LOCATION" \
        --template-file "$PROJECT_ROOT/infrastructure/bicep/main.bicep" \
        --parameters \
            environment="$ENVIRONMENT" \
            uniqueSuffix="$unique_suffix" \
            adminEmail="${ADMIN_EMAIL:-admin@example.com}" \
            allowedDomain="${ALLOWED_DOMAIN:-example.com}" \
        --name "$deployment_name"

    # Store deployment information
    local deployment_info="$BACKUP_LOCATION/infrastructure/target-deployment.json"
    az deployment sub show \
        --name "$deployment_name" \
        --query 'properties.outputs' > "$deployment_info"

    success "Target infrastructure deployment completed"
    log "New resource group: $new_resource_group"
}

# Migrate data to target
migrate_data() {
    if [[ "$SKIP_DATA" == true ]]; then
        log "Skipping data migration (--skip-data specified)"
        return
    fi

    log "Migrating data to target subscription..."

    # This is a placeholder for data migration logic
    # In practice, this would involve:
    # 1. Restoring MySQL database
    # 2. Updating configuration
    # 3. Importing container images
    # 4. Setting up Key Vault secrets

    warn "Data migration requires manual steps - see migration documentation"

    # Create migration checklist
    create_migration_checklist

    success "Data migration preparation completed"
}

# Create migration checklist
create_migration_checklist() {
    local checklist_file="$BACKUP_LOCATION/MIGRATION_CHECKLIST.md"

    cat > "$checklist_file" << 'EOF'
# FYPhish Account Migration Checklist

## Pre-Migration (Source Account)
- [ ] Export infrastructure templates
- [ ] Backup MySQL database
- [ ] Export Key Vault secret names
- [ ] Export container images
- [ ] Document current configuration

## Target Account Setup
- [ ] Deploy infrastructure using Bicep templates
- [ ] Create Key Vault secrets
- [ ] Set up MySQL database
- [ ] Configure networking and security
- [ ] Import container images

## Data Migration
- [ ] Restore MySQL database from backup
- [ ] Update OAuth application registration
- [ ] Test authentication flow
- [ ] Verify application functionality
- [ ] Update DNS/URL configurations

## Post-Migration Verification
- [ ] Smoke test all functionality
- [ ] Performance testing
- [ ] Security validation
- [ ] Monitoring setup
- [ ] Documentation updates

## Cleanup (After Verification)
- [ ] Remove source infrastructure
- [ ] Cancel source subscriptions
- [ ] Update team access
- [ ] Archive migration artifacts

EOF

    log "Migration checklist created: $checklist_file"
}

# Complete migration
complete_migration() {
    log "Completing migration process..."

    if [[ "$FORCE" != true ]]; then
        echo -n "Are you sure you want to complete the migration and cleanup source resources? (y/N): "
        read -r confirmation
        if [[ "$confirmation" != "y" && "$confirmation" != "Y" ]]; then
            log "Migration completion cancelled by user"
            exit 0
        fi
    fi

    # Switch back to source subscription for cleanup
    if [[ -n "$SOURCE_SUBSCRIPTION" ]]; then
        az account set --subscription "$SOURCE_SUBSCRIPTION"

        if [[ "$DRY_RUN" == true ]]; then
            log "DRY RUN: Would cleanup source resource group $SOURCE_RESOURCE_GROUP"
        else
            log "Cleaning up source resource group..."
            # Uncomment when ready for cleanup
            # az group delete --resource-group "$SOURCE_RESOURCE_GROUP" --yes --no-wait
            log "Source cleanup commands prepared (manual execution required)"
        fi
    fi

    success "Migration process completed"
}

# Rollback migration
rollback_migration() {
    log "Rolling back migration..."

    warn "Rollback functionality not fully implemented"
    log "To rollback:"
    log "1. Restore source infrastructure from backup"
    log "2. Restore database from backup"
    log "3. Update DNS to point back to source"
    log "4. Remove target resources"
}

# Main execution
main() {
    log "Starting FYPhish account migration"
    log "Command: $COMMAND"

    check_prerequisites
    create_backup_directory

    case "$COMMAND" in
        export)
            export_infrastructure
            export_data
            ;;
        validate-target)
            validate_target
            ;;
        deploy-target)
            deploy_target
            ;;
        migrate-data)
            migrate_data
            ;;
        complete-migration)
            complete_migration
            ;;
        rollback)
            rollback_migration
            ;;
        *)
            error "Unknown command: $COMMAND"
            ;;
    esac

    success "Account migration script completed"
    log "Check migration log: $MIGRATION_LOG"
    log "Check artifacts: $BACKUP_LOCATION"
}

# Run main function
main "$@"