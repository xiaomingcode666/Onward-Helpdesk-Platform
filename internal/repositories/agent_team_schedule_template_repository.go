package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AgentTeamScheduleTemplateRepository = newAgentTeamScheduleTemplateRepository()

func newAgentTeamScheduleTemplateRepository() *agentTeamScheduleTemplateRepository {
	return &agentTeamScheduleTemplateRepository{}
}

type agentTeamScheduleTemplateRepository struct{}

func (r *agentTeamScheduleTemplateRepository) FindByTenant(db *gorm.DB, tenantID int64) *models.AgentTeamScheduleTemplate {
	if db == nil || tenantID <= 0 {
		return nil
	}
	item := &models.AgentTeamScheduleTemplate{}
	if err := db.Where("tenant_id = ?", tenantID).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *agentTeamScheduleTemplateRepository) Upsert(db *gorm.DB, item *models.AgentTeamScheduleTemplate) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"workdays", "start_minute", "end_minute", "timezone", "status",
			"update_user_id", "update_user_name", "updated_at",
		}),
	}).Create(item).Error
}
