package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AgentWorkStatusRepository = newAgentWorkStatusRepository()

type agentWorkStatusRepository struct{}

func newAgentWorkStatusRepository() *agentWorkStatusRepository {
	return &agentWorkStatusRepository{}
}

func (r *agentWorkStatusRepository) GetByTenantAndUser(db *gorm.DB, tenantID, userID int64) *models.AgentWorkStatus {
	if db == nil || tenantID < 0 || userID <= 0 || !db.Migrator().HasTable(&models.AgentWorkStatus{}) {
		return nil
	}
	var item models.AgentWorkStatus
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *agentWorkStatusRepository) FindByTenantAndUserIDs(db *gorm.DB, tenantID int64, userIDs []int64) []models.AgentWorkStatus {
	if db == nil || tenantID < 0 || len(userIDs) == 0 || !db.Migrator().HasTable(&models.AgentWorkStatus{}) {
		return []models.AgentWorkStatus{}
	}
	items := make([]models.AgentWorkStatus, 0)
	db.Where("tenant_id = ? AND user_id IN ?", tenantID, userIDs).Find(&items)
	return items
}

func (r *agentWorkStatusRepository) Save(db *gorm.DB, item *models.AgentWorkStatus) error {
	if item.ID > 0 {
		return db.Save(item).Error
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "note", "available_at", "confirmed_at", "status_changed_at", "updated_at", "update_user_id", "update_user_name"}),
	}).Create(item).Error
}

func (r *agentWorkStatusRepository) GetForUpdate(db *gorm.DB, tenantID, userID int64) (*models.AgentWorkStatus, error) {
	var item models.AgentWorkStatus
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&item).Error
	return &item, err
}
