package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeIndexGenerationRepository = newKnowledgeIndexGenerationRepository()

func newKnowledgeIndexGenerationRepository() *knowledgeIndexGenerationRepository {
	return &knowledgeIndexGenerationRepository{}
}

type knowledgeIndexGenerationRepository struct{}

func (r *knowledgeIndexGenerationRepository) Get(db *gorm.DB, id int64) *models.KnowledgeIndexGeneration {
	ret := &models.KnowledgeIndexGeneration{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeIndexGenerationRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeIndexGeneration) {
	cnd.Find(db, &list)
	return
}

func (r *knowledgeIndexGenerationRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.KnowledgeIndexGeneration {
	ret := &models.KnowledgeIndexGeneration{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeIndexGenerationRepository) Create(db *gorm.DB, item *models.KnowledgeIndexGeneration) error {
	return db.Create(item).Error
}

func (r *knowledgeIndexGenerationRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.KnowledgeIndexGeneration{}).Where("id = ?", id).Updates(columns).Error
}

func (r *knowledgeIndexGenerationRepository) FindActiveByAlias(db *gorm.DB, alias string) *models.KnowledgeIndexGeneration {
	ret := &models.KnowledgeIndexGeneration{}
	if err := db.Where("collection_alias = ? AND status = ?", alias, "active").
		Order("id DESC").
		First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeIndexGenerationRepository) FindLatestByAlias(db *gorm.DB, alias string) *models.KnowledgeIndexGeneration {
	ret := &models.KnowledgeIndexGeneration{}
	if err := db.Where("collection_alias = ?", alias).
		Order("id DESC").
		First(ret).Error; err != nil {
		return nil
	}
	return ret
}
