#!/bin/sh

# Load environment variables from .env file if it exists
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
fi

# Function to update JSON config safely
update_config() {
    local key="$1"
    local value="$2"
    local type="${3:-string}"

    if [ -n "${value}" ]; then
        if [ "$type" = "boolean" ]; then
            jq --argjson val "$value" "$key = \$val" config.json > config.json.tmp
        elif [ "$type" = "array" ]; then
            jq --arg val "$value" "$key = (\$val|split(\",\"))" config.json > config.json.tmp
        else
            jq --arg val "$value" "$key = \$val" config.json > config.json.tmp
        fi

        if [ $? -eq 0 ]; then
            mv config.json.tmp config.json
        else
            echo "Error updating $key, keeping original value"
            rm -f config.json.tmp
        fi
    fi
}

echo "Starting FYPhish configuration..."

# Substitute environment variables in config.json using envsubst
envsubst < config.json > config.json.tmp && mv config.json.tmp config.json

# Basic Gophish configuration
update_config '.admin_server.listen_url' "$ADMIN_LISTEN_URL"
update_config '.admin_server.use_tls' "$ADMIN_USE_TLS" "boolean"
update_config '.admin_server.cert_path' "$ADMIN_CERT_PATH"
update_config '.admin_server.key_path' "$ADMIN_KEY_PATH"
update_config '.admin_server.trusted_origins' "$ADMIN_TRUSTED_ORIGINS" "array"

update_config '.phish_server.listen_url' "$PHISH_LISTEN_URL"
update_config '.phish_server.use_tls' "$PHISH_USE_TLS" "boolean"
update_config '.phish_server.cert_path' "$PHISH_CERT_PATH"
update_config '.phish_server.key_path' "$PHISH_KEY_PATH"

update_config '.contact_address' "$CONTACT_ADDRESS"

# PostgreSQL is configured via POSTGRES_CONNECTION_STRING environment variable
# No need to update config.json for database settings

# FYPhish SSO Configuration
update_config '.sso.enabled' "$SSO_ENABLED" "boolean"
update_config '.sso.allow_local_login' "$ALLOW_LOCAL_LOGIN" "boolean"
update_config '.sso.hide_local_login' "$HIDE_LOCAL_LOGIN" "boolean"
update_config '.sso.emergency_access' "$EMERGENCY_ACCESS" "boolean"

# Microsoft OAuth Configuration
update_config '.sso.providers.microsoft.enabled' "$MICROSOFT_ENABLED" "boolean"
update_config '.sso.providers.microsoft.client_id' "$MICROSOFT_CLIENT_ID"
update_config '.sso.providers.microsoft.client_secret' "$MICROSOFT_CLIENT_SECRET"
update_config '.sso.providers.microsoft.tenant_id' "$MICROSOFT_TENANT_ID"

# Set Azure production environment
if [ "$GO_ENV" = "production" ]; then
    echo "Setting production configuration..."
    update_config '.admin_server.listen_url' "0.0.0.0:3333"
    # Use ADMIN_TRUSTED_ORIGINS if set, otherwise fall back to default
    if [ -n "$ADMIN_TRUSTED_ORIGINS" ]; then
        update_config '.admin_server.trusted_origins' "$ADMIN_TRUSTED_ORIGINS" "array"
    else
        update_config '.admin_server.trusted_origins' "https://vibephish.xyz,https://www.vibephish.xyz" "array"
    fi
fi

echo "Final runtime configuration:"
cat config.json | jq .

# Verify required environment variables for production
if [ "$GO_ENV" = "production" ]; then
    if [ -z "$SESSION_SIGNING_KEY" ] || [ -z "$SESSION_ENCRYPTION_KEY" ]; then
        echo "ERROR: SESSION_SIGNING_KEY and SESSION_ENCRYPTION_KEY are required for production"
        exit 1
    fi

    if [ -z "$MICROSOFT_CLIENT_ID" ] || [ -z "$MICROSOFT_CLIENT_SECRET" ]; then
        echo "ERROR: Microsoft OAuth credentials are required for production"
        exit 1
    fi
fi

echo "Starting FYPhish application..."

# Start the correct binary
exec ./fyphish