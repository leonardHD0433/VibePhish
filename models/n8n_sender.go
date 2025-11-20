package models

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
)

// N8NSender implements the mailer.Sender interface for sending emails via n8n webhook
type N8NSender struct {
	webhookURL string
	jwtSecret  string
	emailType  string
	campaign   *Campaign
	client     *http.Client
}

// N8NWebhookPayload represents the payload sent to n8n webhook
type N8NWebhookPayload struct {
	EmailType       string                `json:"email_type"`
	CampaignId      int64                 `json:"campaign_id"`
	CampaignURL     string                `json:"campaign_url"`     // Base URL for constructing tracking links
	LaunchDate      time.Time             `json:"launch_date"`
	SendByDate      time.Time             `json:"send_by_date"`
	TotalRecipients int                   `json:"total_recipients"`
	Recipients      []RecipientWithTiming `json:"recipients"` // Enhanced with tracking info
	Subject         string                `json:"subject"`
	Message         string                `json:"message"` // Raw template with {{.FirstName}}, {{.Email}}, {{.URL}} placeholders
}

// RecipientWithTiming contains recipient email, result ID, calculated send time, and personalization data
type RecipientWithTiming struct {
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name"`   // For {{.FirstName}} template placeholder
	LastName    string    `json:"last_name"`    // For {{.LastName}} template placeholder
	Position    string    `json:"position"`     // For {{.Position}} template placeholder
	RId         string    `json:"rid"`          // Result ID for tracking in Gophish
	SendAt      time.Time `json:"send_at"`      // Pre-calculated send time
	PhishingURL string    `json:"phishing_url"` // Phishing landing page URL for {{.URL}} placeholder (click tracking)
	TrackingURL string    `json:"tracking_url"` // Tracking pixel URL for {{.Tracker}} placeholder (open tracking)
}

// N8NDialer implements the mailer.Dialer interface for n8n webhook
type N8NDialer struct {
	webhookURL string
	jwtSecret  string
	emailType  string
	campaign   *Campaign
}

// Dial creates a new N8NSender
func (d *N8NDialer) Dial() (mailer.Sender, error) {
	if d.webhookURL == "" {
		return nil, errors.New("n8n webhook URL not configured")
	}
	if d.jwtSecret == "" {
		return nil, errors.New("JWT secret not configured")
	}
	if d.emailType == "" {
		return nil, errors.New("email type not specified in Email Profile")
	}

	// Create custom transport with DNS and connection timeouts
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second, // DNS resolution + connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &N8NSender{
		webhookURL: d.webhookURL,
		jwtSecret:  d.jwtSecret,
		emailType:  d.emailType,
		campaign:   d.campaign,
		client: &http.Client{
			Timeout:   5 * time.Second, // Overall timeout (DNS + connection + response)
			Transport: transport,
		},
	}, nil
}

// Send sends an email via n8n webhook to multiple recipients in a single call
func (s *N8NSender) Send(from string, to []string, msg io.WriterTo) error {
	if len(to) == 0 {
		return errors.New("no recipients specified")
	}

	if s.campaign == nil {
		return errors.New("campaign context not available")
	}

	// Parse the message to extract subject and body
	buf := &bytes.Buffer{}
	_, err := msg.WriteTo(buf)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	// Parse the raw email message
	subject, htmlBody, err := parseEmailMessage(buf.String())
	if err != nil {
		return fmt.Errorf("failed to parse message: %v", err)
	}

	// Build recipients with tracking information and calculated send times
	recipientsWithTiming := make([]RecipientWithTiming, 0, len(to))
	totalRecipients := len(to)

	for idx, email := range to {
		// Look up Result from campaign's in-memory results (CRITICAL: don't query database during transaction!)
		var result *Result
		for i := range s.campaign.Results {
			if s.campaign.Results[i].Email == email {
				result = &s.campaign.Results[i]
				break
			}
		}

		if result == nil {
			log.Warnf("Failed to find result for %s in campaign results, skipping", email)
			continue
		}

		// Calculate send time using campaign's timing logic
		sendAt := s.campaign.generateSendDate(idx, totalRecipients)

		// Build personalized URLs using public base URL
		// GetPublicBaseURL prioritizes: 1) PUBLIC_BASE_URL env var, 2) Campaign URL (if not localhost)
		phishingURL := GetPublicTrackingURL(nil, s.campaign.URL, result.RId)        // Landing page URL (click tracking)
		trackingPixelURL := GetPublicTrackingPixelURL(nil, s.campaign.URL, result.RId) // /track endpoint (open tracking)

		recipientsWithTiming = append(recipientsWithTiming, RecipientWithTiming{
			Email:       email,
			FirstName:   result.FirstName,
			LastName:    result.LastName,
			Position:    result.Position,
			RId:         result.RId,
			SendAt:      sendAt,
			PhishingURL: phishingURL,
			TrackingURL: trackingPixelURL,
		})
	}

	if len(recipientsWithTiming) == 0 {
		return errors.New("no valid recipients found with results")
	}

	// Build enhanced payload with campaign context
	// Use public-facing URL (Cloudflare Tunnel, App Platform, etc.)
	publicBaseURL := GetPublicBaseURL(nil, s.campaign.URL)

	payload := N8NWebhookPayload{
		EmailType:       s.emailType,
		CampaignId:      s.campaign.Id,
		CampaignURL:     publicBaseURL,
		LaunchDate:      s.campaign.LaunchDate,
		SendByDate:      s.campaign.SendByDate,
		TotalRecipients: len(recipientsWithTiming),
		Recipients:      recipientsWithTiming,
		Subject:         subject,
		Message:         htmlBody,
	}

	err = s.sendToN8N(payload)
	if err != nil {
		log.Errorf("Failed to send email via n8n to %d recipients: %v", len(recipientsWithTiming), err)
		return err
	}

	log.Infof("Successfully sent email batch via n8n to %d recipients (campaign: %d, type: %s)",
		len(recipientsWithTiming), s.campaign.Id, s.emailType)
	return nil
}

