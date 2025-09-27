package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gophish/gophish/config"
	"golang.org/x/oauth2"
	"gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type OAuthSuite struct{}

var _ = check.Suite(&OAuthSuite{})

// Mock UserOperationsProvider for testing
type mockUserOperationsProvider struct {
	findOrCreateUserFunc    func(provider, oauthID, email string) (int64, string, bool, bool, error)
	updateLastLoginFunc     func(userID int64) error
	validateAdminPrivilegeFunc func(userID int64) (bool, error)
	logSecurityEventFunc   func(userID int64, event, details string) error
}

func (m *mockUserOperationsProvider) FindOrCreateUser(provider, oauthID, email string) (int64, string, bool, bool, error) {
	if m.findOrCreateUserFunc != nil {
		return m.findOrCreateUserFunc(provider, oauthID, email)
	}
	return 1, "test-user", false, true, nil
}

func (m *mockUserOperationsProvider) UpdateLastLogin(userID int64) error {
	if m.updateLastLoginFunc != nil {
		return m.updateLastLoginFunc(userID)
	}
	return nil
}

func (m *mockUserOperationsProvider) ValidateAdminPrivilege(userID int64) (bool, error) {
	if m.validateAdminPrivilegeFunc != nil {
		return m.validateAdminPrivilegeFunc(userID)
	}
	return true, nil
}

func (m *mockUserOperationsProvider) LogSecurityEvent(userID int64, event, details string) error {
	if m.logSecurityEventFunc != nil {
		return m.logSecurityEventFunc(userID, event, details)
	}
	return nil
}

// Mock OAuth provider for testing
type mockOAuthProvider struct {
	providerName     string
	authURL          string
	userInfo         *OAuthUserInfo
	exchangeError    error
	userInfoError    error
}

func (m *mockOAuthProvider) GetAuthURL(state string, opts ...oauth2.AuthCodeOption) string {
	return m.authURL + "?state=" + state
}

func (m *mockOAuthProvider) GetAuthURLWithPKCE(state string, pkce *PKCEChallenge) string {
	return m.authURL + "?state=" + state + "&code_challenge=" + pkce.CodeChallenge
}

func (m *mockOAuthProvider) ExchangeCode(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if m.exchangeError != nil {
		return nil, m.exchangeError
	}
	return &oauth2.Token{AccessToken: "test-token"}, nil
}

func (m *mockOAuthProvider) ExchangeCodeWithPKCE(ctx context.Context, code string, pkce *PKCEChallenge) (*oauth2.Token, error) {
	if m.exchangeError != nil {
		return nil, m.exchangeError
	}
	return &oauth2.Token{AccessToken: "test-token"}, nil
}

func (m *mockOAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	if m.userInfoError != nil {
		return nil, m.userInfoError
	}
	return m.userInfo, nil
}

func (m *mockOAuthProvider) GetConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3333/auth/microsoft/callback",
	}
}

func (m *mockOAuthProvider) GetProviderName() string {
	return m.providerName
}

func (m *mockOAuthProvider) ValidateDomain(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	emailParts := strings.Split(email, "@")
	if len(emailParts) != 2 {
		return false
	}
	domain := strings.ToLower(emailParts[1])
	for _, allowedDomain := range allowedDomains {
		if strings.ToLower(allowedDomain) == domain {
			return true
		}
	}
	return false
}

// PKCE Security Tests
func (s *OAuthSuite) TestGeneratePKCE(c *check.C) {
	challenge, err := GeneratePKCE()
	c.Assert(err, check.IsNil)
	c.Assert(challenge, check.NotNil)

	// Verify PKCE challenge properties
	c.Assert(challenge.CodeVerifier, check.Not(check.Equals), "")
	c.Assert(challenge.CodeChallenge, check.Not(check.Equals), "")
	c.Assert(challenge.Method, check.Equals, "S256")

	// Verify code verifier is base64url encoded and appropriate length
	c.Assert(len(challenge.CodeVerifier), check.Not(check.Equals), 0)
	c.Assert(len(challenge.CodeChallenge), check.Not(check.Equals), 0)

	// Verify no padding characters (PKCE spec requirement)
	c.Assert(strings.Contains(challenge.CodeVerifier, "="), check.Equals, false)
	c.Assert(strings.Contains(challenge.CodeChallenge, "="), check.Equals, false)
}

func (s *OAuthSuite) TestPKCEChallengeValidation(c *check.C) {
	challenge1, err := GeneratePKCE()
	c.Assert(err, check.IsNil)

	challenge2, err := GeneratePKCE()
	c.Assert(err, check.IsNil)

	// Verify uniqueness
	c.Assert(challenge1.CodeVerifier, check.Not(check.Equals), challenge2.CodeVerifier)
	c.Assert(challenge1.CodeChallenge, check.Not(check.Equals), challenge2.CodeChallenge)

	// Verify cryptographic properties
	c.Assert(len(challenge1.CodeVerifier) >= 43, check.Equals, true) // Min length per RFC 7636
	c.Assert(len(challenge1.CodeChallenge) >= 43, check.Equals, true)
}

