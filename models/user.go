package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gophish/gophish/config"
	log "github.com/gophish/gophish/logger"
)

// ErrModifyingOnlyAdmin occurs when there is an attempt to modify the only
// user account with the Admin role in such a way that there will be no user
// accounts left in Gophish with that role.
var ErrModifyingOnlyAdmin = errors.New("Cannot remove the only administrator")

// OAuth operations interface for external packages to use
type OAuthUserOperationsInterface interface {
	FindOrCreateUser(provider, oauthID, email string) (userID int64, username string, accountLocked bool, isAdmin bool, err error)
	UpdateLastLogin(userID int64) error
	ValidateAdminPrivilege(userID int64) (bool, error)
	LogSecurityEvent(userID int64, event, details string) error
}

// GetOAuthUserOperations returns the OAuth user operations implementation
func GetOAuthUserOperations() OAuthUserOperationsInterface {
	return &oauthUserOps{}
}

type oauthUserOps struct{}

func (ops *oauthUserOps) FindOrCreateUser(provider, oauthID, email string) (userID int64, username string, accountLocked bool, isAdmin bool, err error) {
	user, err := FindOrCreateOAuthUser(provider, oauthID, email)
	if err != nil {
		return 0, "", false, false, err
	}

	// Check if user has admin role
	isAdminUser := false
	if user.Role.Slug == RoleAdmin {
		isAdminUser = true
	}

	return user.Id, user.Username, user.AccountLocked, isAdminUser, nil
}

func (ops *oauthUserOps) UpdateLastLogin(userID int64) error {
	user, err := GetUser(userID)
	if err != nil {
		return err
	}
	user.LastLogin = time.Now().UTC()
	return PutUser(&user)
}

func (ops *oauthUserOps) ValidateAdminPrivilege(userID int64) (bool, error) {
	user, err := GetUser(userID)
	if err != nil {
		return false, err
	}
	return user.Role.Slug == RoleAdmin, nil
}

func (ops *oauthUserOps) LogSecurityEvent(userID int64, event, details string) error {
	// Log security events to the authorization log
	service := NewEmailAuthorizationService()
	user, err := GetUser(userID)
	if err != nil {
		return err
	}

	// Create a context for the log entry
	ctx := context.Background()
	return service.LogAuthorizationAttempt(ctx, user.Username, event, "security_event", &userID, details)
}

// User represents the user model for gophish.
type User struct {
	Id                     int64     `json:"id"`
	Username               string    `json:"username" sql:"not null;unique"`
	Hash                   string    `json:"-"`
	ApiKey                 string    `json:"api_key" sql:"not null;unique"`
	Role                   Role      `json:"role" gorm:"association_autoupdate:false;association_autocreate:false"`
	RoleID                 int64     `json:"-"`
	PasswordChangeRequired bool      `json:"password_change_required"`
	AccountLocked          bool      `json:"account_locked"`
	LastLogin              time.Time `json:"last_login"`
	// OAuth fields for SSO integration
	OAuthProvider          string    `json:"oauth_provider,omitempty" gorm:"column:oauth_provider"`
	OAuthID                string    `json:"oauth_id,omitempty" gorm:"column:oauth_id"`
}

// GetUser returns the user that the given id corresponds to. If no user is found, an
// error is thrown.
func GetUser(id int64) (User, error) {
	u := User{}
	err := db.Preload("Role").Where("id=?", id).First(&u).Error
	return u, err
}

// GetUsers returns the users registered in Gophish
func GetUsers() ([]User, error) {
	us := []User{}
	err := db.Preload("Role").Find(&us).Error
	return us, err
}

// GetUserByAPIKey returns the user that the given API Key corresponds to. If no user is found, an
// error is thrown.
func GetUserByAPIKey(key string) (User, error) {
	u := User{}
	err := db.Preload("Role").Where("api_key = ?", key).First(&u).Error
	return u, err
}

// GetUserByUsername returns the user that the given username corresponds to. If no user is found, an
// error is thrown.
func GetUserByUsername(username string) (User, error) {
	u := User{}
	err := db.Preload("Role").Where("username = ?", username).First(&u).Error
	return u, err
}

