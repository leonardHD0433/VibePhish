package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gophish/gomail"
	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// SendTestEmail sends a test email using the template name
// and Target given.
func (as *Server) SendTestEmail(w http.ResponseWriter, r *http.Request) {
	s := &models.EmailRequest{
		ErrorChan: make(chan error),
		UserId:    ctx.Get(r, "user_id").(int64),
	}
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusBadRequest)
		return
	}
	err := json.NewDecoder(r.Body).Decode(s)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error decoding JSON Request"}, http.StatusBadRequest)
		return
	}

	storeRequest := false

	// If a Template is not specified use a default
	if s.Template.Name == "" {
		//default message body
		text := "It works!\n\nThis is an email letting you know that your gophish\nconfiguration was successful.\n" +
			"Here are the details:\n\nWho you sent from: {{.From}}\n\nWho you sent to: \n" +
			"{{if .FirstName}} First Name: {{.FirstName}}\n{{end}}" +
			"{{if .LastName}} Last Name: {{.LastName}}\n{{end}}" +
			"{{if .Position}} Position: {{.Position}}\n{{end}}" +
			"\nNow go send some phish!"
		t := models.Template{
			Subject: "Default Email from Gophish",
			Text:    text,
		}
		s.Template = t
	} else {
		// Get the Template requested by name
		s.Template, err = models.GetTemplateByName(s.Template.Name, s.UserId)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"template": s.Template.Name,
			}).Error("Template does not exist")
			JSONResponse(w, models.Response{Success: false, Message: models.ErrTemplateNotFound.Error()}, http.StatusBadRequest)
			return
		} else if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		s.TemplateId = s.Template.Id
		// We'll only save the test request to the database if there is a
		// user-specified template to use.
		storeRequest = true
	}

	if s.Page.Name != "" {
		s.Page, err = models.GetPageByName(s.Page.Name, s.UserId)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"page": s.Page.Name,
			}).Error("Page does not exist")
			JSONResponse(w, models.Response{Success: false, Message: models.ErrPageNotFound.Error()}, http.StatusBadRequest)
			return
		} else if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		s.PageId = s.Page.Id
	}

	// Validate that email type is specified
	if s.EmailType == "" {
		log.Error("Email type not specified")
		JSONResponse(w, models.Response{Success: false, Message: "Email type is required"}, http.StatusBadRequest)
		return
	}

	// Verify the email type exists in the database
	_, err = models.GetEmailTypeByValue(s.EmailType)
	if err != nil {
		log.WithFields(logrus.Fields{
			"email_type": s.EmailType,
		}).Error("Email type does not exist")
		JSONResponse(w, models.Response{Success: false, Message: "Invalid email type"}, http.StatusBadRequest)
		return
	}

	// Log the request details for debugging
	log.WithFields(logrus.Fields{
		"email":      s.Email,
		"first_name": s.FirstName,
		"last_name":  s.LastName,
		"email_type": s.EmailType,
	}).Info("Email request details before validation")

	// Validate the given request
	if err = s.Validate(); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	// Store the request if this wasn't the default template
	if storeRequest {
		err = models.PostEmailRequest(s)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	// Generate the email message using the template
	msg := gomail.NewMessage()
	// Set a placeholder From address for gomail to generate the message
	// n8n will use the actual From address based on the email account
	msg.SetHeader("From", "test@fyphish.local")
	log.WithFields(logrus.Fields{
		"from": "test@fyphish.local",
	}).Info("Set From header before Generate()")

	err = s.Generate(msg)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Error generating email: %v", err)}, http.StatusInternalServerError)
		return
	}

	log.Info("Generate() completed successfully")

	// Extract subject and body from the generated message
	buf := &bytes.Buffer{}
	log.Info("About to call msg.WriteTo()")
	_, err = msg.WriteTo(buf)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Error writing message: %v", err)}, http.StatusInternalServerError)
		return
	}

	subject, htmlBody, err := parseEmailMessage(buf.String())
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Error parsing message: %v", err)}, http.StatusInternalServerError)
		return
	}

	// Send via n8n webhook
	err = sendTestEmailToN8N(s.EmailType, s.Email, subject, htmlBody)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Error sending test email: %v", err)}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "Test email sent successfully via n8n"}, http.StatusOK)
}

// sendTestEmailToN8N sends a test email via n8n webhook
func sendTestEmailToN8N(emailType, recipient, subject, htmlBody string) error {
	// Get n8n webhook URL from environment
	webhookURL := os.Getenv("N8N_SEND_EMAIL")
	if webhookURL == "" {
		return fmt.Errorf("N8N_SEND_EMAIL environment variable not set")
	}

	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable not set")
	}

	// Generate JWT token
	token, err := generateJWT(jwtSecret)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %v", err)
	}

	// Create payload
	payload := map[string]interface{}{
		"email_type": emailType,
		"recipients": []string{recipient}, // Single recipient for test email
		"subject":    subject,
		"message":    htmlBody,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	log.WithFields(logrus.Fields{
		"email_type": emailType,
		"recipient":  recipient,
		"subject":    subject,
	}).Info("Sending test email via n8n webhook")

	// Create HTTP request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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

	log.WithFields(logrus.Fields{
		"status": resp.StatusCode,
	}).Info("Test email sent successfully via n8n")

	return nil
}

// generateJWT generates an HS256 JWT token for n8n webhook authentication
func generateJWT(secret string) (string, error) {
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
	signature := hmacSHA256(signingInput, secret)
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
		return "", "", fmt.Errorf("no subject found in message")
	}

	return subject, body, nil
}
