#!/bin/bash
# FYPhish Development Runner
# ==========================
# This script sets up the complete FYPhish development environment
#
# What it does:
#   1. Builds PostgreSQL container from Dockerfile.postgres
#   2. Starts PostgreSQL container with proper configuration
#   3. Waits for database to be ready
#   4. Starts FYPhish application with go run
#
# Usage:
#   ./run.sh              # Start everything
#   ./run.sh --rebuild    # Rebuild containers and start
#   ./run.sh --stop       # Stop all containers
#   ./run.sh --clean      # Stop and remove containers + volumes

set -e  # Exit on error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
POSTGRES_CONTAINER_NAME="fyphish-postgres-db"
POSTGRES_IMAGE_NAME="fyphish-postgres:latest"
POSTGRES_VOLUME="fyphish-postgres-data"

# Database credentials (matches Dockerfile.postgres defaults)
POSTGRES_USER="${POSTGRES_USER:-fyphish_user}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-fyphish_dev_2025}"
POSTGRES_DB="${POSTGRES_DB:-fyphish}"
N8N_DB_PASSWORD="${N8N_DB_PASSWORD:-n8n_dev_2025}"

# Helper functions
print_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

# Check if container exists
container_exists() {
    docker ps -a --format '{{.Names}}' | grep -q "^${1}$"
}

# Check if container is running
container_running() {
    docker ps --format '{{.Names}}' | grep -q "^${1}$"
}

# Stop containers
stop_containers() {
    print_header "Stopping FYPhish Containers"

    if container_running "$POSTGRES_CONTAINER_NAME"; then
        print_info "Stopping PostgreSQL container..."
        docker stop "$POSTGRES_CONTAINER_NAME"
        print_success "PostgreSQL container stopped"
    else
        print_info "PostgreSQL container is not running"
    fi
}

# Clean up containers and volumes
clean_all() {
    print_header "Cleaning FYPhish Environment"

    stop_containers

    if container_exists "$POSTGRES_CONTAINER_NAME"; then
        print_info "Removing PostgreSQL container..."
        docker rm "$POSTGRES_CONTAINER_NAME"
        print_success "PostgreSQL container removed"
    fi

    if docker volume ls | grep -q "$POSTGRES_VOLUME"; then
        print_warning "Found data volume: $POSTGRES_VOLUME"
        read -p "Do you want to remove the database volume? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            docker volume rm "$POSTGRES_VOLUME"
            print_success "Database volume removed"
        else
            print_info "Database volume preserved"
        fi
    fi

    print_success "Cleanup complete"
    exit 0
}

# Build PostgreSQL container
build_postgres() {
    print_header "Building PostgreSQL Container"

    if [[ ! -f "Dockerfile.postgres" ]]; then
        print_error "Dockerfile.postgres not found!"
        exit 1
    fi

    print_info "Building $POSTGRES_IMAGE_NAME from Dockerfile.postgres..."
    docker build -f Dockerfile.postgres -t "$POSTGRES_IMAGE_NAME" .
    print_success "PostgreSQL image built successfully"
}

# Start PostgreSQL container
start_postgres() {
    print_header "Starting PostgreSQL Container"

    # Check if already running
    if container_running "$POSTGRES_CONTAINER_NAME"; then
        print_success "PostgreSQL container is already running"
        return 0
    fi

    # Remove existing stopped container
    if container_exists "$POSTGRES_CONTAINER_NAME"; then
        print_info "Removing existing container..."
        docker rm "$POSTGRES_CONTAINER_NAME" > /dev/null 2>&1
    fi

    print_info "Starting PostgreSQL container..."
    docker run -d \
        --name "$POSTGRES_CONTAINER_NAME" \
        --hostname fyphish-postgres \
        --env POSTGRES_USER="$POSTGRES_USER" \
        --env POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        --env POSTGRES_DB="$POSTGRES_DB" \
        --env N8N_DB_PASSWORD="$N8N_DB_PASSWORD" \
        --volume "$POSTGRES_VOLUME:/var/lib/postgresql/data" \
        --network bridge \
        -p 5432:5432 \
        --restart unless-stopped \
        --label 'description=PostgreSQL with FYPhish and n8n databases' \
        --label 'maintainer=FYPhish Developer' \
        --label 'version=1.0' \
        "$POSTGRES_IMAGE_NAME"

    print_success "PostgreSQL container started"
}

