package models

import (
	"context"
	"strings"

	"github.com/gophish/gophish/config"
	"gopkg.in/check.v1"
)

// EmailAuthorizationSuite extends the existing ModelsSuite
type EmailAuthorizationSuite struct {
	service *EmailAuthorizationService
}

var _ = check.Suite(&EmailAuthorizationSuite{})

func (s *EmailAuthorizationSuite) SetUpSuite(c *check.C) {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../../../db/db_sqlite3/migrations/",
	}
	err := Setup(conf)
	if err != nil {
		c.Fatalf("Failed creating database: %v", err)
	}
}

func (s *EmailAuthorizationSuite) SetUpTest(c *check.C) {
	s.service = NewEmailAuthorizationService()

	// Ensure test database is clean
	db.Delete(&EmailAuthorizationLog{}, "1=1")
	db.Delete(&AuthorizedEmail{}, "1=1")
	db.Delete(&AuthorizedDomain{}, "1=1")
}

func (s *EmailAuthorizationSuite) TearDownTest(c *check.C) {
	// Clean up test data
	db.Delete(&EmailAuthorizationLog{}, "1=1")
	db.Delete(&AuthorizedEmail{}, "1=1")
	db.Delete(&AuthorizedDomain{}, "1=1")
}

func (s *EmailAuthorizationSuite) TestNormalizeEmail(c *check.C) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Test@Example.Com", "test@example.com"},
		{"  user@domain.org  ", "user@domain.org"},
		{"ADMIN@COMPANY.NET", "admin@company.net"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := s.service.NormalizeEmail(tc.input)
		c.Assert(result, check.Equals, tc.expected)
	}
}

func (s *EmailAuthorizationSuite) TestValidateEmailFormat(c *check.C) {
	validEmails := []string{
		"user@example.com",
		"test.user@domain.org",
		"admin+test@company.net",
		"user123@test-domain.co.uk",
	}

	invalidEmails := []string{
		"",
		"invalid",
		"@domain.com",
		"user@",
		"user@@domain.com",
		"user@domain@com",
		strings.Repeat("a", 250) + "@domain.com", // Too long
	}

	for _, email := range validEmails {
		err := s.service.ValidateEmailFormat(email)
		c.Assert(err, check.IsNil, check.Commentf("Valid email rejected: %s", email))
	}

	for _, email := range invalidEmails {
		err := s.service.ValidateEmailFormat(email)
		c.Assert(err, check.NotNil, check.Commentf("Invalid email accepted: %s", email))
	}
}

func (s *EmailAuthorizationSuite) TestCheckEmailAuthorization(c *check.C) {
	// Test invalid email format
	result, err := s.service.CheckEmailAuthorization("invalid-email")
	c.Assert(err, check.IsNil)
	c.Assert(result.Authorized, check.Equals, false)
	c.Assert(result.Reason, check.Equals, "invalid_format")

	// Test unauthorized email
	result, err = s.service.CheckEmailAuthorization("unauthorized@example.com")
	c.Assert(err, check.IsNil)
	c.Assert(result.Authorized, check.Equals, false)
	c.Assert(result.Reason, check.Equals, "not_authorized")
}

func (s *EmailAuthorizationSuite) TestLogAuthorizationAttempt(c *check.C) {
	email := "test@example.com"
	action := "login_attempt"
	result := "success"
	details := "OAuth login successful"

	ctx := context.Background()
	ctx = context.WithValue(ctx, "ip", "192.168.1.100")
	ctx = context.WithValue(ctx, "user_agent", "Test-Agent/1.0")

	err := s.service.LogAuthorizationAttempt(ctx, email, action, result, nil, details)
	c.Assert(err, check.IsNil)

	// Verify log was created
	var logs []EmailAuthorizationLog
	err = db.Where("email = ?", email).Find(&logs).Error
	c.Assert(err, check.IsNil)
	c.Assert(len(logs), check.Equals, 1)

	log := logs[0]
	c.Assert(log.Email, check.Equals, email)
	c.Assert(log.NormalizedEmail, check.Equals, "test@example.com")
	c.Assert(log.Action, check.Equals, action)
	c.Assert(log.Result, check.Equals, result)
	c.Assert(log.IPAddress, check.Equals, "192.168.1.100")
	c.Assert(log.UserAgent, check.Equals, "Test-Agent/1.0")
	c.Assert(log.Details, check.Equals, details)
}

func (s *EmailAuthorizationSuite) TestAddAuthorizedEmail(c *check.C) {
	email := "test@example.com"
	defaultRole := "user"
	notes := "Test authorized email"

	authEmail, err := AddAuthorizedEmail(email, nil, defaultRole, nil, nil, notes)
	c.Assert(err, check.IsNil)
	c.Assert(authEmail.Email, check.Equals, email)
	c.Assert(authEmail.DefaultRole, check.Equals, defaultRole)
	c.Assert(authEmail.Status, check.Equals, "active")
	c.Assert(authEmail.Notes, check.Equals, notes)
}

