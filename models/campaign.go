package models

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Campaign is a struct representing a created campaign
type Campaign struct {
	Id            int64     `json:"id"`
	UserId        int64     `json:"-"`
	Name          string    `json:"name" sql:"not null"`
	CreatedDate   time.Time `json:"created_date"`
	LaunchDate    time.Time `json:"launch_date"`
	SendByDate    time.Time `json:"send_by_date"`
	CompletedDate time.Time `json:"completed_date"`
	TemplateId    int64     `json:"-"`
	Template      Template  `json:"template"`
	PageId        int64     `json:"-"`
	Page          Page      `json:"page"`
	Status         string       `json:"status"`
	Results        []Result     `json:"results,omitempty"`
	Groups         []Group      `json:"groups,omitempty"`
	Events         []Event      `json:"timeline,omitempty"`
	EmailAccountId int64        `json:"-"`
	EmailAccount   EmailAccount `json:"email_account"`
	EmailType      string       `json:"email_type" gorm:"-"` // Transient field for frontend, not stored in DB
	URL            string       `json:"url"`
}

// CampaignResults is a struct representing the results from a campaign
type CampaignResults struct {
	Id      int64    `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Results []Result `json:"results,omitempty"`
	Events  []Event  `json:"timeline,omitempty"`
}

// CampaignSummaries is a struct representing the overview of campaigns
type CampaignSummaries struct {
	Total     int64             `json:"total"`
	Campaigns []CampaignSummary `json:"campaigns"`
}

// CampaignSummary is a struct representing the overview of a single camaign
type CampaignSummary struct {
	Id            int64         `json:"id"`
	CreatedDate   time.Time     `json:"created_date"`
	LaunchDate    time.Time     `json:"launch_date"`
	SendByDate    time.Time     `json:"send_by_date"`
	CompletedDate time.Time     `json:"completed_date"`
	Status        string        `json:"status"`
	Name          string        `json:"name"`
	Stats         CampaignStats `json:"stats"`
}

// CampaignStats is a struct representing the statistics for a single campaign
type CampaignStats struct {
	Total         int64 `json:"total"`
	EmailsSent    int64 `json:"sent"`
	OpenedEmail   int64 `json:"opened"`
	ClickedLink   int64 `json:"clicked"`
	SubmittedData int64 `json:"submitted_data"`
	EmailReported int64 `json:"email_reported"`
	Error         int64 `json:"error"`
}

// Event contains the fields for an event
// that occurs during the campaign
type Event struct {
	Id         int64     `json:"-"`
	CampaignId int64     `json:"campaign_id"`
	Email      string    `json:"email"`
	Time       time.Time `json:"time"`
	Message    string    `json:"message"`
	Details    string    `json:"details"`
}

// EventDetails is a struct that wraps common attributes we want to store
// in an event
type EventDetails struct {
	Payload url.Values        `json:"payload"`
	Browser map[string]string `json:"browser"`
}

// EventError is a struct that wraps an error that occurs when sending an
// email to a recipient
type EventError struct {
	Error string `json:"error"`
}

// ErrCampaignNameNotSpecified indicates there was no template given by the user
var ErrCampaignNameNotSpecified = errors.New("Campaign name not specified")

// ErrGroupNotSpecified indicates there was no template given by the user
var ErrGroupNotSpecified = errors.New("No groups specified")

// ErrTemplateNotSpecified indicates there was no template given by the user
var ErrTemplateNotSpecified = errors.New("No email template specified")

// ErrPageNotSpecified indicates a landing page was not provided for the campaign
var ErrPageNotSpecified = errors.New("No landing page specified")

// ErrEmailAccountNotSpecified indicates an email account was not provided for the campaign
var ErrEmailAccountNotSpecified = errors.New("No email account specified")

// ErrTemplateNotFound indicates the template specified does not exist in the database
var ErrTemplateNotFound = errors.New("Template not found")

// ErrGroupNotFound indicates a group specified by the user does not exist in the database
var ErrGroupNotFound = errors.New("Group not found")

// ErrPageNotFound indicates a page specified by the user does not exist in the database
var ErrPageNotFound = errors.New("Page not found")

// ErrEmailAccountNotFound indicates an email account specified by the user does not exist in the database
var ErrEmailAccountNotFound = errors.New("Email account not found")

// ErrInvalidSendByDate indicates that the user specified a send by date that occurs before the
// launch date
var ErrInvalidSendByDate = errors.New("The launch date must be before the \"send emails by\" date")

// RecipientParameter is the URL parameter that points to the result ID for a recipient.
const RecipientParameter = "rid"

// Validate checks to make sure there are no invalid fields in a submitted campaign
func (c *Campaign) Validate() error {
	switch {
	case c.Name == "":
		return ErrCampaignNameNotSpecified
	case len(c.Groups) == 0:
		return ErrGroupNotSpecified
	case c.Template.Name == "":
		return ErrTemplateNotSpecified
	case c.Page.Name == "":
		return ErrPageNotSpecified
	case c.EmailAccount.Email == "":
		return ErrEmailAccountNotSpecified
	case !c.SendByDate.IsZero() && !c.LaunchDate.IsZero() && c.SendByDate.Before(c.LaunchDate):
		return ErrInvalidSendByDate
	}
	return nil
}

// UpdateStatus changes the campaign status appropriately
func (c *Campaign) UpdateStatus(s string) error {
	// This could be made simpler, but I think there's a bug in gorm
	return db.Table("campaigns").Where("id=?", c.Id).Update("status", s).Error
}

// AddEvent creates a new campaign event in the database
func AddEvent(e *Event, campaignID int64) error {
	e.CampaignId = campaignID
	e.Time = time.Now().UTC()

	whs, err := GetActiveWebhooks()
	if err == nil {
		whEndPoints := []webhook.EndPoint{}
		for _, wh := range whs {
			whEndPoints = append(whEndPoints, webhook.EndPoint{
				URL:    wh.URL,
				Secret: wh.Secret,
			})
		}
		webhook.SendAll(whEndPoints, e)
	} else {
		log.Errorf("error getting active webhooks: %v", err)
	}

	return db.Save(e).Error
}

// getDetails retrieves the related attributes of the campaign
// from the database. If the Events and the Results are not available,
// an error is returned. Otherwise, the attribute name is set to [Deleted],
// indicating the user deleted the attribute (template, smtp, etc.)
func (c *Campaign) getDetails() error {
	err := db.Model(c).Related(&c.Results).Error
	if err != nil {
		log.Warnf("%s: results not found for campaign", err)
		return err
	}
	err = db.Model(c).Related(&c.Events).Error
	if err != nil {
		log.Warnf("%s: events not found for campaign", err)
		return err
	}
	err = db.Table("templates").Where("id=?", c.TemplateId).Find(&c.Template).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.Template = Template{Name: "[Deleted]"}
		log.Warnf("%s: template not found for campaign", err)
	}
	err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Warn(err)
		return err
	}
	err = db.Table("pages").Where("id=?", c.PageId).Find(&c.Page).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.Page = Page{Name: "[Deleted]"}
		log.Warnf("%s: page not found for campaign", err)
	}
	err = db.Table("email_accounts").Where("id=?", c.EmailAccountId).Find(&c.EmailAccount).Error
	if err != nil {
		// Check if the EmailAccount was deleted
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.EmailAccount = EmailAccount{Email: "[Deleted]"}
		log.Warnf("%s: email account not found for campaign", err)
	}
	return nil
}

// getBaseURL returns the Campaign's configured URL.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getBaseURL() string {
	return c.URL
}

// getFromAddress returns the Campaign's configured email account address.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getFromAddress() string {
	return c.EmailAccount.Email
}

// generateSendDate creates a sendDate
func (c *Campaign) generateSendDate(idx int, totalRecipients int) time.Time {
	// If no send date is specified, just return the launch date
	if c.SendByDate.IsZero() || c.SendByDate.Equal(c.LaunchDate) {
		return c.LaunchDate
	}
	// Otherwise, we can calculate the range of minutes to send emails
	// (since we only poll once per minute)
	totalMinutes := c.SendByDate.Sub(c.LaunchDate).Minutes()

	// Next, we can determine how many minutes should elapse between emails
	minutesPerEmail := totalMinutes / float64(totalRecipients)

	// Then, we can calculate the offset for this particular email
	offset := int(minutesPerEmail * float64(idx))

	// Finally, we can just add this offset to the launch date to determine
	// when the email should be sent
	return c.LaunchDate.Add(time.Duration(offset) * time.Minute)
}

// getCampaignStats returns a CampaignStats object for the campaign with the given campaign ID.
// It also backfills numbers as appropriate with a running total, so that the values are aggregated.
func getCampaignStats(cid int64) (CampaignStats, error) {
	s := CampaignStats{}
	query := db.Table("results").Where("campaign_id = ?", cid)
	err := query.Count(&s.Total).Error
	if err != nil {
		return s, err
	}
	query.Where("status=?", EventDataSubmit).Count(&s.SubmittedData)
	if err != nil {
		return s, err
	}
	query.Where("status=?", EventClicked).Count(&s.ClickedLink)
	if err != nil {
		return s, err
	}
	query.Where("reported=?", true).Count(&s.EmailReported)
	if err != nil {
		return s, err
	}
	// Every submitted data event implies they clicked the link
	s.ClickedLink += s.SubmittedData
	err = query.Where("status=?", EventOpened).Count(&s.OpenedEmail).Error
	if err != nil {
		return s, err
	}
	// Every clicked link event implies they opened the email
	s.OpenedEmail += s.ClickedLink
	err = query.Where("status=?", EventSent).Count(&s.EmailsSent).Error
	if err != nil {
		return s, err
	}
	// Every opened email event implies the email was sent
	s.EmailsSent += s.OpenedEmail
	err = query.Where("status=?", Error).Count(&s.Error).Error
	return s, err
}

// GetCampaigns returns the campaigns owned by the given user.
func GetCampaigns(uid int64) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Model(&User{Id: uid}).Related(&cs).Error
	if err != nil {
		log.Error(err)
	}
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// GetCampaignSummaries gets the summary objects for all the campaigns
// owned by the current user
func GetCampaignSummaries(uid int64) (CampaignSummaries, error) {
	overview := CampaignSummaries{}
	cs := []CampaignSummary{}
	// Get the basic campaign information
	query := db.Table("campaigns").Where("user_id = ?", uid)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return overview, err
	}
	for i := range cs {
		s, err := getCampaignStats(cs[i].Id)
		if err != nil {
			log.Error(err)
			return overview, err
		}
		cs[i].Stats = s
	}
	overview.Total = int64(len(cs))
	overview.Campaigns = cs
	return overview, nil
}

// GetCampaignSummary gets the summary object for a campaign specified by the campaign ID
func GetCampaignSummary(id int64, uid int64) (CampaignSummary, error) {
	cs := CampaignSummary{}
	query := db.Table("campaigns").Where("user_id = ? AND id = ?", uid, id)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return cs, err
	}
	s, err := getCampaignStats(cs.Id)
	if err != nil {
		log.Error(err)
		return cs, err
	}
	cs.Stats = s
	return cs, nil
}

// GetCampaignMailContext returns a campaign object with just the relevant
// data needed to generate and send emails. This includes the top-level
// metadata, the template, and the email account.
//
// This should only ever be used if you specifically want this lightweight
// context, since it returns a non-standard campaign object.
// ref: #1726
func GetCampaignMailContext(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&c).Error
	if err != nil {
		return c, err
	}
	err = db.Table("email_accounts").Where("id=?", c.EmailAccountId).Find(&c.EmailAccount).Error
	if err != nil {
		return c, err
	}
	err = db.Table("templates").Where("id=?", c.TemplateId).Find(&c.Template).Error
	if err != nil {
		return c, err
	}
	err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return c, err
	}
	return c, nil
}

// GetCampaign returns the campaign, if it exists, specified by the given id and user_id.
func GetCampaign(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&c).Error
	if err != nil {
		log.Errorf("%s: campaign not found", err)
		return c, err
	}
	err = c.getDetails()
	return c, err
}

// GetCampaignResults returns just the campaign results for the given campaign
func GetCampaignResults(id int64, uid int64) (CampaignResults, error) {
	cr := CampaignResults{}
	err := db.Table("campaigns").Where("id=? and user_id=?", id, uid).Find(&cr).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"campaign_id": id,
			"error":       err,
		}).Error(err)
		return cr, err
	}
	err = db.Table("results").Where("campaign_id=? and user_id=?", cr.Id, uid).Find(&cr.Results).Error
	if err != nil {
		log.Errorf("%s: results not found for campaign", err)
		return cr, err
	}
	err = db.Table("events").Where("campaign_id=?", cr.Id).Find(&cr.Events).Error
	if err != nil {
		log.Errorf("%s: events not found for campaign", err)
		return cr, err
	}
	return cr, err
}

// GetQueuedCampaigns returns the campaigns that are queued up for this given minute
func GetQueuedCampaigns(t time.Time) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Where("launch_date <= ?", t).
		Where("status = ?", CampaignQueued).Find(&cs).Error
	if err != nil {
		log.Error(err)
	}
	log.Infof("Found %d Campaigns to run\n", len(cs))
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// PostCampaign inserts a campaign and all associated records into the database.
func PostCampaign(c *Campaign, uid int64) error {
	// If EmailType is provided, look up the EmailAccount before validation
	if c.EmailType != "" && c.EmailAccount.Email == "" {
		ea, err := GetEmailAccountByType(c.EmailType)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"email_type": c.EmailType,
			}).Error("Email account with this type does not exist")
			return errors.New("Email account not found for type: " + c.EmailType)
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.EmailAccount = ea
		c.EmailAccountId = ea.Id
	}

	err := c.Validate()
	if err != nil {
		return err
	}
	// Fill in the details
	c.UserId = uid
	c.CreatedDate = time.Now().UTC()
	c.CompletedDate = time.Time{}
	c.Status = CampaignQueued
	if c.LaunchDate.IsZero() {
		c.LaunchDate = c.CreatedDate
	} else {
		c.LaunchDate = c.LaunchDate.UTC()
	}
	if !c.SendByDate.IsZero() {
		c.SendByDate = c.SendByDate.UTC()
	}
	if c.LaunchDate.Before(c.CreatedDate) || c.LaunchDate.Equal(c.CreatedDate) {
		c.Status = CampaignInProgress
	}
	// Check to make sure all the groups already exist
	// Also, later we'll need to know the total number of recipients (counting
	// duplicates is ok for now), so we'll do that here to save a loop.
	totalRecipients := 0
	for i, g := range c.Groups {
		c.Groups[i], err = GetGroupByName(g.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"group": g.Name,
			}).Error("Group does not exist")
			return ErrGroupNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		totalRecipients += len(c.Groups[i].Targets)
	}

	// Auto-calculate send-by date if not provided (rate limiting)
	// This ensures emails are spaced out safely to avoid spam filters and account lockouts
	if c.SendByDate.IsZero() && totalRecipients > 0 {
		c.SendByDate = CalculateMinimumSendByDate(c.LaunchDate, totalRecipients)
		log.Infof("Auto-calculated send-by date for campaign: %v (launch: %v, recipients: %d, interval: %v)",
			c.SendByDate, c.LaunchDate, totalRecipients, GetDefaultSendInterval())
	}

	// Check to make sure the template exists
	t, err := GetTemplateByName(c.Template.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"template": c.Template.Name,
		}).Error("Template does not exist")
		return ErrTemplateNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Template = t
	c.TemplateId = t.Id
	// Check to make sure the page exists
	p, err := GetPageByName(c.Page.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"page": c.Page.Name,
		}).Error("Page does not exist")
		return ErrPageNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Page = p
	c.PageId = p.Id
	// Check to make sure the email account exists
	// Note: Campaigns should reference EmailAccount by ID, Email, or EmailType
	if c.EmailAccountId == 0 && c.EmailAccount.Email != "" {
		// Look up by email address
		ea, err := GetEmailAccountByEmail(c.EmailAccount.Email)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"email": c.EmailAccount.Email,
			}).Error("Email account does not exist")
			return ErrEmailAccountNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.EmailAccount = ea
		c.EmailAccountId = ea.Id
	} else if c.EmailAccountId == 0 && c.EmailType != "" {
		// Look up by email type (for n8n integration)
		ea, err := GetEmailAccountByType(c.EmailType)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"email_type": c.EmailType,
			}).Error("Email account with this type does not exist")
			return ErrEmailAccountNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.EmailAccount = ea
		c.EmailAccountId = ea.Id
	}
	// Start transaction BEFORE saving campaign to ensure atomicity
	// If any error occurs during campaign/results creation, everything will be rolled back
	tx := db.Begin()

	// Insert campaign into the DB (using transaction)
	err = tx.Save(c).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}

	// Add "Campaign Created" event in the same transaction
	event := &Event{Message: "Campaign Created"}
	event.CampaignId = c.Id
	event.Time = time.Now().UTC()

	// Save event in transaction (don't fail entire operation if event save fails)
	err = tx.Save(event).Error
	if err != nil {
		log.Error(err)
		// Continue despite event save failure - this is non-critical
	}

	// Insert all the results (in same transaction)
	resultMap := make(map[string]bool)
	recipientIndex := 0
	for _, g := range c.Groups {
		// Insert a result for each target in the group
		for _, t := range g.Targets {
			// Remove duplicate results - we should only
			// send emails to unique email addresses.
			if _, ok := resultMap[t.Email]; ok {
				continue
			}
			resultMap[t.Email] = true
			sendDate := c.generateSendDate(recipientIndex, totalRecipients)
			r := &Result{
				BaseRecipient: BaseRecipient{
					Email:     t.Email,
					Position:  t.Position,
					FirstName: t.FirstName,
					LastName:  t.LastName,
				},
				Status:       StatusScheduled,
				CampaignId:   c.Id,
				UserId:       c.UserId,
				SendDate:     sendDate,
				Reported:     false,
				ModifiedDate: c.CreatedDate,
			}
			err = r.GenerateId(tx)
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
			processing := false
			if r.SendDate.Before(c.CreatedDate) || r.SendDate.Equal(c.CreatedDate) {
				r.Status = StatusSending
				processing = true
			}
			err = tx.Save(r).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"email": t.Email,
				}).Errorf("error creating result: %v", err)
				tx.Rollback()
				return err
			}
			c.Results = append(c.Results, *r)

			// Skip maillog creation for n8n campaigns (true batch sending)
			// n8n will handle scheduling via Wait nodes and send callbacks
			if !ShouldUseN8NBatchLaunch(c) {
				log.WithFields(logrus.Fields{
					"email":     r.Email,
					"send_date": sendDate,
				}).Debug("creating maillog")
				m := &MailLog{
					UserId:     c.UserId,
					CampaignId: c.Id,
					RId:        r.RId,
					SendDate:   sendDate,
					Processing: processing,
				}
				err = tx.Save(m).Error
				if err != nil {
					log.WithFields(logrus.Fields{
						"email": t.Email,
					}).Errorf("error creating maillog entry: %v", err)
					tx.Rollback()
					return err
				}
			}
			recipientIndex++
		}
	}

	// For n8n campaigns, launch the webhook BEFORE committing transaction
	// This ensures atomicity - if n8n fails, campaign is not created
	if ShouldUseN8NBatchLaunch(c) {
		log.Infof("Launching n8n batch campaign %d (before commit)", c.Id)
		err = LaunchN8NBatchCampaign(c)
		if err != nil {
			log.Errorf("Failed to launch n8n batch campaign %d: %v", c.Id, err)
			tx.Rollback() // Rollback everything if n8n webhook fails
			return fmt.Errorf("n8n webhook failed: %v", err)
		}
	}

	// Commit the transaction (only reached if n8n succeeded or not needed)
	err = tx.Commit().Error
	if err != nil {
		return err
	}

	// Send webhooks AFTER transaction commits (non-blocking, best-effort)
	// This avoids database deadlock from querying inside transaction
	whs, err := GetActiveWebhooks()
	if err == nil && len(whs) > 0 {
		whEndPoints := []webhook.EndPoint{}
		for _, wh := range whs {
			whEndPoints = append(whEndPoints, webhook.EndPoint{
				URL:    wh.URL,
				Secret: wh.Secret,
			})
		}
		webhook.SendAll(whEndPoints, event)
	}

	return nil
}

// DeleteCampaign deletes the specified campaign
func DeleteCampaign(id int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Deleting campaign")
	// Delete all the campaign results
	err := db.Where("campaign_id=?", id).Delete(&Result{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&Event{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Delete the campaign
	err = db.Delete(&Campaign{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// CompleteCampaign effectively "ends" a campaign.
// Any future emails clicked will return a simple "404" page.
func CompleteCampaign(id int64, uid int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Marking campaign as complete")
	c, err := GetCampaign(id, uid)
	if err != nil {
		return err
	}
	// Delete any maillogs still set to be sent out, preventing future emails
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Don't overwrite original completed time
	if c.Status == CampaignComplete {
		return nil
	}
	// Mark the campaign as complete
	c.CompletedDate = time.Now().UTC()
	c.Status = CampaignComplete
	err = db.Model(&Campaign{}).Where("id=? and user_id=?", id, uid).
		Select([]string{"completed_date", "status"}).UpdateColumns(&c).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// RateLimitWarning contains information about rate limiting warnings
type RateLimitWarning struct {
	IsAggressive         bool      `json:"is_aggressive"`
	ProvidedSendByDate   time.Time `json:"provided_send_by_date"`
	MinimumSendByDate    time.Time `json:"minimum_send_by_date"`
	ProvidedInterval     float64   `json:"provided_interval_seconds"`
	MinimumInterval      float64   `json:"minimum_interval_seconds"`
	TotalRecipients      int       `json:"total_recipients"`
	RecommendedDuration  string    `json:"recommended_duration"`
	WarningMessage       string    `json:"warning_message"`
}

// GetDefaultSendInterval returns the default interval between emails in seconds
// from environment variable DEFAULT_EMAIL_SEND_INTERVAL, defaulting to 120 seconds (2 minutes)
func GetDefaultSendInterval() time.Duration {
	intervalStr := os.Getenv("DEFAULT_EMAIL_SEND_INTERVAL")
	if intervalStr == "" {
		return 120 * time.Second // Default: 2 minutes
	}

	interval, err := strconv.ParseInt(intervalStr, 10, 64)
	if err != nil {
		log.Warnf("Invalid DEFAULT_EMAIL_SEND_INTERVAL value '%s', using default 120 seconds", intervalStr)
		return 120 * time.Second
	}

	if interval < 1 {
		log.Warnf("DEFAULT_EMAIL_SEND_INTERVAL too small (%d), using default 120 seconds", interval)
		return 120 * time.Second
	}

	return time.Duration(interval) * time.Second
}

// CalculateMinimumSendByDate calculates the minimum send-by date based on launch date and recipient count
func CalculateMinimumSendByDate(launchDate time.Time, recipientCount int) time.Time {
	interval := GetDefaultSendInterval()
	totalDuration := time.Duration(recipientCount) * interval
	return launchDate.Add(totalDuration)
}

// ValidateCampaignRateLimit checks if a campaign's send-by date is too aggressive
// Returns a RateLimitWarning with details if the rate is too fast
func ValidateCampaignRateLimit(launchDate, sendByDate time.Time, recipientCount int) *RateLimitWarning {
	if recipientCount == 0 {
		return nil // No recipients, no warning needed
	}

	minimumInterval := GetDefaultSendInterval()
	minimumSendByDate := CalculateMinimumSendByDate(launchDate, recipientCount)

	// If send-by date is zero (not provided), it's not aggressive - will be auto-set
	if sendByDate.IsZero() {
		return nil
	}

	// If send-by date is after minimum, it's safe
	if sendByDate.After(minimumSendByDate) || sendByDate.Equal(minimumSendByDate) {
		return nil
	}

	// Calculate provided interval
	duration := sendByDate.Sub(launchDate)
	providedInterval := duration.Seconds() / float64(recipientCount)

	// Build warning message
	warningMsg := fmt.Sprintf(
		"Your campaign will send emails too quickly (%.1f seconds per recipient). "+
			"This may trigger spam filters and lock your email account. "+
			"Microsoft 365 allows 30 emails/minute but sending too fast looks suspicious. "+
			"We recommend spacing emails by %.0f seconds (%.1f minutes) per recipient.",
		providedInterval,
		minimumInterval.Seconds(),
		minimumInterval.Minutes(),
	)

	// Format recommended duration
	recommendedDuration := formatDuration(minimumSendByDate.Sub(launchDate))

	return &RateLimitWarning{
		IsAggressive:        true,
		ProvidedSendByDate:  sendByDate,
		MinimumSendByDate:   minimumSendByDate,
		ProvidedInterval:    providedInterval,
		MinimumInterval:     minimumInterval.Seconds(),
		TotalRecipients:     recipientCount,
		RecommendedDuration: recommendedDuration,
		WarningMessage:      warningMsg,
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%d hours %d minutes", hours, minutes)
		}
		return fmt.Sprintf("%d hours", hours)
	}

	if minutes > 0 {
		return fmt.Sprintf("%d minutes", minutes)
	}

	seconds := int(d.Seconds())
	return fmt.Sprintf("%d seconds", seconds)
}
