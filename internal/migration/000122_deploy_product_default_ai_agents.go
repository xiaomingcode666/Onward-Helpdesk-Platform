package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(122, "deploy system-created product AI agents by default", func() error {
		return deployProductDefaultAIAgents(sqls.DB())
	})
}

func deployProductDefaultAIAgents(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.AIAgent{}) ||
		!db.Migrator().HasTable(&models.AIAgentRelease{}) ||
		!db.Migrator().HasTable(&models.Product{}) ||
		!db.Migrator().HasTable(&models.ProductServiceProfile{}) {
		return nil
	}

	var agents []models.AIAgent
	if err := db.Where("source = ? AND product_id > 0 AND active_release_id = 0 AND status <> ?", "product_auto", enums.StatusDeleted).
		Order("id ASC").Find(&agents).Error; err != nil {
		return err
	}
	for i := range agents {
		agent := &agents[i]
		product := repositories.ProductRepository.GetByTenant(db, agent.ProductID, agent.TenantID)
		if product == nil || product.Status != enums.StatusOk {
			continue
		}
		tenant := repositories.TenantRepository.Get(db, agent.TenantID)
		if tenant == nil || tenant.Status == enums.StatusDeleted || !tenant.IsAIEnabled() {
			continue
		}
		operator := &dto.AuthPrincipal{
			TenantID:   agent.TenantID,
			Username:   "migration-122",
			DomainType: models.DomainTypeEnterprise,
			Domain:     models.DomainTypeEnterprise,
		}
		if _, err := services.ProductAIAgentService.EnsureProductCustomerAgent(agent.TenantID, agent.ProductID, operator); err != nil {
			return fmt.Errorf("deploy product %d default AI agent %d: %w", agent.ProductID, agent.ID, err)
		}
	}
	return nil
}
