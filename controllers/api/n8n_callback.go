package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// FlexibleInt64 is a custom type that can unmarshal from both string and int
type FlexibleInt64 int64

// UnmarshalJSON implements custom unmarshaling to accept both string and int
func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexibleInt64(i)
		return nil
	}

	// If that fails, try as string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Convert string to int64
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("campaign_id must be a number or numeric string: %v", err)
	}

	*f = FlexibleInt64(i)
	return nil
}

// N8NEmailStatusPayload represents the callback payload from n8n
type N8NEmailStatusPayload struct {
	RId        string                 `json:"rid"`         // Result ID to identify the recipient
	CampaignId FlexibleInt64          `json:"campaign_id"` // Campaign ID for validation (accepts string or int)
	Event      string                 `json:"event"`       // Event type: "sent", "error", "bounce"
	Timestamp  time.Time              `json:"timestamp"`   // When the event occurred
	Details    map[string]interface{} `json:"details"`     // Additional event details
	Error      string                 `json:"error,omitempty"` // Error message if applicable
}

// N8NEmailCallback handles email status callbacks from n8n
// POST /api/webhooks/n8n/status
func (as *Server) N8NEmailCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse the callback payload
	var payload N8NEmailStatusPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		log.Errorf("Failed to decode n8n callback payload: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON payload"}, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.RId == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing rid field"}, http.StatusBadRequest)
		return
	}
	if payload.Event == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing event field"}, http.StatusBadRequest)
		return
	}

	log.Infof("Received n8n callback: RId=%s, Event=%s, CampaignId=%d", payload.RId, payload.Event, int64(payload.CampaignId))

	// Get the result by RId
	result, err := models.GetResult(payload.RId)
	if err != nil {
		log.Errorf("Result not found for RId %s: %v", payload.RId, err)
		JSONResponse(w, models.Response{Success: false, Message: "Result not found"}, http.StatusNotFound)
		return
	}

	// Validate campaign ID matches (security check)
	campaignId := int64(payload.CampaignId)
	if campaignId != 0 && result.CampaignId != campaignId {
		log.Warnf("Campaign ID mismatch for RId %s: expected %d, got %d", payload.RId, result.CampaignId, campaignId)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign ID mismatch"}, http.StatusBadRequest)
		return
	}

	// Process the event based on type
	switch payload.Event {
	case "sent":
		err = result.HandleEmailSent()
		if err != nil {
			log.Errorf("Failed to handle email sent event for RId %s: %v", payload.RId, err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("Email sent event recorded for RId %s", payload.RId)

	case "error", "bounce", "failed":
		// Extract error message
		errorMsg := payload.Error
		if errorMsg == "" && payload.Details != nil {
			if msg, ok := payload.Details["error"].(string); ok {
				errorMsg = msg
			} else if msg, ok := payload.Details["message"].(string); ok {
				errorMsg = msg
			}
		}
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("Email %s", payload.Event)
		}

		err = result.HandleEmailError(fmt.Errorf("%s", errorMsg))
		if err != nil {
			log.Errorf("Failed to handle email error event for RId %s: %v", payload.RId, err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Warnf("Email error event recorded for RId %s: %s", payload.RId, errorMsg)

	case "opened":
		// Email opened tracking (if n8n supports tracking pixels)
		details := models.EventDetails{}
		err = result.HandleEmailOpened(details)
		if err != nil {
			log.Errorf("Failed to handle email opened event for RId %s: %v", payload.RId, err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("Email opened event recorded for RId %s", payload.RId)

	case "clicked":
		// Link clicked tracking
		details := models.EventDetails{}
		err = result.HandleClickedLink(details)
		if err != nil {
			log.Errorf("Failed to handle clicked link event for RId %s: %v", payload.RId, err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("Clicked link event recorded for RId %s", payload.RId)

	default:
		log.Warnf("Unknown event type from n8n: %s for RId %s", payload.Event, payload.RId)
		JSONResponse(w, models.Response{Success: false, Message: "Unknown event type"}, http.StatusBadRequest)
		return
	}

	// Return success response
	JSONResponse(w, models.Response{
		Success: true,
		Message: fmt.Sprintf("Event %s processed successfully for RId %s", payload.Event, payload.RId),
	}, http.StatusOK)
}
