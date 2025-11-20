package models

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetPublicBaseURL determines the public-facing base URL from various sources
// Priority: 1) Environment variable, 2) Campaign URL, 3) Request headers
func GetPublicBaseURL(r *http.Request, campaignURL string) string {
	// Priority 1: Environment variable (for production deployments)
	if envURL := os.Getenv("PUBLIC_BASE_URL"); envURL != "" {
		return strings.TrimSuffix(envURL, "/")
	}

	// Priority 2: Use campaign URL if it's not localhost
	if campaignURL != "" && !isLocalhost(campaignURL) {
		return strings.TrimSuffix(campaignURL, "/")
	}

	// Priority 3: Detect from request headers (for Cloudflare Tunnel, App Platform, etc.)
	if r != nil {
		return buildPublicURLFromRequest(r)
	}

	// Fallback: Use campaign URL even if localhost
	if campaignURL != "" {
		return strings.TrimSuffix(campaignURL, "/")
	}

	// Last resort
	return "http://localhost:3333"
}

// buildPublicURLFromRequest constructs the public URL from HTTP request headers
// Supports: Cloudflare Tunnel, DigitalOcean App Platform, Azure Container Apps
func buildPublicURLFromRequest(r *http.Request) string {
	protocol := "http"
	host := r.Host

	// Detect HTTPS from proxy headers
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto == "https" {
		protocol = "https"
	} else if r.Header.Get("X-Forwarded-Scheme") == "https" {
		protocol = "https"
	} else if r.TLS != nil {
		protocol = "https"
	}

	// Use forwarded host if available (Cloudflare, load balancers)
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	// Auto-detect HTTPS for known cloud platforms
	if strings.Contains(host, ".trycloudflare.com") ||
		strings.Contains(host, ".azurecontainerapps.io") ||
		strings.Contains(host, ".ondigitalocean.app") {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s", protocol, host)
}

// isLocalhost checks if a URL points to localhost
func isLocalhost(url string) bool {
	return strings.Contains(url, "localhost") ||
		strings.Contains(url, "127.0.0.1") ||
		strings.Contains(url, "0.0.0.0")
}

// GetPublicTrackingURL builds a complete phishing landing page URL with the recipient parameter
// This URL is used for click tracking ({{.URL}} placeholder)
func GetPublicTrackingURL(r *http.Request, campaignURL string, rid string) string {
	baseURL := GetPublicBaseURL(r, campaignURL)
	return fmt.Sprintf("%s?%s=%s", baseURL, RecipientParameter, rid)
}

// GetPublicTrackingPixelURL builds the tracking pixel URL for email open tracking
// This URL points to the /track endpoint ({{.Tracker}} placeholder)
func GetPublicTrackingPixelURL(r *http.Request, campaignURL string, rid string) string {
	baseURL := GetPublicBaseURL(r, campaignURL)
	return fmt.Sprintf("%s/track?%s=%s", baseURL, RecipientParameter, rid)
}
