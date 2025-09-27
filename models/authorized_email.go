package models

import (
	"strings"
	"time"
	"fmt"
	"net"
	"net/http"
	"context"
)

// AuthorizedEmail represents an email authorized to access the system
type AuthorizedEmail struct {
	Id               int64     `json:"id" gorm:"column:id;primary_key"`
	Email            string    `json:"email" gorm:"column:email;unique;not null"`
	NormalizedEmail  string    `json:"-" gorm:"column:normalized_email;unique;not null"`
	Status           string    `json:"status" gorm:"column:status;not null;default:'active'"`
	RoleID           *int64    `json:"role_id" gorm:"column:role_id"`
	Role             *Role     `json:"role,omitempty" gorm:"foreignkey:RoleID"`
	DefaultRole      string    `json:"default_role" gorm:"column:default_role;default:'user'"`
	CreatedBy        *int64    `json:"created_by" gorm:"column:created_by"`
	CreatedByUser    *User     `json:"created_by_user,omitempty" gorm:"foreignkey:CreatedBy"`
	CreatedAt        time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"column:updated_at"`
	ExpiresAt        *time.Time `json:"expires_at" gorm:"column:expires_at"`
	LastUsedAt       *time.Time `json:"last_used_at" gorm:"column:last_used_at"`
	Notes            string    `json:"notes" gorm:"column:notes"`
}

// EmailAuthorizationLog represents audit log for email authorization attempts
type EmailAuthorizationLog struct {
	Id              int64     `json:"id" gorm:"column:id;primary_key"`
	Email           string    `json:"email" gorm:"column:email;not null"`
	NormalizedEmail string    `json:"-" gorm:"column:normalized_email;not null"`
	Action          string    `json:"action" gorm:"column:action;not null"`
	Result          string    `json:"result" gorm:"column:result;not null"`
	IPAddress       string    `json:"ip_address" gorm:"column:ip_address"`
	UserAgent       string    `json:"user_agent" gorm:"column:user_agent"`
	UserID          *int64    `json:"user_id" gorm:"column:user_id"`
	User            *User     `json:"user,omitempty" gorm:"foreignkey:UserID"`
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at"`
	Details         string    `json:"details" gorm:"column:details"`
}

