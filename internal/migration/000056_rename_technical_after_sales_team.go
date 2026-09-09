package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	legacyTechnicalRepairTeamName = "技术维修组"
	legacyTechnicalRepairTeamPath = "/" + legacyTechnicalRepairTeamName
)

func init() {
	register(56, "rename default technical after-sales organization", func() error {
		return normalizeTechnicalAfterSalesOrganizationNames(sqls.DB())
	})
}

func normalizeTechnicalAfterSalesOrganizationNames(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	migrator := db.Migrator()

	if migrator.HasTable(&models.AgentTeam{}) {
		if err := db.Model(&models.AgentTeam{}).
			Where("(team_type = ? OR system_key = ?) AND status <> ?", services.AgentTeamTypeTechnicalRepair, "technical-repair", enums.StatusDeleted).
			Updates(map[string]any{
				"name":             services.TechnicalAfterSalesTeamName,
				"updated_at":       now,
				"update_user_name": "migration-56",
			}).Error; err != nil {
			return err
		}
	}

	if !migrator.HasTable(&models.Department{}) {
		return nil
	}
	if err := db.Model(&models.Department{}).
		Where("department_code = ? AND status <> ?", "technical-repair", enums.StatusDeleted).
		Updates(map[string]any{
			"name":             services.TechnicalAfterSalesTeamName,
			"path":             services.TechnicalAfterSalesTeamPath,
			"updated_at":       now,
			"update_user_name": "migration-56",
		}).Error; err != nil {
		return err
	}
	if err := db.Model(&models.Department{}).
		Where("path LIKE ? AND status <> ?", legacyTechnicalRepairTeamPath+"/%", enums.StatusDeleted).
		Updates(map[string]any{
			"path":             gorm.Expr("replace(path, ?, ?)", legacyTechnicalRepairTeamPath+"/", services.TechnicalAfterSalesTeamPath+"/"),
			"updated_at":       now,
			"update_user_name": "migration-56",
		}).Error; err != nil {
		return err
	}
	return nil
}
