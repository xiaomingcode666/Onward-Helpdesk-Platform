package migration

import (
	"errors"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(125, "initialize customer default locale from tenant locale", func() error {
		return initializeCustomerDefaultLocale(sqls.DB())
	})
}

func initializeCustomerDefaultLocale(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.TenantBranding{}) {
		return nil
	}
	var brandings []models.TenantBranding
	if err := db.Where("status <> ?", enums.StatusDeleted).Find(&brandings).Error; err != nil {
		return err
	}
	for i := range brandings {
		tenant := &models.Tenant{}
		if err := db.First(tenant, "id = ?", brandings[i].TenantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		defaultLocale := strings.TrimSpace(tenant.DefaultLocale)
		if defaultLocale == "" {
			defaultLocale = "en-US"
		}
		if err := db.Model(&models.TenantBranding{}).
			Where("id = ?", brandings[i].ID).
			UpdateColumn("default_locale", defaultLocale).Error; err != nil {
			return err
		}
	}
	return nil
}
