package models

import (
	"errors"
	"fmt"

	"github.com/gophish/gomail"
	"github.com/gophish/gophish/config"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/sirupsen/logrus"
)

// PreviewPrefix is the standard prefix added to the rid parameter when sending
// test emails.
const PreviewPrefix = "preview-"

// ErrEmailTypeNotSpecified is returned when no email type is provided
var ErrEmailTypeNotSpecified = errors.New("No email type specified")

// EmailRequest is the structure of a request
// to send a test email to test an SMTP connection.
// This type implements the mailer.Mail interface.
type EmailRequest struct {
	Id          int64        `json:"-"`
	Template    Template     `json:"template"`
	TemplateId  int64        `json:"-"`
	Page        Page         `json:"page"`
	PageId      int64        `json:"-"`
	EmailType   string       `json:"email_type"`
	URL         string       `json:"url"`
	Tracker     string       `json:"tracker" gorm:"-"`
	TrackingURL string       `json:"tracking_url" gorm:"-"`
	UserId      int64        `json:"-"`
	ErrorChan   chan (error) `json:"-" gorm:"-"`
	RId         string       `json:"id"`
	BaseRecipient
}

func (s *EmailRequest) getBaseURL() string {
	return s.URL
}

func (s *EmailRequest) getFromAddress() string {
	// For n8n webhook, from address is determined by the email account selected
	// Return placeholder address for template generation
	return "test@fyphish.local"
}

// Validate ensures the SendTestEmailRequest structure
// is valid.
func (s *EmailRequest) Validate() error {
	switch {
	case s.Email == "":
		return ErrEmailNotSpecified
	case s.EmailType == "":
		return ErrEmailTypeNotSpecified
	}
	return nil
}

// Backoff treats temporary errors as permanent since this is expected to be a
// synchronous operation. It returns any errors given back to the ErrorChan
func (s *EmailRequest) Backoff(reason error) error {
	s.ErrorChan <- reason
	return nil
}

// Error returns an error on the ErrorChan.
func (s *EmailRequest) Error(err error) error {
	s.ErrorChan <- err
	return nil
}

// Success returns nil on the ErrorChan to indicate that the email was sent
// successfully.
func (s *EmailRequest) Success() error {
	s.ErrorChan <- nil
	return nil
}

// PostEmailRequest stores a SendTestEmailRequest in the database.
func PostEmailRequest(s *EmailRequest) error {
	// Generate an ID to be used in the underlying Result object
	rid, err := generateResultId()
	if err != nil {
		return err
	}
	s.RId = fmt.Sprintf("%s%s", PreviewPrefix, rid)
	return db.Save(&s).Error
}

// GetEmailRequestByResultId retrieves the EmailRequest by the underlying rid
// parameter.
func GetEmailRequestByResultId(id string) (EmailRequest, error) {
	s := EmailRequest{}
	err := db.Table("email_requests").Where("r_id=?", id).First(&s).Error
	return s, err
}

// Generate fills in the details of a gomail.Message with the contents
// from the SendTestEmailRequest.
func (s *EmailRequest) Generate(msg *gomail.Message) error {
	log.Info("Generate() called - starting email generation")

	ptx, err := NewPhishingTemplateContext(s, s.BaseRecipient, s.RId)
	if err != nil {
		log.Error("Error creating phishing template context:", err)
		return err
	}

	log.Info("Template context created successfully")

	log.Info("Executing URL template")
	url, err := ExecuteTemplate(s.URL, ptx)
	if err != nil {
		log.Error("Error executing URL template:", err)
		return err
	}
	s.URL = url

	log.Info("Setting transparency headers")
	// Add the transparency headers
	msg.SetHeader("X-Mailer", config.ServerName)
	if conf.ContactAddress != "" {
		msg.SetHeader("X-Gophish-Contact", conf.ContactAddress)
	}

	log.Info("Executing subject template")
	// Parse remaining templates
	subject, err := ExecuteTemplate(s.Template.Subject, ptx)
	if err != nil {
		log.Error("Error executing subject template:", err)
	}
	// don't set the Subject header if it is blank
	if subject != "" {
		msg.SetHeader("Subject", subject)
	}

	log.Info("Subject header set, moving to To header")

	// Use SetAddressHeader for proper email address formatting
	log.WithFields(logrus.Fields{
		"email":      s.Email,
		"first_name": s.FirstName,
		"last_name":  s.LastName,
	}).Info("About to set To header")

	if s.FirstName != "" && s.LastName != "" {
		log.Info("Setting To with name")
		msg.SetAddressHeader("To", s.Email, fmt.Sprintf("%s %s", s.FirstName, s.LastName))
	} else {
		log.Info("Setting To without name")
		msg.SetAddressHeader("To", s.Email, "")
	}

	log.Info("To header set successfully")
	if s.Template.Text != "" {
		text, err := ExecuteTemplate(s.Template.Text, ptx)
		if err != nil {
			log.Error(err)
		}
		msg.SetBody("text/plain", text)
	}
	if s.Template.HTML != "" {
		html, err := ExecuteTemplate(s.Template.HTML, ptx)
		if err != nil {
			log.Error(err)
		}
		if s.Template.Text == "" {
			msg.SetBody("text/html", html)
		} else {
			msg.AddAlternative("text/html", html)
		}
	}

	// Attach the files
	for _, a := range s.Template.Attachments {
		addAttachment(msg, a, ptx)
	}

	return nil
}

// GetDialer is not needed for n8n webhook sending but kept for interface compatibility
func (s *EmailRequest) GetDialer() (mailer.Dialer, error) {
	return nil, nil
}
