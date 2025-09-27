package middleware

import (
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gophish/gophish/models"
	log "github.com/gophish/gophish/logger"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
)

// SSOSessionContext contains SSO-specific session information
type SSOSessionContext struct {
	AuthMethod    string    `json:"auth_method"`    // oauth_microsoft, local, etc.
	AuthTime      time.Time `json:"auth_time"`      // When authentication occurred
	IsAdmin       bool      `json:"is_admin"`       // Whether user has admin privileges
	SessionToken  string    `json:"session_token"`  // Secure session token for SSO
	Provider      string    `json:"provider"`       // OAuth provider name
	LastActivity  time.Time `json:"last_activity"`  // Last activity timestamp
	SessionExpiry time.Time `json:"session_expiry"` // When session expires
}

// init registers the necessary models to be saved in the session later
func init() {
	gob.Register(&models.User{})
	gob.Register(&models.Flash{})
	gob.Register(&SSOSessionContext{})
	Store.Options.HttpOnly = true
	Store.Options.Secure = false     // HTTP only (not HTTPS) - should be true in production
	Store.Options.SameSite = http.SameSiteLaxMode  // Allow cross-origin
	Store.Options.Path = "/"         // Root path
	// This sets the maxAge to 5 days for all cookies (shorter for admin sessions)
	Store.MaxAge(86400 * 5)
}

// Store contains the session information for the request
var Store = initializeSessionStore()

// initializeSessionStore creates a session store with production-grade security
func initializeSessionStore() *sessions.CookieStore {
	signingKey, encryptionKey := getSecureSessionKeys()
	store := sessions.NewCookieStore(signingKey, encryptionKey)

	// Configure secure session options
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 5, // 5 days
		HttpOnly: true,
		Secure:   isProductionMode(), // true for HTTPS in production
		SameSite: http.SameSiteLaxMode,
	}

	return store
}

// getSecureSessionKeys returns cryptographically secure session keys
func getSecureSessionKeys() ([]byte, []byte) {
	// Load environment variables from .env file (ignore errors in production)
	_ = godotenv.Load()

	// Try to load from environment variables first (production)
	if signingKeyHex := os.Getenv("SESSION_SIGNING_KEY"); signingKeyHex != "" {
		if encryptionKeyHex := os.Getenv("SESSION_ENCRYPTION_KEY"); encryptionKeyHex != "" {
			signingKey, err1 := hex.DecodeString(signingKeyHex)
			encryptionKey, err2 := hex.DecodeString(encryptionKeyHex)

			if err1 == nil && err2 == nil && len(signingKey) == 64 && len(encryptionKey) == 32 {
				log.Info("Using session keys from environment variables")
				return signingKey, encryptionKey
			} else {
				log.Warn("Invalid session keys in environment variables, using fallback")
			}
		}
	}

	// Development fallback - consistent keys for local development
	if !isProductionMode() {
		log.Warn("Using development session keys - NOT SECURE FOR PRODUCTION")
		return getDevelopmentKeys()
	}

	// Production without env vars - generate and warn
	log.Error("SECURITY WARNING: Generating random session keys. Sessions will not persist across restarts!")
	log.Error("Please set SESSION_SIGNING_KEY and SESSION_ENCRYPTION_KEY environment variables")
	return generateSecureKeys()
}

// getDevelopmentKeys returns consistent keys for development
func getDevelopmentKeys() ([]byte, []byte) {
	// Static keys for development (64 bytes signing, 32 bytes encryption)
	signingKey := []byte("fyPhish-development-signing-key-must-be-exactly-64-chars-long!!")
	encryptionKey := []byte("fyPhish-dev-encryption-32-chars!")
	return signingKey, encryptionKey
}

// generateSecureKeys generates cryptographically secure random keys
func generateSecureKeys() ([]byte, []byte) {
	signingKey := make([]byte, 64)
	encryptionKey := make([]byte, 32)

	if _, err := rand.Read(signingKey); err != nil {
		panic(fmt.Sprintf("Failed to generate signing key: %v", err))
	}
	if _, err := rand.Read(encryptionKey); err != nil {
		panic(fmt.Sprintf("Failed to generate encryption key: %v", err))
	}

	return signingKey, encryptionKey
}

// isProductionMode detects if running in production
func isProductionMode() bool {
	env := os.Getenv("GO_ENV")
	return env == "production" || env == "prod"
}

// CreateSSOSession creates an SSO session context
func CreateSSOSession(authMethod, provider string, isAdmin bool) *SSOSessionContext {
	now := time.Now()

	// Admin sessions expire sooner for security
	sessionDuration := 8 * time.Hour
	if isAdmin {
		sessionDuration = 4 * time.Hour
	}

	return &SSOSessionContext{
		AuthMethod:    authMethod,
		AuthTime:      now,
		IsAdmin:       isAdmin,
		SessionToken:  generateSessionToken(),
		Provider:      provider,
		LastActivity:  now,
		SessionExpiry: now.Add(sessionDuration),
	}
}

// generateSessionToken generates a cryptographically secure session token
func generateSessionToken() string {
	key := securecookie.GenerateRandomKey(32)
	// Convert bytes to hex string
	return hex.EncodeToString(key[:16])
}

// ValidateSession validates SSO session and updates activity
func ValidateSession(session *sessions.Session) bool {
	ssoCtx, ok := session.Values["sso_context"].(*SSOSessionContext)
	if !ok || ssoCtx == nil {
		return true // Non-SSO session, let regular validation handle
	}

	now := time.Now()

	// Check if session has expired
	if now.After(ssoCtx.SessionExpiry) {
		return false
	}

	// Update last activity
	ssoCtx.LastActivity = now
	session.Values["sso_context"] = ssoCtx

	return true
}

// IsAdminSession checks if the current session is for an admin user
func IsAdminSession(session *sessions.Session) bool {
	ssoCtx, ok := session.Values["sso_context"].(*SSOSessionContext)
	if ok && ssoCtx != nil {
		return ssoCtx.IsAdmin
	}
	return false
}

// GetAuthMethod returns the authentication method used for the session
func GetAuthMethod(session *sessions.Session) string {
	ssoCtx, ok := session.Values["sso_context"].(*SSOSessionContext)
	if ok && ssoCtx != nil {
		return ssoCtx.AuthMethod
	}
	return "local"
}
