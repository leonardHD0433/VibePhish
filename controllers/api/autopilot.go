package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// AutopilotAgent1Request represents the request for email type matching
type AutopilotAgent1Request struct {
	UserPrompt string `json:"user_prompt"`
}

// AutopilotAgent1Response represents the response from Agent 1
type AutopilotAgent1Response struct {
	Success        bool    `json:"success"`
	MatchedType    string  `json:"matched_type"`
	EmailTypeName  string  `json:"email_type_name"`
	Confidence     int     `json:"confidence"`
	Reasoning      string  `json:"reasoning"`
	Error          string  `json:"error,omitempty"`
}

// AutopilotAgent2Request represents the request for target filtering
type AutopilotAgent2Request struct {
	UserPrompt string `json:"user_prompt"`
	UserID     int64  `json:"user_id"`
	APIKey     string `json:"api_key"`
	APIBaseURL string `json:"api_base_url"`
	LaunchDate string `json:"launch_date,omitempty"` // Optional: campaign launch date for fatigue filtering
}

// AutopilotAgent2Response represents the response from Agent 2
type AutopilotAgent2Response struct {
	Success           bool   `json:"success"`
	GroupID           int64  `json:"group_id"`
	GroupName         string `json:"group_name"`
	TargetCount       int    `json:"target_count"`
	FilterDescription string `json:"filter_description"`
	Error             string `json:"error,omitempty"`
}

// AutopilotAgent3Request represents the request for template & landing page
type AutopilotAgent3Request struct {
	Prompt string `json:"prompt"`
	UserID int64  `json:"user_id"`
}

// AutopilotAgent3Response represents the response from Agent 3
type AutopilotAgent3Response struct {
	Success             bool   `json:"success"`
	MatchedTemplateID   int64  `json:"matched_template_id"`
	MatchedTemplateName string `json:"matched_template_name"`
	MatchedPageID       int64  `json:"matched_page_id"`
	MatchedPageName     string `json:"matched_page_name"`
	TemplateScore       int    `json:"template_score"`
	PageScore           int    `json:"page_score"`
	ActionTaken         string `json:"action_taken"`
	Explanation         string `json:"explanation"`
	Confidence          int    `json:"confidence"`
	Reasoning           string `json:"reasoning"`
	Error               string `json:"error,omitempty"`
}

// AutopilotAgent1 handles email type matching via n8n workflow
// POST /api/campaigns/ai-workflow/1
func (as *Server) AutopilotAgent1(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req AutopilotAgent1Request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	if req.UserPrompt == "" {
		JSONResponse(w, models.Response{Success: false, Message: "User prompt is required"}, http.StatusBadRequest)
		return
	}

	// Get n8n webhook URL for AI Workflow 1
	webhookURL := os.Getenv("AI_WORKFLOW_1_WEBHOOK")
	if webhookURL == "" {
		log.Error("AI_WORKFLOW_1_WEBHOOK environment variable not set")
		JSONResponse(w, models.Response{Success: false, Message: "AI Workflow 1 not configured"}, http.StatusInternalServerError)
		return
	}

	// Call n8n webhook
	payload := map[string]interface{}{
		"prompt": req.UserPrompt,
	}

	response, err := callN8NWebhook(webhookURL, payload)
	if err != nil {
		log.Errorf("Failed to call AI Workflow 1: %v", err)
		JSONResponse(w, AutopilotAgent1Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to process request: %v", err),
		}, http.StatusInternalServerError)
		return
	}

	// Parse n8n response
	var agentResponse AutopilotAgent1Response
	err = json.Unmarshal(response, &agentResponse)
	if err != nil {
		log.Errorf("Failed to parse AI Workflow 1 response: %v", err)
		JSONResponse(w, AutopilotAgent1Response{
			Success: false,
			Error:   "Failed to parse AI response",
		}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, agentResponse, http.StatusOK)
}

