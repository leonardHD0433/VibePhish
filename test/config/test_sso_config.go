package main

import (
	"fmt"
	"os"

	"github.com/gophish/gophish/config"
)

func main() {
	fmt.Println("🔧 Testing FYPhish SSO Configuration...")
	fmt.Println()

	// Load config from file with automatic SSO secret loading
	conf, err := config.LoadConfigWithSSO("config-test.json")
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Configuration loaded with SSO support")

	// Test SSO configuration
	fmt.Println("📋 Configuration Status:")
	fmt.Printf("  SSO Enabled: %v\n", conf.IsSSOEnabled())
	fmt.Printf("  Allow Local Login: %v\n", conf.GetSSOConfig().AllowLocalLogin)
	fmt.Printf("  Microsoft Provider Enabled: %v\n", conf.IsProviderEnabled("microsoft"))
	fmt.Println()

	// Test Microsoft provider configuration
	if msProvider := conf.GetEffectiveProvider("microsoft"); msProvider != nil {
		fmt.Println("🔐 Microsoft OAuth Configuration:")
		fmt.Printf("  Client ID: %s\n", maskSecret(msProvider.ClientID))
		fmt.Printf("  Client Secret: %s\n", maskSecret(msProvider.ClientSecret))
		fmt.Printf("  Tenant ID: %s\n", msProvider.TenantID)
		fmt.Printf("  Default Role: %s\n", msProvider.DefaultRole)
		fmt.Println()

		// Validate configuration
		if err := conf.ValidateOAuthConfig("microsoft"); err != nil {
			fmt.Printf("❌ Configuration Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Configuration Valid!")
		fmt.Println("🚀 Ready to build OAuth handlers!")
	} else {
		fmt.Println("❌ Microsoft provider not found or disabled")
		os.Exit(1)
	}
}

// maskSecret shows first 4 and last 4 characters of a secret
func maskSecret(secret string) string {
	if len(secret) == 0 {
		return "❌ EMPTY"
	}
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}