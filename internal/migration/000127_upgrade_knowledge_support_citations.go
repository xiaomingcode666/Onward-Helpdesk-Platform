package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(127, "upgrade knowledge support replies with source citations", func() error {
		return upgradeKnowledgeSupportCitations(sqls.DB())
	})
}

func upgradeKnowledgeSupportCitations(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Tenant{}) || !db.Migrator().HasTable(&models.AIAgentRelease{}) {
		return nil
	}
	if err := services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db); err != nil {
		return fmt.Errorf("materialize citation-enabled knowledge workflow: %w", err)
	}
	var tenants []models.Tenant
	if err := db.Where("service_scene = ? AND status <> ?", models.TenantServiceSceneKnowledgeSupport, enums.StatusDeleted).
		Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		if !tenants[i].IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   tenants[i].ID,
			Username:   "migration-127",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if _, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenants[i].ID, operator); err != nil {
			return fmt.Errorf("upgrade tenant %d knowledge citations: %w", tenants[i].ID, err)
		}
	}
	return nil
}
