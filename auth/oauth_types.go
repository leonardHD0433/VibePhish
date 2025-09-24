package auth

import "time"

// OAuthUserInfo represents user information from OAuth provider
type OAuthUserInfo struct {
	Provider    string `json:"provider"`
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// OAuthState represents state for OAuth flow security
type OAuthState struct {
	State       string
	Provider    string
	Timestamp   time.Time
	ReturnURL   string
	PKCE        *PKCEChallenge
}

// PKCEChallenge represents PKCE challenge for OAuth security
type PKCEChallenge struct {
	CodeVerifier  string
	CodeChallenge string
	Method        string
}

// OAuthTokenResponse represents the token response from OAuth provider
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// IsExpired checks if the token response indicates expiration
func (t *OAuthTokenResponse) IsExpired() bool {
	return t.ExpiresIn <= 0
}

// ExpiresAt calculates when the token expires
func (t *OAuthTokenResponse) ExpiresAt() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}