// AutopilotAgent2 handles target filtering and group creation via n8n workflow
// POST /api/campaigns/ai-workflow/2
func (as *Server) AutopilotAgent2(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req AutopilotAgent2Request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	if req.UserPrompt == "" {
		JSONResponse(w, models.Response{Success: false, Message: "User prompt is required"}, http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := ctx.Get(r, "user_id").(int64)

	// Get API key from request header
	apiKey := r.Header.Get("Authorization")

	// Get API base URL from environment (prefer FYPHISH_API_BASE_URL for internal API calls)
	apiBaseURL := os.Getenv("FYPHISH_API_BASE_URL")
	if apiBaseURL == "" {
		// Fallback to PUBLIC_BASE_URL
		apiBaseURL = os.Getenv("PUBLIC_BASE_URL")
	}
	if apiBaseURL == "" {
		// Fallback to request host
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		apiBaseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	// Get n8n webhook URL for AI Workflow 2
	webhookURL := os.Getenv("AI_WORKFLOW_2_WEBHOOK")
	if webhookURL == "" {
		log.Error("AI_WORKFLOW_2_WEBHOOK environment variable not set")
		JSONResponse(w, models.Response{Success: false, Message: "AI Workflow 2 not configured"}, http.StatusInternalServerError)
		return
	}

	// Call n8n webhook
	payload := map[string]interface{}{
		"user_prompt":  req.UserPrompt,
		"user_id":      userID,
		"api_key":      apiKey,
		"api_base_url": apiBaseURL,
		"launch_date":  req.LaunchDate, // For filtering targets based on last_campaign_date
	}

	response, err := callN8NWebhook(webhookURL, payload)
	if err != nil {
		log.Errorf("Failed to call AI Workflow 2: %v", err)
		JSONResponse(w, AutopilotAgent2Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to process request: %v", err),
		}, http.StatusInternalServerError)
		return
	}

	// Parse n8n response
	var agentResponse AutopilotAgent2Response
	err = json.Unmarshal(response, &agentResponse)
	if err != nil {
		log.Errorf("Failed to parse AI Workflow 2 response: %v", err)
		JSONResponse(w, AutopilotAgent2Response{
			Success: false,
			Error:   "Failed to parse AI response",
		}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, agentResponse, http.StatusOK)
}

// AutopilotAgent3 handles template & landing page generation via n8n workflow
// POST /api/campaigns/ai-workflow/3
func (as *Server) AutopilotAgent3(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req AutopilotAgent3Request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Prompt is required"}, http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := ctx.Get(r, "user_id").(int64)

	// Get n8n webhook URL for AI Workflow 3
	webhookURL := os.Getenv("AI_WORKFLOW_3_WEBHOOK")
	if webhookURL == "" {
		log.Error("AI_WORKFLOW_3_WEBHOOK environment variable not set")
		JSONResponse(w, models.Response{Success: false, Message: "AI Workflow 3 not configured"}, http.StatusInternalServerError)
		return
	}

	// Call n8n webhook (simplified payload for MVP)
	payload := map[string]interface{}{
		"prompt":  req.Prompt,
		"user_id": userID,
	}

	response, err := callN8NWebhook(webhookURL, payload)
	if err != nil {
		log.Errorf("Failed to call AI Workflow 3: %v", err)
		JSONResponse(w, AutopilotAgent3Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to process request: %v", err),
		}, http.StatusInternalServerError)
		return
	}

	// Parse n8n response
	var agentResponse AutopilotAgent3Response
	err = json.Unmarshal(response, &agentResponse)
	if err != nil {
		log.Errorf("Failed to parse AI Workflow 3 response: %v", err)
		JSONResponse(w, AutopilotAgent3Response{
			Success: false,
			Error:   "Failed to parse AI response",
		}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, agentResponse, http.StatusOK)
}

// callN8NWebhook sends a POST request to n8n webhook with JWT authentication
func callN8NWebhook(webhookURL string, payload map[string]interface{}) ([]byte, error) {
	// Generate JWT token
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable not set")
	}

	token, err := generateAutopilotJWT(jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %v", err)
	}

	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	log.Debugf("Sending to n8n webhook: %s", string(payloadBytes))

	// Create context with timeout
	httpCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create HTTP request
	req, err := http.NewRequestWithContext(httpCtx, "POST", webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("n8n webhook returned error (status %d): %s", resp.StatusCode, string(body))
	}

	log.Debugf("n8n webhook response: %s", string(body))
	return body, nil
}

// generateAutopilotJWT generates an HS256 JWT token for n8n webhook authentication
func generateAutopilotJWT(secret string) (string, error) {
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
		"sub": "fyphish-autopilot",
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

// Note: base64URLEncode and hmacSHA256 are defined in util.go