// PutUser updates the given user
func PutUser(u *User) error {
	err := db.Save(u).Error
	return err
}

// EnsureEnoughAdmins ensures that there is more than one user account in
// Gophish with the Admin role. This function is meant to be called before
// modifying a user account with the Admin role in a non-revokable way.
func EnsureEnoughAdmins() error {
	role, err := GetRoleBySlug(RoleAdmin)
	if err != nil {
		return err
	}
	var adminCount int
	err = db.Model(&User{}).Where("role_id=?", role.ID).Count(&adminCount).Error
	if err != nil {
		return err
	}
	if adminCount == 1 {
		return ErrModifyingOnlyAdmin
	}
	return nil
}

// DeleteUser deletes the given user. To ensure that there is always at least
// one user account with the Admin role, this function will refuse to delete
// the last Admin.
func DeleteUser(id int64) error {
	existing, err := GetUser(id)
	if err != nil {
		return err
	}
	// If the user is an admin, we need to verify that it's not the last one.
	if existing.Role.Slug == RoleAdmin {
		err = EnsureEnoughAdmins()
		if err != nil {
			return err
		}
	}
	campaigns, err := GetCampaigns(id)
	if err != nil {
		return err
	}
	// Delete the campaigns
	log.Infof("Deleting campaigns for user ID %d", id)
	for _, campaign := range campaigns {
		err = DeleteCampaign(campaign.Id)
		if err != nil {
			return err
		}
	}
	log.Infof("Deleting pages for user ID %d", id)
	// Delete the landing pages
	pages, err := GetPages(id)
	if err != nil {
		return err
	}
	for _, page := range pages {
		err = DeletePage(page.Id, id)
		if err != nil {
			return err
		}
	}
	// Delete the templates
	log.Infof("Deleting templates for user ID %d", id)
	templates, err := GetTemplates(id)
	if err != nil {
		return err
	}
	for _, template := range templates {
		err = DeleteTemplate(template.Id, id)
		if err != nil {
			return err
		}
	}
	// Delete the groups
	log.Infof("Deleting groups for user ID %d", id)
	groups, err := GetGroups(id)
	if err != nil {
		return err
	}
	for _, group := range groups {
		err = DeleteGroup(&group)
		if err != nil {
			return err
		}
	}
	// Delete the sending profiles
	log.Infof("Deleting sending profiles for user ID %d", id)
	profiles, err := GetSMTPs(id)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		err = DeleteSMTP(profile.Id, id)
		if err != nil {
			return err
		}
	}
	// Finally, delete the user
	err = db.Where("id=?", id).Delete(&User{}).Error
	return err
}

// GetUserByOAuthID returns the user that corresponds to the given OAuth provider and ID.
// If no user is found, an error is thrown.
func GetUserByOAuthID(provider, oauthID string) (User, error) {
	u := User{}
	err := db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&u).Error
	return u, err
}

// PostUser creates a new user in the database.
func PostUser(u *User) error {
	return db.Save(u).Error
}

