package models

import (
	"errors"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
)

// EmailType represents a configurable email account type
type EmailType struct {
	Id          int64     `json:"id" gorm:"column:id; primary_key:yes"`
	Value       string    `json:"value" gorm:"column:value; unique; not null"`
	DisplayName string    `json:"display_name" gorm:"column:display_name; not null"`
	Description string    `json:"description" gorm:"column:description"`
	IsActive    bool      `json:"is_active" gorm:"column:is_active; default:true"`
	SortOrder   int       `json:"sort_order" gorm:"column:sort_order; default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the table name for EmailType
func (et *EmailType) TableName() string {
	return "email_types"
}

// Validate ensures the email type has required fields
func (et *EmailType) Validate() error {
	if et.Value == "" {
		return errors.New("type value is required")
	}
	if et.DisplayName == "" {
		return errors.New("display name is required")
	}
	return nil
}

// GetEmailTypes returns all email types from the database
func GetEmailTypes() ([]EmailType, error) {
	types := []EmailType{}
	err := db.Where("is_active = ?", true).Order("sort_order ASC, display_name ASC").Find(&types).Error
	return types, err
}

// GetAllEmailTypes returns all email types (including inactive)
func GetAllEmailTypes() ([]EmailType, error) {
	types := []EmailType{}
	err := db.Order("sort_order ASC, display_name ASC").Find(&types).Error
	return types, err
}

// GetEmailType returns an email type by ID
func GetEmailType(id int64) (EmailType, error) {
	emailType := EmailType{}
	err := db.Where("id = ?", id).First(&emailType).Error
	return emailType, err
}

// GetEmailTypeByValue returns an email type by its value
func GetEmailTypeByValue(value string) (EmailType, error) {
	emailType := EmailType{}
	err := db.Where("value = ?", value).First(&emailType).Error
	return emailType, err
}

// ValidateEmailType checks if a type value exists and is active
func ValidateEmailType(typeValue string) error {
	emailType, err := GetEmailTypeByValue(typeValue)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("invalid email type")
		}
		return err
	}
	if !emailType.IsActive {
		return errors.New("email type is not active")
	}
	return nil
}

// PostEmailType creates a new email type in the database
func PostEmailType(emailType *EmailType) error {
	// Validate the type
	if err := emailType.Validate(); err != nil {
		return err
	}

	// Check if value already exists
	temp := EmailType{}
	err := db.Where("value = ?", emailType.Value).First(&temp).Error
	if err == nil {
		return errors.New("email type with this value already exists")
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Set timestamps
	emailType.CreatedAt = time.Now().UTC()
	emailType.UpdatedAt = time.Now().UTC()

	// Create the type
	err = db.Create(emailType).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// PutEmailType updates an existing email type
func PutEmailType(emailType *EmailType) error {
	// Validate the type
	if err := emailType.Validate(); err != nil {
		return err
	}

	// Check if type exists
	temp := EmailType{}
	err := db.Where("id = ?", emailType.Id).First(&temp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("email type not found")
		}
		return err
	}

	// Check if value is being changed and conflicts with another type
	if emailType.Value != temp.Value {
		conflict := EmailType{}
		err = db.Where("value = ? AND id != ?", emailType.Value, emailType.Id).First(&conflict).Error
		if err == nil {
			return errors.New("value already in use by another type")
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
	}

	// Update timestamp
	emailType.UpdatedAt = time.Now().UTC()

	// Update the type
	err = db.Save(emailType).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// DeleteEmailType deletes an email type from the database
func DeleteEmailType(id int64) error {
	// Check if type exists
	emailType := EmailType{}
	err := db.Where("id = ?", id).First(&emailType).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("email type not found")
		}
		return err
	}

	// Check if any email accounts are using this type
	var count int
	err = db.Model(&EmailAccount{}).Where("type = ?", emailType.Value).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("cannot delete type: it is being used by email accounts")
	}

	// Delete the type
	err = db.Delete(&emailType).Error
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}