// Microsoft Provider Tests
func (s *OAuthSuite) TestNewMicrosoftProvider(c *check.C) {
	cfg := &config.SSOProvider{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TenantID:     "common",
	}

	provider := NewMicrosoftProvider(cfg)
	c.Assert(provider, check.NotNil)
	c.Assert(provider.GetProviderName(), check.Equals, "microsoft")

	oauthConfig := provider.GetConfig()
	c.Assert(oauthConfig.ClientID, check.Equals, "test-client-id")
	c.Assert(oauthConfig.ClientSecret, check.Equals, "test-client-secret")
	c.Assert(len(oauthConfig.Scopes), check.Equals, 4)
	c.Assert(oauthConfig.Scopes[0], check.Equals, "openid")
}

func (s *OAuthSuite) TestMicrosoftProviderAuthURL(c *check.C) {
	cfg := &config.SSOProvider{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TenantID:     "common",
	}

	provider := NewMicrosoftProvider(cfg)
	provider.SetRedirectURL("http://localhost:3333/auth/microsoft/callback")

	state := "test-state-123"
	authURL := provider.GetAuthURL(state)

	c.Assert(authURL, check.Not(check.Equals), "")
	c.Assert(strings.Contains(authURL, "state="+state), check.Equals, true)
	c.Assert(strings.Contains(authURL, "prompt=select_account"), check.Equals, true)
	c.Assert(strings.Contains(authURL, "client_id=test-client-id"), check.Equals, true)
}

func (s *OAuthSuite) TestMicrosoftProviderPKCEAuthURL(c *check.C) {
	cfg := &config.SSOProvider{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TenantID:     "common",
	}

	provider := NewMicrosoftProvider(cfg)
	provider.SetRedirectURL("http://localhost:3333/auth/microsoft/callback")

	pkce, err := GeneratePKCE()
	c.Assert(err, check.IsNil)

	state := "test-state-123"
	authURL := provider.GetAuthURLWithPKCE(state, pkce)

	c.Assert(authURL, check.Not(check.Equals), "")
	c.Assert(strings.Contains(authURL, "state="+state), check.Equals, true)
	c.Assert(strings.Contains(authURL, "code_challenge="+pkce.CodeChallenge), check.Equals, true)
	c.Assert(strings.Contains(authURL, "code_challenge_method=S256"), check.Equals, true)
}

// OAuth Handler Tests
func (s *OAuthSuite) TestNewOAuthHandler(c *check.C) {
	cfg := &config.Config{
		SSO: &config.SSOConfig{
			Enabled: true,
		},
	}

	mockProvider := &mockOAuthProvider{
		providerName: "microsoft",
		authURL:      "https://login.microsoftonline.com/oauth2/v2.0/authorize",
	}

	mockUserOps := &mockUserOperationsProvider{}

	handler := NewOAuthHandler(cfg, mockProvider, mockUserOps)
	c.Assert(handler, check.NotNil)
	c.Assert(handler.config, check.Equals, cfg)
	c.Assert(handler.provider, check.Equals, mockProvider)
	c.Assert(handler.userOps, check.Equals, mockUserOps)
	c.Assert(handler.maxAttempts, check.Equals, 5)
}

func (s *OAuthSuite) TestOAuthHandlerStateGeneration(c *check.C) {
	cfg := &config.Config{
		SSO: &config.SSOConfig{
			Enabled: true,
		},
	}

	mockProvider := &mockOAuthProvider{
		providerName: "microsoft",
		authURL:      "https://login.microsoftonline.com/oauth2/v2.0/authorize",
	}

	mockUserOps := &mockUserOperationsProvider{}
	handler := NewOAuthHandler(cfg, mockProvider, mockUserOps)

	// Test generateSecureState method through internal testing
	state1, err := handler.generateSecureState()
	c.Assert(err, check.IsNil)
	c.Assert(state1, check.Not(check.Equals), "")

	state2, err := handler.generateSecureState()
	c.Assert(err, check.IsNil)
	c.Assert(state2, check.Not(check.Equals), "")

	// Verify uniqueness
	c.Assert(state1, check.Not(check.Equals), state2)

	// Verify length (64 hex characters = 32 bytes)
	c.Assert(len(state1), check.Equals, 64)
	c.Assert(len(state2), check.Equals, 64)
}