// FindOrCreateOAuthUser finds an existing OAuth user or creates a new one
func FindOrCreateOAuthUser(provider, oauthID, email string) (User, error) {
	// First, try to find user by OAuth provider and ID
	existingUser, err := GetUserByOAuthID(provider, oauthID)
	if err == nil {
		// User exists, update info if needed
		needsUpdate := false
		if existingUser.Username != email {
			existingUser.Username = email
			needsUpdate = true
		}

		// Check if this is the admin email and update role accordingly
		if isAdminEmail(email) && existingUser.Role.Slug != RoleAdmin {
			adminRole, roleErr := GetRoleBySlug(RoleAdmin)
			if roleErr == nil {
				existingUser.RoleID = adminRole.ID
				needsUpdate = true
			}
		}

		if needsUpdate {
			if err := PutUser(&existingUser); err != nil {
				return User{}, fmt.Errorf("failed to update existing user: %w", err)
			}
		}
		return existingUser, nil
	}

	// If not found by OAuth ID, check if user exists by email
	existingUser, err = GetUserByUsername(email)
	if err == nil {
		// User exists with this email but no OAuth link
		// Link this OAuth account to existing user
		existingUser.OAuthProvider = provider
		existingUser.OAuthID = oauthID

		// Check if this is the admin email and update role accordingly
		if isAdminEmail(email) && existingUser.Role.Slug != RoleAdmin {
			adminRole, roleErr := GetRoleBySlug(RoleAdmin)
			if roleErr == nil {
				existingUser.RoleID = adminRole.ID
			}
		}

		if err := PutUser(&existingUser); err != nil {
			return User{}, fmt.Errorf("failed to link existing user to OAuth: %w", err)
		}
		return existingUser, nil
	}

	// Create new user - generate API key using existing secure key generation
	apiKey := generateSecureKey() // Use existing function from models.go

	// Determine role based on email
	defaultRole := "user"
	defaultRoleID := int64(2) // Default user role ID (assuming 1=admin, 2=user)

	if isAdminEmail(email) {
		// Grant admin privileges to the special admin email
		adminRole, roleErr := GetRoleBySlug(RoleAdmin)
		if roleErr == nil {
			defaultRole = RoleAdmin
			defaultRoleID = adminRole.ID
		}
	}

	// Create new user
	newUser := User{
		Username:      email,
		Hash:          "", // OAuth users don't have passwords
		ApiKey:        apiKey,
		OAuthProvider: provider,
		OAuthID:       oauthID,
		Role: Role{
			Name: defaultRole,
		},
		RoleID: defaultRoleID,
	}

	if err := PostUser(&newUser); err != nil {
		return User{}, fmt.Errorf("failed to create new OAuth user: %w", err)
	}

	// Reload user with proper role relationship
	return GetUser(newUser.Id)
}

// isAdminEmail checks if the provided email should receive admin privileges
func isAdminEmail(email string) bool {
	// Load configuration to check admin emails
	cfg, err := config.LoadConfigWithSSO("config.json")
	if err != nil {
		log.Warnf("Failed to load config for admin email check: %v", err)
		return false
	}

	return cfg.IsAdminEmail(email)
}

// EnsureAdminEmailAuthorization ensures that admin emails are properly authorized in the system
func EnsureAdminEmailAuthorization() error {
	// Load configuration to get admin emails
	cfg, err := config.LoadConfigWithSSO("config.json")
	if err != nil {
		log.Warnf("Failed to load config for admin email authorization: %v", err)
		return err
	}

	adminEmails := cfg.GetAdminEmails()
	if len(adminEmails) == 0 {
		log.Warn("No admin emails configured in config.json or environment variables")
		return nil
	}

	// Get admin role
	adminRole, err := GetRoleBySlug(RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to get admin role: %w", err)
	}

	service := NewEmailAuthorizationService()

	for _, email := range adminEmails {
		// Check if admin email is already authorized
		result, err := service.CheckEmailAuthorization(email)
		if err != nil || !result.Authorized {
			// Add admin email to authorized list
			_, err := AddAuthorizedEmail(
				email,
				&adminRole.ID,
				"admin",
				nil, // System-created, no user ID
				nil, // No expiration
				"System admin email for Microsoft SSO authentication",
			)
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint") {
				log.Errorf("Failed to add admin email to authorized list: %v", err)
				return fmt.Errorf("failed to authorize admin email: %w", err)
			}
		} else {
			// Update existing authorization to ensure admin role
			if result.GetRole() != "admin" {
				// Find the authorized email record and update it
				emails, err := GetAuthorizedEmails("active", 1, 0)
				if err == nil {
					for _, authEmail := range emails {
						if strings.EqualFold(authEmail.Email, email) {
							authEmail.RoleID = &adminRole.ID
							authEmail.DefaultRole = "admin"

							// Update the record using direct database operation
							if updateErr := db.Save(&authEmail).Error; updateErr != nil {
								log.Errorf("Failed to update admin email role: %v", updateErr)
							}
							break
						}
					}
				}
			}
		}
	}

	return nil
}
