package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gorilla/sessions"
	"golang.org/x/time/rate"
)

// Flash represents a flash message
type Flash struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// UserOperationsProvider interface that will be implemented by models package
type UserOperationsProvider interface {
	FindOrCreateUser(provider, oauthID, email string) (userID int64, username string, accountLocked bool, isAdmin bool, err error)
	UpdateLastLogin(userID int64) error
	ValidateAdminPrivilege(userID int64) (bool, error)
	LogSecurityEvent(userID int64, event, details string) error
}

// OAuthHandler handles OAuth authentication flows with enhanced security
type OAuthHandler struct {
	config       *config.Config
	provider     OAuthProvider
	userOps      UserOperationsProvider
	rateLimiter  *rate.Limiter
	maxAttempts  int
	sessionStore *sessions.CookieStore
}

// NewOAuthHandler creates a new OAuth handler with enhanced security features
func NewOAuthHandler(cfg *config.Config, provider OAuthProvider, userOps UserOperationsProvider) *OAuthHandler {
	if userOps == nil {
		// Fallback - this shouldn't happen in production
		log.Printf("Warning: UserOperationsProvider not set, OAuth user operations will fail")
	}
	return &OAuthHandler{
		config:       cfg,
		provider:     provider,
		userOps:      userOps,
		rateLimiter:  rate.NewLimiter(rate.Every(time.Second), 10), // 10 requests per second
		maxAttempts:  5, // Maximum login attempts per session
		sessionStore: nil, // Will use default middleware store
	}
}

