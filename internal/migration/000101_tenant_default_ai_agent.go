package migration

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(101, "provision tenant default AI diagnosis agents", func() error {
		return ensureTenantDefaultAIAgents(sqls.DB())
	})
}

func ensureTenantDefaultAIAgents(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.AIAgent{}) {
		return nil
	}
	var tenants []models.Tenant
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		if !tenants[i].IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   tenants[i].ID,
			Username:   "system",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if _, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenants[i].ID, operator); err != nil {
			return err
		}
	}
	return nil
}
