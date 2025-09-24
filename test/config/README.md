# Configuration Tests

This directory contains tests and utilities for verifying FYPhish SSO configuration.

## Files

### `test_sso_config.go`
**Purpose**: Validates that SSO configuration is loaded correctly from environment variables.

**Usage**:
```bash
cd /path/to/FYPhish
go run test/config/test_sso_config.go
```

**What it tests**:
- ✅ `.env` file loading
- ✅ SSO configuration parsing
- ✅ Environment variable override
- ✅ Microsoft OAuth provider setup
- ✅ Configuration validation

**Expected Output**:
```
🔧 Testing FYPhish SSO Configuration...
✅ Loaded .env file successfully
📋 Configuration Status:
  SSO Enabled: true
  Microsoft Provider Enabled: true
🔐 Microsoft OAuth Configuration:
  Client ID: 3572****5b76 (masked for security)
  Client Secret: o-a8****rcSy (masked for security)
✅ Configuration Valid!
```

## Prerequisites

1. Create `.env` file with your Azure AD credentials:
   ```bash
   MICROSOFT_CLIENT_ID=your-client-id
   MICROSOFT_CLIENT_SECRET=your-client-secret
   MICROSOFT_TENANT_ID=your-tenant-id
   ```

2. Ensure `config-test.json` exists with SSO configuration enabled

## Security Notes

- Secrets are automatically masked in test output
- Never commit `.env` files to git
- Test files help verify environment setup across different deployments