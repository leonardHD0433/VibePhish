package models

import (
	"time"

	"github.com/gophish/gophish/auth"
	"github.com/jinzhu/gorm"
	"gopkg.in/check.v1"
)

// UserOAuthSuite extends the existing ModelsSuite for OAuth-specific user tests
type UserOAuthSuite struct{}

var _ = check.Suite(&UserOAuthSuite{})

func (s *UserOAuthSuite) SetUpTest(c *check.C) {
	// Ensure clean state for OAuth user tests
	db.Delete(&User{}, "username LIKE 'oauth_%'")
}

func (s *UserOAuthSuite) TearDownTest(c *check.C) {
	// Clean up OAuth test users
	db.Delete(&User{}, "username LIKE 'oauth_%'")
}

// Test FindOrCreateOAuthUser functionality

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserNew(c *check.C) {
	provider := "microsoft"
	oauthID := "oauth-user-id-12345"
	email := "oauth.test@example.com"

	user, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(user.Username, check.Equals, email)
	c.Assert(user.AccountLocked, check.Equals, false)

	// Should have default role
	c.Assert(user.Role.Slug, check.Equals, RoleUser)

	// Verify OAuth fields are set correctly
	c.Assert(user.OAuthProvider, check.Equals, provider)
	c.Assert(user.OAuthID, check.Equals, oauthID)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserExisting(c *check.C) {
	provider := "microsoft"
	oauthID := "existing-oauth-id-12345"
	email := "existing.oauth@example.com"

	// Create user first time
	user1, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	originalID := user1.Id

	// Find same user second time
	user2, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(user2.Id, check.Equals, originalID)
	c.Assert(user2.Username, check.Equals, email)
	c.Assert(user2.OAuthProvider, check.Equals, provider)
	c.Assert(user2.OAuthID, check.Equals, oauthID)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserUpdatesEmail(c *check.C) {
	provider := "microsoft"
	oauthID := "email-update-test-12345"
	originalEmail := "original@example.com"
	updatedEmail := "updated@example.com"

	// Create user with original email
	user1, err := FindOrCreateOAuthUser(provider, oauthID, originalEmail)
	c.Assert(err, check.IsNil)
	originalID := user1.Id

	// Update with new email
	user2, err := FindOrCreateOAuthUser(provider, oauthID, updatedEmail)
	c.Assert(err, check.IsNil)
	c.Assert(user2.Id, check.Equals, originalID)
	c.Assert(user2.Username, check.Equals, updatedEmail)
	c.Assert(user2.Username, check.Equals, updatedEmail) // Username should also update
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserAdminPrivileges(c *check.C) {
	provider := "microsoft"
	oauthID := "admin-oauth-id-12345"
	adminEmail := "admin.oauth@example.com"

	// Temporarily add admin email to configuration
	// Note: In real implementation, this would be checked via config
	// For testing, we'll create an admin role user manually

	// Get admin role
	adminRole, err := GetRoleBySlug(RoleAdmin)
	c.Assert(err, check.IsNil)

	// Create OAuth user
	user, err := FindOrCreateOAuthUser(provider, oauthID, adminEmail)
	c.Assert(err, check.IsNil)

	// Manually assign admin role for this test
	user.RoleID = adminRole.ID
	user.Role = adminRole
	err = PutUser(&user)
	c.Assert(err, check.IsNil)

	// Verify admin role assignment
	retrievedUser, err := GetUser(user.Id)
	c.Assert(err, check.IsNil)
	c.Assert(retrievedUser.Role.Slug, check.Equals, RoleAdmin)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserEmptyProvider(c *check.C) {
	provider := ""
	oauthID := "empty-provider-test"
	email := "test@example.com"

	_, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.NotNil)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserEmptyOAuthID(c *check.C) {
	provider := "microsoft"
	oauthID := ""
	email := "test@example.com"

	_, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.NotNil)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserInvalidEmail(c *check.C) {
	provider := "microsoft"
	oauthID := "invalid-email-test"
	email := "invalid-email"

	_, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.NotNil)
}

func (s *UserOAuthSuite) TestFindOrCreateOAuthUserDuplicateProviderIDs(c *check.C) {
	// Test different providers with same OAuth ID
	microsoftOAuthID := "duplicate-id-12345"
	googleOAuthID := "duplicate-id-12345"
	email1 := "microsoft.user@example.com"
	email2 := "google.user@example.com"

	// Create Microsoft OAuth user
	user1, err := FindOrCreateOAuthUser("microsoft", microsoftOAuthID, email1)
	c.Assert(err, check.IsNil)

	// Create Google OAuth user with same OAuth ID (should be allowed)
	user2, err := FindOrCreateOAuthUser("google", googleOAuthID, email2)
	c.Assert(err, check.IsNil)

	// Should be different users
	c.Assert(user1.Id, check.Not(check.Equals), user2.Id)
	c.Assert(user1.OAuthProvider, check.Equals, "microsoft")
	c.Assert(user2.OAuthProvider, check.Equals, "google")
}

// Test OAuth User Operations Interface

func (s *UserOAuthSuite) TestOAuthUserOperationsInterface(c *check.C) {
	ops := GetOAuthUserOperations()
	c.Assert(ops, check.NotNil)

	provider := "microsoft"
	oauthID := "interface-test-12345"
	email := "interface.test@example.com"

	// Test FindOrCreateUser
	userID, username, accountLocked, isAdmin, err := ops.FindOrCreateUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(userID, check.Not(check.Equals), int64(0))
	c.Assert(username, check.Equals, email)
	c.Assert(accountLocked, check.Equals, false)
	c.Assert(isAdmin, check.Equals, false)

	// Test UpdateLastLogin
	err = ops.UpdateLastLogin(userID)
	c.Assert(err, check.IsNil)

	// Verify last login was updated
	user, err := GetUser(userID)
	c.Assert(err, check.IsNil)
	c.Assert(user.LastLogin.IsZero(), check.Equals, false)

	// Test ValidateAdminPrivilege
	isAdminValidated, err := ops.ValidateAdminPrivilege(userID)
	c.Assert(err, check.IsNil)
	c.Assert(isAdminValidated, check.Equals, false)

	// Test LogSecurityEvent
	err = ops.LogSecurityEvent(userID, "test_event", "Test security event details")
	c.Assert(err, check.IsNil)
}

func (s *UserOAuthSuite) TestOAuthUserOperationsAdminUser(c *check.C) {
	ops := GetOAuthUserOperations()

	provider := "microsoft"
	oauthID := "admin-interface-test-12345"
	email := "admin.interface@example.com"

	// Create admin user
	userID, username, accountLocked, isAdmin, err := ops.FindOrCreateUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(isAdmin, check.Equals, false) // Initially not admin

	// Manually assign admin role
	adminRole, err := GetRoleBySlug(RoleAdmin)
	c.Assert(err, check.IsNil)

	user, err := GetUser(userID)
	c.Assert(err, check.IsNil)
	user.RoleID = adminRole.ID
	user.Role = adminRole
	err = PutUser(&user)
	c.Assert(err, check.IsNil)

	// Test admin validation
	isAdminValidated, err := ops.ValidateAdminPrivilege(userID)
	c.Assert(err, check.IsNil)
	c.Assert(isAdminValidated, check.Equals, true)

	// Verify admin status in user creation result
	userID2, username2, accountLocked2, isAdmin2, err := ops.FindOrCreateUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(userID2, check.Equals, userID)
	c.Assert(username2, check.Equals, username)
	c.Assert(accountLocked2, check.Equals, accountLocked)
	c.Assert(isAdmin2, check.Equals, true) // Should now be true
}

func (s *UserOAuthSuite) TestOAuthUserOperationsAccountLocked(c *check.C) {
	ops := GetOAuthUserOperations()

	provider := "microsoft"
	oauthID := "locked-interface-test-12345"
	email := "locked.interface@example.com"

	// Create user first
	userID, _, _, _, err := ops.FindOrCreateUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)

	// Lock the account
	user, err := GetUser(userID)
	c.Assert(err, check.IsNil)
	user.AccountLocked = true
	err = PutUser(&user)
	c.Assert(err, check.IsNil)

	// Verify locked status is returned
	_, _, accountLocked, _, err := ops.FindOrCreateUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(accountLocked, check.Equals, true)
}

func (s *UserOAuthSuite) TestOAuthUserOperationsNonExistentUser(c *check.C) {
	ops := GetOAuthUserOperations()

	nonExistentUserID := int64(99999)

	// Test UpdateLastLogin with non-existent user
	err := ops.UpdateLastLogin(nonExistentUserID)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)

	// Test ValidateAdminPrivilege with non-existent user
	_, err = ops.ValidateAdminPrivilege(nonExistentUserID)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}

// Test security-specific scenarios

func (s *UserOAuthSuite) TestOAuthUserSecurityScenarios(c *check.C) {
	provider := "microsoft"

	// Test long OAuth ID
	longOAuthID := string(make([]byte, 1000)) // Very long ID
	for i := range longOAuthID {
		longOAuthID = longOAuthID[:i] + "a" + longOAuthID[i+1:]
	}

	_, err := FindOrCreateOAuthUser(provider, longOAuthID, "test@example.com")
	// Should handle long IDs appropriately (may truncate or reject)
	// Implementation dependent on database schema constraints

	// Test special characters in email
	specialEmail := "test+tag@example.com"
	user, err := FindOrCreateOAuthUser(provider, "special-chars-test", specialEmail)
	c.Assert(err, check.IsNil)
	c.Assert(user.Username, check.Equals, specialEmail)

	// Test Unicode characters in email
	unicodeEmail := "tëst@éxample.com"
	user, err = FindOrCreateOAuthUser(provider, "unicode-test", unicodeEmail)
	c.Assert(err, check.IsNil)
	c.Assert(user.Username, check.Equals, unicodeEmail)
}

func (s *UserOAuthSuite) TestOAuthUserConcurrentCreation(c *check.C) {
	provider := "microsoft"
	oauthID := "concurrent-test-12345"
	email := "concurrent.test@example.com"

	// Simulate concurrent user creation
	results := make(chan error, 10)
	userIDs := make(chan int64, 10)

	for i := 0; i < 10; i++ {
		go func() {
			user, err := FindOrCreateOAuthUser(provider, oauthID, email)
			results <- err
			if err == nil {
				userIDs <- user.Id
			} else {
				userIDs <- 0
			}
		}()
	}

	// Collect results
	var firstUserID int64
	errorCount := 0
	for i := 0; i < 10; i++ {
		err := <-results
		userID := <-userIDs

		if err != nil {
			errorCount++
		} else if firstUserID == 0 {
			firstUserID = userID
		} else {
			// All successful operations should return the same user ID
			c.Assert(userID, check.Equals, firstUserID)
		}
	}

	// Should have minimal errors due to race conditions
	c.Assert(errorCount <= 5, check.Equals, true, check.Commentf("Too many concurrent creation errors: %d", errorCount))
}

// Test performance scenarios

func (s *UserOAuthSuite) TestOAuthUserPerformance(c *check.C) {
	provider := "microsoft"

	// Test performance of user lookup vs creation
	start := time.Now()

	// Create many OAuth users
	for i := 0; i < 100; i++ {
		oauthID := "perf-test-" + string(rune(i))
		email := "perftest" + string(rune(i)) + "@example.com"
		_, err := FindOrCreateOAuthUser(provider, oauthID, email)
		c.Assert(err, check.IsNil)
	}

	creationTime := time.Since(start)

	// Test lookup performance
	start = time.Now()
	for i := 0; i < 100; i++ {
		oauthID := "perf-test-" + string(rune(i))
		email := "perftest" + string(rune(i)) + "@example.com"
		_, err := FindOrCreateOAuthUser(provider, oauthID, email)
		c.Assert(err, check.IsNil)
	}

	lookupTime := time.Since(start)

	// Lookup should be faster than creation
	c.Assert(lookupTime < creationTime, check.Equals, true,
		check.Commentf("Lookup time (%v) should be less than creation time (%v)", lookupTime, creationTime))
}

// Test edge cases and error conditions

func (s *UserOAuthSuite) TestOAuthUserEdgeCases(c *check.C) {
	// Test empty string inputs
	testCases := []struct {
		provider string
		oauthID  string
		email    string
		shouldFail bool
	}{
		{"", "valid-id", "valid@example.com", true},
		{"microsoft", "", "valid@example.com", true},
		{"microsoft", "valid-id", "", true},
		{"microsoft", "valid-id", "invalid-email", true},
		{"microsoft", "valid-id", "valid@example.com", false},
	}

	for _, tc := range testCases {
		_, err := FindOrCreateOAuthUser(tc.provider, tc.oauthID, tc.email)
		if tc.shouldFail {
			c.Assert(err, check.NotNil, check.Commentf("Expected failure for: %+v", tc))
		} else {
			c.Assert(err, check.IsNil, check.Commentf("Expected success for: %+v", tc))
		}
	}
}

func (s *UserOAuthSuite) TestOAuthUserRoleConsistency(c *check.C) {
	provider := "microsoft"
	oauthID := "role-consistency-test"
	email := "role.test@example.com"

	// Create user with default role
	user, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(user.Role.Slug, check.Equals, RoleUser)

	// Manually change role
	adminRole, err := GetRoleBySlug(RoleAdmin)
	c.Assert(err, check.IsNil)
	user.RoleID = adminRole.ID
	user.Role = adminRole
	err = PutUser(&user)
	c.Assert(err, check.IsNil)

	// Find user again - role should be preserved
	user2, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)
	c.Assert(user2.Id, check.Equals, user.Id)
	c.Assert(user2.Role.Slug, check.Equals, RoleAdmin)
}