func (s *EmailAuthorizationSuite) TestAddAuthorizedEmailDuplicateEmail(c *check.C) {
	email := "duplicate@example.com"

	// Add first email
	_, err := AddAuthorizedEmail(email, nil, "user", nil, nil, "First entry")
	c.Assert(err, check.IsNil)

	// Try to add duplicate
	_, err = AddAuthorizedEmail(email, nil, "admin", nil, nil, "Duplicate entry")
	c.Assert(err, check.NotNil)
}

func (s *EmailAuthorizationSuite) TestAddAuthorizedEmailInvalidFormat(c *check.C) {
	invalidEmail := "invalid-email-format"

	_, err := AddAuthorizedEmail(invalidEmail, nil, "user", nil, nil, "")
	c.Assert(err, check.NotNil)
}

func (s *EmailAuthorizationSuite) TestGetAuthorizedEmails(c *check.C) {
	// Add test emails
	_, err := AddAuthorizedEmail("active@example.com", nil, "user", nil, nil, "Active email")
	c.Assert(err, check.IsNil)

	inactiveEmail, err := AddAuthorizedEmail("inactive@example.com", nil, "user", nil, nil, "Inactive email")
	c.Assert(err, check.IsNil)

	// Deactivate one email
	err = UpdateAuthorizedEmailStatus(inactiveEmail.Id, "inactive", nil)
	c.Assert(err, check.IsNil)

	// Get all emails
	emails, err := GetAuthorizedEmails("", 0, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(emails), check.Equals, 2)

	// Get only active emails
	emails, err = GetAuthorizedEmails("active", 0, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(emails), check.Equals, 1)
	c.Assert(emails[0].Email, check.Equals, "active@example.com")
}

func (s *EmailAuthorizationSuite) TestDeleteAuthorizedEmail(c *check.C) {
	email := "delete@example.com"

	// Add authorized email
	authEmail, err := AddAuthorizedEmail(email, nil, "user", nil, nil, "Test")
	c.Assert(err, check.IsNil)

	// Delete the email
	err = DeleteAuthorizedEmail(authEmail.Id)
	c.Assert(err, check.IsNil)

	// Verify deletion - try to get emails back
	emails, err := GetAuthorizedEmails("", 0, 0)
	c.Assert(err, check.IsNil)

	for _, email := range emails {
		c.Assert(email.Id, check.Not(check.Equals), authEmail.Id)
	}
}

func (s *EmailAuthorizationSuite) TestGetAuthorizationLogs(c *check.C) {
	ctx := context.Background()

	testLogs := []struct {
		email  string
		action string
		result string
	}{
		{"user1@example.com", "login", "success"},
		{"user1@example.com", "login", "failed"},
		{"user2@example.com", "register", "success"},
		{"user3@example.com", "login", "success"},
	}

	for _, testLog := range testLogs {
		err := s.service.LogAuthorizationAttempt(ctx, testLog.email, testLog.action, testLog.result, nil, "Test log")
		c.Assert(err, check.IsNil)
	}

	// Get all logs
	logs, err := GetAuthorizationLogs("", "", "", 0, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(logs), check.Equals, 4)

	// Filter by email
	logs, err = GetAuthorizationLogs("user1@example.com", "", "", 0, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(logs), check.Equals, 2)

	// Filter by action
	logs, err = GetAuthorizationLogs("", "login", "", 0, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(logs), check.Equals, 3)
}

func (s *EmailAuthorizationSuite) TestEmailAuthorizationPerformance(c *check.C) {
	// Add test email for performance testing
	_, err := AddAuthorizedEmail("performance@example.com", nil, "user", nil, nil, "Performance test")
	c.Assert(err, check.IsNil)

	// Test authorization check
	result, err := s.service.CheckEmailAuthorization("performance@example.com")
	c.Assert(err, check.IsNil)
	c.Assert(result.Authorized, check.Equals, true)
}

func (s *EmailAuthorizationSuite) TestConcurrentEmailAuthorization(c *check.C) {
	email := "concurrent@example.com"
	_, err := AddAuthorizedEmail(email, nil, "user", nil, nil, "Concurrent test")
	c.Assert(err, check.IsNil)

	// Test concurrent authorization checks
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := s.service.CheckEmailAuthorization(email)
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < 5; i++ {
		err := <-results
		c.Assert(err, check.IsNil)
	}
}

func (s *EmailAuthorizationSuite) TestEmailAuthorizationWithUnicodeEmails(c *check.C) {
	unicodeEmails := []string{
		"tëst@exàmple.org",
		"test@exãmple.org",
	}

	for _, email := range unicodeEmails {
		// Test that Unicode emails are handled properly
		err := s.service.ValidateEmailFormat(email)
		c.Assert(err, check.IsNil, check.Commentf("Unicode email validation failed: %s", email))

		normalized := s.service.NormalizeEmail(email)
		c.Assert(normalized, check.Equals, strings.ToLower(email))
	}
}