package repositories

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var KnowledgeAccessGrantRepository = newKnowledgeAccessGrantRepository()

type knowledgeAccessGrantRepository struct{}

func newKnowledgeAccessGrantRepository() *knowledgeAccessGrantRepository {
	return &knowledgeAccessGrantRepository{}
}

func (r *knowledgeAccessGrantRepository) FindByKnowledgeBase(db *gorm.DB, tenantID, knowledgeBaseID int64) ([]models.KnowledgeAccessGrant, error) {
	items := []models.KnowledgeAccessGrant{}
	if db == nil || tenantID <= 0 || knowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeAccessGrant{}) {
		return items, nil
	}
	err := db.Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, knowledgeBaseID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *knowledgeAccessGrantRepository) HasActiveGrant(
	db *gorm.DB,
	tenantID, knowledgeBaseID, assigneeID, teamID int64,
	now time.Time,
) (bool, error) {
	if db == nil || tenantID <= 0 || knowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeAccessGrant{}) {
		return false, nil
	}
	query := db.Model(&models.KnowledgeAccessGrant{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND status = ?", tenantID, knowledgeBaseID, enums.StatusOk).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_until IS NULL OR effective_until >= ?)", now)
	switch {
	case assigneeID > 0 && teamID > 0:
		query = query.Where(
			"(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id = ?)",
			"user", assigneeID, "team", teamID,
		)
	case assigneeID > 0:
		query = query.Where("subject_type = ? AND subject_id = ?", "user", assigneeID)
	case teamID > 0:
		query = query.Where("subject_type = ? AND subject_id = ?", "team", teamID)
	default:
		return false, nil
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *knowledgeAccessGrantRepository) ReplaceForKnowledgeBase(
	db *gorm.DB,
	tenantID, knowledgeBaseID int64,
	items []models.KnowledgeAccessGrant,
) error {
	if db == nil || tenantID <= 0 || knowledgeBaseID <= 0 || !db.Migrator().HasTable(&models.KnowledgeAccessGrant{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, knowledgeBaseID).
			Delete(&models.KnowledgeAccessGrant{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}