// Test integration with existing user management

func (s *UserOAuthSuite) TestOAuthUserIntegrationWithExistingUsers(c *check.C) {
	// Create a regular (non-OAuth) user first
	userRole, err := GetRoleBySlug(RoleUser)
	c.Assert(err, check.IsNil)

	regularUser := User{
		Username:   "regular@example.com",
		Hash:       "password-hash",
		ApiKey:     auth.GenerateSecureKey(32),
		Role:       userRole,
		RoleID:     userRole.ID,
	}
	err = PutUser(&regularUser)
	c.Assert(err, check.IsNil)

	// Try to create OAuth user with same email
	provider := "microsoft"
	oauthID := "integration-test-12345"
	email := "regular@example.com"

	oauthUser, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)

	// Should create a new user, not conflict with existing
	c.Assert(oauthUser.Id, check.Not(check.Equals), regularUser.Id)
	c.Assert(oauthUser.OAuthProvider, check.Equals, provider)
	c.Assert(oauthUser.OAuthID, check.Equals, oauthID)
}

// Test API key generation for OAuth users

func (s *UserOAuthSuite) TestOAuthUserAPIKeyGeneration(c *check.C) {
	provider := "microsoft"
	oauthID := "apikey-test-12345"
	email := "apikey.test@example.com"

	user, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)

	// API key should be generated
	c.Assert(user.ApiKey, check.Not(check.Equals), "")
	c.Assert(len(user.ApiKey), check.Equals, 32)

	// API key should be unique
	user2, err := FindOrCreateOAuthUser("google", "different-id", "different@example.com")
	c.Assert(err, check.IsNil)
	c.Assert(user2.ApiKey, check.Not(check.Equals), user.ApiKey)
}

