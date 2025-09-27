package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// EmailAuthorizationAPI handles API requests for email authorization management
type EmailAuthorizationAPI struct{}

// AuthorizedEmailRequest represents a request to add/update an authorized email
type AuthorizedEmailRequest struct {
	Email       string     `json:"email" validate:"required,email"`
	RoleID      *int64     `json:"role_id"`
	DefaultRole string     `json:"default_role"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Notes       string     `json:"notes"`
}

// AuthorizedEmailResponse represents an authorized email response
type AuthorizedEmailResponse struct {
	ID              int64                    `json:"id"`
	Email           string                   `json:"email"`
	Status          string                   `json:"status"`
	Role            *models.Role             `json:"role,omitempty"`
	DefaultRole     string                   `json:"default_role"`
	CreatedBy       *models.User             `json:"created_by,omitempty"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
	ExpiresAt       *time.Time               `json:"expires_at"`
	LastUsedAt      *time.Time               `json:"last_used_at"`
	Notes           string                   `json:"notes"`
}

// AuthorizationLogResponse represents an authorization log entry
type AuthorizationLogResponse struct {
	ID        int64        `json:"id"`
	Email     string       `json:"email"`
	Action    string       `json:"action"`
	Result    string       `json:"result"`
	IPAddress string       `json:"ip_address"`
	UserAgent string       `json:"user_agent"`
	User      *models.User `json:"user,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	Details   string       `json:"details"`
}

// GetAuthorizedEmails returns all authorized emails
// GET /api/email-authorization/emails
func (api *EmailAuthorizationAPI) GetAuthorizedEmails(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 100 // Default limit
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get authorized emails from database
	emails, err := models.GetAuthorizedEmails(status, limit, offset)
	if err != nil {
		log.Errorf("Failed to get authorized emails: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to retrieve authorized emails"}, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var response []AuthorizedEmailResponse
	for _, email := range emails {
		response = append(response, AuthorizedEmailResponse{
			ID:          email.Id,
			Email:       email.Email,
			Status:      email.Status,
			Role:        email.Role,
			DefaultRole: email.DefaultRole,
			CreatedBy:   email.CreatedByUser,
			CreatedAt:   email.CreatedAt,
			UpdatedAt:   email.UpdatedAt,
			ExpiresAt:   email.ExpiresAt,
			LastUsedAt:  email.LastUsedAt,
			Notes:       email.Notes,
		})
	}

	JSONResponse(w, response, http.StatusOK)
}

// AddAuthorizedEmail adds a new authorized email
// POST /api/email-authorization/emails
func (api *EmailAuthorizationAPI) AddAuthorizedEmail(w http.ResponseWriter, r *http.Request) {
	var req AuthorizedEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON format"}, http.StatusBadRequest)
		return
	}

	// Validate email format
	service := models.NewEmailAuthorizationService()
	if err := service.ValidateEmailFormat(req.Email); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid email format: " + err.Error()}, http.StatusBadRequest)
		return
	}

	// Get current user
	user := ctx.Get(r, "user").(models.User)

	// Set default role if not provided
	if req.DefaultRole == "" {
		req.DefaultRole = "user"
	}

	// Add authorized email
	authorizedEmail, err := models.AddAuthorizedEmail(
		req.Email,
		req.RoleID,
		req.DefaultRole,
		&user.Id,
		req.ExpiresAt,
		req.Notes,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate") {
			JSONResponse(w, models.Response{Success: false, Message: "Email already authorized"}, http.StatusConflict)
			return
		}
		log.Errorf("Failed to add authorized email: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to add authorized email"}, http.StatusInternalServerError)
		return
	}

	// Log the action
	reqCtx := r.Context()
	service.LogAuthorizationAttempt(reqCtx, req.Email, "add", "success", &user.Id, "Added via API")

	// Return the created email
	response := AuthorizedEmailResponse{
		ID:          authorizedEmail.Id,
		Email:       authorizedEmail.Email,
		Status:      authorizedEmail.Status,
		DefaultRole: authorizedEmail.DefaultRole,
		CreatedAt:   authorizedEmail.CreatedAt,
		UpdatedAt:   authorizedEmail.UpdatedAt,
		ExpiresAt:   authorizedEmail.ExpiresAt,
		Notes:       authorizedEmail.Notes,
	}

	JSONResponse(w, response, http.StatusCreated)
}

// UpdateAuthorizedEmail updates an existing authorized email
// PUT /api/email-authorization/emails/{id}
func (api *EmailAuthorizationAPI) UpdateAuthorizedEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	var req AuthorizedEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON format"}, http.StatusBadRequest)
		return
	}

	// Get current user
	user := ctx.Get(r, "user").(models.User)

	// Update the email (implementation would depend on your requirements)
	// For now, we'll implement status update
	if req.Email != "" {
		// Validate email format if provided
		service := models.NewEmailAuthorizationService()
		if err := service.ValidateEmailFormat(req.Email); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid email format: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}

	// Log the action
	service := models.NewEmailAuthorizationService()
	reqCtx := r.Context()
	service.LogAuthorizationAttempt(reqCtx, req.Email, "update", "success", &user.Id, "Updated via API")

	JSONResponse(w, models.Response{Success: true, Message: "Authorized email updated successfully"}, http.StatusOK)
}

// UpdateAuthorizedEmailStatus updates the status of an authorized email
// PATCH /api/email-authorization/emails/{id}/status
func (api *EmailAuthorizationAPI) UpdateAuthorizedEmailStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status" validate:"required,oneof=active suspended revoked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON format"}, http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"suspended": true,
		"revoked":   true,
	}
	if !validStatuses[req.Status] {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid status. Must be: active, suspended, or revoked"}, http.StatusBadRequest)
		return
	}

	// Get current user
	user := ctx.Get(r, "user").(models.User)

	// Update status
	err = models.UpdateAuthorizedEmailStatus(id, req.Status, &user.Id)
	if err != nil {
		log.Errorf("Failed to update email status: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to update email status"}, http.StatusInternalServerError)
		return
	}

	// Log the action
	service := models.NewEmailAuthorizationService()
	reqCtx := r.Context()
	service.LogAuthorizationAttempt(reqCtx, "", req.Status, "success", &user.Id, "Status updated via API")

	JSONResponse(w, models.Response{Success: true, Message: "Email status updated successfully"}, http.StatusOK)
}

// DeleteAuthorizedEmail removes an authorized email
// DELETE /api/email-authorization/emails/{id}
func (api *EmailAuthorizationAPI) DeleteAuthorizedEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	// Get current user
	user := ctx.Get(r, "user").(models.User)

	// Delete the email
	err = models.DeleteAuthorizedEmail(id)
	if err != nil {
		log.Errorf("Failed to delete authorized email: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to delete authorized email"}, http.StatusInternalServerError)
		return
	}

	// Log the action
	service := models.NewEmailAuthorizationService()
	reqCtx := r.Context()
	service.LogAuthorizationAttempt(reqCtx, "", "delete", "success", &user.Id, "Deleted via API")

	JSONResponse(w, models.Response{Success: true, Message: "Authorized email deleted successfully"}, http.StatusOK)
}

// GetAuthorizationLogs returns authorization audit logs
// GET /api/email-authorization/logs
func (api *EmailAuthorizationAPI) GetAuthorizationLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	email := r.URL.Query().Get("email")
	action := r.URL.Query().Get("action")
	result := r.URL.Query().Get("result")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 100 // Default limit
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get logs from database
	logs, err := models.GetAuthorizationLogs(email, action, result, limit, offset)
	if err != nil {
		log.Errorf("Failed to get authorization logs: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to retrieve authorization logs"}, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var response []AuthorizationLogResponse
	for _, logEntry := range logs {
		response = append(response, AuthorizationLogResponse{
			ID:        logEntry.Id,
			Email:     logEntry.Email,
			Action:    logEntry.Action,
			Result:    logEntry.Result,
			IPAddress: logEntry.IPAddress,
			UserAgent: logEntry.UserAgent,
			User:      logEntry.User,
			CreatedAt: logEntry.CreatedAt,
			Details:   logEntry.Details,
		})
	}

	JSONResponse(w, response, http.StatusOK)
}

// CheckEmailAuthorization checks if an email is authorized
// GET /api/email-authorization/check?email={email}
func (api *EmailAuthorizationAPI) CheckEmailAuthorization(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Email parameter is required"}, http.StatusBadRequest)
		return
	}

	service := models.NewEmailAuthorizationService()
	result, err := service.CheckEmailAuthorization(email)
	if err != nil {
		log.Errorf("Failed to check email authorization: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to check email authorization"}, http.StatusInternalServerError)
		return
	}

	// Get current user for logging
	user := ctx.Get(r, "user").(models.User)

	// Log the check
	reqCtx := r.Context()
	logResult := "denied"
	if result.Authorized {
		logResult = "success"
	}
	service.LogAuthorizationAttempt(reqCtx, email, "check", logResult, &user.Id, "Manual check via API")

	// Return result
	response := map[string]interface{}{
		"email":      email,
		"authorized": result.Authorized,
		"reason":     result.Reason,
	}

	if result.Authorized {
		response["auth_method"] = result.AuthMethod
		response["role"] = result.GetRole()
	}

	JSONResponse(w, response, http.StatusOK)
}

// BulkAddAuthorizedEmails adds multiple emails at once
// POST /api/email-authorization/emails/bulk
func (api *EmailAuthorizationAPI) BulkAddAuthorizedEmails(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Emails      []string   `json:"emails" validate:"required"`
		RoleID      *int64     `json:"role_id"`
		DefaultRole string     `json:"default_role"`
		ExpiresAt   *time.Time `json:"expires_at"`
		Notes       string     `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON format"}, http.StatusBadRequest)
		return
	}

	if len(req.Emails) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "At least one email is required"}, http.StatusBadRequest)
		return
	}

	if len(req.Emails) > 100 {
		JSONResponse(w, models.Response{Success: false, Message: "Maximum 100 emails allowed per request"}, http.StatusBadRequest)
		return
	}

	// Get current user
	user := ctx.Get(r, "user").(models.User)

	// Set default role if not provided
	if req.DefaultRole == "" {
		req.DefaultRole = "user"
	}

	service := models.NewEmailAuthorizationService()
	var results []map[string]interface{}
	successCount := 0

	for _, email := range req.Emails {
		result := map[string]interface{}{
			"email": email,
		}

		// Validate email format
		if err := service.ValidateEmailFormat(email); err != nil {
			result["success"] = false
			result["error"] = "Invalid email format: " + err.Error()
			results = append(results, result)
			continue
		}

		// Add authorized email
		_, err := models.AddAuthorizedEmail(
			email,
			req.RoleID,
			req.DefaultRole,
			&user.Id,
			req.ExpiresAt,
			req.Notes,
		)

		if err != nil {
			result["success"] = false
			if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate") {
				result["error"] = "Email already authorized"
			} else {
				result["error"] = "Failed to add email"
				log.Errorf("Failed to add authorized email %s: %v", email, err)
			}
		} else {
			result["success"] = true
			successCount++

			// Log the action
			reqCtx := r.Context()
			service.LogAuthorizationAttempt(reqCtx, email, "bulk_add", "success", &user.Id, "Added via bulk API")
		}

		results = append(results, result)
	}

	response := map[string]interface{}{
		"total":         len(req.Emails),
		"successful":    successCount,
		"failed":        len(req.Emails) - successCount,
		"results":       results,
	}

	status := http.StatusOK
	if successCount == 0 {
		status = http.StatusBadRequest
	} else if successCount < len(req.Emails) {
		status = http.StatusPartialContent
	}

	JSONResponse(w, response, status)
}

