package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// EmailAccounts handles requests for the /api/email_accounts/ endpoint
func (as *Server) EmailAccounts(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		accounts, err := models.GetEmailAccounts()
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching email accounts"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, accounts, http.StatusOK)

	case r.Method == "POST":
		account := models.EmailAccount{}
		err := json.NewDecoder(r.Body).Decode(&account)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}

		// Check if email already exists
		_, err = models.GetEmailAccountByEmail(account.Email)
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Email account already exists"}, http.StatusConflict)
			return
		}

		// Generate n8n credential name
		credentialName, err := models.GenerateN8NCredentialName(account.EmailType)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Failed to generate credential name"}, http.StatusInternalServerError)
			return
		}

		// Create n8n credential
		credID, credName, err := account.CreateN8NCredential(credentialName)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Failed to create n8n credential: %v", err)}, http.StatusInternalServerError)
			return
		}

		// Update account with n8n credential information
		account.N8NCredentialID = credID
		account.N8NCredentialName = credName

		// Create the account in database
		err = models.PostEmailAccount(&account)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}

		JSONResponse(w, account, http.StatusCreated)
	}
}

// EmailAccount handles requests for the /api/email_accounts/:id endpoint
func (as *Server) EmailAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	account, err := models.GetEmailAccount(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Email account not found"}, http.StatusNotFound)
		} else {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching email account"}, http.StatusInternalServerError)
		}
		return
	}

	switch {
	case r.Method == "GET":
		JSONResponse(w, account, http.StatusOK)

	case r.Method == "DELETE":
		err = models.DeleteEmailAccount(id)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting email account"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Email account deleted successfully"}, http.StatusOK)

	case r.Method == "PUT":
		updatedAccount := models.EmailAccount{}
		err = json.NewDecoder(r.Body).Decode(&updatedAccount)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}

		if updatedAccount.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "ID mismatch"}, http.StatusBadRequest)
			return
		}

		err = models.PutEmailAccount(&updatedAccount)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}

		JSONResponse(w, updatedAccount, http.StatusOK)
	}
}

// EmailAccountByType handles requests for the /api/email_accounts/type/:type endpoint
// Returns the first active email account of the specified type
func (as *Server) EmailAccountByType(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	accountType := vars["type"]

	account, err := models.GetEmailAccountByType(accountType)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "No active email account found for type: " + accountType}, http.StatusNotFound)
		} else {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching email account"}, http.StatusInternalServerError)
		}
		return
	}

	JSONResponse(w, account, http.StatusOK)
}