// Integration Tests
func (s *OAuthSuite) TestOAuthUserOperationsInterface(c *check.C) {
	mockUserOps := &mockUserOperationsProvider{
		findOrCreateUserFunc: func(provider, oauthID, email string) (int64, string, bool, bool, error) {
			c.Assert(provider, check.Equals, "microsoft")
			c.Assert(oauthID, check.Equals, "test-user-id")
			c.Assert(email, check.Equals, "test@example.com")
			return 42, "test-username", false, true, nil
		},
		updateLastLoginFunc: func(userID int64) error {
			c.Assert(userID, check.Equals, int64(42))
			return nil
		},
		validateAdminPrivilegeFunc: func(userID int64) (bool, error) {
			c.Assert(userID, check.Equals, int64(42))
			return true, nil
		},
		logSecurityEventFunc: func(userID int64, event, details string) error {
			c.Assert(userID, check.Equals, int64(42))
			c.Assert(event, check.Not(check.Equals), "")
			return nil
		},
	}

	// Test FindOrCreateUser
	userID, username, locked, admin, err := mockUserOps.FindOrCreateUser("microsoft", "test-user-id", "test@example.com")
	c.Assert(err, check.IsNil)
	c.Assert(userID, check.Equals, int64(42))
	c.Assert(username, check.Equals, "test-username")
	c.Assert(locked, check.Equals, false)
	c.Assert(admin, check.Equals, true)

	// Test UpdateLastLogin
	err = mockUserOps.UpdateLastLogin(42)
	c.Assert(err, check.IsNil)

	// Test ValidateAdminPrivilege
	isAdmin, err := mockUserOps.ValidateAdminPrivilege(42)
	c.Assert(err, check.IsNil)
	c.Assert(isAdmin, check.Equals, true)

	// Test LogSecurityEvent
	err = mockUserOps.LogSecurityEvent(42, "test_event", "test details")
	c.Assert(err, check.IsNil)
}

// OAuth State Security Tests
func (s *OAuthSuite) TestOAuthStateGeneration(c *check.C) {
	state, err := GenerateOAuthState("microsoft", "/dashboard")
	c.Assert(err, check.IsNil)
	c.Assert(state, check.NotNil)

	c.Assert(state.State, check.Not(check.Equals), "")
	c.Assert(state.Provider, check.Equals, "microsoft")
	c.Assert(state.ReturnURL, check.Equals, "/dashboard")
	c.Assert(state.PKCE, check.NotNil)
	c.Assert(state.PKCE.CodeVerifier, check.Not(check.Equals), "")
	c.Assert(state.PKCE.CodeChallenge, check.Not(check.Equals), "")
}

func (s *OAuthSuite) TestOAuthStateValidation(c *check.C) {
	state, err := GenerateOAuthState("microsoft", "/dashboard")
	c.Assert(err, check.IsNil)

	// Valid state should pass
	err = ValidateState(state.State, state)
	c.Assert(err, check.IsNil)

	// Invalid state should fail
	err = ValidateState("wrong-state", state)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*mismatch.*")

	// Expired state should fail
	expiredState := &OAuthState{
		State:     state.State,
		Provider:  "microsoft",
		Timestamp: time.Now().Add(-10 * time.Minute),
		ReturnURL: "/dashboard",
	}
	err = ValidateState(state.State, expiredState)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*expired.*")
}

// Domain Validation Tests
func (s *OAuthSuite) TestMicrosoftProviderDomainValidation(c *check.C) {
	cfg := &config.SSOProvider{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TenantID:     "common",
	}

	provider := NewMicrosoftProvider(cfg)

	// Test with no domain restrictions (should allow all)
	c.Assert(provider.ValidateDomain("user@example.com", []string{}), check.Equals, true)
	c.Assert(provider.ValidateDomain("user@any-domain.com", []string{}), check.Equals, true)

	// Test with domain restrictions
	allowedDomains := []string{"company.com", "partner.org"}
	c.Assert(provider.ValidateDomain("user@company.com", allowedDomains), check.Equals, true)
	c.Assert(provider.ValidateDomain("user@partner.org", allowedDomains), check.Equals, true)
	c.Assert(provider.ValidateDomain("user@unauthorized.com", allowedDomains), check.Equals, false)

	// Test invalid email format
	c.Assert(provider.ValidateDomain("invalid-email", allowedDomains), check.Equals, false)
	c.Assert(provider.ValidateDomain("", allowedDomains), check.Equals, false)
}

// OAuth User Info Tests
func (s *OAuthSuite) TestOAuthUserInfoStructure(c *check.C) {
	userInfo := &OAuthUserInfo{
		Provider:  "microsoft",
		ID:        "12345",
		Email:     "test@example.com",
		Name:      "Test User",
		FirstName: "Test",
		LastName:  "User",
		AvatarURL: "https://example.com/avatar.jpg",
	}

	c.Assert(userInfo.Provider, check.Equals, "microsoft")
	c.Assert(userInfo.ID, check.Equals, "12345")
	c.Assert(userInfo.Email, check.Equals, "test@example.com")
	c.Assert(userInfo.Name, check.Equals, "Test User")
	c.Assert(userInfo.FirstName, check.Equals, "Test")
	c.Assert(userInfo.LastName, check.Equals, "User")
	c.Assert(userInfo.AvatarURL, check.Equals, "https://example.com/avatar.jpg")
}

