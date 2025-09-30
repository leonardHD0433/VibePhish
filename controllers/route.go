package controllers

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/controllers/api"
	log "github.com/gophish/gophish/logger"
	mid "github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/util"
	"github.com/gophish/gophish/worker"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jordan-wright/unindexed"
)

// AdminServerOption is a functional option that is used to configure the
// admin server
type AdminServerOption func(*AdminServer)

// AdminServer is an HTTP server that implements the administrative Gophish
// handlers, including the dashboard and REST API.
type AdminServer struct {
	server  *http.Server
	worker  worker.Worker
	config  config.AdminServer
	limiter *ratelimit.PostLimiter
}

// buildOAuthRedirectURL constructs the OAuth callback URL based on server configuration
func buildOAuthRedirectURL(cfg *config.Config, r *http.Request) string {
	// Determine protocol with multiple detection methods
	protocol := "http"

	// Method 1: Check proxy headers (load balancer/reverse proxy)
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto == "https" {
		protocol = "https"
	} else if r.Header.Get("X-Forwarded-Scheme") == "https" {
		protocol = "https"
	} else if strings.Contains(r.Host, "azurecontainerapps.io") {
		// Method 2: Azure Container Apps always use HTTPS externally
		protocol = "https"
	} else if r.Header.Get("X-Forwarded-For") != "" {
		// Method 3: Behind proxy, likely HTTPS in production
		protocol = "https"
	} else if cfg.AdminConf.UseTLS {
		// Method 4: Config-based detection (for local HTTPS)
		protocol = "https"
	}

	// Get host from request or config
	host := r.Host
	if host == "" {
		// Fallback to config listen URL if no host in request
		host = cfg.AdminConf.ListenURL
	}

	// Clean up host (remove protocol if present)
	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}

	return fmt.Sprintf("%s://%s/auth/microsoft/callback", protocol, host)
}

var defaultTLSConfig = &tls.Config{
	PreferServerCipherSuites: true,
	CurvePreferences: []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	},
	MinVersion: tls.VersionTLS12,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		// Kept for backwards compatibility with some clients
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	},
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) AdminServerOption {
	return func(as *AdminServer) {
		as.worker = w
	}
}

