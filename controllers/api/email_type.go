package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// EmailTypes handles requests for the /api/email_types/ endpoint
func (as *Server) EmailTypes(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		// Get only active types by default
		types, err := models.GetEmailTypes()
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching email types"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, types, http.StatusOK)

	case r.Method == "POST":
		emailType := models.EmailType{}
		err := json.NewDecoder(r.Body).Decode(&emailType)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}

		// Check if value already exists
		_, err = models.GetEmailTypeByValue(emailType.Value)
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Email type with this value already exists"}, http.StatusConflict)
			return
		}

		// Create the type
		err = models.PostEmailType(&emailType)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}

		JSONResponse(w, emailType, http.StatusCreated)
	}
}

// EmailType handles requests for the /api/email_types/:id endpoint
func (as *Server) EmailType(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	emailType, err := models.GetEmailType(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Email type not found"}, http.StatusNotFound)
		} else {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching email type"}, http.StatusInternalServerError)
		}
		return
	}

	switch {
	case r.Method == "GET":
		JSONResponse(w, emailType, http.StatusOK)

	case r.Method == "DELETE":
		err = models.DeleteEmailType(id)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Email type deleted successfully"}, http.StatusOK)

	case r.Method == "PUT":
		updatedType := models.EmailType{}
		err = json.NewDecoder(r.Body).Decode(&updatedType)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}

		if updatedType.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "ID mismatch"}, http.StatusBadRequest)
			return
		}

		err = models.PutEmailType(&updatedType)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}

		JSONResponse(w, updatedType, http.StatusOK)
	}
}

// EmailTypesAll handles requests for the /api/email_types/all endpoint
// Returns all types including inactive ones (admin only)
func (as *Server) EmailTypesAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	types, err := models.GetAllEmailTypes()
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error fetching email types"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, types, http.StatusOK)
}
