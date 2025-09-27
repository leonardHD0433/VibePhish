package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
)

// AdminCSRFConfig holds configuration for admin CSRF protection
type AdminCSRFConfig struct {
	TokenLength      int
	TokenExpiry      time.Duration
	SameSiteMode     http.SameSite
	RequireHTTPS     bool
	DoubleSubmit     bool // Enable double-submit cookie pattern
	HeaderValidation bool // Validate Origin/Referer headers
}

// DefaultAdminCSRFConfig returns default admin CSRF configuration
func DefaultAdminCSRFConfig() *AdminCSRFConfig {
	return &AdminCSRFConfig{
		TokenLength:      32,
		TokenExpiry:      4 * time.Hour, // Shorter expiry for admin operations
		SameSiteMode:     http.SameSiteStrictMode,
		RequireHTTPS:     false, // Set to true in production
		DoubleSubmit:     true,
		HeaderValidation: true,
	}
}

// EnforceAdminCSRF provides enhanced CSRF protection for admin operations
func EnforceAdminCSRF(config *AdminCSRFConfig) func(http.Handler) http.HandlerFunc {
	if config == nil {
		config = DefaultAdminCSRFConfig()
	}

	return func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF for safe methods
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user is admin
			user := ctx.Get(r, "user")
			if user == nil {
				JSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			currentUser, ok := user.(models.User)
			if !ok || currentUser.Role.Slug != models.RoleAdmin {
				// Not an admin, let regular CSRF handle it
				next.ServeHTTP(w, r)
				return
			}

			// Validate CSRF token for admin operations
			session := ctx.Get(r, "session").(*sessions.Session)

			// Check double-submit cookie pattern if enabled
			if config.DoubleSubmit {
				if !validateDoubleSubmitToken(r, session) {
					log.Warnf("Admin CSRF double-submit validation failed for user %s", currentUser.Username)
					logAdminCSRFViolation(currentUser.Id, r)
					JSONError(w, http.StatusForbidden, "CSRF token validation failed")
					return
				}
			}

			// Validate Origin/Referer headers
			if config.HeaderValidation {
				if !validateRequestHeaders(r, config.RequireHTTPS) {
					log.Warnf("Admin CSRF header validation failed for user %s", currentUser.Username)
					logAdminCSRFViolation(currentUser.Id, r)
					JSONError(w, http.StatusForbidden, "Request origin validation failed")
					return
				}
			}

			// Generate new CSRF token for next request
			newToken := generateAdminCSRFToken(config.TokenLength)
			session.Values["admin_csrf_token"] = newToken
			session.Values["admin_csrf_time"] = time.Now().Unix()
			session.Save(r, w)

			// Set CSRF cookie for double-submit
			if config.DoubleSubmit {
				setCSRFCookie(w, newToken, config)
			}

			next.ServeHTTP(w, r)
		}
	}
}

// validateDoubleSubmitToken validates the double-submit CSRF token pattern
func validateDoubleSubmitToken(r *http.Request, session *sessions.Session) bool {
	// Get token from session
	sessionToken, ok := session.Values["admin_csrf_token"].(string)
	if !ok || sessionToken == "" {
		return false
	}

	// Check token expiry
	if tokenTime, ok := session.Values["admin_csrf_time"].(int64); ok {
		if time.Since(time.Unix(tokenTime, 0)) > 4*time.Hour {
			return false
		}
	}

	// Get token from request (header or form)
	requestToken := r.Header.Get("X-Admin-CSRF-Token")
	if requestToken == "" {
		requestToken = r.FormValue("admin_csrf_token")
	}

	// Get token from cookie
	cookie, err := r.Cookie("admin_csrf_token")
	if err != nil || cookie.Value == "" {
		return false
	}

	// Validate all tokens match with constant-time comparison
	if subtle.ConstantTimeCompare([]byte(sessionToken), []byte(requestToken)) != 1 {
		return false
	}

	if subtle.ConstantTimeCompare([]byte(sessionToken), []byte(cookie.Value)) != 1 {
		return false
	}

	return true
}

