package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/sessions"
)

// EmailAuthorizationConfig holds configuration for email authorization middleware
type EmailAuthorizationConfig struct {
	Enabled          bool     `json:"enabled"`
	EnforceForOAuth  bool     `json:"enforce_for_oauth"`  // Enforce for OAuth users
	EnforceForLocal  bool     `json:"enforce_for_local"`  // Enforce for local users
	ExemptPaths      []string `json:"exempt_paths"`       // Paths that skip authorization
	FailureRedirect  string   `json:"failure_redirect"`   // Where to redirect on failure
}

// DefaultEmailAuthConfig returns default email authorization configuration
func DefaultEmailAuthConfig() *EmailAuthorizationConfig {
	return &EmailAuthorizationConfig{
		Enabled:         false, // Disabled by default for safety
		EnforceForOAuth: true,
		EnforceForLocal: false, // Don't break existing local users
		ExemptPaths: []string{
			"/login",
			"/logout",
			"/static/",
			"/css/",
			"/js/",
			"/images/",
			"/auth/microsoft/callback", // OAuth callback needs to work
		},
		FailureRedirect: "/login?error=unauthorized",
	}
}

// RequireEmailAuthorization middleware that checks if the user's email is authorized
func RequireEmailAuthorization(config *EmailAuthorizationConfig) func(http.Handler) http.HandlerFunc {
	if config == nil {
		config = DefaultEmailAuthConfig()
	}

	return func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip if disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip exempt paths
			if isExemptPath(r.URL.Path, config.ExemptPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Get user from context
			user := ctx.Get(r, "user")
			if user == nil {
				// No user in context, let RequireLogin middleware handle it
				next.ServeHTTP(w, r)
				return
			}

			currentUser, ok := user.(models.User)
			if !ok {
				log.Error("Invalid user type in context")
				http.Redirect(w, r, config.FailureRedirect, http.StatusTemporaryRedirect)
				return
			}

			// Check if we should enforce authorization for this user type
			isOAuthUser := currentUser.OAuthProvider != ""
			if (isOAuthUser && !config.EnforceForOAuth) || (!isOAuthUser && !config.EnforceForLocal) {
				next.ServeHTTP(w, r)
				return
			}

			// Perform email authorization check
			if err := checkEmailAuthorization(r, &currentUser); err != nil {
				log.Warnf("Email authorization failed for user %s: %v", currentUser.Username, err)

				// Log the authorization attempt
				logAuthorizationAttempt(r, currentUser.Username, "access_denied", err.Error())

				// Handle unauthorized access
				handleUnauthorizedAccess(w, r, config, err)
				return
			}

			// Update last used timestamp
			updateEmailLastUsed(currentUser.Username)

			// Authorization successful, continue
			next.ServeHTTP(w, r)
		}
	}
}

// isExemptPath checks if a path should be exempt from email authorization
func isExemptPath(path string, exemptPaths []string) bool {
	for _, exempt := range exemptPaths {
		if strings.HasPrefix(path, exempt) {
			return true
		}
	}
	return false
}

// checkEmailAuthorization performs the actual email authorization check
func checkEmailAuthorization(r *http.Request, user *models.User) error {
	service := models.NewEmailAuthorizationService()

	// Check email authorization
	result, err := service.CheckEmailAuthorization(user.Username)
	if err != nil {
		return fmt.Errorf("authorization check failed: %w", err)
	}

	if !result.Authorized {
		switch result.Reason {
		case "invalid_format":
			return fmt.Errorf("invalid email format")
		case "not_authorized":
			return fmt.Errorf("email not authorized for access")
		default:
			return fmt.Errorf("access denied")
		}
	}

	// Store authorization result in request context for later use
	reqCtx := context.WithValue(r.Context(), "email_auth_result", result)
	*r = *r.WithContext(reqCtx)

	return nil
}

// logAuthorizationAttempt logs an email authorization attempt
func logAuthorizationAttempt(r *http.Request, email, result, details string) {
	service := models.NewEmailAuthorizationService()

	// Create context with request info
	reqCtx := context.WithValue(r.Context(), "ip", models.ExtractIPFromRequest(r))
	reqCtx = context.WithValue(reqCtx, "user_agent", r.UserAgent())

	// Get user ID if available
	var userID *int64
	if user := ctx.Get(r, "user"); user != nil {
		if currentUser, ok := user.(models.User); ok {
			userID = &currentUser.Id
		}
	}

	if err := service.LogAuthorizationAttempt(reqCtx, email, "access_check", result, userID, details); err != nil {
		log.Errorf("Failed to log authorization attempt: %v", err)
	}
}

