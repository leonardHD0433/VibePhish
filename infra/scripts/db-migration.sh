#!/bin/bash
# Database Migration Script for FYPhish Azure Deployment
# Handles SQLite to MySQL migration with Azure Database for MySQL

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATION_LOG="/tmp/fyphish-migration.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
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
Database Migration Script for FYPhish

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    migrate-to-mysql    Migrate from SQLite to MySQL
    backup-sqlite       Create backup of SQLite database
    restore-mysql       Restore MySQL database from backup
    validate            Validate database schema and data
    setup-azure-mysql   Setup Azure MySQL database

Options:
    --sqlite-db PATH       Path to SQLite database (default: ./gophish.db)
    --mysql-host HOST      MySQL host
    --mysql-port PORT      MySQL port (default: 3306)
    --mysql-db DATABASE    MySQL database name
    --mysql-user USER      MySQL username
    --mysql-pass PASSWORD  MySQL password
    --backup-dir PATH      Backup directory (default: ./backups)
    --dry-run             Show what would be done without executing
    --force               Force migration even if target database exists
    --verbose             Enable verbose output

Environment Variables:
    MYSQL_CONNECTION_STRING    Full MySQL connection string
    AZURE_MYSQL_HOST          Azure MySQL server hostname
    AZURE_MYSQL_DATABASE      Azure MySQL database name
    AZURE_MYSQL_USERNAME      Azure MySQL username
    AZURE_MYSQL_PASSWORD      Azure MySQL password

Examples:
    # Migrate to Azure MySQL
    $0 migrate-to-mysql --mysql-host myserver.mysql.database.azure.com \\
                       --mysql-db fyphish --mysql-user admin@myserver

    # Backup current SQLite database
    $0 backup-sqlite --backup-dir ./backups

    # Validate database after migration
    $0 validate --mysql-host myserver.mysql.database.azure.com

EOF
}

# Parse command line arguments
COMMAND=""
SQLITE_DB="./gophish.db"
MYSQL_HOST=""
MYSQL_PORT="3306"
MYSQL_DB=""
MYSQL_USER=""
MYSQL_PASS=""
BACKUP_DIR="./backups"
DRY_RUN=false
FORCE=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        migrate-to-mysql|backup-sqlite|restore-mysql|validate|setup-azure-mysql)
            COMMAND="$1"
            shift
            ;;
        --sqlite-db)
            SQLITE_DB="$2"
            shift 2
            ;;
        --mysql-host)
            MYSQL_HOST="$2"
            shift 2
            ;;
        --mysql-port)
            MYSQL_PORT="$2"
            shift 2
            ;;
        --mysql-db)
            MYSQL_DB="$2"
            shift 2
            ;;
        --mysql-user)
            MYSQL_USER="$2"
            shift 2
            ;;
        --mysql-pass)
            MYSQL_PASS="$2"
            shift 2
            ;;
        --backup-dir)
            BACKUP_DIR="$2"
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
        --verbose)
            VERBOSE=true
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

# Check if command is provided
if [[ -z "$COMMAND" ]]; then
    error "No command specified. Use --help for usage information."
fi

# Load environment variables if available
if [[ -n "${MYSQL_CONNECTION_STRING:-}" ]]; then
    log "Using MySQL connection string from environment"
    # Parse connection string if needed
fi

if [[ -n "${AZURE_MYSQL_HOST:-}" ]]; then
    MYSQL_HOST="${AZURE_MYSQL_HOST}"
fi

if [[ -n "${AZURE_MYSQL_DATABASE:-}" ]]; then
    MYSQL_DB="${AZURE_MYSQL_DATABASE}"
fi

if [[ -n "${AZURE_MYSQL_USERNAME:-}" ]]; then
    MYSQL_USER="${AZURE_MYSQL_USERNAME}"
fi

if [[ -n "${AZURE_MYSQL_PASSWORD:-}" ]]; then
    MYSQL_PASS="${AZURE_MYSQL_PASSWORD}"
