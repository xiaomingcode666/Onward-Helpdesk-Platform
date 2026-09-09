package migration

import (
	"errors"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(55, "support product repair team dispatch members", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := ensureProductRepairDispatchMemberSchema(ctx.Tx); err != nil {
				return err
			}
			return backfillAgentTeamMembersFromProfiles(ctx.Tx)
		})
	})
}

func ensureProductRepairDispatchMemberSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&models.AgentTeam{}) {
		if err := migrator.CreateTable(&models.AgentTeam{}); err != nil {
			return err
		}
	}
	if !migrator.HasColumn(&models.AgentTeam{}, "AssignmentMode") {
		if err := migrator.AddColumn(&models.AgentTeam{}, "AssignmentMode"); err != nil {
			return err
		}
	}
	if err := db.Model(&models.AgentTeam{}).
		Where("assignment_mode = '' OR assignment_mode IS NULL").
		Update("assignment_mode", services.AgentTeamAssignmentModeBalanced).Error; err != nil {
		return err
	}
	if !migrator.HasTable(&models.AgentTeamMember{}) {
		if err := migrator.CreateTable(&models.AgentTeamMember{}); err != nil {
			return err
		}
	}
	return nil
}

func backfillAgentTeamMembersFromProfiles(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AgentProfile{}) || !db.Migrator().HasTable(&models.AgentTeamMember{}) {
		return nil
	}
	profiles := make([]models.AgentProfile, 0)
	if err := db.Where("team_id > 0 AND user_id > 0 AND status = ?", enums.StatusOk).
		Order("tenant_id ASC, team_id ASC, user_id ASC").
		Find(&profiles).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range profiles {
		tenantID := profiles[i].TenantID
		if tenantID <= 0 {
			team := &models.AgentTeam{}
			if err := db.First(team, "id = ?", profiles[i].TeamID).Error; err == nil {
				tenantID = team.TenantID
			}
		}
		if tenantID <= 0 {
			continue
		}
		existing := &models.AgentTeamMember{}
		err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ?", tenantID, profiles[i].TeamID, profiles[i].UserID).
			First(existing).Error
		if err == nil {
			if err := db.Model(&models.AgentTeamMember{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"dispatch_enabled": profiles[i].AutoAssignEnabled,
				"dispatch_weight":  1,
				"status":           enums.StatusOk,
				"updated_at":       now,
			}).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Create(&models.AgentTeamMember{
			TenantID:        tenantID,
			TeamID:          profiles[i].TeamID,
			UserID:          profiles[i].UserID,
			DispatchEnabled: profiles[i].AutoAssignEnabled,
			DispatchWeight:  1,
			Status:          enums.StatusOk,
			AuditFields: models.AuditFields{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
