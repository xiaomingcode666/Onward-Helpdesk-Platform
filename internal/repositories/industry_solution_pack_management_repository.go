package repositories

import (
	"errors"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IndustrySolutionPackListFilter struct {
	Status       string
	IndustryCode string
	Page         int
	PageSize     int
}

func (r *industrySolutionPackRepository) ListPacks(db *gorm.DB, tenantID int64, filter IndustrySolutionPackListFilter) ([]models.IndustrySolutionPack, int64, error) {
	query := db.Model(&models.IndustrySolutionPack{}).Where("tenant_id = ?", tenantID)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.IndustryCode != "" {
		query = query.Where("industry_code = ?", filter.IndustryCode)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.IndustrySolutionPack
	err := query.Order("updated_at DESC, id DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&items).Error
	return items, total, err
}

func (r *industrySolutionPackRepository) FindPackByCodeVersion(db *gorm.DB, tenantID int64, packCode, version string) (*models.IndustrySolutionPack, error) {
	item := &models.IndustrySolutionPack{}
	err := db.Where("tenant_id = ? AND pack_code = ? AND version = ?", tenantID, packCode, version).First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) LockPack(db *gorm.DB, tenantID, packID int64) (*models.IndustrySolutionPack, error) {
	item := &models.IndustrySolutionPack{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, packID).First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) CreatePack(db *gorm.DB, item *models.IndustrySolutionPack) error {
	return db.Create(item).Error
}

func (r *industrySolutionPackRepository) UpdateDraftPack(db *gorm.DB, tenantID, packID int64, updates map[string]any) error {
	result := db.Model(&models.IndustrySolutionPack{}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, packID, models.IndustrySolutionPackStatusDraft).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *industrySolutionPackRepository) ReplaceDraftResources(db *gorm.DB, tenantID, packID int64, resources []models.IndustrySolutionPackResource) error {
	if err := db.Where("tenant_id = ? AND pack_id = ?", tenantID, packID).
		Delete(&models.IndustrySolutionPackResource{}).Error; err != nil {
		return err
	}
	if len(resources) == 0 {
		return nil
	}
	return db.Create(&resources).Error
}

func (r *industrySolutionPackRepository) CountApplicationsForPack(db *gorm.DB, tenantID, packID int64) (int64, error) {
	var count int64
	err := db.Model(&models.IndustrySolutionPackApplication{}).
		Where("tenant_id = ? AND pack_id = ?", tenantID, packID).Count(&count).Error
	return count, err
}

func (r *industrySolutionPackRepository) PublishPack(db *gorm.DB, tenantID, packID int64, manifestHash string, operatorID int64, operatorName string, now time.Time) error {
	if err := db.Model(&models.IndustrySolutionPackResource{}).
		Where("tenant_id = ? AND pack_id = ?", tenantID, packID).
		Updates(map[string]any{
			"status": models.IndustrySolutionResourceStatusReady, "update_user_id": operatorID,
			"update_user_name": operatorName, "updated_at": now,
		}).Error; err != nil {
		return err
	}
	result := db.Model(&models.IndustrySolutionPack{}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, packID, models.IndustrySolutionPackStatusDraft).
		Updates(map[string]any{
			"status": models.IndustrySolutionPackStatusPublished, "manifest_hash": manifestHash,
			"published_at": now, "published_by_id": operatorID, "published_by_name": operatorName,
			"update_user_id": operatorID, "update_user_name": operatorName, "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *industrySolutionPackRepository) FindDiagnosisEvalCase(db *gorm.DB, tenantID int64, caseCode string, productID, productModelID int64) (*models.DiagnosisEvalCase, error) {
	item := &models.DiagnosisEvalCase{}
	err := db.Where("tenant_id = ? AND case_code = ? AND product_id = ? AND product_model_id = ?", tenantID, caseCode, productID, productModelID).
		Order("updated_at DESC, id DESC").First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) FindDiagnosisEvalCaseVersion(db *gorm.DB, tenantID int64, caseCode, version string) (*models.DiagnosisEvalCase, error) {
	item := &models.DiagnosisEvalCase{}
	err := db.Where("tenant_id = ? AND case_code = ? AND version = ?", tenantID, caseCode, version).First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) CreateDiagnosisEvalCase(db *gorm.DB, item *models.DiagnosisEvalCase) error {
	return db.Create(item).Error
}

func (r *industrySolutionPackRepository) UpdateDiagnosisEvalCase(db *gorm.DB, tenantID, id int64, updates map[string]any) error {
	result := db.Model(&models.DiagnosisEvalCase{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *industrySolutionPackRepository) FindARWorkInstruction(db *gorm.DB, tenantID int64, instructionCode string, productID, productModelID int64) (*models.ARWorkInstruction, error) {
	item := &models.ARWorkInstruction{}
	err := db.Where("tenant_id = ? AND instruction_code = ? AND product_id = ? AND product_model_id = ?", tenantID, instructionCode, productID, productModelID).
		Order("updated_at DESC, id DESC").First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) FindARWorkInstructionVersion(db *gorm.DB, tenantID int64, instructionCode, version string) (*models.ARWorkInstruction, error) {
	item := &models.ARWorkInstruction{}
	err := db.Where("tenant_id = ? AND instruction_code = ? AND version = ?", tenantID, instructionCode, version).First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *industrySolutionPackRepository) ListARWorkSteps(db *gorm.DB, tenantID, instructionID int64) ([]models.ARWorkStep, error) {
	var items []models.ARWorkStep
	err := db.Where("tenant_id = ? AND instruction_id = ?", tenantID, instructionID).
		Order("sequence_no ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *industrySolutionPackRepository) CreateARWorkInstructionWithSteps(db *gorm.DB, item *models.ARWorkInstruction, steps []models.ARWorkStep) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		for i := range steps {
			steps[i].InstructionID = item.ID
		}
		if len(steps) > 0 {
			return tx.Create(&steps).Error
		}
		return nil
	})
}

func (r *industrySolutionPackRepository) ReplaceARWorkInstruction(db *gorm.DB, tenantID, instructionID int64, updates map[string]any, steps []models.ARWorkStep) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.ARWorkInstruction{}).
			Where("tenant_id = ? AND id = ?", tenantID, instructionID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("tenant_id = ? AND instruction_id = ?", tenantID, instructionID).Delete(&models.ARWorkStep{}).Error; err != nil {
			return err
		}
		for i := range steps {
			steps[i].InstructionID = instructionID
		}
		if len(steps) > 0 {
			return tx.Create(&steps).Error
		}
		return nil
	})
}

func IsIndustrySolutionPackNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