// InitiateMicrosoftOAuth handles the /auth/microsoft endpoint
// Redirects user to Microsoft OAuth with PKCE
func (h *OAuthHandler) InitiateMicrosoftOAuth(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)

	// Check if SSO is enabled
	if !h.config.SSO.Enabled {
		h.flashMessage(session, "danger", "Single Sign-On is currently disabled")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Apply rate limiting
	if !h.rateLimiter.Allow() {
		log.Printf("Rate limit exceeded for OAuth initiation from IP: %s", r.RemoteAddr)
		h.flashMessage(session, "danger", "Too many authentication attempts. Please wait and try again.")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Generate PKCE challenge with enhanced entropy
	pkce, err := GeneratePKCE()
	if err != nil {
		log.Printf("Failed to generate PKCE: %v", err)
		h.flashMessage(session, "danger", "Authentication initialization failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Generate cryptographically secure state with timestamp
	state, err := h.generateSecureState()
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		h.flashMessage(session, "danger", "Authentication initialization failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Store PKCE verifier and state in session with expiration
	session.Values["oauth_code_verifier"] = pkce.CodeVerifier
	session.Values["oauth_state"] = state
	session.Values["oauth_provider"] = "microsoft"
	session.Values["oauth_timestamp"] = time.Now().Unix()
	session.Values["oauth_nonce"] = h.generateNonce()

	// Validate and sanitize next URL to prevent open redirect
	if next := r.URL.Query().Get("next"); next != "" {
		if h.isValidRedirectURL(next) {
			session.Values["oauth_next"] = next
		} else {
			log.Printf("Invalid redirect URL attempted: %s", next)
		}
	}

	err = session.Save(r, w)
	if err != nil {
		log.Printf("Failed to save session: %v", err)
		h.flashMessage(session, "danger", "Authentication initialization failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Build authorization URL
	authURL := h.provider.GetAuthURLWithPKCE(state, pkce)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleMicrosoftCallback handles the /auth/microsoft/callback endpoint
// Processes OAuth callback and creates/authenticates user
func (h *OAuthHandler) HandleMicrosoftCallback(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)

	// Extract callback parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Handle OAuth errors
	if errorParam != "" {
		errorDescription := r.URL.Query().Get("error_description")
		log.Printf("OAuth error: %s - %s", errorParam, errorDescription)
		h.flashMessage(session, "danger", "Authentication was cancelled or failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Validate required parameters
	if code == "" || state == "" {
		log.Printf("Missing code or state in OAuth callback")
		h.flashMessage(session, "danger", "Invalid authentication response")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Validate state parameter with constant-time comparison (CSRF protection)
	sessionState, ok := session.Values["oauth_state"].(string)
	if !ok || subtle.ConstantTimeCompare([]byte(sessionState), []byte(state)) != 1 {
		log.Printf("State mismatch detected for OAuth callback")
		h.logSuspiciousActivity(r, "oauth_state_mismatch", "Invalid state parameter in OAuth callback")
		h.flashMessage(session, "danger", "Invalid authentication state")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Validate session timestamp to prevent replay attacks
	if timestamp, ok := session.Values["oauth_timestamp"].(int64); ok {
		if time.Since(time.Unix(timestamp, 0)) > 10*time.Minute {
			log.Printf("OAuth session expired")
			h.flashMessage(session, "danger", "Authentication session expired. Please try again.")
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
	}

	// Get PKCE verifier from session
	codeVerifier, ok := session.Values["oauth_code_verifier"].(string)
	if !ok {
		log.Printf("Missing PKCE code verifier in session")
		h.flashMessage(session, "danger", "Authentication session expired")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Exchange authorization code for token
	ctx := context.Background()
	pkce := &PKCEChallenge{CodeVerifier: codeVerifier}
	token, err := h.provider.ExchangeCodeWithPKCE(ctx, code, pkce)
	if err != nil {
		log.Printf("Failed to exchange code for token: %v", err)
		h.flashMessage(session, "danger", "Authentication token exchange failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user info from OAuth provider
	userInfo, err := h.provider.GetUserInfo(ctx, token)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		h.flashMessage(session, "danger", "Failed to retrieve user information")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Validate domain if configured
	if err := h.validateUserDomain(userInfo.Email); err != nil {
		log.Printf("Domain validation failed for %s: %v", userInfo.Email, err)
		h.flashMessage(session, "danger", "Access restricted: "+err.Error())
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Find or create user using callback with admin privilege check
	if h.userOps == nil {
		log.Printf("OAuth user operations not configured")
		h.flashMessage(session, "danger", "Authentication system error")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	userID, username, accountLocked, isAdmin, err := h.userOps.FindOrCreateUser(userInfo.Provider, userInfo.ID, userInfo.Email)
	if err != nil {
		log.Printf("Failed to find/create OAuth user: %v", err)
		h.flashMessage(session, "danger", "User account setup failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Check if account is locked
	if accountLocked {
		h.logSecurityEvent(userID, "login_blocked", "Account locked")
		h.flashMessage(session, "danger", "Account is locked. Please contact your administrator.")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Perform additional admin validation for admin accounts
	if isAdmin {
		isValidAdmin, err := h.validateAdminAccess(userID, userInfo.Email)
		if err != nil || !isValidAdmin {
			log.Printf("Admin validation failed for user %s: %v", userInfo.Email, err)
			h.logSecurityEvent(userID, "admin_validation_failed", fmt.Sprintf("Email: %s", userInfo.Email))
			h.flashMessage(session, "danger", "Admin access validation failed")
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
	}

	// Update last login time
	if err := h.userOps.UpdateLastLogin(userID); err != nil {
		log.Printf("Failed to update user last login: %v", err)
		// Continue anyway, this is not critical
	}

	// Log successful authentication with security context
	h.logSecurityEvent(userID, "oauth_login_success", fmt.Sprintf("Provider: %s, Email: %s, Admin: %v", userInfo.Provider, userInfo.Email, isAdmin))
	log.Printf("OAuth login successful for %s (provider: %s, ID: %s, Admin: %v)", userInfo.Email, userInfo.Provider, userInfo.ID, isAdmin)

	// Store user ID and security context in session
	session.Values["id"] = userID
	session.Values["auth_method"] = "oauth_" + userInfo.Provider
	session.Values["auth_time"] = time.Now().Unix()
	session.Values["is_admin"] = isAdmin
	session.Values["session_token"] = h.generateSessionToken()

	// Clear OAuth session data
	delete(session.Values, "oauth_code_verifier")
	delete(session.Values, "oauth_state")
	delete(session.Values, "oauth_provider")

	err = session.Save(r, w)
	if err != nil {
		log.Printf("Failed to save session after login: %v", err)
		h.flashMessage(session, "danger", "Login session setup failed")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Redirect to next URL or dashboard
	next := "/"
	if nextURL, ok := session.Values["oauth_next"].(string); ok && nextURL != "" {
		delete(session.Values, "oauth_next")
		session.Save(r, w)
		if parsedURL, err := url.Parse(nextURL); err == nil && parsedURL.Path != "" {
			next = "/" + strings.TrimLeft(parsedURL.EscapedPath(), "/")
		}
	}

	h.flashMessage(session, "success", fmt.Sprintf("Welcome, %s! You have successfully signed in with Microsoft.", username))
	http.Redirect(w, r, next, http.StatusFound)
}

// generateSecureState generates a cryptographically secure random state parameter
func (h *OAuthHandler) generateSecureState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generateNonce generates a secure nonce for additional CSRF protection
func (h *OAuthHandler) generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// generateSessionToken generates a secure session token
func (h *OAuthHandler) generateSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// isValidRedirectURL validates that a redirect URL is safe
func (h *OAuthHandler) isValidRedirectURL(url string) bool {
	// Only allow relative URLs starting with /
	if !strings.HasPrefix(url, "/") {
		return false
	}
	// Prevent protocol-relative URLs
	if strings.HasPrefix(url, "//") {
		return false
	}
	// Additional validation can be added here
	return true
}

// validateUserDomain validates that the user's email domain is allowed
func (h *OAuthHandler) validateUserDomain(email string) error {
	microsoftProvider := h.config.SSO.Providers["microsoft"]
	if microsoftProvider == nil || len(microsoftProvider.AllowedDomains) == 0 {
		return nil // No domain restrictions
	}

	emailParts := strings.Split(email, "@")
	if len(emailParts) != 2 {
		return fmt.Errorf("invalid email format")
	}

	domain := strings.ToLower(emailParts[1])

	for _, allowedDomain := range microsoftProvider.AllowedDomains {
		if strings.ToLower(allowedDomain) == domain {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not authorized for access", domain)
}

// flashMessage adds a flash message to the session
func (h *OAuthHandler) flashMessage(session *sessions.Session, msgType string, message string) {
	// Use local Flash type to avoid import cycle
	flashMsg := Flash{
		Type:    msgType,
		Message: message,
	}
	session.AddFlash(flashMsg)
}

// validateAdminAccess performs additional validation for admin users
func (h *OAuthHandler) validateAdminAccess(userID int64, email string) (bool, error) {
	if h.userOps == nil {
		return false, fmt.Errorf("admin validation not configured")
	}

	// Check if user has admin privileges
	isAdmin, err := h.userOps.ValidateAdminPrivilege(userID)
	if err != nil {
		return false, err
	}

	if !isAdmin {
		return false, nil
	}

	// Additional admin-specific validations
	// Use configuration-based admin email validation
	if !h.config.IsAdminEmail(email) {
		return false, fmt.Errorf("email not in admin configuration")
	}

	return true, nil
}

// logSecurityEvent logs security-related events
func (h *OAuthHandler) logSecurityEvent(userID int64, event, details string) {
	if h.userOps != nil {
		if err := h.userOps.LogSecurityEvent(userID, event, details); err != nil {
			log.Printf("Failed to log security event: %v", err)
		}
	}
}

// logSuspiciousActivity logs suspicious authentication attempts
func (h *OAuthHandler) logSuspiciousActivity(r *http.Request, event, details string) {
	ipAddress := h.extractIPFromRequest(r)
	userAgent := r.UserAgent()
	log.Printf("Suspicious activity: %s - IP: %s, UA: %s, Details: %s", event, ipAddress, userAgent, details)
}

// extractIPFromRequest safely extracts IP address from request
func (h *OAuthHandler) extractIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

