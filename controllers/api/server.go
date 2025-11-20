package api

import (
	"net/http"

	mid "github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/worker"
	"github.com/gorilla/mux"
)

// ServerOption is an option to apply to the API server.
type ServerOption func(*Server)

// Server represents the routes and functionality of the Gophish API.
// It's not a server in the traditional sense, in that it isn't started and
// stopped. Rather, it's meant to be used as an http.Handler in the
// AdminServer.
type Server struct {
	handler http.Handler
	worker  worker.Worker
	limiter *ratelimit.PostLimiter
}

// NewServer returns a new instance of the API handler with the provided
// options applied.
func NewServer(options ...ServerOption) *Server {
	defaultWorker, _ := worker.New()
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &Server{
		worker:  defaultWorker,
		limiter: defaultLimiter,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) ServerOption {
	return func(as *Server) {
		as.worker = w
	}
}

func WithLimiter(limiter *ratelimit.PostLimiter) ServerOption {
	return func(as *Server) {
		as.limiter = limiter
	}
}

func (as *Server) registerRoutes() {
	root := mux.NewRouter()
	root = root.StrictSlash(true)

	// n8n callback endpoint (JWT authenticated, called by n8n after email send)
	// Must be registered on root router BEFORE /api/ subrouter to bypass RequireAPIKey middleware
	// Note: Full path /api/webhooks/n8n/status because admin server uses .Handler() not .Subrouter()
	root.HandleFunc("/api/webhooks/n8n/status", mid.RequireN8NJWT(as.N8NEmailCallback))

	router := root.PathPrefix("/api/").Subrouter()
	router.Use(mid.RequireAPIKey)
	router.Use(mid.EnforceViewOnly)
	router.HandleFunc("/imap/", as.IMAPServer)
	router.HandleFunc("/imap/validate", as.IMAPServerValidate)
	router.HandleFunc("/reset", as.Reset)
	router.HandleFunc("/campaigns/", as.Campaigns)
	router.HandleFunc("/campaigns/summary", as.CampaignsSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}", as.Campaign)
	router.HandleFunc("/campaigns/{id:[0-9]+}/results", as.CampaignResults)
	router.HandleFunc("/campaigns/{id:[0-9]+}/summary", as.CampaignSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}/complete", as.CampaignComplete)
	router.HandleFunc("/groups/", as.Groups)
	router.HandleFunc("/groups/summary", as.GroupsSummary)
	router.HandleFunc("/groups/{id:[0-9]+}", as.Group)
	router.HandleFunc("/groups/{id:[0-9]+}/summary", as.GroupSummary)
	router.HandleFunc("/templates/", as.Templates)
	router.HandleFunc("/templates/{id:[0-9]+}", as.Template)
	router.HandleFunc("/pages/", as.Pages)
	router.HandleFunc("/pages/{id:[0-9]+}", as.Page)
	router.HandleFunc("/smtp/", as.SendingProfiles)
	router.HandleFunc("/smtp/{id:[0-9]+}", as.SendingProfile)
	router.HandleFunc("/users/", mid.Use(as.Users, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/users/{id:[0-9]+}", mid.Use(as.User))
	router.HandleFunc("/util/send_test_email", as.SendTestEmail)
	router.HandleFunc("/import/group", as.ImportGroup)
	router.HandleFunc("/import/email", as.ImportEmail)
	router.HandleFunc("/import/site", as.ImportSite)
	router.HandleFunc("/webhooks/", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}/validate", mid.Use(as.ValidateWebhook, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}", mid.Use(as.Webhook, mid.RequirePermission(models.PermissionModifySystem)))

	// Email authorization routes (admin-only)
	router.HandleFunc("/email-authorization/emails", mid.Use(as.EmailAuthorizationEmails, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email-authorization/emails/bulk", mid.Use(as.EmailAuthorizationEmailsBulk, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email-authorization/emails/{id:[0-9]+}", mid.Use(as.EmailAuthorizationEmail, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email-authorization/emails/{id:[0-9]+}/status", mid.Use(as.EmailAuthorizationEmailStatus, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email-authorization/check", mid.Use(as.EmailAuthorizationCheck, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email-authorization/logs", mid.Use(as.EmailAuthorizationLogs, mid.RequirePermission(models.PermissionModifySystem)))

	// Email accounts routes (admin-only)
	router.HandleFunc("/email_accounts/", mid.Use(as.EmailAccounts, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email_accounts/{id:[0-9]+}", mid.Use(as.EmailAccount, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email_accounts/type/{type}", mid.Use(as.EmailAccountByType, mid.RequirePermission(models.PermissionModifySystem)))

	// Email types routes (admin-only)
	router.HandleFunc("/email_types/", mid.Use(as.EmailTypes, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email_types/all", mid.Use(as.EmailTypesAll, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/email_types/{id:[0-9]+}", mid.Use(as.EmailType, mid.RequirePermission(models.PermissionModifySystem)))

	// Use root router as handler to include both root routes (n8n callback) and subrouter routes (API endpoints)
	as.handler = root
}

func (as *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	as.handler.ServeHTTP(w, r)
}

// Email Authorization API handlers

// EmailAuthorizationEmails handles CRUD operations for authorized emails
func (as *Server) EmailAuthorizationEmails(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodGet:
		api.GetAuthorizedEmails(w, r)
	case http.MethodPost:
		api.AddAuthorizedEmail(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// EmailAuthorizationEmailsBulk handles bulk operations for authorized emails
func (as *Server) EmailAuthorizationEmailsBulk(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodPost:
		api.BulkAddAuthorizedEmails(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// EmailAuthorizationEmail handles operations for individual authorized emails
func (as *Server) EmailAuthorizationEmail(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodPut:
		api.UpdateAuthorizedEmail(w, r)
	case http.MethodDelete:
		api.DeleteAuthorizedEmail(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// EmailAuthorizationEmailStatus handles status updates for authorized emails
func (as *Server) EmailAuthorizationEmailStatus(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodPatch:
		api.UpdateAuthorizedEmailStatus(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// EmailAuthorizationCheck handles email authorization checks
func (as *Server) EmailAuthorizationCheck(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodGet:
		api.CheckEmailAuthorization(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// EmailAuthorizationLogs handles authorization audit log retrieval
func (as *Server) EmailAuthorizationLogs(w http.ResponseWriter, r *http.Request) {
	api := EmailAuthorizationAPI{}
	switch r.Method {
	case http.MethodGet:
		api.GetAuthorizationLogs(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}
