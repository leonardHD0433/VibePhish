# FYPhish Setup Guide for Teammates

Quick setup guide to get FYPhish running on your local machine in under 5 minutes.

## Prerequisites

1. **Docker** - [Install Docker Desktop](https://www.docker.com/products/docker-desktop/)
2. **Go 1.24+** - [Install Go](https://go.dev/doc/install)
3. **Git** - Clone this repository

## Quick Start (3 Steps)

### 1. Clone the Repository

```bash
git clone <repository-url>
cd FYPhish
```

### 2. Configure Environment (Optional)

If you want to use Microsoft SSO or customize settings:

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env with your preferred text editor
nano .env  # or vim, code, etc.
```

For basic testing, you can skip this step and use default values.

### 3. Run the Application

```bash
# Make sure the script is executable
chmod +x run.sh

# Start everything
./run.sh
```

That's it! The script will:
- ✅ Build PostgreSQL container from `Dockerfile.postgres`
- ✅ Start PostgreSQL with FYPhish and n8n databases
- ✅ Wait for database to be ready
- ✅ Start FYPhish application

### Access the Application

Once started, open your browser:

- **Admin Interface**: http://localhost:3333
- **Default Login**:
  - Username: `admin`
  - Password: `gophish`

## Script Commands

```bash
# Normal startup (recommended)
./run.sh

# Rebuild PostgreSQL image and start
./run.sh --rebuild

# Stop all containers
./run.sh --stop

# Clean everything (removes containers and optionally volumes)
./run.sh --clean

# Show help
./run.sh --help
```

## Database Information

The PostgreSQL container includes two databases:

### FYPhish Database
- **Database**: `fyphish`
- **User**: `fyphish_user`
- **Password**: `fyphish_dev_2025`
- **Connection**: `postgres://fyphish_user:fyphish_dev_2025@localhost:5432/fyphish?sslmode=disable`

### n8n Database (for automation workflows)
- **Database**: `n8n`
- **User**: `n8n_user`
- **Password**: `n8n_dev_2025`
- **Connection**: `postgres://n8n_user:n8n_dev_2025@localhost:5432/n8n?sslmode=disable`

## Troubleshooting

### Port 5432 Already in Use

If you have another PostgreSQL instance running:

```bash
# Option 1: Stop your existing PostgreSQL
sudo systemctl stop postgresql  # Linux
brew services stop postgresql    # macOS

# Option 2: Change the port in run.sh
# Edit line: -p 5432:5432
# To:        -p 5433:5432
# Then update DB_PORT in the script
```

### Container Won't Start

```bash
# Check Docker logs
docker logs fyphish-postgres-db

# Clean and restart
./run.sh --clean
./run.sh --rebuild
```

### Database Connection Errors

```bash
# Verify PostgreSQL is running
docker ps | grep fyphish-postgres

# Test connection manually
docker exec -it fyphish-postgres-db psql -U fyphish_user -d fyphish

# Check if port is accessible
nc -zv localhost 5432
```

### Go Application Won't Start

```bash
# Verify Go version
go version  # Should be 1.24+

# Download dependencies
go mod download

# Try running manually
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=fyphish_user
export DB_PASSWORD=fyphish_dev_2025
export DB_NAME=postgres
export DB_SSLMODE=disable
go run gophish.go
```

## Advanced Configuration

### Microsoft SSO Setup

To enable Microsoft SSO authentication:

1. **Create Azure App Registration**
   - Go to [Azure Portal](https://portal.azure.com/) → Azure Active Directory → App registrations
   - Create new registration
   - Set redirect URI: `http://localhost:3333/auth/microsoft/callback`
   - Copy Client ID, Client Secret, and Tenant ID

2. **Configure `.env` file**

```bash
# Admin Configuration
ADMIN_EMAIL=your-admin@company.com

# Microsoft OAuth
MICROSOFT_CLIENT_ID=your-client-id-from-azure
MICROSOFT_CLIENT_SECRET=your-client-secret-from-azure
MICROSOFT_TENANT_ID=common  # or your tenant ID

# Domain Configuration
ALLOWED_DOMAIN=company.com
ADMIN_DOMAIN=company.com

# Session Security (generate with commands below)
SESSION_SIGNING_KEY=$(openssl rand -hex 64)
SESSION_ENCRYPTION_KEY=$(openssl rand -hex 32)

# Enable SSO
SSO_ENABLED=true
MICROSOFT_ENABLED=true
HIDE_LOCAL_LOGIN=true
```

3. **Restart the application**

```bash
./run.sh --stop
./run.sh
```

### Custom Database Credentials

Edit the variables in [run.sh](run.sh) (lines 23-27):

```bash
POSTGRES_USER="${POSTGRES_USER:-your_custom_user}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-your_custom_password}"
POSTGRES_DB="${POSTGRES_DB:-your_custom_db}"
```

Or set environment variables before running:

```bash
export POSTGRES_USER=myuser
export POSTGRES_PASSWORD=mypassword
./run.sh
```

## Project Structure

```
FYPhish/
├── Dockerfile.postgres       # PostgreSQL container definition
├── Dockerfile.fyphish        # FYPhish application container (for production)
├── docker/
│   ├── postgres/
│   │   └── init-databases.sh # Database initialization script
│   └── fyphish/
│       └── run.sh            # Container startup script
├── run.sh                    # 🎯 Main development script (USE THIS)
├── gophish.go                # Main application entry point
├── .env.example              # Environment configuration template
└── config.json               # Application configuration
```

## Development Workflow

### Daily Development

```bash
# Morning: Start everything
./run.sh

# Work on your code...
# The application runs with `go run` so code changes require restart

# Stop for lunch/break
# Ctrl+C to stop the app
# Database continues running in the background

# Resume work
./run.sh
# (PostgreSQL container is already running, just starts the app)

# End of day: Stop everything
./run.sh --stop
```

### Making Database Changes

```bash
# Database is persistent across restarts
# Data is stored in Docker volume: fyphish-postgres-data

# To reset database:
./run.sh --clean  # Warning: deletes all data!
./run.sh          # Fresh start
```

### Testing with Fresh Database

```bash
# Clean slate
./run.sh --clean

# Start fresh
./run.sh

# First-time setup will run automatically
```

## What's Running?

After `./run.sh` starts successfully:

1. **PostgreSQL Container** (fyphish-postgres-db)
   - Running in background
   - Listening on port 5432
   - Persists data in Docker volume
   - Auto-restarts unless stopped manually

2. **FYPhish Application** (foreground)
   - Running via `go run gophish.go`
   - Admin server on port 3333
   - Phishing server on port 8080
   - Connected to PostgreSQL at localhost:5432
   - Stops when you press Ctrl+C

## Getting Help

### View Application Logs

The FYPhish logs appear directly in your terminal when running `./run.sh`.

### View Database Logs

```bash
docker logs fyphish-postgres-db

# Follow logs in real-time
docker logs -f fyphish-postgres-db
```

### Access PostgreSQL Shell

```bash
# Connect to database
docker exec -it fyphish-postgres-db psql -U fyphish_user -d fyphish

# Useful commands in psql:
# \l              - List all databases
# \dt             - List all tables
# \d table_name   - Describe table structure
# \q              - Quit
```

### Check What's Running

```bash
# Docker containers
docker ps

# Go processes
ps aux | grep gophish
```

## Next Steps

1. **Read the main README**: [README.md](README.md) for full project documentation
2. **Review Microsoft SSO setup**: [.claude/CLAUDE.md](.claude/CLAUDE.md) for architecture details
3. **Configure SSO**: Follow the "Microsoft SSO Setup" section above
4. **Start developing**: The codebase is ready for your changes!

## Common Questions

**Q: Do I need to rebuild the container every time?**
A: No! The script only builds if the image doesn't exist. Use `--rebuild` only when Dockerfile.postgres changes.

**Q: Can I use a different database?**
A: For production, PostgreSQL is recommended. For development/testing, you can configure SQLite or MySQL in config.json.

**Q: How do I stop just the application but keep the database running?**
A: Press Ctrl+C to stop the Go application. The PostgreSQL container keeps running. Run `./run.sh` again to restart just the app.

**Q: Where is my data stored?**
A: PostgreSQL data is in Docker volume `fyphish-postgres-data`. It persists across restarts. Use `./run.sh --clean` to remove it.

**Q: Can I run this in production?**
A: This script is for development. For production, use `Dockerfile.fyphish` with Docker Compose or container orchestration (Kubernetes, Azure Container Apps, etc.).

## Support

- **Issues**: Report bugs on the GitHub repository
- **Documentation**: Check [.claude/CLAUDE.md](.claude/CLAUDE.md) for architecture
- **Azure Deployment**: See deployment notes in the main README

---

**Happy Phishing Simulation Testing! 🎣**