fi

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    # Check if SQLite database exists for migration commands
    if [[ "$COMMAND" == "migrate-to-mysql" || "$COMMAND" == "backup-sqlite" ]]; then
        if [[ ! -f "$SQLITE_DB" ]]; then
            error "SQLite database not found: $SQLITE_DB"
        fi
    fi

    # Check MySQL connection parameters for MySQL commands
    if [[ "$COMMAND" =~ ^(migrate-to-mysql|restore-mysql|validate|setup-azure-mysql)$ ]]; then
        if [[ -z "$MYSQL_HOST" || -z "$MYSQL_DB" || -z "$MYSQL_USER" ]]; then
            error "MySQL connection parameters are required. Use --mysql-host, --mysql-db, --mysql-user"
        fi
    fi

    # Check required tools
    local required_tools=("mysql" "mysqldump" "sqlite3")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            error "Required tool not found: $tool"
        fi
    done

    success "Prerequisites check passed"
}

# Create MySQL connection string
get_mysql_connection() {
    if [[ -n "$MYSQL_PASS" ]]; then
        echo "mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB"
    else
        echo "mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p $MYSQL_DB"
    fi
}

# Test MySQL connection
test_mysql_connection() {
    log "Testing MySQL connection..."

    local mysql_cmd=$(get_mysql_connection)
    if echo "SELECT 1;" | $mysql_cmd &>/dev/null; then
        success "MySQL connection successful"
        return 0
    else
        error "Failed to connect to MySQL database"
    fi
}

# Backup SQLite database
backup_sqlite() {
    log "Creating SQLite database backup..."

    mkdir -p "$BACKUP_DIR"
    local backup_file="$BACKUP_DIR/gophish_backup_$(date +%Y%m%d_%H%M%S).db"

    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would copy $SQLITE_DB to $backup_file"
        return
    fi

    cp "$SQLITE_DB" "$backup_file"
    success "SQLite backup created: $backup_file"

    # Create SQL dump as well
    local sql_dump="$BACKUP_DIR/gophish_backup_$(date +%Y%m%d_%H%M%S).sql"
    sqlite3 "$SQLITE_DB" .dump > "$sql_dump"
    success "SQL dump created: $sql_dump"
}

# Setup Azure MySQL database
setup_azure_mysql() {
    log "Setting up Azure MySQL database..."

    local mysql_cmd=$(get_mysql_connection)

    # Create database if it doesn't exist
    local create_db_sql="CREATE DATABASE IF NOT EXISTS \`$MYSQL_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would execute: $create_db_sql"
        return
    fi

    echo "$create_db_sql" | mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS"
    success "Azure MySQL database setup completed"
}

# Migrate from SQLite to MySQL
migrate_to_mysql() {
    log "Starting migration from SQLite to MySQL..."

    # First, backup SQLite
    backup_sqlite

    # Test MySQL connection
    test_mysql_connection

    # Setup MySQL database
    setup_azure_mysql

    # Convert SQLite schema to MySQL
    log "Converting SQLite schema to MySQL..."

    local schema_file="$BACKUP_DIR/mysql_schema_$(date +%Y%m%d_%H%M%S).sql"
    local data_file="$BACKUP_DIR/mysql_data_$(date +%Y%m%d_%H%M%S).sql"

    # Extract schema
    sqlite3 "$SQLITE_DB" .schema > "$schema_file.tmp"

    # Convert SQLite schema to MySQL
    convert_sqlite_to_mysql_schema "$schema_file.tmp" "$schema_file"

    # Extract data
    log "Extracting data from SQLite..."
    sqlite3 "$SQLITE_DB" .dump | grep -E '^INSERT' > "$data_file.tmp"

    # Convert SQLite INSERT statements to MySQL
    convert_sqlite_to_mysql_data "$data_file.tmp" "$data_file"

    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would execute MySQL schema and data migration"
        log "Schema file: $schema_file"
        log "Data file: $data_file"
        return
    fi

    # Execute MySQL schema
    log "Creating MySQL schema..."
    local mysql_cmd=$(get_mysql_connection)
    $mysql_cmd < "$schema_file"

    # Execute MySQL data
    log "Importing data to MySQL..."
    $mysql_cmd < "$data_file"

    success "Migration from SQLite to MySQL completed"

    # Validate migration
    validate_migration
}

