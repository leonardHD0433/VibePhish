package models

import (
	"bytes"
	"fmt"
	"io"

	log "github.com/gophish/gophish/logger"
)

// LaunchN8NBatchCampaign sends a single batch webhook to n8n with all recipients
// This bypasses the maillog system entirely and lets n8n handle scheduling and callbacks
func LaunchN8NBatchCampaign(c *Campaign) error {
	log.Infof("Launching n8n batch campaign: CampaignId=%d, Recipients=%d", c.Id, len(c.Results))

	// Get n8n dialer with campaign context
	dialer, err := c.EmailAccount.GetN8NDialer(c)
	if err != nil {
		return fmt.Errorf("failed to get n8n dialer: %v", err)
	}

	// Create sender
	sender, err := dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to create n8n sender: %v", err)
	}
	defer sender.Close()

	// Build recipient list from Results
	recipients := make([]string, 0, len(c.Results))
	for _, result := range c.Results {
		recipients = append(recipients, result.Email)
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no recipients found for campaign %d", c.Id)
	}

	// Generate email message (subject and body)
	// This will be the same for all recipients - personalization happens in n8n or already in template
	msg := &mockWriterTo{
		campaign: c,
	}

	// Send batch to n8n (single webhook call with all recipients)
	err = sender.Send(c.EmailAccount.Email, recipients, msg)
	if err != nil {
		log.Errorf("Failed to send batch to n8n for campaign %d: %v", c.Id, err)
		return fmt.Errorf("failed to send batch to n8n: %v", err)
	}

	log.Infof("Successfully sent batch webhook to n8n for campaign %d with %d recipients", c.Id, len(recipients))
	return nil
}

// mockWriterTo implements io.WriterTo for generating email messages
type mockWriterTo struct {
	campaign *Campaign
}

// WriteTo writes the email message (headers + body) to the provided writer
func (m *mockWriterTo) WriteTo(w io.Writer) (int64, error) {
	// Build email message with headers and body
	var buf bytes.Buffer

	// Write headers
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", m.campaign.Template.Subject))
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("\r\n")

	// Write HTML body
	buf.WriteString(m.campaign.Template.HTML)

	// Write to provided writer
	n, err := w.Write(buf.Bytes())
	return int64(n), err
}

// ShouldUseN8NBatchLaunch determines if a campaign should use true batch sending
// For now, ALL campaigns use n8n, so always return true
// In the future, this could check email account configuration
func ShouldUseN8NBatchLaunch(c *Campaign) bool {
	// Check if email account has n8n credential configured
	return c.EmailAccount.N8NCredentialID != "" || c.EmailAccount.N8NCredentialName != ""
}

// LaunchCampaignWithN8N is a wrapper that decides between batch and traditional launch
func LaunchCampaignWithN8N(c *Campaign) error {
	if ShouldUseN8NBatchLaunch(c) {
		log.Infof("Using n8n batch launch for campaign %d", c.Id)
		return LaunchN8NBatchCampaign(c)
	}

	// Fallback to traditional maillog system (for SMTP-based campaigns)
	log.Infof("Using traditional maillog launch for campaign %d (non-n8n)", c.Id)
	// This would be the existing maillog creation logic
	// For now, since we're all-n8n, this won't be reached
	return fmt.Errorf("traditional SMTP launch not implemented")
}

