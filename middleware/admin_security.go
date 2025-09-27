package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/sessions"
	"golang.org/x/time/rate"
)

// AdminSecurityConfig holds configuration for admin security middleware
type AdminSecurityConfig struct {
	RequireEmailAuthorization bool          `json:"require_email_authorization"`
	SessionTimeout           time.Duration `json:"session_timeout"`
	MaxFailedAttempts        int           `json:"max_failed_attempts"`
	LockoutDuration          time.Duration `json:"lockout_duration"`
	RequireMFA               bool          `json:"require_mfa"`
	IPWhitelist              []string      `json:"ip_whitelist"`
	EnforceSessionBinding    bool          `json:"enforce_session_binding"`
}

// DefaultAdminSecurityConfig returns default admin security configuration
func DefaultAdminSecurityConfig() *AdminSecurityConfig {
	return &AdminSecurityConfig{
		RequireEmailAuthorization: true,
		SessionTimeout:           30 * time.Minute,
		MaxFailedAttempts:        3,
		LockoutDuration:          15 * time.Minute,
		RequireMFA:               false, // Can be enabled when MFA is implemented
		IPWhitelist:              []string{},
		EnforceSessionBinding:    true,
	}
}

// AdminSessionManager manages admin sessions with enhanced security
type AdminSessionManager struct {
	config         *AdminSecurityConfig
	sessions       map[string]*AdminSession
	mu             sync.RWMutex
	rateLimiter    *rate.Limiter
	failedAttempts map[string]int
	lockouts       map[string]time.Time
}

// AdminSession represents an admin session with security context
type AdminSession struct {
	ID            string
	UserID        int64
	Username      string
	IPAddress     string
	UserAgent     string
	CreatedAt     time.Time
	LastActivity  time.Time
	SessionToken  string
	IsValid       bool
	AuthMethod    string // "oauth_microsoft", "local", etc.
}

var adminSessionManager *AdminSessionManager

func init() {
	adminSessionManager = NewAdminSessionManager(DefaultAdminSecurityConfig())
}

// NewAdminSessionManager creates a new admin session manager
func NewAdminSessionManager(config *AdminSecurityConfig) *AdminSessionManager {
	return &AdminSessionManager{
		config:         config,
		sessions:       make(map[string]*AdminSession),
		rateLimiter:    rate.NewLimiter(rate.Every(time.Second), 10),
		failedAttempts: make(map[string]int),
		lockouts:       make(map[string]time.Time),
	}
}

// RequireAdminPrivileges middleware that enforces admin-only access with enhanced security
func RequireAdminPrivileges(config *AdminSecurityConfig) func(http.Handler) http.HandlerFunc {
	if config == nil {
		config = DefaultAdminSecurityConfig()
	}

	return func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			user := ctx.Get(r, "user")
			if user == nil {
				JSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			currentUser, ok := user.(models.User)
			if !ok {
				JSONError(w, http.StatusInternalServerError, "Invalid user context")
				return
			}

			// Check if user has admin role
			if currentUser.Role.Slug != models.RoleAdmin {
				log.Warnf("Non-admin user %s attempted to access admin resource", currentUser.Username)
				logAdminSecurityEvent(currentUser.Id, "admin_access_denied", fmt.Sprintf("Path: %s", r.URL.Path))
				JSONError(w, http.StatusForbidden, "Admin privileges required")
				return
			}

			// Validate admin session
			session := ctx.Get(r, "session").(*sessions.Session)
			if !validateAdminSession(session, r, &currentUser) {
				log.Warnf("Invalid admin session for user %s", currentUser.Username)
				logAdminSecurityEvent(currentUser.Id, "invalid_admin_session", "Session validation failed")

				// Clear session
				delete(session.Values, "id")
				session.Save(r, w)

				JSONError(w, http.StatusUnauthorized, "Invalid admin session. Please re-authenticate.")
				return
			}

			// Check email authorization for admin
			if config.RequireEmailAuthorization {
				service := models.NewEmailAuthorizationService()
				result, err := service.CheckEmailAuthorization(currentUser.Username)
				if err != nil || !result.Authorized || result.GetRole() != "admin" {
					log.Errorf("Admin email authorization failed for %s", currentUser.Username)
					logAdminSecurityEvent(currentUser.Id, "admin_email_auth_failed", currentUser.Username)
					JSONError(w, http.StatusForbidden, "Admin email authorization failed")
					return
				}
			}

			// Check IP whitelist if configured
			if len(config.IPWhitelist) > 0 {
				clientIP := models.ExtractIPFromRequest(r)
				if !isIPWhitelisted(clientIP, config.IPWhitelist) {
					log.Warnf("Admin access attempted from non-whitelisted IP: %s", clientIP)
					logAdminSecurityEvent(currentUser.Id, "admin_ip_blocked", fmt.Sprintf("IP: %s", clientIP))
					JSONError(w, http.StatusForbidden, "Access denied from this IP address")
					return
				}
			}

			// Update session activity
			updateAdminSessionActivity(session, r)

			// Log successful admin access
			logAdminSecurityEvent(currentUser.Id, "admin_access_granted", fmt.Sprintf("Path: %s", r.URL.Path))

			// Continue to next handler
			next.ServeHTTP(w, r)
		}
	}
}