# Convert SQLite schema to MySQL
convert_sqlite_to_mysql_schema() {
    local sqlite_schema="$1"
    local mysql_schema="$2"

    log "Converting SQLite schema to MySQL format..."

    # SQLite to MySQL schema conversion
    sed -e 's/AUTOINCREMENT/AUTO_INCREMENT/g' \
        -e 's/INTEGER PRIMARY KEY/INT PRIMARY KEY AUTO_INCREMENT/g' \
        -e 's/INTEGER/INT/g' \
        -e 's/DATETIME/TIMESTAMP/g' \
        -e 's/TEXT/VARCHAR(255)/g' \
        -e 's/BLOB/LONGBLOB/g' \
        -e '/^PRAGMA/d' \
        -e '/^BEGIN TRANSACTION/d' \
        -e '/^COMMIT/d' \
        -e 's/`/\`/g' \
        "$sqlite_schema" > "$mysql_schema"

    # Add MySQL-specific settings
    cat >> "$mysql_schema" << 'EOF'

-- MySQL specific settings for FYPhish
SET sql_mode = 'NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION';
SET time_zone = '+00:00';
SET foreign_key_checks = 1;
EOF

    success "Schema conversion completed"
}

# Convert SQLite data to MySQL
convert_sqlite_to_mysql_data() {
    local sqlite_data="$1"
    local mysql_data="$2"

    log "Converting SQLite data to MySQL format..."

    # Convert INSERT statements
    sed -e 's/INSERT INTO \([^(]*\)/INSERT INTO `\1`/g' \
        -e "s/''/NULL/g" \
        -e 's/\([0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\} [0-9]\{2\}:[0-9]\{2\}:[0-9]\{2\}\)/"\1"/g' \
        "$sqlite_data" > "$mysql_data"

    success "Data conversion completed"
}

# Validate migration
validate_migration() {
    log "Validating migration..."

    local mysql_cmd=$(get_mysql_connection)

    # Count tables
    local sqlite_tables=$(sqlite3 "$SQLITE_DB" "SELECT COUNT(*) FROM sqlite_master WHERE type='table';")
    local mysql_tables=$(echo "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$MYSQL_DB';" | $mysql_cmd | tail -n 1)

    log "SQLite tables: $sqlite_tables"
    log "MySQL tables: $mysql_tables"

    if [[ "$sqlite_tables" -eq "$mysql_tables" ]]; then
        success "Table count validation passed"
    else
        warn "Table count mismatch: SQLite=$sqlite_tables, MySQL=$mysql_tables"
    fi

    # Validate specific FYPhish tables
    local fyphish_tables=("users" "campaigns" "templates" "targets" "groups" "results" "authorized_emails")
    for table in "${fyphish_tables[@]}"; do
        local count=$(echo "SELECT COUNT(*) FROM $table;" | $mysql_cmd 2>/dev/null | tail -n 1 || echo "0")
        log "Table '$table' has $count records"
    done

    success "Migration validation completed"
}

# Main execution
main() {
    log "Starting FYPhish database migration script"
    log "Command: $COMMAND"

    check_prerequisites

    case "$COMMAND" in
        backup-sqlite)
            backup_sqlite
            ;;
        setup-azure-mysql)
            setup_azure_mysql
            ;;
        migrate-to-mysql)
            migrate_to_mysql
            ;;
        validate)
            test_mysql_connection
            validate_migration
            ;;
        restore-mysql)
            # TODO: Implement MySQL restore functionality
            error "MySQL restore functionality not yet implemented"
            ;;
        *)
            error "Unknown command: $COMMAND"
            ;;
    esac

    success "Database migration script completed successfully"
}

# Run main function
main "$@"