package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/csrf"
)

// CSRFExemptPrefixes are a list of routes that are exempt from CSRF protection
var CSRFExemptPrefixes = []string{
	"/auth/microsoft/callback",
	"/api/", // API routes use Bearer token authentication, not CSRF tokens
}

// CSRFExceptions is a middleware that prevents CSRF checks on routes listed in
// CSRFExemptPrefixes.
func CSRFExceptions(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range CSRFExemptPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				r = csrf.UnsafeSkipCheck(r)
				break
			}
		}
		handler.ServeHTTP(w, r)
	}
}

// Use allows us to stack middleware to process the request
// Example taken from https://github.com/gorilla/mux/pull/36#issuecomment-25849172
func Use(handler http.HandlerFunc, mid ...func(http.Handler) http.HandlerFunc) http.HandlerFunc {
	for _, m := range mid {
		handler = m(handler)
	}
	return handler
}

// GetContext wraps each request in a function which fills in the context for a given request.
// This includes setting the User and Session keys and values as necessary for use in later functions.
func GetContext(handler http.Handler) http.HandlerFunc {
	// Set the context here
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the request form
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing request", http.StatusInternalServerError)
		}
		// Set the context appropriately here.
		// Set the session
		session, err := Store.Get(r, "gophish")
		log.Infof("GetContext: Session error: %v", err)
		log.Infof("GetContext: Session values: %v", session.Values)

		// Validate SSO session if present
		if !ValidateSession(session) {
			log.Warnf("GetContext: SSO session validation failed, clearing session")
			// Clear invalid session
			delete(session.Values, "id")
			delete(session.Values, "sso_context")
			session.Save(r, w)
		}

		// Put the session in the context so that we can
		// reuse the values in different handlers
		r = ctx.Set(r, "session", session)
		if id, ok := session.Values["id"]; ok {
			log.Infof("GetContext: Found user ID in session: %v", id)
			u, err := models.GetUser(id.(int64))
			if err != nil {
				log.Errorf("GetContext: Error getting user: %v", err)
				r = ctx.Set(r, "user", nil)
			} else {
				log.Infof("GetContext: Loaded user: %s (Auth: %s)", u.Username, GetAuthMethod(session))
				r = ctx.Set(r, "user", u)

				// Add SSO context to request context for templates
				if ssoCtx, ok := session.Values["sso_context"].(*SSOSessionContext); ok {
					r = ctx.Set(r, "sso_context", ssoCtx)
				}
			}
		} else {
			log.Infof("GetContext: No user ID in session")
			r = ctx.Set(r, "user", nil)
		}
		handler.ServeHTTP(w, r)
		// Remove context contents
		ctx.Clear(r)
	}
}

// RequireAPIKey ensures that a valid API key is set as either the api_key GET
// parameter, or a Bearer token.
func RequireAPIKey(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Max-Age", "1000")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
			return
		}
		r.ParseForm()
		ak := r.Form.Get("api_key")
		// If we can't get the API key, we'll also check for the
		// Authorization Bearer token
		if ak == "" {
			tokens, ok := r.Header["Authorization"]
			if ok && len(tokens) >= 1 {
				ak = tokens[0]
				ak = strings.TrimPrefix(ak, "Bearer ")
			}
		}
		if ak == "" {
			JSONError(w, http.StatusUnauthorized, "API Key not set")
			return
		}
		u, err := models.GetUserByAPIKey(ak)
		if err != nil {
			JSONError(w, http.StatusUnauthorized, "Invalid API Key")
			return
		}
		r = ctx.Set(r, "user", u)
		r = ctx.Set(r, "user_id", u.Id)
		r = ctx.Set(r, "api_key", ak)
		handler.ServeHTTP(w, r)
	})
}

// RequireLogin checks to see if the user is currently logged in.
// If not, the function returns a 302 redirect to the login page.
func RequireLogin(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Infof("RequireLogin: Checking path %s", r.URL.Path)
		if u := ctx.Get(r, "user"); u != nil {
			log.Infof("RequireLogin: User found in context: %v", u)
		} else {
			log.Infof("RequireLogin: No user in context, redirecting to login")
		}
		if u := ctx.Get(r, "user"); u != nil {
			currentUser := u.(models.User)

			// Skip password change requirement for OAuth users (they don't have passwords)
			if currentUser.PasswordChangeRequired && currentUser.OAuthProvider == "" && r.URL.Path != "/reset_password" {
				q := r.URL.Query()
				q.Set("next", r.URL.Path)
				http.Redirect(w, r, fmt.Sprintf("/reset_password?%s", q.Encode()), http.StatusTemporaryRedirect)
				return
			}

			// Ensure admin email authorization is set up for OAuth admin users
			if currentUser.OAuthProvider != "" && currentUser.Role.Slug == models.RoleAdmin {
				if err := models.EnsureAdminEmailAuthorization(); err != nil {
					log.Warnf("Failed to ensure admin email authorization: %v", err)
				}
			}

			handler.ServeHTTP(w, r)
			return
		}
		q := r.URL.Query()
		q.Set("next", r.URL.Path)
		http.Redirect(w, r, fmt.Sprintf("/login?%s", q.Encode()), http.StatusTemporaryRedirect)
	}
}

// EnforceViewOnly is a global middleware that limits the ability to edit
// objects to accounts with the PermissionModifyObjects permission.
func EnforceViewOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the request is for any non-GET HTTP method, e.g. POST, PUT,
		// or DELETE, we need to ensure the user has the appropriate
		// permission.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			user := ctx.Get(r, "user").(models.User)
			access, err := user.HasPermission(models.PermissionModifyObjects)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if !access {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission checks to see if the user has the requested permission
// before executing the handler. If the request is unauthorized, a JSONError
// is returned.
func RequirePermission(perm string) func(http.Handler) http.HandlerFunc {
	return func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user := ctx.Get(r, "user").(models.User)
			access, err := user.HasPermission(perm)
			if err != nil {
				JSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !access {
				JSONError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}

// ApplySecurityHeaders applies various security headers according to best-
// practices.
func ApplySecurityHeaders(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csp := "frame-ancestors 'none';"
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	}
}

// JSONError returns an error in JSON format with the given
// status code and message
func JSONError(w http.ResponseWriter, c int, m string) {
	cj, _ := json.MarshalIndent(models.Response{Success: false, Message: m}, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)
	fmt.Fprintf(w, "%s", cj)
}