// NewAdminServer returns a new instance of the AdminServer with the
// provided config and options applied.
func NewAdminServer(config config.AdminServer, options ...AdminServerOption) *AdminServer {
	defaultWorker, _ := worker.New()
	defaultServer := &http.Server{
		ReadTimeout: 10 * time.Second,
		Addr:        config.ListenURL,
	}
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &AdminServer{
		worker:  defaultWorker,
		server:  defaultServer,
		limiter: defaultLimiter,
		config:  config,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// Start launches the admin server, listening on the configured address.
func (as *AdminServer) Start() {
	if as.worker != nil {
		go as.worker.Start()
	}
	if as.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		as.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(as.config.CertPath, as.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting admin server at https://%s", as.config.ListenURL)
		log.Fatal(as.server.ListenAndServeTLS(as.config.CertPath, as.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting admin server at http://%s", as.config.ListenURL)
	log.Fatal(as.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (as *AdminServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return as.server.Shutdown(ctx)
}

// SetupAdminRoutes creates the routes for handling requests to the web interface.
// This function returns an http.Handler to be used in http.ListenAndServe().
func (as *AdminServer) registerRoutes() {
	router := mux.NewRouter()
	// Health check endpoints (no authentication required)
	// TODO: Re-enable health endpoints after PostgreSQL migration
	// router.HandleFunc("/health", as.HealthHandler)
	// router.HandleFunc("/ready", as.ReadinessHandler)
	// router.HandleFunc("/live", as.LivenessHandler)

	// Base Front-end routes
	router.HandleFunc("/", mid.Use(as.Base, mid.RequireLogin))
	router.HandleFunc("/login", mid.Use(as.Login, as.limiter.Limit))
	router.HandleFunc("/logout", mid.Use(as.Logout, mid.RequireLogin))
	router.HandleFunc("/reset_password", mid.Use(as.ResetPassword, mid.RequireLogin))
	// OAuth SSO routes
	router.HandleFunc("/auth/microsoft", mid.Use(as.OAuthMicrosoft))
	router.HandleFunc("/auth/microsoft/callback", mid.Use(as.OAuthMicrosoftCallback))
	router.HandleFunc("/campaigns", mid.Use(as.Campaigns, mid.RequireLogin))
	router.HandleFunc("/campaigns/{id:[0-9]+}", mid.Use(as.CampaignID, mid.RequireLogin))
	router.HandleFunc("/templates", mid.Use(as.Templates, mid.RequireLogin))
	router.HandleFunc("/groups", mid.Use(as.Groups, mid.RequireLogin))
	router.HandleFunc("/landing_pages", mid.Use(as.LandingPages, mid.RequireLogin))
	router.HandleFunc("/sending_profiles", mid.Use(as.SendingProfiles, mid.RequireLogin))
	router.HandleFunc("/settings", mid.Use(as.Settings, mid.RequireLogin))
	router.HandleFunc("/users", mid.Use(as.UserManagement, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/webhooks", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/impersonate", mid.Use(as.Impersonate, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	// Create the API routes
	api := api.NewServer(
		api.WithWorker(as.worker),
		api.WithLimiter(as.limiter),
	)
	router.PathPrefix("/api/").Handler(api)

	// Setup static file serving
	router.PathPrefix("/").Handler(http.FileServer(unindexed.Dir("./static/")))

	// Setup CSRF Protection
	csrfKey := []byte(as.config.CSRFKey)
	if len(csrfKey) == 0 {
		csrfKey = []byte(auth.GenerateSecureKey(auth.APIKeyLength))
	}
	// Debug: Print trusted origins
	log.Infof("Loading CSRF with trusted origins: %v", as.config.TrustedOrigins)
	csrfHandler := csrf.Protect(csrfKey,
		csrf.FieldName("csrf_token"),
		csrf.Secure(as.config.UseTLS),
		csrf.TrustedOrigins(as.config.TrustedOrigins))
	adminHandler := csrfHandler(router)
	adminHandler = mid.Use(adminHandler.ServeHTTP, mid.CSRFExceptions, mid.GetContext, mid.ApplySecurityHeaders)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	adminHandler = gzipWrapper(adminHandler)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	adminHandler = handlers.ProxyHeaders(adminHandler)

	// Setup logging
	adminHandler = handlers.CombinedLoggingHandler(log.Writer(), adminHandler)
	as.server.Handler = adminHandler
}

type templateParams struct {
	Title        string
	Flashes      []interface{}
	User         models.User
	Token        string
	Version      string
	ModifySystem bool
}

// newTemplateParams returns the default template parameters for a user and
// the CSRF token.
func newTemplateParams(r *http.Request) templateParams {
	user := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	modifySystem, _ := user.HasPermission(models.PermissionModifySystem)
	return templateParams{
		Token:        csrf.Token(r),
		User:         user,
		ModifySystem: modifySystem,
		Version:      config.Version,
		Flashes:      session.Flashes(),
	}
}

// Base handles the default path and template execution
func (as *AdminServer) Base(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Dashboard"
	getTemplate(w, "dashboard").ExecuteTemplate(w, "base", params)
}

// Campaigns handles the default path and template execution
func (as *AdminServer) Campaigns(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaigns"
	getTemplate(w, "campaigns").ExecuteTemplate(w, "base", params)
}

// CampaignID handles the default path and template execution
func (as *AdminServer) CampaignID(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaign Results"
	getTemplate(w, "campaign_results").ExecuteTemplate(w, "base", params)
}

// Templates handles the default path and template execution
func (as *AdminServer) Templates(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Email Templates"
	getTemplate(w, "templates").ExecuteTemplate(w, "base", params)
}

// Groups handles the default path and template execution
func (as *AdminServer) Groups(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Users & Groups"
	getTemplate(w, "groups").ExecuteTemplate(w, "base", params)
}

// LandingPages handles the default path and template execution
func (as *AdminServer) LandingPages(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Landing Pages"
	getTemplate(w, "landing_pages").ExecuteTemplate(w, "base", params)
}

// SendingProfiles handles the default path and template execution
func (as *AdminServer) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Sending Profiles"
	getTemplate(w, "sending_profiles").ExecuteTemplate(w, "base", params)
}

// Settings handles the changing of settings
func (as *AdminServer) Settings(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		params := newTemplateParams(r)
		params.Title = "Settings"
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Save(r, w)
		getTemplate(w, "settings").ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		u := ctx.Get(r, "user").(models.User)
		currentPw := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_new_password")
		// Check the current password
		err := auth.ValidatePassword(currentPw, u.Hash)
		msg := models.Response{Success: true, Message: "Settings Updated Successfully"}
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		u.Hash = string(newHash)
		if err = models.PutUser(&u); err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusInternalServerError)
			return
		}
		api.JSONResponse(w, msg, http.StatusOK)
	}
}

// UserManagement is an admin-only handler that allows for the registration
// and management of user accounts within Gophish.
func (as *AdminServer) UserManagement(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "User Management"
	getTemplate(w, "users").ExecuteTemplate(w, "base", params)
}

func (as *AdminServer) nextOrIndex(w http.ResponseWriter, r *http.Request) {
	next := "/"
	url, err := url.Parse(r.FormValue("next"))
	if err == nil {
		path := url.EscapedPath()
		if path != "" {
			next = "/" + strings.TrimLeft(path, "/")
		}
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (as *AdminServer) handleInvalidLogin(w http.ResponseWriter, r *http.Request, message string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	Flash(w, r, "danger", message)
	params := struct {
		User    models.User
		Title   string
		Flashes []interface{}
		Token   string
	}{Title: "Login", Token: csrf.Token(r)}
	params.Flashes = session.Flashes()
	session.Save(r, w)
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	template.Must(templates, err).ExecuteTemplate(w, "base", params)
}

// Webhooks is an admin-only handler that handles webhooks
func (as *AdminServer) Webhooks(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Webhooks"
	getTemplate(w, "webhooks").ExecuteTemplate(w, "base", params)
}

// Impersonate allows an admin to login to a user account without needing the password
func (as *AdminServer) Impersonate(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		username := r.FormValue("username")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Values["id"] = u.Id
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Login handles the authentication flow for a user. If credentials are valid,
// a session is created
func (as *AdminServer) Login(w http.ResponseWriter, r *http.Request) {
	// Check if user is already authenticated
	session := ctx.Get(r, "session").(*sessions.Session)
	if currentUser := ctx.Get(r, "user"); currentUser != nil {
		// User is already logged in, redirect to dashboard
		next := r.URL.Query().Get("next")
		if next != "" && isValidRedirectURL(next) {
			http.Redirect(w, r, next, http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
		return
	}

	params := struct {
		User                models.User
		Title               string
		Flashes             []interface{}
		Token               string
		SSOEnabled          bool
		AllowLocalLogin     bool
		HideLocalLogin      bool
		EmergencyAccess     bool
		EmergencyMode       bool
		MicrosoftEnabled    bool
	}{
		Title:            "Login",
		Token:            csrf.Token(r),
		SSOEnabled:       true,  // Default assumption
		AllowLocalLogin:  true,  // Will be determined by config
		HideLocalLogin:   false, // Will be determined by config
		EmergencyAccess:  true,  // Will be determined by config
		EmergencyMode:    false,
		MicrosoftEnabled: false,
	}

	// Load SSO configuration to determine login options
	cfg, err := config.LoadConfigWithSSO("./config.json")
	if err == nil && cfg.SSO != nil {
		params.SSOEnabled = cfg.IsSSOEnabled()
		params.AllowLocalLogin = cfg.ShouldAllowLocalLogin()
		params.HideLocalLogin = cfg.ShouldHideLocalLogin()
		params.EmergencyAccess = cfg.IsEmergencyAccessEnabled()
		params.MicrosoftEnabled = cfg.IsProviderEnabled("microsoft")
		params.EmergencyMode = r.URL.Query().Get("emergency") == "true"
	}

	switch {
	case r.Method == "GET":
		// Preserve OAuth session data before calling Flashes() which can clear session data
		var oauthData = make(map[string]interface{})
		for _, key := range []string{"oauth_state", "oauth_code_verifier", "oauth_provider", "oauth_timestamp", "oauth_nonce", "oauth_next"} {
			if value, exists := session.Values[key]; exists {
				oauthData[key] = value
			}
		}

		params.Flashes = session.Flashes()

		// Restore OAuth session data after Flashes()
		for key, value := range oauthData {
			session.Values[key] = value
		}

		session.Save(r, w)
		templates := template.New("template")
		_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
		if err != nil {
			log.Error(err)
		}
		template.Must(templates, err).ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		// Check if this is an emergency login attempt
		isEmergencyLogin := r.FormValue("emergency_login") == "true"

		// If SSO is enabled and local login is disabled, only allow emergency access
		if cfg != nil && cfg.IsSSOEnabled() && !cfg.ShouldAllowLocalLogin() && !isEmergencyLogin {
			log.Warnf("Local login attempt blocked - SSO-only mode active")
			as.handleInvalidLogin(w, r, "Please use Single Sign-On to access this system")
			return
		}

		// If emergency access is disabled, block emergency login attempts
		if isEmergencyLogin && cfg != nil && !cfg.IsEmergencyAccessEnabled() {
			log.Warnf("Emergency login attempt blocked - emergency access disabled")
			as.handleInvalidLogin(w, r, "Emergency access is not available")
			return
		}

		// Find the user with the provided username
		username, password := r.FormValue("username"), r.FormValue("password")
		if username == "" || password == "" {
			as.handleInvalidLogin(w, r, "Username and password are required")
			return
		}

		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			// Enhanced logging for emergency access attempts
			if isEmergencyLogin {
				log.Warnf("Emergency login attempt failed for username: %s", username)
			}
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		// Validate the user's password
		err = auth.ValidatePassword(password, u.Hash)
		if err != nil {
			log.Error(err)
			// Enhanced logging for emergency access attempts
			if isEmergencyLogin {
				log.Warnf("Emergency login password validation failed for user: %s", username)
			}
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		if u.AccountLocked {
			if isEmergencyLogin {
				log.Warnf("Emergency login attempt on locked account: %s", username)
			}
			as.handleInvalidLogin(w, r, "Account Locked")
			return
		}

		// Log successful emergency access for security monitoring
		if isEmergencyLogin {
			log.Warnf("Emergency login successful for user: %s (ID: %d)", username, u.Id)
		}

		u.LastLogin = time.Now().UTC()
		err = models.PutUser(&u)
		if err != nil {
			log.Error(err)
		}
		// If we've logged in, save the session and redirect to the dashboard
		log.Infof("Login: Setting user ID %d in session", u.Id)
		session.Values["id"] = u.Id
		// Mark login method for security tracking
		if isEmergencyLogin {
			session.Values["auth_method"] = "emergency_local"
			session.Values["auth_time"] = time.Now().Unix()
		} else {
			session.Values["auth_method"] = "local"
		}
		log.Infof("Login: Session values before save: %v", session.Values)
		err = session.Save(r, w)
		if err != nil {
			log.Errorf("Login: Error saving session: %v", err)
		} else {
			log.Infof("Login: Session saved successfully")
		}
		as.nextOrIndex(w, r)
	}
}

// Logout destroys the current user session
func (as *AdminServer) Logout(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)
	delete(session.Values, "id")
	Flash(w, r, "success", "You have successfully logged out")
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ResetPassword handles the password reset flow when a password change is
// required either by the Gophish system or an administrator.
//
// This handler is meant to be used when a user is required to reset their
// password, not just when they want to.
//
// This is an important distinction since in this handler we don't require
// the user to re-enter their current password, as opposed to the flow
// through the settings handler.
//
// To that end, if the user doesn't require a password change, we will
// redirect them to the settings page.
func (as *AdminServer) ResetPassword(w http.ResponseWriter, r *http.Request) {
	u := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	if !u.PasswordChangeRequired {
		Flash(w, r, "info", "Please reset your password through the settings page")
		session.Save(r, w)
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
		return
	}
	params := newTemplateParams(r)
	params.Title = "Reset Password"
	switch {
	case r.Method == http.MethodGet:
		params.Flashes = session.Flashes()
		session.Save(r, w)
		getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
		return
	case r.Method == http.MethodPost:
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusBadRequest)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		u.PasswordChangeRequired = false
		u.Hash = newHash
		if err = models.PutUser(&u); err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusInternalServerError)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		// TODO: We probably want to flash a message here that the password was
		// changed successfully. The problem is that when the user resets their
		// password on first use, they will see two flashes on the dashboard-
		// one for their password reset, and one for the "no campaigns created".
		//
		// The solution to this is to revamp the empty page to be more useful,
		// like a wizard or something.
		as.nextOrIndex(w, r)
	}
}

// OAuthMicrosoft handles the Microsoft OAuth initiation endpoint
func (as *AdminServer) OAuthMicrosoft(w http.ResponseWriter, r *http.Request) {

	// Load config with SSO settings
	cfg, err := config.LoadConfigWithSSO("./config.json")
	if err != nil {
		log.Errorf("Failed to load SSO config: %v", err)
		Flash(w, r, "danger", "SSO is temporarily unavailable. Please use emergency access or try again later.")
		http.Redirect(w, r, "/login?emergency=true", http.StatusTemporaryRedirect)
		return
	}

	// Check if Microsoft SSO is enabled
	if !cfg.IsSSOEnabled() || !cfg.IsProviderEnabled("microsoft") {
		Flash(w, r, "warning", "Single Sign-On is currently disabled. Please use local login.")
		http.Redirect(w, r, "/login?emergency=true", http.StatusFound)
		return
	}

	// Create Microsoft OAuth provider
	microsoftProvider := auth.NewMicrosoftProvider(cfg.SSO.Providers["microsoft"])
	if microsoftProvider == nil {
		log.Errorf("Failed to create Microsoft OAuth provider")
		Flash(w, r, "danger", "Microsoft SSO is temporarily unavailable. Please use emergency access.")
		http.Redirect(w, r, "/login?emergency=true", http.StatusTemporaryRedirect)
		return
	}

	// Set the redirect URL for OAuth callback (dynamic based on server config)
	redirectURL := buildOAuthRedirectURL(cfg, r)
	log.Infof("Setting OAuth redirect URL to: %s", redirectURL)
	microsoftProvider.SetRedirectURL(redirectURL)

	// Create OAuth handler and initiate flow
	userOps := models.GetOAuthUserOperations()
	oauthHandler := auth.NewOAuthHandler(cfg, microsoftProvider, userOps)

	// Add error recovery mechanism
	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Errorf("OAuth initiation panic: %v", panicErr)
			Flash(w, r, "danger", "SSO initialization failed. Please use emergency access.")
			http.Redirect(w, r, "/login?emergency=true", http.StatusTemporaryRedirect)
		}
	}()

	oauthHandler.InitiateMicrosoftOAuth(w, r)
}

// OAuthMicrosoftCallback handles the Microsoft OAuth callback endpoint
func (as *AdminServer) OAuthMicrosoftCallback(w http.ResponseWriter, r *http.Request) {
	// Load config with SSO settings
	cfg, err := config.LoadConfigWithSSO("./config.json")
	if err != nil {
		log.Errorf("Failed to load SSO config: %v", err)
		http.Error(w, "SSO configuration error", http.StatusInternalServerError)
		return
	}

	// Create Microsoft OAuth provider
	microsoftProvider := auth.NewMicrosoftProvider(cfg.SSO.Providers["microsoft"])

	// Set the redirect URL for OAuth callback (dynamic based on server config)
	redirectURL := buildOAuthRedirectURL(cfg, r)
	log.Infof("OAuth callback using redirect URL: %s", redirectURL)
	microsoftProvider.SetRedirectURL(redirectURL)

	// Create OAuth handler and handle callback
	userOps := models.GetOAuthUserOperations()
	oauthHandler := auth.NewOAuthHandler(cfg, microsoftProvider, userOps)
	oauthHandler.HandleMicrosoftCallback(w, r)
}

// TODO: Make this execute the template, too
func getTemplate(w http.ResponseWriter, tmpl string) *template.Template {
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/base.html", "templates/nav.html", "templates/"+tmpl+".html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	return template.Must(templates, err)
}

// isValidRedirectURL validates that a redirect URL is safe
func isValidRedirectURL(url string) bool {
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

// Flash handles the rendering flash messages
func Flash(w http.ResponseWriter, r *http.Request, t string, m string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	session.AddFlash(models.Flash{
		Type:    t,
		Message: m,
	})
}
