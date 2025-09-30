Secure SSO Implementation Plan for FYPhish

  Based on my analysis of your current authentication architecture, here's a
  comprehensive plan for implementing secure Microsoft SSO (with Google support):

✻ Thinking…

  Now I'll provide a comprehensive technical plan covering all aspects of the SSO 
  implementation.

● 1. Current Architecture Assessment

  Existing Auth Flow:
  - Traditional username/password with bcrypt hashing
  - Session-based authentication using Gorilla sessions
  - User model: models/user.go:16-26 with ID, Username, Hash, ApiKey, Role
  - Session store: 5-day cookie expiry, HTTP-only, secure flags
  - CSRF protection and rate limiting implemented

  2. OAuth 2.0/OIDC Architecture Design

  Required Components:

  // New OAuth provider struct
  type OAuthProvider struct {
      Name         string
      ClientID     string
      ClientSecret string
      RedirectURL  string
      AuthURL      string
      TokenURL     string
      UserInfoURL  string
      Scopes       []string
  }

  // Microsoft configuration
  microsoftProvider := OAuthProvider{
      Name: "microsoft",
      AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
      TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
      UserInfoURL: "https://graph.microsoft.com/v1.0/me",
      Scopes: []string{"openid", "profile", "email", "User.Read"},
  }

  Security Flow:
  1. Authorization: /auth/microsoft → Azure AD with PKCE
  2. Callback: /auth/microsoft/callback with state validation
  3. Token Exchange: Authorization code → Access/ID tokens
  4. User Info: Graph API call for user details
  5. Session Creation: Standard Gophish session with user ID

  3. Database Schema Extensions

  -- Add to existing User table
  ALTER TABLE users ADD COLUMN oauth_provider VARCHAR(50);
  ALTER TABLE users ADD COLUMN oauth_id VARCHAR(255);
  ALTER TABLE users ADD COLUMN email VARCHAR(255);
  ALTER TABLE users ADD COLUMN display_name VARCHAR(255);

  -- New table for OAuth tokens (encrypted)
  CREATE TABLE oauth_tokens (
      id BIGINT PRIMARY KEY,
      user_id BIGINT REFERENCES users(id),
      provider VARCHAR(50) NOT NULL,
      access_token_hash VARCHAR(255), -- encrypted
      refresh_token_hash VARCHAR(255), -- encrypted  
      expires_at TIMESTAMP,
      created_at TIMESTAMP,
      updated_at TIMESTAMP
  );

  -- Unique constraints
  CREATE UNIQUE INDEX idx_oauth_user ON users(oauth_provider, oauth_id);

✻ Thinking…

  Continuing with the user provisioning and role mapping strategy.

● 4. Secure Token Management

  Encryption Strategy:
  // Use existing auth.GenerateSecureKey for token encryption
  func encryptToken(token string) (string, error) {
      key := []byte(config.TokenEncryptionKey) // 32-byte key
      return util.Encrypt(token, key)
  }

  // Store tokens with limited lifetime
  type OAuthToken struct {
      UserID       int64
      Provider     string
      AccessToken  string `gorm:"-"` // Never serialize
      RefreshToken string `gorm:"-"` // Never serialize
      ExpiresAt    time.Time
  }

  Token Security:
  - AES-256-GCM encryption for stored tokens
  - 2-hour access token lifetime (configurable)
  - Automatic refresh before expiration
  - Secure deletion on logout/expiry

  5. User Provisioning & Role Mapping

  Provisioning Strategy:

  type UserProvisioner struct {
      DefaultRole    string // "user" for students, configurable
      AllowedDomains []string // ["university.edu", "student.university.edu"]  
      AdminDomains   []string // ["admin.university.edu"]
  }

  func (p *UserProvisioner) ProvisionUser(oauthUser OAuthUserInfo) (*models.User, error)
  {
      // 1. Domain validation
      if !p.isAllowedDomain(oauthUser.Email) {
          return nil, ErrUnauthorizedDomain
      }

      // 2. Check existing user by OAuth ID
      existing, err := models.GetUserByOAuthID(oauthUser.Provider, oauthUser.ID)
      if err == nil {
          return existing, nil // User exists, return
      }

      // 3. Auto-provision new user
      role := p.determineRole(oauthUser.Email)
      user := &models.User{
          Username:     oauthUser.Email,
          Email:        oauthUser.Email,
          DisplayName:  oauthUser.Name,
          OAuthProvider: oauthUser.Provider,
          OAuthID:      oauthUser.ID,
          ApiKey:       auth.GenerateSecureKey(auth.APIKeyLength),
          Role:         role,
          Hash:         "", // No password for OAuth users
      }

      return models.CreateUser(user)
  }

  University-Specific Configuration:
  {
    "oauth": {
      "microsoft": {
        "client_id": "your-app-id",
        "client_secret": "your-secret",
        "tenant_id": "your-university-tenant-id",
        "allowed_domains": ["university.edu", "student.university.edu"],
        "admin_domains": ["admin.university.edu"],
        "default_role": "user"
      }
    }
  }

  6. Critical Security Considerations

✻ Thinking…

  Now continuing with the security considerations and mitigation strategies.