// validateAdminSession validates an admin session with enhanced security checks
func validateAdminSession(session *sessions.Session, r *http.Request, user *models.User) bool {
	// Check if session has required security attributes
	sessionToken, ok := session.Values["session_token"].(string)
	if !ok || sessionToken == "" {
		return false
	}

	// Validate session age
	if authTime, ok := session.Values["auth_time"].(int64); ok {
		sessionAge := time.Since(time.Unix(authTime, 0))
		if sessionAge > adminSessionManager.config.SessionTimeout {
			log.Infof("Admin session expired for user %s", user.Username)
			return false
		}
	} else {
		return false // No auth time means invalid session
	}

	// Validate session binding (IP and User-Agent)
	if adminSessionManager.config.EnforceSessionBinding {
		if sessionIP, ok := session.Values["session_ip"].(string); ok {
			currentIP := models.ExtractIPFromRequest(r)
			if sessionIP != currentIP {
				log.Warnf("IP mismatch for admin session: expected %s, got %s", sessionIP, currentIP)
				return false
			}
		}

		if sessionUA, ok := session.Values["session_ua"].(string); ok {
			currentUA := r.UserAgent()
			if sessionUA != currentUA {
				log.Warnf("User-Agent mismatch for admin session")
				return false
			}
		}
	}

	// Check if admin flag is set
	if isAdmin, ok := session.Values["is_admin"].(bool); !ok || !isAdmin {
		return false
	}

	return true
}

// updateAdminSessionActivity updates the last activity time for an admin session
func updateAdminSessionActivity(session *sessions.Session, r *http.Request) {
	session.Values["last_activity"] = time.Now().Unix()
	session.Values["session_ip"] = models.ExtractIPFromRequest(r)
	session.Values["session_ua"] = r.UserAgent()
	session.Save(r, nil)
}

// CreateAdminSession creates a new secure admin session
func CreateAdminSession(userID int64, username, authMethod string, r *http.Request) (*AdminSession, error) {
	// Generate secure session token
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	sessionID, err := generateSecureToken(16)
	if err != nil {
		return nil, err
	}

	adminSession := &AdminSession{
		ID:           sessionID,
		UserID:       userID,
		Username:     username,
		IPAddress:    models.ExtractIPFromRequest(r),
		UserAgent:    r.UserAgent(),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		SessionToken: token,
		IsValid:      true,
		AuthMethod:   authMethod,
	}

	// Store in session manager
	adminSessionManager.mu.Lock()
	adminSessionManager.sessions[sessionID] = adminSession
	adminSessionManager.mu.Unlock()

	// Clean up old sessions
	go cleanupExpiredSessions()

	return adminSession, nil
}

// ValidateAdminToken validates an admin session token with constant-time comparison
func ValidateAdminToken(sessionID, token string) bool {
	adminSessionManager.mu.RLock()
	defer adminSessionManager.mu.RUnlock()

	session, exists := adminSessionManager.sessions[sessionID]
	if !exists || !session.IsValid {
		return false
	}

	// Check session expiry
	if time.Since(session.LastActivity) > adminSessionManager.config.SessionTimeout {
		session.IsValid = false
		return false
	}

	// Constant-time comparison for token
	return subtle.ConstantTimeCompare([]byte(session.SessionToken), []byte(token)) == 1
}

// InvalidateAdminSession invalidates an admin session
func InvalidateAdminSession(sessionID string) {
	adminSessionManager.mu.Lock()
	defer adminSessionManager.mu.Unlock()

	if session, exists := adminSessionManager.sessions[sessionID]; exists {
		session.IsValid = false
		delete(adminSessionManager.sessions, sessionID)
	}
}

// cleanupExpiredSessions removes expired admin sessions
func cleanupExpiredSessions() {
	adminSessionManager.mu.Lock()
	defer adminSessionManager.mu.Unlock()

	now := time.Now()
	for id, session := range adminSessionManager.sessions {
		if time.Since(session.LastActivity) > adminSessionManager.config.SessionTimeout || !session.IsValid {
			delete(adminSessionManager.sessions, id)
		}
	}

	// Clean up old lockouts
	for email, lockoutTime := range adminSessionManager.lockouts {
		if now.After(lockoutTime) {
			delete(adminSessionManager.lockouts, email)
			delete(adminSessionManager.failedAttempts, email)
		}
	}
}

