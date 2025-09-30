#!/bin/bash

# Development startup script for FYPhish
# This script handles environment variable loading and config substitution

echo "🚀 Starting FYPhish (Development Mode)..."

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo "💡 Copy .env.example to .env and configure your settings"
    exit 1
fi

# Load environment variables and export them
echo "📁 Loading environment variables from .env..."
set -a
source .env
set +a

# Create processed config (don't overwrite original)
echo "⚙️  Processing configuration..."
envsubst < config.json > /tmp/fyphish-config.json

# Verify PostgreSQL connection string
if [ -z "$POSTGRES_CONNECTION_STRING" ]; then
    echo "❌ Error: POSTGRES_CONNECTION_STRING not set in .env"
    exit 1
fi

echo "✅ Configuration processed successfully"
echo "🎯 Starting FYPhish server..."

# Start FYPhish with processed config
exec ./fyphish --config /tmp/fyphish-config.json