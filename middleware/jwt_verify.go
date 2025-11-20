package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
)

// JWTClaims represents the JWT payload claims
type JWTClaims struct {
	Sub string `json:"sub"` // Subject (e.g., "n8n" or "fyphish")
	Iat int64  `json:"iat"` // Issued at timestamp
	Exp int64  `json:"exp"` // Expiry timestamp
}

// VerifyN8NJWT is middleware that verifies JWT tokens from n8n callbacks
func VerifyN8NJWT(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Warn("n8n callback request missing Authorization header")
			http.Error(w, `{"success": false, "message": "Missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Check for Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Warn("n8n callback request has invalid Authorization header format")
			http.Error(w, `{"success": false, "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Verify JWT token
		err := verifyJWT(token)
		if err != nil {
			log.Warnf("n8n callback JWT verification failed: %v", err)
			http.Error(w, fmt.Sprintf(`{"success": false, "message": "JWT verification failed: %s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed to handler
		handler.ServeHTTP(w, r)
	})
}

// verifyJWT verifies an HS256 JWT token using the JWT_SECRET environment variable
func verifyJWT(token string) error {
	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable not set")
	}

	// Split token into parts: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	headerB64 := parts[0]
	payloadB64 := parts[1]
	signatureB64 := parts[2]

	// Verify signature
	signingInput := headerB64 + "." + payloadB64
	expectedSignature := hmacSHA256(signingInput, jwtSecret)
	expectedSignatureB64 := base64URLEncode(expectedSignature)

	if signatureB64 != expectedSignatureB64 {
		return fmt.Errorf("invalid JWT signature")
	}

	// Decode and parse payload
	payloadJSON, err := base64URLDecode(payloadB64)
	if err != nil {
		return fmt.Errorf("failed to decode JWT payload: %v", err)
	}

	var claims JWTClaims
	err = json.Unmarshal(payloadJSON, &claims)
	if err != nil {
		return fmt.Errorf("failed to parse JWT claims: %v", err)
	}

	// Verify expiry
	now := time.Now().Unix()
	if claims.Exp > 0 && now > claims.Exp {
		return fmt.Errorf("JWT token expired at %d (now: %d)", claims.Exp, now)
	}

	// Verify issued-at is not in the future (clock skew tolerance: 5 minutes)
	if claims.Iat > 0 && claims.Iat > now+300 {
		return fmt.Errorf("JWT iat is in the future: %d (now: %d)", claims.Iat, now)
	}

	// Optional: Verify subject (sub) claim
	if claims.Sub != "" && claims.Sub != "n8n" && claims.Sub != "fyphish" {
		log.Warnf("JWT token has unexpected subject: %s", claims.Sub)
	}

	log.Debugf("JWT token verified successfully (sub: %s, iat: %d, exp: %d)", claims.Sub, claims.Iat, claims.Exp)
	return nil
}

// base64URLDecode decodes a base64url-encoded string
func base64URLDecode(encoded string) ([]byte, error) {
	// Add padding if needed
	padding := len(encoded) % 4
	if padding > 0 {
		encoded += strings.Repeat("=", 4-padding)
	}

	// Replace base64url characters with standard base64
	encoded = strings.ReplaceAll(encoded, "-", "+")
	encoded = strings.ReplaceAll(encoded, "_", "/")

	return base64.StdEncoding.DecodeString(encoded)
}

// base64URLEncode encodes bytes to base64url format
func base64URLEncode(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return encoded
}

// hmacSHA256 generates HMAC-SHA256 signature
func hmacSHA256(message, secret string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return h.Sum(nil)
}

// RequireN8NJWT wraps the VerifyN8NJWT middleware for use with Use() pattern
func RequireN8NJWT(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		VerifyN8NJWT(http.HandlerFunc(handler)).ServeHTTP(w, r)
	}
}
