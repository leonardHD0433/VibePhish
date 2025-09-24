package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gophish/gophish/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// OAuthProvider interface for different OAuth providers
type OAuthProvider interface {
	GetAuthURL(state string, opts ...oauth2.AuthCodeOption) string
	GetAuthURLWithPKCE(state string, pkce *PKCEChallenge) string
	ExchangeCode(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	ExchangeCodeWithPKCE(ctx context.Context, code string, pkce *PKCEChallenge) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
	GetConfig() *oauth2.Config
	GetProviderName() string
	ValidateDomain(email string, allowedDomains []string) bool
}

// MicrosoftProvider implements Microsoft OAuth
type MicrosoftProvider struct {
	config   *oauth2.Config
	tenantID string
}

// NewMicrosoftProvider creates a new Microsoft OAuth provider
func NewMicrosoftProvider(cfg *config.SSOProvider) *MicrosoftProvider {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "common"
	}

	endpoint := microsoft.AzureADEndpoint(tenantID)

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       []string{"openid", "profile", "email", "User.Read"},
		Endpoint:     endpoint,
		// RedirectURL will be set dynamically
	}

	return &MicrosoftProvider{
		config:   oauthConfig,
		tenantID: tenantID,
	}
}

func (p *MicrosoftProvider) GetConfig() *oauth2.Config {
	return p.config
}

func (p *MicrosoftProvider) GetProviderName() string {
	return "microsoft"
}

func (p *MicrosoftProvider) SetRedirectURL(redirectURL string) {
	p.config.RedirectURL = redirectURL
}

func (p *MicrosoftProvider) GetAuthURL(state string, opts ...oauth2.AuthCodeOption) string {
	return p.config.AuthCodeURL(state, opts...)
}

func (p *MicrosoftProvider) GetAuthURLWithPKCE(state string, pkce *PKCEChallenge) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", pkce.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
	}
	return p.config.AuthCodeURL(state, opts...)
}

func (p *MicrosoftProvider) ExchangeCode(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code, opts...)
}

func (p *MicrosoftProvider) ExchangeCodeWithPKCE(ctx context.Context, code string, pkce *PKCEChallenge) (*oauth2.Token, error) {
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", pkce.CodeVerifier),
	}
	return p.config.Exchange(ctx, code, opts...)
}

func (p *MicrosoftProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)

	// Use Microsoft Graph API to get user info
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Microsoft API error: %s", string(body))
	}

	var msUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		GivenName         string `json:"givenName"`
		Surname           string `json:"surname"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		PreferredUsername string `json:"preferredUsername"`
		JobTitle          string `json:"jobTitle,omitempty"`
		Department        string `json:"department,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&msUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Email priority: Mail (primary) > UserPrincipalName > PreferredUsername
	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}
	if email == "" {
		email = msUser.PreferredUsername
	}

	return &OAuthUserInfo{
		Provider:  "microsoft",
		ID:        msUser.ID,
		Email:     email,
		Name:      msUser.DisplayName,
		FirstName: msUser.GivenName,
		LastName:  msUser.Surname,
	}, nil
}

// ValidateDomain checks if the email domain is allowed
func (p *MicrosoftProvider) ValidateDomain(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true // No restrictions
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	domain := strings.ToLower(parts[1])
	for _, allowed := range allowedDomains {
		if strings.ToLower(allowed) == domain {
			return true
		}
	}
	return false
}

// CreateProvider creates an OAuth provider based on the provider name
func CreateProvider(providerName string, cfg *config.SSOProvider) (OAuthProvider, error) {
	switch providerName {
	case "microsoft":
		return NewMicrosoftProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", providerName)
	}
}