● A. CSRF Protection:
  // Extend existing CSRFExemptPrefixes in middleware/middleware.go:15
  var CSRFExemptPrefixes = []string{
      "/api",
      "/auth/microsoft/callback", // OAuth callbacks are CSRF-exempt by design
      "/auth/google/callback",
  }

  B. State Parameter Validation:
  func generateOAuthState(session *sessions.Session) string {
      state := auth.GenerateSecureKey(32)
      session.Values["oauth_state"] = state
      session.Values["oauth_timestamp"] = time.Now().Unix()
      return state
  }

  func validateOAuthState(r *http.Request, expectedState string) error {
      session := ctx.Get(r, "session").(*sessions.Session)
      storedState := session.Values["oauth_state"]
      timestamp := session.Values["oauth_timestamp"]

      // Validate state matches and is recent (5 min max)
      if storedState != expectedState ||
         time.Now().Unix()-timestamp.(int64) > 300 {
          return ErrInvalidOAuthState
      }
      return nil
  }

  C. PKCE Implementation:
  // Use PKCE for added security (required for Azure AD)
  type PKCEChallenge struct {
      CodeVerifier  string
      CodeChallenge string
      Method        string // "S256"
  }

  func generatePKCE() PKCEChallenge {
      verifier := auth.GenerateSecureKey(32)
      challenge := base64.URLEncoding.WithoutPadding().EncodeToString(
          sha256.Sum256([]byte(verifier))
      )
      return PKCEChallenge{
          CodeVerifier:  verifier,
          CodeChallenge: challenge,
          Method:        "S256",
      }
  }

  D. Token Validation:
  func validateIDToken(idToken string, provider OAuthProvider) (*jwt.Token, error) {
      // Validate JWT signature, issuer, audience, expiry
      token, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
          // Fetch and cache Microsoft's public keys
          return getProviderPublicKey(provider.Name, token.Header["kid"].(string))
      })

      if err != nil || !token.Valid {
          return nil, ErrInvalidIDToken
      }
      return token, nil
  }

  7. Implementation Phases

✻ Thinking…

  Final section - implementation phases and testing approach.

● Phase 1: Foundation (Week 1-2)
  1. Database Schema: Add OAuth columns to User table, create OAuthToken table
  2. Configuration: Extend config.json with OAuth providers
  3. Dependencies: Add golang.org/x/oauth2 and github.com/golang-jwt/jwt/v5
  4. Base Structures: Create OAuth provider, user info, and token models

  Phase 2: Core OAuth Flow (Week 3-4)
  1. OAuth Handlers: Implement /auth/microsoft and /auth/microsoft/callback
  2. Token Management: Secure encryption/decryption, refresh logic
  3. User Provisioning: Auto-creation with domain validation
  4. Session Integration: Extend existing session handling

  Phase 3: Security Hardening (Week 5)
  1. State/PKCE Validation: Implement all security checks
  2. JWT Validation: ID token verification with Microsoft keys
  3. Rate Limiting: Apply to OAuth endpoints
  4. Error Handling: Secure error messages, logging

  Phase 4: Testing & Integration (Week 6)
  1. Unit Tests: All OAuth functions and security validations
  2. Integration Tests: Full login flows, edge cases
  3. Security Testing: CSRF, state tampering, token replay
  4. Documentation: Configuration guide, deployment steps

  8. Required Dependencies

  require (
      golang.org/x/oauth2 v0.15.0
      github.com/golang-jwt/jwt/v5 v5.0.0
      github.com/microsoft/kiota-abstractions-go v1.3.0 // Optional: for Graph SDK
  )

  9. Security Checklist

  ✅ CSRF Protection: OAuth callbacks exempted appropriately✅ State Validation:
  Cryptographically secure, time-limited✅ PKCE: Prevents authorization code
  interception✅ Token Encryption: AES-256-GCM for stored tokens✅ Domain Validation:
  Restrict to university domains✅ JWT Verification: Signature, issuer, audience
  validation✅ Session Security: Maintain existing secure cookie settings✅ Rate 
  Limiting: Apply to OAuth endpoints✅ Audit Logging: Track OAuth events and failures

  10. Recommended File Structure

  auth/
  ├── oauth.go          # Core OAuth logic
  ├── oauth_test.go     # OAuth tests  
  ├── providers.go      # Provider configurations
  └── tokens.go         # Token encryption/management

  controllers/
  ├── oauth.go          # OAuth route handlers
  └── oauth_test.go     # Integration tests

  models/
  ├── oauth_token.go    # OAuthToken model
  └── migrations/       # Schema migration files

  This implementation maintains backward compatibility while adding enterprise-grade SSO
  security suitable for university deployment. The phased approach allows for iterative
  testing and validation of each security component.

✻ Thinking…

  I've provided a comprehensive security implementation plan for SSO integration into 
  FYPhish. The plan covers all the critical aspects including:

  1. ✅ Current architecture analysis
  2. ✅ OAuth 2.0/OIDC flow design 
  3. ✅ Secure token handling
  4. ✅ User provisioning strategy
  5. ✅ Security considerations and mitigations
  6. ✅ Implementation phases

  Key security highlights:
  - PKCE implementation for authorization code flow protection
  - State parameter validation with time limits
  - AES-256-GCM encryption for stored tokens
  - JWT verification with proper signature validation
  - Domain-based access control for university deployment
  - Rate limiting and CSRF protections
  - Comprehensive testing plan

  This is a thorough, production-ready security implementation plan suitable for an 
  academic/research environment.

● The implementation prioritizes security-first principles with university-specific
  configurations, making it ideal for your FYP while maintaining the defensive security
  focus appropriate for academic use.