// RecordFailedAdminAttempt records a failed admin authentication attempt
func RecordFailedAdminAttempt(email string) bool {
	adminSessionManager.mu.Lock()
	defer adminSessionManager.mu.Unlock()

	// Check if already locked out
	if lockoutTime, exists := adminSessionManager.lockouts[email]; exists {
		if time.Now().Before(lockoutTime) {
			return true // Still locked out
		}
		// Lockout expired, clear it
		delete(adminSessionManager.lockouts, email)
		delete(adminSessionManager.failedAttempts, email)
	}

	// Increment failed attempts
	adminSessionManager.failedAttempts[email]++

	// Check if should lock out
	if adminSessionManager.failedAttempts[email] >= adminSessionManager.config.MaxFailedAttempts {
		adminSessionManager.lockouts[email] = time.Now().Add(adminSessionManager.config.LockoutDuration)
		log.Warnf("Admin account %s locked out due to %d failed attempts", email, adminSessionManager.failedAttempts[email])
		return true
	}

	return false
}

// ClearFailedAttempts clears failed attempts for a successful login
func ClearFailedAttempts(email string) {
	adminSessionManager.mu.Lock()
	defer adminSessionManager.mu.Unlock()

	delete(adminSessionManager.failedAttempts, email)
	delete(adminSessionManager.lockouts, email)
}

// isIPWhitelisted checks if an IP is in the whitelist
func isIPWhitelisted(ip string, whitelist []string) bool {
	for _, allowedIP := range whitelist {
		if strings.Contains(allowedIP, "/") {
			// Handle CIDR notation if needed
			// For now, simple string comparison
			if ip == allowedIP {
				return true
			}
		} else if ip == allowedIP {
			return true
		}
	}
	return false
}

// generateSecureToken generates a cryptographically secure token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// logAdminSecurityEvent logs admin security events
func logAdminSecurityEvent(userID int64, event, details string) {
	service := models.NewEmailAuthorizationService()
	user, err := models.GetUser(userID)
	if err != nil {
		log.Errorf("Failed to get user for security logging: %v", err)
		return
	}

	ctxBg := context.Background()
	if err := service.LogAuthorizationAttempt(ctxBg, user.Username, event, "admin_security", &userID, details); err != nil {
		log.Errorf("Failed to log admin security event: %v", err)
	}
}

// EnforceAdminAPIKey ensures that admin API operations have valid API keys with admin privileges
func EnforceAdminAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This runs after RequireAPIKey, so user should be in context
		user := ctx.Get(r, "user")
		if user == nil {
			JSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		currentUser, ok := user.(models.User)
		if !ok {
			JSONError(w, http.StatusInternalServerError, "Invalid user context")
			return
		}

		// Check if user has admin role
		if currentUser.Role.Slug != models.RoleAdmin {
			log.Warnf("Non-admin API key used for admin operation by %s", currentUser.Username)
			logAdminSecurityEvent(currentUser.Id, "admin_api_denied", fmt.Sprintf("Path: %s", r.URL.Path))
			JSONError(w, http.StatusForbidden, "Admin API key required")
			return
		}

		// Additional validation for admin API operations
		service := models.NewEmailAuthorizationService()
		result, err := service.CheckEmailAuthorization(currentUser.Username)
		if err != nil || !result.Authorized || result.GetRole() != "admin" {
			log.Errorf("Admin API email authorization failed for %s", currentUser.Username)
			JSONError(w, http.StatusForbidden, "Admin email authorization required for API access")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetAdminSessionInfo returns information about the current admin session
func GetAdminSessionInfo(r *http.Request) map[string]interface{} {
	session := ctx.Get(r, "session").(*sessions.Session)

	info := make(map[string]interface{})
	info["is_admin"] = false

	if user := ctx.Get(r, "user"); user != nil {
		if currentUser, ok := user.(models.User); ok {
			info["is_admin"] = currentUser.Role.Slug == models.RoleAdmin
			info["username"] = currentUser.Username
			info["user_id"] = currentUser.Id

			if authMethod, ok := session.Values["auth_method"].(string); ok {
				info["auth_method"] = authMethod
			}

			if authTime, ok := session.Values["auth_time"].(int64); ok {
				info["session_age"] = time.Since(time.Unix(authTime, 0)).String()
			}
		}
	}

	return info
}