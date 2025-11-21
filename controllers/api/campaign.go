package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}

// ValidateCampaignRateLimitRequest represents the request payload for rate limit validation
type ValidateCampaignRateLimitRequest struct {
	LaunchDate time.Time `json:"launch_date"`
	SendByDate time.Time `json:"send_by_date"`
	GroupIDs   []int64   `json:"group_ids"`
}

// ValidateCampaignRateLimitResponse represents the response for rate limit validation
type ValidateCampaignRateLimitResponse struct {
	Success bool                      `json:"success"`
	Warning *models.RateLimitWarning  `json:"warning,omitempty"`
	Message string                    `json:"message,omitempty"`
}

// ValidateCampaignRateLimit validates if a campaign's send-by date meets rate limiting requirements
// POST /api/campaigns/validate-rate-limit
func (as *Server) ValidateCampaignRateLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req ValidateCampaignRateLimitRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.LaunchDate.IsZero() {
		JSONResponse(w, models.Response{Success: false, Message: "Launch date is required"}, http.StatusBadRequest)
		return
	}

	if len(req.GroupIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "At least one group is required"}, http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := ctx.Get(r, "user_id").(int64)

	// Count total recipients from all groups
	totalRecipients := 0
	for _, groupID := range req.GroupIDs {
		g, err := models.GetGroup(groupID, userID)
		if err != nil {
			log.Errorf("Failed to get group %d: %v", groupID, err)
			JSONResponse(w, models.Response{Success: false, Message: "Failed to retrieve group information"}, http.StatusInternalServerError)
			return
		}
		totalRecipients += len(g.Targets)
	}

	// If no recipients, return success
	if totalRecipients == 0 {
		JSONResponse(w, ValidateCampaignRateLimitResponse{
			Success: true,
			Message: "No recipients found in selected groups",
		}, http.StatusOK)
		return
	}

	// Validate rate limit
	warning := models.ValidateCampaignRateLimit(req.LaunchDate, req.SendByDate, totalRecipients)

	if warning != nil {
		// Rate limit is too aggressive - return warning
		JSONResponse(w, ValidateCampaignRateLimitResponse{
			Success: false,
			Warning: warning,
			Message: "Campaign sending rate is too aggressive",
		}, http.StatusOK)
		return
	}

	// Rate limit is acceptable or will be auto-calculated
	JSONResponse(w, ValidateCampaignRateLimitResponse{
		Success: true,
		Message: "Campaign rate limit is acceptable",
	}, http.StatusOK)
}
