package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var CustomerPrivacyConsentRepository = &customerPrivacyConsentRepository{}

type customerPrivacyConsentRepository struct{}

func (r *customerPrivacyConsentRepository) Create(db *gorm.DB, item *models.CustomerPrivacyConsent) error {
	return db.Create(item).Error
}

func (r *customerPrivacyConsentRepository) FindLatestAccepted(db *gorm.DB, entrySessionID int64, policyVersion string) *models.CustomerPrivacyConsent {
	item := &models.CustomerPrivacyConsent{}
	if err := db.Where(
		"entry_session_id = ? AND policy_version = ? AND required_accepted = ?",
		entrySessionID,
		policyVersion,
		true,
	).Order("consented_at DESC, id DESC").First(item).Error; err != nil {
		return nil
	}
	return item
}