// validateRequestHeaders validates Origin and Referer headers
func validateRequestHeaders(r *http.Request, requireHTTPS bool) bool {
	// Get the host from the request
	host := r.Host
	if host == "" {
		return false
	}

	// Build expected origin
	scheme := "http"
	if r.TLS != nil || requireHTTPS {
		scheme = "https"
	}
	expectedOrigin := fmt.Sprintf("%s://%s", scheme, host)

	// Check Origin header
	origin := r.Header.Get("Origin")
	if origin != "" {
		// Strict origin check for admin operations
		if origin != expectedOrigin {
			log.Warnf("Origin mismatch: expected %s, got %s", expectedOrigin, origin)
			return false
		}
	}

	// Check Referer header
	referer := r.Header.Get("Referer")
	if referer != "" {
		// Referer should start with our expected origin
		if !strings.HasPrefix(referer, expectedOrigin) {
			log.Warnf("Referer mismatch: expected prefix %s, got %s", expectedOrigin, referer)
			return false
		}
	}

	// At least one header should be present for state-changing operations
	if origin == "" && referer == "" {
		return false
	}

	return true
}

// generateAdminCSRFToken generates a secure CSRF token
func generateAdminCSRFToken(length int) string {
	token := make([]byte, length)
	if _, err := rand.Read(token); err != nil {
		// Fallback to Gorilla's CSRF token generation
		return csrf.Token(nil)
	}
	return hex.EncodeToString(token)
}

// setCSRFCookie sets the CSRF cookie for double-submit pattern
func setCSRFCookie(w http.ResponseWriter, token string, config *AdminCSRFConfig) {
	cookie := &http.Cookie{
		Name:     "admin_csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Must be readable by JavaScript for AJAX requests
		Secure:   config.RequireHTTPS,
		SameSite: config.SameSiteMode,
		MaxAge:   int(config.TokenExpiry.Seconds()),
	}
	http.SetCookie(w, cookie)
}

// logAdminCSRFViolation logs potential CSRF attempts on admin endpoints
func logAdminCSRFViolation(userID int64, r *http.Request) {
	ipAddress := models.ExtractIPFromRequest(r)
	userAgent := r.UserAgent()
	details := fmt.Sprintf("CSRF violation - Path: %s, Method: %s, IP: %s, UA: %s",
		r.URL.Path, r.Method, ipAddress, userAgent)

	service := models.NewEmailAuthorizationService()
	user, err := models.GetUser(userID)
	if err != nil {
		log.Errorf("Failed to get user for CSRF logging: %v", err)
		return
	}

	ctxBg := context.Background()
	service.LogAuthorizationAttempt(ctxBg, user.Username, "csrf_violation", "blocked", &userID, details)
	log.Warnf("Admin CSRF violation: %s", details)
}

// GenerateAdminCSRFToken generates a CSRF token for templates
func GenerateAdminCSRFToken(r *http.Request) string {
	session := ctx.Get(r, "session").(*sessions.Session)

	// Check if we have an existing valid token
	if token, ok := session.Values["admin_csrf_token"].(string); ok {
		if tokenTime, ok := session.Values["admin_csrf_time"].(int64); ok {
			if time.Since(time.Unix(tokenTime, 0)) < 4*time.Hour {
				return token
			}
		}
	}

	// Generate new token
	config := DefaultAdminCSRFConfig()
	newToken := generateAdminCSRFToken(config.TokenLength)
	session.Values["admin_csrf_token"] = newToken
	session.Values["admin_csrf_time"] = time.Now().Unix()
	session.Save(r, nil)

	return newToken
}

// VerifyAdminCSRFToken verifies a CSRF token for admin operations
func VerifyAdminCSRFToken(r *http.Request, token string) bool {
	session := ctx.Get(r, "session").(*sessions.Session)

	sessionToken, ok := session.Values["admin_csrf_token"].(string)
	if !ok || sessionToken == "" {
		return false
	}

	// Check token expiry
	if tokenTime, ok := session.Values["admin_csrf_time"].(int64); ok {
		if time.Since(time.Unix(tokenTime, 0)) > 4*time.Hour {
			return false
		}
	}

	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(sessionToken), []byte(token)) == 1
}