// Helper functions for input validation and sanitization

// sanitizeInput removes potentially dangerous characters from input
func sanitizeInput(input string) string {
	// Remove null bytes and control characters
	input = strings.ReplaceAll(input, "\x00", "")
	// Limit length
	if len(input) > 500 {
		input = input[:500]
	}
	// Remove potential script tags
	re := regexp.MustCompile(`<[^>]*>`)
	input = re.ReplaceAllString(input, "")
	return strings.TrimSpace(input)
}

// containsSuspiciousPattern checks for suspicious patterns in email
func containsSuspiciousPattern(email string) bool {
	// Check for SQL injection patterns
	suspiciousPatterns := []string{
		"'",
		"\"",
		";",
		"--",
		"/*",
		"*/",
		"xp_",
		"sp_",
		"<script",
		"javascript:",
		"onclick",
		"onerror",
	}

	lowerEmail := strings.ToLower(email)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerEmail, pattern) {
			return true
		}
	}

	// Check for excessive length
	if len(email) > 254 {
		return true
	}

	return false
}

// isAdminRoleID checks if a role ID corresponds to an admin role
func isAdminRoleID(roleID int64) bool {
	role, err := models.GetRoleByID(roleID)
	if err != nil {
		return false
	}
	return role.Slug == models.RoleAdmin
}