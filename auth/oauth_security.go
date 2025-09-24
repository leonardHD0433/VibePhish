package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// GenerateOAuthState creates a secure state parameter for OAuth flow
func GenerateOAuthState(provider, returnURL string) (*OAuthState, error) {
	state := GenerateSecureKey(32)

	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	return &OAuthState{
		State:     state,
		Provider:  provider,
		Timestamp: time.Now(),
		ReturnURL: returnURL,
		PKCE:      pkce,
	}, nil
}

// GeneratePKCE creates a PKCE challenge for OAuth security
func GeneratePKCE() (*PKCEChallenge, error) {
	// Generate 32 random bytes for code verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Base64 URL encode without padding (manual padding removal)
	codeVerifier := base64.URLEncoding.EncodeToString(verifierBytes)
	codeVerifier = strings.TrimRight(codeVerifier, "=")

	// Create SHA256 hash of verifier
	hash := sha256.Sum256([]byte(codeVerifier))

	// Base64 URL encode the hash without padding
	codeChallenge := base64.URLEncoding.EncodeToString(hash[:])
	codeChallenge = strings.TrimRight(codeChallenge, "=")

	return &PKCEChallenge{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		Method:        "S256",
	}, nil
}

// ValidateStateAge checks if OAuth state is still valid (not too old)
func (s *OAuthState) IsValid(maxAgeMinutes int) bool {
	age := time.Since(s.Timestamp)
	maxAge := time.Duration(maxAgeMinutes) * time.Minute
	return age <= maxAge
}

// ValidateState checks if the provided state matches expected state
func ValidateState(provided string, expected *OAuthState) error {
	if provided != expected.State {
		return fmt.Errorf("state parameter mismatch")
	}

	if !expected.IsValid(5) { // 5 minute max age
		return fmt.Errorf("state parameter expired")
	}

	return nil
}