// URL Validation Tests
func (s *OAuthSuite) TestRedirectURLValidation(c *check.C) {
	cfg := &config.Config{
		SSO: &config.SSOConfig{
			Enabled: true,
		},
	}

	mockProvider := &mockOAuthProvider{
		providerName: "microsoft",
	}

	mockUserOps := &mockUserOperationsProvider{}
	handler := NewOAuthHandler(cfg, mockProvider, mockUserOps)

	// Valid relative URLs
	c.Assert(handler.isValidRedirectURL("/dashboard"), check.Equals, true)
	c.Assert(handler.isValidRedirectURL("/admin/campaigns"), check.Equals, true)

	// Invalid URLs (security risk)
	c.Assert(handler.isValidRedirectURL("//evil.com"), check.Equals, false)
	c.Assert(handler.isValidRedirectURL("http://evil.com"), check.Equals, false)
	c.Assert(handler.isValidRedirectURL("https://evil.com"), check.Equals, false)
	c.Assert(handler.isValidRedirectURL("javascript:alert(1)"), check.Equals, false)
}

// Rate Limiting Tests
func (s *OAuthSuite) TestOAuthHandlerRateLimiting(c *check.C) {
	cfg := &config.Config{
		SSO: &config.SSOConfig{
			Enabled: true,
		},
	}

	mockProvider := &mockOAuthProvider{
		providerName: "microsoft",
		authURL:      "https://login.microsoftonline.com/oauth2/v2.0/authorize",
	}

	mockUserOps := &mockUserOperationsProvider{}
	handler := NewOAuthHandler(cfg, mockProvider, mockUserOps)

	// Test that rate limiter exists and is configured
	c.Assert(handler.rateLimiter, check.NotNil)

	// Rate limiter should allow initial requests
	allowed := handler.rateLimiter.Allow()
	c.Assert(allowed, check.Equals, true)
}

// Token Response Tests
func (s *OAuthSuite) TestOAuthTokenResponseExpiration(c *check.C) {
	// Test non-expired token
	token := &OAuthTokenResponse{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600, // 1 hour
	}

	c.Assert(token.IsExpired(), check.Equals, false)

	expiresAt := token.ExpiresAt()
	c.Assert(expiresAt.After(time.Now()), check.Equals, true)

	// Test expired token
	expiredToken := &OAuthTokenResponse{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresIn:   0,
	}

	c.Assert(expiredToken.IsExpired(), check.Equals, true)
}

// Edge Cases and Error Handling Tests
func (s *OAuthSuite) TestOAuthHandlerWithNilUserOps(c *check.C) {
	cfg := &config.Config{
		SSO: &config.SSOConfig{
			Enabled: true,
		},
	}

	mockProvider := &mockOAuthProvider{
		providerName: "microsoft",
	}

	// Test handler creation with nil user operations
	handler := NewOAuthHandler(cfg, mockProvider, nil)
	c.Assert(handler, check.NotNil)
	c.Assert(handler.userOps, check.IsNil)
}

func (s *OAuthSuite) TestPKCEEmptyStringValues(c *check.C) {
	challenge, err := GeneratePKCE()
	c.Assert(err, check.IsNil)

	// Ensure no empty strings
	c.Assert(challenge.CodeVerifier != "", check.Equals, true)
	c.Assert(challenge.CodeChallenge != "", check.Equals, true)
	c.Assert(challenge.Method != "", check.Equals, true)

	// Ensure minimum security requirements
	c.Assert(len(challenge.CodeVerifier) >= 43, check.Equals, true)
	c.Assert(len(challenge.CodeChallenge) >= 43, check.Equals, true)
}

func (s *OAuthSuite) TestMultiplePKCEGenerationsUnique(c *check.C) {
	challenges := make([]*PKCEChallenge, 10)

	// Generate multiple challenges
	for i := 0; i < 10; i++ {
		challenge, err := GeneratePKCE()
		c.Assert(err, check.IsNil)
		challenges[i] = challenge
	}

	// Verify all are unique
	for i := 0; i < 10; i++ {
		for j := i + 1; j < 10; j++ {
			c.Assert(challenges[i].CodeVerifier != challenges[j].CodeVerifier, check.Equals, true)
			c.Assert(challenges[i].CodeChallenge != challenges[j].CodeChallenge, check.Equals, true)
		}
	}
}