// updateEmailLastUsed updates the last used timestamp for authorized email
func updateEmailLastUsed(email string) {
	service := models.NewEmailAuthorizationService()
	if err := service.UpdateLastUsed(email); err != nil {
		log.Warnf("Failed to update last used timestamp for %s: %v", email, err)
	}
}

// handleUnauthorizedAccess handles unauthorized access attempts
func handleUnauthorizedAccess(w http.ResponseWriter, r *http.Request, config *EmailAuthorizationConfig, err error) {
	session := ctx.Get(r, "session").(*sessions.Session)

	// Add flash message
	session.AddFlash(models.Flash{
		Type:    "danger",
		Message: "Your email address is not authorized to access this system. Please contact your administrator.",
	})
	session.Save(r, w)

	// For OAuth users, we might want to logout to clear their session
	if user := ctx.Get(r, "user"); user != nil {
		if currentUser, ok := user.(models.User); ok && currentUser.OAuthProvider != "" {
			// Clear session for OAuth users who are not authorized
			delete(session.Values, "id")
			session.Save(r, w)
		}
	}

	http.Redirect(w, r, config.FailureRedirect, http.StatusTemporaryRedirect)
}

// EmailAuthorizationInfo provides information about email authorization for templates
type EmailAuthorizationInfo struct {
	Enabled      bool   `json:"enabled"`
	UserEmail    string `json:"user_email"`
	IsAuthorized bool   `json:"is_authorized"`
	AuthMethod   string `json:"auth_method"`
	Role         string `json:"role"`
}

// GetEmailAuthorizationInfo returns email authorization info for the current user
func GetEmailAuthorizationInfo(r *http.Request) *EmailAuthorizationInfo {
	info := &EmailAuthorizationInfo{
		Enabled: false,
	}

	// Check if user is in context
	user := ctx.Get(r, "user")
	if user == nil {
		return info
	}

	currentUser, ok := user.(models.User)
	if !ok {
		return info
	}

	info.UserEmail = currentUser.Username
	info.Enabled = true

	// Check if we have authorization result in context
	if authResult := r.Context().Value("email_auth_result"); authResult != nil {
		if result, ok := authResult.(*models.EmailAuthorizationResult); ok {
			info.IsAuthorized = result.Authorized
			info.AuthMethod = result.AuthMethod
			info.Role = result.GetRole()
		}
	}

	return info
}

// RequireEmailAuthorizationAPI is a version for API endpoints that returns JSON errors
func RequireEmailAuthorizationAPI(config *EmailAuthorizationConfig) func(http.Handler) http.HandlerFunc {
	if config == nil {
		config = DefaultEmailAuthConfig()
	}

	return func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip if disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip exempt paths
			if isExemptPath(r.URL.Path, config.ExemptPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Get user from context
			user := ctx.Get(r, "user")
			if user == nil {
				// No user in context, let API authentication handle it
				next.ServeHTTP(w, r)
				return
			}

			currentUser, ok := user.(models.User)
			if !ok {
				JSONError(w, http.StatusInternalServerError, "Invalid user context")
				return
			}

			// Check if we should enforce authorization for this user type
			isOAuthUser := currentUser.OAuthProvider != ""
			if (isOAuthUser && !config.EnforceForOAuth) || (!isOAuthUser && !config.EnforceForLocal) {
				next.ServeHTTP(w, r)
				return
			}

			// Perform email authorization check
			if err := checkEmailAuthorization(r, &currentUser); err != nil {
				log.Warnf("API email authorization failed for user %s: %v", currentUser.Username, err)

				// Log the authorization attempt
				logAuthorizationAttempt(r, currentUser.Username, "api_access_denied", err.Error())

				JSONError(w, http.StatusForbidden, "Email not authorized for API access")
				return
			}

			// Update last used timestamp
			updateEmailLastUsed(currentUser.Username)

			// Authorization successful, continue
			next.ServeHTTP(w, r)
		}
	}
}