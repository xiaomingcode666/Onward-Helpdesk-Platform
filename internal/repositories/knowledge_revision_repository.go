package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeRevisionRepository = newKnowledgeRevisionRepository()

func newKnowledgeRevisionRepository() *knowledgeRevisionRepository {
	return &knowledgeRevisionRepository{}
}

type knowledgeRevisionRepository struct{}

func (r *knowledgeRevisionRepository) Get(db *gorm.DB, id int64) *models.KnowledgeRevision {
	ret := &models.KnowledgeRevision{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeRevisionRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeRevision) {
	cnd.Find(db, &list)
	return
}

func (r *knowledgeRevisionRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.KnowledgeRevision {
	ret := &models.KnowledgeRevision{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeRevisionRepository) Create(db *gorm.DB, item *models.KnowledgeRevision) error {
	return db.Create(item).Error
}

func (r *knowledgeRevisionRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.KnowledgeRevision{}).Where("id = ?", id).Updates(columns).Error
}

func (r *knowledgeRevisionRepository) FindLatestByEntry(db *gorm.DB, tenantID int64, entryType string, entryID int64) *models.KnowledgeRevision {
	ret := &models.KnowledgeRevision{}
	if err := db.Where("tenant_id = ? AND entry_type = ? AND entry_id = ?", tenantID, entryType, entryID).
		Order("version_no DESC, id DESC").
		First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeRevisionRepository) FindPublishedForEntry(
	db *gorm.DB,
	revisionID, tenantID, knowledgeBaseID int64,
	entryType string,
	entryID int64,
) *models.KnowledgeRevision {
	ret := &models.KnowledgeRevision{}
	if err := db.Where(
		"id = ? AND tenant_id = ? AND knowledge_base_id = ? AND entry_type = ? AND entry_id = ? AND review_status = ?",
		revisionID,
		tenantID,
		knowledgeBaseID,
		entryType,
		entryID,
		"published",
	).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeRevisionRepository) FindByEntry(db *gorm.DB, tenantID int64, entryType string, entryID int64) (list []models.KnowledgeRevision) {
	db.Where("tenant_id = ? AND entry_type = ? AND entry_id = ?", tenantID, entryType, entryID).
		Order("version_no DESC, id DESC").
		Find(&list)
	return
}