# Wait for PostgreSQL to be ready
wait_for_postgres() {
    print_header "Waiting for PostgreSQL"

    print_info "Checking database readiness..."
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if docker exec "$POSTGRES_CONTAINER_NAME" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null 2>&1; then
            print_success "PostgreSQL is ready!"

            # Show database info
            print_info "Database configuration:"
            echo "  Host: localhost"
            echo "  Port: 5432"
            echo "  Database: $POSTGRES_DB"
            echo "  User: $POSTGRES_USER"
            echo "  Connection string: postgres://$POSTGRES_USER:****@localhost:5432/$POSTGRES_DB?sslmode=disable"
            echo ""

            return 0
        fi

        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done

    print_error "PostgreSQL failed to start within ${max_attempts} attempts"
    print_info "Check logs with: docker logs $POSTGRES_CONTAINER_NAME"
    exit 1
}

# Load environment variables
load_env() {
    if [[ -f ".env" ]]; then
        print_info "Loading environment variables from .env file..."
        export $(grep -v '^#' .env | grep -v '^$' | xargs)
        print_success "Environment variables loaded"
    else
        print_warning ".env file not found. Using default values."
        print_info "Copy .env.example to .env to customize configuration"
    fi
}

# Start FYPhish application
start_fyphish() {
    print_header "Starting FYPhish Application"

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed! Please install Go 1.24 or later."
        exit 1
    fi

    # Set database environment variables for Go application
    export DB_NAME="postgres"
    export DB_HOST="localhost"
    export DB_PORT="5432"
    export DB_USER="$POSTGRES_USER"
    export DB_PASSWORD="$POSTGRES_PASSWORD"
    export DB_SSLMODE="disable"

    print_info "Database connection: postgres://$DB_USER:****@$DB_HOST:$DB_PORT/$POSTGRES_DB"
    print_info "Starting FYPhish with 'go run gophish.go'..."
    echo ""

    # Run the application
    go run gophish.go
}

# Main execution
main() {
    print_header "FYPhish Development Environment Setup"

    # Parse arguments
    case "${1:-}" in
        --stop)
            stop_containers
            exit 0
            ;;
        --clean)
            clean_all
            ;;
        --rebuild)
            print_info "Rebuild mode enabled"
            stop_containers
            build_postgres
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  (no args)     Start PostgreSQL and FYPhish"
            echo "  --rebuild     Rebuild PostgreSQL image and start"
            echo "  --stop        Stop all containers"
            echo "  --clean       Stop and remove containers + volumes"
            echo "  --help, -h    Show this help message"
            echo ""
            exit 0
            ;;
        "")
            # Normal startup - build only if image doesn't exist
            if ! docker images | grep -q "$POSTGRES_IMAGE_NAME"; then
                print_info "PostgreSQL image not found, building..."
                build_postgres
            else
                print_success "PostgreSQL image found"
            fi
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac

    # Load environment
    load_env

    # Start PostgreSQL
    start_postgres
    wait_for_postgres

    # Start FYPhish
    print_success "Infrastructure ready!"
    echo ""
    print_info "Access FYPhish at: http://localhost:3333"
    print_info "Default credentials: admin / gophish"
    print_info "Press Ctrl+C to stop the application"
    echo ""

    # Trap Ctrl+C to stop gracefully
    trap 'echo ""; print_info "Stopping FYPhish..."; exit 0' INT TERM

    start_fyphish
}

# Run main function
main "$@"
