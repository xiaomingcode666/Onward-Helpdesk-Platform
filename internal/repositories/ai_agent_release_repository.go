package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AIAgentReleaseRepository = &aiAgentReleaseRepository{}

type aiAgentReleaseRepository struct{}

func (r *aiAgentReleaseRepository) Get(db *gorm.DB, id int64) *models.AIAgentRelease {
	if db == nil || id <= 0 {
		return nil
	}
	ret := &models.AIAgentRelease{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiAgentReleaseRepository) GetForUpdate(db *gorm.DB, id int64) *models.AIAgentRelease {
	if db == nil || id <= 0 {
		return nil
	}
	ret := &models.AIAgentRelease{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiAgentReleaseRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.AIAgentRelease) {
	if db == nil || cnd == nil {
		return []models.AIAgentRelease{}
	}
	cnd.Find(db, &list)
	return
}

func (r *aiAgentReleaseRepository) ExistsActiveByWorkflowID(db *gorm.DB, workflowID int64) bool {
	if db == nil || workflowID <= 0 {
		return false
	}
	var count int64
	db.Model(&models.AIAgentRelease{}).
		Where("workflow_id = ? AND deployment_status = ? AND status <> ?", workflowID, models.AIAgentReleaseDeploymentActive, enums.StatusDeleted).
		Limit(1).
		Count(&count)
	return count > 0
}

func (r *aiAgentReleaseRepository) Create(db *gorm.DB, item *models.AIAgentRelease) error {
	return db.Create(item).Error
}

func (r *aiAgentReleaseRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.AIAgentRelease{}).Where("id = ?", id).Updates(columns).Error
}

func (r *aiAgentReleaseRepository) ConditionalUpdates(db *gorm.DB, id int64, currentReviewStatus string, columns map[string]any) (bool, error) {
	result := db.Model(&models.AIAgentRelease{}).
		Where("id = ? AND review_status = ?", id, currentReviewStatus).
		Updates(columns)
	return result.RowsAffected == 1, result.Error
}

func (r *aiAgentReleaseRepository) MaxReleaseNo(db *gorm.DB, agentID int64) int {
	var value int
	db.Model(&models.AIAgentRelease{}).
		Where("agent_id = ?", agentID).
		Select("COALESCE(MAX(release_no), 0)").
		Scan(&value)
	return value
}
