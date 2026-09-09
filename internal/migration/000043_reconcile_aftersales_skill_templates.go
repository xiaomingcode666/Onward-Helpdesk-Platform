package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(43, "reconcile after-sales skill responsibilities and product bindings", func() error {
		return reconcileAfterSalesSkillTemplates(sqls.DB())
	})
}

func reconcileAfterSalesSkillTemplates(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	for _, template := range afterSalesSkillTemplates() {
		if err := db.Model(&models.SkillDefinition{}).
			Where("tenant_id = ? AND remark = ?", 0, builtinSkillTemplateRemarkPrefix+template.Code).
			Updates(map[string]any{
				"name":             template.Name,
				"description":      template.Description,
				"instruction":      template.Instruction,
				"examples":         template.Examples,
				"tool_whitelist":   template.ToolWhitelist,
				"updated_at":       now,
				"update_user_name": "system",
			}).Error; err != nil {
			return err
		}
	}
	return services.ProductAIAgentService.BindDefaultProductAgentSkills(db)
}