// AuthorizedDomain represents domain-based authorization
type AuthorizedDomain struct {
	Id          int64     `json:"id" gorm:"column:id;primary_key"`
	Domain      string    `json:"domain" gorm:"column:domain;unique;not null"`
	Status      string    `json:"status" gorm:"column:status;not null;default:'active'"`
	DefaultRole string    `json:"default_role" gorm:"column:default_role;default:'user'"`
	CreatedBy   *int64    `json:"created_by" gorm:"column:created_by"`
	CreatedByUser *User   `json:"created_by_user,omitempty" gorm:"foreignkey:CreatedBy"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	Notes       string    `json:"notes" gorm:"column:notes"`
}

// EmailAuthorizationService provides email authorization functionality
type EmailAuthorizationService struct{}

// NewEmailAuthorizationService creates a new email authorization service
func NewEmailAuthorizationService() *EmailAuthorizationService {
	return &EmailAuthorizationService{}
}

// NormalizeEmail normalizes an email for consistent storage and lookup
func (s *EmailAuthorizationService) NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmailFormat performs basic email validation
func (s *EmailAuthorizationService) ValidateEmailFormat(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Basic email format validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid email format")
	}

	// Additional validation can be added here
	if len(email) > 254 { // RFC 5321 limit
		return fmt.Errorf("email too long")
	}

	return nil
}

// IsEmailAuthorized checks if an email is authorized to access the system
func (s *EmailAuthorizationService) IsEmailAuthorized(email string) (*AuthorizedEmail, error) {
	normalizedEmail := s.NormalizeEmail(email)

	var authorizedEmail AuthorizedEmail
	err := db.Where("normalized_email = ? AND status = 'active'", normalizedEmail).
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now()).
		Preload("Role").
		First(&authorizedEmail).Error

	if err != nil {
		return nil, err
	}

	return &authorizedEmail, nil
}

// IsEmailAuthorizedByDomain checks if an email domain is authorized
func (s *EmailAuthorizationService) IsEmailAuthorizedByDomain(email string) (*AuthorizedDomain, error) {
	normalizedEmail := s.NormalizeEmail(email)
	parts := strings.Split(normalizedEmail, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid email format")
	}

	domain := parts[1]

	var authorizedDomain AuthorizedDomain
	err := db.Where("domain = ? AND status = 'active'", domain).
		First(&authorizedDomain).Error

	if err != nil {
		return nil, err
	}

	return &authorizedDomain, nil
}

// CheckEmailAuthorization performs comprehensive email authorization check
func (s *EmailAuthorizationService) CheckEmailAuthorization(email string) (*EmailAuthorizationResult, error) {
	if err := s.ValidateEmailFormat(email); err != nil {
		return &EmailAuthorizationResult{
			Authorized: false,
			Reason:     "invalid_format",
			Error:      err,
		}, nil
	}

	// Check direct email authorization
	authorizedEmail, err := s.IsEmailAuthorized(email)
	if err == nil {
		return &EmailAuthorizationResult{
			Authorized:      true,
			AuthorizedEmail: authorizedEmail,
			AuthMethod:      "email",
		}, nil
	}

	// Check domain-based authorization
	authorizedDomain, err := s.IsEmailAuthorizedByDomain(email)
	if err == nil {
		return &EmailAuthorizationResult{
			Authorized:       true,
			AuthorizedDomain: authorizedDomain,
			AuthMethod:       "domain",
		}, nil
	}

	return &EmailAuthorizationResult{
		Authorized: false,
		Reason:     "not_authorized",
	}, nil
}

// LogAuthorizationAttempt logs an email authorization attempt
func (s *EmailAuthorizationService) LogAuthorizationAttempt(ctx context.Context, email, action, result string, userID *int64, details string) error {
	// Extract IP and User-Agent from context if available
	ipAddress := ""
	userAgent := ""

	if ip, ok := ctx.Value("ip").(string); ok {
		ipAddress = ip
	}
	if ua, ok := ctx.Value("user_agent").(string); ok {
		userAgent = ua
	}

	log := EmailAuthorizationLog{
		Email:           email,
		NormalizedEmail: s.NormalizeEmail(email),
		Action:          action,
		Result:          result,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		UserID:          userID,
		CreatedAt:       time.Now(),
		Details:         details,
	}

	return db.Create(&log).Error
}

// UpdateLastUsed updates the last used timestamp for an authorized email
func (s *EmailAuthorizationService) UpdateLastUsed(email string) error {
	normalizedEmail := s.NormalizeEmail(email)
	now := time.Now()

	return db.Model(&AuthorizedEmail{}).
		Where("normalized_email = ?", normalizedEmail).
		Update("last_used_at", now).Error
}

// EmailAuthorizationResult represents the result of an email authorization check
type EmailAuthorizationResult struct {
	Authorized       bool                `json:"authorized"`
	AuthorizedEmail  *AuthorizedEmail    `json:"authorized_email,omitempty"`
	AuthorizedDomain *AuthorizedDomain   `json:"authorized_domain,omitempty"`
	AuthMethod       string              `json:"auth_method,omitempty"` // "email" or "domain"
	Reason           string              `json:"reason,omitempty"`       // reason for denial
	Error            error               `json:"-"`
}

// GetRole returns the appropriate role for the authorization result
func (r *EmailAuthorizationResult) GetRole() string {
	if r.AuthorizedEmail != nil {
		if r.AuthorizedEmail.Role != nil {
			return r.AuthorizedEmail.Role.Name
		}
		return r.AuthorizedEmail.DefaultRole
	}

	if r.AuthorizedDomain != nil {
		return r.AuthorizedDomain.DefaultRole
	}

	return "user" // fallback
}

// Helper functions for managing authorized emails

// GetAuthorizedEmails returns all authorized emails with optional filtering
func GetAuthorizedEmails(status string, limit, offset int) ([]AuthorizedEmail, error) {
	var emails []AuthorizedEmail
	query := db.Preload("Role").Preload("CreatedByUser")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	err := query.Find(&emails).Error
	return emails, err
}

// AddAuthorizedEmail adds a new authorized email
func AddAuthorizedEmail(email string, roleID *int64, defaultRole string, createdBy *int64, expiresAt *time.Time, notes string) (*AuthorizedEmail, error) {
	service := NewEmailAuthorizationService()

	if err := service.ValidateEmailFormat(email); err != nil {
		return nil, err
	}

	authorizedEmail := AuthorizedEmail{
		Email:           email,
		NormalizedEmail: service.NormalizeEmail(email),
		Status:          "active",
		RoleID:          roleID,
		DefaultRole:     defaultRole,
		CreatedBy:       createdBy,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       expiresAt,
		Notes:           notes,
	}

	err := db.Create(&authorizedEmail).Error
	if err != nil {
		return nil, err
	}

	return &authorizedEmail, nil
}

// UpdateAuthorizedEmailStatus updates the status of an authorized email
func UpdateAuthorizedEmailStatus(id int64, status string, updatedBy *int64) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	return db.Model(&AuthorizedEmail{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteAuthorizedEmail removes an authorized email
func DeleteAuthorizedEmail(id int64) error {
	return db.Delete(&AuthorizedEmail{}, id).Error
}

// GetAuthorizationLogs returns authorization logs with optional filtering
func GetAuthorizationLogs(email, action, result string, limit, offset int) ([]EmailAuthorizationLog, error) {
	var logs []EmailAuthorizationLog
	query := db.Preload("User")

	if email != "" {
		service := NewEmailAuthorizationService()
		normalizedEmail := service.NormalizeEmail(email)
		query = query.Where("normalized_email = ?", normalizedEmail)
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}

	if result != "" {
		query = query.Where("result = ?", result)
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	query = query.Order("created_at DESC")
	err := query.Find(&logs).Error
	return logs, err
}

// ExtractIPFromRequest safely extracts IP address from request
func ExtractIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Return as-is if parsing fails
	}

	return ip
}