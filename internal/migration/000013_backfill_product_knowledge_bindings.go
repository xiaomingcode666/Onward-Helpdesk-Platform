package migration

import (
	"errors"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(13, "backfill default product knowledge bindings", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			var profiles []models.ProductServiceProfile
			if err := ctx.Tx.
				Where("default_knowledge_base_id > 0 AND status <> ?", enums.StatusDeleted).
				Find(&profiles).Error; err != nil {
				return err
			}
			for i := range profiles {
				profile := profiles[i]
				var product models.Product
				if err := ctx.Tx.First(&product, "id = ? AND tenant_id = ? AND status <> ?", profile.ProductID, profile.TenantID, enums.StatusDeleted).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}
				var knowledgeBase models.KnowledgeBase
				if err := ctx.Tx.First(&knowledgeBase, "id = ? AND tenant_id = ? AND status <> ?", profile.DefaultKnowledgeBaseID, profile.TenantID, enums.StatusDeleted).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}
				if _, err := services.ProductKnowledgeBindingService.EnsureDefaultBindingTx(
					ctx.Tx,
					profile.TenantID,
					profile.ProductID,
					profile.DefaultKnowledgeBaseID,
					nil,
				); err != nil {
					return err
				}
			}
			return nil
		})
	})
}
