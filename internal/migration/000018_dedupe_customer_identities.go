package migration

import (
	"strings"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(18, "deduplicate external customer identities and enforce external identity uniqueness", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return ensureUniqueCustomerIdentitySchema(ctx.Tx)
		})
	})
}

func ensureUniqueCustomerIdentitySchema(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.CustomerIdentity{}) {
		return nil
	}
	var identities []models.CustomerIdentity
	if err := db.Order("id ASC").Find(&identities).Error; err != nil {
		return err
	}
	canonicalCustomers := make(map[string]int64, len(identities))
	for i := range identities {
		identity := &identities[i]
		key := strings.TrimSpace(string(identity.ExternalSource)) + ":" + strings.TrimSpace(identity.ExternalID)
		if canonicalCustomerID, exists := canonicalCustomers[key]; exists {
			if err := reassignDuplicateCustomerReferences(db, identity.CustomerID, canonicalCustomerID); err != nil {
				return err
			}
			if err := db.Delete(&models.CustomerIdentity{}, identity.ID).Error; err != nil {
				return err
			}
			continue
		}
		canonicalCustomers[key] = identity.CustomerID
	}

	if db.Migrator().HasIndex(&models.CustomerIdentity{}, "uk_customer_external") {
		if err := db.Migrator().DropIndex(&models.CustomerIdentity{}, "uk_customer_external"); err != nil {
			return err
		}
	}
	return db.Migrator().CreateIndex(&models.CustomerIdentity{}, "uk_customer_external")
}

func reassignDuplicateCustomerReferences(db *gorm.DB, duplicateCustomerID, canonicalCustomerID int64) error {
	if duplicateCustomerID <= 0 || canonicalCustomerID <= 0 || duplicateCustomerID == canonicalCustomerID {
		return nil
	}
	for _, model := range []any{
		&models.Conversation{},
		&models.Ticket{},
	} {
		if !db.Migrator().HasTable(model) {
			continue
		}
		if err := db.Model(model).
			Where("customer_id = ?", duplicateCustomerID).
			Update("customer_id", canonicalCustomerID).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(&models.CustomerContact{}) {
		return nil
	}
	var contacts []models.CustomerContact
	if err := db.Where("customer_id = ?", duplicateCustomerID).Find(&contacts).Error; err != nil {
		return err
	}
	for i := range contacts {
		contact := &contacts[i]
		var count int64
		if err := db.Model(&models.CustomerContact{}).
			Where("customer_id = ? AND contact_type = ? AND contact_value = ?", canonicalCustomerID, contact.ContactType, contact.ContactValue).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			if err := db.Delete(&models.CustomerContact{}, contact.ID).Error; err != nil {
				return err
			}
			continue
		}
		if err := db.Model(&models.CustomerContact{}).Where("id = ?", contact.ID).Update("customer_id", canonicalCustomerID).Error; err != nil {
			return err
		}
	}
	return nil
}