// Test compatibility with existing ModelsSuite tests

func (s *ModelsSuite) TestOAuthUserCompatibility(c *check.C) {
	// Test that OAuth users work with existing user operations
	provider := "microsoft"
	oauthID := "compatibility-test-12345"
	email := "compatibility@example.com"

	// Create OAuth user
	oauthUser, err := FindOrCreateOAuthUser(provider, oauthID, email)
	c.Assert(err, check.IsNil)

	// Test with existing GetUser function
	retrievedUser, err := GetUser(oauthUser.Id)
	c.Assert(err, check.IsNil)
	c.Assert(retrievedUser.Id, check.Equals, oauthUser.Id)
	c.Assert(retrievedUser.Username, check.Equals, email)

	// Test with existing GetUserByAPIKey function
	foundUser, err := GetUserByAPIKey(oauthUser.ApiKey)
	c.Assert(err, check.IsNil)
	c.Assert(foundUser.Id, check.Equals, oauthUser.Id)

	// Test with existing PutUser function
	oauthUser.AccountLocked = true
	err = PutUser(&oauthUser)
	c.Assert(err, check.IsNil)

	// Verify update
	updatedUser, err := GetUser(oauthUser.Id)
	c.Assert(err, check.IsNil)
	c.Assert(updatedUser.AccountLocked, check.Equals, true)
}