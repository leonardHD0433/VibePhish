package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	log "github.com/gophish/gophish/logger"
)

// SSOProvider represents a single OAuth provider configuration
type SSOProvider struct {
	Enabled        bool     `json:"enabled"`
	ClientID       string   `json:"client_id"`
	ClientSecret   string   `json:"client_secret"`
	TenantID       string   `json:"tenant_id,omitempty"`
	AllowedDomains []string `json:"allowed_domains"`
	AdminDomains   []string `json:"admin_domains"`
	DefaultRole    string   `json:"default_role"`
}

// SSOConfig represents the SSO configuration
type SSOConfig struct {
	Enabled          bool                    `json:"enabled"`
	AllowLocalLogin  bool                    `json:"allow_local_login"`
	Providers        map[string]*SSOProvider `json:"providers"`
}

// GetSSOConfig returns the SSO configuration with safe defaults
func (c *Config) GetSSOConfig() *SSOConfig {
	if c.SSO != nil {
		return c.SSO
	}
	// Return safe defaults if SSO not configured
	return &SSOConfig{
		Enabled:         false,
		AllowLocalLogin: true,
		Providers:       make(map[string]*SSOProvider),
	}
}

// IsSSOEnabled returns true if SSO is enabled and configured
func (c *Config) IsSSOEnabled() bool {
	sso := c.GetSSOConfig()
	return sso.Enabled
}

// IsProviderEnabled checks if a specific provider is enabled
func (c *Config) IsProviderEnabled(provider string) bool {
	sso := c.GetSSOConfig()
	if !sso.Enabled {
		return false
	}

	p, exists := sso.Providers[provider]
	return exists && p.Enabled && p.ClientID != ""
}

// LoadSecretsFromEnv populates OAuth secrets from environment variables
// This allows keeping secrets out of config files while maintaining flexibility
// It automatically tries to load .env file if present
func (c *Config) LoadSecretsFromEnv() {
	// Try to load .env file automatically (fail silently if not found)
	c.loadDotEnv()

	if c.SSO == nil || c.SSO.Providers == nil {
		return
	}

	// Load Microsoft OAuth secrets from environment
	if ms := c.SSO.Providers["microsoft"]; ms != nil {
		if clientID := os.Getenv("MICROSOFT_CLIENT_ID"); clientID != "" {
			ms.ClientID = clientID
		}
		if clientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET"); clientSecret != "" {
			ms.ClientSecret = clientSecret
		}
		if tenantID := os.Getenv("MICROSOFT_TENANT_ID"); tenantID != "" {
			ms.TenantID = tenantID
		}
	}

	// Future: Add Google OAuth secrets
	if google := c.SSO.Providers["google"]; google != nil {
		if clientID := os.Getenv("GOOGLE_CLIENT_ID"); clientID != "" {
			google.ClientID = clientID
		}
		if clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET"); clientSecret != "" {
			google.ClientSecret = clientSecret
		}
	}
}

// loadDotEnv attempts to load .env file if it exists
// Logs warnings but doesn't fail if .env is missing (production might use other methods)
func (c *Config) loadDotEnv() {
	if err := godotenv.Load(); err != nil {
		// Only log if the file exists but has issues
		if !os.IsNotExist(err) {
			log.Warn("Could not load .env file: ", err)
		}
		// Silently continue if .env doesn't exist (normal in production)
	}
}

// ValidateOAuthConfig checks if OAuth configuration is complete
func (c *Config) ValidateOAuthConfig(provider string) error {
	sso := c.GetSSOConfig()
	if !sso.Enabled {
		return nil // SSO disabled, no validation needed
	}

	p, exists := sso.Providers[provider]
	if !exists || !p.Enabled {
		return nil // Provider disabled, no validation needed
	}

	if p.ClientID == "" {
		return fmt.Errorf("OAuth provider '%s': client_id is required", provider)
	}
	if p.ClientSecret == "" {
		return fmt.Errorf("OAuth provider '%s': client_secret is required", provider)
	}

	return nil
}

// GetEffectiveProvider returns provider config with environment variables applied
func (c *Config) GetEffectiveProvider(provider string) *SSOProvider {
	sso := c.GetSSOConfig()
	if !sso.Enabled {
		return nil
	}

	p, exists := sso.Providers[provider]
	if !exists || !p.Enabled {
		return nil
	}

	// Create a copy to avoid modifying original config
	effective := &SSOProvider{
		Enabled:        p.Enabled,
		ClientID:       p.ClientID,
		ClientSecret:   p.ClientSecret,
		TenantID:       p.TenantID,
		AllowedDomains: p.AllowedDomains,
		AdminDomains:   p.AdminDomains,
		DefaultRole:    p.DefaultRole,
	}

	// Override with environment variables if present
	switch provider {
	case "microsoft":
		if clientID := os.Getenv("MICROSOFT_CLIENT_ID"); clientID != "" {
			effective.ClientID = clientID
		}
		if clientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET"); clientSecret != "" {
			effective.ClientSecret = clientSecret
		}
		if tenantID := os.Getenv("MICROSOFT_TENANT_ID"); tenantID != "" {
			effective.TenantID = tenantID
		}
	}

	return effective
}