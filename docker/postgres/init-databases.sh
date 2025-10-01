#!/bin/bash
# PostgreSQL Multi-Database Initialization Script
# ================================================
# This script runs automatically when the PostgreSQL container starts for the first time
# It creates both FYPhish and n8n databases in the same PostgreSQL instance
#
# Environment Variables (set via Dockerfile or docker-compose):
#   POSTGRES_USER       - Main PostgreSQL superuser (default: fyphish_user)
#   POSTGRES_PASSWORD   - Password for main user
#   POSTGRES_DB         - Main database name (default: fyphish)
#   N8N_DB_PASSWORD     - Password for n8n user (default: n8n_dev_2025)
# ================================================

set -e

echo ""
echo "========================================="
echo "FYPhish Multi-Database Initialization"
echo "========================================="
echo ""

# Function to execute SQL as the main user
psql_exec() {
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        $1
EOSQL
}

# =====================================================
# n8n Database Setup
# =====================================================
echo "Creating n8n database and user..."

# Create n8n user
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "postgres" <<-EOSQL
    -- Create n8n database
    CREATE DATABASE n8n;

    -- Create n8n user with password from environment
    CREATE USER n8n_user WITH PASSWORD '${N8N_DB_PASSWORD}';

    -- Grant all privileges on n8n database to n8n_user
    GRANT ALL PRIVILEGES ON DATABASE n8n TO n8n_user;

    -- Grant schema privileges
    \c n8n
    GRANT ALL ON SCHEMA public TO n8n_user;
    ALTER SCHEMA public OWNER TO n8n_user;
EOSQL

echo "✓ n8n database created successfully"

# =====================================================
# Verification
# =====================================================
echo ""
echo "========================================="
echo "Database Initialization Complete!"
echo "========================================="
echo ""
echo "Created databases:"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "postgres" -c "\l" | grep -E "fyphish|n8n"

echo ""
echo "FYPhish Configuration:"
echo "  Database: fyphish"
echo "  User: $POSTGRES_USER"
echo "  Connection: postgres://$POSTGRES_USER:****@localhost:5432/fyphish"
echo ""
echo "n8n Configuration:"
echo "  Database: n8n"
echo "  User: n8n_user"
echo "  Connection: postgres://n8n_user:****@localhost:5432/n8n"
echo ""
echo "========================================="
echo ""