// sendToN8N sends the payload to n8n webhook with JWT authentication
func (s *N8NSender) sendToN8N(payload N8NWebhookPayload) error {
	// Generate JWT token
	token, err := s.generateJWT()
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %v", err)
	}

	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	log.Debugf("Sending to n8n webhook: %s", string(payloadBytes))

	// Create context with absolute 3-second deadline
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request (will be cancelled after 3 seconds no matter what)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("n8n webhook returned error (status %d): %s", resp.StatusCode, string(body))
	}

	log.Debugf("n8n webhook response: %s", string(body))
	return nil
}

// generateJWT generates an HS256 JWT token for n8n webhook authentication
func (s *N8NSender) generateJWT() (string, error) {
	// Header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerB64 := base64URLEncode(headerJSON)

	// Payload
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"iat": now,
		"exp": now + 300, // 5 minutes expiry
		"sub": "fyphish",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadB64 := base64URLEncode(payloadJSON)

	// Signature
	signingInput := headerB64 + "." + payloadB64
	signature := hmacSHA256(signingInput, s.jwtSecret)
	signatureB64 := base64URLEncode(signature)

	// Combine
	token := signingInput + "." + signatureB64
	return token, nil
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

// parseEmailMessage extracts subject and HTML body from raw email message
func parseEmailMessage(rawMessage string) (string, string, error) {
	lines := strings.Split(rawMessage, "\r\n")

	var subject string
	var htmlBody strings.Builder
	inBody := false
	inHTML := false

	for i, line := range lines {
		// Extract subject
		if strings.HasPrefix(line, "Subject: ") {
			subject = strings.TrimPrefix(line, "Subject: ")
			continue
		}

		// Detect body start (empty line after headers)
		if !inBody && line == "" {
			inBody = true
			continue
		}

		// Extract HTML body content
		if inBody {
			// Look for HTML content boundaries
			if strings.Contains(line, "Content-Type: text/html") {
				inHTML = true
				// Skip past the Content-Type and empty line
				for j := i + 1; j < len(lines); j++ {
					if lines[j] == "" {
						i = j
						break
					}
				}
				continue
			}

			// Stop at next MIME boundary or end of message
			if strings.HasPrefix(line, "--") && !strings.HasPrefix(line, "<!--") {
				break
			}

			if inHTML {
				htmlBody.WriteString(line)
				htmlBody.WriteString("\n")
			}
		}
	}

	// If no HTML body found, return the entire body section
	body := strings.TrimSpace(htmlBody.String())
	if body == "" {
		// Try to extract any content after headers
		bodyStart := strings.Index(rawMessage, "\r\n\r\n")
		if bodyStart != -1 {
			body = strings.TrimSpace(rawMessage[bodyStart+4:])
		}
	}

	if subject == "" {
		return "", "", errors.New("no subject found in message")
	}

	return subject, body, nil
}

// Close is a no-op for n8n sender (satisfies mailer.Sender interface)
func (s *N8NSender) Close() error {
	return nil
}

// Reset is a no-op for n8n sender (satisfies mailer.Sender interface)
// n8n webhook connections don't need reset since each request is independent
func (s *N8NSender) Reset() error {
	return nil
}

// GetN8NDialer creates a new N8NDialer for the given Email Account with campaign context
func (ea *EmailAccount) GetN8NDialer(campaign *Campaign) (mailer.Dialer, error) {
	// Get n8n configuration from environment
	webhookURL := os.Getenv("N8N_SEND_EMAIL")
	if webhookURL == "" {
		return nil, errors.New("N8N_SEND_EMAIL environment variable not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET environment variable not set")
	}

	if ea.EmailType == "" {
		return nil, errors.New("email type not specified in Email Account")
	}

	return &N8NDialer{
		webhookURL: webhookURL,
		jwtSecret:  jwtSecret,
		emailType:  ea.EmailType,
		campaign:   campaign,
	}, nil
}
