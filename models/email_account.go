package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
)

// EmailAccount represents an email sender account used for campaigns
// Uses Microsoft Outlook OAuth2 API credentials in n8n (no SMTP fields needed)
type EmailAccount struct {
	Id                int64     `json:"id" gorm:"column:id; primary_key:yes"`
	Email             string    `json:"email" gorm:"column:email; unique; not null"`
	EmailType         string    `json:"email_type" gorm:"column:email_type; not null"` // noreply, notification, forgetpassword, marketing, support
	N8NCredentialID   string    `json:"n8n_credential_id" gorm:"column:n8n_credential_id"`
	N8NCredentialName string    `json:"n8n_credential_name" gorm:"column:n8n_credential_name"`
	UsageCount        int       `json:"usage_count" gorm:"column:usage_count; default:0"`
	LastUsed          time.Time `json:"last_used" gorm:"column:last_used"`
	IsActive          bool      `json:"is_active" gorm:"column:is_active; default:true"`
	CreatedAt         time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the table name for EmailAccount
func (ea *EmailAccount) TableName() string {
	return "email_accounts"
}

// Validate ensures the email account has required fields
func (ea *EmailAccount) Validate() error {
	if ea.Email == "" {
		return errors.New("email address is required")
	}
	if ea.EmailType == "" {
		return errors.New("email type is required")
	}

	// Validate type exists in database and is active
	if err := ValidateEmailType(ea.EmailType); err != nil {
		return err
	}

	return nil
}

// GetEmailAccounts returns all email accounts from the database
func GetEmailAccounts() ([]EmailAccount, error) {
	accounts := []EmailAccount{}
	err := db.Order("created_at DESC").Find(&accounts).Error
	return accounts, err
}

// GetEmailAccount returns an email account by ID
func GetEmailAccount(id int64) (EmailAccount, error) {
	account := EmailAccount{}
	err := db.Where("id = ?", id).First(&account).Error
	return account, err
}

// GetEmailAccountByEmail returns an email account by email address
func GetEmailAccountByEmail(email string) (EmailAccount, error) {
	account := EmailAccount{}
	err := db.Where("email = ?", email).First(&account).Error
	return account, err
}

// GetEmailAccountByType returns the first active email account of a specific type
func GetEmailAccountByType(accountType string) (EmailAccount, error) {
	account := EmailAccount{}
	err := db.Where("email_type = ? AND is_active = ?", accountType, true).First(&account).Error
	return account, err
}

// PostEmailAccount creates a new email account in the database
func PostEmailAccount(account *EmailAccount) error {
	// Validate the account
	if err := account.Validate(); err != nil {
		return err
	}

	// Check if email already exists
	temp := EmailAccount{}
	err := db.Where("email = ?", account.Email).First(&temp).Error
	if err == nil {
		return errors.New("email account already exists")
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Set timestamps
	account.CreatedAt = time.Now().UTC()
	account.UpdatedAt = time.Now().UTC()

	// Create the account
	err = db.Create(account).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// PutEmailAccount updates an existing email account
func PutEmailAccount(account *EmailAccount) error {
	// Validate the account
	if err := account.Validate(); err != nil {
		return err
	}

	// Check if account exists
	temp := EmailAccount{}
	err := db.Where("id = ?", account.Id).First(&temp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("email account not found")
		}
		return err
	}

	// Check if email is being changed and conflicts with another account
	if account.Email != temp.Email {
		conflict := EmailAccount{}
		err = db.Where("email = ? AND id != ?", account.Email, account.Id).First(&conflict).Error
		if err == nil {
			return errors.New("email address already in use by another account")
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
	}

	// Update timestamp
	account.UpdatedAt = time.Now().UTC()

	// Update the account
	err = db.Save(account).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// DeleteEmailAccount deletes an email account from the database
func DeleteEmailAccount(id int64) error {
	// Check if account exists
	account := EmailAccount{}
	err := db.Where("id = ?", id).First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("email account not found")
		}
		return err
	}

	// Delete the account
	err = db.Delete(&account).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// IncrementUsageCount increments the usage counter and updates last_used timestamp
func (ea *EmailAccount) IncrementUsageCount() error {
	ea.UsageCount++
	ea.LastUsed = time.Now().UTC()
	return db.Model(ea).Updates(map[string]interface{}{
		"usage_count": ea.UsageCount,
		"last_used":   ea.LastUsed,
	}).Error
}

// GenerateN8NCredentialName generates an incremental credential name based on type
// Format: {type}-{number}, e.g., noreply-1, noreply-2, notification-1
func GenerateN8NCredentialName(accountType string) (string, error) {
	// Get all existing accounts of the same type
	var accounts []EmailAccount
	err := db.Where("email_type = ?", accountType).Order("id DESC").Find(&accounts).Error
	if err != nil {
		log.Error(err)
		return "", errors.New("failed to query existing accounts")
	}

	// Find the highest number for this type
	highestNum := 0
	prefix := accountType + "-"
	for _, acc := range accounts {
		if acc.N8NCredentialName != "" && len(acc.N8NCredentialName) > len(prefix) {
			// Extract the number suffix
			numStr := acc.N8NCredentialName[len(prefix):]
			if num, err := strconv.Atoi(numStr); err == nil && num > highestNum {
				highestNum = num
			}
		}
	}

	// Generate the next credential name
	nextNum := highestNum + 1
	return fmt.Sprintf("%s-%d", accountType, nextNum), nil
}

// N8NCredentialResponse represents the response from n8n credential creation API
type N8NCredentialResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// CreateN8NCredential creates a Microsoft Outlook OAuth2 credential in n8n via API
// Note: The credential is created with empty data and must be configured interactively
// in the n8n UI to complete the OAuth2 authorization flow
// Returns the credential ID and name
func (ea *EmailAccount) CreateN8NCredential(credentialName string) (string, string, error) {
	// Get n8n API configuration from environment
	n8nAPIURL := os.Getenv("N8N_API_URL")
	n8nAPIKey := os.Getenv("N8N_API")

	if n8nAPIURL == "" || n8nAPIKey == "" {
		return "", "", errors.New("n8n API configuration missing (N8N_API_URL or N8N_API)")
	}

	// Construct the API endpoint
	apiEndpoint := n8nAPIURL + "/api/v1/credentials"

	// Get Microsoft Azure app credentials from environment variables
	clientID := os.Getenv("MICROSOFT_CLIENT_ID")
	clientSecret := os.Getenv("N8N_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("MICROSOFT_CLIENT_ID and N8N_CLIENT_SECRET must be set in environment")
	}

	// Prepare the credential payload for Microsoft Outlook OAuth2
	// Uses Azure app credentials from environment variables
	// OAuth2 authorization must still be completed in n8n UI (interactive flow)
	payload := map[string]interface{}{
		"name": credentialName,
		"type": "microsoftOutlookOAuth2Api",
		"data": map[string]interface{}{
			"clientId":                      clientID,
			"clientSecret":                  clientSecret,
			"userPrincipalName":             ea.Email, // The phishing email address
			"sendAdditionalBodyProperties":  false,
			"additionalBodyProperties":      map[string]interface{}{},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error(err)
		return "", "", errors.New("failed to marshal credential payload")
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Error(err)
		return "", "", errors.New("failed to create HTTP request")
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return "", "", fmt.Errorf("failed to call n8n API: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return "", "", errors.New("failed to read n8n API response")
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Errorf("n8n API returned error status %d: %s", resp.StatusCode, string(body))
		return "", "", fmt.Errorf("n8n API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var credResp N8NCredentialResponse
	err = json.Unmarshal(body, &credResp)
	if err != nil {
		log.Error(err)
		return "", "", errors.New("failed to parse n8n API response")
	}

	log.Infof("Created n8n credential: ID=%s, Name=%s", credResp.ID, credResp.Name)
	return credResp.ID, credResp.Name, nil
}
