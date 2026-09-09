package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(128, "correct ODT portal brand name", func() error {
		return correctODTPortalBrandName(sqls.DB())
	})
}

func correctODTPortalBrandName(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.TenantBranding{}) {
		return nil
	}
	var brandings []models.TenantBranding
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("tenant_id ASC").Find(&brandings).Error; err != nil {
		return err
	}
	for i := range brandings {
		if !isODTDomain(brandings[i].CustomDomain) || brandings[i].BrandName == "ODT" {
			continue
		}
		if err := db.Model(&models.TenantBranding{}).
			Where("id = ?", brandings[i].ID).
			Updates(map[string]any{
				"brand_name":       "ODT",
				"update_user_name": "migration-128",
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
	}
	return nil
}
